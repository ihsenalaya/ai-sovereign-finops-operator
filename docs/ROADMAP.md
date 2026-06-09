# Roadmap — backlog par sprint

Statut : ✅ fait · 🛠️ en cours · ⬜ à venir

## Sprint 1 — Skeleton operator ✅
- [x] `kubebuilder init` + 7 CRDs (`create api`)
- [x] Types complets, validations OpenAPI, print-columns
- [x] Status + conditions Kubernetes standard (`Ready`), `observedGeneration`
- [x] 7 controllers avec reconcile réel, events, logs structurés
- [x] Résolution `AIModel→AIProvider` (+ watch), `AIGateway→namespaces`
- [x] Samples, Makefile (versions outils bumpées Go 1.25)
- [x] Tests envtest (~74 % controllers), validé sur kind

## Sprint 2 — Cost engine ✅
- [x] `collectors.TelemetryCollector` + `aigw` (Envoy AI Gateway/OTel, réel), `prometheus`, `configmap`, `fake` (opt-in). Mode `litellm` (stub) retiré.
- [x] `costengine` : coût input/output, par modèle/équipe/namespace/requête/token
- [x] Pricing depuis `AIProvider`, catalogue `AIModel`
- [x] Wiring du coût dans `AIFinOpsReport.status`
- [x] Tests unitaires

## Sprint 3 — Budget engine ✅
- [x] `budgetengine` : conso vs budget, % consommé, seuils warning/critical/hard
- [x] Recommandations d'action (mode recommandation only)
- [x] `internal/metrics` : métriques `ai_finops_*`
- [x] Wiring dans le controller `AIBudgetPolicy`
- [x] Tests unitaires

## Sprint 4 — Sovereignty engine ✅
- [x] `sovereigntyengine` : fournisseur vs policy, zones FR/EU, donnée sensible
- [x] Findings `info|warning|critical`
- [x] Wiring dans le controller `AISovereigntyPolicy`
- [x] Tests unitaires

## Sprint 5 — Break-even engine ✅
- [x] `breakevenengine` : coût managé vs auto-hébergé, économie, point mort
- [x] Recommandation `keep-managed|investigate|self-host`
- [x] Wiring dans le controller `AIBreakEvenAnalysis`
- [x] Tests unitaires

## Sprint 6 — Reporting & demo ✅
- [x] `reporting` : export Markdown + JSON
- [x] Dashboard Grafana
- [x] Helm chart (deployment, RBAC, CRDs, values)
- [x] Démo kind

## Sprint 7 — Enforcement (slice 1) ✅
- [x] `enforcementengine` (moteur pur, testé) : `reportOnly`→report, `warn`→alerte actuée, `enforce`→`reroute` (vers modèle conforme le moins cher) ou `block`
- [x] Recommandations **conscientes de la souveraineté** (jamais de swap vers une zone interdite)
- [x] Wiring controller `AISovereigntyPolicy` : Events Kubernetes différenciés, métrique `ai_finops_enforcement_actions`, posture en status, finalizer + tracker anti-fantôme
- [x] Métriques renommées (gauges sans `_total`), repli `fake` silencieux supprimé
- [x] Validé en réel sur kind (modes reportOnly→warn→enforce sur les violations US)

## Sprint 8 — Enforcement slice 2 : actuation Envoy AI Gateway ✅
- [x] `gatewayactuator` : reroute réversible du modèle interdit vers le backend conforme (mutation de l'`AIGatewayRoute` via client unstructured : backend + `bodyMutation.set model`)
- [x] Revert automatique (annotation `aiops.imperium.io/enforced-reroutes`) au changement de mode et à la suppression (finalizer)
- [x] RBAC `aigateway.envoyproxy.io/aigatewayroutes` ; `actuated=true` reflète l'état réel de la route
- [x] Validé en réel sur kind (la route gpt-4o bascule vers le backend Mistral EU, puis revert)

> Ce fichier est mis à jour au fil de l'avancement par l'agent.

## Post-MVP (engagé / en cours)
- **Enforcement `block`** au gateway pour un modèle interdit **sans** fallback conforme (retrait/deny de la route).
- **Enforcement budget** : même moteur, déclenché par dépassement de seuil (warning/critical/hardLimit + repli).
- **Télémétrie** : durcir le chemin Envoy/OTel — latence, erreurs, **détection de reset** des compteurs gateway.
- Multi-devise, FX ; persistance historique ; export S3/PDF du dossier d'audit.
- Webhooks de validation/défaut, conversion de versions d'API.
