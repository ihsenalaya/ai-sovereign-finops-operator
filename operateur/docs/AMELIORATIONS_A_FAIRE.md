# Ameliorations a faire

Ce fichier note les ameliorations a discuter avant de transformer les recommandations FinOps en decisions automatiques.

Aujourd'hui, le `cost saving` estime une economie potentielle a partir du volume de tokens observe et du prix des modeles. Cette approche est utile pour la visibilite, mais elle ne suffit pas a garantir qu'un modele moins cher donnera une qualite de service equivalente.

## 1. Ajouter un quality gate pour detecter la degradation qualite

Objectif : verifier si le deuxieme modele, propose comme alternative moins chere, degrade le service sans dependre uniquement d'un juge LLM.

Principe propose :

- definir un golden dataset par application avec des prompts representatifs valides par le metier ;
- executer le modele actuel et le modele candidat sur ce golden dataset ;
- appliquer des controles deterministes : format attendu, schema JSON, champs obligatoires, absence de refus inattendu, longueur min/max, mots-cles attendus, absence de fuite de donnees sensibles ;
- lancer un canary limite avant tout reroute complet ;
- observer les metriques reelles : erreurs HTTP, timeouts, retries, latence, taux de fallback, escalades humaines si disponibles ;
- bloquer tout changement automatique si un controle metier ou operationnel degrade le service.

Resultat attendu :

```text
golden dataset + modele actuel + modele candidat
-> controles deterministes
-> canary limite
-> metriques reelles
-> verdict: pass / fail / rollback
```

Exemple de ressource cible :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: finance-quality-gate
  namespace: default
spec:
  target:
    namespace: finance
    application: risk-assistant
  sourceModel: gpt-us-mini
  candidateModel: gpt-france-mini
  goldenDatasetRef:
    name: finance-golden-prompts
    namespace: default
  requiredChecks:
    schemaValid: true
    noUnexpectedRefusal: true
    noSensitiveDataLeak: true
    requiredKeywords:
      - risk
      - liquidity
    maxErrorRatePercent: 2
    maxLatencyIncreasePercent: 20
    maxRetryIncreasePercent: 10
  canary:
    enabled: true
    percent: 5
    duration: 30m
  rollback:
    enabled: true
    onErrorRatePercent: 3
    onLatencyIncreasePercent: 30
```

Exemple de golden dataset configure par l'utilisateur :

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: finance-golden-prompts
  namespace: default
data:
  prompts.yaml: |
    - id: risk-summary
      prompt: "Summarize key counterparty risks for a new supplier."
      expected:
        requiredKeywords: ["counterparty", "liquidity", "risk"]
        maxTokens: 300
        mustBeJSON: false

    - id: var-explain
      prompt: "Explain value-at-risk in two sentences."
      expected:
        requiredKeywords: ["loss", "confidence", "portfolio"]
        maxTokens: 120
        mustBeJSON: false
```

Configuration par l'utilisateur :

```bash
kubectl apply -f finance-golden-prompts.yaml
kubectl apply -f finance-quality-gate.yaml
```

Status attendu :

```yaml
status:
  phase: Passed
  checkedSamples: 20
  failedChecks: 0
  canaryStatus: Stable
  verdict: candidate-safe
```

Un juge LLM peut rester optionnel pour les cas ambigus, mais il ne doit pas etre le mecanisme principal de decision. La decision principale doit venir des controles metier, des metriques observees, du canary et du rollback.

## 2. Ajouter un score global de decision

Objectif : ne plus recommander uniquement sur le cout, mais sur un score multi-criteres.

Parametres du score :

- cout : economie potentielle en EUR ;
- latence : temps de reponse du modele candidat ;
- qualite : resultat du quality gate, base sur golden dataset, controles deterministes, metriques reelles et canary ;
- souverainete : conformite avec les zones autorisees et les politiques ;
- fiabilite : taux d'erreur, timeouts, retries et disponibilite du backend.

Score propose :

```text
score_global =
  cout_score * poids_cout +
  latence_score * poids_latence +
  qualite_score * poids_qualite +
  souverainete_score * poids_souverainete +
  fiabilite_score * poids_fiabilite
```

Regle de decision proposee :

- recommander si le score global est superieur au seuil ;
- ne jamais recommander si la souverainete est non conforme ;
- ne jamais recommander si la qualite est sous le seuil minimal ;
- ne jamais recommander si la fiabilite est sous le seuil minimal ;
- utiliser le cout comme un avantage, pas comme seul critere.

## 3. Ajouter des fenetres Grafana pour le score et le schema en etoile

Objectif : rendre visible la raison d'une recommandation ou d'un blocage.

Fenetres dashboard a ajouter :

- score global par application et modele candidat ;
- detail des scores : cout, latence, qualite, souverainete, fiabilite ;
- schema en etoile par recommandation pour comparer les dimensions ;
- evolution du score dans le temps ;
- recommandations bloquees avec raison du blocage.
- retirer le score de la fenetre `Observed latency telemetry`, car le score sera affiche dans les futurs schemas en etoile ; cette fenetre doit rester focalisee sur la latence observee et la disponibilite de la telemetrie.

Exemple de schema en etoile :

