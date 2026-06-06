// Command experiment runs the AI Sovereign FinOps routing experiments (RQ1-RQ6)
// with real OpenAI calls plus the operator's pure engines. Every test is
// journaled (status, duration, details); results are written as CSV. No test is
// skipped: any API error aborts the run so results are never silently missing.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/catalog"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/journal"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/quality"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/runner"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
)

func main() {
	keyPath := flag.String("key", "../docs/openaikey.txt", "path to OpenAI API key file")
	resultsDir := flag.String("results", "results", "results directory")
	datasetsDir := flag.String("datasets", "datasets", "datasets directory")
	judgeModel := flag.String("judge", "gpt-4o", "LLM-as-judge model")
	maxTokens := flag.Int("max-tokens", 256, "max completion tokens per answer")
	timeoutMin := flag.Int("timeout-min", 40, "overall timeout in minutes")
	smoke := flag.Bool("smoke", false, "run a single real call to validate the pipeline")
	flag.Parse()

	j, err := journal.New(*resultsDir)
	if err != nil {
		die("journal", err)
	}
	defer j.Close()

	key, err := llm.LoadKey(*keyPath)
	if err != nil {
		die("key", err)
	}
	client := llm.NewOpenAI(key)

	if *smoke {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := j.Run("setup", "openai-smoke-gpt-4o-mini", func() (map[string]any, error) {
			r, err := client.Chat(ctx, "gpt-4o-mini", []llm.Message{{Role: "user", Content: "Reply with exactly: OK"}}, 8, 0)
			if err != nil {
				return nil, err
			}
			return map[string]any{"reply": r.Text, "latencyMs": r.LatencyMS, "inTok": r.Usage.InputTokens, "outTok": r.Usage.OutputTokens}, nil
		}); err != nil {
			die("smoke", err)
		}
		report(j)
		return
	}

	ws, err := workload.Load(*datasetsDir)
	if err != nil {
		die("datasets", err)
	}
	if len(ws) == 0 {
		die("datasets", fmt.Errorf("no workloads found in %s", *datasetsDir))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)
	defer cancel()

	eng := runner.New(client, quality.Judge{Client: client, Model: *judgeModel}, catalog.Default(), ws, j, *resultsDir)
	eng.MaxTokens = *maxTokens

	cachePath := *resultsDir + "/cache.json"
	if err := eng.LoadCache(cachePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cache load: %v\n", err)
	}

	fmt.Printf("Running experiments: %d workloads, %d prompts, judge=%s\n", len(ws), workload.TotalPrompts(ws), *judgeModel)

	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"RQ1-3 main matrix (cost/quality/latency)", eng.RunMainMatrix},
		{"RQ4 sovereignty", eng.RunSovereignty},
		{"RQ5 budget degradation", eng.RunBudget},
		{"RQ6 break-even (modeled)", eng.RunBreakEven},
		{"RQ8 ablation", eng.RunAblation},
	}
	for _, s := range steps {
		fmt.Printf("\n=== %s ===\n", s.name)
		if err := s.fn(ctx); err != nil {
			_ = eng.WriteCallsCSV()
			_ = eng.SaveCache(cachePath)
			die(s.name, err)
		}
		_ = eng.SaveCache(cachePath)
	}
	if err := eng.WriteCallsCSV(); err != nil {
		die("calls.csv", err)
	}
	report(j)
}

func report(j *journal.Journal) {
	pass, fail := j.Summary()
	fmt.Printf("\nJournal: %d PASS, %d FAIL (see results/TEST_STATUS.md)\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func die(stage string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", stage, err)
	os.Exit(1)
}
