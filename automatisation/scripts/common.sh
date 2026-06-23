#!/usr/bin/env bash
# Shared variables and helpers for the automatisation scripts.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AUTOMATISATION_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${AUTOMATISATION_DIR}/.." && pwd)"
OPERATOR_DIR="${REPO_ROOT}/operateur"
export OPERATOR_DIR

git_origin_url() {
  local url
  url="$(git -C "${REPO_ROOT}" remote get-url origin 2>/dev/null || true)"
  case "${url}" in
    git@github.com:*)
      printf 'https://github.com/%s\n' "${url#git@github.com:}"
      ;;
    ssh://git@github.com/*)
      printf 'https://github.com/%s\n' "${url#ssh://git@github.com/}"
      ;;
    *)
      printf '%s\n' "${url}"
      ;;
  esac
}

git_default_revision() {
  local branch
  branch="$(git -C "${REPO_ROOT}" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  if [ -n "${branch}" ] && [ "${branch}" != "HEAD" ]; then
    printf '%s\n' "${branch}"
    return
  fi
  git -C "${REPO_ROOT}" rev-parse HEAD 2>/dev/null || printf 'main\n'
}

# Cluster / image
export CLUSTER_NAME="${CLUSTER_NAME:-greenops}"
export KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.31.0}"
export IMAGE_REPO="${IMAGE_REPO:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator}"
export IMAGE_TAG="${IMAGE_TAG:-0.4.0}"

# In-cluster Gitea (self-contained GitOps source — no external remote required).
export GITEA_NAMESPACE="${GITEA_NAMESPACE:-gitea}"
export GIT_USER="${GIT_USER:-ci}"
export GIT_PASSWORD="${GIT_PASSWORD:-ci-Pass123}"
export GIT_EMAIL="${GIT_EMAIL:-ci@greenops.local}"
export REPO_NAME="${REPO_NAME:-greenops}"
# Host URL (NodePort, used to seed the repo) and in-cluster URL (used by ArgoCD).
export GITEA_HOST_URL="${GITEA_HOST_URL:-http://localhost:30083}"
export GITEA_INCLUSTER_URL="${GITEA_INCLUSTER_URL:-http://gitea-http.${GITEA_NAMESPACE}.svc.cluster.local:3000}"
export GITEA_REPO_URL="${GITEA_REPO_URL:-${GITEA_INCLUSTER_URL}/${GIT_USER}/${REPO_NAME}.git}"
export GITOPS_SOURCE="${GITOPS_SOURCE:-auto}"

# GitOps source for ArgoCD. Defaults to the workspace origin remote when present,
# otherwise falls back to the in-cluster Gitea repo. Force Gitea with
# GITOPS_SOURCE=gitea.
DEFAULT_REPO_URL="$(git_origin_url)"
if [ -z "${DEFAULT_REPO_URL}" ] || [ "${GITOPS_SOURCE}" = "gitea" ]; then
  DEFAULT_REPO_URL="${GITEA_REPO_URL}"
fi
export REPO_URL="${REPO_URL:-${DEFAULT_REPO_URL}}"
export REVISION="${REVISION:-$(git_default_revision)}"
export USE_GITEA="${USE_GITEA:-false}"
if [ "${REPO_URL}" = "${GITEA_REPO_URL}" ]; then
  export USE_GITEA="true"
fi
export REPO_USERNAME="${REPO_USERNAME:-}"
export REPO_PASSWORD="${REPO_PASSWORD:-}"

export ARGOCD_NAMESPACE="${ARGOCD_NAMESPACE:-argocd}"
# Pin a known-good ArgoCD release; override as needed.
export ARGOCD_MANIFEST="${ARGOCD_MANIFEST:-https://raw.githubusercontent.com/argoproj/argo-cd/v2.13.2/manifests/install.yaml}"

log()  { printf '\033[1;36m[greenops]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[greenops]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[greenops]\033[0m %s\n' "$*" >&2; exit 1; }

require() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }
