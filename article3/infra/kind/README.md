# Kind Automation

The prompt-required entrypoints are provided with idempotent wrappers:

```bash
bash article3/infra/kind/create.sh
bash article3/infra/kind/install.sh
bash article3/infra/kind/healthcheck.sh
bash article3/infra/kind/collect-diagnostics.sh
bash article3/infra/kind/destroy.sh
```

Legacy helper scripts remain available:

- `create_cluster.sh`
- `deploy_experiments.sh`
- `destroy_cluster.sh`

Useful overrides:

- `EXPERIMENT_COMMAND=e5`
- `IMAGE_REPO=gov-ar-experiment`
- `IMAGE_TAG=local`
- `CLUSTER_NAME=article3-validation`
- `NAMESPACE=gov-ar`

When `CLUSTER_NAME` is left at its default and an existing `gov-ar` cluster is
already present, the wrappers reuse it automatically for verification and
diagnostics.
