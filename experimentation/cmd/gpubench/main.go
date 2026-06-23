// Command gpubench load-tests an OpenAI-compatible chat endpoint (self-hosted
// vLLM on GPU, or OpenAI for local validation). It measures throughput
// (tokens/s), latency p50/p95/p99, error rate, and — when a DCGM/Prometheus
// metrics URL is given — average GPU utilization. Results go to CSV + journal.
//
// Local validation (no GPU needed), proving the whole path before Azure:
//
//	go run ./cmd/gpubench -base https://api.openai.com/v1 -key ../operateur/docs/openaikey.txt \
//	  -model gpt-4o-mini -concurrency 4 -requests 20 -label local-openai
//
// On Azure (vLLM): -base http://<vllm-svc>:8000/v1 -model <served-model> -dcgm http://<dcgm>:9400/metrics
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/journal"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
)

func main() {
	base := flag.String("base", "https://api.openai.com/v1", "OpenAI-compatible API base URL (.../v1)")
	keyPath := flag.String("key", "", "API key file (optional; empty for unauthenticated vLLM)")
	model := flag.String("model", "gpt-4o-mini", "served model id")
	datasetsDir := flag.String("datasets", "datasets", "datasets dir (prompts cycled)")
	concurrency := flag.Int("concurrency", 4, "concurrent in-flight requests")
	requests := flag.Int("requests", 40, "total measured requests")
	warmup := flag.Int("warmup", 4, "warmup requests (not measured)")
	maxTokens := flag.Int("max-tokens", 128, "max completion tokens")
	dcgm := flag.String("dcgm", "", "optional DCGM/Prometheus metrics URL for GPU utilization")
	results := flag.String("results", "results", "results dir")
	label := flag.String("label", "bench", "run label (used in filenames/rows)")
	flag.Parse()

	j, err := journal.New(*results)
	if err != nil {
		die("journal", err)
	}
	defer j.Close()

	key := ""
	if *keyPath != "" {
		if k, err := llm.LoadKey(*keyPath); err == nil {
			key = k
		} else {
			die("key", err)
		}
	}
	client := llm.NewOpenAICompatible(*base, key, "bench")

	prompts := loadPrompts(*datasetsDir)
	if len(prompts) == 0 {
		prompts = []string{"Summarize the benefits of Kubernetes in two sentences."}
	}

	err = j.Run("gpubench", *label, func() (map[string]any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Warmup (not measured).
		for i := 0; i < *warmup; i++ {
			_, _ = client.Chat(ctx, *model, msg(prompts[i%len(prompts)]), *maxTokens, 0)
		}

		// Optional GPU utilization sampling.
		var gpuMu sync.Mutex
		var gpuSamples []float64
		stopGPU := make(chan struct{})
		if *dcgm != "" {
			go func() {
				t := time.NewTicker(time.Second)
				defer t.Stop()
				for {
					select {
					case <-stopGPU:
						return
					case <-t.C:
						if u, ok := scrapeGPUUtil(*dcgm); ok {
							gpuMu.Lock()
							gpuSamples = append(gpuSamples, u)
							gpuMu.Unlock()
						}
					}
				}
			}()
		}

		type res struct {
			latMS   float64
			outTok  int
			inTok   int
			errored bool
		}
		jobs := make(chan int)
		out := make(chan res, *requests)
		var wg sync.WaitGroup
		for w := 0; w < *concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range jobs {
					r, err := client.Chat(ctx, *model, msg(prompts[idx%len(prompts)]), *maxTokens, 0)
					if err != nil {
						out <- res{errored: true}
						continue
					}
					out <- res{latMS: float64(r.LatencyMS), outTok: r.Usage.OutputTokens, inTok: r.Usage.InputTokens}
				}
			}()
		}

		start := time.Now()
		go func() {
			for i := 0; i < *requests; i++ {
				jobs <- i
			}
			close(jobs)
		}()
		wg.Wait()
		wall := time.Since(start).Seconds()
		close(out)
		close(stopGPU)

		var lats []float64
		totalOut, totalIn, errs := 0, 0, 0
		for r := range out {
			if r.errored {
				errs++
				continue
			}
			lats = append(lats, r.latMS)
			totalOut += r.outTok
			totalIn += r.inTok
		}
		served := len(lats)
		tokPerSec := 0.0
		reqPerSec := 0.0
		if wall > 0 {
			tokPerSec = float64(totalOut) / wall
			reqPerSec = float64(served) / wall
		}
		gpuAvg := mean(samplesCopy(&gpuMu, gpuSamples))

		row := []string{
			*label, *model, *base,
			itoa(*concurrency), itoa(served), itoa(errs),
			f2(wall), f2(reqPerSec), f2(tokPerSec),
			f2(pct(lats, 50)), f2(pct(lats, 95)), f2(pct(lats, 99)), f2(mean(lats)),
			itoa(totalIn), itoa(totalOut), f2(gpuAvg),
		}
		writeCSVRow(*results+"/gpubench.csv",
			[]string{"label", "model", "base", "concurrency", "served", "errors",
				"wall_s", "req_per_s", "tokens_per_s", "lat_p50_ms", "lat_p95_ms", "lat_p99_ms", "lat_mean_ms",
				"in_tokens", "out_tokens", "gpu_util_avg_pct"}, row)

		return map[string]any{
			"served": served, "errors": errs, "tokensPerSec": round(tokPerSec),
			"reqPerSec": round(reqPerSec), "p95ms": round(pct(lats, 95)), "gpuUtilAvg": round(gpuAvg),
		}, nil
	})
	if err != nil {
		die("bench", err)
	}
	pass, fail := j.Summary()
	fmt.Printf("gpubench done: %d pass, %d fail -> %s/gpubench.csv\n", pass, fail, *results)
}

func msg(text string) []llm.Message { return []llm.Message{{Role: "user", Content: text}} }

func loadPrompts(dir string) []string {
	ws, err := workload.Load(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, w := range ws {
		for _, p := range w.Prompts {
			out = append(out, p.Text)
		}
	}
	return out
}

var dcgmRe = regexp.MustCompile(`(?m)^DCGM_FI_DEV_GPU_UTIL\b[^ ]*\s+([0-9.]+)`)

func scrapeGPUUtil(url string) (float64, bool) {
	resp, err := http.Get(url) //nolint:gosec // internal metrics endpoint
	if err != nil {
		return 0, false
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	ms := dcgmRe.FindAllStringSubmatch(string(b), -1)
	if len(ms) == 0 {
		return 0, false
	}
	sum, n := 0.0, 0
	for _, m := range ms {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

func samplesCopy(mu *sync.Mutex, s []float64) []float64 {
	mu.Lock()
	defer mu.Unlock()
	return append([]float64(nil), s...)
}

func pct(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	i := int(math.Ceil(p/100*float64(len(s)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		i = len(s) - 1
	}
	return s[i]
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func round(x float64) float64 { return math.Round(x*100) / 100 }
func f2(x float64) string     { return fmt.Sprintf("%.2f", x) }
func itoa(i int) string       { return strconv.Itoa(i) }

func writeCSVRow(path string, header, row []string) {
	_, err := os.Stat(path)
	newFile := os.IsNotExist(err)
	f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if e != nil {
		return
	}
	defer func() { _ = f.Close() }()
	if newFile {
		fmt.Fprintln(f, join(header))
	}
	fmt.Fprintln(f, join(row))
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

func die(stage string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", stage, err)
	os.Exit(1)
}
