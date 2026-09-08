#!/usr/bin/env bash
# afterFileEdit: gofmt (and goimports if present) the edited .go file, then go vet its package.
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "gofmt hook: missing $1 on PATH" >&2
    exit 1
  fi
}

need jq
need gofmt

input="$(cat)"
file_path="$(echo "$input" | jq -r '.file_path // empty')"

if [[ -z "$file_path" || "$file_path" != *.go ]]; then
  exit 0
fi

if [[ ! -f "$file_path" ]]; then
  exit 0
fi

gofmt -w "$file_path"

if command -v goimports >/dev/null 2>&1; then
  goimports -w "$file_path"
fi

pkg_dir="$(dirname "$file_path")"
# vet the package; do not fail the hook on generated stubs that are not yet buildable as a module package
if [[ -f go.mod ]]; then
  go vet "./${pkg_dir#./}/..." >/dev/null 2>&1 || go vet "./${pkg_dir#./}" >/dev/null 2>&1 || true
fi

exit 0
