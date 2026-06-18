# Démo réelle — apps bornées consommant des tokens via Envoy AI Gateway

Exemple end-to-end **100 % réel** : des applications déployées dans le cluster
font des appels bornés à de vrais LLM **à travers Envoy AI Gateway** (projet CNCF).
La gateway **mesure les vrais tokens et la durée des requêtes** ; l'opérateur les **lit** et calcule
**coût, souveraineté, budget et score de latence par application**, exposés dans **Grafana**. Le scénario
valide aussi le plan shadow-AI : un workload contourne la gateway et Tetragon
alimente `shadow-egress` avec des événements réels.

```
 rh/finance/legal ──► Envoy AI Gateway ──► Azure Foundry Cohere (global)
 marketing ────────► Envoy AI Gateway ──► Azure Foundry Mistral (EU)
 finance/rogue ────► api.openai.com direct ──► Tetragon ──► shadow-egress

 Envoy gen_ai_* metrics ──► operator collector "aigw" ──► cost/sovereignty/budget/latency score
```

## Versions (IMPORTANT, testées)
- **Envoy Gateway `v1.5.0`** — ⚠️ `v1.3.0` est trop ancien : l'ext_proc n'est pas
  injecté et le routage renvoie « No matching route found ».
- **Envoy AI Gateway `v0.7.0`**.
- helm OCI échoue si le credential-helper Docker WSL est utilisé → le script force
  `DOCKER_CONFIG=/tmp/emptydockercfg` (pull anonyme).

## Lancer

```bash
cd automatisation/envoy-aigw
./deploy.sh up        # laisse la démo vivante
./deploy.sh verify    # collecte les preuves, stoppe les apps, supprime kind
./deploy.sh test      # un vrai appel Cohere à travers la gateway (sanity)
./deploy.sh down      # retire la démo (laisse les control planes)
```

Depuis la racine `automatisation/`, le même scénario est disponible via :

```bash
make real-demo
make real-demo-verify
make real-demo-test
make real-demo-down
```

Prérequis : `kind`, `kubectl`, `helm`, Docker, et une clé Azure AI Foundry
réellement utilisable dans `docs/foundrykey.txt` (ou `docs/mistralkey.txt` pour
compatibilité). La présence d'une clé seule ne suffit pas : `deploy.sh` fait un
**préflight Cohere et Mistral** avant de démarrer.

`verify` est le mode recommandé pour une preuve reproductible sans laisser
d'app consommatrice tourner. Il borne les clients à environ une minute de trafic par app,
collecte les preuves, scale les apps à zéro, puis supprime le cluster kind.

## Fichiers
- `01-gateway-cohere.yaml` — GatewayClass/Gateway/EnvoyProxy + AIGatewayRoute
  pour Azure Foundry Cohere + AIServiceBackend + BackendSecurityPolicy
  (clé Foundry via Secret, créé par `deploy.sh`) + Backend Foundry + TLS.
- `02-metrics-and-catalog.yaml` — Service métriques de l'extproc (`:1064`),
  `AIGateway` en **mode `aigw`**, `AIProvider`/`AIModel` (prix réels par modèle,
  attribution `servesNamespace/Application/Team`), `AISovereigntyPolicy` (EU-only),
  `AIFinOpsReport`, `AIBudgetPolicy`.
- `03-consumer-apps.yaml` — Service data stable + 3 Deployments (rh/chatbot-rh,
  finance/risk-assistant, legal/contract-review) qui font des appels bornés.
- `05-mistral-eu.yaml` — **2ᵉ provider EU** : route Mistral (Azure AI Foundry, zone EU)
  via le schéma `AzureOpenAI` + catalogue `mistral-eu`/`mistral-large` + 4ᵉ app
  `marketing/content-writer`. Sert à **vérifier la souveraineté zone-aware** (l'app EU
  ne produit aucune violation de zone, contrairement au provider global).
  Prérequis : clé Foundry dans `docs/foundrykey.txt`.
