#!/bin/sh
set -eu
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/db"
cp -a db/migrations db/queries "$root/db/"
cat > "$root/sqlc.yaml" << 'EOF'
version: "2"
sql:
  - engine: postgresql
    schema: db/migrations
    queries: db/queries
    gen:
      go:
        package: pgdb
        out: pgdb
        emit_pointers_for_null_types: true
        sql_package: database/sql
EOF
cd "$root"
go mod init probe >/dev/null
(cd /workspace && go tool sqlc generate -f "$root/sqlc.yaml")
go mod tidy >/dev/null
go build ./pgdb
echo "postgres probe ok"
