# Critères de la démo — comprendre les chartes Grafana

Ce document liste **les critères (paramètres de configuration) qui font tourner la
démo** et explique **comment chacun donne son sens à un panneau** du dashboard
« AI Sovereign FinOps — Overview ». Toutes les valeurs ci-dessous sont **réelles** et
proviennent des manifestes de `automatisation/envoy-aigw/` — rien n'est inventé ni simulé.

> Principe : de vraies apps appellent OpenAI **à travers Envoy AI Gateway**, qui
> **mesure les vrais tokens et la durée des requêtes**. L'opérateur lit ces signaux et applique les critères
> ci-dessous pour calculer **coût / souveraineté / budget / recommandations / score de latence**. Dans
> cette démo, la souveraineté reste `reportOnly`, mais le budget finance peut
> **rerouter réellement** `gpt-4o -> gpt-4o-mini`.

---

## 1. Le flux (qui produit les chiffres)

```
 apps (rh, finance, legal)  ──►  Envoy AI Gateway  ──►  OpenAI (US)
   sidecar webhook injecte        mesure gen_ai_* (tokens réels)
   les en-têtes x-greenops-*               │
        │                                │
        └──────────► Opérateur (collector "aigw") lit tokens + durée observée
                          applique les critères (§2)
                                   │
                          /metrics ai_finops_*  ──►  Prometheus  ──►  Grafana
```

---

## 2. Les critères de configuration

### 2.1 Apps consommatrices (`03-consumer-apps.yaml`)

Chaque app tourne dans **son namespace** et appelle **un modèle** en boucle. Elle
reçoit un **sidecar proxy injecté par le webhook** qui ajoute automatiquement
`x-greenops-namespace` / `x-greenops-app` → c'est ce qui permet l'**attribution
par namespace/app même sur un modèle partagé**, sans changer le code applicatif.

| Namespace | Application | Modèle appelé | Fournisseur (zone) | Particularité |
|-----------|-------------|---------------|--------------------|---------------|
| `rh` | chatbot-rh | **gpt-4o-mini** | OpenAI (US) | modèle le moins cher |
| `finance` | risk-assistant | **gpt-4o** | OpenAI (US) | modèle cher |
| `legal` | contract-review | **gpt-4o-mini** | OpenAI (US) | **même modèle que rh**, autre namespace → prouve l'attribution par namespace |
| `marketing` | content-writer | **mistral-large-latest** | Mistral / Azure Foundry (**EU**) | **provider EU** → prouve que la souveraineté est *zone-aware* (aucune violation, cf. §6) |

> Les 3 premières apps tapent sur OpenAI **US** (zone interdite) ; `marketing` tape sur
> **Mistral EU** (zone autorisée). C'est ce contraste qui valide le moteur de souveraineté.

### 2.2 Catalogue — fournisseurs & prix (`02-metrics-and-catalog.yaml`)

Le coût n'est **pas** estimé : c'est `tokens réels × prix réel du modèle`. Les prix
sont les **tarifs publics OpenAI** convertis en EUR (USD × 0,92).

| Provider (`AIProvider`) | Zone (`dataResidency`) | Géré | Sensible OK ? | Prix entrée €/M | Prix sortie €/M |
|-------------------------|------------------------|------|---------------|----------------:|----------------:|
| `openai-us` (gpt-4o) | **us** | oui | non | 2,30 | 9,20 |
| `openai-us-mini` (gpt-4o-mini) | **us** | oui | non | 0,14 | 0,55 |
| `mistral-eu` (mistral-large) | **eu** (francecentral) | oui | **oui** | 1,84 | 5,52 |

| Modèle (`AIModel`) | Provider | Tier coût | Données sensibles ? | Sert (ns/app) |
|--------------------|----------|-----------|---------------------|---------------|
| `gpt-4o` | openai-us | high | non | finance/risk-assistant |
| `gpt-4o-mini` | openai-us-mini | low | non | rh/chatbot-rh (fallback réel pour finance ; aussi utilisé par legal via headers) |
| `mistral-large` (`mistral-large-latest`) | mistral-eu | high | oui | marketing/content-writer |

> gpt-4o est **~16× plus cher en sortie** que gpt-4o-mini (9,20 vs 0,55) — c'est ce
> qui crée les recommandations *cost-saving*.
> `mistral-eu` est en **zone EU** avec `allowedForSensitiveData: true` → **zéro violation**
> (ni zone, ni données sensibles). Le provider Mistral est **Azure AI Foundry**
> (`greenops-foundry`, francecentral, SKU DataZoneStandard), routé via le schéma
> `AzureOpenAI` d'Envoy AI Gateway (clé Azure dans un Secret, depuis `docs/mistralkey.txt`).

