#!/bin/sh
set -eu

. ./scripts/container-engine.sh
select_container_engine

if [ "$CONTAINER_ENGINE" = podman ]; then
  unset DOCKER_CONTEXT
  # Ask Podman for the endpoint it passes to Compose. This also handles
  # the forwarded socket on macOS and an explicit DOCKER_HOST override.
  DOCKER_HOST=$(PODMAN_COMPOSE_PROVIDER=/bin/sh PODMAN_COMPOSE_WARNING_LOGS=false \
    podman compose -c 'printf "%s\n" "$DOCKER_HOST"')
  podman --url "$DOCKER_HOST" info >/dev/null
else
  # Testcontainers reads DOCKER_HOST rather than the active Docker CLI context.
  endpoint=$(docker context inspect --format '{{.Endpoints.docker.Host}}')
  docker info >/dev/null
  DOCKER_HOST=$endpoint
fi

export DOCKER_HOST
export TESTCONTAINERS_PROVIDER="$CONTAINER_ENGINE"
cd apps/reconciliation
exec go test -count=1 ./...