- `deploy.sh` — installe et câble le tout (versions épinglées), avec mode `verify`.

## Comment l'opérateur lit les vrais tokens et la latence réelle
Envoy AI Gateway expose l'histogramme OpenTelemetry
`gen_ai_client_token_usage` (labels `gen_ai_request_model`, `gen_ai_token_type`
input/output ; `_sum` = tokens, `_count` = requêtes). Le collector **`aigw`**
(`internal/collectors/aigw`) scrape cet endpoint, et **attribue** chaque modèle à
l'app qui le consomme via le catalogue `AIModel` (`providerRef` + `serves*`).
Le coût = tokens réels × prix réel du modèle (sur l'`AIProvider`).

Pour la latence, le même collector lit uniquement l'histogramme réel
`gen_ai_server_request_duration_seconds`. Si cette métrique n'est pas exposée,
l'opérateur continue à calculer un score de routage, mais marque explicitement
`latencyTelemetryAvailable=false` et n'émet pas `ai_finops_measured_latency_millis`.
Il n'y a pas de latence de catalogue présentée comme une mesure.

## Attribution automatique PAR NAMESPACE / APP (même modèle partagé)

La métrique `gen_ai_*` de la gateway ne porte que le **modèle** ; deux apps sur le
même modèle seraient donc fusionnées. La solution mise en place :

1. Le webhook de l'opérateur injecte un **sidecar HTTP proxy** dans les namespaces
   labellisés `aiops.imperium.io/sidecar-injection: "true"`. Le proxy ajoute
   automatiquement **`x-greenops-namespace`** et **`x-greenops-app`**.
2. Envoy AI Gateway est configuré (helm `controller.metricsRequestHeaderAttributes=
   x-greenops-namespace:k8s.namespace,x-greenops-app:k8s.app`) pour transformer ces
   en-têtes en **labels de métrique** (`k8s_namespace`, `k8s_app`).
3. Le collector **`aigw`** lit ces labels et attribue le coût **par namespace/app
   réel** (fallback sur `AIModel.serves*` si l'en-tête est absent).

➡️ Résultat : **`rh/chatbot-rh`, `finance/risk-assistant` et
`legal/contract-review` utilisent tous le même modèle Cohere, mais sont
comptabilisés séparément, par namespace/app** — automatiquement.
Déployer une nouvelle app dans un namespace opt-in suffit : aucune modification du
code applicatif ni ajout manuel d'en-têtes n'est nécessaire. Dans la démo, les pods
restreignent l'injection au host de la gateway via
`aiops.imperium.io/target-hosts=greenops-aigw.envoy-gateway-system.svc.cluster.local`.

## Ce que la démo vérifie en plus

- **Souveraineté** : les apps Cohere global produisent des findings ; l'app Mistral EU reste dans la zone EU.
- **Budget** : les budgets sont calculés sur les tokens réels observés, sans forcer de faux dépassement.
- **Shadow-AI** : `finance/shadow-ai-rogue` appelle `api.openai.com` directement sans clé; Tetragon capture l'egress réel.
- **Honnêteté de la latence** : le score de latence vient de
  `gen_ai_server_request_duration_seconds`. Le mode `verify` échoue si la démo réelle ne produit pas
  cette télémétrie, au lieu d'afficher une fausse latence. Les garde-fous d'erreur restent dépendants
  d'une source qui expose des erreurs observées.

## Comprendre les critères qui pilotent la démo

Pour savoir **quels paramètres** (prix, zones, budgets, politiques, apps/modèles)
produisent chaque chiffre des panneaux, voir **[`CRITERES.md`](CRITERES.md)** —
il relie chaque critère de configuration à la signification de chaque chart.

## Grafana — dashboard « AI Sovereign FinOps — Overview »

Ouvrir : `kubectl -n greenops-system port-forward svc/demo-grafana 3000:3000`
→ http://localhost:3000 (admin anonyme). Source : `dashboards/ai-finops-overview.json`.
Réconciliation opérateur toutes les **60 s** (dashboard vivant). Toutes les valeurs
proviennent du **vrai trafic** des apps via Envoy AI Gateway.

### Les panneaux principaux, un par un

| # | Panneau | Ce qu'il montre | Requête PromQL | Comment le lire |
|---|---------|-----------------|----------------|-----------------|
| 1 | **Total cost (EUR) — observed** | Dépense réelle cumulée depuis le démarrage de la gateway (tous apps) | `sum(ai_finops_cost_eur)` | Coût réel = tokens mesurés × prix de liste réels. Croît dans le temps. |
| 2 | **Tokens (total in + out)** | Volume total de tokens mesurés par la gateway | `sum(ai_finops_input_tokens) + sum(ai_finops_output_tokens)` | Le « carburant » consommé ; croît avec le trafic. |
| 3 | **Requests (total)** | Nombre total de requêtes LLM routées et comptabilisées | `sum(ai_finops_requests)` | Volume d'appels réels. |
| 4 | **Budget usage (%) by policy** | Prévision mensuelle de dépense ÷ plafond mensuel, **une barre par AIBudgetPolicy** | `ai_finops_budget_usage_percent` | Vert <70%, jaune ≥70, orange ≥90, **rouge ≥100 (dépassé)**. En mode `verify`, les budgets restent normalement faibles car les appels sont bornés. |
| 5 | **Cost (EUR) by namespace** | Coût réel par namespace dans le temps | `ai_finops_cost_eur` (par `namespace`) | Compare la dépense des équipes ; courbes croissantes. |
| 6 | **Tokens by namespace (input / output)** | Tokens entrée/sortie réels par namespace | `ai_finops_input_tokens`, `ai_finops_output_tokens` | Profil de consommation par équipe (out ≫ in pour les générations). |
| 7 | **Requests violating sovereignty (by app)** | Nb de requêtes envoyées par chaque app vers un fournisseur **non conforme** (zone interdite) | `sum by (namespace, application) (ai_finops_sovereignty_requests{severity="critical"})` | Volume réel **à risque** par app. 0 = conforme ; >0 = violation (rouge). |
| 8 | **Spend by sovereignty zone (EUR)** | Part de la dépense réelle par **zone de souveraineté** (résidence du provider) : **EU conforme** vs zone interdite/global | `ai_finops_cost_by_zone_eur` (label `zone`) | La part hors zone autorisée = exposition souveraineté. |
| 9 | **Observed latency telemetry** | Disponibilité de télémétrie et latence moyenne observée quand elle existe | `ai_finops_measured_latency_millis or ai_finops_latency_telemetry_available` | Si `latency_telemetry_available=0`, la latence mesurée est absente : pas de valeur inventée. |

### Métriques sous-jacentes
Émises par l'opérateur sur `/metrics` (:8080), scrapées par Prometheus :
`ai_finops_cost_eur`, `ai_finops_input/output_tokens`,
`ai_finops_requests`, `ai_finops_budget_usage_percent`,
`ai_finops_sovereignty_findings` / `ai_finops_sovereignty_requests`,
`ai_finops_recommendations` (labels `type`/`namespace`/`application`/`severity`),
`ai_finops_projected_monthly_cost_eur`, `ai_finops_latency_score`,
`ai_finops_measured_latency_millis`, `ai_finops_latency_telemetry_available`,
`ai_finops_routing_score`. Détail dans
[`docs/features/metrics.md`](../../docs/features/metrics.md).

> Note de véracité : les métriques `gen_ai_*` de la gateway sont **cumulatives**
> depuis son démarrage → le coût **observé** est exact. La prévision mensuelle
> fiable nécessite un calcul de débit (run-rate) ; le panneau 9 affiche un
> *potentiel d'économie* (et-si), pas un gain réalisé.
