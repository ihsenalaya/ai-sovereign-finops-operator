#!/usr/bin/env bash
# Shared config + helpers for the Azure (AKS + GPU) automation.
# Cost discipline: the GPU node pool autoscales 0->N, so you only pay GPU while
# a benchmark Job is pending. Tear down or scale to 0 when idle.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AZURE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${AZURE_DIR}/../.." && pwd)"

# --- Azure resources ---
export RG="${RG:-greenops-rg}"
export LOCATION="${LOCATION:-francecentral}"          # EU/FR for sovereignty
export CLUSTER="${CLUSTER:-greenops-aks}"
export K8S_VERSION="${K8S_VERSION:-}"                  # empty = AKS default

# --- System (CPU) node pool: small, for operator/Envoy/Prometheus ---
# NOTE: choose a family that HAS quota. On the FHIR sub, DSv5 had 0 cores while
# DSv4/DSv3/BS families have 10 (Total Regional vCPUs = 10). D4s_v4 = 4 vCPU.
export SYS_VMSIZE="${SYS_VMSIZE:-Standard_D4s_v4}"
export SYS_COUNT="${SYS_COUNT:-1}"

# --- GPU node pool: autoscale 0..MAX. Quota granted: NCASv3_T4 = 8 vCPU =>
#     one Standard_NC8as_T4_v3 (8 vCPU, 1x T4 16GB). Override for A10/A100/H100. ---
export GPU_POOL="${GPU_POOL:-gpupool}"
export GPU_VMSIZE="${GPU_VMSIZE:-Standard_NC8as_T4_v3}"   # 1x T4 16GB (fits the 8 vCPU quota)
export GPU_MIN="${GPU_MIN:-0}"
export GPU_MAX="${GPU_MAX:-1}"                            # 1 node = 8 vCPU = whole quota
export GPU_TAINT="${GPU_TAINT:-sku=gpu:NoSchedule}"

# --- vLLM ---
# T4 = 16GB VRAM: an 8B model in FP16 does NOT fit. Default to an AWQ 4-bit 7B
# (no HF gating) that fits comfortably. Override VLLM_MODEL/EXTRA_ARGS as needed.
export VLLM_MODEL="${VLLM_MODEL:-Qwen/Qwen2.5-7B-Instruct-AWQ}"
export VLLM_SERVED_NAME="${VLLM_SERVED_NAME:-qwen2.5-7b-awq}"
export VLLM_EXTRA_ARGS="${VLLM_EXTRA_ARGS:---quantization awq --max-model-len 8192 --gpu-memory-utilization 0.92}"
export HF_TOKEN="${HF_TOKEN:-}"                        # for gated models (optional)

# --- Operator image (from the GHCR push) ---
export OPERATOR_IMAGE="${OPERATOR_IMAGE:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator}"
export OPERATOR_TAG="${OPERATOR_TAG:-0.3.7}"

# --- Key Vault (secrets: OpenAI key, HF token, gateway creds) ---
# KV names are global + 3-24 chars; derive a stable suffix from the subscription.
_subid="$(az account show --query id -o tsv 2>/dev/null | tr -d '-' || echo 000000)"
export KEYVAULT="${KEYVAULT:-greenops-kv-${_subid:0:6}}"
export KV_OPENAI_SECRET="${KV_OPENAI_SECRET:-openai-api-key}"
export KV_HF_SECRET="${KV_HF_SECRET:-hf-token}"
# Local source file (never printed/committed; docs/openaikey.txt is gitignored).
export OPENAI_KEY_FILE="${OPENAI_KEY_FILE:-${REPO_ROOT}/docs/openaikey.txt}"

log()  { printf '\033[1;36m[azure]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[azure]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[azure]\033[0m %s\n' "$*" >&2; exit 1; }
require() { command -v "$1" >/dev/null 2>&1 || die "missing tool: $1"; }
