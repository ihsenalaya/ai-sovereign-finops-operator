#!/usr/bin/env bash
# Full GitOps bring-up:
# kind + image + ArgoCD + Applications + wait sync.
# Defaults to the workspace origin remote; use GITOPS_SOURCE=gitea for a
# self-contained in-cluster Git server.
set -euo pipefail
DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${DIR}/common.sh"

"${DIR}/01-create-cluster.sh"
"${DIR}/02-build-load-image.sh"
"${DIR}/03-install-argocd.sh"
if [ "${USE_GITEA}" = "true" ]; then
  "${DIR}/03b-install-gitea.sh"
else
  log "using external GitOps source ${REPO_URL}@${REVISION}"
fi
"${DIR}/04-bootstrap-apps.sh"
"${DIR}/05-wait-sync.sh"

log "GitOps bring-up complete."
log "ArgoCD UI: http://localhost:30080  (user: admin)"
log "  password: kubectl -n ${ARGOCD_NAMESPACE} get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
if [ "${USE_GITEA}" = "true" ]; then
  log "Gitea UI:  http://localhost:30083  (user: ${GIT_USER})"
fi
