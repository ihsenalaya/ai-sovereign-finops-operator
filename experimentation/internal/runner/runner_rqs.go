package runner

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/catalog"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/router"
	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/workload"
	"github.com/imperium/ai-sovereign-finops-operator/internal/breakevenengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
)

func round(x float64) float64 { return math.Round(x*1e6) / 1e6 }

// RunMainMatrix executes RQ1 (cost), RQ2 (quality), RQ3 (latency) in one pass
// over all strategies x workloads x prompts under the unrestricted scenario.
func (e *Engine) RunMainMatrix(ctx context.Context) error {
	global := router.SovScenario{Name: "global", ExternalProvidersAllowed: true}

	// Pre-warm premium references (needed for pairwise quality).
	for _, w := range e.Workloads {
		pm := e.Models[w.PremiumModel]
		for _, p := range w.Prompts {
			if _, _, err := e.callModel(ctx, pm, p); err != nil {
				return fmt.Errorf("prewarm premium %s/%s: %w", w.Name, p.ID, err)
			}
		}
	}

	var matrix []callRecord
	for _, strat := range router.All() {
		sname := strat.Name()
		for wi := range e.Workloads {
			w := e.Workloads[wi]
			err := e.J.Run("RQ1-3-matrix", sname+" / "+w.Name, func() (map[string]any, error) {
				var recs []callRecord
				for _, p := range w.Prompts {
					rc := callRecord{Strategy: sname, Workload: w.Name, Team: w.Team, Namespace: w.Namespace, PromptID: p.ID, Scenario: "global"}
					rctx := router.RequestContext{
						Team: w.Team, Namespace: w.Namespace, Sensitive: p.Sensitive || w.DefaultSensitive,
						AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: w.MinQuality,
						BudgetTotalEUR: w.MonthlyBudgetEUR, BudgetUsedEUR: 0, Scenario: global,
						EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
					}
					t0 := nowNS()
					dec := strat.Choose(rctx, e.Models)
					rc.RoutingNS = nowNS() - t0
					if dec.Blocked {
						rc.Blocked = true
						recs = append(recs, rc)
						continue
					}
					m := e.Models[dec.ModelID]
					rc.ModelID, rc.Provider, rc.Real, rc.Modeled = m.ID, m.Provider, m.Real, !m.Real
					rc.Reroute = m.ID != w.PremiumModel
					resp, _, err := e.callModel(ctx, m, p)
					if err != nil {
						return nil, err
					}
					rc.InTok, rc.OutTok = resp.Usage.InputTokens, resp.Usage.OutputTokens
					rc.CostEUR, rc.LatencyMS = e.cost(m, resp.Usage), resp.LatencyMS
					q, err := e.acceptability(ctx, m, p, resp.Text)
					if err != nil {
						return nil, err
					}
					rc.Quality1to5 = q
					win, err := e.pairwiseVsPremium(ctx, m, p, w.PremiumModel, resp.Text)
					if err != nil {
						return nil, err
					}
					rc.Win = win
					recs = append(recs, rc)
				}
				matrix = append(matrix, recs...)
				tot, served, blocked := 0.0, 0, 0
				var qs []float64
				for _, r := range recs {
					if r.Blocked {
						blocked++
						continue
					}
					served++
					tot += r.CostEUR
					qs = append(qs, r.Quality1to5)
				}
				return map[string]any{"served": served, "blocked": blocked, "costEUR": round(tot), "meanQuality1to5": round(mean(qs))}, nil
			})
			if err != nil {
				return err
			}
		}
	}
	e.records = append(e.records, matrix...)
	return e.writeMatrixCSVs(matrix)
}

