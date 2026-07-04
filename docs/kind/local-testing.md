# Local Testing with kind

## Prerequisites

```bash
make bootstrap-tools
```

Required:
- `kind` (v0.22+)
- `kubectl`
- `helm` (v3.14+)
- `go` 1.21+
- `docker` or `containerd`

Optional but recommended:
- `govulncheck`, `gosec`, `trivy` (security scanning)

## Quick start

```bash
# 1. Create kind cluster
make kind-up

# 2. Verify Go code
make lint test-unit

# 3. Deploy platform
make deploy-kind

# 4. Wait for rollout
kubectl rollout status deploy -n ai-platform

# 5. Access the UI (nginx-unprivileged runs on 8080 internally)
kubectl port-forward svc/platform-ui 8090:80 -n ai-platform &
open http://localhost:8090

# 6. Access the API
kubectl port-forward svc/platform-api 8083:8083 -n ai-platform &
curl http://localhost:8083/healthz
```

## Running thesis bench

```bash
make thesis-bench
```

Results are written to `operateur/results/thesis/latest/`:
- `results.json` — machine-readable
- `results.csv` — for statistical tools
- `report.md` — human-readable summary

Exit code 1 if any attacks are not blocked.

## Running unit tests

```bash
# All tests
make test

# Only unit tests (no envtest, no cluster)
make test-unit

# Scheduler + token + audit + crypto
make test-scheduler

# With verbose output
cd operateur && go test -v ./pkg/... ./internal/...
```

## Environment flags

| Variable | Default | Effect |
|---|---|---|
| `AIOPS_PLATFORM_MODE` | (empty) | Set to `production` to block simulated RuntimeClass |
| `AIOPS_INSECURE_AUTH` | (empty) | Set to `true` to skip token auth on platform-api |
| `SCHEDULER_SIGNING_KEY_HEX` | (empty) | Hex-encoded Ed25519 private key; auto-generated if absent |
| `TOKEN_PUBLIC_KEY_HEX` | (empty) | Hex-encoded Ed25519 public key for token verification |

In kind, these are injected via Helm values or set in the pod environment. See `values-kind.yaml`.

## Port layout (kind)

| Port local | Service | Commande port-forward |
|---|---|---|
| 8090 | platform-ui | `kubectl port-forward svc/platform-ui 8090:80 -n ai-platform` |
| 8083 | platform-api | `kubectl port-forward svc/platform-api 8083:8083 -n ai-platform` |
| 8082 | key-release-gateway | `kubectl port-forward svc/key-release-gateway 8082:8082 -n ai-platform` |
| 8080 | operator metrics | `kubectl port-forward svc/governance-operator 8080:8080 -n ai-platform` |

> Note: platform-ui utilise `nginxinc/nginx-unprivileged:1.27-alpine` qui écoute sur le port 8080 en interne (pas 80). Le Service Kubernetes expose le port 80 → targetPort 8080.

## Tearing down

```bash
make undeploy-kind
make kind-down
```

## Common issues

**Pods stuck in Pending with SchedulingGate**
The scheduling gate `aiops.imperium.io/attestation-evidence` is added by the webhook. The ai-attestation-scheduler removes it after placement. Check that the scheduler pod is running:
```bash
kubectl get pod -n ai-platform -l component=attestation-scheduler
kubectl logs -n ai-platform -l component=attestation-scheduler
```

**Webhook not mutating pods**
Check that the webhook service is reachable and cert is valid:
```bash
kubectl get mutatingwebhookconfiguration
kubectl get validatingwebhookconfiguration
kubectl logs -n ai-platform -l component=governance-operator
```

**thesis-bench exit code 1**
One or more attack scenarios were not blocked. Read `report.md` to identify which attacks passed. This is a correctness failure — the blocking logic needs investigation.

**attestation-scheduler in CrashLoopBackOff — healthz returns 404**
controller-runtime requires explicit health check registration. The manager must call:
```go
mgr.AddHealthzCheck("healthz", healthz.Ping)
mgr.AddReadyzCheck("readyz", healthz.Ping)
```
Without this, `/healthz` and `/readyz` return HTTP 404 and the liveness probe kills the pod.

**platform-ui in CrashLoopBackOff — "chown: operation not permitted"**
`nginx:alpine` tries to `chown` cache directories at startup but `CAP_CHOWN` is dropped in the pod security context. The Dockerfile must use `nginxinc/nginx-unprivileged:1.27-alpine` (runs as non-root, port 8080).

**Docker OOM during image build**
Building all 6 images in parallel exhausts ~7.4GB RAM on WSL2. Build them sequentially (`02-build-load-image.sh` handles this). If Docker crashes mid-build:
```bash
docker system prune -f   # free ~7GB of layer cache
```

**kind API timeout after Docker Desktop restart**
If Docker Desktop restarts, the kind cluster may become unreachable. Delete and recreate:
```bash
kind delete cluster --name greenops
kind create cluster --config automatisation/kind/kind-config.yaml --name greenops
./automatisation/scripts/02-build-load-image.sh
./automatisation/scripts/07-install-confidential-platform.sh
```
