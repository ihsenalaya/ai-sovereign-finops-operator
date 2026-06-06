# automatisation/ — déploiement end-to-end (kind + ArgoCD)

Ce dossier crée un cluster **kind**, le configure et déploie l'opérateur **de bout en bout
via ArgoCD (GitOps)** — plus un chemin **offline** (Helm direct) sans ArgoCD.

## Prérequis

`docker`, `kind`, `kubectl`, `helm`, et (pour le mode GitOps) `git`. Tout est CNCF/OSS.

## Deux chemins

### A. GitOps avec ArgoCD (recommandé pour la démo « plateforme »)

ArgoCD tire la configuration depuis un dépôt Git. Poussez d'abord ce repo sur un remote
accessible par le cluster, puis :

```bash
cd automatisation
make up REPO_URL=https://github.com/<vous>/greenops REVISION=main
```

`make up` enchaîne :
1. `01-create-cluster.sh` — crée le cluster kind (`kind/kind-config.yaml`).
2. `02-build-load-image.sh` — build l'image `greenops:dev` et la charge dans kind.
3. `03-install-argocd.sh` — installe ArgoCD, expose l'UI sur `http://localhost:30080`.
4. `04-bootstrap-apps.sh` — crée l'`AppProject` + 2 `Application` :
   - `greenops-operator` → chart Helm `charts/ai-sovereign-finops-operator` (image locale, `pullPolicy: Never`),
   - `greenops-samples` → `config/samples` (catalogue + policies, sync-wave 1).

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

## Variables (override possible)

| Variable | Défaut | Rôle |
|----------|--------|------|
| `CLUSTER_NAME` | `greenops` | nom du cluster kind |
| `IMAGE_REPO` / `IMAGE_TAG` | `greenops` / `dev` | image de l'opérateur |
| `REPO_URL` | remote `origin` ou placeholder | source Git pour ArgoCD |
| `REVISION` | branche courante | révision Git ciblée |
| `ARGOCD_MANIFEST` | v2.13.2 install.yaml | version d'ArgoCD |

## Teardown

```bash
make down        # supprime le cluster kind
```

## Note RAM

ArgoCD + l'opérateur tiennent sur un kind mono-nœud, mais sur une machine ~7 GiB (WSL2)
gardez les autres clusters éteints. En cas de pression mémoire, préférez `make local`
(sans ArgoCD). Voir `docs/DEMO_KIND.md`.