func (e *Engine) writeMatrixCSVs(matrix []callRecord) error {
	type agg struct {
		cost          float64
		served, block int
		inTok, outTok int
		qNorm         []float64
		accept        int
		win, tie, los int
		lat           []float64
		routeNS       []float64
	}
	by := map[string]*agg{}
	order := []string{}
	for _, r := range matrix {
		a := by[r.Strategy]
		if a == nil {
			a = &agg{}
			by[r.Strategy] = a
			order = append(order, r.Strategy)
		}
		a.routeNS = append(a.routeNS, float64(r.RoutingNS))
		if r.Blocked {
			a.block++
			continue
		}
		a.served++
		a.cost += r.CostEUR
		a.inTok += r.InTok
		a.outTok += r.OutTok
		a.qNorm = append(a.qNorm, (r.Quality1to5-1)/4)
		if r.Quality1to5 >= 3 {
			a.accept++
		}
		a.lat = append(a.lat, float64(r.LatencyMS))
		if r.ModelID != "" && r.Reroute && r.Real { // pairwise only meaningful for non-premium real
			switch r.Win {
			case "win":
				a.win++
			case "tie":
				a.tie++
			case "lose":
				a.los++
			}
		}
	}
	premiumTotal := 0.0
	if a := by["B1-premium-static"]; a != nil {
		premiumTotal = a.cost
	}

	// RQ1 cost
	rows1 := [][]string{}
	for _, s := range order {
		a := by[s]
		tokens := a.inTok + a.outTok
		costPerReq, costPerTok, savings := 0.0, 0.0, 0.0
		if a.served > 0 {
			costPerReq = a.cost / float64(a.served)
		}
		if tokens > 0 {
			costPerTok = a.cost / float64(tokens)
		}
		if premiumTotal > 0 {
			savings = (premiumTotal - a.cost) / premiumTotal * 100
		}
		rows1 = append(rows1, []string{s, f(a.cost), itoa(a.served), itoa(a.block), f(costPerReq), f(costPerTok), f2(savings)})
	}
	if err := writeCSV(filepath.Join(e.ResultsDir, "rq1_cost.csv"),
		[]string{"strategy", "total_cost_eur", "served", "blocked", "cost_per_request_eur", "cost_per_token_eur", "savings_vs_premium_pct"}, rows1); err != nil {
		return err
	}

	// RQ1 cost by team (via the operator's costengine)
	rowsT := [][]string{}
	pb := catalog.PriceBook(e.ModelList)
	for _, s := range order {
		var samples []collectors.UsageSample
		for _, r := range matrix {
			if r.Strategy != s || r.Blocked {
				continue
			}
			samples = append(samples, collectors.UsageSample{
				Namespace: r.Namespace, Team: r.Team, Application: r.Workload,
				Provider: r.Provider, Model: r.ModelID, Requests: 1,
				InputTokens: int64(r.InTok), OutputTokens: int64(r.OutTok),
			})
		}
		bd := costengine.Compute(samples, pb)
		teams := make([]string, 0, len(bd.ByTeam))
		for t := range bd.ByTeam {
			teams = append(teams, t)
		}
		sort.Strings(teams)
		for _, t := range teams {
			rowsT = append(rowsT, []string{s, t, f(bd.ByTeam[t].CostTotal)})
		}
	}
	if err := writeCSV(filepath.Join(e.ResultsDir, "rq1_cost_by_team.csv"),
		[]string{"strategy", "team", "cost_eur"}, rowsT); err != nil {
		return err
	}

	// RQ2 quality
	rows2 := [][]string{}
	for _, s := range order {
		a := by[s]
		acceptRate := 0.0
		if a.served > 0 {
			acceptRate = float64(a.accept) / float64(a.served) * 100
		}
		comp := a.win + a.tie + a.los
		winRate := 0.0
		if comp > 0 {
			winRate = (float64(a.win) + 0.5*float64(a.tie)) / float64(comp) * 100
		}
		rows2 = append(rows2, []string{s, f(mean(a.qNorm)), f2(acceptRate), f2(winRate), itoa(comp)})
	}
	if err := writeCSV(filepath.Join(e.ResultsDir, "rq2_quality.csv"),
		[]string{"strategy", "mean_quality_norm", "acceptable_rate_pct", "winrate_vs_premium_pct", "pairwise_comparisons"}, rows2); err != nil {
		return err
	}

	// RQ3 latency + routing overhead
	rows3 := [][]string{}
	for _, s := range order {
		a := by[s]
		rows3 = append(rows3, []string{s,
			f2(percentile(a.lat, 50)), f2(percentile(a.lat, 95)), f2(percentile(a.lat, 99)),
			f2(mean(a.lat)), f2(mean(a.routeNS) / 1000.0)})
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq3_latency.csv"),
		[]string{"strategy", "latency_p50_ms", "latency_p95_ms", "latency_p99_ms", "latency_mean_ms", "routing_decision_us"}, rows3)
}

