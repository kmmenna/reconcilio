#!/bin/sh
set -eu

project="reconcilio-smoke-$$"
export ANALYST_WEB_PORT="${RECONCILIO_SMOKE_PORT:-3100}"
export WEB_ORIGIN="http://localhost:$ANALYST_WEB_PORT"
export RECONCILIO_SMOKE_ORIGIN="$WEB_ORIGIN"

cleanup() {
  docker compose -p "$project" down --volumes
}
trap cleanup EXIT

if ! docker compose -p "$project" up --build -d --wait; then
  docker compose -p "$project" logs --tail 30
  exit 1
fi
bootstrap=$(docker compose -p "$project" exec -T backend reconcilio bootstrap-admin)
RECONCILIO_SMOKE_PASSWORD=$(printf '%s\n' "$bootstrap" | sed -n 's/^Generated password: //p')
if [ -z "$RECONCILIO_SMOKE_PASSWORD" ]; then
  printf '%s\n' "The smoke instance did not return its initial administrator password." >&2
  exit 1
fi
export RECONCILIO_SMOKE_PASSWORD

npm --prefix apps/analyst-web run smoke
