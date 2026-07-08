# Overleaf project — Article 1

**Title:** *Attestation-Aware Scheduling for Verifiable AI Placement on SEV-SNP
Confidential Kubernetes Nodes*

Self-contained Overleaf source updated after the real AKS rerun. The empirical
blockers from the previous package are resolved; remaining submission gating is
editorial/IP review and final author approval, not missing AKS security data.

## Upload to Overleaf
1. Zip this `overleaf/` folder (or upload the folder directly): `main.tex`, all
   section `*.tex`, `references.bib`, and `figures/`.
2. In Overleaf: **New Project → Upload Project** → select the zip.
3. Overleaf settings: **Compiler = pdfLaTeX**, **Main document = main.tex**.
   The bibliography uses `\bibliographystyle{IEEEtran}` + BibTeX (default).
4. Recompile twice (Overleaf runs pdfLaTeX→BibTeX→pdfLaTeX automatically).

## Structure
```
main.tex                 IEEEtran conference class; \input's all sections
abstract.tex … conclusion.tex   source sections, including properties.tex
references.bib           47 verified references (real; arXiv/venue/DOI checked)
figures/                 security_attack_heatmap, scheduling_latency_cdf,
                         toctou_window_b4_b5, identity_binding_matrix,
                         ablation_security_latency
main.bbl / main.pdf      latest local build artifacts; rebuild after edits
```

## Honesty / scope (IMPORTANT — do not overclaim)
- Main results are from **real AMD SEV-SNP AKS** (`aks-real-sevsnp`): real MAA
  attestation on node-level SEV-SNP, four active confidential nodes in the
  final multi-node snapshot, A1-A10 `300/300` blocked, A11 fail-closed
  GPU-scope, B4-vs-B5 TOCTOU (Mann-Whitney U=900, p<1e-5, Cliff delta=1.0),
  AI workloads 3/3, positive verifiable placement, identity binding 6/6, and
  scheduler self-security scope checks.
- AKS B1-B5 scheduling-latency is measured on real AKS: 30 measured runs per
  baseline plus 2 warmups; the comparable client-observed B5 median is
  1167.0 ms. B5 scheduler-internal phase median is separately 216.232 ms and
  must not be mixed with B1-B4 on the same latency axis.
- **kind/kwok are CI/debug/regression only** and are NOT presented as security or
  performance results.
- The paper claims **node-level SEV-SNP** attested placement only — not
  pod-level, not confidential GPU, not Intel TDX, not OpenAI service
  confidentiality, and not model confidentiality.
- Sections describing the placement token carry
  `% IP REVIEW REQUIRED BEFORE SUBMISSION` — resolve IP review before submitting.
- Current verdict: **technical evidence complete for the claimed scope**;
  perform final editorial, bibliography, and IP review before submission.

## Source of truth
Results trace to `article1/results/raw/aks/*` and
`article1/paper/tables/claim_evidence_mapping.csv`. The bibliography audit is in
`article1/paper/literature_audit.md`.
