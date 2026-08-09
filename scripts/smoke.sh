#!/bin/sh
set -eu

BASE_URL=${1:-http://localhost:8080}
FRONTEND_URL=${FRONTEND_URL:-http://localhost:5173}

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

squad=$(curl -fsS "${BASE_URL}/api/v1/squad?seasonId=${season_id}")
printf '%s\n' "${squad}" | grep -q "\"seasonId\":${season_id}"
echo "Fantasy Helper smoke test passed against ${BASE_URL}"
