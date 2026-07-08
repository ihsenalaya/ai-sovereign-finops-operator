# Attestation Scheduler

Scheduler component for attestation-aware placement.

## Current repository state

- Binary scaffold: `operateur/cmd/attestation-scheduler/main.go`
- Pure placement evaluation: `operateur/internal/placement/placement.go`
- Placement CRD: `AIPlacementDecision`

## Intended scheduler-plugin path

Target plugin phases:

- `Filter`
- `Score`
- `Reserve`
- `Permit`
- `PreBind`

## Version note

Real Confidential GPU preparation via DRA should target Kubernetes `>= 1.34`.
The current repository still compiles against Kubernetes `1.29` libraries, so the
plugin scaffold is present but not yet upgraded to full DRA-GA integration.
