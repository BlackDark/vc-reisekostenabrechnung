# Configuration

Every setting is an environment variable. A secret also has a `<NAME>_FILE` form (a file path, for example a Docker secret). The file wins when both are set. `reisekosten config check` validates a configuration without serving.

Compose reads `.env` from the directory of the compose file. `APP_BASE_URL` is required there (`${APP_BASE_URL:?required}`). `APP_VERSION` and `APP_PORT` are Compose variables, not application settings: they select the image tag (default `latest`) and the published host port (default `8080`).

`FX_ECB_URL`, `FX_TIMEOUT` and `EXPORT_PDF_STANDARD` are read at startup and applied; their defaults match the previously compiled-in values. Every variable below is read at startup, and a malformed value (including a non-boolean where a boolean is expected) stops the process.

Invalid combinations stop the process with a clear error (every login method off, or S3 without an EU/EEA location).

| Variable | Default | Meaning |
|---|---|---|
| **General** | | |
| `APP_BASE_URL` | required | Public URL, for example `https://reisekosten.example.com`. OIDC redirect, CSRF origin, links |
| `LISTEN_ADDR` | `:8080` | HTTP listen address. TLS ends at the reverse proxy |
| `DATA_DIR` | `/data` | Database and local files |
| `DB_PATH` | `${DATA_DIR}/reisekosten.db` | SQLite file |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` or `text` |
| `DEFAULT_LOCALE` | `de` | `de` or `en` |
| `TZ_DEFAULT` | `Europe/Berlin` | Validated, not applied. A new trip still defaults to `Europe/Berlin` unless the trip sets a zone |
| `TRUSTED_PROXIES` | empty | Comma-separated CIDRs. Client IP from `X-Forwarded-For`, and header auth, only from these addresses |
| `COOKIE_SECURE` | `true` | Set `false` only for local HTTP |
| `SESSION_IDLE_TIMEOUT` | `168h` | |
| `SESSION_LIFETIME` | `720h` | |
| **First admin** | | |
| `INITIAL_ADMIN_USERNAME` | empty | Creates the first admin when no user exists |
| `INITIAL_ADMIN_EMAIL` | empty | |
| `INITIAL_ADMIN_PASSWORD` (`_FILE`) | empty | |
| **Password** | | |
| `AUTH_PASSWORD_ENABLED` | `true` | |
| `ARGON2_MEMORY_KIB` | `65536` | |
| `ARGON2_TIME` | `3` | |
| `ARGON2_THREADS` | `4` | |
| **OIDC** | | |
| `OIDC_ENABLED` | `false` | |
| `OIDC_ISSUER_URL` | | For example `https://id.example.com` (Pocket ID) |
| `OIDC_CLIENT_ID` | | |
| `OIDC_CLIENT_SECRET` (`_FILE`) | empty | Empty means a public client (PKCE only) |
| `OIDC_SCOPES` | `openid profile email groups` | |
| `OIDC_GROUPS_CLAIM` | `groups` | |
| `OIDC_ADMIN_GROUP` | empty | Members are admins |
| `OIDC_ALLOWED_GROUP` | empty | Members get an account on first login. Empty means no automatic account |
| `OIDC_ALLOW_EMAIL_LINKING` | `false` | Link a verified email to an existing account |
| `OIDC_BUTTON_LABEL` | `Mit SSO anmelden` | |
| `OIDC_AUTO_REDIRECT` | `false` | |
| **Header auth** | | |
| `HEADER_AUTH_ENABLED` | `false` | Requires `TRUSTED_PROXIES` |
| `HEADER_AUTH_USER_HEADER` | `Remote-User` | |
| `HEADER_AUTH_EMAIL_HEADER` | `Remote-Email` | |
| `HEADER_AUTH_NAME_HEADER` | `Remote-Name` | |
| `HEADER_AUTH_GROUPS_HEADER` | `Remote-Groups` | |
| `HEADER_AUTH_ADMIN_GROUP` | `${OIDC_ADMIN_GROUP}` | |
| `HEADER_AUTH_ALLOWED_GROUP` | `${OIDC_ALLOWED_GROUP}` | |
| `HEADER_AUTH_LOGOUT_URL` | empty | |
| **Storage** | | |
| `STORAGE_BACKEND` | `local` | `local` or `s3` |
| `STORAGE_LOCAL_PATH` | `${DATA_DIR}/files` | |
| `S3_ENDPOINT` | | For example `https://fsn1.your-objectstorage.com` |
| `S3_REGION` | | |
| `S3_BUCKET` | | |
| `S3_PREFIX` | empty | |
| `S3_ACCESS_KEY_ID` | | |
| `S3_SECRET_ACCESS_KEY` (`_FILE`) | | |
| `S3_USE_PATH_STYLE` | `false` | `true` for MinIO or Garage |
| `S3_DATA_LOCATION` | required for S3 | ISO country of the bucket (EU/EEA), for example `DE` or `FI` |
| `S3_ALLOW_NON_EU` | `false` | Only with permission under § 146 (2b) AO. Logs a warning |
| **Receipts** | | |
| `UPLOAD_MAX_BYTES` | `26214400` | 25 MiB |
| `BELEG_FORMAT` | `avif` | `avif` or `webp` |
| `BELEG_AVIF_QUALITY` | `70` | 1–100. 70 keeps a clean scan above 0.95 SSIM, lower values blur small print |
| `BELEG_AVIF_SPEED` | `8` | 0 is slow, 10 is fast. 8 encodes all-intra about four times faster than 6 |
| `BELEG_WEBP_QUALITY` | `55` | When `BELEG_FORMAT=webp` |
| `BELEG_JPEG_QUALITY` | `70` | JPEG quality for the photo and PDF-page derivatives, including receipts embedded in the export |
| `BELEG_ERFASSUNG_KARENZ` | `720h` | How long the capture JPEG is kept after confirmation |
| **Optional receipt reading** | | |
| `AI_ENABLED` | `false` | Suggestions only, and only after the user opts in |
| `AI_BASE_URL` | | For example `https://api.openai.com/v1` or `http://ollama:11434/v1` |
| `AI_API_KEY` (`_FILE`) | empty | |
| `AI_MODEL` | | |
| `AI_TIMEOUT` | `60s` | |
| `AI_MAX_CONCURRENCY` | `2` | |
| `AI_MAX_IMAGE_PX` | `2480` | Longest edge sent to the model |
| `AI_RESPONSE_FORMAT` | `auto` | `auto`, `json_schema`, `json_object`, or `text` |
| **Exchange rates and export** | | |
| `FX_ECB_URL` | `https://data-api.ecb.europa.eu` | Scheme and host for the ECB rates endpoint. Only the scheme and host are used; the data path is fixed by the currency |
| `FX_TIMEOUT` | `4s` | Deadline and HTTP client timeout for the ECB and BMF rate fetches. Must be positive |
| `TYPST_PATH` | `/usr/local/bin/typst` | |
| `EXPORT_TIMEOUT` | `120s` | Bounds the Typst export pipeline (render, raster and PDF compile) |
| `EXPORT_PDF_STANDARD` | `a-3b` | PDF standard passed to Typst (for example `a-3b` for PDF/A-3b) |
| **Rate limits** | | |
| `RATE_LIMIT_LOGIN` | `5/m` | Per client IP |
| `RATE_LIMIT_LOGIN_ACCOUNT` | `20/h` | Per account and client IP |
| `RATE_LIMIT_API` | `300/m` | |
| `RATE_LIMIT_UPLOAD` | `30/m` | |
| `RATE_LIMIT_AI` | `20/m` | |
| **Jobs and retention** | | |
| `JOB_WORKERS` | `2` | Background workers |
| `RETENTION_REPORT_ENABLED` | `true` | Monthly retention report for admins |
| **Litestream overlay** | | |
| `LITESTREAM_ACCESS_KEY_ID` | | Required only with `docker-compose.backup.yml` |
| `LITESTREAM_SECRET_ACCESS_KEY` | | |
| `LITESTREAM_REPLICA_URL` | | `s3://bucket/path` in the EU/EEA |

