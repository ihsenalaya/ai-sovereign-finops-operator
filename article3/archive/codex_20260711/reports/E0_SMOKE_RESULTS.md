# E0 Smoke Results

Local smoke validation of the current GOV-AR scaffold.

```json
{
  "experiment_id": "E0",
  "variant": "local_scaffold_smoke",
  "checks": [
    {
      "name": "go_test",
      "status": "passed",
      "detail": "?   \tgithub.com/imperium/ai-sovereign-finops-operator/article3/cmd/experiment\t[no test files]\nok  \tgithub.com/imperium/ai-sovereign-finops-operator/article3/src/admission\t(cached)\nok  \tgithub.com/imperium/ai-sovereign-finops-operator/article3/src/fault_injector\t(cached)\nok  \tgithub.com/imperium/ai-sovereign-finops-operator/article3/src/ledger\t(cached)\nok  \tgithub.com/imperium/ai-sovereign-finops-operator/article3/src/predictor\t(cached)\nok  \tgithub.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway\t(cached)\n"
    },
    {
      "name": "status_md",
      "status": "passed",
      "detail": "STATUS.md"
    },
    {
      "name": "operator_audit",
      "status": "passed",
      "detail": "operator_audit/architecture.md"
    },
    {
      "name": "experiment_registry",
      "status": "passed",
      "detail": "provenance/experiment_registry.csv"
    }
  ]
}
```

