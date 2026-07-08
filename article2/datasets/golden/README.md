# Article 2 Golden Datasets

These files contain manually curated golden prompts and reference answers for
the four application-level quality gates used in Article 2:

- `finance-quality-golden.prompts.yaml`
- `legal-quality-golden.prompts.yaml`
- `marketing-quality-golden.prompts.yaml`
- `rh-quality-golden.prompts.yaml`

The references were written and verified manually for this article workflow.
They are not LLM-generated placeholders. Each item is intentionally concise so
that the current `AIQualityGate` can evaluate:

- keyword coverage,
- semantic proximity to a stable reference,
- latency under a bounded max token budget.

Dataset scope:

- 20 prompts per application dataset,
- deterministic references,
- explicit required keywords,
- token ceilings matched to the application style.

The companion manifest `configmaps.yaml` materializes these datasets as the
Kubernetes `ConfigMap` objects mounted by `AIQualityGate`.
