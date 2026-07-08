# Multi-Provider Evaluation Notes

The revised paper reports a bounded two-provider evaluation with managed model
endpoints and declared provider metadata.

Future provider extensions should add:

- another managed endpoint with documented pricing and residency metadata;
- explicit credential handling via environment variables or files excluded from
  Git;
- new rows in the model catalog and workload allow-lists;
- cached-response and exact-match evidence labels consistent with the paper;
- refreshed `results/`, `results-stats/`, `results-bench/`, and figures.

Any new provider result must be measured or replayed from measured calls before
appearing in the manuscript.
