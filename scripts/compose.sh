#!/bin/sh
set -eu

. ./scripts/container-engine.sh
select_container_engine
export COMPOSE_DISABLE_ENV_FILE=1

if [ "$CONTAINER_ENGINE" = docker ]; then
  exec docker compose "$@"
fi

unset DOCKER_CONTEXT
if [ -n "${PODMAN_COMPOSE_PROVIDER:-}" ]; then
  case $("$PODMAN_COMPOSE_PROVIDER" version 2>/dev/null) in
    'Docker Compose version '*) ;;
    *)
      printf '%s\n' 'PODMAN_COMPOSE_PROVIDER must point to a Docker Compose executable.' >&2
      exit 1
      ;;
  esac
elif ! PODMAN_COMPOSE_PROVIDER=$(command -v docker-compose); then
  # Docker Compose may be installed only as a Docker CLI plugin.
  PODMAN_COMPOSE_PROVIDER=$(docker info --format '{{range .ClientInfo.Plugins}}{{if eq .Name "compose"}}{{.Path}}{{end}}{{end}}' 2>/dev/null || true)
fi
if [ -z "$PODMAN_COMPOSE_PROVIDER" ]; then
  printf '%s\n' 'Podman requires Docker Compose. Install the docker-compose executable or the Docker Compose CLI plugin.' >&2
  exit 1
fi

export PODMAN_COMPOSE_PROVIDER
exec podman compose "$@"