```text
                 qualite
                    *
                   / \
                  /   \
        latence *-----* souverainete
                 \   /
                  \ /
                   *
                  cout
                   |
                   *
               fiabilite
```

Le dashboard doit permettre de voir rapidement si une economie est saine ou si elle cache un risque de degradation de service.

## 4. Simplifier le routage manuel et automatique

Objectif : eviter que l'utilisateur modifie directement les `AIGatewayRoute` ou detourne les policies souverainete/budget pour faire un routage simple.

### Routage manuel

Ajouter une ressource dediee, par exemple `AIRouteOverride`.

Exemple :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRouteOverride
metadata:
  name: route-us-to-france
  namespace: default
spec:
  sourceModel: gpt-us-mini
  targetModel: gpt-france-mini
  mode: manual
  reason: test-routage-france
```

Comportement attendu :

- l'utilisateur applique uniquement cet objet ;
- l'operateur trouve la route Envoy AI Gateway correspondante ;
- l'operateur applique le changement de backend ;
- l'operateur ajoute la `bodyMutation` pour reecrire le champ `model` ;
- le status indique `Actuated`, `Failed` ou `Reverted` ;
- la suppression de l'objet restaure automatiquement la route d'origine.

### Routage automatique

Ajouter une ressource dediee, par exemple `AIRoutingPolicy`.

Exemple :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRoutingPolicy
metadata:
  name: optimize-ai-routing
  namespace: default
spec:
  mode: automatic
  objective: cost
  guardrails:
    minQualityScore: 0.85
    maxLatencyMillis: 2000
    requireSovereigntyCompliance: true
  canary:
    enabled: true
    percent: 10
```

Comportement attendu :

- l'operateur choisit le meilleur modele selon le score global ;
- le routage automatique respecte les garde-fous qualite, latence, souverainete et fiabilite ;
- le changement commence par un shadow test ou un canary ;
- le reroute complet n'est applique que si les indicateurs restent bons ;
- le rollback est automatique si la qualite, la latence ou la fiabilite se degradent.

Resultat attendu :

```text
routage manuel simple:
kubectl apply -f route-override.yaml

routage automatique controle:
kubectl apply -f routing-policy.yaml
```

L'objectif est que l'utilisateur ne touche plus directement aux objets bas niveau `AIGatewayRoute` pour piloter le routage.

## 5. Ajouter une validation humaine avant les changements sensibles

Objectif : eviter qu'un changement automatique de modele degrade un service critique sans validation explicite.

Ajouter une ressource dediee, par exemple `AIChangeRequest`.

Exemple :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIChangeRequest
metadata:
  name: reroute-finance-to-france
  namespace: default
spec:
  action: reroute
  sourceModel: gpt-us-mini
  targetModel: gpt-france-mini
  reason: cost-saving-and-sovereignty-compliant
  expectedSavingEUR: 12.40
  qualityScore: 0.91
  latencyImpact: "+120ms"
  riskLevel: medium
  expiresAfter: 24h
```

Comportement attendu :

- l'operateur detecte une opportunite de changement ;
- l'operateur execute les controles necessaires : souverainete, cout, latence, qualite, fiabilite ;
- si le changement est sensible, l'operateur cree une `AIChangeRequest` au lieu de rerouter directement ;
- un humain approuve ou refuse la demande ;
- si la demande est approuvee, l'operateur applique le routage ;
- si elle expire ou est refusee, aucun changement n'est applique ;
- toute decision reste auditable dans le status, les Events Kubernetes et les metriques.

Workflow cible :

```text
recommendation
-> shadow test / quality check
-> AIChangeRequest
-> approbation humaine
-> reroute
-> monitoring
-> rollback si degradation
```

Fenetres dashboard a ajouter :

- changements en attente d'approbation ;
- changements approuves, refuses ou expires ;
- gain attendu ;
- niveau de risque ;
- impact qualite et latence ;
- rollbacks declenches apres changement.

## 6. Simplifier l'export des documents lisibles par un humain

Objectif : permettre a un utilisateur de recuperer facilement les rapports generes par l'operateur, sans devoir connaitre les details Kubernetes du `ConfigMap`.

Aujourd'hui, l'operateur genere deja un rapport lisible par un humain dans un `ConfigMap` :

```text
ConfigMap: <nom-du-report>-report
cles: report.md, report.json
```

Exemple actuel :

```bash
kubectl -n default get cm ai-report-all-report -o jsonpath='{.data.report\.md}'
kubectl -n default get cm ai-report-all-report -o jsonpath='{.data.report\.json}'
```

Amelioration proposee :

- ajouter une commande `make export-report` ;
- exporter automatiquement `report.md` et `report.json` dans un dossier local, par exemple `exports/reports/` ;
- permettre de choisir le report et le namespace ;
- afficher clairement le chemin du fichier genere ;
- ajouter plus tard un export PDF si necessaire.

Exemple cible :

```bash
make export-report REPORT=ai-report-all NAMESPACE=default
```

Resultat attendu :

```text
exports/reports/ai-report-all/report.md
exports/reports/ai-report-all/report.json
```

Cette fonctionnalite rend les donnees de l'operateur directement utilisables pour un humain : revue FinOps, audit, partage equipe, validation avant changement et documentation de decision.
