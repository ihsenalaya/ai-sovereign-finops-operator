# Economic-Aware and Sovereignty-Constrained Routing for Enterprise LLM Gateways

**Draft v0.1 — single-provider (OpenAI) evaluation.** This is a first complete write-up built from
real measurements (`../results/`). Numbers are reproduced from the committed run; figures are in
`../figures/`. Placeholders `[CIT]` mark citations to add. Items explicitly labeled *modeled* are not
real measurements (see §8).

---

## Abstract

Enterprises are adopting large language models (LLMs) faster than they can govern them: they cannot
say which model truly costs the most, which team consumes what, when to route to a cheaper model,
what a data-sovereignty constraint costs, or when self-hosting beats a managed API. We present an
**economic-aware, sovereignty-constrained, budget-aware routing control plane** for enterprise LLM
gateways. We formalize model selection as a constrained scoring problem that rejects any model
violating declarative sovereignty rules and, among the rest, minimizes a weighted combination of
cost, expected quality loss, latency, and budget pressure. We realize the approach as a Kubernetes
operator whose FinOps and sovereignty engines drive routing, and we evaluate it on a reproducible
testbed with **real LLM calls** across four enterprise workloads and six routing strategies. Against
an always-premium policy, our approach reduces operational cost by **64.6%** with **no measurable
quality loss** (LLM-as-judge), adds only **tens of microseconds** of routing overhead, enforces
sovereignty scenarios with **zero violations** (vs 40 for a sovereignty-blind baseline), sustains
**100% availability with 0% budget overrun** under budget pressure where a hard-block policy drops to
60% availability, and predicts a managed-vs-self-hosted break-even from runtime telemetry. All code,
datasets, cached responses, and analysis are released for reproducibility.

## 1. Introduction

LLMs are now embedded in HR assistants, document RAG, developer tooling, and analytical agents. Each
call has a price that depends on the model, the provider, and the token volume; each call may also
send data across jurisdictions. Three problems recur in regulated enterprises:

1. **Cost opacity.** Teams cannot attribute spend per application/team or decide when a cheaper model
   would suffice.
2. **Sovereignty/compliance.** Data may not leave a permitted zone, and sensitive prompts may not go
   to external providers; today this is enforced ad hoc, if at all.
3. **Managed vs self-hosted arbitrage.** It is unclear at what volume self-hosting on GPU becomes
   cheaper than a managed API.

Existing gateways and FinOps tools mostly *measure and report*. Our thesis is that a control plane at
the **gateway** can *act*: route each request to the cheapest model that meets a minimum quality bar
and all hard sovereignty constraints, degrade gracefully under budget pressure instead of blocking,
and surface the self-hosting break-even from observed usage — without significant quality loss.

**Contributions.**
- A constrained scoring formulation for LLM routing unifying cost, quality, latency, sovereignty, and
  budget (§3).
- A Kubernetes/Envoy control-plane realization: the *AI Sovereign FinOps Operator* with Cost, Budget,
  Sovereignty, Break-even, and Report engines exposed as CRDs (§4).
- A reproducible evaluation methodology and testbed driving the operator's actual engines with real
  LLM telemetry (§5).
- Empirical evidence across six research questions (§6), including cost, quality, latency,
  sovereignty, budget degradation, and break-even.

## 2. Background and Related Work

**LLM gateways.** Envoy AI Gateway and LiteLLM provide a unified data path to multiple providers
`[CIT]`. They expose telemetry but leave routing policy to the operator/user.

**Model routing and cascades.** Prior work routes between models by difficulty or confidence, or
cascades from cheap to expensive `[CIT]`. We add *hard* sovereignty constraints and *budget-aware*
degradation as first-class signals, and we attribute and bound cost at the team/namespace level.

**FinOps for AI.** Cloud FinOps practice emphasizes attribution and budgets `[CIT]`; we operationalize
it for LLM traffic with per-token pricing and budget policies enforced at routing time.

**Serving economics.** Self-hosting via vLLM/TGI trades fixed GPU/ops cost for marginal token cost
`[CIT]`. We predict the break-even point from runtime telemetry.

## 3. Problem Formulation and Algorithm

A request $r$ carries metadata (team, namespace, sensitivity) and an estimated token footprint. Given
a catalog of models $M$ (each with provider, zone, managed flag, per-token price, quality prior,
latency prior), a sovereignty policy $P$, and budget state $(used, total)$:

