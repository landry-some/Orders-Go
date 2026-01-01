# Wayfinder Backend

A test-driven gRPC backend that pairs order/payment workflows with live location ingest, built with production-grade patterns. 

## Why It Stands Out
- Test-first discipline: every feature was driven by failing tests; coverage spans units and bufconn integration.
- Resilience patterns: sagas with compensation, idempotency keys, circuit breakers, ingress/egress rate limits, graceful shutdown. 
- Dual-store design: Redis for hot, shared state and bounded streams; Postgres for ACID money flow and audit history.

## Core Capabilities
- Location stream → latest state in Redis, event stream (TTL/trim) for replay/audit, durable history in Postgres.
- Order pipeline → ACID payments in Postgres, assignment to an available unit, saga log with compensation (refund on assignment failure), idempotency keys.

## Architecture
- Services: a gRPC order service (creates orders, charges payments, assigns a unit) and a gRPC ingest service (streaming location updates).
- Postgres: payments, saga metadata/steps, location history.
- Redis: fast shared state (`unit:<id>` hash) + stream for events (bounded).

## Reliability & Operations
- Outbound protections: retry with jitter, circuit breaker, egress rate limiting; context-driven timeouts.
- Ingress rate limiting for gRPC.
- Graceful shutdown with health status flip.
- Readiness: `/readyz` pings Redis + Postgres; gRPC health service.

## Observability
- `/metrics` JSON snapshot (per-method latency/errors/in-flight, rate-limit waits, uptime).
- `/readyz` checks Redis and Postgres on each call.


## Testing
- All features are fully covered by tests.
