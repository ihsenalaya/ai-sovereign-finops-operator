SHELL := /usr/bin/env bash

# ─── Variables ────────────────────────────────────────────────────────────────
REGISTRY       ?=
VERSION        ?= 0.5.4
IMAGE_TAG      ?= $(VERSION)
CHART_VERSION  ?= $(VERSION)
PUSH           ?= false
HELM_REGISTRY  ?=
GHCR_NAMESPACE ?= ghcr.io

IMAGES := governance-operator attestation-scheduler key-release-gateway platform-api platform-ui thesis-bench

NAMESPACE ?= ai-platform
KIND_CLUSTER ?= ai-platform

.PHONY: help \
  generate manifests \
  test test-unit test-envtest test-scheduler \
  lint security-scan \
  build-images push-images \
  kind-up kind-down kind-load deploy-kind undeploy-kind \
  article1-build-images article1-kind-up article1-kind-down article1-load-kind article1-install article1-run-security article1-run-races article1-run-all article1-collect \
  helm-lint helm-template-kind helm-template-aks-private helm-package helm-push \
  thesis-bench \
  bootstrap-tools \
  ui-install ui-build ui-dev \
  ci-local release \
  operator-build operator-test operator-run operator-manifests \
  experimentation-test automation-local automation-up automation-down clean-artifacts

help:
	@printf "AI Confidential Governance Platform — Make Targets\n\n"
	@printf "  bootstrap-tools              Install required tools (kind, helm, govulncheck, gosec, trivy)\n"
	@printf "  generate                     Run controller-gen: deepcopy + CRDs + RBAC\n"
	@printf "  manifests                    Alias for generate\n"
	@printf "  test                         Run all Go tests\n"
	@printf "  test-unit                    Run unit tests only (no envtest)\n"
	@printf "  test-envtest                 Run envtest (requires setup-envtest)\n"
	@printf "  test-scheduler               Run scheduler unit tests\n"
	@printf "  lint                         go vet + go mod tidy check\n"
	@printf "  security-scan                govulncheck + gosec + trivy (missing tools reported)\n"
	@printf "  build-images                 Build all Docker images\n"
	@printf "  push-images                  Push images (PUSH=true REGISTRY=... required)\n"
	@printf "  kind-up                      Create kind cluster\n"
	@printf "  kind-down                    Delete kind cluster\n"
	@printf "  kind-load                    Load images into kind cluster\n"
	@printf "  deploy-kind                  Deploy platform to kind cluster\n"
	@printf "  undeploy-kind                Remove platform from kind cluster\n"
	@printf "  helm-lint                    Lint all Helm charts\n"
	@printf "  helm-template-kind           Render chart with kind values\n"
	@printf "  helm-template-aks-private    Render chart with aks-private values\n"
	@printf "  helm-package                 Package Helm chart\n"
	@printf "  helm-push                    Push chart to OCI registry (HELM_REGISTRY=... required)\n"
	@printf "  thesis-bench                 Run thesis bench inside kind cluster\n"
	@printf "  ui-install                   npm install for platform-ui\n"
	@printf "  ui-build                     Build platform-ui for production\n"
	@printf "  ui-dev                       Start platform-ui dev server\n"
	@printf "  ci-local                     Run full CI pipeline locally\n"
	@printf "  release                      Build + (optionally) push images + package charts\n"

# ─── Code generation ──────────────────────────────────────────────────────────

generate:
	$(MAKE) -C operateur manifests generate

manifests: generate

# ─── Tests ───────────────────────────────────────────────────────────────────

test:
	cd operateur && go test ./... -count=1 -timeout 120s

test-unit:
	cd operateur && go test ./internal/... ./pkg/... -count=1 -timeout 60s

test-envtest:
	$(MAKE) -C operateur test

test-scheduler:
	cd operateur && go test ./internal/scheduler/... ./pkg/token/... ./pkg/audit/... ./pkg/crypto/... -v -count=1

# ─── Lint / security ─────────────────────────────────────────────────────────

lint:
	cd operateur && go vet ./...
	cd operateur && go mod tidy -diff 2>/dev/null || true

