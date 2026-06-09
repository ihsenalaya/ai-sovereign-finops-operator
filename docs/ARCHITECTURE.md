# Architecture (HLD)

## Vue d'ensemble

L'opérateur applique le pattern *controller* de Kubernetes à la gouvernance FinOps & souveraineté
de l'IA. Chaque objet métier est une **CRD** ; un **reconciler** maintient le `.status` à jour à
partir du `.spec` et de la télémétrie collectée. Les calculs métier vivent dans des **packages
moteurs** purs (sans dépendance Kubernetes), donc testables en isolation et réutilisables.

```
api/v1alpha1            types CRD + conditions communes
internal/controller     reconcilers (orchestration k8s)
internal/collectors     TelemetryCollector: aigw (Envoy/OTel, réel) | prometheus | configmap | fake (opt-in)
internal/costengine     coûts (pur)
internal/budgetengine   budgets & seuils (pur)
internal/sovereigntyengine  findings souveraineté (pur)
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
- **Moteurs purs** : `costengine`, `budgetengine`, `enforcementengine`, etc. ne dépendent pas de
  controller-runtime. Ils prennent des structs et renvoient des résultats → tests unitaires rapides.
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

RBAC minimal généré par les markers `+kubebuilder:rbac`. Accès lecture seule aux ressources externes ;
les secrets de gateway sont référencés via `secretRef` (jamais copiés dans le status). Le manager
tourne en non-root via le `config/manager` standard de Kubebuilder.
