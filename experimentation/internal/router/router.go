// Package router implements the routing strategies compared in the experiments:
// the baselines (B1-B5) and the economic-aware, sovereignty-constrained,
// budget-aware approach (B6, "Ours"). It reuses the operator's sovereignty
// normalization so routing constraints match the operator's semantics.
package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/catalog"
	"github.com/imperium/ai-sovereign-finops-operator/internal/sovereigntyengine"
)

// SovScenario is a declarative sovereignty constraint set (RQ4).
type SovScenario struct {
	Name                     string
	AllowedZones             []string
	ForbiddenZones           []string
	ExternalProvidersAllowed bool
}

// Scenarios returns the RQ4 sovereignty scenarios.
func Scenarios() []SovScenario {
	return []SovScenario{
		{Name: "global", ExternalProvidersAllowed: true},
		{Name: "eu-only", AllowedZones: []string{"EU"}, ExternalProvidersAllowed: true},
		{Name: "france-only", AllowedZones: []string{"FR"}, ExternalProvidersAllowed: true},
		{Name: "no-external-sensitive", ExternalProvidersAllowed: false},
		{Name: "self-hosted-only", AllowedZones: []string{"FR"}, ExternalProvidersAllowed: false},
	}
}

// RequestContext carries everything a strategy needs to decide.
type RequestContext struct {
	Team            string
	Namespace       string
	Sensitive       bool
	AllowedModels   []string
	PremiumModel    string
	MinQuality      float64
	BudgetTotalEUR  float64
	BudgetUsedEUR   float64
	Scenario        SovScenario
	EstInputTokens  int
	EstOutputTokens int
}

// Decision is a routing outcome.
type Decision struct {
	ModelID         string
	Reason          string
	ExpectedCostEUR float64
	RejectedModels  []string
	Findings        []string
	Fallback        bool
	Blocked         bool // B5 hard block when budget exhausted
}

// Strategy chooses a model for a request.
type Strategy interface {
	Name() string
	Choose(ctx RequestContext, models map[string]catalog.Model) Decision
}

// estCost returns the estimated EUR cost of a request on a model using priors.
func estCost(m catalog.Model, inTok, outTok int) float64 {
	return float64(inTok)/1e6*m.InPerMillion + float64(outTok)/1e6*m.OutPerMillion
}

// violatesSovereignty reports whether a model breaks a hard constraint, reusing
// the operator's zone normalization (EU covers EU member countries).
func violatesSovereignty(m catalog.Model, s SovScenario, sensitive bool) bool {
	zone := sovereigntyengine.NormalizeZone(m.Zone)
	for _, f := range s.ForbiddenZones {
		if sovereigntyengine.NormalizeZone(f) == zone {
			return true
		}
	}
	if len(s.AllowedZones) > 0 && !zoneAllowed(zone, s.AllowedZones) {
		return true
	}
	if sensitive && !s.ExternalProvidersAllowed && m.Managed {
		return true
	}
	return false
}

var euMembers = map[string]bool{"FR": true, "DE": true, "ES": true, "IT": true, "NL": true, "BE": true,
	"LU": true, "IE": true, "PT": true, "AT": true, "FI": true, "SE": true, "DK": true, "PL": true}

func zoneAllowed(zone string, allowed []string) bool {
	for _, a := range allowed {
		na := sovereigntyengine.NormalizeZone(a)
		if na == zone {
			return true
		}
		if na == "EU" && euMembers[zone] {
			return true
		}
	}
	return false
}

// Violates reports whether a model breaks the scenario's hard sovereignty
// constraints for a (possibly sensitive) request. Exported for measurement.
func Violates(m catalog.Model, s SovScenario, sensitive bool) bool {
	return violatesSovereignty(m, s, sensitive)
}

// candidates returns allowed, sovereignty-compliant models for the request,
// plus the list of model IDs rejected by sovereignty.
func candidates(ctx RequestContext, models map[string]catalog.Model) (ok []catalog.Model, rejected []string) {
	for _, id := range ctx.AllowedModels {
		m, exists := models[id]
		if !exists {
			continue
		}
		if violatesSovereignty(m, ctx.Scenario, ctx.Sensitive) {
			rejected = append(rejected, id)
			continue
		}
		ok = append(ok, m)
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].ID < ok[j].ID })
	sort.Strings(rejected)
	return ok, rejected
}

func findingsFor(rejected []string) []string {
	if len(rejected) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("sovereignty rejected: %s", strings.Join(rejected, ","))}
}

