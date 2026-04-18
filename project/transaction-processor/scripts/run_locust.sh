#!/usr/bin/env bash
set -euo pipefail

if [[ -x /tmp/locust-venv/bin/locust ]]; then
  exec /tmp/locust-venv/bin/locust "$@"
fi

if command -v locust >/dev/null 2>&1; then
  exec "$(command -v locust)" "$@"
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MOUNT_DIR="${ROOT_DIR}/load-tests"
ARGS=()

for arg in "$@"; do
  arg="${arg//http:\/\/localhost/http:\/\/host.docker.internal}"
  arg="${arg//http:\/\/127.0.0.1/http:\/\/host.docker.internal}"
  ARGS+=("${arg}")
done

exec docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  -v "${MOUNT_DIR}:/mnt/load-tests" \
  -w /mnt/load-tests \
  locustio/locust:2.32.1 \
  "${ARGS[@]}"
