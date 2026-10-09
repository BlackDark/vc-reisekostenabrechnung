#!/bin/sh
# Renders both Compose files from .env.example with the required values filled in.
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp "$repo/deploy/docker-compose.yml" "$repo/deploy/docker-compose.backup.yml" "$repo/deploy/litestream.yml" "$repo/deploy/.env.example" "$tmp/"
sed -e 's|^APP_BASE_URL=.*|APP_BASE_URL=https://reisekosten.example|' \
  -e 's|^LITESTREAM_ACCESS_KEY_ID=.*|LITESTREAM_ACCESS_KEY_ID=test|' \
  -e 's|^LITESTREAM_SECRET_ACCESS_KEY=.*|LITESTREAM_SECRET_ACCESS_KEY=test|' \
  -e 's|^LITESTREAM_REPLICA_URL=.*|LITESTREAM_REPLICA_URL=s3://bucket/pfad|' \
  "$tmp/.env.example" > "$tmp/.env"
cd "$tmp"
docker compose --env-file .env -f docker-compose.yml config >/dev/null
docker compose --env-file .env -f docker-compose.yml -f docker-compose.backup.yml config >/dev/null
echo "compose config ok"