// ---------- B1 Premium Static ----------

type PremiumStatic struct{}

func (PremiumStatic) Name() string { return "B1-premium-static" }
func (PremiumStatic) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	_, rejected := candidates(ctx, models)
	m := models[ctx.PremiumModel]
	return Decision{ModelID: m.ID, Reason: "always premium", ExpectedCostEUR: estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B2 Round Robin ----------

type RoundRobin struct{ n int }

func (r *RoundRobin) Name() string { return "B2-round-robin" }
func (r *RoundRobin) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	ok, rejected := candidates(ctx, models)
	if len(ok) == 0 {
		return blockedNoCandidate(rejected)
	}
	m := ok[r.n%len(ok)]
	r.n++
	return Decision{ModelID: m.ID, Reason: "round-robin", ExpectedCostEUR: estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B3 Least Cost ----------

type LeastCost struct{}

func (LeastCost) Name() string { return "B3-least-cost" }
func (LeastCost) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	ok, rejected := candidates(ctx, models)
	if len(ok) == 0 {
		return blockedNoCandidate(rejected)
	}
	best := ok[0]
	for _, m := range ok[1:] {
		if estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens) < estCost(best, ctx.EstInputTokens, ctx.EstOutputTokens) {
			best = m
		}
	}
	return Decision{ModelID: best.ID, Reason: "cheapest allowed", ExpectedCostEUR: estCost(best, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B4 Static Policy (by namespace) ----------

type StaticPolicy struct{}

func (StaticPolicy) Name() string { return "B4-static-policy" }
func (StaticPolicy) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	mapping := map[string]string{"hr": "gpt-4.1-nano", "rag": "gpt-4o-mini", "dev": "gpt-4o", "agent": "gpt-4o"}
	pick := mapping[ctx.Namespace]
	if pick == "" {
		pick = ctx.PremiumModel
	}
	ok, rejected := candidates(ctx, models)
	// Respect sovereignty: if mapped model is rejected, fall back to first candidate.
	if violatesSovereignty(models[pick], ctx.Scenario, ctx.Sensitive) {
		if len(ok) == 0 {
			return blockedNoCandidate(rejected)
		}
		m := ok[0]
		return Decision{ModelID: m.ID, Reason: "static mapping rerouted (sovereignty)", ExpectedCostEUR: estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens),
			RejectedModels: rejected, Findings: findingsFor(rejected), Fallback: true}
	}
	m := models[pick]
	return Decision{ModelID: m.ID, Reason: "static namespace mapping", ExpectedCostEUR: estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B5 Budget Hard Block ----------

type BudgetHardBlock struct{}

func (BudgetHardBlock) Name() string { return "B5-budget-hard-block" }
func (BudgetHardBlock) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	_, rejected := candidates(ctx, models)
	if ctx.BudgetUsedEUR >= ctx.BudgetTotalEUR {
		return Decision{Blocked: true, Reason: "budget exhausted: hard block", RejectedModels: rejected, Findings: findingsFor(rejected)}
	}
	m := models[ctx.PremiumModel]
	return Decision{ModelID: m.ID, Reason: "premium within budget", ExpectedCostEUR: estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B6 Ours: economic-aware, sovereignty-constrained, budget-aware ----------

// Weights for the scoring function (lower score = preferred).
type Weights struct{ Alpha, Beta, Gamma, Delta, Epsilon float64 }

// DefaultWeights are the experiment defaults (documented in methodology).
func DefaultWeights() Weights {
	return Weights{Alpha: 1.0, Beta: 1.5, Gamma: 0.3, Delta: 0.5, Epsilon: 1.0}
}

type Ours struct{ W Weights }

func (Ours) Name() string { return "B6-ours" }

func (o Ours) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	ok, rejected := candidates(ctx, models)
	if len(ok) == 0 {
		return blockedNoCandidate(rejected)
	}
	premium := models[ctx.PremiumModel]
	premiumCost := estCost(premium, ctx.EstInputTokens, ctx.EstOutputTokens)
	if premiumCost <= 0 {
		premiumCost = 1e-9
	}
	premiumQ := premium.QualityPrior
	if premiumQ <= 0 {
		premiumQ = 1
	}
	budgetPressure := 0.0
	if ctx.BudgetTotalEUR > 0 {
		budgetPressure = ctx.BudgetUsedEUR / ctx.BudgetTotalEUR
		if budgetPressure > 1 {
			budgetPressure = 1
		}
	}
	exhausted := ctx.BudgetTotalEUR > 0 && ctx.BudgetUsedEUR >= ctx.BudgetTotalEUR

	// Quality gate: normally require >= MinQuality; relax when budget exhausted
	// (graceful degradation keeps the service available instead of blocking).
	var pool []catalog.Model
	for _, m := range ok {
		if m.QualityPrior >= ctx.MinQuality || exhausted {
			pool = append(pool, m)
		}
	}
	degraded := false
	if len(pool) == 0 {
		pool = ok // last resort: best-effort rather than fail
		degraded = true
	}

	maxLat := 1.0
	for _, m := range pool {
		if m.LatencyPriorMS > maxLat {
			maxLat = m.LatencyPriorMS
		}
	}

	best := pool[0]
	bestScore := 1e18
	for _, m := range pool {
		normCost := estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens) / premiumCost
		qualityLoss := (premiumQ - m.QualityPrior) / premiumQ
		if qualityLoss < 0 {
			qualityLoss = 0
		}
		latPenalty := m.LatencyPriorMS / maxLat
		sovRisk := 0.0
		if m.Managed && !ctx.Scenario.ExternalProvidersAllowed {
			sovRisk = 1.0
		}
		score := o.W.Alpha*normCost*(1+o.W.Epsilon*budgetPressure) +
			o.W.Beta*qualityLoss + o.W.Gamma*latPenalty + o.W.Delta*sovRisk
		if score < bestScore {
			bestScore = score
			best = m
		}
	}

	reason := fmt.Sprintf("score-optimal (budgetPressure=%.2f)", budgetPressure)
	if exhausted {
		reason = "budget exhausted: graceful degradation"
	}
	return Decision{
		ModelID: best.ID, Reason: reason,
		ExpectedCostEUR: estCost(best, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels:  rejected, Findings: findingsFor(rejected),
		Fallback: degraded || best.ID != ctx.PremiumModel,
	}
}

func blockedNoCandidate(rejected []string) Decision {
	return Decision{Blocked: true, Reason: "no sovereignty-compliant model available", RejectedModels: rejected, Findings: findingsFor(rejected)}
}

// ---------- B7 Difficulty router (Hybrid-LLM / RouteLLM-style proxy) ----------

// DifficultyRouter routes by an estimated query difficulty/quality need, like the
// learned difficulty routers in the literature — but using a transparent
// heuristic (token length + the workload's minimum-quality requirement) rather
// than a trained predictor. It is a stronger, literature-aligned baseline than
// the naïve B1-B5, and it respects sovereignty (chooses among compliant models).
type DifficultyRouter struct{}

func (DifficultyRouter) Name() string { return "B7-difficulty-router" }

func (DifficultyRouter) Choose(ctx RequestContext, models map[string]catalog.Model) Decision {
	ok, rejected := candidates(ctx, models)
	if len(ok) == 0 {
		return blockedNoCandidate(rejected)
	}
	lenScore := float64(ctx.EstInputTokens) / 400.0
	if lenScore > 1 {
		lenScore = 1
	}
	// The quality requirement dominates (Hybrid-LLM-style: quality-needy/long
	// queries -> stronger models), so it does not collapse to always-cheapest.
	difficulty := ctx.MinQuality
	if lenScore > difficulty {
		difficulty = lenScore
	}
	var best catalog.Model
	found := false
	for _, m := range ok {
		if m.QualityPrior >= difficulty {
			if !found || estCost(m, ctx.EstInputTokens, ctx.EstOutputTokens) < estCost(best, ctx.EstInputTokens, ctx.EstOutputTokens) {
				best, found = m, true
			}
		}
	}
	if !found {
		best = ok[0]
		for _, m := range ok[1:] {
			if m.QualityPrior > best.QualityPrior {
				best = m
			}
		}
	}
	return Decision{ModelID: best.ID, Reason: fmt.Sprintf("difficulty=%.2f", difficulty),
		ExpectedCostEUR: estCost(best, ctx.EstInputTokens, ctx.EstOutputTokens),
		RejectedModels:  rejected, Findings: findingsFor(rejected), Fallback: best.ID != ctx.PremiumModel}
}

// All returns the full strategy set in canonical order. RoundRobin is stateful,
// so a fresh instance is created per call site.
func All() []Strategy {
	return []Strategy{
		PremiumStatic{}, &RoundRobin{}, LeastCost{}, StaticPolicy{}, BudgetHardBlock{},
		DifficultyRouter{}, Ours{W: DefaultWeights()},
	}
}
