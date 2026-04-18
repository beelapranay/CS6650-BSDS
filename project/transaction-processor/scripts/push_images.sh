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

docker build -t transaction-processor-api:${IMAGE_TAG} -f "${ROOT_DIR}/api-svc/Dockerfile" "${ROOT_DIR}"
docker build -t transaction-processor-worker:${IMAGE_TAG} -f "${ROOT_DIR}/worker-svc/Dockerfile" "${ROOT_DIR}"

docker tag transaction-processor-api:${IMAGE_TAG} "${API_REPO}:${IMAGE_TAG}"
docker tag transaction-processor-worker:${IMAGE_TAG} "${WORKER_REPO}:${IMAGE_TAG}"

docker push "${API_REPO}:${IMAGE_TAG}"
docker push "${WORKER_REPO}:${IMAGE_TAG}"

echo "Pushed API image to ${API_REPO}:${IMAGE_TAG}"
echo "Pushed worker image to ${WORKER_REPO}:${IMAGE_TAG}"
