# Immutable Q1 Gate Contract — Article 3

This contract is external to the agent’s self-reported status. It defines the minimum evidence required before Article 3 can be called complete. The agent may add stricter checks but must not weaken these requirements.

## 1. Integrity

- All previous Codex-era Article 3 data, figures, tables, PDFs, and reports are archived and excluded from final analyses unless regenerated under the frozen protocol.
- Every final numerical claim is linked to raw data, processing code, statistical output, and a figure/table through `claims_to_evidence.csv`.
- No result is entered manually in LaTeX.
- No secret, credential, private key, access token, or unredacted private endpoint appears in Git or release packages.

## 2. Literature and novelty

- A documented current search covers primary research through the execution date.
- The final bibliography contains at least 40 verified relevant references unless a written saturation analysis justifies fewer.
- Metadata and review status are verified from primary sources.
- The closest prior work is identified and compared dimension by dimension.
- An independent novelty auditor and scientific red-team reviewer have no unresolved critical or major finding.

## 3. Operator and implementation

- The actual remote base version and SHA are recorded; no assumed version is used.
- All relevant existing tests pass.
- New unit, concurrency, race, transaction, integration, gateway, and Kind tests pass.
- The measured path is real: gateway → admission → atomic reservation → selected backend/provider → usage → idempotent settlement.
- No CRD is created per request.
- Duplicate, missing, late, and reordered events are handled according to the documented state machine.
- Docker images and Helm chart are built, tested, pushed with immutable tags, and recorded by digest.

## 4. Data and protocol

- Dataset sources, licenses, checksums, preparation scripts, and frozen split hashes exist.
- Train, calibration, development, and test data are separated.
- `frozen_protocol.yaml` is not marked draft.
- Any post-freeze change has a protocol-deviation record and affected runs are invalidated and rerun.
- All baselines are run on identical traces, seeds, budgets, prices, delays, failures, policies, and model outcome tables.

## 5. Minimum experimental evidence

Unless a stronger frozen power/precision analysis requires more:

- principal trace-driven comparisons: at least 10 independent seeds and 10,000 request events per principal cell;
- secondary sensitivity cells: at least 5,000 request events;
- multi-tenant principal scenarios: at least 10 seeds and 50,000 events per seed;
- every final fault-injection scenario: at least 100 post-fix independent repetitions;
- primary Kind performance points: at least three independent cluster recreations and at least 10 minutes of steady measurement per point;
- Azure live validation: at least 300 successful requests per model, at least three independent windows, and at least three deployments when available within the configured cost cap;
- all attempted, successful, retried, failed, excluded, and analyzed observations are counted from raw data.

The experiment suite must cover:

- concurrency and delayed settlement;
- budget risk–utilization trade-off;
- multi-tenant isolation and noisy-neighbor behavior;
- output-length and policy drift;
- duplicate/lost/late/out-of-order settlement and crash recovery;
- scalability and overhead;
- bounded live Azure validation;
- component ablations.

## 6. Statistics

- Primary metrics and tests were frozen before final test outcomes were examined.
- Matched seeds/streams are used for paired comparisons.
- Confidence intervals, exact rare-event bounds, effect sizes, and multiple-comparison correction are reported as appropriate.
- An independent statistical auditor recomputes counts, metrics, confidence intervals, tests, and figure source data from immutable raw data.
- Null and negative results are retained.

## 7. Manuscript

- The target journal is verified as current Q1 and in scope at submission preparation time.
- Current official author instructions and template are used.
- The manuscript is a substantive journal article, normally 8,000–14,000 words, not a short scaffold.
- No TODO, TBD, placeholder, “not yet evaluated,” “scaffold,” or knowingly unsupported completion language remains.
- Abstract, results, discussion, and conclusion agree with the evidence.
- Assumptions distinguish deterministic safety, conditional probability bounds, empirical calibration, trace-driven findings, Kind-local performance, and Azure live validation.
- The paper does not claim legal compliance, universal cloud performance, or production scale without evidence.

## 8. Final reproducibility and artifacts

- A clean checkout can rebuild the software, deploy the Kind path, reproduce primary tables/figures, and compile the manuscript.
- The Overleaf ZIP compiles without external local files or absolute paths.
- The replication package contains code, configs, manifests, checksums, analysis, and permitted data or download scripts.
- PDF, Overleaf ZIP, replication package, final report, test report, literature report, experiment summary, Azure summary, image/chart digests, and claims-to-evidence files exist only after all other checks pass.
- A release-verifier subagent reports no unresolved critical or major issue.

`python3 article3/tools/q1_gate.py --strict` may return success only when it has recomputed these facts rather than trusting prose or status labels.