// RunSovereignty executes RQ4: cost/quality/violations across sovereignty
// scenarios, contrasting a sovereignty-blind baseline (B1) with Ours.
func (e *Engine) RunSovereignty(ctx context.Context) error {
	strategies := []router.Strategy{router.PremiumStatic{}, router.Ours{W: router.DefaultWeights()}}
	rows := [][]string{}
	for _, sc := range router.Scenarios() {
		for _, strat := range strategies {
			sc, strat := sc, strat
			err := e.J.Run("RQ4-sovereignty", sc.Name+" / "+strat.Name(), func() (map[string]any, error) {
				cost, served, blocked, violations, reroutes := 0.0, 0, 0, 0, 0
				var qs []float64
				for wi := range e.Workloads {
					w := e.Workloads[wi]
					for _, p := range w.Prompts {
						sensitive := p.Sensitive || w.DefaultSensitive
						rctx := router.RequestContext{
							Team: w.Team, Namespace: w.Namespace, Sensitive: sensitive,
							AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: w.MinQuality,
							BudgetTotalEUR: w.MonthlyBudgetEUR, Scenario: sc,
							EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
						}
						dec := strat.Choose(rctx, e.Models)
						if dec.Blocked {
							blocked++
							continue
						}
						m := e.Models[dec.ModelID]
						if router.Violates(m, sc, sensitive) {
							violations++
						}
						if m.ID != w.PremiumModel {
							reroutes++
						}
						resp, _, err := e.callModel(ctx, m, p)
						if err != nil {
							return nil, err
						}
						served++
						cost += e.cost(m, resp.Usage)
						q, err := e.acceptability(ctx, m, p, resp.Text)
						if err != nil {
							return nil, err
						}
						qs = append(qs, (q-1)/4)
						e.records = append(e.records, callRecord{
							Strategy: strat.Name(), Workload: w.Name, Team: w.Team, Namespace: w.Namespace,
							PromptID: p.ID, ModelID: m.ID, Provider: m.Provider, Scenario: sc.Name,
							Real: m.Real, Modeled: !m.Real, InTok: resp.Usage.InputTokens, OutTok: resp.Usage.OutputTokens,
							CostEUR: e.cost(m, resp.Usage), LatencyMS: resp.LatencyMS, Quality1to5: q,
							SovViolation: router.Violates(m, sc, sensitive), Reroute: m.ID != w.PremiumModel,
						})
					}
				}
				rows = append(rows, []string{sc.Name, strat.Name(), f(cost), itoa(served), itoa(blocked), itoa(violations), itoa(reroutes), f(mean(qs))})
				return map[string]any{"served": served, "blocked": blocked, "violations": violations, "reroutes": reroutes, "costEUR": round(cost), "meanQualityNorm": round(mean(qs))}, nil
			})
			if err != nil {
				return err
			}
		}
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq4_sovereignty.csv"),
		[]string{"scenario", "strategy", "total_cost_eur", "served", "blocked", "violations", "reroutes", "mean_quality_norm"}, rows)
}