Sign-in details and a Pocket ID example are in [Authentication](authentication.md). Backup, restore, and retention are in [Operations](operations.md).

## Email receipt intake (IMAP poller)

Forward or redirect receipts to a mailbox the app polls; their attachments then land in the Belegeingang (unassigned receipts). Off by default; set `MAIL_IMAP_HOST` to enable.

| Variable | Default | Meaning |
|---|---|---|
| `MAIL_IMAP_HOST` | | IMAP host. Empty disables the poller; when set, `MAIL_FORWARD_DOMAIN` is also required |
| `MAIL_IMAP_PORT` | `993` | IMAP port (implicit TLS) |
| `MAIL_IMAP_USERNAME` | | Mailbox login (required with `MAIL_IMAP_HOST`) |
| `MAIL_IMAP_PASSWORD` (`_FILE`) | | Mailbox password |
| `MAIL_IMAP_FOLDER` | `INBOX` | Mailbox folder to poll |
| `MAIL_IMAP_INTERVAL` | `5m` | Poll interval; must be positive |
| `MAIL_IMAP_PROCESSED_ACTION` | `flag` | After filing: `flag`, `delete`, or `move:<mailbox>` |
| `MAIL_FORWARD_DOMAIN` | | Domain of the per-user plus address `belege+<hash>@<domain>` |

Each user gets a `mail:ingest` token (Profile; the plaintext is shown once). Its forwarding address is `belege+<SHA-256 of that token>@MAIL_FORWARD_DOMAIN`, because the address travels through mail servers and mailboxes: whoever reads it can send mail into the account, but the API does not accept it as a bearer token. Addresses issued before this change carried the token itself and keep working; a new token replaces one. On each poll the app reads unseen mail and, per message, picks the owner by that plus-address token first, then by the sender allowlist (addresses and `@domains` the user listed). It files PDF, image and XML attachments into that owner's Belegeingang (inline logos and text parts are skipped, duplicates dedupe by checksum), archives the original message, and then flags, deletes or moves it per the processed action. A message it cannot map, or that fails to ingest, is left unseen and retried next tick.

Trust note: the poller files whatever a mapped sender attaches, so only a user's forwarding-token senders and allowlisted senders can add receipts to their inbox.
