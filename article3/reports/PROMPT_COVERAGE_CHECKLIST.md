# Prompt Coverage Checklist

This checklist records the current coverage status of `article 3/propmt.txt`
against the material implemented in `article3/`.

## Section coverage

| Prompt section | Status | Evidence |
|---|---|---|
| I. Regles absolues | partially covered | provenance files, protocol deviation log, environment capture, no secret persistence in article3 artifacts |
| II. Objectif scientifique | covered at scaffold level | `design/problem_formulation.md`, `design/gov_ar_algorithm.md` |
| III. Contributions a demontrer | partially covered | formulation, scaffold algorithm, delayed-settlement replay and experiments E0-E7 except final operator coupling |
| IV. Audit complet de l'operateur | covered | `operator_audit/` |
| V. Structure du repertoire article3 | covered | `article3/` tree populated |
| VI. Etude bibliographique | partially covered | `literature/` populated and expanded with additional primary sources on FrugalGPT, RouteLLM, Hybrid LLM, conformal routing, InferenceDynamics, FLARE, and MTRouter, but final saturation pass still pending |
| VII. Decouverte et utilisation d'Azure | covered at validation level | `experiments/raw/E6_azure_live.json`, `infra/azure/run_e6_live.py` |
| VIII. Conception de GOV-AR | covered at scaffold level | `design/` |
| IX. Integration dans l'operateur | partial | shared CRD-to-GOV-AR snapshot code, HTTP service, optional Helm deployment, optional PostgreSQL-backed ledger, and proxy-triggered admit/settle/cancel path added under `operateur/internal/govar`, `operateur/cmd/gov-ar-admission`, and `operateur/internal/sidecarproxy`, but final Envoy-native integration still pending |
| X. Tests logiciels obligatoires | partially covered | `go test ./...`, replay tests, fault injector tests, smoke artifacts; broader operator-path additions still pending |
| XI. Build, GHCR et Helm | partial | Docker build, Helm lint/package, and local Kind deployment verified; GHCR publication blocked by package-write denial documented in `reports/GHCR_PUBLICATION_STATUS.md` |
| XII. Infrastructure Kind automatisee | covered locally | `infra/kind/`, cluster created, Helm job completed in Kind |
| XIII. Datasets et traces | partial | synthetic trace generation implemented; broader dataset preparation remains light |
| XIV. Protocole experimental | covered | `experiments/registry/frozen_protocol.yaml`, configs, registry, statistical plan, protocol deviations |
| XV. E0 | covered | smoke run outputs |
| XVI. E1 | covered | raw, processed, campaign, matrix, comparison outputs |
| XVII. E2 | covered | raw, processed, campaign, matrix, comparison outputs |
| XVIII. E3 | covered | drift outputs |
| XIX. E4 | covered | fault outputs |
| XX. E5 | covered | scalability outputs |
| Azure live extension E6 | covered | live Azure outputs |
| E7 ablation | covered | ablation outputs |
| Final article, PDF, Overleaf, replication bundle | partially covered | manuscript hardened and recompiled, PDF produced, Overleaf zip produced, replication bundle zip produced, and prompt-named final artifact reports materialized; final submission polish still pending |

## Important remaining gaps

- final Envoy-native operator integration beyond the current proxy-triggered `gov-ar-admission` path
- completion and hardening of the literature review to a true saturation point
- final submission polish for the paper
- GHCR publication and final artifact hardening