// RunBudget executes RQ5: graceful degradation vs hard block vs alert-only on a
// tight budget (W4), measuring availability, cost, quality and overrun.
func (e *Engine) RunBudget(ctx context.Context) error {
	var w workload.Workload
	for _, x := range e.Workloads {
		if x.Namespace == "agent" {
			w = x
		}
	}
	if w.Name == "" {
		w = e.Workloads[len(e.Workloads)-1]
	}
	// Premium cost of serving the whole workload once (sets a meaningful budget).
	premiumAll := 0.0
	pm := e.Models[w.PremiumModel]
	for _, p := range w.Prompts {
		resp, _, err := e.callModel(ctx, pm, p)
		if err != nil {
			return err
		}
		premiumAll += e.cost(pm, resp.Usage)
	}
	budget := premiumAll * 0.4 // tight: ~40% of premium-serving everything
	global := router.SovScenario{Name: "global", ExternalProvidersAllowed: true}

	policies := []struct {
		name  string
		strat router.Strategy
	}{
		{"alert-only", router.PremiumStatic{}},
		{"hard-block", router.BudgetHardBlock{}},
		{"ours-graceful", router.Ours{W: router.DefaultWeights()}},
	}
	rows := [][]string{}
	for _, pol := range policies {
		pol := pol
		err := e.J.Run("RQ5-budget", pol.name, func() (map[string]any, error) {
			used, served, blocked := 0.0, 0, 0
			var qs []float64
			for _, p := range w.Prompts {
				rctx := router.RequestContext{
					Team: w.Team, Namespace: w.Namespace, Sensitive: p.Sensitive || w.DefaultSensitive,
					AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: w.MinQuality,
					BudgetTotalEUR: budget, BudgetUsedEUR: used, Scenario: global,
					EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
				}
				dec := pol.strat.Choose(rctx, e.Models)
				if dec.Blocked {
					blocked++
					continue
				}
				m := e.Models[dec.ModelID]
				resp, _, err := e.callModel(ctx, m, p)
				if err != nil {
					return nil, err
				}
				served++
				used += e.cost(m, resp.Usage)
				q, err := e.acceptability(ctx, m, p, resp.Text)
				if err != nil {
					return nil, err
				}
				qs = append(qs, (q-1)/4)
			}
			overrun := 0.0
			if used > budget {
				overrun = (used - budget) / budget * 100
			}
			availability := float64(served) / float64(len(w.Prompts)) * 100
			rows = append(rows, []string{pol.name, f(budget), f(used), itoa(served), itoa(blocked), f2(availability), f2(overrun), f(mean(qs))})
			return map[string]any{"served": served, "blocked": blocked, "availabilityPct": round(availability), "usedEUR": round(used), "budgetEUR": round(budget), "overrunPct": round(overrun), "meanQualityNorm": round(mean(qs))}, nil
		})
		if err != nil {
			return err
		}
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq5_budget.csv"),
		[]string{"policy", "budget_eur", "used_eur", "served", "blocked", "availability_pct", "budget_overrun_pct", "mean_quality_norm"}, rows)
}

// RunBreakEven executes RQ6: managed-API vs self-hosted break-even across daily
// token volumes (MODELED via the operator's breakevenengine).
func (e *Engine) RunBreakEven(ctx context.Context) error {
	premium := e.Models[e.Workloads[0].PremiumModel]
	// Blended price assuming a 70/30 input/output token split.
	blendedPerM := 0.7*premium.InPerMillion + 0.3*premium.OutPerMillion
	const gpuMonthly, opsMonthly, migration = 1800.0, 700.0, 5000.0

	volumes := []float64{} // tokens per day
	for v := 1_000_000.0; v <= 200_000_000.0; v *= 1.5 {
		volumes = append(volumes, v)
	}
	rows := [][]string{}
	breakEvenVol := 0.0
	err := e.J.Run("RQ6-breakeven", "managed-vs-selfhosted-curve", func() (map[string]any, error) {
		for _, v := range volumes {
			managedMonthly := v * 30 / 1e6 * blendedPerM
			res := breakevenengine.Analyze(breakevenengine.Inputs{
				ManagedTokenCostMonthly: managedMonthly, GpuMonthly: gpuMonthly, OpsMonthly: opsMonthly, MigrationCost: migration,
			}, breakevenengine.DefaultPaybackThresholdMonths)
			if breakEvenVol == 0 && res.MonthlySavings > 0 {
				breakEvenVol = v
			}
			rows = append(rows, []string{f2(v), f2(managedMonthly), f2(res.SelfHostedMonthly), f2(res.MonthlySavings), fpayback(res), res.Recommendation})
		}
		return map[string]any{"breakEvenTokensPerDay": breakEvenVol, "points": len(rows), "modeled": true}, nil
	})
	if err != nil {
		return err
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq6_breakeven.csv"),
		[]string{"tokens_per_day", "managed_monthly_eur", "selfhosted_monthly_eur", "monthly_savings_eur", "payback_months", "recommendation"}, rows)
}

