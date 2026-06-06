# Architecture (HLD)

## Vue d'ensemble

L'opérateur applique le pattern *controller* de Kubernetes à la gouvernance FinOps & souveraineté
de l'IA. Chaque objet métier est une **CRD** ; un **reconciler** maintient le `.status` à jour à
partir du `.spec` et de la télémétrie collectée. Les calculs métier vivent dans des **packages
moteurs** purs (sans dépendance Kubernetes), donc testables en isolation et réutilisables.

```
api/v1alpha1            types CRD + conditions communes
internal/controller     reconcilers (orchestration k8s)
internal/collectors     TelemetryCollector: fake | prometheus | litellm
internal/costengine     coûts (pur)
internal/budgetengine   budgets & seuils (pur)
internal/sovereigntyengine  findings souveraineté (pur)
internal/breakevenengine    point mort managé vs GPU (pur)
internal/reporting      export Markdown/JSON (pur)
internal/metrics        métriques Prometheus ai_finops_*
config/                 CRDs, RBAC, manager, samples (Kustomize)
charts/                 Helm chart
dashboards/             Grafana
```

## Décisions clés

- **Lecture seule & non-bloquant** : le MVP n'altère jamais la gateway ni les workloads.
  `AISovereigntyPolicy.enforcementMode` est `reportOnly`. La valeur est produite par le *rapport*.
- **Moteurs purs** : `costengine`, `budgetengine`, etc. ne dépendent pas de controller-runtime.
  Ils prennent des structs (`collectors.UsageSample`, types `v1alpha1`) et renvoient des résultats.
  → tests unitaires rapides, pas d'envtest requis pour la logique métier.
- **Monnaie en `resource.Quantity`** : décimaux exacts, idiomatique k8s, pas de float dans le schéma.
- **Télémétrie abstraite** : `TelemetryCollector` découple les moteurs de la source réelle.
  MVP par défaut = `fake` (démontrable hors gateway). `prometheus` scrute un endpoint ; `litellm`
  est préparé (stub) pour l'API d'usage LiteLLM. Une intégration **Envoy/OpenTelemetry** est la
  prochaine cible (CNCF).
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
