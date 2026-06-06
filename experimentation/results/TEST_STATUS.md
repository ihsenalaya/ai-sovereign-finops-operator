# Experiment test status

- Run started: 2026-06-06T21:24:41Z
- Last update: 2026-06-06T21:25:44Z
- Totals: **42 PASS**, **0 FAIL**, 42 total

| # | Group | Test | Status | Duration | Details |
|--:|-------|------|:------:|---------:|---------|
| 1 | RQ1-3-matrix | B1-premium-static / W1-HR-Chatbot | PASS | 0ms | {"blocked":0,"costEUR":0.015415,"meanQuality1to5":4.5,"served":10} |
| 2 | RQ1-3-matrix | B1-premium-static / W2-RAG-Docs | PASS | 0ms | {"blocked":0,"costEUR":0.003018,"meanQuality1to5":5,"served":10} |
| 3 | RQ1-3-matrix | B1-premium-static / W3-Dev-Assistant | PASS | 0ms | {"blocked":0,"costEUR":0.012403,"meanQuality1to5":4.7,"served":10} |
| 4 | RQ1-3-matrix | B1-premium-static / W4-Analytical-Agent | PASS | 0ms | {"blocked":0,"costEUR":0.008095,"meanQuality1to5":4.2,"served":10} |
| 5 | RQ1-3-matrix | B2-round-robin / W1-HR-Chatbot | PASS | 6281ms | {"blocked":0,"costEUR":0.005547,"meanQuality1to5":4.392,"served":10} |
| 6 | RQ1-3-matrix | B2-round-robin / W2-RAG-Docs | PASS | 4056ms | {"blocked":0,"costEUR":0.001865,"meanQuality1to5":5,"served":10} |
| 7 | RQ1-3-matrix | B2-round-robin / W3-Dev-Assistant | PASS | 9769ms | {"blocked":0,"costEUR":0.005364,"meanQuality1to5":4.9,"served":10} |
| 8 | RQ1-3-matrix | B2-round-robin / W4-Analytical-Agent | PASS | 4385ms | {"blocked":0,"costEUR":0.001892,"meanQuality1to5":4.192,"served":10} |
| 9 | RQ1-3-matrix | B3-least-cost / W1-HR-Chatbot | PASS | 0ms | {"blocked":0,"costEUR":0.000055,"meanQuality1to5":3.96,"served":10} |
| 10 | RQ1-3-matrix | B3-least-cost / W2-RAG-Docs | PASS | 0ms | {"blocked":0,"costEUR":0.000117,"meanQuality1to5":5,"served":10} |
| 11 | RQ1-3-matrix | B3-least-cost / W3-Dev-Assistant | PASS | 0ms | {"blocked":0,"costEUR":0.000472,"meanQuality1to5":4.8,"served":10} |
| 12 | RQ1-3-matrix | B3-least-cost / W4-Analytical-Agent | PASS | 0ms | {"blocked":0,"costEUR":0.000056,"meanQuality1to5":3.96,"served":10} |
| 13 | RQ1-3-matrix | B4-static-policy / W1-HR-Chatbot | PASS | 0ms | {"blocked":0,"costEUR":0.000331,"meanQuality1to5":4,"served":10} |
| 14 | RQ1-3-matrix | B4-static-policy / W2-RAG-Docs | PASS | 0ms | {"blocked":0,"costEUR":0.000182,"meanQuality1to5":5,"served":10} |
| 15 | RQ1-3-matrix | B4-static-policy / W3-Dev-Assistant | PASS | 0ms | {"blocked":0,"costEUR":0.012403,"meanQuality1to5":4.7,"served":10} |
| 16 | RQ1-3-matrix | B4-static-policy / W4-Analytical-Agent | PASS | 0ms | {"blocked":0,"costEUR":0.008095,"meanQuality1to5":4.2,"served":10} |
| 17 | RQ1-3-matrix | B5-budget-hard-block / W1-HR-Chatbot | PASS | 0ms | {"blocked":0,"costEUR":0.015415,"meanQuality1to5":4.5,"served":10} |
| 18 | RQ1-3-matrix | B5-budget-hard-block / W2-RAG-Docs | PASS | 0ms | {"blocked":0,"costEUR":0.003018,"meanQuality1to5":5,"served":10} |
| 19 | RQ1-3-matrix | B5-budget-hard-block / W3-Dev-Assistant | PASS | 0ms | {"blocked":0,"costEUR":0.012403,"meanQuality1to5":4.7,"served":10} |
| 20 | RQ1-3-matrix | B5-budget-hard-block / W4-Analytical-Agent | PASS | 0ms | {"blocked":0,"costEUR":0.008095,"meanQuality1to5":4.2,"served":10} |
| 21 | RQ1-3-matrix | B6-ours / W1-HR-Chatbot | PASS | 0ms | {"blocked":0,"costEUR":0.000717,"meanQuality1to5":4.3,"served":10} |
| 22 | RQ1-3-matrix | B6-ours / W2-RAG-Docs | PASS | 0ms | {"blocked":0,"costEUR":0.000182,"meanQuality1to5":5,"served":10} |
| 23 | RQ1-3-matrix | B6-ours / W3-Dev-Assistant | PASS | 20615ms | {"blocked":0,"costEUR":0.009958,"meanQuality1to5":4.9,"served":10} |
| 24 | RQ1-3-matrix | B6-ours / W4-Analytical-Agent | PASS | 0ms | {"blocked":0,"costEUR":0.000497,"meanQuality1to5":4.4,"served":10} |
| 25 | RQ4-sovereignty | global / B1-premium-static | PASS | 0ms | {"blocked":0,"costEUR":0.03893,"meanQualityNorm":0.9,"reroutes":0,"served":40,"violations":0} |
| 26 | RQ4-sovereignty | global / B6-ours | PASS | 0ms | {"blocked":0,"costEUR":0.011353,"meanQualityNorm":0.9125,"reroutes":40,"served":40,"violations":0} |
| 27 | RQ4-sovereignty | eu-only / B1-premium-static | PASS | 0ms | {"blocked":0,"costEUR":0.03893,"meanQualityNorm":0.9,"reroutes":0,"served":40,"violations":40} |
| 28 | RQ4-sovereignty | eu-only / B6-ours | PASS | 14751ms | {"blocked":0,"costEUR":0.018161,"meanQualityNorm":0.86625,"reroutes":40,"served":40,"violations":0} |
| 29 | RQ4-sovereignty | france-only / B1-premium-static | PASS | 0ms | {"blocked":0,"costEUR":0.03893,"meanQualityNorm":0.9,"reroutes":0,"served":40,"violations":40} |
| 30 | RQ4-sovereignty | france-only / B6-ours | PASS | 0ms | {"blocked":20,"costEUR":0.00011,"meanQualityNorm":0.74,"reroutes":20,"served":20,"violations":0} |
| 31 | RQ4-sovereignty | no-external-sensitive / B1-premium-static | PASS | 0ms | {"blocked":0,"costEUR":0.03893,"meanQualityNorm":0.9,"reroutes":0,"served":40,"violations":10} |
| 32 | RQ4-sovereignty | no-external-sensitive / B6-ours | PASS | 0ms | {"blocked":0,"costEUR":0.010691,"meanQualityNorm":0.89125,"reroutes":40,"served":40,"violations":0} |
| 33 | RQ4-sovereignty | self-hosted-only / B1-premium-static | PASS | 0ms | {"blocked":0,"costEUR":0.03893,"meanQualityNorm":0.9,"reroutes":0,"served":40,"violations":40} |
| 34 | RQ4-sovereignty | self-hosted-only / B6-ours | PASS | 0ms | {"blocked":20,"costEUR":0.00011,"meanQualityNorm":0.74,"reroutes":20,"served":20,"violations":0} |
| 35 | RQ5-budget | alert-only | PASS | 0ms | {"availabilityPct":100,"blocked":0,"budgetEUR":0.003238,"meanQualityNorm":0.8,"overrunPct":150,"served":10,"usedEUR":0.008095} |
| 36 | RQ5-budget | hard-block | PASS | 0ms | {"availabilityPct":60,"blocked":4,"budgetEUR":0.003238,"meanQualityNorm":0.75,"overrunPct":54.030266,"served":6,"usedEUR":0.004987} |
| 37 | RQ5-budget | ours-graceful | PASS | 0ms | {"availabilityPct":100,"blocked":0,"budgetEUR":0.003238,"meanQualityNorm":0.85,"overrunPct":0,"served":10,"usedEUR":0.000497} |
| 38 | RQ6-breakeven | managed-vs-selfhosted-curve | PASS | 0ms | {"breakEvenTokensPerDay":25628906.25,"modeled":true,"points":14} |
| 39 | RQ-ablation | full-system | PASS | 0ms | {"costEUR":0.011353,"meanQualityNorm":0.9125} |
| 40 | RQ-ablation | no-cost-term | PASS | 0ms | {"costEUR":0.03893,"meanQualityNorm":0.9} |
| 41 | RQ-ablation | no-quality-term | PASS | 0ms | {"costEUR":0.001213,"meanQualityNorm":0.8625} |
| 42 | RQ-ablation | no-latency-term | PASS | 0ms | {"costEUR":0.011353,"meanQualityNorm":0.9125} |