func fpayback(r breakevenengine.Result) string {
	if !r.HasPayback {
		return "n/a"
	}
	return f2(r.PaybackMonths)
}

var lastNumRe = regexp.MustCompile(`-?\d[\d,]*\.?\d*`)

// numericCorrect checks whether the answer's last number equals the reference
// (objective exact-match for numeric benchmarks like GSM8K; no LLM judge).
func numericCorrect(answer, ref string) bool {
	nums := lastNumRe.FindAllString(strings.ReplaceAll(answer, ",", ""), -1)
	if len(nums) == 0 {
		return false
	}
	g, err1 := strconv.ParseFloat(strings.TrimRight(nums[len(nums)-1], "."), 64)
	r, err2 := strconv.ParseFloat(strings.ReplaceAll(ref, ",", ""), 64)
	if err1 != nil || err2 != nil {
		return false
	}
	return math.Abs(g-r) < 1e-6
}

// RunBenchmark routes each strategy over a public benchmark (ground-truth answers)
// and scores correctness by objective exact-match — no LLM judge, so quality is
// measured without judge bias. Writes results/rq_benchmark.csv (accuracy, cost).
func (e *Engine) RunBenchmark(ctx context.Context, bws []workload.Workload) error {
	global := router.SovScenario{Name: "global", ExternalProvidersAllowed: true}
	type agg struct {
		correct, total int
		cost           float64
		lat            []float64
	}
	rows := [][]string{}
	for _, strat := range router.All() {
		strat := strat
		err := e.J.Run("BENCHMARK", strat.Name(), func() (map[string]any, error) {
			a := &agg{}
			for wi := range bws {
				w := bws[wi]
				for _, p := range w.Prompts {
					rctx := router.RequestContext{
						Team: w.Team, Namespace: w.Namespace, Sensitive: p.Sensitive || w.DefaultSensitive,
						AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: w.MinQuality,
						BudgetTotalEUR: w.MonthlyBudgetEUR, Scenario: global,
						EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
					}
					dec := strat.Choose(rctx, e.Models)
					if dec.Blocked {
						a.total++
						continue
					}
					m := e.Models[dec.ModelID]
					resp, _, err := e.callModel(ctx, m, p)
					if err != nil {
						return nil, err
					}
					a.total++
					a.cost += e.cost(m, resp.Usage)
					a.lat = append(a.lat, float64(resp.LatencyMS))
					if numericCorrect(resp.Text, p.Reference) {
						a.correct++
					}
				}
			}
			acc := 0.0
			if a.total > 0 {
				acc = float64(a.correct) / float64(a.total) * 100
			}
			rows = append(rows, []string{strat.Name(), itoa(a.correct), itoa(a.total), f2(acc), f(a.cost), f2(percentile(a.lat, 95))})
			return map[string]any{"accuracyPct": round(acc), "correct": a.correct, "total": a.total, "costEUR": round(a.cost)}, nil
		})
		if err != nil {
			return err
		}
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq_benchmark.csv"),
		[]string{"strategy", "correct", "total", "accuracy_pct", "total_cost_eur", "latency_p95_ms"}, rows)
}

