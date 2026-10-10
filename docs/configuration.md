# Configuration

Every setting is an environment variable. A secret also has a `<NAME>_FILE` form (a file path, for example a Docker secret). The file wins when both are set. `reisekosten config check` validates a configuration without serving.

Compose reads `.env` from the directory of the compose file. `APP_BASE_URL` is required there (`${APP_BASE_URL:?fehlt}`). `APP_VERSION` and `APP_PORT` are Compose variables, not application settings: they select the image tag (default `latest`) and the published host port (default `8080`).

`FX_ECB_URL` and `EXPORT_PDF_STANDARD` are listed in `.env.example` for operators. This build still uses the ECB host and PDF/A-3b that are compiled in. The other variables below are read at startup.

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
| `BELEG_JPEG_QUALITY` | `70` | JPEG derivative of a photo receipt and of a PDF page. A PDF-source receipt embedded in the export still uses quality 70 |
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
| `FX_ECB_URL` | ECB data API | Listed for operators. This build fetches the compiled-in ECB host |
| `FX_TIMEOUT` | | Listed in `.env.example`. This build uses a compiled-in 4 second fetch timeout |
| `TYPST_PATH` | `/usr/local/bin/typst` | |
| `EXPORT_TIMEOUT` | `120s` | Loaded and not applied. Typst export uses a fixed 60 second timeout |
| `EXPORT_PDF_STANDARD` | `a-3b` | Listed for operators. This build always writes PDF/A-3b |
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
