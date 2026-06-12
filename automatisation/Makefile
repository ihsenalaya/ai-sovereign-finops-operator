# Convenience targets for the end-to-end demo automation.
# Override variables, e.g.: make up REPO_URL=https://github.com/me/greenops REVISION=main

SHELL := /usr/bin/env bash
SCRIPTS := scripts

.PHONY: up down cluster image argocd gitea apps wait local help real-demo real-demo-test real-demo-down full-demo

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

up: ## Full self-contained GitOps bring-up (kind + image + ArgoCD + Gitea + Apps + wait)
	@$(SCRIPTS)/up.sh

local: ## Offline bring-up (kind + image + Helm, no ArgoCD)
	@$(SCRIPTS)/install-local.sh

real-demo: ## Real AI Gateway demo (kind + operator + Envoy + apps + Grafana)
	@envoy-aigw/deploy.sh up

real-demo-test: ## Sanity call through the real AI Gateway demo
	@envoy-aigw/deploy.sh test

real-demo-down: ## Remove real AI Gateway demo workloads (keeps control planes)
	@envoy-aigw/deploy.sh down

full-demo: ## Real AI Gateway demo + Tetragon shadow-AI plane
	@tetragon/demo.sh

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
