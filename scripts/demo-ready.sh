#!/usr/bin/env bash
# One-command path from a running Compose fleet to a recordable demo fixture.
# docs/10-mvp-roadmap.md § W14
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
COMPOSE="docker compose -f deploy/compose/docker-compose.yml"
PASSPHRASE="${DSP_PASSPHRASE:-demo-passphrase}"

stale="$($COMPOSE exec -T postgres psql -U dsp -d dsp -tAc \
  "SELECT count(*) FROM nodes WHERE split_part(endpoint, ':', 2) !~ '^[0-9]+$' OR split_part(endpoint, ':', 2)::int NOT BETWEEN 7443 AND 7466")"
stale="$(echo "$stale" | tr -d '[:space:]')"
if [ "$stale" != "0" ]; then
  echo "stale leftover nodes ($stale) with non-compose endpoints." >&2
  echo "run: make fleet-down && make up" >&2
  exit 1
fi

echo "waiting for 24 online agents..."
online="0"
for _ in $(seq 1 72); do
  online="$($COMPOSE exec -T postgres psql -U dsp -d dsp -tAc "SELECT count(*) FROM nodes WHERE status = 'online'")"
  online="$(echo "$online" | tr -d '[:space:]')"
  if [ "$online" = "24" ]; then
    break
  fi
  sleep 5
done
if [ "$online" != "24" ]; then
  echo "only $online/24 agents online. run: make fleet-down && make up" >&2
  exit 1
fi

mkdir -p .local
$COMPOSE cp metadata:/ca/ca.crt .local/ca.crt
$COMPOSE cp agent1:/seed/api.key .local/api.key
dd if=/dev/urandom of=.local/Vacation.mp4 bs=1048576 count=31 status=none

export DSP_API_URL="${DSP_API_URL:-http://127.0.0.1:8080}"
export DSP_CA_FILE="$ROOT/.local/ca.crt"
export DSP_API_KEY
DSP_API_KEY="$(tr -d '[:space:]' < .local/api.key)"
export DSP_PASSPHRASE="$PASSPHRASE"

go run ./cmd/dsp buckets create -name demo >/dev/null || true
go run ./cmd/dsp put .local/Vacation.mp4 demo:/Vacation.mp4

GET_CMD="DSP_API_URL=$DSP_API_URL DSP_CA_FILE=$DSP_CA_FILE DSP_API_KEY=$DSP_API_KEY DSP_PASSPHRASE=$DSP_PASSPHRASE go run ./cmd/dsp get demo:/Vacation.mp4 .local/Vacation.out"
cat > .local/demo.env <<EOF
export DSP_API_URL=$DSP_API_URL
export DSP_CA_FILE=$DSP_CA_FILE
export DSP_API_KEY=$DSP_API_KEY
export DSP_PASSPHRASE=$DSP_PASSPHRASE
export DSP_GET_CMD='$GET_CMD'
EOF

if ! go run ./cmd/harness invariants; then
  echo "invariants failed (often leftover cap-violating nodes). run: make fleet-down && make up" >&2
  exit 1
fi

echo
echo "demo ready. visualizer: $DSP_API_URL/demo/"
echo "env written to .local/demo.env"
echo "  source .local/demo.env"
echo "  make chaos-m4"
echo "  make demo"
