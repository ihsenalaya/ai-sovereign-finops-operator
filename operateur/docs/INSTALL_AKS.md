# Installation sur AKS (Azure Kubernetes Service)

L'opérateur est cloud-agnostique ; AKS est cité car la cible (entreprises FR/EU régulées) utilise
souvent Azure (région `francecentral`, Azure OpenAI). Stack de déploiement : **Helm** (CNCF).

## Prérequis
`az`, `kubectl`, `helm`, un registre (ACR ou ghcr.io).

## 1. Cluster & contexte
```bash
az aks create -g <rg> -n <cluster> --location francecentral --node-count 2 --generate-ssh-keys
az aks get-credentials -g <rg> -n <cluster>
```

## 2. Image
```bash
# Avec ACR :
az acr login -n <acr>
docker build -t <acr>.azurecr.io/ai-sovereign-finops-operator:0.5.4 operateur
docker push <acr>.azurecr.io/ai-sovereign-finops-operator:0.5.4
```

## 3. Déploiement Helm
```bash
helm upgrade --install greenops operateur/charts/ai-sovereign-finops-operator \
  -n greenops-system --create-namespace \
  --set image.repository=<acr>.azurecr.io/ai-sovereign-finops-operator \
  --set image.tag=0.5.4
```
> Sur AKS, laisser `image.pullPolicy=IfNotPresent` (défaut) — ne pas utiliser `Never` (réservé à kind).

## 4. Catalogue & policies
Adapter `operateur/config/samples` à votre contexte (provider `azure-openai`, `dataResidency: france`,
pricing réel) puis `kubectl apply -k operateur/config/samples/`.

## 5. Observabilité
Activer le `ServiceMonitor` si Prometheus Operator est présent :
`--set metrics.serviceMonitor.enabled=true`. Importer `operateur/dashboards/ai-finops-overview.json` dans Grafana.

## GitOps
Pour un déploiement piloté par ArgoCD, voir [automatisation/README.md](../../automatisation/README.md)
(adapter `destination.server` vers le cluster AKS).
