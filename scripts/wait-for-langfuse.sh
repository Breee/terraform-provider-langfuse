#!/usr/bin/env bash
# wait-for-langfuse.sh waits until the Langfuse web container in the docker-compose
# stack is healthy so acceptance tests can run.
# Usage: ./scripts/wait-for-langfuse.sh [HOST] [PORT] [TIMEOUT_SEC] [PATH]
# Defaults: HOST=localhost, PORT=3000, TIMEOUT_SEC=120, PATH=/api/public/health

set -euo pipefail

HOST=${1:-localhost}
PORT=${2:-3000}
TIMEOUT_SEC=${3:-120}
PATH_SUFFIX=${4:-/api/public/health?failIfDatabaseUnavailable=true}

echo "Waiting for Langfuse to become healthy at ${HOST}:${PORT}${PATH_SUFFIX} (timeout: ${TIMEOUT_SEC}s)..."

ATTEMPTS=$(( TIMEOUT_SEC / 2 ))
if [ "$ATTEMPTS" -lt 1 ]; then ATTEMPTS=1; fi

for i in $(seq 1 "$ATTEMPTS"); do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://${HOST}:${PORT}${PATH_SUFFIX}" || true)
  if [ "$CODE" = "200" ]; then
    echo "Langfuse is up! ($CODE)"
    exit 0
  fi
  sleep 2
done

echo "Langfuse did not become healthy within ${TIMEOUT_SEC}s" >&2
exit 1
