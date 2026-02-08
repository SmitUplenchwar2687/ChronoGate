#!/usr/bin/env bash
set -euo pipefail

FILE="${FILE:-recordings.json}"
ALGORITHM="${ALGORITHM:-token_bucket}"
RATE="${RATE:-10}"
WINDOW="${WINDOW:-1m}"
BURST="${BURST:-10}"
SPEED="${SPEED:-0}"
KEYS="${KEYS:-}"
ENDPOINTS="${ENDPOINTS:-}"

if [[ ! -f "$FILE" ]]; then
  echo "recording file not found: $FILE" >&2
  exit 1
fi

cmd=(
  go run ./cmd/chronogate replay
  --file "$FILE"
  --algorithm "$ALGORITHM"
  --rate "$RATE"
  --window "$WINDOW"
  --burst "$BURST"
  --speed "$SPEED"
)

if [[ -n "$KEYS" ]]; then
  cmd+=(--keys "$KEYS")
fi
if [[ -n "$ENDPOINTS" ]]; then
  cmd+=(--endpoints "$ENDPOINTS")
fi

"${cmd[@]}"
