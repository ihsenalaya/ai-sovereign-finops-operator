# Démo réelle — vraies apps consommant des tokens via Envoy AI Gateway

Exemple end-to-end **100 % réel** : deux applications déployées dans le cluster
appellent en boucle de vrais LLM **à travers Envoy AI Gateway** (projet CNCF).
La gateway **mesure les vrais tokens** ; l'opérateur les **lit** et calcule
**coût, souveraineté et budget par application**, exposés dans **Grafana**.

```
 app rh-chatbot (ns rh) ─────┐                                   ┌─► OpenAI gpt-4o-mini (US)
                             ├─► Envoy AI Gateway (compte tokens) ┤
 app risk-assistant (finance)┘            │ gen_ai_* metrics      └─► OpenAI gpt-4o (US)
                                          ▼
                       Opérateur (collector "aigw") ─► coût/souveraineté/budget par app
                                          ▼
                          Prometheus ─► Grafana (dashboard véridique)
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
./deploy.sh up        # Envoy Gateway + AI Gateway + route OpenAI + catalogue + apps
./deploy.sh test      # un vrai appel gpt-4o à travers la gateway (sanity)
./deploy.sh down      # retire la démo (laisse les control planes)
```

Depuis la racine `automatisation/`, le même scénario est disponible via :

```bash
make real-demo
make real-demo-test
make real-demo-down
```

Prérequis : cluster kind `greenops`, `kubectl`, `helm`, `docs/openaikey.txt`.

## Fichiers
- `01-gateway-openai.yaml` — GatewayClass/Gateway/EnvoyProxy + AIGatewayRoute
  (modèles gpt-4o, gpt-4o-mini) + AIServiceBackend + BackendSecurityPolicy
  (clé OpenAI via Secret, créé par `deploy.sh`) + Backend api.openai.com + TLS.
- `02-metrics-and-catalog.yaml` — Service métriques de l'extproc (`:1064`),
  `AIGateway` en **mode `aigw`**, `AIProvider`/`AIModel` (prix réels par modèle,
  attribution `servesNamespace/Application/Team`), `AISovereigntyPolicy` (EU-only),
  `AIFinOpsReport`, `AIBudgetPolicy`.
- `03-consumer-apps.yaml` — Service data stable + 3 Deployments (rh/chatbot-rh,
  finance/risk-assistant, legal/contract-review) qui appellent la gateway en boucle.
- `05-mistral-eu.yaml` — **2ᵉ provider EU** : route Mistral (Azure AI Foundry, zone EU)
  via le schéma `AzureOpenAI` + catalogue `mistral-eu`/`mistral-large` + 4ᵉ app
  `marketing/content-writer`. Sert à **vérifier la souveraineté zone-aware** (l'app EU
  ne produit aucune violation, contrairement aux apps OpenAI US). Voir [`CRITERES.md` §6](CRITERES.md).
  Prérequis : clé Mistral dans `docs/mistralkey.txt` (récupérée depuis Azure Foundry/Key Vault).
- `deploy.sh` — installe et câble le tout (versions épinglées).

