# Fonctionnalité — Reporting (`internal/reporting`)

Rend un rapport FinOps & souveraineté en **Markdown** (lisible) et **JSON** (contrat stable pour
l'outillage). **Pur** et déterministe, couverture ~87 %.

## API
```go
type Data struct {
    Name, Namespace, Period, Collector string
    GeneratedAt time.Time
    Breakdown   costengine.Breakdown
    Sovereignty []v1alpha1.SovereigntyFinding
    Recommends  []v1alpha1.Recommendation
    RoutingScores []v1alpha1.RoutingScore
}
func RenderMarkdown(d Data) string
func RenderJSON(d Data) ([]byte, error)
func Assumptions() []string
```

## Contenu du rapport
Résumé exécutif (coût total, requêtes, tokens, coût/req, coût/token), **coût par modèle / par
fournisseur / par équipe**, scores runtime de routage/latence, findings de souveraineté,
recommandations, et **limites & hypothèses** (dont : « préparation à l'audit, pas une attestation
juridique » ; « latence observée uniquement si télémétrie réelle »).

## Intégration
Le controller [AIFinOpsReport](../crds/aifinopsreport.md) écrit le rendu dans un **ConfigMap**
`<nom>-report` (clés `report.md`, `report.json`) avec owner-reference (GC automatique).

```bash
kubectl get cm monthly-ai-report-rh-report -o jsonpath='{.data.report\.md}'
kubectl get cm monthly-ai-report-rh-report -o jsonpath='{.data.report\.json}' | jq .
```
