# Economic-Aware and Sovereignty-Constrained Routing for Enterprise LLM Gateways

**Draft v0.4 — two-provider (OpenAI US + Mistral EU) evaluation with a 30-repetition statistical run.**
A complete write-up built from real measurements (`../results/`, `../results-stats/`). Numbers are reproduced from the committed run; figures are in
`../figures/`; citations use the verified `references.bib` (keys in brackets). Items explicitly
labeled *modeled* are not real measurements (see §8). This is preliminary work toward a Q1 submission;
the gap to camera-ready is tracked in `ROADMAP_Q1.md`.

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
testbed with **real LLM calls across two providers**. **Scope of current evidence (preliminary):**
*two providers* — OpenAI (US, managed) and **Mistral (EU)** served via **Azure AI Foundry**
(Mistral-Large-3, *DataZoneStandard* EU data zone) — over *four synthetic enterprise workloads*
(40 prompts), *one deterministic pass* (temperature 0), with *modeled* self-hosting economics
(no GPU); we report effects *within this testbed*, without significance testing yet. Within this
scope: relative to an always-premium policy, our approach reduces measured cost by **70.8%** over the
40-prompt matrix while keeping LLM-as-judge quality **comparable to (slightly above) premium**
(normalized 0.9125 vs 0.900; pairwise win-rate vs premium 50.0%); routing adds **tens of microseconds**
of decision overhead; it enforces declarative sovereignty scenarios with **zero violations** versus 40
for a sovereignty-blind baseline, and under an **EU-only** policy it keeps **100% availability by
routing to the real EU provider (Mistral)** where the blind baseline commits 40 violations; and it
sustains **100% request availability with 0% budget overrun** under a tight budget where a hard-block
policy drops to 60%. The managed-vs-self-hosted break-even (RQ6) remains a **modeled** prediction to be
validated on real GPUs. A **30-repetition** run (temperature 0.7) confirms the cost reduction is highly
significant (p ≈ 3×10⁻¹¹, Cohen d ≈ −43) with quality at least comparable, at a significant but bounded
latency increase (§6). All code, datasets, cached responses, and analysis are released; we present this
as preliminary evidence and a reproducible framework, not a universal claim.

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

References are in `references.bib` (recent, verified); citation keys are shown in brackets.

**Model routing and cascades.** A growing line of work trades quality for cost by selecting among
models per request. FrugalGPT [chen2023frugalgpt] cascades from cheap to expensive models using a
learned scorer; Hybrid LLM [ding2024hybridllm] routes by predicted query difficulty with a tunable
quality target; RouteLLM [ong2024routellm] learns routers from preference data; AutoMix
[aggarwal2024automix] uses few-shot self-verification and a POMDP router. These works optimize a
*cost–quality* trade-off, largely via per-request difficulty/confidence prediction for a single
tenant. Our work is **orthogonal and complementary, not a replacement**: they decide *which model is
good enough*; we add the **governance layer** around that decision — hard data-sovereignty constraints,
per-team/namespace budgets, budget-aware graceful degradation, attribution, and hosting economics — at
the gateway/control-plane level. Crucially, our routing score is intentionally simple (priors, not a
learned difficulty predictor); **combining a learned difficulty router (e.g., Hybrid LLM/RouteLLM) with
our sovereignty/budget constraints is explicit future work** (§9), and such learned routers are planned
as *baselines* we compare against, not methods we claim to surpass here.

**LLM serving systems.** Orca [yu2022orca], vLLM/PagedAttention [kwon2023pagedattention], SGLang
[zheng2024sglang], and DistServe [zhong2024distserve] optimize *throughput/latency* of a single
self-hosted engine; surveys [zhou2024surveyinference; li2025tamingtitans] organize this space. They
are complementary: they define the *cost* of the self-hosted option whose break-even our system
predicts (RQ6), but they do not address cross-provider routing, attribution, or sovereignty.

**LLM gateways.** Envoy AI Gateway [envoyaigateway] and LiteLLM [litellm] provide a unified data path
to multiple providers and expose usage telemetry, but leave *policy* (which model, under which
constraints, within which budget) to the operator. We contribute that policy layer as a Kubernetes
control plane and validate it.

**Evaluation.** We follow the LLM-as-a-judge methodology of Zheng et al. [zheng2023judge] (MT-Bench /
Chatbot Arena), which shows strong judges approximate human preference at ~80% agreement, and we adopt
both absolute and pairwise protocols while acknowledging its documented biases (§8).

**Regulation.** Data-residency and sensitive-data handling are driven by the GDPR [gdpr2016] and the
EU AI Act [euaiact2024]; our system targets *audit preparation* for these regimes, not legal
attestation.

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

