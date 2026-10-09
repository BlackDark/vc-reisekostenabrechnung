# Authentication

Three ways to sign in can be combined. At least one must be enabled or the process refuses to start. Sessions are HTTP-only cookies. Set `COOKIE_SECURE=false` only for HTTP on a local machine.

## Password

`AUTH_PASSWORD_ENABLED=true` (the default). Passwords are stored with Argon2id (`ARGON2_MEMORY_KIB`, `ARGON2_TIME`, `ARGON2_THREADS`).

The first account is the setup token from the log, or `INITIAL_ADMIN_USERNAME` plus `INITIAL_ADMIN_PASSWORD` / `INITIAL_ADMIN_PASSWORD_FILE`. Further users are created by an admin.

Login attempts are limited by `RATE_LIMIT_LOGIN` (default `5/m`, per client IP) and `RATE_LIMIT_LOGIN_ACCOUNT` (default `20/h`, per account and client IP).

## OIDC (Pocket ID)

Pocket ID, or any other OpenID Connect provider, can be the identity source. Create a client whose redirect URL is:

```text
${APP_BASE_URL}/api/v1/auth/oidc/callback
```

Example for a Pocket ID instance at `https://id.example.com`. Pocket ID puts group names in the `groups` claim; create two groups and assign people there.

```sh
OIDC_ENABLED=true
OIDC_ISSUER_URL=https://id.example.com
OIDC_CLIENT_ID=reisekosten
OIDC_CLIENT_SECRET_FILE=/run/secrets/oidc_secret
OIDC_SCOPES=openid profile email groups
OIDC_GROUPS_CLAIM=groups
OIDC_ALLOWED_GROUP=reisekosten-users
OIDC_ADMIN_GROUP=reisekosten-admins
OIDC_BUTTON_LABEL=Sign in with Pocket ID
OIDC_AUTO_REDIRECT=false
OIDC_ALLOW_EMAIL_LINKING=false
```

Leave `OIDC_CLIENT_SECRET` empty for a public client that uses PKCE only. `OIDC_ALLOWED_GROUP` is who may be created on first login. `OIDC_ADMIN_GROUP` members become admins. `OIDC_ALLOW_EMAIL_LINKING=true` attaches a verified email to an existing account. `OIDC_AUTO_REDIRECT=true` skips the login form when password login is off.

## Trusted header

Use this when a reverse proxy has already authenticated the browser. It starts only if `TRUSTED_PROXIES` lists the proxy (comma-separated CIDRs). Headers from any other address are ignored.

```sh
HEADER_AUTH_ENABLED=true
TRUSTED_PROXIES=10.0.0.0/8
HEADER_AUTH_USER_HEADER=Remote-User
HEADER_AUTH_EMAIL_HEADER=Remote-Email
HEADER_AUTH_NAME_HEADER=Remote-Name
HEADER_AUTH_GROUPS_HEADER=Remote-Groups
HEADER_AUTH_ALLOWED_GROUP=reisekosten-users
HEADER_AUTH_ADMIN_GROUP=reisekosten-admins
HEADER_AUTH_LOGOUT_URL=https://id.example.com/logout
```

Caddy and oauth2-proxy both speak this pattern: they set `Remote-User` and `Remote-Groups` and forward only from the proxy network. The app does not trust `X-Forwarded-For` except from `TRUSTED_PROXIES`.
