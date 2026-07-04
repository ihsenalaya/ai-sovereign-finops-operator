#!/usr/bin/env bash
set -euo pipefail

REGISTRY=""
VERSION="${VERSION:-0.5.4}"
NAMESPACE="${NAMESPACE:-ai-platform}"
CLUSTER="${CLUSTER:-ai-platform}"
SKIP_BUILD="${SKIP_BUILD:-false}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OPERATEUR="$ROOT/operateur"
CHART="$ROOT/charts/ai-confidential-governance-platform"

log() { echo "[kind-deploy] $*"; }

# ── 1. Cluster ────────────────────────────────────────────────────────────────
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER}$"; then
  log "Creating kind cluster '$CLUSTER'..."
  kind create cluster --name "$CLUSTER" --config "$ROOT/deploy/kind-config.yaml" --wait 60s
else
  log "Cluster '$CLUSTER' already exists — skipping creation."
fi

# Point kubectl to this cluster
kubectl config use-context "kind-${CLUSTER}"

# ── 2. Build images ───────────────────────────────────────────────────────────
# Built sequentially to avoid OOM on RAM-constrained hosts (WSL2 ~7 GiB).
if [ "$SKIP_BUILD" != "true" ]; then
  log "Building Docker images (tag: $VERSION) — sequential to avoid OOM..."

  docker build -t "controller:${VERSION}" \
    -f "$OPERATEUR/Dockerfile" "$OPERATEUR"

  docker build -t "attestation-scheduler:${VERSION}" \
    -f "$OPERATEUR/Dockerfile.scheduler" "$OPERATEUR"

  docker build -t "key-release-gateway:${VERSION}" \
    -f "$OPERATEUR/Dockerfile.key-release-gateway" "$OPERATEUR"

  docker build -t "platform-api:${VERSION}" \
    -f "$OPERATEUR/Dockerfile.platform-api" "$OPERATEUR"

  docker build -t "thesis-bench:${VERSION}" \
    -f "$OPERATEUR/Dockerfile.thesis-bench" "$OPERATEUR"

  # UI — platform-ui uses nginxinc/nginx-unprivileged (port 8080, no CAP_CHOWN needed)
  UI_DIR="$ROOT/platform-ui"
  if [ -f "$UI_DIR/src/main.tsx" ]; then
    log "Building platform-ui..."
    docker build -t "platform-ui:${VERSION}" "$UI_DIR"
  else
    log "Skipping platform-ui build (no src/main.tsx found) — using nginx-unprivileged placeholder."
    docker pull nginxinc/nginx-unprivileged:1.27-alpine
    docker tag nginxinc/nginx-unprivileged:1.27-alpine "platform-ui:${VERSION}"
  fi
else
  log "SKIP_BUILD=true — skipping image builds."
fi

# ── 3. Load images into kind ──────────────────────────────────────────────────
log "Loading images into kind cluster '$CLUSTER'..."
for img in controller attestation-scheduler key-release-gateway platform-api platform-ui thesis-bench; do
  kind load docker-image "${img}:${VERSION}" --name "$CLUSTER"
done

# ── 4. Apply CRDs (Helm does not update existing CRDs on upgrade) ─────────────
log "Applying CRDs..."
kubectl apply -f "$CHART/crds/"

# ── 5. Namespace ──────────────────────────────────────────────────────────────
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# ── 6. Deploy ─────────────────────────────────────────────────────────────────
log "Deploying Helm chart (namespace: $NAMESPACE)..."
helm upgrade --install ai-platform "$CHART" \
  -f "$CHART/values-kind.yaml" \
  --set images.tag="$VERSION" \
  --namespace "$NAMESPACE" \
  --wait --timeout 5m

# ── 7. Status ─────────────────────────────────────────────────────────────────
log ""
log "=== Deployment status ==="
kubectl get pods -n "$NAMESPACE"

log ""
log "=== Access ==="
log "  UI:      kubectl port-forward svc/platform-ui 8090:80 -n $NAMESPACE  → http://localhost:8090"
log "  API:     kubectl port-forward svc/platform-api 8083:8083 -n $NAMESPACE"
log "  Gateway: kubectl port-forward svc/key-release-gateway 8082:8082 -n $NAMESPACE"
log ""
log "Done."
