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
- [x] `collectors.TelemetryCollector` + `fake`, `prometheus`, `litellm` (stub)
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

> Ce fichier est mis à jour au fil de l'avancement par l'agent.

## Post-MVP (non engagé)
- Mode `enforce` (blocage / approbation) — aujourd'hui `reportOnly`.
- Collecteur LiteLLM complet (auth, pagination), collecteur Envoy/OTel.
- Multi-devise, FX ; persistance historique ; export S3/PDF du dossier d'audit.
- Webhooks de validation/défaut, conversion de versions d'API.
