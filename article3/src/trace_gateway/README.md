# Trace Gateway

Planned responsibilities:

- trace-driven replay
- synthetic delayed-settlement interface
- controlled routing experiments

Current scaffold:

- deterministic replay loop over request arrivals
- delayed settlement events
- integration with the admission layer and request ledger
- retryable queue behavior with configurable retry delay
- replay-time reservation derivation from predictor policies
