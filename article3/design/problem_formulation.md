# Problem Formulation

## Working Title

GOV-AR: Risk-Bounded Multi-Tenant LLM Admission and Routing under Delayed Token-Cost Feedback

## Context

The current operator can:

- observe LLM usage
- compute post-hoc spend
- evaluate budget phases
- recommend cheaper or more compliant models
- reroute or block in specific enforcement paths

However, enterprise multi-tenant AI gateways face a harder online decision problem:

- the final output token cost is unknown before completion
- multiple requests are in flight concurrently
- telemetry and cost settlement can arrive late
- each tenant has isolated budget constraints
- governance constraints exclude some providers or regions
- route changes must remain auditable and enforceable

## Decision Problem

For each incoming request `r` from tenant `t`, the system must choose one action from:

- `admit(model, reservation)`
- `queue(reservation_hint)`
- `reject(reason)`
- `abstain(reason)`

subject to:

- hard governance constraints
- tenant budget limits
- uncertainty on output token count
- finite in-flight liability capacity
- latency and quality guardrails

## State Available at Decision Time

At decision time, GOV-AR may know:

- tenant identity and policy scope
- application, namespace, or team labels
- prompt-side observable features
- admissible models after sovereignty and compliance filtering
- historical token-length and quality statistics
- already reserved but unsettled budget for the tenant
- settled spend for the active budget window
- queue occupancy and recent drift indicators

At decision time, GOV-AR does not yet know:

- exact output token count
- exact final request cost
- whether delayed telemetry for prior requests will revise current estimates

## Formal Goal

For a stream of requests across tenants, maximize policy-compliant utility while controlling budget risk.

Informally, GOV-AR should:

1. never violate hard governance constraints
2. keep tenant budget overshoot bounded under explicit assumptions
3. prefer higher-value model assignments when risk permits
4. fall back conservatively when evidence is insufficient or drift is detected

## Distinction from Existing Routing Work

Existing LLM routing papers typically focus on:

- choosing the best model for cost-quality trade-offs
- preference learning or bandit-style adaptation
- static or average budget constraints

GOV-AR targets the stricter setting where:

- output cost is only partially observed at admission time
- the system must account for in-flight liabilities
- settlement is delayed
- multiple tenants compete under isolated budgets
- hard sovereignty and provider constraints apply
- decisions can include queueing or abstention, not just model selection

## Proposed Abstraction

We model each request as consuming two budget views:

1. settled budget
   - cost already measured and committed

2. reserved budget
   - risk-adjusted hold for in-flight requests whose final cost is not yet known

Available budget at time `k` is therefore:

`available_t(k) = budget_t - settled_t(k) - reserved_t(k)`

where `reserved_t(k)` depends on predictive uncertainty and risk policy.

## Research Questions

RQ1. How should a gateway reserve budget for an LLM request before the final output token count is known?

RQ2. How should concurrent in-flight reservations be incorporated into routing and admission decisions across multiple tenants?

RQ3. How can delayed settlement be reconciled without breaking auditability, budget isolation, or enforcement semantics?

RQ4. What utility and overhead trade-offs result from strict versus risk-bounded admission modes?

## Article Boundary

This article should not claim to solve all governance features of the operator.

Its scientific focus is narrower:

- a new online decision method
- implemented within the operator ecosystem
- evaluated via trace-driven and live experiments
