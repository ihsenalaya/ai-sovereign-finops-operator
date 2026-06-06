#!/usr/bin/env bash
# Full self-contained GitOps bring-up:
# kind + image + ArgoCD + in-cluster Gitea (seeded) + Applications + wait sync.
# No external Git remote and no manual step required.
set -euo pipefail
DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${DIR}/common.sh"

"${DIR}/01-create-cluster.sh"
"${DIR}/02-build-load-image.sh"
"${DIR}/03-install-argocd.sh"
"${DIR}/03b-install-gitea.sh"
"${DIR}/04-bootstrap-apps.sh"
"${DIR}/05-wait-sync.sh"

log "GitOps bring-up complete."
log "ArgoCD UI: http://localhost:30080  (user: admin)"
log "  password: kubectl -n ${ARGOCD_NAMESPACE} get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
log "Gitea UI:  http://localhost:30083  (user: ${GIT_USER})"