security-scan:
	@echo "=== go vet ==="
	cd operateur && go vet ./...
	@echo "=== govulncheck ==="
	@if command -v govulncheck &>/dev/null; then cd operateur && govulncheck ./...; else echo "tool missing: govulncheck. Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; fi
	@echo "=== gosec ==="
	@if command -v gosec &>/dev/null; then cd operateur && gosec ./...; else echo "tool missing: gosec. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; fi
	@echo "=== trivy ==="
	@if command -v trivy &>/dev/null; then trivy fs --exit-code 0 .; else echo "tool missing: trivy. See https://aquasecurity.github.io/trivy"; fi
	@echo "=== helm lint ==="
	$(MAKE) helm-lint
	@echo "=== UI audit ==="
	@if [ -f platform-ui/package.json ]; then cd platform-ui && npm audit --audit-level=high 2>/dev/null || true; fi
	@echo "=== no secrets check ==="
	@! grep -rn 'PRIVATE KEY\|BEGIN EC\|BEGIN RSA\|-----BEGIN' --include="*.go" --include="*.yaml" --include="*.json" . 2>/dev/null | grep -v vendor | grep -v "_test.go"

# ─── Images ──────────────────────────────────────────────────────────────────

OPERATEUR_DIR := operateur

build-images:
	@echo "Building images (tag: $(IMAGE_TAG))"
	docker build -t $(if $(REGISTRY),$(REGISTRY)/,)controller:$(IMAGE_TAG) \
	  -f $(OPERATEUR_DIR)/Dockerfile $(OPERATEUR_DIR)
	docker build -t $(if $(REGISTRY),$(REGISTRY)/,)attestation-scheduler:$(IMAGE_TAG) \
	  -f $(OPERATEUR_DIR)/Dockerfile.scheduler $(OPERATEUR_DIR)
	docker build -t $(if $(REGISTRY),$(REGISTRY)/,)key-release-gateway:$(IMAGE_TAG) \
	  -f $(OPERATEUR_DIR)/Dockerfile.key-release-gateway $(OPERATEUR_DIR)
	docker build -t $(if $(REGISTRY),$(REGISTRY)/,)platform-api:$(IMAGE_TAG) \
	  -f $(OPERATEUR_DIR)/Dockerfile.platform-api $(OPERATEUR_DIR)
	docker build -t $(if $(REGISTRY),$(REGISTRY)/,)thesis-bench:$(IMAGE_TAG) \
	  -f $(OPERATEUR_DIR)/Dockerfile.thesis-bench $(OPERATEUR_DIR)
	@if [ -f platform-ui/src/main.tsx ]; then \
	  docker build -t $(if $(REGISTRY),$(REGISTRY)/,)platform-ui:$(IMAGE_TAG) platform-ui; \
	else \
	  docker pull nginx:1.27-alpine && docker tag nginx:1.27-alpine platform-ui:$(IMAGE_TAG); \
	fi

push-images:
	@if [ "$(PUSH)" != "true" ]; then echo "PUSH is not true — skipping. Use PUSH=true to push."; exit 0; fi
	@if [ -z "$(REGISTRY)" ]; then echo "ERROR: REGISTRY is not set"; exit 1; fi
	for img in controller attestation-scheduler key-release-gateway platform-api platform-ui thesis-bench; do \
	  docker push $(REGISTRY)/$$img:$(IMAGE_TAG); \
	done

# ─── Article 1 ───────────────────────────────────────────────────────────────

article1-build-images:
	$(MAKE) build-images REGISTRY=$(GHCR_NAMESPACE)

article1-kind-up:
	$(MAKE) kind-up

article1-kind-down:
	$(MAKE) kind-down

article1-load-kind:
	$(MAKE) kind-load

article1-install:
	$(MAKE) deploy-kind

article1-run-security:
	cd operateur && go test ./pkg/crypto ./pkg/token ./internal/webhook/podinjector ./internal/scheduler -count=1 -v

article1-run-races:
	cd operateur && go test ./pkg/token ./internal/scheduler -race -count=1

article1-run-all: article1-run-security
	$(MAKE) thesis-bench

article1-collect:
	chmod +x automation/scripts/article1-collect.sh
	./automation/scripts/article1-collect.sh

# ─── Kind ────────────────────────────────────────────────────────────────────

kind-up:
	@if ! command -v kind &>/dev/null; then echo "tool missing: kind. See https://kind.sigs.k8s.io"; exit 1; fi
	kind create cluster --name $(KIND_CLUSTER) --config deploy/kind-config.yaml --wait 90s || true

kind-down:
	kind delete cluster --name $(KIND_CLUSTER) || true

kind-load: build-images
	@for img in controller attestation-scheduler key-release-gateway platform-api platform-ui thesis-bench; do \
	  kind load docker-image $$img:$(IMAGE_TAG) --name $(KIND_CLUSTER); \
	done

deploy-kind: kind-load
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install ai-platform \
	  charts/ai-confidential-governance-platform \
	  -f charts/ai-confidential-governance-platform/values-kind.yaml \
	  --set images.tag=$(IMAGE_TAG) \
	  --namespace $(NAMESPACE) \
	  --wait --timeout 5m

