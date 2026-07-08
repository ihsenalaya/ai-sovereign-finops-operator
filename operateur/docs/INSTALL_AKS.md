# Installation sur AKS (Azure Kubernetes Service)

L'opérateur est cloud-agnostique ; AKS est cité car la cible (entreprises FR/EU régulées) utilise
souvent Azure (région `francecentral`, Azure OpenAI). Stack de déploiement : **Helm** (CNCF).

## Prérequis
`az`, `kubectl`, `helm`, accès au registre GHCR du projet.

## 1. Cluster & contexte
```bash
az aks create -g <rg> -n <cluster> --location francecentral --node-count 2 --generate-ssh-keys
az aks get-credentials -g <rg> -n <cluster>
```

## 2. Image
```bash
export REGISTRY=ghcr.io/ihsenalaya/ai-sovereign-finops-operator
docker login ghcr.io
make build-images REGISTRY=$REGISTRY VERSION=0.5.11
make push-images REGISTRY=$REGISTRY VERSION=0.5.11 PUSH=true
```

## 3. Déploiement Helm
```bash
helm upgrade --install greenops operateur/charts/ai-sovereign-finops-operator \
  -n greenops-system --create-namespace \
  --set image.repository=ghcr.io/ihsenalaya/ai-sovereign-finops-operator/controller \
  --set image.tag=0.5.11
```
> Sur AKS, laisser `image.pullPolicy=IfNotPresent` (défaut) — ne pas utiliser `Never` (réservé à kind).

Pour l'article 1, les résultats papier doivent provenir d'AKS réel SEV-SNP.
Les démos `kind` restent uniquement CI/debug/régression.

## 4. Catalogue & policies
Adapter `operateur/config/samples` à votre contexte (provider `azure-openai`, `dataResidency: france`,
pricing réel) puis `kubectl apply -k operateur/config/samples/`.

## 5. Observabilité
Activer le `ServiceMonitor` si Prometheus Operator est présent :
`--set metrics.serviceMonitor.enabled=true`. Importer `operateur/dashboards/ai-finops-overview.json` dans Grafana.

## GitOps
Pour un déploiement piloté par ArgoCD, voir [automatisation/README.md](../../automatisation/README.md)
(adapter `destination.server` vers le cluster AKS).
