#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INFRA_DIR="${ROOT_DIR}/infra"
AWS_REGION="${AWS_REGION:-$(terraform -chdir="${INFRA_DIR}" output -raw aws_region)}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
API_REPO="${API_REPO:-$(terraform -chdir="${INFRA_DIR}" output -raw api_ecr_repository_url)}"
WORKER_REPO="${WORKER_REPO:-$(terraform -chdir="${INFRA_DIR}" output -raw worker_ecr_repository_url)}"

aws ecr get-login-password --region "${AWS_REGION}" \
  | docker login --username AWS --password-stdin "${API_REPO%/*}"

# Build for linux/amd64 explicitly. Fargate runs amd64 unless the task def
# sets runtimePlatform to ARM64, so ARM hosts (Apple Silicon) must cross-build.
docker buildx inspect trx-amd64 >/dev/null 2>&1 || docker buildx create --name trx-amd64 --use >/dev/null

docker buildx build --builder trx-amd64 --platform linux/amd64 \
  -f "${ROOT_DIR}/api-svc/Dockerfile" \
  -t "${API_REPO}:${IMAGE_TAG}" \
  --push "${ROOT_DIR}"

docker buildx build --builder trx-amd64 --platform linux/amd64 \
  -f "${ROOT_DIR}/worker-svc/Dockerfile" \
  -t "${WORKER_REPO}:${IMAGE_TAG}" \
  --push "${ROOT_DIR}"

echo "Pushed API image to ${API_REPO}:${IMAGE_TAG}"
echo "Pushed worker image to ${WORKER_REPO}:${IMAGE_TAG}"
