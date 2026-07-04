# Platform API

Repository scaffold for the platform control API.

## Current implementation

- Compilable HTTP server at `operateur/cmd/platform-api/main.go`
- Health endpoint: `/healthz`
- Status endpoint: `/api/v1/status`

## Planned follow-up

- CRD-backed read APIs
- Policy management endpoints
- Audit timeline endpoints
- Experiment and benchmark endpoints
- Authentication and private-mode access control
