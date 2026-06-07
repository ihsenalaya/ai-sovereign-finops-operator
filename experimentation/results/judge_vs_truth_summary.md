# Judge vs ground truth — n=300 (objective benchmark items)

- Overall exact-match accuracy: **50.7%**
- Mean judge score — **correct: 4.49**, **incorrect: 4.14** (scale 1-5, n_correct=152, n_incorrect=148)
- **% of *incorrect* answers the judge rated acceptable (>=3): 81.1%**
- Point-biserial correlation (correct vs judge score): **r=0.123** (p=0.033)

## Per-model exact-match accuracy
| model | n | accuracy % | mean judge |
|---|---:|---:|---:|
| gpt-4.1-nano | 100 | 41.0 | 4.00 |
| gpt-4o | 100 | 66.0 | 4.62 |
| gpt-4o-mini | 100 | 45.0 | 4.32 |

**Finding.** On objective tasks the LLM judge assigns high acceptability even to many
*wrong* answers, so judge-based quality over-estimates true correctness — this explains the
exact-match vs judge contradiction. Consequence for the paper: report judge quality on
open-ended workloads only, and use exact-match on tasks with ground truth; do not claim
quality preservation from the judge alone.