// RunStatsMatrix runs the full strategy x workload x prompt matrix `reps` times
// at temperature e.Temp with caches bypassed, so each repetition is an
// independent sample. It records cost/latency/quality per call (acceptability
// only; no pairwise) and writes results/calls_stats.csv with a `rep` column for
// statistical analysis (CIs, significance, effect sizes via scripts/stats.py).
func (e *Engine) RunStatsMatrix(ctx context.Context, reps int) error {
	global := router.SovScenario{Name: "global", ExternalProvidersAllowed: true}
	concurrency := 6
	var all []callRecord

	// task is a routing decision awaiting its (concurrent) API call.
	type task struct {
		w   workload.Workload
		p   workload.Prompt
		s   string
		m   catalog.Model
		rep int
	}

	for rep := 1; rep <= reps; rep++ {
		rep := rep
		err := e.J.Run("STATS-matrix", fmt.Sprintf("rep-%02d", rep), func() (map[string]any, error) {
			// Phase 1 (sequential, CPU-only, safe for stateful round-robin): decide routes.
			var tasks []task
			for _, strat := range router.All() {
				sname := strat.Name()
				for wi := range e.Workloads {
					w := e.Workloads[wi]
					for _, p := range w.Prompts {
						rctx := router.RequestContext{
							Team: w.Team, Namespace: w.Namespace, Sensitive: p.Sensitive || w.DefaultSensitive,
							AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: w.MinQuality,
							BudgetTotalEUR: w.MonthlyBudgetEUR, BudgetUsedEUR: 0, Scenario: global,
							EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
						}
						dec := strat.Choose(rctx, e.Models)
						if dec.Blocked {
							continue
						}
						tasks = append(tasks, task{w: w, p: p, s: sname, m: e.Models[dec.ModelID], rep: rep})
					}
				}
			}
			// Phase 2 (concurrent): real API answer + judge per task.
			recs := make([]callRecord, len(tasks))
			errs := make([]error, len(tasks))
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			for i := range tasks {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int) {
					defer wg.Done()
					defer func() { <-sem }()
					t := tasks[i]
					resp, _, err := e.callModel(ctx, t.m, t.p)
					if err != nil {
						errs[i] = err
						return
					}
					q, err := e.acceptability(ctx, t.m, t.p, resp.Text)
					if err != nil {
						errs[i] = err
						return
					}
					recs[i] = callRecord{
						Strategy: t.s, Workload: t.w.Name, Team: t.w.Team, Namespace: t.w.Namespace,
						PromptID: t.p.ID, ModelID: t.m.ID, Provider: t.m.Provider, Scenario: "global",
						Real: t.m.Real, Modeled: !t.m.Real, InTok: resp.Usage.InputTokens, OutTok: resp.Usage.OutputTokens,
						CostEUR: e.cost(t.m, resp.Usage), LatencyMS: resp.LatencyMS, Quality1to5: q,
						Reroute: t.m.ID != t.w.PremiumModel, Rep: t.rep,
					}
				}(i)
			}
			wg.Wait()
			for _, err := range errs {
				if err != nil {
					return nil, err
				}
			}
			all = append(all, recs...)
			return map[string]any{"rep": rep, "calls": len(recs)}, nil
		})
		if err != nil {
			return err
		}
	}
	rows := make([][]string, 0, len(all))
	for _, r := range all {
		rows = append(rows, []string{
			itoa(r.Rep), r.Strategy, r.Workload, r.Team, r.Namespace, r.PromptID, r.ModelID, r.Provider,
			b2s(r.Real), itoa(r.InTok), itoa(r.OutTok), f(r.CostEUR), itoa64(r.LatencyMS), f2(r.Quality1to5), b2s(r.Reroute),
		})
	}
	return writeCSV(filepath.Join(e.ResultsDir, "calls_stats.csv"),
		[]string{"rep", "strategy", "workload", "team", "namespace", "prompt", "model", "provider",
			"real", "in_tokens", "out_tokens", "cost_eur", "latency_ms", "quality_1to5", "reroute"}, rows)
}

