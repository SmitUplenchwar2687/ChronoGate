#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ROUTE="${ROUTE:-/api/profile}"
METHOD="${METHOD:-GET}"
REQUESTS="${REQUESTS:-200}"
CONCURRENCY="${CONCURRENCY:-20}"
API_KEY="${API_KEY:-demo-key}"
PAYLOAD="${PAYLOAD:-{"item":"demo"}}"

status_file="$(mktemp)"
start_ts="$(date +%s)"

cleanup() {
  rm -f "$status_file"
}
trap cleanup EXIT

run_request() {
  local status_code

  if [[ "$METHOD" == "POST" ]]; then
    status_code="$(curl -s -o /dev/null -w "%{http_code}" \
      -X POST \
      -H "X-API-Key: ${API_KEY}" \
      -H "Content-Type: application/json" \
      --data "$PAYLOAD" \
      "${BASE_URL}${ROUTE}" || true)"
  else
    status_code="$(curl -s -o /dev/null -w "%{http_code}" \
      -X "$METHOD" \
      -H "X-API-Key: ${API_KEY}" \
      "${BASE_URL}${ROUTE}" || true)"
  fi

  if [[ -z "$status_code" ]]; then
    status_code="000"
  fi

  printf '%s\n' "$status_code" >> "$status_file"
}

for ((i = 1; i <= REQUESTS; i++)); do
  run_request &

  while (( $(jobs -pr | wc -l | tr -d ' ') >= CONCURRENCY )); do
    sleep 0.02
  done
done

wait

end_ts="$(date +%s)"
elapsed="$((end_ts - start_ts))"
if (( elapsed <= 0 )); then
  elapsed=1
fi

ok_count="$(awk '/^2/{c++} END{print c+0}' "$status_file")"
limited_count="$(awk '/^429$/{c++} END{print c+0}' "$status_file")"
other_count="$((REQUESTS - ok_count - limited_count))"
rps="$((REQUESTS / elapsed))"

cat <<SUMMARY
Load test complete
BASE_URL: ${BASE_URL}
ROUTE: ${ROUTE}
METHOD: ${METHOD}
REQUESTS: ${REQUESTS}
CONCURRENCY: ${CONCURRENCY}
API_KEY: ${API_KEY}
2xx: ${ok_count}
429: ${limited_count}
other: ${other_count}
elapsed_seconds: ${elapsed}
req_per_sec: ${rps}
SUMMARY
