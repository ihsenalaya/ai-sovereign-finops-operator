# Kubernetes Manifests

This directory documents integration manifests for the gateway-control-plane
artifact.

The current paper uses Kubernetes integration only to verify:

- operator deployment;
- Envoy AI Gateway telemetry ingestion;
- workload attribution;
- quality-evaluation plumbing;
- shadow-AI evidence ingestion.

Integration logs are not presented as production security-evaluation results.
Use `kind` for CI, debug, and regression testing only.
