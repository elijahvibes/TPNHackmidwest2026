#!/usr/bin/env bash
# Smoke test every API endpoint. Usage: scripts/smoke.sh [base-url]
# Exits non-zero on the first failure. Needs curl + node.
set -euo pipefail
BASE="${1:-http://localhost:8080}"
EMAIL="smoke+$(date +%s)@trailmix.app"
PASS="smoke-pass-123"
fail() { echo "FAIL: $*" >&2; exit 1; }
j() { node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{const o=JSON.parse(s);const v=eval('o.'+process.argv[1]);if(v===undefined||v===null||v===false)process.exit(2);console.log(typeof v==='object'?JSON.stringify(v):v)})" "$1"; }

echo "== health"
curl -sf "$BASE/api/health" | j ok >/dev/null || fail health
DB=$(curl -s "$BASE/api/health" | j db || echo false)
echo "   db=$DB"

echo "== plan preview (no auth)"
PREVIEW=$(curl -sf -X POST "$BASE/api/plan/preview" -H 'Content-Type: application/json' \
  -d '{"origin":"Kansas City","destination":"Denver","start_date":"2026-11-06","end_date":"2026-11-09","travelers":2,"budget_usd":1200}')
echo "$PREVIEW" | j recommended_mode >/dev/null || fail preview
echo "   recommended=$(echo "$PREVIEW" | j recommended_mode) total=$(echo "$PREVIEW" | j total_usd)"

echo "== vehicle mpg"
curl -sf "$BASE/api/vehicles/mpg?year=2019&make=Honda&model=Civic" | j mpg >/dev/null || fail mpg

if [ "$DB" != "true" ]; then
  echo "db unavailable; skipping account tests"
  exit 0
fi

echo "== register"
TOKEN=$(curl -sf -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"name\":\"Smoke Test\",\"password\":\"$PASS\"}" | j token) || fail register
AUTH="Authorization: Bearer $TOKEN"

echo "== login"
curl -sf -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | j token >/dev/null || fail login

echo "== me + update home"
curl -sf "$BASE/api/auth/me" -H "$AUTH" | j email >/dev/null || fail me
curl -sf -X PATCH "$BASE/api/auth/me" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"home_city":"Kansas City","home_lat":39.1,"home_lng":-94.6}' | j home_city >/dev/null || fail update-me

echo "== create trip"
TRIP=$(curl -sf -X POST "$BASE/api/trips" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"origin":"Kansas City","destination":"Nashville","start_date":"2026-11-13","end_date":"2026-11-16","travelers":2,"budget_usd":900,"mode":"auto"}' | j id) || fail create-trip
echo "   trip=$TRIP"

echo "== plan trip"
curl -sf -X POST "$BASE/api/trips/$TRIP/plan" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"vehicle_mpg":33}' | j total_usd >/dev/null || fail plan-trip

echo "== list + get"
curl -sf "$BASE/api/trips" -H "$AUTH" | j "length" >/dev/null || fail list
STATUS=$(curl -sf "$BASE/api/trips/$TRIP" -H "$AUTH" | j status)
[ "$STATUS" = "planned" ] || fail "status=$STATUS want planned"

echo "== delete trip"
code=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$BASE/api/trips/$TRIP" -H "$AUTH")
[ "$code" = "204" ] || fail "delete code=$code"

echo "ALL GREEN ($BASE)"
