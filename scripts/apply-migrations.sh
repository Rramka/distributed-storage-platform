#!/usr/bin/env bash
# Apply deploy/migrations/*.sql to a Postgres DSN. Idempotent (IF NOT EXISTS).
set -euo pipefail
dsn=${1:?usage: apply-migrations.sh <postgres-dsn>}
root=$(cd "$(dirname "$0")/.." && pwd)
for f in "$root"/deploy/migrations/*.sql; do
  echo "applying $f"
  psql "$dsn" -v ON_ERROR_STOP=1 -f "$f"
done
