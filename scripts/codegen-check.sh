#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go tool sqlc generate
go tool oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml
pnpm --filter web codegen
git diff --exit-code -- api internal/api internal/store/sqlitedb web/src/lib/api/schema.ts
echo "codegen diff ok"
