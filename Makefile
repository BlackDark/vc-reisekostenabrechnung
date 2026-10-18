.PHONY: check test web generate fmt embed dev dev-reset

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

# Local test server: builds the SPA, embeds it and starts the app on :8080.
# Login: smoke / smoke-password-1. Data lives in tmp/dev-data (see dev-reset).
DEVELOP_DIR ?= $(CURDIR)/tmp/dev-data
TYPST_PATH ?= $(shell mise which typst 2>/dev/null || command -v typst)

embed:
	pnpm install --frozen-lockfile
	pnpm --filter web build
	rm -rf internal/webui/dist
	cp -R web/dist internal/webui/dist
	touch internal/webui/dist/.gitkeep

dev: embed
	mkdir -p $(DEVELOP_DIR)
	DATA_DIR=$(DEVELOP_DIR) \
	INITIAL_ADMIN_USERNAME=smoke \
	INITIAL_ADMIN_PASSWORD=smoke-password-1 \
	APP_BASE_URL=http://127.0.0.1:8080 \
	COOKIE_SECURE=false \
	LISTEN_ADDR=:8080 \
	TYPST_PATH=$(TYPST_PATH) \
	RATE_LIMIT_LOGIN=600/m RATE_LIMIT_LOGIN_ACCOUNT=5000/h RATE_LIMIT_API=50000/m \
	go run ./cmd/reisekosten serve

dev-reset:
	rm -rf $(DEVELOP_DIR)