**Hard constraint.** Reject any $m$ that violates $P$: its zone is forbidden; or an allow-list is set
and $m$'s zone is outside it (EU covers EU member states); or $r$ is sensitive, $m$ is a managed
external provider, and external providers are disallowed for sensitive data. Let $C$ be the surviving
candidates. If $C=\varnothing$, the request is not served (a measured outcome, not a crash).

**Soft objective.** Among $C$ meeting a minimum quality $q_{\min}$ (relaxed only when the budget is
exhausted, enabling graceful degradation), select

$$\arg\min_{m\in C}\; \alpha\cdot \widehat{c}(m)\,(1+\varepsilon\,\rho) + \beta\,\ell_q(m) + \gamma\,\ell_\lambda(m) + \delta\,\sigma(m)$$

where $\widehat{c}(m)=\text{cost}(m)/\text{cost}(\text{premium})$ is normalized cost, $\rho=used/total$
is budget pressure, $\ell_q(m)=\max(0,(q_{\text{premium}}-q_m)/q_{\text{premium}})$ is expected
quality loss, $\ell_\lambda$ is normalized latency, and $\sigma$ a residual sovereignty risk. Budget
pressure multiplies the cost term, so as the budget fills the router shifts to cheaper models.
Defaults: $\alpha{=}1.0,\beta{=}1.5,\gamma{=}0.3,\delta{=}0.5,\varepsilon{=}1.0$.

Pseudocode and the exact normalization are in `methodology.md`; the implementation is
`experimentation/internal/router` (strategy `B6-ours`), reusing the operator's sovereignty
normalization.

## 4. System Design

Requests flow client → **Envoy AI Gateway** → provider (managed API or self-hosted). The **AI
Sovereign FinOps Operator** is the control plane: CRDs declare gateways, providers, models, budgets,
and sovereignty policies; pure engines compute cost (per model/team/namespace), budget phase,
sovereignty findings, and break-even; a report engine emits Markdown/JSON and Prometheus
`ai_finops_*` metrics. The architecture (Figure 1) keeps the data path in Envoy and the decision
logic in the operator. In this paper the routing decision is evaluated at the control-plane level;
data-path enforcement inside Envoy is the live-integration step (§9).

## 5. Methodology

**Testbed.** A Go harness drives the operator's *actual* engines and issues **real** LLM calls. The
provider in this phase is **OpenAI** (the harness is provider-agnostic; a second provider is the next
step). Models: gpt-4o (premium), gpt-4o-mini (medium), gpt-4.1-nano (cheap), and a *modeled* EU
self-hosted model (no GPU locally; cost computed, response stubbed, rows tagged *modeled*).

**Workloads (W1–W4).** HR chatbot (short, sensitive), RAG docs (long context, quality-sensitive), Dev
assistant (high quality), Analytical agent (high volume). Each declares team, namespace, sensitivity,
monthly budget, premium model, allowed models, and $q_{\min}$ (`datasets/`).

**Baselines.** B1 premium-static, B2 round-robin, B3 least-cost, B4 static namespace policy, B5 budget
hard-block, B6 ours.

**Metrics.** Cost (total, per request, per token, per team via the operator cost engine, % savings);
quality (LLM-as-judge acceptability 1–5 → normalized 0–1, acceptable-rate ≥3, pairwise win-rate vs the
premium answer); latency (p50/p95/p99, mean, routing-decision µs); sovereignty (violations, reroutes,
blocked); budget (availability, overrun); break-even (managed vs self-hosted monthly, payback).

**Reproducibility.** Temperature 0; all responses and judge scores cached (`results/cache.json`) so
re-runs are identical and free; every test journaled with status+duration (`results/TEST_STATUS.md`,
42 PASS / 0 FAIL). Latency 95% CIs via bootstrap. The current pass is deterministic; the protocol
supports ≥30 repetitions (§8).

## 6. Results

### RQ1 — Cost (Figure 2, Table 1)
Relative to premium-static, **B6-ours cuts total cost by 64.6%** (0.0138 vs 0.0389 EUR over the 40-
prompt matrix; cost/request 0.000345 vs 0.000973). Naïve least-cost (B3) saves 98.2% but at a quality
cost (RQ2); round-robin 73.9%; static policy 46.0%.

