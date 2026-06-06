#!/usr/bin/env bash
# Deploy an in-cluster Gitea, create the admin user + repo, and push this repo.
# This makes the GitOps flow fully self-contained (no external Git remote).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kubectl
require git
require curl

log "deploying in-cluster Gitea..."
kubectl apply -f "${AUTOMATISATION_DIR}/gitea/gitea.yaml"
kubectl -n "${GITEA_NAMESPACE}" rollout status deploy/gitea --timeout=240s

log "waiting for Gitea HTTP to answer on ${GITEA_HOST_URL}..."
for i in $(seq 1 60); do
  if curl -fsS "${GITEA_HOST_URL}/api/healthz" >/dev/null 2>&1; then break; fi
  sleep 3
  [ "$i" = "60" ] && die "Gitea did not become ready on ${GITEA_HOST_URL}"
done

log "creating admin user '${GIT_USER}' (idempotent)..."
kubectl -n "${GITEA_NAMESPACE}" exec deploy/gitea -- \
  gitea admin user create --username "${GIT_USER}" --password "${GIT_PASSWORD}" \
  --email "${GIT_EMAIL}" --admin --must-change-password=false 2>/dev/null \
  || log "admin user already exists; continuing."

log "creating repo '${GIT_USER}/${REPO_NAME}' (idempotent)..."
curl -fsS -o /dev/null -X POST "${GITEA_HOST_URL}/api/v1/user/repos" \
  -u "${GIT_USER}:${GIT_PASSWORD}" -H 'Content-Type: application/json' \
  -d "{\"name\":\"${REPO_NAME}\",\"private\":false,\"default_branch\":\"main\"}" \
  || log "repo already exists; continuing."

log "pushing working tree to Gitea..."
push_url="http://${GIT_USER}:${GIT_PASSWORD}@localhost:30083/${GIT_USER}/${REPO_NAME}.git"
git -C "${REPO_ROOT}" add -A
if ! git -C "${REPO_ROOT}" diff --cached --quiet; then
  git -C "${REPO_ROOT}" -c user.email="${GIT_EMAIL}" -c user.name="${GIT_USER}" \
    commit -q -m "greenops snapshot for GitOps deploy"
fi
git -C "${REPO_ROOT}" push -f "${push_url}" HEAD:refs/heads/main

log "repo available in-cluster at ${REPO_URL}"
