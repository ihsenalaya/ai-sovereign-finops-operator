# Experiment summary


**Headline:** Ours reduces cost by **70.8%** vs premium-static with a quality change of **-1.39%** (win-rate vs premium 50.0%).

## RQ1 Cost
| strategy | total_cost_eur | served | blocked | cost_per_request_eur | cost_per_token_eur | savings_vs_premium_pct |
| --- | --- | --- | --- | --- | --- | --- |
| B1-premium-static | 0.03893 | 40 | 0 | 0.000973 | 7e-06 | 0.0 |
| B4-static-policy | 0.021011 | 40 | 0 | 0.000525 | 5e-06 | 46.03 |
| B5-budget-hard-block | 0.03893 | 40 | 0 | 0.000973 | 7e-06 | 0.0 |
| B6-ours | 0.011353 | 40 | 0 | 0.000284 | 2e-06 | 70.84 |

## RQ2 Quality
| strategy | mean_quality_norm | acceptable_rate_pct | winrate_vs_premium_pct | pairwise_comparisons |
| --- | --- | --- | --- | --- |
| B1-premium-static | 0.9 | 97.5 | 0.0 | 0 |
| B4-static-policy | 0.86875 | 97.5 | 35.0 | 20 |
| B5-budget-hard-block | 0.9 | 97.5 | 0.0 | 0 |
| B6-ours | 0.9125 | 97.5 | 50.0 | 40 |

## RQ3 Latency
| strategy | latency_p50_ms | latency_p95_ms | latency_p99_ms | latency_mean_ms | routing_decision_us |
| --- | --- | --- | --- | --- | --- |
| B1-premium-static | 906.0 | 2494.0 | 2646.0 | 1125.97 | 10.47 |
| B4-static-policy | 797.0 | 1911.0 | 2634.0 | 941.3 | 9.5 |
| B5-budget-hard-block | 906.0 | 2494.0 | 2646.0 | 1125.97 | 9.38 |
| B6-ours | 946.0 | 3475.0 | 4265.0 | 1509.95 | 10.83 |

## RQ4 Declared-policy scenarios
| scenario | strategy | total_cost_eur | served | blocked | violations | reroutes | mean_quality_norm |
| --- | --- | --- | --- | --- | --- | --- | --- |
| global | B1-premium-static | 0.03893 | 40 | 0 | 0 | 0 | 0.9 |
| global | B6-ours | 0.011353 | 40 | 0 | 0 | 40 | 0.9125 |
| eu-only | B1-premium-static | 0.03893 | 40 | 0 | 40 | 0 | 0.9 |
| eu-only | B6-ours | 0.01816 | 40 | 0 | 0 | 40 | 0.86625 |
| no-external-sensitive | B1-premium-static | 0.03893 | 40 | 0 | 10 | 0 | 0.9 |
| no-external-sensitive | B6-ours | 0.010691 | 40 | 0 | 0 | 40 | 0.89125 |

## RQ5 Budget
| policy | budget_eur | used_eur | served | blocked | availability_pct | budget_overrun_pct | mean_quality_norm |
| --- | --- | --- | --- | --- | --- | --- | --- |
| alert-only | 0.003238 | 0.008095 | 10 | 0 | 100.0 | 150.0 | 0.8 |
| hard-block | 0.003238 | 0.004987 | 6 | 4 | 60.0 | 54.03 | 0.75 |
| ours-graceful | 0.003238 | 0.000497 | 10 | 0 | 100.0 | 0.0 | 0.85 |

## Ablation
| variant | total_cost_eur | savings_vs_nocost_pct | mean_quality_norm |
| --- | --- | --- | --- |
| full-system | 0.011353 | 0.0 | 0.9125 |
| no-cost-term | 0.03893 | 0.0 | 0.9 |
| no-quality-term | 0.001213 | 96.88 | 0.8625 |
| no-latency-term | 0.011353 | 70.84 | 0.9125 |