### RQ2 — Quality preservation (Figure 3, Table 2)
**B6-ours preserves quality exactly** at the premium level (normalized 0.900, acceptable-rate 97.5%),
with a pairwise win-rate vs premium of 41.7% (statistical parity). Least-cost drops to 0.858 and
static-policy to 0.869. Thus the 64.6% saving comes with **0.00% quality loss**, well within the 3–5%
target.

### RQ3 — Latency and overhead (Figure 4, Table 3)
The **routing decision adds tens of microseconds** (B6 ≈ 26.5 µs/request) — negligible vs network
latency. End-to-end latency tracks the chosen model: B6's p95 (3475 ms) is higher than premium's
because it routes quality-critical workloads to the premium model while saving elsewhere; least-cost
has the lowest latency (p95 1476 ms).

### RQ4 — Sovereignty (Figure 5, Table 4)
A sovereignty-blind baseline (B1) commits **40 violations** under eu-only, france-only, and
self-hosted-only, and 10 under no-external-sensitive. **B6-ours commits zero violations in every
scenario.** It reveals the *cost of sovereignty*: under no-external-sensitive it still serves all 40
requests (quality 0.879) by rerouting; under the strict france-only/self-hosted-only scenarios only
the workloads that permit the self-hosted model are served (20/40), the rest are refused rather than
violated — quantifying the availability price of strict residency (self-hosted quality is *modeled*).

### RQ5 — Budget-aware degradation (Figure 6, Table 5)
Under a budget tight enough to be exceeded by always-premium, **ours-graceful keeps 100% availability
with 0% budget overrun** (quality 0.85), whereas **hard-block** falls to **60% availability** (4/10
requests blocked) and **alert-only overshoots the budget by 150%**. Graceful degradation dominates on
availability *and* spend.

### RQ6 — Managed vs self-hosted break-even (Figure 7) — *modeled*
Using real per-token prices and modeled GPU+ops cost (2500 EUR/month, 5000 EUR migration), managed
cost grows with volume while self-hosted is flat. The crossover is at **≈25.6M tokens/day** (payback
≈4.3 months); above it the engine recommends *self-host*. This is a modeled prediction to be
validated against real GPU economics (§9).

### Ablation (Figure 8, Table 7)
Removing the **cost term** reverts spend to premium (0.0389 EUR) at unchanged quality — the cost term
is what produces the saving. Removing the **quality term** drives spend to the floor (0.0012 EUR) but
lowers quality to 0.863 — the quality term is what protects it. The latency term has negligible effect
here. The full system attains the cost saving *and* the quality floor.

## 7. Discussion

Economic-aware routing is most valuable when a workload mixes easy and hard requests: the router sends
cheap requests to cheap models and reserves the premium model for quality-critical ones, capturing
most of the savings at no quality cost. Strict sovereignty has a real availability/quality price that
the system makes explicit and auditable rather than hidden. For budget control, graceful degradation
strictly dominates hard-blocking on availability and overrun. Self-hosting pays off only at high,
sustained volume.

## 8. Threats to Validity

Summarized here; full version in `threats_to_validity.md`. **Internal:** API/latency variability
(mitigated by temperature 0, caching, bootstrap CIs); LLM-as-judge bias (absolute + pairwise, fixed
judge). **External:** single provider and **modeled** self-hosting in this phase; four synthetic
workloads; volatile cloud prices (isolated as configuration). **Construct:** acceptability/win-rate
are quality proxies; declarative sovereignty simplifies legal reality — the system targets *audit
preparation*, not legal compliance. **Conclusion:** one deterministic pass (temperature 0); the
protocol supports ≥30 repetitions with significance tests and effect sizes for the camera-ready.

## 9. Conclusion and Future Work

A gateway-level, economic-aware, sovereignty-constrained, budget-aware control plane can cut LLM cost
substantially without quality loss while enforcing sovereignty and preserving availability. Next:
(1) a **second provider** for cross-provider routing; (2) **real GPU self-hosting** (vLLM on Azure)
to validate the break-even; (3) **data-path enforcement in Envoy** and live operator metrics;
(4) **≥30-repetition** runs with full statistics.

## Artifact / Reproducibility

Operator + `experimentation/` (datasets, harness, scripts, results, figures, cached responses).
Reproduce with `scripts/run_experiment.sh` then `python3 scripts/analyze_results.py`. Every test is
journaled; no test is skipped.
