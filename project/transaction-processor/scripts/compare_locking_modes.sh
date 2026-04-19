#!/usr/bin/env bash
# Experiment 3: optimistic vs pessimistic locking on AWS.
#
# For each locking mode, this script:
#   1. Runs terraform apply to flip the worker's LOCKING_MODE env var.
#   2. Waits for the worker service to stabilize on the new task def.
#   3. Reseeds cloud accounts.
#   4. Runs Locust (HotAccountUser by default) against the ALB.
#   5. Captures worker counter snapshots from CloudWatch Logs.
#   6. Verifies balances on the cloud accounts table.
#
# All artifacts land under load-tests/experiments/<timestamp>/.
#
# Usage:
#   ./scripts/compare_locking_modes.sh
#
# Env:
#   LOCK_MODES        space-separated list (default "optimistic pessimistic")
#   USER_COUNTS       space-separated list (default "25 50")
#   SPAWN_RATE        Locust spawn rate (default 5)
#   RUN_TIME          Locust run duration (default 2m)
#   SCENARIO_CLASS    Locust user class (default HotAccountUser)
#   TF_CHDIR          terraform working directory (default infra)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

TF_CHDIR="${TF_CHDIR:-infra}"
LOCK_MODES="${LOCK_MODES:-optimistic pessimistic}"
USER_COUNTS="${USER_COUNTS:-25 50}"
SPAWN_RATE="${SPAWN_RATE:-5}"
RUN_TIME="${RUN_TIME:-2m}"
SCENARIO_CLASS="${SCENARIO_CLASS:-HotAccountUser}"

RESULT_STAMP="${RESULT_STAMP:-$(date +%Y%m%d-%H%M%S)}"
RESULT_DIR="${ROOT_DIR}/load-tests/experiments/${RESULT_STAMP}"
mkdir -p "${RESULT_DIR}"

log() { printf '[compare] %s\n' "$*"; }

"${ROOT_DIR}/scripts/preflight_labrole.sh"

for tool in terraform aws jq python3; do
  command -v "${tool}" >/dev/null || { log "missing required tool: ${tool}"; exit 1; }
done

AWS_REGION="$(terraform -chdir="${TF_CHDIR}" output -raw aws_region)"
ALB_URL="$(terraform -chdir="${TF_CHDIR}" output -raw base_url)"
ACCOUNTS_TABLE="$(terraform -chdir="${TF_CHDIR}" output -raw accounts_table_name)"
QUEUE_URL="$(terraform -chdir="${TF_CHDIR}" output -raw queue_url)"
CLUSTER_NAME="$(terraform -chdir="${TF_CHDIR}" output -raw ecs_cluster_name)"
API_SERVICE="$(terraform -chdir="${TF_CHDIR}" output -raw api_service_name)"
WORKER_SERVICE="$(terraform -chdir="${TF_CHDIR}" output -raw worker_service_name)"
WORKER_LOG_GROUP="$(terraform -chdir="${TF_CHDIR}" output -raw worker_log_group)"

log "ALB:              ${ALB_URL}"
log "accounts table:   ${ACCOUNTS_TABLE}"
log "worker log group: ${WORKER_LOG_GROUP}"

wait_stable() {
  local svc="$1"
  aws ecs wait services-stable \
    --region "${AWS_REGION}" \
    --cluster "${CLUSTER_NAME}" \
    --services "${svc}"
}

fetch_worker_metrics_since() {
  local since_ms="$1"
  aws logs filter-log-events \
    --region "${AWS_REGION}" \
    --log-group-name "${WORKER_LOG_GROUP}" \
    --start-time "${since_ms}" \
    --filter-pattern '"METRICS"' \
    --output json \
    | python3 -c '
import json, re, sys
events = json.load(sys.stdin).get("events", [])
latest = {}
for ev in events:
    m = re.search(r"METRICS (\{.*\})$", ev.get("message", ""))
    if not m:
        continue
    try:
        payload = json.loads(m.group(1))
    except json.JSONDecodeError:
        continue
    snap = payload.get("metrics") or {}
    if snap:
        latest = snap
print(json.dumps(latest))
'
}

for mode in ${LOCK_MODES}; do
  log "applying locking_mode=${mode}"
  terraform -chdir="${TF_CHDIR}" apply -auto-approve \
    -var "locking_mode=${mode}"

  wait_stable "${WORKER_SERVICE}"
  wait_stable "${API_SERVICE}"

  for users in ${USER_COUNTS}; do
    slug="${mode}-u${users}"
    base="experiments/${RESULT_STAMP}/${slug}"
    log "==== mode=${mode} users=${users} scenario=${SCENARIO_CLASS} ===="

    log "purging SQS queue to drop any leftover in-flight messages"
    aws sqs purge-queue --region "${AWS_REGION}" --queue-url "${QUEUE_URL}" >/dev/null || true
    sleep 65

    log "reseeding accounts"
    AWS_REGION="${AWS_REGION}" \
    DYNAMODB_ACCOUNTS_TABLE="${ACCOUNTS_TABLE}" \
    go run ./scripts/seed_accounts.go >/dev/null

    window_start_ms=$(python3 -c 'import time; print(int(time.time()*1000)-5000)')

    log "running locust"
    (
      cd "${ROOT_DIR}/load-tests" && \
      "${ROOT_DIR}/scripts/run_locust.sh" -f locustfile.py "${SCENARIO_CLASS}" \
        --host "${ALB_URL}" \
        --users "${users}" \
        --spawn-rate "${SPAWN_RATE}" \
        --run-time "${RUN_TIME}" \
        --headless \
        --csv="${base}" \
        --html="${base}.html"
    )

    log "waiting for SQS queue to drain before verify"
    for _ in $(seq 1 120); do
      attrs="$(aws sqs get-queue-attributes --region "${AWS_REGION}" --queue-url "${QUEUE_URL}" \
        --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible \
        --output json)"
      visible="$(echo "${attrs}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["Attributes"]["ApproximateNumberOfMessages"])')"
      inflight="$(echo "${attrs}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["Attributes"]["ApproximateNumberOfMessagesNotVisible"])')"
      if [ "${visible}" = "0" ] && [ "${inflight}" = "0" ]; then
        break
      fi
      sleep 5
    done
    sleep 15

    log "capturing worker metrics snapshot"
    fetch_worker_metrics_since "${window_start_ms}" > "${RESULT_DIR}/${slug}.metrics.json"

    cat > "${RESULT_DIR}/${slug}.meta.json" <<EOF
{"slug":"${slug}","mode":"${mode}","users":${users},"scenario":"${SCENARIO_CLASS}","spawn_rate":"${SPAWN_RATE}","run_time":"${RUN_TIME}"}
EOF

    log "verifying balances"
    if ! AWS_REGION="${AWS_REGION}" \
      DYNAMODB_ACCOUNTS_TABLE="${ACCOUNTS_TABLE}" \
      "${VERIFY_PYTHON:-python3}" load-tests/verify_balances.py \
        > "${RESULT_DIR}/${slug}.verify.txt" 2>&1; then
      log "WARNING: verify reported a mismatch for ${slug}; continuing sweep"
    fi
  done
done

log "generating summary + charts"
python3 "${ROOT_DIR}/scripts/summarize_locust_results.py" "${RESULT_DIR}" \
  | tee "${RESULT_DIR}/summary.md"
python3 "${ROOT_DIR}/scripts/render_charts.py" "${RESULT_DIR}" || true

log "artifacts under ${RESULT_DIR}"
