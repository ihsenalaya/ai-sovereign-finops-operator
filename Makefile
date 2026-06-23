SHELL := /usr/bin/env bash

.PHONY: help operator-build operator-test operator-run operator-manifests experimentation-test automation-local automation-up automation-down clean-artifacts

help:
	@printf "Targets:\n"
	@printf "  operator-build        Build the Kubernetes operator\n"
	@printf "  operator-test         Run operator tests\n"
	@printf "  operator-run          Run the operator locally\n"
	@printf "  operator-manifests    Regenerate CRDs/RBAC/deepcopy\n"
	@printf "  experimentation-test  Run experimentation Go tests\n"
	@printf "  automation-local      Run local kind+Helm automation\n"
	@printf "  automation-up         Run GitOps automation\n"
	@printf "  automation-down       Delete the automation kind cluster\n"
	@printf "  clean-artifacts       Remove generated local artifacts\n"

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
	rm -rf bin operateur/bin
	rm -f cover.out operateur/cover.out
	rm -f experimentation/paper/latex/main.aux experimentation/paper/latex/main.bbl experimentation/paper/latex/main.log experimentation/paper/latex/main.out
