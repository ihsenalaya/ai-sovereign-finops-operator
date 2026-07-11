# Statistical Plan

## Objectives

The statistical analysis must quantify whether GOV-AR improves budget safety and governance-aware utility under delayed cost feedback.

## Primary Outcomes

1. budget violation rate
   - fraction of runs or requests where tenant spend exceeds allowed bound

2. overshoot magnitude
   - amount by which actual settled spend exceeds allowed budget

3. admit utility
   - utility of admitted requests under the evaluation scoring rule

4. governance violation count
   - should be zero under valid operation

## Secondary Outcomes

- queue rate
- reject rate
- abstain rate
- average reservation slack
- under-reservation frequency
- latency overhead
- controller reconciliation overhead
- calibration error of reservation predictor

## Comparisons

Planned comparisons include:

- GOV-AR strict vs GOV-AR risk-bounded
- GOV-AR vs observed-spend-only baseline
- GOV-AR vs no-reservation routing baseline
- ablations removing predictor, risk budget, or queueing

## Experimental Unit

Depending on experiment:

- request-level unit for online decision traces
- tenant-run unit for budget outcomes
- campaign-run unit for aggregated system outcomes

## Confidence Intervals and Tests

Unless assumptions clearly fail:

- bootstrap confidence intervals for mean or median differences
- paired analysis when the same trace is replayed under several methods
- non-parametric tests when distributions are skewed
- multiplicity correction for families of related comparisons

## Frozen Protocol Principle

The protocol must be frozen before final test execution.

Any post-freeze fix requires:

- invalidating affected results
- rerunning affected campaigns
- logging the deviation in `protocol_deviations.csv`
