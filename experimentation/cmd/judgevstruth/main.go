// Command judgevstruth investigates the exact-match vs LLM-judge contradiction:
// for objective benchmark items (GSM8K/MMLU, ground-truth answers) it records,
// per model, BOTH the objective correctness (exact-match) AND the LLM-judge
// acceptability score. scripts/judge_vs_truth.py then shows whether the judge
// over-rates incorrect answers (which would explain why judge-quality and
// exact-match disagree). All real calls; no fabricated data.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/quality"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/runner"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
)

func main() {
	keyPath := flag.String("key", "../operateur/docs/openaikey.txt", "OpenAI key file")
	benchDir := flag.String("benchmark", "datasets-public", "benchmark datasets (ground-truth)")
	results := flag.String("results", "results", "results dir")
	judgeModel := flag.String("judge", "gpt-4o", "LLM judge model")
	perDataset := flag.Int("per-dataset", 50, "items per dataset")
	flag.Parse()

	key, err := llm.LoadKey(*keyPath)
	if err != nil {
		die("key", err)
	}
	client := llm.NewOpenAI(key)
	judge := quality.Judge{Client: client, Model: *judgeModel}
	models := []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1-nano"}

	bws, err := workload.Load(*benchDir)
	if err != nil {
		die("datasets", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()

	rows := [][]string{}
	n := 0
	for _, w := range bws {
		ps := w.Prompts
		if len(ps) > *perDataset {
			ps = ps[:*perDataset]
		}
		for _, p := range ps {
			for _, m := range models {
				msgs := []llm.Message{}
				if p.System != "" {
					msgs = append(msgs, llm.Message{Role: "system", Content: p.System})
				}
				msgs = append(msgs, llm.Message{Role: "user", Content: p.Text})
				resp, err := client.Chat(ctx, m, msgs, 256, 0)
				if err != nil {
					die("answer", err)
				}
				correct := 0
				if runner.AnswerCorrect(resp.Text, p.Reference) {
					correct = 1
				}
				js, err := judge.Acceptability(ctx, p.Text, resp.Text)
				if err != nil {
					die("judge", err)
				}
				rows = append(rows, []string{w.Name, p.ID, m, fmt.Sprintf("%d", correct), fmt.Sprintf("%.0f", js)})
				n++
				if n%30 == 0 {
					fmt.Printf("  %d graded\n", n)
				}
			}
		}
	}

	out := *results + "/judgevstruth.csv"
	var sb strings.Builder
	sb.WriteString("dataset,prompt,model,correct,judge_score\n")
	for _, r := range rows {
		sb.WriteString(strings.Join(r, ",") + "\n")
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		die("write", err)
	}
	fmt.Printf("wrote %s (%d graded answer/judge pairs)\n", out, len(rows))
}

func die(s string, err error) { fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", s, err); os.Exit(1) }
