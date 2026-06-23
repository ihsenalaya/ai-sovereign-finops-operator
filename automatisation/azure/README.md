# Azure (AKS + GPU) automation — for the RQ6 / live phase

Turnkey, **cost-disciplined** automation to run the cluster-dependent experiments on Azure with
**no on-cluster fiddling**: the GPU node pool autoscales **0→N** so you pay GPU only while a benchmark
runs, and the GPU benchmark harness (`experimentation/cmd/gpubench`) is **already validated locally
against OpenAI** (same OpenAI-compatible API as vLLM).

## Prerequisites
- `az` (logged in), `kubectl`, `helm`, `go`. A subscription with **GPU quota** (MPN/VS starts at 0 —
  run `scripts/00-quota-check.sh` and request an increase first; T4 is the cheapest/most-grantable).

## One-shot
```bash
cd automatisation/azure
# optional overrides: GPU_VMSIZE=Standard_NC40ads_H100_v5 VLLM_MODEL=... HF_TOKEN=...
scripts/up.sh        # quota check -> AKS -> GPU pool(0..N) -> NVIDIA+operator -> vLLM -> bench -> cost
MODE=idle scripts/down.sh   # scale GPU pool to 0 (stop paying GPU); MODE=delete removes everything
```

## Step by step (same scripts)
| # | Script | Does | GPU $ |
|---|--------|------|:-----:|
| 0 | `00-quota-check.sh` | show GPU quota + how to request | no |
| 1 | `01-create-aks.sh` | RG + AKS (Free tier) + small CPU pool | no |
| 2 | `02-add-gpu-nodepool.sh` | GPU pool, **autoscale 0..MAX**, tainted | no (stays at 0) |
| 3 | `03-install-stack.sh` | NVIDIA GPU Operator (+DCGM) + finops operator (Helm) | no |
| 4 | `04-deploy-vllm.sh` | deploy vLLM (OpenAI-compatible) → **GPU scales 0→1** | **yes** |
| 5 | `05-run-bench.sh` | port-forward vLLM+DCGM, run `gpubench` sweep | yes |
| 6 | `06-collect-cost.sh` | record GPU node-hours / cost | yes |
| 7 | `07-deploy-mistral-foundry.sh` | provision the three Azure AI Foundry demo deployments: Cohere, Mistral Large and GPT-4.1 Mini | no GPU |
|   | `down.sh` (MODE=idle) | scale GPU → 0 | stops |

Config is in `scripts/common.sh` (region `francecentral` for EU sovereignty, SKU, model, image).

## Azure AI Foundry models for the real Kind demo

The Envoy AI Gateway demo expects one Foundry account named `greenops-foundry`
and these deployments:

| Deployment | Model | Format | SKU |
|------------|-------|--------|-----|
| `cohere-command-a-latest` | `cohere-command-a` | Cohere | `GlobalStandard` |
| `mistral-large-latest` | `Mistral-Large-3` | Mistral AI | `DataZoneStandard` |
| `gpt-foundry-eu-mini` | `gpt-4.1-mini` | OpenAI | `DataZoneStandard` |

Provision or verify them in the active `az` subscription:

```bash
automatisation/azure/scripts/07-deploy-mistral-foundry.sh
az cognitiveservices account keys list -n greenops-foundry -g greenops-rg --query key1 -o tsv > operateur/docs/foundrykey.txt
```

`mistral-small-2503` is intentionally not the default third provider: in the
current subscription it is available only as `GlobalStandard`, so it cannot be
used as the compliant EU/DataZone radar provider.

## Cost model
- AKS control plane: **free** (Free tier). System CPU node: cheap, keep on/off.
- GPU: billed **only while the vLLM pod is scheduled** (autoscale 0→1→0). Target **~4–8 GPU-hours
  total** across the SKU sweep → fits a small MPN credit if you `down.sh` between runs.

## Parallelism
Independent work runs concurrently with `experimentation/scripts/parallel.sh` (bounded by `MAX_PAR`):
```bash
# Overlap local OpenAI experiments with the Azure GPU benchmark — no idle waiting:
experimentation/scripts/parallel.sh \
  "local-suite::cd experimentation && go run ./cmd/experiment -key ../operateur/docs/openaikey.txt" \
  "gpu-bench::automatisation/azure/scripts/05-run-bench.sh"
```
- GPU jobs are kept **sequential on one node** (cost/quota); CPU-side work (local experiments, load
  gen, analysis) is parallelized freely. To parallelize GPU jobs, raise `GPU_MAX` (more nodes = more $).

## What is already validated locally (no Azure)
- `gpubench` runs end-to-end against OpenAI (`results/gpubench.csv`) — proves the bench path.
- `vllm.yaml` passes **server dry-run** (valid K8s); the operator Helm chart templates clean.
- All scripts pass `bash -n`; `parallel.sh` verified with concurrent build+tests.
So on Azure the only new variables are GPU scheduling and the vLLM model download — no code surprises.

## Output → paper
`05-run-bench.sh` writes `experimentation/results/gpubench.csv` (throughput, p50/p95/p99, GPU util);
`06-collect-cost.sh` writes `results/azure_cost.txt`. Feed measured EUR/1M-tokens into
`internal/catalog` to turn RQ6 from **modeled** into **measured** (see
`experimentation/paper/GPU_SELF_HOSTING_VALIDATION_PLAN.md`).
