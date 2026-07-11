# Helm Release

## Local release status

- chart path:
  `operateur/charts/ai-sovereign-finops-operator`
- baseline chart version: `0.5.11`
- packaged experimental artifact:
  `article3/artifacts/helm/gov-ar-experiment-0.1.0.tgz`
- package SHA256:
  `1e3da0e199fc90fa1666318d524545506ce1ca6cf2f18c3b85ea27d6a3ca26f5`

## GOV-AR additions

- optional `gov-ar-admission` deployment and service
- optional PostgreSQL secret wiring for `DATABASE_URL`
- image and resource settings under `govArAdmission.*` in `values.yaml`

## Verified local check

`helm lint charts/ai-sovereign-finops-operator --set govArAdmission.enabled=true --set govArAdmission.postgres.enabled=true`

passed on 2026-07-11.

## Remaining gap

OCI publication to GHCR was not completed because the active GitHub
authentication context did not have effective package publication rights.
