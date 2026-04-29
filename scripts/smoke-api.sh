#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8080}"

status() {
  code="$(curl -s -o /dev/null -w '%{http_code}' "$@")"
  printf '%s\n' "$code"
}

unauthorized="$(status "$BASE_URL/internal/auth/verify")"
if [ "$unauthorized" != "401" ]; then
  echo "expected /internal/auth/verify without token to return 401, got $unauthorized"
  exit 1
fi

sales_token="$(curl -s "$BASE_URL/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"sales","password":"demo"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"

if [ -z "$sales_token" ]; then
  echo "failed to get sales token"
  exit 1
fi

forbidden="$(status "$BASE_URL/api/schedules/jobs" \
  -H "Authorization: Bearer $sales_token" \
  -H 'Content-Type: application/json' \
  -d '{"lineId":"A","startDate":"2026-05-01"}')"
if [ "$forbidden" != "403" ]; then
  echo "expected sales schedule job request to return 403, got $forbidden"
  exit 1
fi

echo "API smoke verification passed"
