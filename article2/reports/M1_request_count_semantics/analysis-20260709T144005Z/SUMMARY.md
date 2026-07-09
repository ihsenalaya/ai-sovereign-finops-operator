# M1 — Request-count semantics decomposition

Source (measured): `experiments/runs/E1_finops_attribution/aks-live-20260708T160246Z/metrics/gateway_metrics.prom`

| ns | app | model | success | error | total | in-tok(success) | out-tok(success) |
|---|---|---|--:|--:|--:|--:|--:|
| finance | risk-assistant | gpt-france-mini | 1 | 0 | 1 | 33 | 80 |
| finance | risk-assistant | gpt-us-mini | 85 | 660 | 745 | 1299 | 10882 |
| finance | risk-assistant | mistral-large-latest | 1 | 0 | 1 | 30 | 80 |
| legal | contract-review | gpt-france-mini | 6 | 0 | 6 | 115 | 836 |
| legal | contract-review | mistral-large-latest | 1 | 0 | 1 | 30 | 90 |
| marketing | content-writer | gpt-france-mini | 1 | 0 | 1 | 30 | 90 |
| marketing | content-writer | mistral-large-latest | 6 | 0 | 6 | 93 | 390 |
| rh | chatbot-rh | gpt-france-mini | 8 | 0 | 8 | 149 | 571 |
| rh | chatbot-rh | mistral-large-latest | 1 | 0 | 1 | 29 | 37 |

**Finding.** Report 'requests' equalled the ERROR count (collector took max over duration series, incl. error_type). Cost/tokens come only from the token metric, which the gateway emits solely for successful responses -> error responses carry ZERO billed tokens.

**Do error responses carry billed tokens? NO.** The token metric (`gen_ai_client_token_usage`) is emitted only for successful responses; every error series has `response_model=unknown` and appears solely in the duration histogram. Cost-of-errors = 0 EUR; all attributed cost is from successful calls.
