#!/usr/bin/env bash
# Experiment 2: worker crash recovery on AWS.
#
# 1. Ensures the worker task definition has PRE_COMMIT_DELAY_MS set.
#    If the current running task definition does not match the requested
#    delay, this script runs terraform apply to roll a new revision and
#    waits for the service to stabilize.
# 2. Submits a single transfer against the ALB.
# 3. Stops one running worker ECS task during the pre-commit delay window.
# 4. Waits for ECS to replace the task and for SQS to redeliver the message.
# 5. Polls the API for a terminal status on the transaction.
# 6. Runs verify_balances.py against the cloud accounts table.
#
# Usage:
#   ./scripts/crash_recovery_test.sh [from_account] [to_account] [amount]
#
# Env:
#   PRE_COMMIT_DELAY_MS       worker-side delay (default 8000 ms)
#   KILL_AT_MS                delay before stopping the task (default 4000 ms)
#   REDELIVERY_WAIT_SECONDS   max wait for terminal status (default 300 s)
#   TF_CHDIR                  terraform working directory (default infra)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

TF_CHDIR="${TF_CHDIR:-infra}"
FROM_ACCT="${1:-account-0}"
TO_ACCT="${2:-account-1}"
AMOUNT="${3:-25.00}"

PRE_COMMIT_DELAY_MS="${PRE_COMMIT_DELAY_MS:-8000}"
KILL_AT_MS="${KILL_AT_MS:-4000}"
REDELIVERY_WAIT_SECONDS="${REDELIVERY_WAIT_SECONDS:-300}"

log() { printf '[crash-test] %s\n' "$*"; }

"${ROOT_DIR}/scripts/preflight_labrole.sh"

for tool in terraform aws jq curl python3; do
  command -v "${tool}" >/dev/null || { log "missing required tool: ${tool}"; exit 1; }
done

AWS_REGION="$(terraform -chdir="${TF_CHDIR}" output -raw aws_region)"
ALB_URL="$(terraform -chdir="${TF_CHDIR}" output -raw base_url)"
ACCOUNTS_TABLE="$(terraform -chdir="${TF_CHDIR}" output -raw accounts_table_name)"
CLUSTER_NAME="$(terraform -chdir="${TF_CHDIR}" output -raw ecs_cluster_name)"
WORKER_SERVICE="$(terraform -chdir="${TF_CHDIR}" output -raw worker_service_name)"
WORKER_LOG_GROUP="$(terraform -chdir="${TF_CHDIR}" output -raw worker_log_group)"

log "applying pre_commit_delay_ms=${PRE_COMMIT_DELAY_MS} to the worker task def"
terraform -chdir="${TF_CHDIR}" apply -auto-approve \
  -var "pre_commit_delay_ms=${PRE_COMMIT_DELAY_MS}"

log "waiting for worker service to stabilize"
aws ecs wait services-stable \
  --region "${AWS_REGION}" \
  --cluster "${CLUSTER_NAME}" \
  --services "${WORKER_SERVICE}"

TASK_ARNS_JSON="$(aws ecs list-tasks \
  --region "${AWS_REGION}" \
  --cluster "${CLUSTER_NAME}" \
  --service-name "${WORKER_SERVICE}" \
  --desired-status RUNNING \
  --output json)"
TASK_ARN="$(echo "${TASK_ARNS_JSON}" | jq -r '.taskArns[0]')"
if [ -z "${TASK_ARN}" ] || [ "${TASK_ARN}" = "null" ]; then
  log "no running worker task found — is the service scaled to at least 1?"
  exit 1
fi
log "target worker task: ${TASK_ARN}"

TXN_ID="crash-$(date +%s)-$$"
log "submitting transfer ${TXN_ID} (${FROM_ACCT} -> ${TO_ACCT}, amount=${AMOUNT}) against ${ALB_URL}"
curl -sf -X POST "${ALB_URL}/transfer" \
  -H "Content-Type: application/json" \
  -d "{\"transaction_id\":\"${TXN_ID}\",\"from_account\":\"${FROM_ACCT}\",\"to_account\":\"${TO_ACCT}\",\"amount\":${AMOUNT}}" \
  >/dev/null

KILL_SLEEP_S=$(awk "BEGIN{printf \"%.3f\", ${KILL_AT_MS}/1000}")
log "sleeping ${KILL_SLEEP_S}s then stopping worker task"
sleep "${KILL_SLEEP_S}"

aws ecs stop-task \
  --region "${AWS_REGION}" \
  --cluster "${CLUSTER_NAME}" \
  --task "${TASK_ARN}" \
  --reason "crash-recovery-test" >/dev/null
log "worker task stopped; waiting for service replacement + SQS redelivery"

deadline=$(( $(date +%s) + REDELIVERY_WAIT_SECONDS ))
status=""
while [ "$(date +%s)" -lt "${deadline}" ]; do
  body="$(curl -sf "${ALB_URL}/transfer/${TXN_ID}" || true)"
  if [ -n "${body}" ]; then
    status="$(echo "${body}" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("status",""))' 2>/dev/null || true)"
    if [ "${status}" = "COMPLETED" ] || [ "${status}" = "FAILED" ]; then
      break
    fi
  fi
  sleep 5
done

log "final transaction status: ${status:-<unknown>}"
if [ "${status}" != "COMPLETED" ] && [ "${status}" != "FAILED" ]; then
  log "transaction never reached a terminal state within ${REDELIVERY_WAIT_SECONDS}s"
  log "inspect worker logs: ${WORKER_LOG_GROUP}"
  exit 1
fi

log "verifying balances on ${ACCOUNTS_TABLE}"
if ! AWS_REGION="${AWS_REGION}" \
  DYNAMODB_ACCOUNTS_TABLE="${ACCOUNTS_TABLE}" \
  "${VERIFY_PYTHON:-python3}" load-tests/verify_balances.py; then
  log "verify_balances.py reported a mismatch"
  exit 1
fi

log "crash recovery test PASSED (tx ${TXN_ID} reached ${status}, balances sum cleanly)"
