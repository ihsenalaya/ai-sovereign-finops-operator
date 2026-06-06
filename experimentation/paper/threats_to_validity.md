# Threats to validity

## Internal validity
- **LLM API variability:** managed endpoints vary in latency and, occasionally, content. We fix
  temperature=0 and cache responses; latency is reported with bootstrap 95% CIs over per-call data.
- **Judge bias:** quality uses an LLM-as-judge (gpt-4o). The judge may favor verbose or same-family
  answers. Mitigations: absolute (1-5) and pairwise metrics; the judge is held constant across
  strategies; a human or alternate-judge audit is planned.
- **Cache effects:** the answer cache ensures determinism but means a single measured pass; latency
  variance is therefore estimated from intra-pass per-call samples.
- **Premium-reference anchoring:** the premium model is simultaneously the routing reference *and* the
  anchor of the pairwise quality comparison, which can bias pairwise win-rate toward premium. We report
  it explicitly (win-rate 41.7% < 50%) and do not claim "no quality loss"; a multi-judge + human
  protocol (`QUALITY_EVALUATION_PROTOCOL.md`) is planned to remove this dependency.
- **Latency tail (RQ3):** B6's p95 exceeds premium's; with a single pass, tail percentiles are
  sensitive to a few slow API responses. We flag this as not fully resolved and require ≥30 reps with
  cross-run CIs to separate routing-distribution effects from API jitter.

## External validity
- **Model/provider scope:** current results use OpenAI tiers only; a second provider (cross-provider
  routing) is the immediate next step. Generalization to other providers/models is not yet shown.
- **Self-hosting is modeled:** the self-hosted/vLLM path uses modeled cost and a stub response (no GPU
  locally); real GPU runs are deferred to an Azure phase. Break-even (RQ6) is therefore a modeled
  prediction, clearly labeled, intended to be validated against real GPU economics later.
- **Workloads:** four synthetic enterprise workloads; real production traffic may differ in
  distribution and sensitivity.
- **Prices:** cloud LLM and GPU prices change; prices are configuration (isolated in the catalog) and
  results include sensitivity via the break-even curve.

## Construct validity
- **Quality measurement:** acceptability/win-rate are proxies for task success; exact-match/F1 would
  require labeled datasets and are workload-specific.
- **Sovereignty model:** declarative zone/sensitivity policies simplify real legal/regulatory
  constraints; the system targets *audit preparation*, not legal compliance.

## Conclusion validity
- **Repetitions:** the current pass is deterministic (temperature 0); the protocol supports ≥30
  repetitions per scenario for stronger statistics. Latency CIs are reported; cost/quality are exact
  under the fixed configuration.
- **Effect sizes:** to be reported alongside significance tests when multi-repetition runs are added.
