#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="docker-compose.test.yml"
PROJECT_NAME="fleetcommerce-test"

cleanup() {
    echo ""
    echo "==> Tearing down test containers..."
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null
}
trap cleanup EXIT

echo "==> Starting test database and running tests..."
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null

docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up \
    --build \
    --abort-on-container-exit \
    --exit-code-from test-runner

echo "==> All tests passed."
