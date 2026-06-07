# Experiment summary


**Headline:** Ours reduces cost by **70.8%** vs premium-static with a quality change of **-1.39%** (win-rate vs premium 50.0%).

## RQ1 Cost
| strategy             |   total_cost_eur |   served |   blocked |   cost_per_request_eur |   cost_per_token_eur |   savings_vs_premium_pct |
|:---------------------|-----------------:|---------:|----------:|-----------------------:|---------------------:|-------------------------:|
| B1-premium-static    |         0.03893  |       40 |         0 |               0.000973 |                7e-06 |                     0    |
| B2-round-robin       |         0.014667 |       40 |         0 |               0.000367 |                3e-06 |                    62.32 |
| B3-least-cost        |         0.0007   |       40 |         0 |               1.7e-05  |                0     |                    98.2  |
| B4-static-policy     |         0.021011 |       40 |         0 |               0.000525 |                5e-06 |                    46.03 |
| B5-budget-hard-block |         0.03893  |       40 |         0 |               0.000973 |                7e-06 |                     0    |
| B6-ours              |         0.011353 |       40 |         0 |               0.000284 |                2e-06 |                    70.84 |

## RQ2 Quality
| strategy             |   mean_quality_norm |   acceptable_rate_pct |   winrate_vs_premium_pct |   pairwise_comparisons |
|:---------------------|--------------------:|----------------------:|-------------------------:|-----------------------:|
| B1-premium-static    |             0.9     |                  97.5 |                     0    |                      0 |
| B2-round-robin       |             0.90525 |                  97.5 |                    51.85 |                     27 |
| B3-least-cost        |             0.8575  |                 100   |                    42.5  |                     20 |
| B4-static-policy     |             0.86875 |                  97.5 |                    35    |                     20 |
| B5-budget-hard-block |             0.9     |                  97.5 |                     0    |                      0 |
| B6-ours              |             0.9125  |                  97.5 |                    50    |                     40 |

## RQ3 Latency
| strategy             |   latency_p50_ms |   latency_p95_ms |   latency_p99_ms |   latency_mean_ms |   routing_decision_us |
|:---------------------|-----------------:|-----------------:|-----------------:|------------------:|----------------------:|
| B1-premium-static    |              906 |             2494 |             2646 |           1125.97 |                 10.68 |
| B2-round-robin       |              797 |             2902 |             3576 |           1075.38 |                 13.6  |
| B3-least-cost        |              700 |             1476 |             2815 |            753.98 |                  7.15 |
| B4-static-policy     |              797 |             1911 |             2634 |            941.3  |                 13.21 |
| B5-budget-hard-block |              906 |             2494 |             2646 |           1125.97 |                  9    |
| B6-ours              |              946 |             3475 |             4265 |           1509.95 |                 14.66 |

## RQ4 Sovereignty
| scenario              | strategy          |   total_cost_eur |   served |   blocked |   violations |   reroutes |   mean_quality_norm |
|:----------------------|:------------------|-----------------:|---------:|----------:|-------------:|-----------:|--------------------:|
| global                | B1-premium-static |         0.03893  |       40 |         0 |            0 |          0 |             0.9     |
| global                | B6-ours           |         0.011353 |       40 |         0 |            0 |         40 |             0.9125  |
| eu-only               | B1-premium-static |         0.03893  |       40 |         0 |           40 |          0 |             0.9     |
| eu-only               | B6-ours           |         0.01816  |       40 |         0 |            0 |         40 |             0.86625 |
| france-only           | B1-premium-static |         0.03893  |       40 |         0 |           40 |          0 |             0.9     |
| france-only           | B6-ours           |         0.00011  |       20 |        20 |            0 |         20 |             0.74    |
| no-external-sensitive | B1-premium-static |         0.03893  |       40 |         0 |           10 |          0 |             0.9     |
| no-external-sensitive | B6-ours           |         0.010691 |       40 |         0 |            0 |         40 |             0.89125 |
| self-hosted-only      | B1-premium-static |         0.03893  |       40 |         0 |           40 |          0 |             0.9     |
| self-hosted-only      | B6-ours           |         0.00011  |       20 |        20 |            0 |         20 |             0.74    |

## RQ5 Budget
| policy        |   budget_eur |   used_eur |   served |   blocked |   availability_pct |   budget_overrun_pct |   mean_quality_norm |
|:--------------|-------------:|-----------:|---------:|----------:|-------------------:|---------------------:|--------------------:|
| alert-only    |     0.003238 |   0.008095 |       10 |         0 |                100 |               150    |                0.8  |
| hard-block    |     0.003238 |   0.004987 |        6 |         4 |                 60 |                54.03 |                0.75 |
| ours-graceful |     0.003238 |   0.000497 |       10 |         0 |                100 |                 0    |                0.85 |

## RQ6 Break-even (modeled)
|   tokens_per_day |   managed_monthly_eur |   selfhosted_monthly_eur |   monthly_savings_eur |   payback_months | recommendation   |
|-----------------:|----------------------:|-------------------------:|----------------------:|-----------------:|:-----------------|
|      1e+06       |                142.5  |                     2500 |              -2357.5  |            nan   | keep-managed     |
|      1.5e+06     |                213.75 |                     2500 |              -2286.25 |            nan   | keep-managed     |
|      2.25e+06    |                320.62 |                     2500 |              -2179.38 |            nan   | keep-managed     |
|      3.375e+06   |                480.94 |                     2500 |              -2019.06 |            nan   | keep-managed     |
|      5.0625e+06  |                721.41 |                     2500 |              -1778.59 |            nan   | keep-managed     |
|      7.59375e+06 |               1082.11 |                     2500 |              -1417.89 |            nan   | keep-managed     |
|      1.13906e+07 |               1623.16 |                     2500 |               -876.84 |            nan   | keep-managed     |
|      1.70859e+07 |               2434.75 |                     2500 |                -65.25 |            nan   | keep-managed     |
|      2.56289e+07 |               3652.12 |                     2500 |               1152.12 |              4.3 | self-host        |
|      3.84434e+07 |               5478.18 |                     2500 |               2978.18 |              1.7 | self-host        |
|      5.7665e+07  |               8217.27 |                     2500 |               5717.27 |              0.9 | self-host        |
|      8.64976e+07 |              12325.9  |                     2500 |               9825.9  |              0.5 | self-host        |
|      1.29746e+08 |              18488.8  |                     2500 |              15988.9  |              0.3 | self-host        |
|      1.9462e+08  |              27733.3  |                     2500 |              25233.3  |              0.2 | self-host        |

## Ablation
| variant         |   total_cost_eur |   savings_vs_nocost_pct |   mean_quality_norm |
|:----------------|-----------------:|------------------------:|--------------------:|
| full-system     |         0.011353 |                    0    |              0.9125 |
| no-cost-term    |         0.03893  |                    0    |              0.9    |
| no-quality-term |         0.001213 |                   96.88 |              0.8625 |
| no-latency-term |         0.011353 |                   70.84 |              0.9125 |
