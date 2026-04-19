#!/usr/bin/env bash
# Experiment 4: horizontal scaling on AWS.
#
# For each (api_replicas, worker_replicas) pair, this script:
#   1. Sets desired counts via terraform apply.
#   2. Waits for ECS services to stabilize.
#   3. Reseeds accounts so every run starts from the known initial total.
#   4. Runs Locust against the ALB for a fixed workload.
#   5. Captures worker metrics from CloudWatch Logs (before/after diff).
#   6. Runs verify_balances.py against the cloud accounts table.
#
# All artifacts land under load-tests/experiments/<timestamp>/scale-<api>-<worker>/ ...
#
# Usage:
#   ./scripts/scale_experiment.sh
#
# Env:
#   REPLICA_PAIRS     space-separated list of "<api>:<worker>" (default: "1:1 2:2 4:4")
#   LOCUST_USERS      Locust user count (default 100)
#   LOCUST_SPAWN_RATE spawn rate (default 10)
#   LOCUST_RUN_TIME   run duration (default 3m)
#   SCENARIO_CLASS    locust user class (default TransferUser)
#   LOCKING_MODE      optimistic|pessimistic (default optimistic)
#   TF_CHDIR          terraform working directory (default infra)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

REPLICA_PAIRS="${REPLICA_PAIRS:-1:1 2:2 4:4}"
LOCUST_USERS="${LOCUST_USERS:-100}"
LOCUST_SPAWN_RATE="${LOCUST_SPAWN_RATE:-10}"
LOCUST_RUN_TIME="${LOCUST_RUN_TIME:-3m}"
SCENARIO_CLASS="${SCENARIO_CLASS:-TransferUser}"
LOCKING_MODE="${LOCKING_MODE:-optimistic}"
TF_CHDIR="${TF_CHDIR:-infra}"

RESULT_STAMP="${RESULT_STAMP:-$(date +%Y%m%d-%H%M%S)}"
RESULT_DIR="${ROOT_DIR}/load-tests/experiments/${RESULT_STAMP}"
mkdir -p "${RESULT_DIR}"

log() { printf '[scale] %s\n' "$*"; }

need() {
  command -v "$1" >/dev/null || { log "missing required tool: $1"; exit 1; }
}
need terraform
need aws
need jq
need python3

AWS_REGION="$(terraform -chdir="${TF_CHDIR}" output -raw aws_region)"
ALB_URL="$(terraform -chdir="${TF_CHDIR}" output -raw base_url)"
ACCOUNTS_TABLE="$(terraform -chdir="${TF_CHDIR}" output -raw accounts_table_name)"
QUEUE_URL="$(terraform -chdir="${TF_CHDIR}" output -raw queue_url)"
CLUSTER_NAME="$(terraform -chdir="${TF_CHDIR}" output -raw ecs_cluster_name)"
API_SERVICE="$(terraform -chdir="${TF_CHDIR}" output -raw api_service_name)"
WORKER_SERVICE="$(terraform -chdir="${TF_CHDIR}" output -raw worker_service_name)"
WORKER_LOG_GROUP="$(terraform -chdir="${TF_CHDIR}" output -raw worker_log_group)"

log "ALB:                ${ALB_URL}"
log "accounts table:     ${ACCOUNTS_TABLE}"
log "cluster:            ${CLUSTER_NAME}"
log "worker log group:   ${WORKER_LOG_GROUP}"

wait_service_stable() {
  local svc="$1"
  log "waiting for service ${svc} to stabilize"
  aws ecs wait services-stable \
    --region "${AWS_REGION}" \
    --cluster "${CLUSTER_NAME}" \
    --services "${svc}"
}

fetch_worker_metrics_since() {
  # Grep the METRICS lines from CloudWatch for the window [since_ms, now] and
  # emit the latest cumulative snapshot as JSON.
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

run_pair() {
  local api="$1" worker="$2"
  local slug="scale-a${api}-w${worker}-${LOCKING_MODE}"
  local base="experiments/${RESULT_STAMP}/${slug}"

  log "==== pair api=${api} worker=${worker} mode=${LOCKING_MODE} ===="

  log "terraform apply desired counts"
  terraform -chdir="${TF_CHDIR}" apply -auto-approve \
    -var "api_desired_count=${api}" \
    -var "worker_desired_count=${worker}" \
    -var "locking_mode=${LOCKING_MODE}"

  wait_service_stable "${API_SERVICE}"
  wait_service_stable "${WORKER_SERVICE}"

  log "purging SQS queue to drop any leftover in-flight messages"
  aws sqs purge-queue --region "${AWS_REGION}" --queue-url "${QUEUE_URL}" >/dev/null || true
  sleep 65

  log "reseeding accounts in ${ACCOUNTS_TABLE}"
  AWS_REGION="${AWS_REGION}" \
  DYNAMODB_ACCOUNTS_TABLE="${ACCOUNTS_TABLE}" \
  go run ./scripts/seed_accounts.go >/dev/null

  local window_start_ms
  window_start_ms=$(python3 -c 'import time; print(int(time.time()*1000)-5000)')

  log "running locust: users=${LOCUST_USERS} spawn=${LOCUST_SPAWN_RATE} duration=${LOCUST_RUN_TIME}"
  (
    cd "${ROOT_DIR}/load-tests" && \
    "${ROOT_DIR}/scripts/run_locust.sh" -f locustfile.py "${SCENARIO_CLASS}" \
      --host "${ALB_URL}" \
      --users "${LOCUST_USERS}" \
      --spawn-rate "${LOCUST_SPAWN_RATE}" \
      --run-time "${LOCUST_RUN_TIME}" \
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
  sleep 15  # small buffer so last TransactWriteItems settles before strongly-consistent scan

  log "capturing worker metrics snapshot"
  fetch_worker_metrics_since "${window_start_ms}" > "${RESULT_DIR}/${slug}.metrics.json"

  cat > "${RESULT_DIR}/${slug}.meta.json" <<EOF
{"slug":"${slug}","mode":"${LOCKING_MODE}","users":${LOCUST_USERS},"scenario":"${SCENARIO_CLASS}","api_replicas":${api},"worker_replicas":${worker},"run_time":"${LOCUST_RUN_TIME}"}
EOF

  log "verifying balances on cloud table ${ACCOUNTS_TABLE}"
  if ! AWS_REGION="${AWS_REGION}" \
    DYNAMODB_ACCOUNTS_TABLE="${ACCOUNTS_TABLE}" \
    "${VERIFY_PYTHON:-python3}" load-tests/verify_balances.py \
      > "${RESULT_DIR}/${slug}.verify.txt" 2>&1; then
    log "WARNING: verify reported a mismatch for ${slug}; continuing sweep (see ${slug}.verify.txt)"
  fi
}

for pair in ${REPLICA_PAIRS}; do
  api="${pair%%:*}"
  worker="${pair##*:}"
  run_pair "${api}" "${worker}"
done

log "generating summary + charts"
python3 "${ROOT_DIR}/scripts/summarize_locust_results.py" "${RESULT_DIR}" \
  | tee "${RESULT_DIR}/summary.md"
python3 "${ROOT_DIR}/scripts/render_charts.py" "${RESULT_DIR}" || true

log "all artifacts under ${RESULT_DIR}"
