# Artifact release & DOI (GitHub + Zenodo)

The repository is the reproducible artifact. To mint a citable DOI:

1. **Submission format:** `paper/paper.tex` compiles on Overleaf with `paper/references.bib`
   (swap `\documentclass` for IEEEtran/acmart per venue). Figures are under `../figures/`.
2. **Reproduce results:** `pip install -r requirements.txt`; `scripts/run_experiment.sh`;
   `cmd/experiment -stats-reps 30`; `scripts/analyze_results.py`, `analyze_stats.py`, `dashboard.py`.
   All LLM responses are cached, so re-runs are deterministic and free.
3. **Zenodo DOI (one external step):**
   - Sign in to Zenodo, enable the GitHub integration for `ihsenalaya/ai-sovereign-finops-operator`.
   - Create a GitHub **Release** (e.g. `v0.1.0`). Zenodo reads `.zenodo.json` (metadata) and mints a DOI.
   - Add the DOI badge to the README and `CITATION.cff`.

Metadata files are in place: `.zenodo.json`, `CITATION.cff` (repo root).

## Artifact-evaluation claims → evidence
| Claim | Evidence |
|---|---|
| Cost −71% (significant) | `results/rq1_cost.csv`, `results-stats/stats_summary.md` |
| Real EU sovereignty (0 violations, 100% availability) | `results/rq4_sovereignty.csv` |
| Objective accuracy / cost trade-off | `results-bench/rq_benchmark.csv` |
| LLM-judge over-rates wrong answers | `results/judge_vs_truth_summary.md`, `figures/fig_judge_vs_truth.png` |
| Inter-judge agreement | `results/judge_agreement_summary.md` |
| All tests pass, no skips | `experimentation/DASHBOARD.md` (journals) |
