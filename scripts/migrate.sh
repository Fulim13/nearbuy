#!/usr/bin/env bash
# Usage: ./scripts/migrate.sh <service> [up|down|...]   e.g. ./scripts/migrate.sh user up
set -euo pipefail

SVC="${1:?service name required (user|shop|voucher|order|dtm)}"
shift

HOST="${PG_HOST:-localhost}"
PORT="${PG_PORT:-5432}"
DSN="postgres://postgres:postgres@${HOST}:${PORT}/${SVC}_db?sslmode=disable"

if [[ "$SVC" == "dtm" ]]; then
	MIG_DIR="deploy/dev/sql/dtm"
else
	MIG_DIR="services/${SVC}-service/database/migrations"
fi

for _ in $(seq 1 60); do
	pg_isready -h "$HOST" -p "$PORT" -U postgres -q && break
	sleep 1
done

migrate -path "$MIG_DIR" -database "$DSN" "$@"
