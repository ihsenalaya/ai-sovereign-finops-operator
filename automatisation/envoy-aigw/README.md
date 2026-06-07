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

Prérequis : cluster kind `greenops`, `kubectl`, `helm`, `docs/openaikey.txt`.

## Fichiers
- `01-gateway-openai.yaml` — GatewayClass/Gateway/EnvoyProxy + AIGatewayRoute
  (modèles gpt-4o, gpt-4o-mini) + AIServiceBackend + BackendSecurityPolicy
  (clé OpenAI via Secret, créé par `deploy.sh`) + Backend api.openai.com + TLS.
- `02-metrics-and-catalog.yaml` — Service métriques de l'extproc (`:1064`),
  `AIGateway` en **mode `aigw`**, `AIProvider`/`AIModel` (prix réels par modèle,
  attribution `servesNamespace/Application/Team`), `AISovereigntyPolicy` (EU-only),
  `AIFinOpsReport`, `AIBudgetPolicy`.
- `03-consumer-apps.yaml` — Service data stable + 2 Deployments (rh-chatbot,
  risk-assistant) qui appellent la gateway en boucle.
- `deploy.sh` — installe et câble le tout (versions épinglées).

## Comment l'opérateur lit les vrais tokens
Envoy AI Gateway expose l'histogramme OpenTelemetry
`gen_ai_client_token_usage` (labels `gen_ai_request_model`, `gen_ai_token_type`
input/output ; `_sum` = tokens, `_count` = requêtes). Le collector **`aigw`**
(`internal/collectors/aigw`) scrape cet endpoint, et **attribue** chaque modèle à
l'app qui le consomme via le catalogue `AIModel` (`providerRef` + `serves*`).
Le coût = tokens réels × prix réel du modèle (sur l'`AIProvider`).

## Grafana — dashboard véridique
8 panneaux, tous adossés à des métriques réelles (`ai_finops_*`) :
coût total observé, tokens total, requêtes, **budget % (prévision vs cap)**,
coût par namespace, tokens par namespace, **requêtes en violation de souveraineté
par app**, recommandations. Réconciliation toutes les 60 s (dashboard vivant).

Ouvrir : `kubectl -n greenops-system port-forward svc/demo-grafana 3000:3000`
→ http://localhost:3000.

> Note : les métriques `gen_ai_*` sont cumulatives depuis le démarrage de la
> gateway. Le coût **observé** est donc exact ; une prévision mensuelle fiable
> nécessite un calcul de débit (run-rate) — non affiché ici pour rester véridique.
