#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="${1:-$SCRIPT_DIR/../agent/actionagentd}"
CFG_PATH="${2:-$SCRIPT_DIR/../agent/actionAgent.json}"

BIN_PATH="$(cd "$(dirname "$BIN_PATH")" && pwd)/$(basename "$BIN_PATH")"
CFG_PATH="$(cd "$(dirname "$CFG_PATH")" && pwd)/$(basename "$CFG_PATH")"

mkdir -p "$(dirname "$CFG_PATH")"

if [[ $# -gt 2 ]]; then
  EXTRA_ARGS=("${@:3}")
else
  EXTRA_ARGS=()
fi

exec "$BIN_PATH" --config "$CFG_PATH" "${EXTRA_ARGS[@]}"
