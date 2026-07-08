# Etat final Codex AKS - 2026-07-06

Derniere mise a jour: 2026-07-06T08:03:49Z

## Decision

Le travail technique Article 1 Q1 est termine pour le scope revendique:
attestation AKS SEV-SNP au niveau noeud/VM, scheduling attestation-aware,
token de placement verifiable, workloads AI gouvernes, comparaison B1-B5,
comparaison TOCTOU B4/B5, et tests de securite AKS reels.

Verdict courant: `GO_Q1_TECHNICAL_DRAFT`.

Ce verdict ne remplace pas la revue finale auteur: il reste la relecture
editoriale, la coherence venue/bibliographie, la strategie IP et l'approbation
auteurs.

## AKS

- Resource group: `rg-article1-confidential`.
- Cluster: `aks-article1`.
- Region: `westus2`.
- Pool confidentiel final: `conf`, `Standard_DC8as_v6`.
- Snapshot multi-noeuds final: 4 noeuds confidentiels actifs
  `Standard_DC8as_v6` avec evidence SEV-SNP reelle verifiee.
- Suppression cout: a faire/confirmer des que l'interop Windows/WSL repond.

Tentatives de verification/suppression depuis cette session:

```bash
az group exists --name rg-article1-confidential
powershell.exe -NoProfile -Command "az group exists --name rg-article1-confidential"
```

Les deux appels echouent avant Azure avec:

```text
WSL ERROR: UtilAcceptVsock:271: accept4 failed 110
```

Conclusion cout actuelle: AKS doit etre considere comme `DELETE_PENDING_UNCONFIRMED`
depuis cette session. Ne pas ecrire que la verification Azure retourne `false`
tant qu'une verification Azure reelle n'a pas ete obtenue.

Commande de fermeture a relancer quand l'interop Windows/WSL est retablie:

```bash
az group delete --name rg-article1-confidential --yes
az group exists --name rg-article1-confidential
```

La seconde commande doit retourner `false`.

## Images

- Registry final: `ghcr.io/ihsenalaya/ai-sovereign-finops-operator`.
- Version finale testee: `0.5.11`.
- ACR ne doit pas etre reintroduit.

## Resultats AKS finaux

- Attestation reelle multi-noeuds:
  `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`.
  Snapshot final: 4 noeuds confidentiels actifs, evidenceMode `real`,
  verificationStatus `Verified`.
- Workloads AI gouvernes:
  `article1/results/raw/aks/ai_workloads.csv`.
  `openai-chat-minimal`: 3/3 PASS, mediane `2216.815 ms`.
  `openai-embedding-minimal`: 3/3 PASS, mediane `705.118 ms`.
  `local-cpu-minimal`: 5/5 PASS, mediane `0.001 ms`.
- Multi-node scheduling:
  `article1/results/raw/aks/multinode_node_selection.csv`.
  Cas valides/revoques/expires/sans evidence: `4/4` resultats attendus.
- Securite principale:
  `article1/results/raw/aks/security_attacks_A1_A11.csv` et
  `article1/paper/tables/security_attack_matrix.csv`.
  A1-A10: `300/300` attaques bloquees sur AKS reel.
- A11:
  `1/1` fail-closed quand une evidence confidential GPU est demandee mais
  indisponible. Ce n'est pas une evaluation confidential GPU.
- Scheduler self-security:
  `article1/results/raw/aks/scheduler_security_tests.csv`, `30/30 PASS`.
- Identity binding:
  `article1/results/raw/aks/identity_binding.csv`, `6/6` resultats attendus.
- B4 vs B5:
  `article1/results/raw/aks/b4_vs_b5.csv`, B4 mediane `1104.5 ms`.
  B5 supprime la fenetre externe gate-to-bind et emet un token de placement.
- Performance B1-B5:
  `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` et
  `article1/results/tables/performance.csv`.
  2 warmups + 30 runs mesures par baseline, 150/150 succes mesures,
  anti-quantization PASS.
  B1 mediane `1170.5 ms`, B2 `1189.0 ms`, B3 `1168.5 ms`,
  B4 `1128.0 ms`, B5 client-observed `1167.0 ms`;
  B5 scheduler-internal phase median `216.232 ms` is reported separately.

## Documents et artefacts synchronises

- Manuscrit Overleaf: `article1/overleaf/main.pdf`.
- Copie papier: `article1/paper/manuscript/main.pdf`.
- Figures finales:
  `article1/paper/figures/security_attack_heatmap.pdf`,
  `article1/paper/figures/scheduling_latency_cdf.pdf`,
  `article1/overleaf/figures/security_attack_heatmap.pdf`,
  `article1/overleaf/figures/scheduling_latency_cdf.pdf`.
- Tables finales:
  `article1/paper/tables/security_attack_matrix.csv`,
  `article1/paper/tables/b4_b5_comparison.csv`,
  `article1/paper/tables/claim_evidence_mapping.csv`,
  `article1/results/tables/ai_workloads.csv`,
  `article1/results/tables/multinode_node_selection.csv`,
  `article1/results/tables/performance.csv`.
- Status courant:
  `article1/status/q1_final_go_nogo_report.md`,
  `article1/status/go_no_go_submission.md`,
  `article1/status/q1_execution_report.md`,
  `article1/status/tests_performed.md`,
  `article1/status/unsupported_claims_report.md`,
  `article1/status/reviewer_q1_critique_action_matrix.md`.
- Handover:
  `article1/suite.txt` contient en tete l'etat final qui supersede les anciens
  diagnostics historiques.

## Compilation et validations

Compilation LaTeX OK:

```bash
cd article1/paper/manuscript && latexmk -pdf -interaction=nonstopmode -halt-on-error main.tex
cd article1/overleaf && latexmk -pdf -interaction=nonstopmode -halt-on-error main.tex
```

Validations code/package OK:

```bash
make test-unit
go test -race ./internal/scheduler ./pkg/token ./pkg/crypto ./pkg/audit -count=1 -timeout 120s
make lint
make helm-lint
make helm-template-kind
make helm-template-aks-private
```

`go test ./...` echoue uniquement sur l'E2E Kubebuilder `kind`: d'abord cluster
`kind` absent, puis runtime Docker Desktop/WSL instable (`error getting
credentials`, kube-scheduler kind perd la leader election). Ce test reste une
validation CI/regression locale et ne doit pas etre cite comme resultat de
securite/performance du papier.

## Regles a conserver

- Les resultats principaux du papier doivent venir d'AKS reel.
- `kind` et KWOK restent CI/debug/regression uniquement.
- Ne pas revendiquer attestation pod-level.
- Ne pas revendiquer confidential GPU execution.
- Ne pas revendiquer Intel TDX.
- Ne pas revendiquer confidentialite du modele ni du service OpenAI.
- Ne pas revendiquer scalabilite cloud AKS large-scale au-dela du workload
  mesure.
