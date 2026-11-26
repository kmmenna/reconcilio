#!/bin/sh
set -eu

project="reconcilio-smoke-$$"
export RECONCILIATION_API_PORT=0
export ANALYST_WEB_PORT="${RECONCILIO_SMOKE_PORT:-3100}"
export WEB_ORIGIN="http://localhost:$ANALYST_WEB_PORT"
export RECONCILIO_SMOKE_ORIGIN="$WEB_ORIGIN"
export COMPOSE_DISABLE_ENV_FILE=1
unset DATABASE_URL POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD BACKEND_API_ORIGIN
unset RECONCILIO_API_ENABLED RECONCILIO_WORKER_ENABLED
unset OPENAI_API_KEY OPENAI_MODEL OPENAI_RESPONSES_URL

compose() {
  if [ "${RECONCILIO_SMOKE_RANKING:-false}" = true ]; then
    docker compose -p "$project" -f compose.yaml -f scripts/smoke-ranking.compose.yaml "$@"
  else
    docker compose -p "$project" "$@"
  fi
}

cleanup() {
  compose down --volumes
}
trap cleanup EXIT

if ! compose up --build -d --wait; then
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
