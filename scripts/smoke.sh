#!/bin/sh
set -eu

BASE_URL=${1:-http://localhost:8080}
FRONTEND_URL=${FRONTEND_URL:-http://localhost:5173}
SMOKE_EMAIL=${SMOKE_EMAIL:-smoke@local.invalid}
SMOKE_PASSWORD=${SMOKE_PASSWORD:-Smoke-check-password-42}

cookie_jar=$(mktemp)
auth_body=$(mktemp)
cleanup() {
  rm -f "${cookie_jar}" "${auth_body}"
}
trap cleanup EXIT INT TERM

curl -fsS "${FRONTEND_URL}" >/dev/null

health=$(curl -fsS "${BASE_URL}/healthz")
printf '%s\n' "${health}" | grep -q '"status":"ok"'

seasons=$(curl -fsS "${BASE_URL}/api/v1/seasons")
printf '%s\n' "${seasons}" | grep -q '"items"'
season_id=$(printf '%s\n' "${seasons}" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
test -n "${season_id}"

players=$(curl -fsS "${BASE_URL}/api/v1/players?seasonId=${season_id}&sort=form&direction=desc&pageSize=3")
printf '%s\n' "${players}" | grep -q '"items"'
printf '%s\n' "${players}" | grep -q "\"seasonId\":${season_id}"

snapshots=$(curl -fsS "${BASE_URL}/api/v1/data/snapshots?seasonId=${season_id}")
printf '%s\n' "${snapshots}" | grep -q '"data"'
printf '%s\n' "${snapshots}" | grep -q '"meta"'

auth_status=$(curl -sS -o "${auth_body}" -w '%{http_code}' -c "${cookie_jar}" \
  -H 'Content-Type: application/json' \
  -H "Origin: ${FRONTEND_URL}" \
  --data "{\"email\":\"${SMOKE_EMAIL}\",\"password\":\"${SMOKE_PASSWORD}\",\"displayName\":\"Smoke Test\"}" \
  "${BASE_URL}/api/v1/auth/register")

if [ "${auth_status}" = "409" ]; then
  auth_status=$(curl -sS -o "${auth_body}" -w '%{http_code}' -c "${cookie_jar}" \
    -H 'Content-Type: application/json' \
    -H "Origin: ${FRONTEND_URL}" \
    --data "{\"email\":\"${SMOKE_EMAIL}\",\"password\":\"${SMOKE_PASSWORD}\"}" \
    "${BASE_URL}/api/v1/auth/login")
fi

if [ "${auth_status}" != "200" ] && [ "${auth_status}" != "201" ]; then
  echo "Smoke authentication failed with HTTP ${auth_status}. Set SMOKE_EMAIL and SMOKE_PASSWORD to an existing local account when registration is disabled." >&2
  exit 1
fi

squad=$(curl -fsS -b "${cookie_jar}" "${BASE_URL}/api/v1/squad?seasonId=${season_id}")
printf '%s\n' "${squad}" | grep -q "\"seasonId\":${season_id}"
echo "Fantasy Helper smoke test passed against ${BASE_URL}"
