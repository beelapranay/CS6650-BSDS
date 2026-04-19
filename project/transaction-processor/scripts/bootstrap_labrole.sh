#!/usr/bin/env bash
# One-shot: create LabRole with ECS trust and attach the managed policies
# every task needs. Idempotent — safe to re-run.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TRUST_FILE="${ROOT_DIR}/infra/labrole-trust.json"
ROLE_NAME="LabRole"

POLICIES=(
  "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
  "arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"
  "arn:aws:iam::aws:policy/AmazonSQSFullAccess"
  "arn:aws:iam::aws:policy/CloudWatchLogsFullAccess"
)

if aws iam get-role --role-name "${ROLE_NAME}" >/dev/null 2>&1; then
  echo "[bootstrap] role ${ROLE_NAME} already exists, skipping create"
else
  echo "[bootstrap] creating role ${ROLE_NAME}"
  aws iam create-role \
    --role-name "${ROLE_NAME}" \
    --assume-role-policy-document "file://${TRUST_FILE}" >/dev/null
fi

for arn in "${POLICIES[@]}"; do
  echo "[bootstrap] attaching ${arn}"
  aws iam attach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${arn}"
done

echo "[bootstrap] done"
aws iam get-role --role-name "${ROLE_NAME}" --query 'Role.Arn' --output text
