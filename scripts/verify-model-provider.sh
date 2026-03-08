#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-http://127.0.0.1:8787}"
MODEL="${2:-gpt-4o-mini}"
MESSAGE="${3:-Say hello in one sentence.}"

curl -fsS "$BASE_URL/healthz" >/dev/null

curl -fsS -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"${MODEL}\",
    \"messages\": [
      {\"role\": \"user\", \"content\": \"${MESSAGE}\"}
    ]
  }"

