#!/usr/bin/env bash

# Usage: source scripts/run_project.sh
# Loads .env and adds Postgres client tools to PATH for this shell.

if [ -n "${BASH_SOURCE:-}" ]; then
  SCRIPT_PATH="${BASH_SOURCE[0]}"
elif [ -n "${ZSH_VERSION:-}" ]; then
  SCRIPT_PATH="${(%):-%x}"
else
  SCRIPT_PATH="$0"
fi

ROOT="$(cd -- "$(dirname -- "$SCRIPT_PATH")/.." && pwd)"

if [ -f "$ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ROOT/.env"
  set +a
else
  echo "Warning: .env not found at $ROOT/.env" >&2
fi

# Prefer PG_BIN env, then pg_config --bindir, then common Homebrew path.
PG_BIN="${PG_BIN:-}"
if [ -z "$PG_BIN" ] && command -v pg_config >/dev/null 2>&1; then
  PG_BIN="$(pg_config --bindir 2>/dev/null || true)"
fi
if [ -z "$PG_BIN" ] && [ -d "/opt/homebrew/opt/postgresql@16/bin" ]; then
  PG_BIN="/opt/homebrew/opt/postgresql@16/bin"
fi

if [ -n "$PG_BIN" ]; then
  export PATH="$PG_BIN:$PATH"
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql not found on PATH; set PG_BIN or install PostgreSQL client tools." >&2
fi

echo "Project env loaded."
echo "  DATABASE_URL=${DATABASE_URL:-<unset>}"
echo "  REDIS_URL=${REDIS_URL:-<unset>}"
echo "Postgres client path added: ${PG_BIN:-<none>}"
