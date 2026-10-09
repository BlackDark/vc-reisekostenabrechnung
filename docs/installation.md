# Installation

The app is a single container. Docker Compose v2 is enough. The repository is public, so these downloads do not need a token. The container image is published to `ghcr.io/blackdark/vc-reisekostenabrechnung` when a release is cut; until that package is public, `docker compose pull` needs a GitHub login with `read:packages`.

## Quickstart

```sh
mkdir reisekosten && cd reisekosten
for f in docker-compose.yml .env.example; do
  curl -fsSL -o "$f" "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/$f"
done
cp .env.example .env
```

Set `APP_BASE_URL` to the public URL (for example `https://reisekosten.example.com`). TLS ends at the reverse proxy. The app listens on port 8080 inside the container.

```sh
docker compose up -d
docker compose logs app | grep -i setup
```

The log line is the setup token for the first admin, and only while no user exists. Open `APP_BASE_URL`, enter the token, and choose a password. After that, sign in with the password.

Compose refuses to start when `APP_BASE_URL` is empty (`fehlt`).

## First admin without the setup page

Set these before the first start, then `docker compose up -d`:

```sh
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=replace-me
```

Prefer `INITIAL_ADMIN_PASSWORD_FILE` when the password comes from a Docker secret. The variables do nothing once a user exists.

## Litestream

Leave Litestream off until you want continuous database replication. Download the overlay and set the three variables:

```sh
curl -fsSL -o docker-compose.backup.yml \
  "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/docker-compose.backup.yml"
curl -fsSL -o litestream.yml \
  "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/litestream.yml"
```

In `.env`:

```sh
LITESTREAM_ACCESS_KEY_ID=...
LITESTREAM_SECRET_ACCESS_KEY=...
LITESTREAM_REPLICA_URL=s3://bucket/reisekosten
```

```sh
docker compose -f docker-compose.yml -f docker-compose.backup.yml up -d
```

Restore steps are in [Operations](operations.md).

## Health

`/healthz` answers liveness. `/readyz` checks the database, migrations, write access to `/data` and `/tmp`, and that Typst runs. The image runs as UID 65532 with a read-only root filesystem.
