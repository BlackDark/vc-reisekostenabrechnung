#!/bin/sh
set -eu
image=${1:-vc-reisekosten:ci}
name=rk-smoke
vol=rk-smoke-data
pw=$(mktemp)
printf '%s' 'smoke-password-1' > "$pw"
# The container runs as uid 65532 and cannot read a 0600 file owned by the runner.
chmod a+r "$pw"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$vol" >/dev/null 2>&1 || true
  rm -f "$pw"
}
trap cleanup EXIT
docker volume create "$vol" >/dev/null
docker run -d --name "$name" \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=256m \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  -v "$vol":/data \
  -p 8080:8080 \
  -e APP_BASE_URL=http://127.0.0.1:8080 \
  -e COOKIE_SECURE=false \
  -e TRUSTED_PROXIES=127.0.0.1/32 \
  -e INITIAL_ADMIN_USERNAME=smoke \
  -e INITIAL_ADMIN_PASSWORD_FILE=/run/secrets/pw \
  -v "$pw:/run/secrets/pw:ro" \
  "$image"
fail() {
  echo "smoke failed: $1" >&2
  docker logs "$name" >&2 || true
  exit 1
}
for i in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    fail "healthz"
  fi
  sleep 1
done
curl -fsS http://127.0.0.1:8080/readyz >/dev/null || fail "readyz"
curl -fsS http://127.0.0.1:8080/login | grep -q 'name="rk-app"' || fail "login html"
code=$(curl -sS -o /tmp/login.json -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d '{"benutzername":"smoke","passwort":"smoke-password-1"}' \
  http://127.0.0.1:8080/api/v1/auth/login)
if [ "$code" != "200" ]; then
  cat /tmp/login.json >&2 || true
  fail "login $code"
fi
# docker cp cannot read files on the /tmp tmpfs, so the sample is written on the data volume.
docker exec "$name" /usr/local/bin/reisekosten export-sample --out /data/sample.pdf || fail "export-sample"
docker cp "$name":/data/sample.pdf /tmp/sample.pdf || fail "pdf copy"
head -c 5 /tmp/sample.pdf | grep -q '%PDF' || fail "pdf magic"
# The image probes every 60s, so allow two probe intervals before giving up.
healthy=0
for i in $(seq 1 40); do
  status=$(docker inspect --format '{{.State.Health.Status}}' "$name" 2>/dev/null || true)
  if [ "$status" = "healthy" ]; then
    healthy=1
    break
  fi
  sleep 3
done
if [ "$healthy" != "1" ]; then
  fail "health status ${status:-missing}"
fi
docker restart "$name" >/dev/null
for i in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    echo "smoke ok"
    exit 0
  fi
  sleep 1
done
fail "healthz after restart"
