#!/bin/sh
set -eu

BASE_URL=${1:-http://localhost:8080}

health=$(curl -fsS "${BASE_URL}/healthz")
printf '%s\n' "${health}" | grep -q '"status":"ok"'

players=$(curl -fsS "${BASE_URL}/api/v1/players?sort=form&direction=desc&pageSize=3")
printf '%s\n' "${players}" | grep -q '"items"'

curl -fsS -X PUT "${BASE_URL}/api/v1/squad" \
  -H 'Content-Type: application/json' \
  --data '{"name":"Smoke squad","budget":100,"purchasePrices":{"1":5,"2":5,"4":5,"5":5,"6":5,"7":5,"22":5,"8":5,"9":5,"10":5,"11":5,"12":5,"13":5,"14":5,"15":5},"startingPlayerIds":[1,4,5,6,8,9,10,11,13,14,15],"benchPlayerIds":[2,7,12,22],"captainId":13,"viceCaptainId":8,"formation":"3-4-3"}' >/dev/null

recommendation=$(curl -fsS -X POST "${BASE_URL}/api/v1/recommendations" -H 'Content-Type: application/json' --data '{}')
printf '%s\n' "${recommendation}" | grep -q '"recommendation"'
echo "Fantasy Helper smoke test passed against ${BASE_URL}"

