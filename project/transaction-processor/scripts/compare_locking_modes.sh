#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESULT_STAMP="${RESULT_STAMP:-$(date +%Y%m%d-%H%M%S)}"
RESULT_DIR="${ROOT_DIR}/load-tests/experiments/${RESULT_STAMP}"
SCENARIO_CLASS="${SCENARIO_CLASS:-HotAccountUser}"
LOCK_MODES="${LOCK_MODES:-optimistic pessimistic}"
USER_COUNTS="${USER_COUNTS:-25 50}"
SPAWN_RATE="${SPAWN_RATE:-5}"
RUN_TIME="${RUN_TIME:-45s}"

mkdir -p "${RESULT_DIR}"

cleanup() {
  (cd "${ROOT_DIR}" && make down >/dev/null 2>&1) || true
}
trap cleanup EXIT

for mode in ${LOCK_MODES}; do
  for users in ${USER_COUNTS}; do
    slug="${mode}-u${users}"
    base="experiments/${RESULT_STAMP}/${slug}"

    echo "==> mode=${mode} users=${users} scenario=${SCENARIO_CLASS}"
    (cd "${ROOT_DIR}" && make down >/dev/null 2>&1) || true
    (
      cd "${ROOT_DIR}" && \
      LOCKING_MODE="${mode}" make init
    )
    (
      cd "${ROOT_DIR}/load-tests" && \
      "${ROOT_DIR}/scripts/run_locust.sh" -f locustfile.py "${SCENARIO_CLASS}" \
        --host=http://localhost:8080 \
        --users "${users}" \
        --spawn-rate "${SPAWN_RATE}" \
        --run-time "${RUN_TIME}" \
        --headless \
        --csv="${base}" \
        --html="${base}.html"
    )
    (
      cd "${ROOT_DIR}" && \
      DYNAMODB_ENDPOINT_URL=http://localhost:8000 \
      AWS_REGION=us-east-1 \
      AWS_ACCESS_KEY_ID=test \
      AWS_SECRET_ACCESS_KEY=test \
      DYNAMODB_ACCOUNTS_TABLE=accounts \
      "${VERIFY_PYTHON:-python3}" load-tests/verify_balances.py >/dev/null
    )

    cat > "${RESULT_DIR}/${slug}.meta.json" <<EOF
{"slug":"${slug}","mode":"${mode}","users":${users},"scenario":"${SCENARIO_CLASS}","spawn_rate":"${SPAWN_RATE}","run_time":"${RUN_TIME}"}
EOF
  done
done

"${VERIFY_PYTHON:-python3}" "${ROOT_DIR}/scripts/summarize_locust_results.py" "${RESULT_DIR}" \
  | tee "${RESULT_DIR}/summary.md"

echo "comparison artifacts written to ${RESULT_DIR}"
