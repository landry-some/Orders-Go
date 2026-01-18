
## Description
- Test-first discipline: every feature was driven by failing tests.
- Resilience patterns: sagas with compensation, idempotency keys, circuit breakers, ingress/egress rate limits, graceful shutdown.
- Hot/cold split: Redis for hot shared state and bounded streams; Postgres for ACID money flow and audit history.
