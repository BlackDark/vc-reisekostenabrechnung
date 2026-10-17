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

Set `APP_VERSION` to the release to run (for example `v1.1.0`) and `APP_BASE_URL` to the public URL (for example `https://reisekosten.example.com`). Compose refuses to start when either is missing or empty (`fehlt`). TLS ends at the reverse proxy. The app listens on port 8080 inside the container.

Secrets are files, not environment variables: everything in the container environment shows up in `docker inspect` and `/proc/<pid>/environ`. Compose mounts one file per secret from `./secrets` into `/run/secrets`; an empty file means the setting stays unset, and a plaintext value in `.env` next to a mounted file stops the app at startup.

```sh
mkdir -p secrets && chmod 700 secrets
printf '%s' 'das-passwort' > secrets/initial_admin_password
for name in oidc_client_secret s3_secret_access_key mail_imap_password ai_api_key; do
  : > "secrets/$name"
done
chmod 644 secrets/*
```

The container runs as UID 65532 and must read those files, so they stay `0644`; the surrounding `0700` directory keeps other accounts on the host out. Own them by `65532:65532` instead if you prefer `0600`.

```sh
docker compose up -d
docker compose logs app | grep -i setup
```

The log line is the setup token for the first admin, and only while no user exists. Open `APP_BASE_URL`, enter the token, and choose a password. After that, sign in with the password.

## First admin without the setup page

Set `INITIAL_ADMIN_USERNAME=admin` in `.env` and write the password into its file before the first start, then `docker compose up -d`:

```sh
printf '%s' 'replace-me' > secrets/initial_admin_password
```

Both do nothing once a user exists.

## Litestream

Leave Litestream off until you want continuous database replication. Download the overlay and put its configuration into the secrets directory:

```sh
curl -fsSL -o docker-compose.backup.yml \
  "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/docker-compose.backup.yml"
curl -fsSL -o secrets/litestream.yml \
  "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/litestream.yml"
chmod 644 secrets/litestream.yml
```

Fill in the replica and its credentials in `secrets/litestream.yml`:

```yaml
dbs:
  - path: /data/reisekosten.db
    replicas:
      - url: s3://bucket/reisekosten
        access-key-id: ...
        secret-access-key: ...
```

```sh
docker compose -f docker-compose.yml -f docker-compose.backup.yml up -d
```

Compose mounts the file read-only, so the credentials never enter the container environment. Earlier versions read `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY` and `LITESTREAM_REPLICA_URL` from `.env`; the file replaces them. Restore steps are in [Operations](operations.md).

## Health

`/healthz` answers liveness. `/readyz` checks the database, migrations, write access to `/data` and `/tmp`, and that Typst runs. The image runs as UID 65532 with a read-only root filesystem.
