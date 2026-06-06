// Package quality implements LLM-as-judge scoring used for RQ2 (quality
// preservation): absolute acceptability (1-5) and pairwise win-rate vs a
// reference answer. The judge is a real LLM call (configurable model).
package quality

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
)

// Judge wraps a client + judge model.
type Judge struct {
	Client llm.Client
	Model  string
}

var numRe = regexp.MustCompile(`[1-5]`)

// Acceptability returns a 1..5 quality score for an answer to a prompt.
func (j Judge) Acceptability(ctx context.Context, prompt, answer string) (float64, error) {
	sys := "You are a strict evaluator. Rate the assistant answer's quality for the user task on a 1-5 integer scale " +
		"(1=useless/incorrect, 3=acceptable, 5=excellent). Reply with ONLY the integer."
	user := fmt.Sprintf("TASK:\n%s\n\nANSWER:\n%s\n\nScore (1-5):", prompt, answer)
	resp, err := j.Client.Chat(ctx, j.Model, []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}, 4, 0)
	if err != nil {
		return 0, err
	}
	m := numRe.FindString(resp.Text)
	if m == "" {
		return 0, fmt.Errorf("judge returned no score: %q", resp.Text)
	}
	v, _ := strconv.Atoi(m)
	return float64(v), nil
}

// Pairwise compares a candidate answer against a reference. Returns "win",
// "tie", or "lose" (candidate vs reference).
func (j Judge) Pairwise(ctx context.Context, prompt, candidate, reference string) (string, error) {
	sys := "You compare two answers (A and B) to the same task. Decide which is better. " +
		"Reply with ONLY one token: A, B, or TIE."
	user := fmt.Sprintf("TASK:\n%s\n\nANSWER A:\n%s\n\nANSWER B:\n%s\n\nWhich is better (A/B/TIE)?", prompt, candidate, reference)
	resp, err := j.Client.Chat(ctx, j.Model, []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}, 4, 0)
	if err != nil {
		return "", err
	}
	t := strings.ToUpper(strings.TrimSpace(resp.Text))
	switch {
	case strings.HasPrefix(t, "A"):
		return "win", nil
	case strings.HasPrefix(t, "B"):
		return "lose", nil
	default:
		return "tie", nil
	}
}