**Testbed.** A Go harness drives the operator's *actual* engines and issues **real** LLM calls across
**two providers**: (1) **OpenAI (US, managed)** — gpt-4o (premium), gpt-4o-mini (medium), gpt-4.1-nano
(cheap); (2) **Mistral (EU)** — `mistral-large` served via **Azure AI Foundry** as **Mistral-Large-3**
in **DataZoneStandard** (EU data zone), a real EU-hosted provider. A *modeled* EU self-hosted model
(no GPU; cost computed, response stubbed, rows tagged *modeled*) is retained only for the RQ6
break-even prediction. The harness is provider-agnostic (per-provider OpenAI-compatible clients), so
adding further providers is additive.

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
Relative to premium-static, **B6-ours cuts total cost by 70.8%** (0.01135 vs 0.03893 EUR over the
40-prompt matrix; cost/request 0.000284 vs 0.000973). With the second provider available, Ours routes
much traffic to the cheaper Mistral EU and OpenAI mid/cheap tiers. Naïve least-cost (B3) saves 98.2%
but at a quality cost (RQ2); round-robin 62.3%; static policy 46.0%.

### RQ2 — Quality (Figure 3, Table 2)
Within this two-provider, single-pass scope, B6-ours's mean normalized quality is **comparable to —
marginally above — premium** (0.9125 vs 0.900; acceptable-rate 97.5%), with a pairwise win-rate vs the
premium reference of **50.0%** (statistical parity). The cross-provider mix helps: Mistral-Large-3
sometimes matches or beats gpt-4o on the judge. Least-cost drops to 0.858 and static-policy to 0.869.
Two caveats remain: (i) the **premium model is also the routing reference and the judge anchor**, which
can bias pairwise results; (ii) we have **not yet run significance tests** (one deterministic pass).
We therefore claim only that *quality remained comparable within the evaluated scope*, and defer a
statistically supported statement to the multi-repetition, multi-judge protocol
(§8, `QUALITY_EVALUATION_PROTOCOL.md`).

### RQ3 — Latency and overhead (Figure 4, Table 3)
The **routing decision adds tens of microseconds** (B6 ≈ 26.5 µs/request) — negligible vs network
latency. End-to-end latency tracks the chosen model and provider-side variance. Notably, **B6's p95
(3475 ms) exceeds premium-static's (2494 ms)**, which we do not hide. Two factors plausibly explain it,
and we flag it as not fully resolved: (i) B6 still sends quality-critical workloads to the premium
model *plus* mixes in other models, so its latency distribution has a heavier tail than the
single-model premium policy; (ii) with **one pass**, tail percentiles are sensitive to a few slow API
responses (measurement variance), and our 95% CIs are over intra-pass per-call samples, not across
repetitions. **TODO (§8):** with ≥30 repetitions, report tail latency with cross-run CIs and
disentangle routing-distribution effects from API jitter. Least-cost has the lowest latency
(p95 1476 ms), consistent with always picking small models.

### RQ4 — Sovereignty (Figure 5, Table 4)
A sovereignty-blind baseline (B1) commits **40 violations** under eu-only, france-only and
self-hosted-only, and 10 under no-external-sensitive. **B6-ours commits zero violations in every
scenario.** Crucially, with a **real EU provider** (Mistral-Large-3, Azure AI Foundry DataZone EU),
under the **EU-only** policy Ours **keeps 100% availability (40/40 served) with zero violations** by
rerouting to Mistral EU (quality 0.866) — where the blind baseline would commit 40 violations. Under
**no-external-sensitive** it also serves all 40 (quality 0.891). The *cost of sovereignty* appears in
the stricter **france-only/self-hosted-only** scenarios: only the modeled FR self-hosted model
qualifies, so 20/40 are served and the rest refused rather than violated — quantifying the
availability price of strict, France-only residency (that fallback's quality is *modeled*; a real
French-hosted endpoint would remove the modeling).

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

