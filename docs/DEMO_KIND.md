# Démo locale sur kind

Prérequis : `go`, `kubectl`, `kind`, `helm`. Machine contrainte en RAM ? Réutilisez un cluster
existant plutôt que d'en créer un nouveau (voir note RAM en bas).

## 1. Cluster

```bash
kind create cluster --name greenops      # ou réutiliser: kind export kubeconfig --name <cluster>
kubectl cluster-info
```

## 2. Installer les CRDs

```bash
make install
kubectl get crds | grep imperium
```

## 3. Lancer l'opérateur

Option A — local (le plus léger, idéal kind/WSL) :

```bash
make run     # tourne en avant-plan ; Ctrl-C pour arrêter
```

Option B — déployé dans le cluster (image) :

```bash
make docker-build IMG=ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.9
kind load docker-image ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.9 --name greenops
make deploy IMG=ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.9
# ou via Helm :
helm install greenops charts/ai-sovereign-finops-operator \
  --set image.repository=ghcr.io/ihsenalaya/ai-sovereign-finops-operator \
  --set image.tag=0.3.9 \
  --set image.pullPolicy=Never
```

## 4. Appliquer les exemples

```bash
kubectl apply -k config/samples/
kubectl get aigw,aiprov,aimodel,aibudget,aisov,aibreakeven,aireport
```

Attendu : toutes les ressources `Ready=True`. `AIModel/mistral-small` résout son provider
(`resolvedProvider=azure-openai`). `AIBreakEvenAnalysis` calcule un `recommendation` et un
`paybackMonths`. `AIFinOpsReport` peuple coûts/tokens/recommandations dans `.status`.

## 5. Inspecter

```bash
kubectl describe aireport monthly-ai-report-rh
kubectl get aibreakeven chatbot-rh-analysis -o yaml | yq '.status'
kubectl get events --sort-by=.lastTimestamp | tail
```

## 6. Métriques & dashboard

```bash
kubectl port-forward deploy/ai-sovereign-finops-operator-controller-manager 8080:8080
curl -s localhost:8080/metrics | grep ai_finops_
# Importer dashboards/ai-finops-overview.json dans Grafana.
```

## 7. Nettoyage

```bash
kubectl delete -k config/samples/
make uninstall
kind delete cluster --name greenops    # si créé pour la démo
```

---

### Note RAM (WSL2 ~7 GiB)

Créer un nouveau cluster + builder une image peut saturer la RAM. Préférez `make run` (manager local)
contre un cluster kind déjà démarré ; ne builder l'image que pour valider le chemin Helm. Si besoin,
augmentez la RAM WSL2 (`.wslconfig`).
