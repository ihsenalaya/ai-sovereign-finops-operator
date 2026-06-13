# automatisation/ — déploiement end-to-end (kind + ArgoCD)

Ce dossier crée un cluster **kind**, le configure et déploie l'opérateur **de bout en bout
via ArgoCD (GitOps)** — plus un chemin **offline** (Helm direct) sans ArgoCD.

## Prérequis

`docker`, `kind`, `kubectl`, `helm`, et (pour le mode GitOps) `git`. Tout est CNCF/OSS.

## Trois chemins

### A. GitOps avec ArgoCD (recommandé pour la démo « plateforme »)

ArgoCD tire la configuration depuis un dépôt Git. Par défaut, `make up` utilise le
remote `origin` du workspace courant. Vous pouvez aussi surcharger `REPO_URL` /
`REVISION` explicitement :

```bash
cd automatisation
make up
# ou:
make up REPO_URL=https://github.com/<vous>/greenops REVISION=main
```

`make up` enchaîne :
1. `01-create-cluster.sh` — crée le cluster kind (`kind/kind-config.yaml`).
2. `02-build-load-image.sh` — build l'image `ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.7` et la charge dans kind.
3. `03-install-argocd.sh` — installe ArgoCD, expose l'UI sur `http://localhost:30080`.
4. `04-bootstrap-apps.sh` — crée l'`AppProject` + 2 `Application` :
   - `greenops-operator` → chart Helm `charts/ai-sovereign-finops-operator` (image locale, `pullPolicy: Never`),
   - `greenops-samples` → `config/samples` (catalogue + policies, sync-wave 1).

Si vous voulez malgré tout un dépôt Git in-cluster auto-contenu :

```bash
cd automatisation
make up-gitea
```

Mot de passe admin ArgoCD :
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
```

Suivi de la synchro :
```bash
kubectl -n argocd get applications
```

### B. Offline (Helm direct, sans ArgoCD)

Chemin **validé en développement**, idéal machine contrainte / sans remote Git :

```bash
cd automatisation
make local
```

→ kind + image + `helm install` + samples, puis affiche les commandes d'inspection.

### C. Démo réelle complète (zéro étape manuelle)

Ce chemin ne seed rien et ne simule rien: de vraies apps tournent dans le cluster,
passent par Envoy AI Gateway, le webhook de l'opérateur injecte les sidecars
d’attribution, Tetragon capture le shadow-AI qui contourne le gateway, et Grafana
affiche les vraies métriques `ai_finops_*`.

```bash
cd automatisation
make real-demo
make real-demo-reset
make real-demo-test
make real-demo-down
```

`make real-demo` enchaîne automatiquement : cluster kind, image opérateur,
Helm operator, Envoy Gateway, Envoy AI Gateway, catalogue Cohere + Mistral, les
4 apps `rh/finance/legal/marketing`, Prometheus/Grafana, Tetragon, le workload
`finance/shadow-ai-rogue`, puis le refresh du ConfigMap `shadow-egress`.

Pour une exécution strictement sans état précédent, utilisez `make real-demo-reset` :
la cible supprime d'abord le cluster kind `greenops`, le recrée, puis relance toute
la stack de bout en bout.

Important : ce chemin requiert une **clé Azure AI Foundry réellement utilisable**
dans `docs/foundrykey.txt` (ou `docs/mistralkey.txt` pour compatibilité). Le
script fait un préflight Cohere et Mistral avant de démarrer, pour éviter une
démo verte avec des workloads en erreur fournisseur.

Ce chemin s’appuie sur [`envoy-aigw/deploy.sh`](envoy-aigw/deploy.sh) et
[`tetragon/demo.sh`](tetragon/demo.sh).

## Variables (override possible)

| Variable | Défaut | Rôle |
|----------|--------|------|
| `CLUSTER_NAME` | `greenops` | nom du cluster kind |
| `KIND_NODE_IMAGE` | `kindest/node:v1.31.0` | image Kubernetes utilisée par kind |
| `IMAGE_REPO` / `IMAGE_TAG` | `ghcr.io/ihsenalaya/ai-sovereign-finops-operator` / `0.3.7` | image de l'opérateur |
| `ENABLE_MISTRAL_DEMO` | `true` | active la 4e app `marketing/content-writer` sur Mistral EU |
| `ENABLE_TETRAGON` / `ENABLE_SHADOW_ROGUE` | `true` / `true` | installe Tetragon et lance le workload shadow-AI |
| `REQUIRE_SHADOW_EGRESS` | `true` | fait échouer la démo si `shadow-egress` reste vide |
| `REAL_DEMO_ISOLATE_GITOPS` | `true` | désactive l'auto-sync ArgoCD existant et nettoie les anciens samples avant la démo réelle |
| `RESET_CLUSTER` | `false` | supprime/recrée le cluster avant la démo (`make real-demo-reset`) |
| `REPO_URL` | remote `origin` ou repo Gitea in-cluster | source Git pour ArgoCD |
| `REVISION` | branche courante | révision Git ciblée |
| `GITOPS_SOURCE` | `auto` | `auto` (origin), `gitea` (repo in-cluster) |
| `ARGOCD_MANIFEST` | v2.13.2 install.yaml | version d'ArgoCD |

## Teardown

```bash
make down        # supprime le cluster kind
```

## Note RAM

ArgoCD + l'opérateur tiennent sur un kind mono-nœud, mais sur une machine ~7 GiB (WSL2)
gardez les autres clusters éteints. En cas de pression mémoire, préférez `make local`
(sans ArgoCD). Voir `docs/DEMO_KIND.md`.
