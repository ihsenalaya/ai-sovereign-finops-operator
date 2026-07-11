# ExecPlans

An ExecPlan is a durable, self-contained plan for work that spans many hours or resumptions. It is a living document, but its decision and evidence history must remain auditable.

Every ExecPlan must contain:

1. objective, scope, non-goals, and authoritative acceptance contract;
2. repository/base/version facts with exact SHAs and remote evidence;
3. a dependency graph of phases, with one current phase and explicit entry/exit checks;
4. reproducible commands, expected outputs, recovery commands, and cost/destructive-action guards;
5. append-only decisions, discoveries, protocol deviations, and checkpoint commits;
6. evidence paths and checksums rather than self-reported completion;
7. independent reviewer assignments and resolution status;
8. a final clean-checkout reproduction and release-verification procedure.

Update the plan whenever evidence changes the approach. Do not mark a phase complete until its acceptance commands pass. If interrupted, resume from the last verified checkpoint rather than rerunning or trusting partial output.
