#!/usr/bin/env bash
set -euo pipefail

N="${1:-100}"
CONCURRENCY="${2:-10}"

if ! command -v grpcurl >/dev/null 2>&1; then
  echo "grpcurl is required (brew install grpcurl)" >&2
  exit 1
fi

echo "Sending $N requests with concurrency $CONCURRENCY..."

# Driver updates
seq "$N" | xargs -n1 -P"$CONCURRENCY" -I{} sh -c '
  grpcurl -plaintext -d "{\"driverId\":\"bench-{}\",\"latitude\":1.23,\"longitude\":4.56,\"timestamp\":\"2024-01-02T03:04:05Z\"}" \
    localhost:50051 driver.DriverService/UpdateLocation >/dev/null
'

# Order creations (unique idempotency keys per request)
seq "$N" | xargs -n1 -P"$CONCURRENCY" -I{} sh -c '
  grpcurl -plaintext -d "{\"userId\":\"bench-user\",\"amount\":9.99,\"idempotencyKey\":\"bench-idem-{}\"}" \
    localhost:50051 order.OrderService/CreateOrder >/dev/null
'

echo "Done. Hit /metrics to see counters:"
echo "  curl -s http://localhost:9090/metrics | jq"
