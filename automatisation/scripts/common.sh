#!/usr/bin/env bash
# Shared variables and helpers for the automatisation scripts.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AUTOMATISATION_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${AUTOMATISATION_DIR}/.." && pwd)"

# Cluster / image
export CLUSTER_NAME="${CLUSTER_NAME:-greenops}"
export IMAGE_REPO="${IMAGE_REPO:-greenops}"
export IMAGE_TAG="${IMAGE_TAG:-dev}"

# In-cluster Gitea (self-contained GitOps source — no external remote required).
export GITEA_NAMESPACE="${GITEA_NAMESPACE:-gitea}"
export GIT_USER="${GIT_USER:-ci}"
export GIT_PASSWORD="${GIT_PASSWORD:-ci-Pass123}"
export GIT_EMAIL="${GIT_EMAIL:-ci@greenops.local}"
export REPO_NAME="${REPO_NAME:-greenops}"
# Host URL (NodePort, used to seed the repo) and in-cluster URL (used by ArgoCD).
export GITEA_HOST_URL="${GITEA_HOST_URL:-http://localhost:30083}"
export GITEA_INCLUSTER_URL="${GITEA_INCLUSTER_URL:-http://gitea-http.${GITEA_NAMESPACE}.svc.cluster.local:3000}"

# GitOps source for ArgoCD. Defaults to the in-cluster Gitea repo; override
# REPO_URL/REVISION to point ArgoCD at an external Git remote instead.
export REPO_URL="${REPO_URL:-${GITEA_INCLUSTER_URL}/${GIT_USER}/${REPO_NAME}.git}"
export REVISION="${REVISION:-main}"

export ARGOCD_NAMESPACE="${ARGOCD_NAMESPACE:-argocd}"
# Pin a known-good ArgoCD release; override as needed.
export ARGOCD_MANIFEST="${ARGOCD_MANIFEST:-https://raw.githubusercontent.com/argoproj/argo-cd/v2.13.2/manifests/install.yaml}"

log()  { printf '\033[1;36m[greenops]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[greenops]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[greenops]\033[0m %s\n' "$*" >&2; exit 1; }

require() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }
