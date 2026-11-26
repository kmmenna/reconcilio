#!/bin/sh
set -eu

. ./scripts/container-engine.sh
select_container_engine
export COMPOSE_DISABLE_ENV_FILE=1

if [ "$CONTAINER_ENGINE" = docker ]; then
  exec docker compose "$@"
fi

unset DOCKER_CONTEXT

is_docker_compose() {
  [ -n "$1" ] || return 1
  version=$("$1" version 2>/dev/null) || return 1
  case "$version" in
    'Docker Compose version '*) return 0 ;;
    *) return 1 ;;
  esac
}

if [ -n "${PODMAN_COMPOSE_PROVIDER:-}" ]; then
  if ! is_docker_compose "$PODMAN_COMPOSE_PROVIDER"; then
    printf '%s\n' 'PODMAN_COMPOSE_PROVIDER must point to a Docker Compose executable.' >&2
    exit 1
  fi
elif provider=$(command -v docker-compose) && is_docker_compose "$provider"; then
  PODMAN_COMPOSE_PROVIDER=$provider
else
  # Docker Compose may be installed only as a Docker CLI plugin.
  PODMAN_COMPOSE_PROVIDER=$(docker info --format '{{range .ClientInfo.Plugins}}{{if eq .Name "compose"}}{{.Path}}{{end}}{{end}}' 2>/dev/null || true)
  if ! is_docker_compose "$PODMAN_COMPOSE_PROVIDER"; then
    printf '%s\n' 'Podman requires Docker Compose. Install the docker-compose executable or the Docker Compose CLI plugin.' >&2
    exit 1
  fi
fi

export PODMAN_COMPOSE_PROVIDER
exec podman compose "$@"
