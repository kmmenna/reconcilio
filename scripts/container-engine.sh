select_container_engine() {
  CONTAINER_ENGINE=${CONTAINER_ENGINE:-docker}
  case "$CONTAINER_ENGINE" in
    docker|podman) export CONTAINER_ENGINE ;;
    *)
      printf 'Invalid CONTAINER_ENGINE "%s". Accepted values are docker and podman.\n' "$CONTAINER_ENGINE" >&2
      return 1
      ;;
  esac
}

podman_compose_endpoint() (
  unset DOCKER_CONTEXT
  # Use the socket Podman passes to Compose, including on macOS.
  PODMAN_COMPOSE_PROVIDER=/bin/sh PODMAN_COMPOSE_WARNING_LOGS=false \
    podman compose -c 'printf "%s\n" "$DOCKER_HOST"'
)