// RunAblation isolates the contribution of the scoring terms (Fig 8). Each
// variant routes over the full matrix (unrestricted scenario) reusing cached
// answers; cost and quality are reported per variant.
func (e *Engine) RunAblation(ctx context.Context) error {
	global := router.SovScenario{Name: "global", ExternalProvidersAllowed: true}
	w0 := router.DefaultWeights()
	variants := []struct {
		name      string
		w         router.Weights
		dropQGate bool
	}{
		{"full-system", w0, false},
		{"no-cost-term", router.Weights{Alpha: 0, Beta: w0.Beta, Gamma: w0.Gamma, Delta: w0.Delta, Epsilon: w0.Epsilon}, false},
		{"no-quality-term", router.Weights{Alpha: w0.Alpha, Beta: 0, Gamma: w0.Gamma, Delta: w0.Delta, Epsilon: w0.Epsilon}, true},
		{"no-latency-term", router.Weights{Alpha: w0.Alpha, Beta: w0.Beta, Gamma: 0, Delta: w0.Delta, Epsilon: w0.Epsilon}, false},
	}
	rows := [][]string{}
	premiumTotal := 0.0
	for _, v := range variants {
		v := v
		err := e.J.Run("RQ-ablation", v.name, func() (map[string]any, error) {
			strat := router.Ours{W: v.w}
			cost := 0.0
			var qs []float64
			for wi := range e.Workloads {
				w := e.Workloads[wi]
				for _, p := range w.Prompts {
					minQ := w.MinQuality
					if v.dropQGate {
						minQ = 0
					}
					rctx := router.RequestContext{
						Team: w.Team, Namespace: w.Namespace, Sensitive: p.Sensitive || w.DefaultSensitive,
						AllowedModels: w.AllowedModels, PremiumModel: w.PremiumModel, MinQuality: minQ,
						BudgetTotalEUR: w.MonthlyBudgetEUR, Scenario: global,
						EstInputTokens: estTokens(p.System + p.Text), EstOutputTokens: 200,
					}
					dec := strat.Choose(rctx, e.Models)
					m := e.Models[dec.ModelID]
					resp, _, err := e.callModel(ctx, m, p)
					if err != nil {
						return nil, err
					}
					cost += e.cost(m, resp.Usage)
					q, err := e.acceptability(ctx, m, p, resp.Text)
					if err != nil {
						return nil, err
					}
					qs = append(qs, (q-1)/4)
				}
			}
			if v.name == "no-cost-term" {
				premiumTotal = cost // upper-bound proxy
			}
			savings := 0.0
			if premiumTotal > 0 {
				savings = (premiumTotal - cost) / premiumTotal * 100
			}
			rows = append(rows, []string{v.name, f(cost), f2(savings), f(mean(qs))})
			return map[string]any{"costEUR": round(cost), "meanQualityNorm": round(mean(qs))}, nil
		})
		if err != nil {
			return err
		}
	}
	return writeCSV(filepath.Join(e.ResultsDir, "rq8_ablation.csv"),
		[]string{"variant", "total_cost_eur", "savings_vs_nocost_pct", "mean_quality_norm"}, rows)
}

// WriteCallsCSV dumps the full per-call log for downstream analysis.
func (e *Engine) WriteCallsCSV() error {
	rows := make([][]string, 0, len(e.records))
	for _, r := range e.records {
		rows = append(rows, []string{
			r.Strategy, r.Scenario, r.Workload, r.Team, r.Namespace, r.PromptID, r.ModelID, r.Provider,
			b2s(r.Real), b2s(r.Modeled), b2s(r.Blocked), itoa(r.InTok), itoa(r.OutTok), f(r.CostEUR),
			itoa64(r.LatencyMS), f2(r.Quality1to5), r.Win, b2s(r.SovViolation), b2s(r.Reroute),
		})
	}
	return writeCSV(filepath.Join(e.ResultsDir, "calls.csv"),
		[]string{"strategy", "scenario", "workload", "team", "namespace", "prompt", "model", "provider",
			"real", "modeled", "blocked", "in_tokens", "out_tokens", "cost_eur", "latency_ms", "quality_1to5", "win_vs_premium", "sov_violation", "reroute"}, rows)
}
