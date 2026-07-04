# Quality Gate Statistics

## Overview

The platform enforces statistical quality gates before allowing AI model traffic routing changes. The gate logic is in [operateur/pkg/qualitystats/qualitystats.go](../../operateur/pkg/qualitystats/qualitystats.go).

## Non-inferiority test

The non-inferiority test determines whether a new provider is statistically not worse than the reference provider by more than a defined margin.

```
H₀: μ_new ≤ μ_ref - δ   (new provider is inferior by more than margin δ)
H₁: μ_new > μ_ref - δ   (new provider is non-inferior)
```

Parameters:
- `RefMean` — reference provider's observed mean score
- `TestMean` — new provider's observed mean score
- `Pooled StdDev` — pooled standard deviation across both samples
- `RefN`, `TestN` — sample sizes
- `Margin` — non-inferiority margin δ (default 0.05 = 5%)
- `Alpha` — significance level (default 0.05)

The test computes a one-sided z-score and compares against `z_alpha`. If `z > z_alpha`, reject H₀ → new provider is non-inferior.

## Required sample size

Before running the test, compute the minimum required sample size:

```
N = ((z_alpha + z_beta)² * 2 * σ²) / δ²
```

Where:
- `z_alpha` = 1.645 for α = 0.05
- `z_beta` = 0.842 for β = 0.20 (80% power)
- `σ` = expected standard deviation
- `δ` = non-inferiority margin

The function `RequiredSampleSize(stddev, margin, alpha, power float64) int` returns this value.

## Composite score

The composite score aggregates multiple quality dimensions into a single score [0,1]:

```
CompositeScore = w_latency * LatencyScore + w_error * ErrorScore + w_cost * CostScore
```

Default weights: `latency=0.4, error=0.4, cost=0.2`

Component scores:
- `PercentileScore(p99ms, baseline)` — score based on P99 latency vs baseline
- `ErrorRateScore(rate)` — score = 1 - error_rate (capped at 0)
- `CostScore(costUSD, baseline)` — score based on cost per request vs baseline

## Hysteresis

To prevent oscillation between providers, the routing decision applies hysteresis:

```
if current == A:
    switch to B only if B_score > A_score + hysteresisDelta
if current == B:
    switch to A only if A_score > B_score + hysteresisDelta
```

`ApplyHysteresis(currentScore, candidateScore, currentName, candidateName, delta float64) string`

Default `hysteresisDelta = 0.05`.

## Thesis experimental connection

The quality gate statistics underpin the baseline scenarios:
- **B3** — baseline quality measurement with no policy violations
- **B5** — quality measurement with GPU workloads (simulated)

The attack scenarios that affect quality:
- **attack01** — fake label causes misplacement, degrading quality metrics
- **attack11** — TTL bypass leads to stale evidence, which would degrade actual TEE quality in production

The thesis bench collects `CompositeScore` and `NonInferiorityResult` for each scenario and writes them to `results.json` for analysis.

## Statistical validity notes

- The sample sizes used in thesis experiments (N=30 per scenario) meet the `RequiredSampleSize` floor for `stddev=0.1, margin=0.05, alpha=0.05, power=0.80`
- All latency and error measurements in the thesis bench are from real Go benchmark calls, not fabricated constants
- Cost metrics in kind are synthetic (no real billing API) — this is acknowledged in the report output
