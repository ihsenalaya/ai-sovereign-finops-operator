# Novelty Assessment

## Initial Assessment

The verified primary-source set now suggests that GOV-AR still occupies a distinct intersection:

- LLM routing under budget constraints exists
- LLM cascades and cost-aware strong-versus-weak routing exist
- quality-aware dynamic query routing exists
- conformal safety-gated routing exists
- delayed feedback theory exists
- contextual bandits with knapsack constraints exist
- output-length prediction for serving exists

What is not yet evidenced by the initial pass is a single method combining:

- multi-tenant budget isolation
- unknown output cost before response completion
- in-flight liability reservation
- delayed cost settlement
- admit / queue / reject / abstain decisions
- hard sovereignty and provider governance constraints
- Kubernetes-native enforcement

## Updated comparison after primary-source expansion

- `FrugalGPT` establishes cost-aware cascades, but not request-time reservation,
  delayed settlement, or governance constraints.
- `RouteLLM` and `Hybrid LLM` establish query-dependent routing, but still as
  cost-quality routing rather than budget-safe multi-tenant admission.
- `Adaptive LLM Routing under Budget Constraints` is the closest budget-aware
  routing baseline found so far, but it frames the problem around bandit
  routing rather than reserve-settle liability accounting under delayed token
  cost feedback.
- `Conformal LLM Routing with Distribution-Free Safety Guarantees` adds a
  valuable safety guarantee for routing mistakes, but not tenant-isolated
  budget risk or atomic financial settlement.
- `InferenceDynamics` and `FLARE` strengthen the recent evidence that routing is
  moving toward broader model pools and explicit efficiency modeling, including
  cost and length signals. However, they still optimize per-query selection
  rather than tenant-isolated reserve-settle accounting under delayed cost
  feedback.
- `MTRouter` extends cost-aware routing into multi-turn agent trajectories,
  which is important for long-horizon interaction. Even there, the focus
  remains trajectory-level routing efficiency rather than atomic budget
  reservation and delayed settlement in a governed gateway.

## Caution

This is still not a final novelty gate. Novelty claims must remain provisional until:

- more 2023-2026 routing papers are screened
- multi-tenant gateway papers are examined
- reserve-settle or budget reservation systems literature is checked more deeply
- the final literature pass before manuscript freeze confirms no directly
  matching prior method
