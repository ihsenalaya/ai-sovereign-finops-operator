# Methodology Summary

The revised methodology is self-contained in `latex/main.tex`.

Main evidence classes:

- `MEASURED`: live calls, uncached repetitions, or integration logs.
- `CACHED`: replay or reanalysis of previously collected real API calls.
- `SIMULATED`: deterministic policy or budget scenarios over declared metadata
  and existing result rows.

The main paper reports the bounded two-provider evaluation, objective
GSM8K/MMLU exact-match analysis, prompt-level bootstrap, and integration
verification. It does not use unmeasured hosting-economics material as a result.
