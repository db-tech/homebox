#!/usr/bin/env bash
# Checks a running Homebox container from inside its network namespace.
# Piped into the builder image via stdin, so no bind mount is needed - which
# matters because Jenkins talks to the host Docker daemon and its own paths do
# not exist there.
set -euo pipefail

BASE="http://localhost:7745/api/v1"

up=0
for _ in $(seq 1 60); do
  sleep 2
  if curl -sf "$BASE/status" >/dev/null 2>&1; then
    up=1
    break
  fi
done

if [ "$up" -ne 1 ]; then
  echo "ERROR: container did not become healthy"
  exit 1
fi
echo "  container healthy on an empty database"

curl -sf -X POST "$BASE/users/register" \
  -H 'Content-Type: application/json' \
  -d '{"name":"ci","email":"ci@example.com","password":"ci-verify-password"}' >/dev/null

TOKEN=$(curl -sf -X POST "$BASE/users/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"ci@example.com","password":"ci-verify-password"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

if [ -z "$TOKEN" ]; then
  echo "ERROR: could not obtain a token"
  exit 1
fi

# The pantry endpoints exercise the columns and the table the new migration
# adds, so a migration that did not apply shows up here rather than in prod.
for ep in "pantry/expiring" "pantry/low-stock" "pantry/consumption/statistics"; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: $TOKEN" "$BASE/$ep")
  echo "  $ep -> $code"
  if [ "$code" != "200" ]; then
    exit 1
  fi
done

echo "  image verified on a fresh database"
