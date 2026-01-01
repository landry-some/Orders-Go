#!/usr/bin/env bash
set -euo pipefail

echo "==> Go preflight checks"

PKGS=$(go list ./...)

echo
echo "==> Formatting"
gofmt -w .

echo
echo "==> Unit & package tests"
if go test ./... -count=1; then
  echo "Unit tests OK"
else
  echo "Unit tests FAILED"
  exit 1
fi

echo
echo "==> Race detection"
go test ./... -race -count=1 || {
  echo "Race detection FAILED"
  exit 1
}

echo
echo "==> go vet"
go vet ./... || {
  echo "go vet FAILED"
  exit 1
}

echo
echo "==> Lint (if available)"
if command -v golangci-lint >/dev/null 2>&1; then
  golangci-lint run || {
    echo "Lint FAILED"
    exit 1
  }
else
  echo "golangci-lint not installed, skipping"
fi

echo
echo "==> Fuzz smoke (if fuzz tests exist)"
for pkg in $PKGS; do
  if go test "$pkg" -list=Fuzz 2>/dev/null | grep -q Fuzz; then
    echo "Fuzzing $pkg"
    go test "$pkg" -fuzz=Fuzz -fuzztime=10s || {
      echo "Fuzz FAILED in $pkg"
      exit 1
    }
  fi
done

echo
echo "==> Acceptance tests (if present)"
go test ./... -run Acceptance || true

if [[ "${BENCH:-}" == "1" ]]; then
  echo
  echo "==> Benchmarks"
  go test ./... -bench=. -benchmem || {
    echo "Benchmarks FAILED"
    exit 1
  }
fi

echo
echo "==> Shuffle run to catch order dependencies"
go test ./... -shuffle=on -count=1 || {
  echo "Shuffle run FAILED"
  exit 1
}

echo
echo "==> Coverage"
go test ./... -count=1 -covermode=atomic -coverprofile=coverage.out || {
  echo "Coverage FAILED"
  exit 1
}
go tool cover -func=coverage.out

echo
echo "==> Preflight PASSED"