### 2.3 Politique de souveraineté (`AISovereigntyPolicy`)

| Critère | Valeur | Effet |
|---------|--------|-------|
| Zones **autorisées** | `FR`, `EU` | seules ces zones sont conformes |
| Zones **interdites** | `US`, `CN` | toute requête vers ces zones = **violation critique** |
| Providers externes pour données sensibles | **interdits** | génère un *warning* en plus |
| Mode | `reportOnly` | on alerte, on ne bloque pas (pas de conflit avec le fallback budget) |

➡️ Comme les **3 apps** tapent sur OpenAI **US** (zone interdite), chacune produit une
**violation critique**. C'est la source des panneaux #6 et des lignes `sovereignty`.

### 2.4 Budgets (`AIBudgetPolicy`)

| Budget | Cible | Plafond mensuel | Modèle utilisé | Résultat attendu |
|--------|-------|----------------:|----------------|------------------|
| `finance-budget` | finance/risk-assistant | **0,02 €** | gpt-4o (cher) | atteint vite `Critical` puis **reroute live** vers `gpt-4o-mini` |
| `rh-budget` | rh/chatbot-rh | **0,20 €** | gpt-4o-mini (peu cher) | reste **WithinBudget** |
| `legal-budget` | legal/contract-review | **0,20 €** | gpt-4o-mini (peu cher) | reste **WithinBudget** |

Seuils communs : **warning ≥ 70 %**, **critical ≥ 90 %**, **hard limit ≥ 100 %**.
Le pourcentage affiché = **dépense mensuelle observée** ÷ plafond. Pour `finance-budget`,
`enforcementMode=enforce`, `fallbackOnPhase=Critical` et `minFallbackQualityTier=medium`
autorisent le reroute live vers `gpt-4o-mini` dès que le budget entre en zone critique.

### 2.5 Règles du moteur de recommandations

| Type | Quand l'opérateur l'émet | Sévérité |
|------|--------------------------|----------|
| `cost-saving` | un modèle moins cher ferait le même travail **et économiserait ≥ 20 %** | info |
| `sovereignty` | l'app envoie du trafic vers une **zone interdite** | critical |
| `data-quality` | un modèle consommé **sans prix** dans le catalogue (coût incalculable) | warning |

### 2.6 Score de routage et latence observée

Le score de routage est calculé pour chaque couple namespace/application/modèle observé. Il combine
coût, qualité du catalogue, fiabilité et latence. La latence n'est utilisée comme mesure que si
Envoy AI Gateway expose `gen_ai_server_request_duration_seconds`.

| Cas | Effet dans le status/dashboard |
|-----|--------------------------------|
| Durée AIGW observée | `latencyTelemetryAvailable=true`, `latencySource=observed`, `observedLatencyMillis>0`, métrique `ai_finops_measured_latency_millis` |
| Durée AIGW absente | `latencyTelemetryAvailable=false`, pas de latence mesurée, composant latence neutre, score total toujours présent |

---

## 3. Comment les critères donnent du sens à chaque panneau

