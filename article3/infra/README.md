# Article 3 Infrastructure

This directory contains the local reproducibility infrastructure for the GOV-AR
Article 3 workspace.

## Scope

- `kind/`: local Kubernetes cluster automation
- `helm/`: Helm packaging for the experiment runner job
- `postgres/`: local PostgreSQL bootstrap for persistent GOV-AR reservation state
- `gateway/`: placeholders for future gateway-side integration
- `azure/`: live-cloud notes and execution traces

The current implementation focuses on a reproducible local execution path using
Docker, Kind, and Helm.