## Comment l'opérateur lit les vrais tokens
Envoy AI Gateway expose l'histogramme OpenTelemetry
`gen_ai_client_token_usage` (labels `gen_ai_request_model`, `gen_ai_token_type`
input/output ; `_sum` = tokens, `_count` = requêtes). Le collector **`aigw`**
(`internal/collectors/aigw`) scrape cet endpoint, et **attribue** chaque modèle à
l'app qui le consomme via le catalogue `AIModel` (`providerRef` + `serves*`).
Le coût = tokens réels × prix réel du modèle (sur l'`AIProvider`).

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

➡️ Résultat : **`finance/risk-assistant` et `legal/contract-review` utilisent tous
deux gpt-4o, mais sont comptabilisés séparément, par namespace/app** — automatiquement.
Déployer une nouvelle app dans un namespace opt-in suffit : aucune modification du
code applicatif ni ajout manuel d'en-têtes n'est nécessaire. Dans la démo, les pods
restreignent l'injection au host de la gateway via
`aiops.imperium.io/target-hosts=greenops-aigw.envoy-gateway-system.svc.cluster.local`.

## Comprendre les critères qui pilotent la démo

Pour savoir **quels paramètres** (prix, zones, budgets, politiques, apps/modèles)
produisent chaque chiffre des panneaux, voir **[`CRITERES.md`](CRITERES.md)** —
il relie chaque critère de configuration à la signification de chaque chart.

## Grafana — dashboard « AI Sovereign FinOps — Overview »

Ouvrir : `kubectl -n greenops-system port-forward svc/demo-grafana 3000:3000`
→ http://localhost:3000 (admin anonyme). Source : `dashboards/ai-finops-overview.json`.
Réconciliation opérateur toutes les **60 s** (dashboard vivant). Toutes les valeurs
proviennent du **vrai trafic** des apps via Envoy AI Gateway.

### Les 10 panneaux, un par un

| # | Panneau | Ce qu'il montre | Requête PromQL | Comment le lire |
|---|---------|-----------------|----------------|-----------------|
| 1 | **Total cost (EUR) — observed** | Dépense réelle cumulée depuis le démarrage de la gateway (tous apps) | `sum(ai_finops_cost_eur)` | Coût réel = tokens mesurés × prix de liste réels. Croît dans le temps. |
| 2 | **Tokens (total in + out)** | Volume total de tokens mesurés par la gateway | `sum(ai_finops_input_tokens) + sum(ai_finops_output_tokens)` | Le « carburant » consommé ; croît avec le trafic. |
| 3 | **Requests (total)** | Nombre total de requêtes LLM routées et comptabilisées | `sum(ai_finops_requests)` | Volume d'appels réels. |
| 4 | **Budget usage (%) by policy** | Prévision mensuelle de dépense ÷ plafond mensuel, **une barre par AIBudgetPolicy** | `ai_finops_budget_usage_percent` | Vert <70%, jaune ≥70, orange ≥90, **rouge ≥100 (dépassé)**. Ex. finance 162% (dépassé) / rh 21% (ok). |
| 5 | **Cost (EUR) by namespace** | Coût réel par namespace dans le temps | `ai_finops_cost_eur` (par `namespace`) | Compare la dépense des équipes ; courbes croissantes. |
| 6 | **Tokens by namespace (input / output)** | Tokens entrée/sortie réels par namespace | `ai_finops_input_tokens`, `ai_finops_output_tokens` | Profil de consommation par équipe (out ≫ in pour les générations). |
| 7 | **Requests violating sovereignty (by app)** | Nb de requêtes envoyées par chaque app vers un fournisseur **non conforme** (zone interdite) | `sum by (namespace, application) (ai_finops_sovereignty_requests{severity="critical"})` | Volume réel **à risque** par app. 0 = conforme ; >0 = violation (rouge). |
| 8 | **Cost-saving recommendations (action + gain €)** | **Table** : une ligne = une action cost-saving concrète, avec le **modèle actuel → modèle recommandé** et le **gain €** par app | `ai_finops_cost_saving_eur` (labels `namespace`, `application`, `current_model`, `recommended_model` ; valeur = € économisables) | « To-do » d'économies : *qui*, *quel swap de modèle*, *combien*. Voir « Comment lire la table » ci-dessous. |
| 9 | **Potential savings (EUR)** | Économie **potentielle** totale si on appliquait les recos cost-saving (sur la fenêtre observée) | `ai_finops_potential_savings_eur` (total) · `ai_finops_potential_savings_by_app_eur` (par app) | ⚠️ *Potentiel*, pas réalisé : coût actuel − coût avec modèle moins cher. |
| 10 | **Spend by sovereignty zone (EUR)** | Part de la dépense réelle par **zone de souveraineté** (résidence du provider) : **EU conforme** vs **US interdit** | `ai_finops_cost_by_zone_eur` (label `zone`) | La part qui **quitte l'UE** = exposition souveraineté. US (rouge) ≫ EU (vert) tant que les apps tapent OpenAI. |

#### Comment lire la table « Cost-saving recommendations (action + gain €) »

Ce panneau a remplacé l'ancien **camembert** « Recommendations by type » : un camembert ne
montrait qu'un *compte par type*, sans dire **quelle app**, **quoi faire**, ni **combien on
gagne**. La table répond aux trois — **chaque ligne = une action d'économie chiffrée**.

| Colonne | Question | Sens |
|---------|----------|------|
| **Namespace / Application** | **QUI ?** | L'équipe / l'app concernée. |
| **Modèle actuel → Recommandé** | **QUOI ?** | Le swap de modèle proposé (ex. `gpt-4o → gpt-4o-mini`). |
| **Gain (€)** | **COMBIEN ?** | Économie estimée sur la fenêtre observée si on applique le swap. |

Exemple typique (apps `finance`/`legal` sur `gpt-4o`, app `marketing` sur `mistral-large`) :

| Namespace | Application | Modèle actuel | Recommandé | Gain (€) |
|-----------|-------------|---------------|-----------|---------:|
| finance | risk-assistant | gpt-4o | gpt-4o-mini | 0,26 |
| legal | contract-review | gpt-4o | gpt-4o-mini | 0,23 |
| marketing | content-writer | mistral-large-latest | *(modèle conforme le moins cher)* | … |

- `rh` n'apparaît pas : elle utilise déjà `gpt-4o-mini` (le moins cher) → aucune économie.
- Le moteur de recos est désormais **zone-aware** : le modèle proposé reste dans une **zone
  autorisée** (il ne recommandera pas un provider qui violerait la souveraineté).
- Le **total** € est dans le panneau **#9 Potential savings** ; la répartition zone EU/US
  dans le panneau **#10 Spend by sovereignty zone**.

### Métriques sous-jacentes
Émises par l'opérateur sur `/metrics` (:8080), scrapées par Prometheus :
`ai_finops_cost_eur`, `ai_finops_input/output_tokens`,
`ai_finops_requests`, `ai_finops_budget_usage_percent`,
`ai_finops_sovereignty_findings` / `ai_finops_sovereignty_requests`,
`ai_finops_recommendations` (labels `type`/`namespace`/`application`/`severity`),
`ai_finops_potential_savings_eur` / `ai_finops_potential_savings_by_app_eur`,
`ai_finops_projected_monthly_cost_eur`. Détail dans
[`docs/features/metrics.md`](../../docs/features/metrics.md).

> Note de véracité : les métriques `gen_ai_*` de la gateway sont **cumulatives**
> depuis son démarrage → le coût **observé** est exact. La prévision mensuelle
> fiable nécessite un calcul de débit (run-rate) ; le panneau 9 affiche un
> *potentiel d'économie* (et-si), pas un gain réalisé.
