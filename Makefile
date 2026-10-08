.PHONY: check test web generate fmt

check: test web

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './docs/*' -not -path './internal/api/*' -not -path './internal/store/sqlitedb/*')

web:
	pnpm install --frozen-lockfile
	pnpm exec biome ci .
	pnpm --filter web check
	pnpm --filter web build
	node scripts/check-i18n.mjs
	node scripts/bundle-budget.mjs

generate:
	go tool sqlc generate
	go tool oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml
	pnpm --filter web codegen
