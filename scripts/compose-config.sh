#!/bin/sh
# Renders both Compose files from .env.example with the required values filled in.
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp "$repo/deploy/docker-compose.yml" "$repo/deploy/docker-compose.backup.yml" "$repo/deploy/.env.example" "$tmp/"
sed -e 's|^APP_VERSION=.*|APP_VERSION=v1.1.0|' \
  -e 's|^APP_BASE_URL=.*|APP_BASE_URL=https://reisekosten.example|' \
  "$tmp/.env.example" > "$tmp/.env"
# Compose mounts one file per secret from ./secrets; docker compose up refuses to
# start without them. An empty file means the setting stays unset.
mkdir -p "$tmp/secrets"
for name in initial_admin_password oidc_client_secret s3_secret_access_key mail_imap_password ai_api_key; do
  : > "$tmp/secrets/$name"
done
cp "$repo/deploy/litestream.yml" "$tmp/secrets/litestream.yml"
cd "$tmp"
docker compose --env-file .env -f docker-compose.yml config >/dev/null
docker compose --env-file .env -f docker-compose.yml -f docker-compose.backup.yml config >/dev/null
echo "compose config ok"
