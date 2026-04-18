#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <base-url>" >&2
  exit 1
fi

BASE_URL="${1%/}"
FROM_ACCOUNT="${FROM_ACCOUNT:-account-0}"
TO_ACCOUNT="${TO_ACCOUNT:-account-1}"
AMOUNT="${AMOUNT:-10.0}"
TXN_ID="${TXN_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}"

curl -fsS "${BASE_URL}/health" > /dev/null
echo "Health check passed for ${BASE_URL}"

POST_STATUS="$(
  curl -sS -o /tmp/transaction-processor-smoke-post.json -w "%{http_code}" \
    -X POST "${BASE_URL}/transfer" \
    -H "Content-Type: application/json" \
    -d "{\"transaction_id\":\"${TXN_ID}\",\"from_account\":\"${FROM_ACCOUNT}\",\"to_account\":\"${TO_ACCOUNT}\",\"amount\":${AMOUNT}}"
)"

if [[ "${POST_STATUS}" != "200" && "${POST_STATUS}" != "202" ]]; then
  echo "transfer submission failed with status ${POST_STATUS}" >&2
  cat /tmp/transaction-processor-smoke-post.json >&2
  exit 1
fi

echo "Submitted transfer ${TXN_ID}; polling for completion"

for _ in $(seq 1 30); do
  BODY="$(curl -sS "${BASE_URL}/transfer/${TXN_ID}")"
  STATUS="$(python3 -c 'import json,sys; print(json.loads(sys.stdin.read()).get("status",""))' <<< "${BODY}")"
  if [[ "${STATUS}" == "COMPLETED" || "${STATUS}" == "FAILED" ]]; then
    echo "${BODY}"
    exit 0
  fi
  sleep 2
done

echo "transfer ${TXN_ID} did not reach a terminal state in time" >&2
exit 1
