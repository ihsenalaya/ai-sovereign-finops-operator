// Command judgeagree measures inter-judge agreement for the quality metric:
// it re-scores a sample of (prompt, answer) pairs with TWO independent judges
// (e.g. gpt-4o and Mistral-Large via Azure AI Foundry) and writes their scores
// to results/judge_agreement.csv. scripts/judge_agreement.py then computes
// Cohen's weighted kappa, Krippendorff's alpha and correlation. This addresses
// the judge-dependence caveat of the quality results.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/quality"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
)

type cacheFile struct {
	Answers map[string]llm.Response `json:"answers"`
}

func main() {
	keyPath := flag.String("key", "../docs/openaikey.txt", "OpenAI key file")
	cachePath := flag.String("cache", "results/cache.json", "answer cache to sample from")
	datasetsDir := flag.String("datasets", "datasets", "datasets dir")
	results := flag.String("results", "results", "results dir")
	judge1 := flag.String("judge1", "gpt-4o", "judge #1 model (OpenAI)")
	judge2 := flag.String("judge2", "mistral-large-latest", "judge #2 model (Mistral via Foundry)")
	mistralBase := flag.String("mistral-base", "", "Mistral Foundry base URL (.../models)")
	mistralAPIVer := flag.String("mistral-api-version", "2024-05-01-preview", "Foundry api-version")
	sampleN := flag.Int("n", 120, "number of (prompt,answer) pairs to score")
	flag.Parse()

	key, err := llm.LoadKey(*keyPath)
	if err != nil {
		die("key", err)
	}
	oai := llm.NewOpenAI(key)
	jg1 := quality.Judge{Client: oai, Model: *judge1}

	if *mistralBase == "" {
		die("config", fmt.Errorf("-mistral-base required for the second judge"))
	}
	mkey := strings.TrimSpace(os.Getenv("MISTRAL_API_KEY"))
	mc := llm.NewAzureFoundry(*mistralBase, mkey, "mistral-eu", *mistralAPIVer)
	jg2 := quality.Judge{Client: mc, Model: *judge2}

	// Map prompt IDs to text.
	ws, err := workload.Load(*datasetsDir)
	if err != nil {
		die("datasets", err)
	}
	ptext := map[string]string{}
	for _, w := range ws {
		for _, p := range w.Prompts {
			ptext[p.ID] = p.Text
		}
	}

	b, err := os.ReadFile(*cachePath)
	if err != nil {
		die("cache", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(b, &cf); err != nil {
		die("cache-parse", err)
	}

	// Build a deterministic sample of (promptID, model, answer) over real answers.
	keys := make([]string, 0, len(cf.Answers))
	for k := range cf.Answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type pair struct{ pid, model, answer string }
	var pairs []pair
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		model, pid := parts[0], parts[1]
		if model == "selfhosted-eu-llama" { // modeled stub, skip
			continue
		}
		txt := ptext[pid]
		ans := cf.Answers[k].Text
		if txt == "" || ans == "" {
			continue
		}
		pairs = append(pairs, pair{pid, model, ans})
		if len(pairs) >= *sampleN {
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var rows [][]string
	for i, pr := range pairs {
		s1, err := jg1.Acceptability(ctx, ptext[pr.pid], pr.answer)
		if err != nil {
			die("judge1", err)
		}
		s2, err := jg2.Acceptability(ctx, ptext[pr.pid], pr.answer)
		if err != nil {
			die("judge2", err)
		}
		rows = append(rows, []string{pr.pid, pr.model, fmt.Sprintf("%.0f", s1), fmt.Sprintf("%.0f", s2)})
		if (i+1)%20 == 0 {
			fmt.Printf("  judged %d/%d\n", i+1, len(pairs))
		}
	}

	out := *results + "/judge_agreement.csv"
	var sb strings.Builder
	sb.WriteString("prompt,model,judge_" + *judge1 + ",judge_" + strings.ReplaceAll(*judge2, "-latest", "") + "\n")
	for _, r := range rows {
		sb.WriteString(strings.Join(r, ",") + "\n")
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		die("write", err)
	}
	fmt.Printf("wrote %s (%d pairs, judges: %s vs %s)\n", out, len(rows), *judge1, *judge2)
}

func die(stage string, err error) { fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", stage, err); os.Exit(1) }