### Statistical significance (N=30 repetitions)
To move beyond the single deterministic pass, we re-ran the full matrix **30 times** at temperature
0.7 with caches bypassed (independent samples; judge = gpt-4o-mini to bound cost), and tested each
strategy against premium-static on the per-repetition distributions (Mann-Whitney U, Cliff's δ,
Cohen's d, 95% CIs; `scripts/stats.py`, `scripts/analyze_stats.py`, `results-stats/stats_summary.md`).
- **Cost.** Ours 0.0110 EUR/rep [0.0109, 0.0110] vs premium 0.0385 [0.0382, 0.0389] — a **71.4%
  reduction**, p ≈ 3×10⁻¹¹, Cliff δ = −1.00, Cohen d = −43 (non-overlapping CIs): highly significant
  with a maximal effect size.
- **Quality.** Ours mean normalized quality 0.9587 [0.9564, 0.9611] vs premium 0.9362 — under this
  judge Ours is **significantly higher** (p ≈ 6×10⁻¹¹, δ = +0.97, d = +3.4), while naïve least-cost is
  significantly lower (0.869, d = −13). **Judge dependence:** the single-pass gpt-4o judge showed
  *parity* (win-rate 50.0%); the *sign* of the small quality gap depends on the judge — exactly why a
  multi-judge + human protocol is planned (§8). We thus claim quality is *at least comparable*.
- **Latency.** Ours is **significantly higher** (1506 ms/rep [1459, 1553] vs 998, p ≈ 5×10⁻¹⁰,
  d = +3.7) — confirming the routing-mix latency cost is **real, not single-pass noise** (resolving the
  RQ3 TODO). Least-cost is lowest (707 ms); the latency/cost trade-off is now quantified.

Figures `figures/fig_stats_{cost,quality,latency}.png` show means ±95% CI over the 30 repetitions.

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
workloads; **modeled** self-hosting/GPU economics (RQ6); the Mistral EU model uses Azure AI Foundry
DataZoneStandard (EU data zone) — strict France-only residency still relies on a modeled fallback;
volatile cloud prices (isolated as configuration). **Construct:** acceptability/win-rate
are quality proxies; declarative sovereignty simplifies legal reality — the system targets *audit
preparation*, not legal compliance. **Conclusion:** beyond the deterministic single pass we now report
a **30-repetition** run (temperature 0.7) with 95% CIs, Mann-Whitney U significance and Cliff's
δ/Cohen's d effect sizes (§6); the cost reduction and the latency increase are highly significant with
large effect sizes. The remaining caveat is **judge dependence** of the small quality gap (the N=30
run used a gpt-4o-mini judge; the single-pass gpt-4o judge showed parity). A **two-judge agreement
study** (gpt-4o vs Mistral-Large, n=120; `results/judge_agreement_summary.md`) finds **moderate
agreement** — Cohen's quadratic-weighted κ=0.40, Krippendorff α=0.39, Spearman ρ=0.61 (p≈10⁻¹³),
within-1 agreement 94.2% — i.e. judges correlate significantly but do not agree strongly on the 1–5
scale, which is precisely why we treat the small quality gap as *judge-dependent* and plan a human
evaluation (`QUALITY_EVALUATION_PROTOCOL.md`).

## 9. Conclusion and Future Work

Within our preliminary, two-provider, single-pass testbed, a gateway-level, economic-aware,
sovereignty-constrained, budget-aware control plane substantially reduced cost (−70.8%) while keeping
quality comparable, enforcing sovereignty with zero violations — including **maintaining 100%
availability under an EU-only policy via a real EU provider** — and preserving availability under
budget pressure. We deliberately frame these as *preliminary evidence and a reproducible framework*,
not universal claims. To reach a defensible Q1 result we plan (see `ROADMAP_Q1.md` and the dedicated
protocol files):
(1) **more providers** (Azure OpenAI, additional Mistral/Foundry and other models) beyond the current
OpenAI+Mistral pair (`MULTI_PROVIDER_EVALUATION_PLAN.md`);
(2) **real GPU self-hosting** (vLLM) to validate the modeled break-even
(`GPU_SELF_HOSTING_VALIDATION_PLAN.md`);
(3) **multi-judge + human** quality evaluation with agreement statistics
(`QUALITY_EVALUATION_PROTOCOL.md`);
(4) **multi-judge/human** quality evaluation to settle the judge-dependent quality sign — the
**≥30-repetition** statistical run (CIs, significance, effect sizes) is **done** (§6);
(5) **data-path enforcement in Envoy** with load testing and failover;
(6) **RQ7 — human FinOps expert vs automated control plane** (`RQ7_HUMAN_EXPERT_PROTOCOL.md`), a
planned human study comparing expert and automated routing/hosting decisions;
(7) integrating a **learned difficulty router** (Hybrid LLM/RouteLLM) with our governance constraints.
Items (1)–(3),(6) are *not yet evaluated*; the paper claims only what the committed results support.

## Artifact / Reproducibility

Operator + `experimentation/` (datasets, harness, scripts, results, figures, cached responses).
Reproduce with `scripts/run_experiment.sh` then `python3 scripts/analyze_results.py`. Every test is
journaled; no test is skipped.

## References

See `references.bib` (BibTeX). Key works cited: model routing/cascades — FrugalGPT
[chen2023frugalgpt], Hybrid LLM [ding2024hybridllm], RouteLLM [ong2024routellm], AutoMix
[aggarwal2024automix]; serving — Orca [yu2022orca], vLLM [kwon2023pagedattention], SGLang
[zheng2024sglang], DistServe [zhong2024distserve], surveys [zhou2024surveyinference;
li2025tamingtitans]; gateways — Envoy AI Gateway [envoyaigateway], LiteLLM [litellm]; evaluation —
[zheng2023judge]; regulation — GDPR [gdpr2016], EU AI Act [euaiact2024].
