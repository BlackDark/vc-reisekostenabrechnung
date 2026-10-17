.PHONY: check test web generate fmt

TOOLS := $(CURDIR)/tools

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
	go -C $(TOOLS) tool sqlc generate -f $(CURDIR)/sqlc.yaml
	go -C $(TOOLS) tool oapi-codegen --config $(CURDIR)/api/oapi-codegen.yaml \
		-o $(CURDIR)/internal/api/server.gen.go $(CURDIR)/api/openapi.yaml
	pnpm --filter web codegen