| # | Panneau | Piloté par quels critères | Pourquoi le chiffre est ce qu'il est |
|---|---------|---------------------------|--------------------------------------|
| 1 | **Total cost (EUR)** | tokens réels × prix §2.2 | somme du coût de toutes les apps |
| 2 | **Tokens (in+out)** | trafic réel des apps §2.1 | volume mesuré par la gateway |
| 3 | **Requests (total)** | boucles d'appel des apps | nombre d'appels réellement routés |
| 4 | **Budget usage (%)** | budgets §2.4 | finance dépasse vite (gpt-4o cher, plafond 0,02 €) ; rh/legal restent sous (gpt-4o-mini) |
| 5 | **Cost by namespace** | prix §2.2 par namespace | finance coûte plus que rh/legal tant que le fallback n'a pas encore basculé sur `gpt-4o-mini` |
| 6 | **Requests violating sovereignty (by app)** | politique §2.3 (US interdit) | **volume** de requêtes fautives par app : rh/finance/legal (US) >0 ; **`marketing` (Mistral EU) absent = 0** (cf. §6) |
| 7 | *(idem #6 selon disposition)* | — | — |
| 8 | **Spend by sovereignty zone (EUR)** | prix §2.2 + résidence provider | part de dépense par zone ; la zone US matérialise l'exposition |
| 9 | **Observed latency telemetry** | règles §2.6 + durée AIGW | latence mesurée affichée seulement si la durée réelle est observée ; le score est réservé aux futurs panneaux de décision |

---

## 4. Les 7 capacités démontrées (rappel)

1. **Catalogue & gateway** — `AIGateway` / `AIProvider` / `AIModel` (§2.2).
2. **Attribution des coûts** — coût réel par namespace/app, même modèle partagé (§2.1).
3. **Souveraineté par flux** — violations US réelles, par app (§2.3, panneau #6).
4. **Budget & dégradation** — finance passe `Critical` puis reroute vers le fallback ; rh/legal restent WithinBudget (§2.4, panneau #4).
5. **Break-even** managé vs auto-hébergé — `AIBreakEvenAnalysis`.
6. **Reporting** — ConfigMap `ai-report-all-report` (`report.md` / `report.json`).
7. **Observabilité** — métriques `ai_finops_*` → Prometheus → Grafana (ce dashboard).

---

## 5. Notes de véracité (important pour lire les chiffres)

- Les métriques `gen_ai_*` de la gateway sont **cumulatives depuis son démarrage**.
  → Le **coût observé est exact** ; un redémarrage du pod envoy **remet les compteurs à
  zéro** (les valeurs repartent du bas, c'est normal).
- Si tu **arrêtes les apps** (`scale --replicas=0`), les chiffres **se figent** au
  dernier état mesuré — ils ne bougent plus jusqu'à ce que tu relances les apps.
- Réconciliation de l'opérateur **toutes les ~60 s** → le dashboard est vivant.

> Métriques exposées : `ai_finops_cost_eur`, `ai_finops_input/output_tokens`,
> `ai_finops_requests`, `ai_finops_budget_usage_percent`,
> `ai_finops_sovereignty_requests`,
> `ai_finops_recommendations` (labels `type`/`namespace`/`application`/`severity`),
> `ai_finops_measured_latency_millis`,
> `ai_finops_latency_telemetry_available`, `ai_finops_routing_score`.
> Détail : [`docs/features/metrics.md`](../../docs/features/metrics.md).

---

## 6. Vérification : le moteur est *zone-aware* (EU vs US)

**Objectif du test** : prouver que la souveraineté flague la **zone**, pas « tout appel
externe ». Critère discriminant : une app sur un provider **EU autorisé** ne doit générer
**aucune** violation, alors que les apps **US interdit** en génèrent.

**Montage** (`05-mistral-eu.yaml`) : une 4ᵉ app réelle, `marketing/content-writer`, appelle
**Mistral (Azure AI Foundry, zone EU)** à travers la même gateway. Tout le reste est
identique aux apps OpenAI (vrai trafic, mesure des tokens, attribution par namespace).

**Pourquoi `marketing` est conforme** (cf. moteur, [`internal/sovereigntyengine`](../../internal/sovereigntyengine/sovereigntyengine.go)) :

| Règle du moteur | OpenAI US | Mistral EU |
|-----------------|-----------|------------|
| Zone interdite (`US`,`CN`) ? | ✅ → `critical` | ❌ EU autorisé → **rien** |
| Hors zones autorisées (`FR`,`EU`) ? | ✅ → (déjà critical) | ❌ EU autorisé → **rien** |
| Managed + données sensibles interdites ? | ✅ → `warning` | ❌ `allowedForSensitiveData: true` → **rien** |

**Résultat observé** (réel, via Prometheus) :

| App | Provider (zone) | Vue/facturée ? | `sovereignty` critical | Ligne reco `sovereignty` |
|-----|-----------------|----------------|------------------------|--------------------------|
| rh / finance / legal | OpenAI (**US**) | oui | **oui** (>0 req) | **oui** |
| **marketing** | Mistral (**EU**) | **oui** | **non (0)** | **aucune** |

➡️ L'app EU est bien **vue et facturée** (donc le pipeline fonctionne), mais **0 violation** :
c'est bien la **zone de souveraineté** qui déclenche le flag. Test concluant.

> Lancer / relancer ce test :
> ```bash
> kubectl -n marketing scale deploy/content-writer --replicas=1   # générer du trafic EU
> kubectl -n marketing scale deploy/content-writer --replicas=0   # stopper (Mistral Large est cher)
> ```
