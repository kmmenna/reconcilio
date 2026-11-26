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
