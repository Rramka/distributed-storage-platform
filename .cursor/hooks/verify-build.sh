#!/usr/bin/env bash
# stop: require a green short test run before the agent may declare victory.
# On failure, return followup_message so the agent fixes the build (loop_limit: 2).
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "verify-build hook: missing $1 on PATH" >&2
    exit 1
  fi
}

need jq
need go

input="$(cat)"
status="$(echo "$input" | jq -r '.status // "completed"')"
loop_count="$(echo "$input" | jq -r '.loop_count // 0')"

if [[ "$status" != "completed" ]]; then
  echo '{}'
  exit 0
fi

if [[ ! -f go.mod ]]; then
  echo '{}'
  exit 0
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

set +e
go build ./... >"$tmp" 2>&1
build_ec=$?
if [[ $build_ec -eq 0 ]]; then
  go test ./... -short >>"$tmp" 2>&1
  test_ec=$?
else
  test_ec=1
fi
set -e

if [[ $build_ec -eq 0 && $test_ec -eq 0 ]]; then
  echo '{}'
  exit 0
fi

log="$(head -c 4000 "$tmp")"
jq -n --arg log "$log" --arg lc "$loop_count" '{
  followup_message: (
    "Build or tests failed (loop " + $lc + "). Fix the failures, then stop. Do not claim the task is done while this is red.\n\n```\n" + $log + "\n```"
  )
}'
