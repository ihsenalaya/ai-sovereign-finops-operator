# Live kind verification - partial run

Date: 2026-06-13

This bundle is preserved as negative/partial evidence. The automation completed
and collected real cluster evidence, but the application verification was not
fully successful.

## What happened

- Operator image used: `ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.7`
- `BUILD_OPERATOR=false`, so this run used the already available image instead of
  rebuilding from the current source.
- `rh/chatbot-rh` reached one real 200 after one initial `000`.
- `finance/risk-assistant` reached one real 200.
- `marketing/content-writer` reached one real 200 after one initial `000`.
- `legal/contract-review` received four real `429` responses and reached zero
  successful calls.
- `finance/rogue-script` made four direct unauthenticated calls to
  `api.openai.com` and received four real `401` responses.

## Operator status captured

- `totalInputTokens: 28`
- `totalOutputTokens: 543`
- `totalCostEUR: "0"`
- `AISovereigntyPolicy findingsCount: 5`

The zero cost was not a zero-spend result. It exposed a sub-cent formatting bug
in the operator status path; the following run fixed this by preserving
micro-EUR precision.

## Cleanup

The automation collected logs, CRDs, gateway metrics, Tetragon evidence, and
shadow-egress output, then stopped the demo workloads and deleted the kind
cluster.
