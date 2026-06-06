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
export SYS_VMSIZE="${SYS_VMSIZE:-Standard_D4s_v5}"
export SYS_COUNT="${SYS_COUNT:-1}"

# --- GPU node pool: autoscale 0..MAX. T4 is cheapest/most quota-friendly on MPN;
#     override GPU_VMSIZE for A10/A100/H100 once quota is granted. ---
export GPU_POOL="${GPU_POOL:-gpupool}"
export GPU_VMSIZE="${GPU_VMSIZE:-Standard_NC4as_T4_v3}"   # T4 (validation). H100: Standard_NC40ads_H100_v5
export GPU_MIN="${GPU_MIN:-0}"
export GPU_MAX="${GPU_MAX:-1}"
export GPU_TAINT="${GPU_TAINT:-sku=gpu:NoSchedule}"

# --- vLLM ---
export VLLM_MODEL="${VLLM_MODEL:-meta-llama/Meta-Llama-3-8B-Instruct}"
export VLLM_SERVED_NAME="${VLLM_SERVED_NAME:-llama-3-8b}"
export HF_TOKEN="${HF_TOKEN:-}"                        # for gated models (optional)

# --- Operator image (from the GHCR push) ---
export OPERATOR_IMAGE="${OPERATOR_IMAGE:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator}"
export OPERATOR_TAG="${OPERATOR_TAG:-0.1.0}"

log()  { printf '\033[1;36m[azure]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[azure]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[azure]\033[0m %s\n' "$*" >&2; exit 1; }
require() { command -v "$1" >/dev/null 2>&1 || die "missing tool: $1"; }
