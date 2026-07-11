# Helm Packaging

Current local chart:

- `gov-ar-experiment`: runs the Article 3 experiment binary as a Kubernetes Job.

Example:

```bash
helm lint article3/infra/helm/gov-ar-experiment
helm template gov-ar article3/infra/helm/gov-ar-experiment \
  --set experiment.command=e5
```
