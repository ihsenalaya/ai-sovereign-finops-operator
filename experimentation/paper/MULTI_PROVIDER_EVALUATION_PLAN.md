# Multi-provider evaluation plan (PLANNED — not yet evaluated)

Status: **future work**. The current paper evaluates a single provider (OpenAI). Nothing here is
claimed as completed. The harness is already provider-agnostic (`internal/llm.Client` + `internal/
catalog`), so adding providers is additive.

## Providers to cover
| Provider | Models (tiers) | Zone(s) | Notes |
|----------|----------------|---------|-------|
| OpenAI (done) | gpt-4o / gpt-4o-mini / gpt-4.1-nano | US | baseline phase, real |
| Azure OpenAI | gpt-4o / gpt-4o-mini (EU deployment) | FR/EU | makes RQ4 EU-only **real**, not modeled |
| Mistral API | mistral-large / mistral-small | EU (FR) | EU-native managed provider |
| Self-hosted vLLM | Llama-3-70B / Mistral on GPU | FR (on-prem/AKS) | see GPU_SELF_HOSTING_VALIDATION_PLAN.md |

## What it unlocks
- **RQ4 fully real:** with an EU-hosted provider (Azure EU / Mistral), france-only / eu-only scenarios
  route to *real* compliant models instead of the modeled stub — measuring real sovereignty cost.
- **Cross-provider routing:** RQ1/RQ2 gain provider diversity (price/quality/latency differ by
  provider), strengthening generality.
- **Failover (new):** provider outage → reroute; measure availability (ties to §3 system rigor).

## Implementation steps
1. Add `llm.Client` impls (Azure: same Chat API, different base URL + `api-version`; Mistral: native
   API; vLLM: OpenAI-compatible server URL). Keys via files/env, never logged.
2. Add catalog entries (real pricing, zone, managed flag, quality prior) per provider.
3. Add `provider` as an experiment dimension; extend `calls.csv` and figures (group by provider).
4. Re-run RQ1–RQ5 with ≥30 reps (stats via `scripts/stats.py`).

## Scenarios to add
- Cross-provider cost/quality Pareto (same tier across providers).
- EU-only with a real EU provider (RQ4 real).
- Provider-failover availability (kill one provider mid-run).

## Honesty
Until executed, all multi-provider results remain **planned**; the paper must not imply more than the
single-provider evidence currently supports.
