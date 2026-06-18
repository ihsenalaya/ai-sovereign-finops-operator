# Convenience targets for the end-to-end demo automation.
# Override variables, e.g.: make up REPO_URL=https://github.com/me/greenops REVISION=main

SHELL := /usr/bin/env bash
SCRIPTS := scripts
REPORT ?= ai-report-all
NAMESPACE ?= default
EXPORT_DIR ?= ../exports/reports

.PHONY: up up-gitea down cluster image argocd gitea apps wait local shadow-ai export-report help real-demo real-demo-reset real-demo-verify real-demo-test real-demo-down full-demo

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

up: ## GitOps bring-up from the workspace origin/REPO_URL (kind + image + ArgoCD + Apps + wait)
	@$(SCRIPTS)/up.sh

up-gitea: ## Self-contained GitOps bring-up with in-cluster Gitea
	@GITOPS_SOURCE=gitea $(SCRIPTS)/up.sh

local: ## Offline bring-up (kind + image + Helm + samples + Shadow-AI, no ArgoCD)
	@$(SCRIPTS)/install-local.sh

shadow-ai: ## Install/refresh Tetragon + shadow-ai rogue workload + shadow-egress
	@$(SCRIPTS)/06-install-shadow-ai.sh

export-report: ## Export report.md/report.json from the operator report ConfigMap
	@mkdir -p "$(EXPORT_DIR)/$(REPORT)"
	@kubectl -n "$(NAMESPACE)" get cm "$(REPORT)-report" -o jsonpath='{.data.report\.md}' > "$(EXPORT_DIR)/$(REPORT)/report.md"
	@kubectl -n "$(NAMESPACE)" get cm "$(REPORT)-report" -o jsonpath='{.data.report\.json}' > "$(EXPORT_DIR)/$(REPORT)/report.json"
	@echo "Exported $(REPORT) to $(EXPORT_DIR)/$(REPORT)/report.md and report.json"

real-demo: ## Full zero-manual demo: 4 apps + AI Gateway + Tetragon shadow-AI
	@envoy-aigw/deploy.sh up

real-demo-reset: ## Delete/recreate kind, then run the full zero-manual real demo
	@RESET_CLUSTER=true envoy-aigw/deploy.sh up

real-demo-verify: ## Bounded real demo: collect evidence, stop apps, delete kind on exit
	@RESET_CLUSTER=true envoy-aigw/deploy.sh verify

real-demo-test: ## Sanity call through the real AI Gateway demo
	@envoy-aigw/deploy.sh test

real-demo-down: ## Remove real AI Gateway demo workloads (keeps control planes)
	@envoy-aigw/deploy.sh down

full-demo: ## Alias: full zero-manual real demo
	@envoy-aigw/deploy.sh up

cluster: ## Create the kind cluster
	@$(SCRIPTS)/01-create-cluster.sh

image: ## Build and load the operator image into kind
	@$(SCRIPTS)/02-build-load-image.sh

argocd: ## Install ArgoCD
	@$(SCRIPTS)/03-install-argocd.sh

gitea: ## Install in-cluster Gitea and seed the repo
	@$(SCRIPTS)/03b-install-gitea.sh

apps: ## Register the repo + ArgoCD AppProject + Applications
	@$(SCRIPTS)/04-bootstrap-apps.sh

wait: ## Wait for ArgoCD Applications to be Synced + Healthy
	@$(SCRIPTS)/05-wait-sync.sh

down: ## Delete the kind cluster
	@$(SCRIPTS)/down.sh
