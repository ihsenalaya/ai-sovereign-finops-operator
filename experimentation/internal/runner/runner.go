// Package runner executes the experiment scenarios (RQ1-RQ6) with real LLM calls
// (OpenAI) plus the operator's pure engines, journaling every test and writing
// reproducible CSV results. Real and modeled data are tagged distinctly.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/catalog"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/journal"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/llm"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/quality"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
)

// Engine orchestrates the experiments.
type Engine struct {
	Client     llm.Client            // default/fallback client
	Clients    map[string]llm.Client // per-provider clients (keyed by Model.Provider)
	Judge      quality.Judge
	Models     map[string]catalog.Model
	ModelList  []catalog.Model
	Workloads  []workload.Workload
	J          *journal.Journal
	ResultsDir string
	MaxTokens  int
	Temp       float64 // answer temperature (0 = deterministic; >0 for multi-rep variation)
	Bypass     bool    // when true, skip answer/judge caches (fresh calls each rep)

	answerCache map[string]llm.Response // modelID|promptID -> response
	acceptCache map[string]float64      // modelID|promptID -> 1..5
	pairCache   map[string]string       // modelID|promptID -> win/tie/lose
	records     []callRecord            // full per-call log (calls.csv)
}

// New builds an Engine. clients maps a provider name (Model.Provider) to its LLM
// client; client is the default/fallback used when a provider has no entry.
func New(client llm.Client, clients map[string]llm.Client, judge quality.Judge, models []catalog.Model, ws []workload.Workload, j *journal.Journal, resultsDir string) *Engine {
	if clients == nil {
		clients = map[string]llm.Client{}
	}
	return &Engine{
		Client: client, Clients: clients, Judge: judge, Models: catalog.ByID(models), ModelList: models,
		Workloads: ws, J: j, ResultsDir: resultsDir, MaxTokens: 256,
		answerCache: map[string]llm.Response{}, acceptCache: map[string]float64{}, pairCache: map[string]string{},
	}
}

// clientFor returns the client for a model's provider (fallback: default).
func (e *Engine) clientFor(m catalog.Model) llm.Client {
	if c, ok := e.Clients[m.Provider]; ok && c != nil {
		return c
	}
	return e.Client
}

func estTokens(s string) int {
	n := len(s) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// callModel returns an answer for (model, prompt). Real models hit the live API
// (cached); the modeled self-hosted model returns a deterministic stub with
// modeled cost/latency.
func (e *Engine) callModel(ctx context.Context, m catalog.Model, p workload.Prompt) (llm.Response, bool, error) {
	key := m.ID + "|" + p.ID
	if !e.Bypass {
		if r, ok := e.answerCache[key]; ok {
			return r, m.Real, nil
		}
	}
	if !m.Real {
		in := estTokens(p.System + p.Text)
		r := llm.Response{
			Text:      "[modeled self-hosted answer; quality from prior]",
			Usage:     llm.Usage{InputTokens: in, OutputTokens: 80},
			LatencyMS: int64(m.LatencyPriorMS), Model: m.ID, Provider: m.Provider,
		}
		if !e.Bypass {
			e.answerCache[key] = r
		}
		return r, false, nil
	}
	msgs := []llm.Message{}
	if p.System != "" {
		msgs = append(msgs, llm.Message{Role: "system", Content: p.System})
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: p.Text})
	r, err := e.clientFor(m).Chat(ctx, m.APIModel, msgs, e.MaxTokens, e.Temp)
	if err != nil {
		return llm.Response{}, true, err
	}
	r.Model = m.ID
	if !e.Bypass {
		e.answerCache[key] = r
	}
	return r, true, nil
}

func (e *Engine) cost(m catalog.Model, u llm.Usage) float64 {
	return float64(u.InputTokens)/1e6*m.InPerMillion + float64(u.OutputTokens)/1e6*m.OutPerMillion
}

// acceptability returns a 1..5 score; modeled models use their prior.
func (e *Engine) acceptability(ctx context.Context, m catalog.Model, p workload.Prompt, answer string) (float64, error) {
	if !m.Real {
		return 1 + 4*m.QualityPrior, nil // modeled mapping to 1..5
	}
	key := m.ID + "|" + p.ID
	if !e.Bypass {
		if s, ok := e.acceptCache[key]; ok {
			return s, nil
		}
	}
	s, err := e.Judge.Acceptability(ctx, p.Text, answer)
	if err != nil {
		return 0, err
	}
	if !e.Bypass {
		e.acceptCache[key] = s
	}
	return s, nil
}

// pairwiseVsPremium returns win/tie/lose vs the premium reference answer.
func (e *Engine) pairwiseVsPremium(ctx context.Context, m catalog.Model, p workload.Prompt, premiumID, answer string) (string, error) {
	if !m.Real || m.ID == premiumID {
		return "tie", nil
	}
	key := m.ID + "|" + p.ID
	if w, ok := e.pairCache[key]; ok {
		return w, nil
	}
	ref, ok := e.answerCache[premiumID+"|"+p.ID]
	if !ok {
		return "tie", nil
	}
	w, err := e.Judge.Pairwise(ctx, p.Text, answer, ref.Text)
	if err != nil {
		return "", err
	}
	e.pairCache[key] = w
	return w, nil
}

// callRecord is one routed request outcome.
type callRecord struct {
	Strategy, Workload, Team, Namespace, PromptID, ModelID, Provider, Scenario string
	Real, Modeled, Blocked                                                     bool
	InTok, OutTok                                                              int
	CostEUR                                                                    float64
	LatencyMS                                                                  int64
	RoutingNS                                                                  int64
	Quality1to5                                                                float64
	Win                                                                        string
	SovViolation                                                               bool
	Reroute                                                                    bool
	Rep                                                                        int
}

func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	idx := int(math.Ceil(p/100*float64(len(s)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
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

func writeCSV(path string, header []string, rows [][]string) error {
	var b strings.Builder
	b.WriteString(strings.Join(header, ",") + "\n")
	for _, r := range rows {
		b.WriteString(strings.Join(r, ",") + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func f(x float64) string  { return fmt.Sprintf("%.6f", x) }
func f2(x float64) string { return fmt.Sprintf("%.2f", x) }

// cacheFile is the on-disk persistence format for LLM answers and judge scores,
// making runs reproducible and cheap to repeat (no re-billing identical calls).
type cacheFile struct {
	Answers map[string]llm.Response `json:"answers"`
	Accept  map[string]float64      `json:"accept"`
	Pair    map[string]string       `json:"pair"`
}

// LoadCache loads a persisted response/judge cache if present.
func (e *Engine) LoadCache(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil // no cache yet is fine
	}
	var cf cacheFile
	if err := json.Unmarshal(b, &cf); err != nil {
		return err
	}
	if cf.Answers != nil {
		e.answerCache = cf.Answers
	}
	if cf.Accept != nil {
		e.acceptCache = cf.Accept
	}
	if cf.Pair != nil {
		e.pairCache = cf.Pair
	}
	return nil
}

// SaveCache persists the response/judge cache.
func (e *Engine) SaveCache(path string) error {
	b, err := json.MarshalIndent(cacheFile{Answers: e.answerCache, Accept: e.acceptCache, Pair: e.pairCache}, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func nowNS() int64          { return time.Now().UnixNano() }
func itoa(i int) string     { return strconv.Itoa(i) }
func itoa64(i int64) string { return strconv.FormatInt(i, 10) }
func b2s(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
