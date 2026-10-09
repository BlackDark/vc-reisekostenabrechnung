#!/bin/sh
# Proves Litestream 0.5.17-scratch can replicate and restore as UID 65532
# with a read-only root filesystem and HOME=/tmp (SPEC O16).
set -eu
image=litestream/litestream:0.5.17-scratch@sha256:f757c70d070ac278d45b8847d31a54ab2de24de5e77b09018c642eca263e3967
root=$(mktemp -d)
name=rk-ls-smoke-$$
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  # Litestream writes replica files and .reisekosten.db-litestream as UID 65532.
  # The runner cannot unlink those; root can. A cleanup failure must not fail the job.
  if [ -n "${root:-}" ] && [ -d "$root" ]; then
    sudo -n rm -rf "$root" || rm -rf "$root" || true
  fi
}
trap cleanup EXIT
mkdir -p "$root/data" "$root/replica"
python3 - "$root/data/reisekosten.db" << 'PY'
import sqlite3, sys
c = sqlite3.connect(sys.argv[1])
c.execute("create table probe(v text)")
c.execute("insert into probe values ('restored')")
c.commit()
c.close()
PY
chmod 777 "$root" "$root/data" "$root/replica"
chmod 666 "$root/data/reisekosten.db"
cat > "$root/litestream.yml" << 'EOF'
snapshot:
  interval: 1s
dbs:
  - path: /data/reisekosten.db
    replicas:
      - url: file:///replica
EOF
docker run -d --name "$name" \
  --user 65532:65532 \
  --read-only \
  --cap-drop ALL \
  --tmpfs /tmp:rw,nosuid,size=32m \
  -e HOME=/tmp \
  -v "$root/data":/data \
  -v "$root/replica":/replica \
  -v "$root/litestream.yml":/etc/litestream.yml:ro \
  "$image" replicate
sleep 12
docker logs "$name" || true
docker stop -t 10 "$name" >/dev/null
docker rm -f "$name" >/dev/null
if ! find "$root/replica" -type f | grep -q .; then
  echo "litestream replica is empty" >&2
  exit 1
fi
docker run --rm \
  --user 65532:65532 \
  --read-only \
  --cap-drop ALL \
  --tmpfs /tmp:rw,nosuid,size=32m \
  -e HOME=/tmp \
  -v "$root/data":/data \
  -v "$root/replica":/replica \
  -v "$root/litestream.yml":/etc/litestream.yml:ro \
  "$image" restore -config /etc/litestream.yml -o /data/restored.db /data/reisekosten.db
python3 - "$root/data" << 'PY'
import os, sqlite3, sys
root = sys.argv[1]
path = os.path.join(root, "restored.db")
if not os.path.exists(path):
    raise SystemExit("restored database missing")
value = sqlite3.connect(path).execute("select v from probe").fetchone()[0]
if value != "restored":
    raise SystemExit("unexpected value " + value)
print("restored", value)
print("data", os.listdir(root))
PY
find "$root/replica" -type f -printf '%u %p\n' | head
if find "$root/replica" -type f ! -uid 65532 | grep -q .; then
  echo "replica files are not owned by uid 65532" >&2
  exit 1
fi
echo "litestream restore ok"
