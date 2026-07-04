# AKS Private Deployment

## Prerequisites

- AKS cluster with:
  - Private cluster networking (no public API server endpoint)
  - Azure CNI or Azure CNI Overlay
  - Node pool with confidential VM SKU (future — see GPU validation doc)
  - Azure Key Vault with CSI driver enabled (for secrets injection)
  - Internal load balancer only (`service.beta.kubernetes.io/azure-load-balancer-internal: "true"`)
- Azure Container Registry (ACR) linked to the AKS cluster
- Helm 3.14+
- kubectl with cluster access

## Image registry

All images must be pushed to ACR before deployment:

```bash
export REGISTRY=myacr.azurecr.io
export VERSION=0.5.4

make build-images REGISTRY=$REGISTRY VERSION=$VERSION
make push-images REGISTRY=$REGISTRY VERSION=$VERSION PUSH=true
```

## Secrets management

**NEVER** put secrets in `values-aks-private.yaml` or any file committed to Git.

Required secrets — inject via Azure Key Vault CSI driver or Sealed Secrets:

| Secret | Description | Reference |
|---|---|---|
| `scheduler-signing-key` | Ed25519 private key hex for placement token signing | `SCHEDULER_SIGNING_KEY_HEX` |
| `token-public-key` | Ed25519 public key hex for gateway verification | `TOKEN_PUBLIC_KEY_HEX` |
| OIDC config | Client ID + tenant ID for platform-api auth | `platformApi.oidc.*` |

Generate the Ed25519 key pair once and store in AKV:
```bash
# One-time: generate key pair
go run ./operateur/cmd/attestation-scheduler/... --generate-key-only
# Outputs two hex strings: PRIVATE_KEY and PUBLIC_KEY

# Store in Azure Key Vault
az keyvault secret set --vault-name <vault> --name scheduler-signing-key --value "<PRIVATE_KEY>"
az keyvault secret set --vault-name <vault> --name token-public-key --value "<PUBLIC_KEY>"
```

## Deployment

```bash
helm upgrade --install ai-platform \
  charts/ai-confidential-governance-platform \
  -f charts/ai-confidential-governance-platform/values-aks-private.yaml \
  --set global.registry=myacr.azurecr.io \
  --namespace ai-platform \
  --create-namespace \
  --wait --timeout 10m
```

## Validation checklist

After deployment:
```bash
# No LoadBalancer services
kubectl get svc -n ai-platform | grep LoadBalancer
# Expected: no output

# Simulated evidence not active
kubectl get configmap -n ai-platform -o yaml | grep simulatedEvidence
# Expected: false

# NetworkPolicies applied
kubectl get networkpolicy -n ai-platform
# Expected: at least one policy

# API auth active (should reject unauthenticated requests)
curl -f http://<internal-platform-api>:8083/api/v1/overview
# Expected: 401 Unauthorized

# Scheduler running
kubectl get pod -n ai-platform -l component=attestation-scheduler
```

## Ingress (internal)

The platform UI and API are ClusterIP only. Access via:
- Internal nginx ingress with annotation `service.beta.kubernetes.io/azure-load-balancer-internal: "true"`
- VPN or bastion host for direct access
- No public endpoint should be created

See the commented ingress block in `values-aks-private.yaml`.

## Audit anchoring

The production audit anchoring backend is planned (Azure Blob). In the interim, use the file backend with a persistent volume backed by Azure Disk (ZRS for HA):

```yaml
# In your deployment values overlay:
global:
  audit:
    anchoring:
      mode: file
      filePath: /data/audit/checkpoints.jsonl
# Mount a PVC backed by Azure Disk at /data/audit
```

Switch to `azure-blob` mode when that backend is implemented.
