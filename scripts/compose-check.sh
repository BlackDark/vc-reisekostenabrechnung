#!/bin/sh
set -eu
cd "$(dirname "$0")/../deploy"
cp .env.example .env
# Required public URL. Secrets stay empty in the example; the backup override
# needs non-empty values only so Compose interpolation can succeed.
sed -i 's|^APP_BASE_URL=.*|APP_BASE_URL=https://reisekosten.example|' .env
trap 'rm -f .env' EXIT
set +e
out=$(docker compose -f docker-compose.yml config 2>&1)
status=$?
set -e
if [ "$status" -ne 0 ]; then
  echo "$out" >&2
  exit "$status"
fi
env -u APP_BASE_URL APP_BASE_URL= docker compose --env-file /dev/null -f docker-compose.yml config >/tmp/compose-missing.out 2>&1 || true
if ! grep -q "fehlt" /tmp/compose-missing.out; then
  echo "expected a missing APP_BASE_URL to fail with 'fehlt'" >&2
  cat /tmp/compose-missing.out >&2
  exit 1
fi
LITESTREAM_ACCESS_KEY_ID=ci \
LITESTREAM_SECRET_ACCESS_KEY=ci \
LITESTREAM_REPLICA_URL=s3://bucket/reisekosten \
  docker compose -f docker-compose.yml -f docker-compose.backup.yml config >/dev/null
echo "compose config ok"
