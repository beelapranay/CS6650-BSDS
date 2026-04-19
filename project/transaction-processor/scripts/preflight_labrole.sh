#!/usr/bin/env bash
# Preflight for the cloud stack. Two independent checks:
#
#  1. The shell has valid AWS credentials (get-caller-identity succeeds).
#  2. LabRole exists in the account, is trusted by ecs-tasks.amazonaws.com,
#     and is the identity this Terraform stack will wire into every ECS
#     task. Resources created by Terraform are tagged under the caller's
#     principal, but every workload (api-svc + worker-svc) runs as LabRole.
#
# Exits non-zero if either check fails.

set -euo pipefail

LAB_ROLE_NAME="${LAB_ROLE_NAME:-LabRole}"

if ! command -v aws >/dev/null; then
  echo "[preflight] aws CLI not found on PATH" >&2
  exit 1
fi
if ! command -v jq >/dev/null; then
  echo "[preflight] jq not found on PATH" >&2
  exit 1
fi

ID_JSON="$(aws sts get-caller-identity --output json)"
ARN="$(echo "${ID_JSON}" | jq -r .Arn)"
ACCOUNT="$(echo "${ID_JSON}" | jq -r .Account)"

echo "[preflight] caller arn:        ${ARN}"
echo "[preflight] account id:        ${ACCOUNT}"
echo "[preflight] required ECS role: ${LAB_ROLE_NAME}"

if ! ROLE_JSON="$(aws iam get-role --role-name "${LAB_ROLE_NAME}" --output json 2>/dev/null)"; then
  echo "[preflight] FAIL: role ${LAB_ROLE_NAME} does not exist in account ${ACCOUNT}" >&2
  echo "[preflight] create it (see README) before running terraform" >&2
  exit 1
fi

TRUST_JSON="$(echo "${ROLE_JSON}" | jq -r '.Role.AssumeRolePolicyDocument | tostring')"
if ! echo "${TRUST_JSON}" | grep -q 'ecs-tasks.amazonaws.com'; then
  echo "[preflight] FAIL: ${LAB_ROLE_NAME} trust policy does not allow ecs-tasks.amazonaws.com" >&2
  echo "[preflight] ECS tasks will not be able to assume it" >&2
  exit 1
fi

echo "[preflight] OK — caller authenticated and ${LAB_ROLE_NAME} is ECS-assumable"
