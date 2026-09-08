#!/usr/bin/env bash
# beforeShellExecution: ask before destructive compose/git/fs/db commands.
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "guard-shell hook: missing $1 on PATH" >&2
    exit 1
  fi
}

need jq

input="$(cat)"
command="$(echo "$input" | jq -r '.command // empty')"

ask() {
  local reason="$1"
  jq -n --arg reason "$reason" '{
    permission: "ask",
    user_message: $reason,
    agent_message: $reason
  }'
  exit 0
}

# Destructive volume / database / git / recursive delete
if echo "$command" | grep -Eqi 'docker[[:space:]]+compose[[:space:]]+.*down[[:space:]]+.*-v'; then
  ask "This command drops Compose volumes (database wipe). Confirm before continuing."
fi

if echo "$command" | grep -Eqi '(drop[[:space:]]+database|drop[[:space:]]+schema|drop[[:space:]]+table)'; then
  ask "This command drops database objects. Confirm before continuing."
fi

if echo "$command" | grep -Eqi 'git[[:space:]]+push[[:space:]]+.*--force'; then
  ask "Force-push is blocked until you confirm. Never force-push main/master."
fi

if echo "$command" | grep -Eqi 'git[[:space:]]+(reset[[:space:]]+--hard|clean[[:space:]]+-fd)'; then
  ask "This git command discards local work. Confirm before continuing."
fi

if echo "$command" | grep -Eqi '(^|[[:space:]])rm[[:space:]]+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)'; then
  ask "Recursive rm -rf is destructive. Confirm the path before continuing."
fi

echo '{ "permission": "allow" }'
exit 0
