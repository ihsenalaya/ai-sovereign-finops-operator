# Azure Validation Summary

## Validated on 2026-07-11

The live E6 validation used already-existing Azure resources discovered through
the authenticated local Azure CLI context.

## Resources touched

- `a2fr60f9b020260708` in `francecentral`
- `a2us60f9b020260708` in `eastus`
- `greenops-fdry-60f9b0` in `westus2`

## Deployments successfully invoked

- `gpt_france_mini` using `gpt-4.1-mini` version `2025-04-14`
- `gpt_us_mini` using `gpt-4.1-mini` version `2025-04-14`
- `mistral_large_latest` using `Mistral-Large-3` version `1`

## Observed result

All recorded live invocation attempts in
`article3/experiments/processed/E6_azure_live_summary.json` completed with HTTP
status `200`, including OpenAI chat, OpenAI responses, and Foundry-served
Mistral chat paths.

## Important limitation

This was a bounded validation pass, not the full prompt target of at least 300
successful requests per model and three independent windows. The correct claim
is therefore live functional validation with estimated token-cost telemetry, not
full-scale statistical saturation.
