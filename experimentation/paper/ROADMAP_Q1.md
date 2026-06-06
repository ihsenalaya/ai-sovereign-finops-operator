# Roadmap to a Q1-grade paper

Honest assessment: the current draft (`paper.md` v0.2) is **promising preliminary work**, not yet
Q1-ready. It has real measurements, a clean artifact, and a defensible narrative, but a Q1 systems/ML
venue will require stronger novelty positioning, larger and more rigorous experiments, real
self-hosting, and live data-path enforcement. This file is the concrete plan to close that gap.

Legend: ✅ done · 🟡 partial · ⬜ to do.

## Companion protocol documents (this directory)
- `MULTI_PROVIDER_EVALUATION_PLAN.md` — add Azure OpenAI / Mistral / vLLM (planned).
- `GPU_SELF_HOSTING_VALIDATION_PLAN.md` — real GPU on AKS to validate RQ6 (planned).
- `QUALITY_EVALUATION_PROTOCOL.md` — multi-judge + human eval, Cohen's Kappa / Krippendorff's Alpha.
- `RQ7_HUMAN_EXPERT_PROTOCOL.md` — human FinOps expert vs automated control plane (planned).
- `../scripts/stats.py` — significance/effect-size scaffolding (ready; needs ≥30-rep data).

## 0. Target venues
- **Systems / cloud:** USENIX ATC, ACM SoCC, IEEE TPDS (Q1 journal), Middleware, EuroSys.
- **ML systems:** MLSys, NeurIPS D&B (for the benchmark), ICLR (if framed as routing method).
- **Services/FinOps angle:** IEEE TSC, ACM TOIT (Q1 journals).
Pick one primary; tailor framing (systems contribution vs routing method vs services).

## 1. Scientific positioning & novelty (✅/🟡)
- ✅ Frame as *method + artifact*, not "we built an operator".
- 🟡 **Sharpen the delta vs prior routing** (FrugalGPT, Hybrid LLM, RouteLLM, AutoMix): we are the
  only one unifying **hard sovereignty constraints + per-tenant budgets + graceful degradation** at the
  **gateway/control-plane** level with attribution. ⬜ Make this a crisp claim with a feature matrix
  table (us vs each prior work across: cost, quality, sovereignty, budget, attribution, multi-tenant,
  gateway-native).
- ⬜ State the **research gap** precisely and the **falsifiable hypotheses** per RQ.

## 2. Experimental rigor (🟡 → the biggest gap)
- 🟡 **Repetitions & statistics.** Current run = 1 deterministic pass (temp 0). ⬜ Move to **≥30
  repetitions** per scenario (config already supported); for quality, vary temperature and prompts
  (bootstrap), report **mean ± 95% CI**, **significance tests** (paired bootstrap / Wilcoxon), and
  **effect sizes** (Cliff's δ / Cohen's d). Add seeds + warm-up/measure split (already designed).
- ⬜ **Scale of data.** 40 prompts → **≥1k prompts** per workload drawn from *public* datasets
  (MT-Bench, MMLU subsets, GSM8K, HumanEval, a RAG set) so results are comparable and not synthetic-
  only. Keep the enterprise workloads as a case study.
- ⬜ **Literature baselines.** Add FrugalGPT-style cascade and a RouteLLM/Hybrid-LLM learned router as
  baselines (not just B1–B5), so we beat or match real routing methods, not only naïve ones.
- ⬜ **Quality robustness.** Multiple judges (e.g., GPT-4o + a second strong model), human spot-check
  on a sample, report inter-judge agreement; mitigate position/verbosity bias (randomize A/B order).
- ⬜ **Sensitivity analysis.** Sweep the scoring weights (α,β,γ,δ,ε) and the quality threshold; show
  Pareto fronts (cost vs quality) and stability of conclusions.

## 3. System rigor (🟡)
- 🟡 Routing decision overhead measured at control-plane (~µs). ⬜ **Live Envoy data-path
  enforcement** (ext_proc / dynamic metadata) and measure end-to-end overhead, throughput, and
  failover under load (RQ3 at scale, with a load generator).
- ⬜ **Multi-tenant / concurrency** experiments (many namespaces, contention, fairness of budgets).
- ⬜ **Fault tolerance:** provider outage → automatic failover/degradation; measure availability.

## 4. Provider & self-hosting coverage (⬜)
- ⬜ **Second provider** (e.g., Mistral/Azure OpenAI/Anthropic) → real **cross-provider** routing and
  sovereignty (EU-hosted provider makes RQ4 fully real, not modeled).
- ⬜ **Real GPU self-hosting** (vLLM on Azure GPU) to replace the *modeled* self-hosted entry →
  validate RQ6 break-even against measured GPU economics; report prediction error.

## 5. Reproducibility / artifact evaluation (✅/🟡)
- ✅ One-command run, response cache, test journal, CSV + figures, public repo.
- 🟡 ⬜ Pin exact model snapshots and prices (with date); Dockerized analysis env (`requirements.txt`
  + container) so figures regenerate bit-for-bit; deposit an **archived artifact** (Zenodo DOI).
- ⬜ Prepare for **Artifact Evaluation** badges (claims ↔ scripts mapping table).

## 6. Writing (🟡)
- ✅ Full draft structure with real numbers.
- 🟡 Related work cites verified recent papers. ⬜ Expand to ~25–35 references; add the feature-matrix
  table; write Intro that leads with the gap; strengthen Discussion with Pareto analysis; finalize
  Threats with the added statistics.
- ⬜ Figures to publication quality (vector PDF, consistent fonts, captions, CIs on every bar).

## 7. Concrete milestones (suggested)
| Phase | Work | Output | Rough effort |
|------|------|--------|--------------|
| P1 | 2nd provider + public datasets + ≥30 reps + stats | strong RQ1–RQ5 with CIs/effect sizes | ~2–4 days |
| P2 | Literature baselines (FrugalGPT/RouteLLM/Hybrid) | competitive comparison | ~2–3 days |
| P3 | Real vLLM on Azure GPU | RQ6 validated, break-even error | ~2–3 days |
| P4 | Live Envoy enforcement + load test | RQ3 at scale, failover | ~3–5 days |
| P5 | Sensitivity/Pareto + multi-judge quality (Kappa/Alpha) | robustness section | ~2 days |
| P6 | RQ7 human FinOps expert study | expert-vs-system agreement | ~3–5 days |
| P7 | Writing polish + feature matrix + artifact/Zenodo | submission-ready | ~3–5 days |

→ Realistic **2–4 weeks** of focused work to a credible Q1 submission, most of it in P1–P4. P6
(human study) can run in parallel once scenarios are frozen.

## 8. Definition of done (Q1 bar)
- ≥30 reps, CIs, significance + effect sizes on every headline claim.
- ≥2 providers + real GPU self-hosting; RQ4 and RQ6 no longer modeled.
- Beats/matches at least one *learned* routing baseline from the literature.
- Live gateway enforcement with measured overhead at load.
- Public datasets + archived, reproducible artifact (DOI), AE-ready.
- 8 publication figures with CIs; 25–35 references; sharp novelty positioning.
