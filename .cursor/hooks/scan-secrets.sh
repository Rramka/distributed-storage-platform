#!/usr/bin/env bash
# beforeSubmitPrompt: block prompts that look like they contain key material.
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "scan-secrets hook: missing $1 on PATH" >&2
    exit 1
  fi
}

need jq

input="$(cat)"
prompt="$(echo "$input" | jq -r '.prompt // .content // empty')"

# Heuristics for private keys, PEM blocks, and high-entropy key-looking assignments.
if echo "$prompt" | grep -Eq -- '-----BEGIN ([A-Z0-9 ]+)?PRIVATE KEY-----'; then
  echo '{"continue":false,"user_message":"Prompt looks like it contains a PEM private key. Remove the secret and retry."}'
  exit 0
fi

if echo "$prompt" | grep -Eqi -- '(aws_secret_access_key|stripe_secret_key|openai_api_key|sk_live_[0-9a-zA-Z]+)'; then
  echo '{"continue":false,"user_message":"Prompt looks like it contains a live API secret. Remove the secret and retry."}'
  exit 0
fi

if echo "$prompt" | grep -Eqi -- 'master[_-]?key[[:space:]]*=[[:space:]]*['\''\"]?[A-Za-z0-9+/=]{32,}'; then
  echo '{"continue":false,"user_message":"Prompt looks like it contains a master key. Master Keys never leave the client and must not be pasted into chat."}'
  exit 0
fi

echo '{"continue":true}'
exit 0
