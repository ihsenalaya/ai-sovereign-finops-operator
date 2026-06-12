# Architecture (HLD)

## Vue d'ensemble

L'opérateur applique le pattern *controller* de Kubernetes à la gouvernance FinOps & souveraineté
de l'IA. Chaque objet métier est une **CRD** ; un **reconciler** maintient le `.status` à jour à
partir du `.spec` et de la télémétrie collectée. Les calculs métier vivent dans des **packages
moteurs** purs (sans dépendance Kubernetes), donc testables en isolation et réutilisables.

```
api/v1alpha1            types CRD + conditions communes
internal/controller     reconcilers (orchestration k8s) + gatewayactuator/gatewayreroute, shadow, modelstubs
internal/catalog        catalogue par défaut (prix/zones intégrés) + EndpointToZone (pur)
internal/collectors     TelemetryCollector: aigw (Envoy/OTel, réel) | prometheus | configmap | fake (opt-in)
internal/costengine     coûts (pur)
internal/budgetengine   budgets & seuils (pur)
internal/sovereigntyengine  findings souveraineté — plan gateway (pur)
internal/shadowengine   findings shadow-AI — plan eBPF, indépendant de la gateway (pur)
internal/breakevenengine    point mort managé vs GPU (pur)
internal/recommendationengine  recommandations chiffrées, souveraineté-aware (pur)
internal/enforcementengine  décisions d'enforcement report/warn/reroute/block (pur)
internal/reporting      export Markdown/JSON (pur)
internal/metrics        métriques Prometheus ai_finops_* (gauges, sans _total)
config/                 CRDs, RBAC, manager, samples (Kustomize)
charts/                 Helm chart
dashboards/             Grafana
```

## Décisions clés

- **Enforcement = plan de contrôle (slices 1 & 2)** : selon `AISovereigntyPolicy.enforcementMode`,
  l'opérateur **agit** — `reportOnly` (constat), `warn` (alerte : Events + métrique), `enforce` (reroute
  **réellement actué** dans Envoy AI Gateway via mutation réversible de l'`AIGatewayRoute` —
  `internal/controller/gatewayactuator.go`, client unstructured). Décision dans le moteur pur
  `enforcementengine` ; actuation + revert (annotation, finalizer) dans le controller.
- **Fallback budget managé** : `AIBudgetPolicy` peut, en `enforce`, rerouter un modèle trop cher vers
  un fallback managé moins cher au gateway. La décision reste prudente : pas de modèle partagé hors
  cible, pas de conflit avec une policy souveraineté déjà en `enforce`, et garde-fous qualité /
  télémétrie explicites (`internal/controller/budgetfallback.go`).
- **Deux plans de souveraineté** : (1) **gateway** — `sovereigntyengine` sur la télémétrie de la
  gateway ; (2) **eBPF / shadow-AI** — `shadowengine` sur l'egress par pod observé par **Tetragon**
  (ConfigMap `shadow-egress`), **indépendant de la gateway**, donc capte le trafic qui la contourne.
  Les deux partagent le « cerveau » `internal/catalog` (`EndpointToZone` + zones/prix).
- **Autonomie (catalogue intégré)** : `internal/catalog` fournit prix + zone des modèles connus et
  `EndpointToZone(host)` ; câblé en fallback (`priceBook`/`zoneForModel`/`flows`) → l'opérateur est
  utile **dès l'installation**, sans CR ; un CR override toujours. Modèle inconnu → AIModel stub.
- **Moteurs purs** : `costengine`, `budgetengine`, `enforcementengine`, `shadowengine`, etc. ne
  dépendent pas de controller-runtime. Ils prennent des structs et renvoient des résultats → tests
  unitaires rapides.
- **Monnaie en `resource.Quantity`** : décimaux exacts, idiomatique k8s, pas de float dans le schéma.
- **Télémétrie réelle, sans fake silencieux** : `TelemetryCollector` découple les moteurs de la source.
  Le chemin réel de production est `aigw` (**Envoy AI Gateway / OpenTelemetry**, lit `gen_ai_*`) ;
  `prometheus` et `configmap` complètent. **Pas de repli `fake` par défaut** : sans source réelle,
  l'opérateur pose une condition `NoTelemetrySource` plutôt que d'inventer des chiffres ; `fake` est un
  opt-in explicite (démo). Le mode `litellm` (stub) a été retiré.
- **CNCF/OSS d'abord** : Kubernetes, controller-runtime, Prometheus, Grafana, Helm, Kustomize, Envoy.

## Flux de réconciliation type (rapport)

1. `AIFinOpsReport` créé/édité → reconcile.
2. Le controller résout la gateway (`gatewayRef`/sélecteur) et instancie le `TelemetryCollector`.
3. `collector.Collect(window)` → `[]UsageSample`.
4. `costengine.Cost(samples, providers)` → ventilation des coûts.
5. `sovereigntyengine` + `budgetengine` + `breakevenengine` produisent findings & recommandations.
6. `reporting` rend Markdown/JSON ; les agrégats sont écrits dans `.status` ; events + métriques émis.

## Observabilité

`internal/metrics` enregistre des métriques Prometheus préfixées `ai_finops_` sur le registre
controller-runtime (exposées via `/metrics` du manager) : requêtes, tokens, coût EUR, usage budget %,
findings souveraineté, économies break-even, recommandations. Un dashboard Grafana est fourni dans
`dashboards/`.

## Sécurité

RBAC minimal généré par les markers `+kubebuilder:rbac`. Lecture majoritaire, avec écriture limitée aux
ressources explicitement pilotées par l'opérateur (CRDs/status, `AIGatewayRoute`, webhook config, Events) ;
les secrets de gateway sont référencés via `secretRef` (jamais copiés dans le status). Le manager tourne
en non-root via le `config/manager` standard de Kubebuilder.
