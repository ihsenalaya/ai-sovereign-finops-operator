# Experiment registry

`frozen_protocol.yaml` will be created only after the development-data pilot and power/precision review. Once frozen, it is immutable; changes require a new version and a deviation record.

For RouterEval MATH, the protocol must separately bind the model catalog, each
physical feature split, and each corresponding `oracle_only` outcome split.
Online methods may receive only selected-model feedback; the full aligned
matrix is evaluator-only. The RouterEval GSM8K and ARC members are excluded
because they were accidentally inspected during exploratory recovery and must
not appear in a frozen data hash set.
