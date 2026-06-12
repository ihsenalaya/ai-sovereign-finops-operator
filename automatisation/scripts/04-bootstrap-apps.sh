#!/usr/bin/env bash
# Register the Git repo in ArgoCD and create the AppProject + Applications.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kubectl

log "registering repository in ArgoCD: ${REPO_URL}"
# Repo credentials secret is only needed for authenticated remotes.
if [ "${USE_GITEA}" = "true" ]; then
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: greenops-repo
  namespace: ${ARGOCD_NAMESPACE}
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: ${REPO_URL}
  username: ${GIT_USER}
  password: ${GIT_PASSWORD}
EOF
elif [ -n "${REPO_USERNAME}" ] && [ -n "${REPO_PASSWORD}" ]; then
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: greenops-repo
  namespace: ${ARGOCD_NAMESPACE}
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: ${REPO_URL}
  username: ${REPO_USERNAME}
  password: ${REPO_PASSWORD}
EOF
else
  kubectl -n "${ARGOCD_NAMESPACE}" delete secret greenops-repo --ignore-not-found >/dev/null
  log "repository is treated as public/read-only; no ArgoCD repo secret created."
fi

render() {
  sed -e "s|__REPO_URL__|${REPO_URL}|g" \
      -e "s|__REVISION__|${REVISION}|g" \
      -e "s|__IMAGE_REPO__|${IMAGE_REPO}|g" \
      -e "s|__IMAGE_TAG__|${IMAGE_TAG}|g" \
      "$1"
}

kubectl apply -f "${AUTOMATISATION_DIR}/argocd/project.yaml"
render "${AUTOMATISATION_DIR}/argocd/application-operator.yaml" | kubectl apply -f -
render "${AUTOMATISATION_DIR}/argocd/application-samples.yaml"  | kubectl apply -f -

log "Applications registered. ArgoCD will sync from ${REPO_URL}@${REVISION}."
log "Watch with: kubectl -n ${ARGOCD_NAMESPACE} get applications"
