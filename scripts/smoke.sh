#!/bin/sh
set -eu

. ./scripts/container-engine.sh
select_container_engine

project=$(node -p '"reconcilio-smoke-" + require("node:crypto").randomBytes(8).toString("hex")')
export COMPOSE_PROJECT_NAME="$project" RECONCILIATION_API_PORT=0
export POSTGRES_DB=reconcilio_smoke POSTGRES_USER=reconcilio_smoke
POSTGRES_PASSWORD=$(node -p 'require("node:crypto").randomBytes(16).toString("hex")')
export POSTGRES_PASSWORD
export DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@postgres:5432/$POSTGRES_DB?sslmode=disable"
unset ANALYST_WEB_PORT BACKEND_API_ORIGIN WEB_ORIGIN JWT_SECRET COMPOSE_PROFILES
unset RECONCILIO_API_ENABLED RECONCILIO_WORKER_ENABLED RECONCILIO_TRACE_STDOUT RECONCILIO_METRIC_STDOUT
unset RECONCILIO_AI_PER_JOB_LIMIT RECONCILIO_AI_DAILY_LIMIT RECONCILIO_SMOKE_RANKING_URL
unset OPENAI_API_KEY OPENAI_MODEL OPENAI_RESPONSES_URL

compose() {
  if [ "${RECONCILIO_SMOKE_RANKING:-false}" = true ]; then
    set -- -f scripts/smoke-ranking.compose.yaml "$@"
  fi
  sh scripts/compose.sh --env-file /dev/null -p "$project" -f compose.yaml "$@"
}

remove_image() {
  if [ "$CONTAINER_ENGINE" = podman ]; then
    podman --url "$endpoint" image rm --ignore --no-prune "$1"
    return
  fi
  image_id=$(docker image ls --filter "reference=$1" --quiet) || return 1
  if [ -n "$image_id" ]; then
    docker image rm --no-prune "$1"
  fi
}

cleanup() {
  status=$?
  trap - EXIT INT HUP TERM
  if ! compose down --volumes && [ "$status" -eq 0 ]; then
    status=1
  fi
  while IFS= read -r image; do
    case "$image" in
      "$project"-*|"$project"_*)
        # Remove the project tag by name, even when it shares an image ID.
        case "$image" in
          *:*) ;;
          *) image="$image:latest" ;;
        esac
        if ! remove_image "$image" && [ "$status" -eq 0 ]; then
          status=1
        fi
        ;;
    esac
  done <<EOF
$images
EOF
  exit "$status"
}

images=$(compose config --images)
if [ "$CONTAINER_ENGINE" = podman ]; then
  endpoint=$(podman_compose_endpoint)
fi
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 129' HUP
trap 'exit 143' TERM

compose build

# Build before choosing a port so the time between releasing it and Compose
# binding it stays short. Next.js needs the browser origin at startup.
ANALYST_WEB_PORT=${RECONCILIO_SMOKE_PORT:-$(node -e '
  const server = require("node:net").createServer();
  server.listen(0, "127.0.0.1", () => {
    console.log(server.address().port);
    server.close();
  });
')}
export ANALYST_WEB_PORT
export WEB_ORIGIN="http://localhost:$ANALYST_WEB_PORT"
export RECONCILIO_SMOKE_ORIGIN="$WEB_ORIGIN"

if ! compose up -d --wait; then
  compose logs --tail 30
  exit 1
fi
bootstrap=$(compose exec -T backend reconcilio bootstrap-admin)
RECONCILIO_SMOKE_PASSWORD=$(printf '%s\n' "$bootstrap" | sed -n 's/^Generated password: //p')
if [ -z "$RECONCILIO_SMOKE_PASSWORD" ]; then
  printf '%s\n' "The smoke instance did not return its initial administrator password." >&2
  exit 1
fi
export RECONCILIO_SMOKE_PASSWORD

if [ "${RECONCILIO_SMOKE_RANKING:-false}" = true ]; then
  RECONCILIO_SMOKE_RANKING_URL="http://$(compose port ranking-provider 4010)"
  export RECONCILIO_SMOKE_RANKING_URL
  npm --prefix apps/analyst-web run smoke -- e2e/bilingual-ranking.spec.ts
else
  npm --prefix apps/analyst-web run smoke
fi
