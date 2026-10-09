#!/bin/sh
set -eu
docker rm -f rk-e2e rk-mock >/dev/null 2>&1 || true
pkill -f scripts/ki-stub.py >/dev/null 2>&1 || true
docker volume rm rk-e2e-data >/dev/null 2>&1 || true
docker volume create rk-e2e-data >/dev/null
docker run -d --name rk-mock --network host \
  -e SERVER_PORT=8089 \
  ghcr.io/navikt/mock-oauth2-server:6.0.5@sha256:5ed2f078c6503147860a5114f06414257bb7eeccc68b0e806bdf0869f2602509
# Each Playwright test signs in, so the suite needs a wider login bucket than production.
docker run -d --name rk-e2e --network host \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=256m \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  -v rk-e2e-data:/data \
  -e APP_BASE_URL=http://127.0.0.1:8080 \
  -e COOKIE_SECURE=false \
  -e LISTEN_ADDR=:8080 \
  -e INITIAL_ADMIN_USERNAME=smoke \
  -e INITIAL_ADMIN_PASSWORD=smoke-password-1 \
  -e BELEG_AVIF_SPEED=8 \
  -e RATE_LIMIT_LOGIN=60/m \
  -e OIDC_ENABLED=true \
  -e OIDC_ISSUER_URL=http://127.0.0.1:8089/default \
  -e OIDC_CLIENT_ID=reisekosten \
  -e OIDC_ADMIN_GROUP=rk-admins \
  -e OIDC_ALLOWED_GROUP=rk-users \
  -e OIDC_BUTTON_LABEL=SSO \
  -e AI_ENABLED=true \
  -e AI_BASE_URL=http://127.0.0.1:8091/v1 \
  -e AI_API_KEY=e2e-key \
  -e AI_MODEL=fake \
  -e AI_TIMEOUT=15s \
  -e AI_RESPONSE_FORMAT=json_object \
  vc-reisekosten:ci
nohup python3 scripts/ki-stub.py >/tmp/ki-stub.log 2>&1 &
for i in $(seq 1 40); do
  if curl -fsS http://127.0.0.1:8080/readyz >/dev/null 2>&1 \
    && curl -fsS http://127.0.0.1:8089/default/.well-known/openid-configuration >/dev/null 2>&1 \
    && curl -fsS http://127.0.0.1:8091/health >/dev/null 2>&1; then
    echo "e2e stack ready"
    exit 0
  fi
  sleep 1
done
docker logs rk-e2e >&2 || true
docker logs rk-mock >&2 || true
cat /tmp/ki-stub.log >&2 || true
exit 1
