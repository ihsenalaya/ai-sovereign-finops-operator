# Quality evaluation protocol (PARTIALLY DONE / EXTENSION PLANNED)

Status: the current paper uses a **single** LLM judge (GPT-4o), **one pass**, no significance test and
no human evaluation. This protocol defines the stronger evaluation required for Q1. No results here are
invented; sections marked *planned* are not yet run.

## Current (done)
- LLM-as-a-judge (GPT-4o): absolute acceptability (1–5) + pairwise vs premium reference.
- Known limitation: the **premium model is both the routing reference and the comparison anchor**,
  which can bias pairwise results toward premium (discussed in paper §6/§8).

## Planned extensions
### Multiple judges (reduce single-judge bias)
- Judges: **GPT-4o** and **Mistral Large** (and optionally a third). Run the full acceptability +
  pairwise battery with each.
- Report **inter-judge agreement**: **Cohen's Kappa** (two judges) and **Krippendorff's Alpha**
  (≥2 judges, ordinal). Disagreement bounds confidence in quality claims.
- Mitigate position/verbosity bias: randomize A/B order; length-control checks.

### Human evaluation (gold reference on a sample)
- Sample ~100–150 (prompt, answer) pairs stratified by workload and model tier.
- ≥3 human raters; blind to the routing strategy; 1–5 acceptability + pairwise.
- Report human–judge agreement (Kappa/Alpha) to validate the LLM judge as a proxy.

### Statistics
- With ≥30 repetitions: bootstrap CIs on quality; Wilcoxon signed-rank (paired, per-prompt) for
  strategy vs premium; Cliff's delta / Cohen's d effect sizes (via `scripts/stats.py`).

## Honesty
Until executed, quality claims are limited to "comparable within the evaluated scope" (single judge,
single pass). Human-evaluation and multi-judge agreement numbers must NOT be reported until collected.
