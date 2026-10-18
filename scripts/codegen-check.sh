#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$repo"
# The generators live in their own module (tools/), so the main module graph stays
# free of their dependencies; -C runs them from there and the paths stay absolute.
go -C tools tool sqlc generate -f "$repo/sqlc.yaml"
go -C tools tool oapi-codegen --config "$repo/api/oapi-codegen.yaml" \
  -o "$repo/internal/api/server.gen.go" "$repo/api/openapi.yaml"
pnpm --filter web codegen
git diff --exit-code -- api internal/api internal/store/sqlitedb web/src/lib/api/schema.ts
echo "codegen diff ok"
