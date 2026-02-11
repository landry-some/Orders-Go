# Orders-Go

Orders-Go is a Go-based backend service that models a resilient order processing system. It demonstrates production-grade engineering practices including sagas, idempotency, observability, and test-driven development.

---

## Features

- Test-driven development (TDD)
- Saga pattern with compensation workflows
- Idempotency keys for safe retries
- Circuit breakers and graceful shutdown
- Ingress and egress rate limiting
- Redis for shared state and stream processing
- Postgres for ACID-compliant order storage and audit history
- Structured logging and observability support

---

## Getting Started

Clone the repository:

```bash
git clone https://github.com/<your-username>/Orders-Go.git
cd Orders-Go

go mod download

go run ./cmd/server

go test ./...

go build ./cmd/server