undeploy-kind:
	helm uninstall ai-platform --namespace $(NAMESPACE) || true

# ─── Helm ────────────────────────────────────────────────────────────────────

helm-lint:
	@if ! command -v helm &>/dev/null; then echo "tool missing: helm. See https://helm.sh"; exit 1; fi
	helm lint charts/ai-confidential-governance-platform
	@if [ -d operateur/charts ]; then helm lint operateur/charts/ai-sovereign-finops-operator; fi

helm-template-kind:
	helm template ai-platform charts/ai-confidential-governance-platform \
	  -f charts/ai-confidential-governance-platform/values-kind.yaml

helm-template-aks-private:
	@echo "=== AKS Private template ==="
	helm template ai-platform charts/ai-confidential-governance-platform \
	  -f charts/ai-confidential-governance-platform/values-aks-private.yaml
	@echo "=== Checking: no LoadBalancer ==="
	@! helm template ai-platform charts/ai-confidential-governance-platform \
	  -f charts/ai-confidential-governance-platform/values-aks-private.yaml 2>/dev/null | grep -q 'type: LoadBalancer' \
	  && echo "OK: no LoadBalancer found" || (echo "FAIL: LoadBalancer found in aks-private template"; exit 1)
	@echo "=== Checking: simulatedEvidence=false ==="
	@! helm template ai-platform charts/ai-confidential-governance-platform \
	  -f charts/ai-confidential-governance-platform/values-aks-private.yaml 2>/dev/null | grep -q 'simulatedEvidence.*true' \
	  && echo "OK: simulatedEvidence not enabled" || echo "WARNING: check simulatedEvidence"

helm-package:
	mkdir -p dist/charts
	helm package charts/ai-confidential-governance-platform --destination dist/charts --version $(CHART_VERSION) --app-version $(VERSION)

helm-push:
	@if [ -z "$(HELM_REGISTRY)" ]; then echo "HELM_REGISTRY is not set — skipping chart push"; exit 0; fi
	helm push dist/charts/ai-confidential-governance-platform-$(CHART_VERSION).tgz oci://$(HELM_REGISTRY)

# ─── Thesis bench ─────────────────────────────────────────────────────────────

thesis-bench:
	cd operateur && go run ./cmd/thesis-bench/... --mode simulated-kind --out ./results/thesis/latest

# ─── UI ──────────────────────────────────────────────────────────────────────

ui-install:
	cd platform-ui && npm install

ui-build: ui-install
	cd platform-ui && npm run build

ui-dev:
	cd platform-ui && npm run dev

# ─── Bootstrap tools ─────────────────────────────────────────────────────────

bootstrap-tools:
	@echo "Installing/verifying tools..."
	@command -v kind &>/dev/null || (echo "Installing kind..." && go install sigs.k8s.io/kind@latest)
	@command -v helm &>/dev/null || echo "helm missing — install from https://helm.sh"
	@command -v govulncheck &>/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	@command -v gosec &>/dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	@command -v trivy &>/dev/null || echo "trivy missing — see https://aquasecurity.github.io/trivy"
	@echo "Bootstrap complete."

# ─── CI local ────────────────────────────────────────────────────────────────

ci-local: lint test-unit helm-lint helm-template-kind
	@echo "=== CI local passed ==="

# ─── Release ─────────────────────────────────────────────────────────────────

release:
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION is not set"; exit 1; fi
	$(MAKE) ci-local
	$(MAKE) build-images
	$(MAKE) helm-package
	@if [ "$(PUSH)" = "true" ]; then \
	  if [ -z "$(REGISTRY)" ]; then echo "ERROR: REGISTRY is not set for push"; exit 1; fi; \
	  $(MAKE) push-images; \
	fi
	@if [ -n "$(HELM_REGISTRY)" ]; then $(MAKE) helm-push; fi
	@echo "Release $(VERSION) complete."

# ─── Legacy aliases ──────────────────────────────────────────────────────────

operator-build:
	$(MAKE) -C operateur build

operator-test:
	$(MAKE) -C operateur test

operator-run:
	$(MAKE) -C operateur run

operator-manifests:
	$(MAKE) -C operateur manifests generate

experimentation-test:
	cd experimentation && go test ./...

automation-local:
	$(MAKE) -C automatisation local

automation-up:
	$(MAKE) -C automatisation up

automation-down:
	$(MAKE) -C automatisation down

clean-artifacts:
	rm -rf bin operateur/bin dist/
	rm -f cover.out operateur/cover.out
	rm -f experimentation/paper/latex/main.aux experimentation/paper/latex/main.bbl \
	      experimentation/paper/latex/main.log experimentation/paper/latex/main.out
