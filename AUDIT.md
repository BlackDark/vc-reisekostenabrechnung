# Codebase audit, 2026-10-17

Baseline: main 0604fce (v1.1.0). Method: the codebase-audit checklist, three read-only subagents
(code + comments, CI/CD + cleanup, dependencies + security), then one skeptical review pass.
Related: `docs/ideas/code-health.md` holds the earlier pass (ids 1.x, 2.x, 3.x, landed or decided
against). Those items are not repeated here.

Grade: B. The tree is small for what it does, pinned end to end, and documented more than most. The
real problems are one dead distribution channel (CI-1), duplicated security and error-mapping paths in
`internal/httpapi` (CODE-1, CODE-2), and no license file for a product meant to be self-hosted (DEP-1).

Numbers: 652 tracked files, 53,985 Go lines (106 non-test + 44 test), 17,694 web lines, 19 CI jobs,
0 reachable known CVEs, 34 findings: 0 high, 14 medium, 20 low. By area: code 12, comments 3, CI/CD 7,
dependencies 4, security 6, cleanup 2.

Top 5 by benefit / effort:

1. CI-1 make `publish-edge` run again; the documented `:edge` tag never published (S).
2. CODE-3 delete the byte-identical `Rates`/`RatesFor` pair with its false doc claim (S, COM-2).
3. SEC-1 hash session tokens at rest, as API tokens already are (M).
4. CODE-9 ask the server for the receipt ids instead of filtering the first 100 in the browser (M).

Tool coverage: ripgrep, govulncheck 1.8.0, `go vet`, `go test`, `go mod tidy -diff`, `go mod verify`,
shellcheck, actionlint, `pnpm audit`, `pnpm outdated`, docker. Not available, so unverified: gitleaks,
trivy, osv-scanner, semgrep, hadolint, tokei/cloc, license tooling; `yamllint` fails to start.
Worktree leftovers (`out/`, `e2e/test-results/`, `e2e/playwright-report/`, `node_modules/`) are
git-ignored and untracked, so they are not committed clutter.

Areas: code 12 findings, CI/CD 7 -> `docs/audit/code.md`, `docs/audit/cicd.md`. The rest below.

review: kept 34, dropped 3 (COM-5, CLN-4, CLN-5), merged 2 (COM-2 into CODE-3, CLN-2 into DEP-1),
downgraded 0; 2 findings added from the review (SEC-5, SEC-6).
Dropped: COM-5 wording nit, CLN-4 speculative (the tool header ties it to an open ADR 0005),
CLN-5 the research docs reference those files.

## Comments (3)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| COM-1 | M | `internal/httpapi/mail.go:131-133` | Says "Not in this slice (documented follow-ups): per-user sender allowlist and SPF/DKIM verification, a Beleg source field, and the IMAP poller itself." All four exist: `mailauth.Parse`/`Rejected` (`:190-193`), `MailOwnerForAbsender` (`mailpoll.go:156`), `Quelle: "mail"` (`:226`), `StartMailPoller` (`mailpoll.go:27`). | Delete the stale follow-up list. | S | Readers stop believing verified-sender handling is absent. |
| COM-3 | M | `internal/auth/provision.go:50-52`, `internal/auth/json.go:1-7` | "Local wrapper keeps encoding/json out of the hot import cycle for tests." There is no import cycle: `unmarshalStringList` calls `jsonUnmarshal`, which calls `json.Unmarshal`. Two wrappers, one call. | Delete both wrappers and the comment. | S | Removes a false rationale plus two layers of indirection. |
| COM-4 | L | `internal/berechnung/policy.go:1` | "Package berechnung holds calculation policy that later milestones implement." The package holds the whole calculator now (`berechne.go` 982 lines plus `land.go`, `frist.go`, `mwst.go`). | Update the package comment. | S | Package doc matches the package. |

## Dependencies (4)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| DEP-1 | M | repository root; `docs/SPEC.md:1024` | No license file is tracked (`git ls-files \| rg -i license` is empty) although the SPEC lists `LICENSE` in the layout and the product is published for self-hosting. Without one, users get no grant and no terms. | Add the intended license and its SPDX identifier; the SPEC layout entry then matches (absorbs CLN-2). | S | Legal clarity for a self-hosted product. |
| DEP-2 | L | `go.mod:16` (`golang.org/x/crypto@v0.57.0`) | govulncheck: "1 vulnerability in modules you require, but your code doesn't appear to call these vulnerabilities" (GO-2026-5932, `openpgp`, `Fixed in: N/A`). `openpgp` is never imported (`rg openpgp internal/` is empty). | Track `v0.58.0`; nothing else is actionable while no fix exists. | S | Clears the only govulncheck module hit. |
| DEP-3 | L | `go.mod:12,14,15,16,17` | Five direct dependencies are behind latest, all minor or patch, none a major: minio-go 7.3.0→7.4.1, goose 3.28.0→3.29.0, jsonschema 6.0.2→6.0.3, x/crypto 0.57.0→0.58.0, x/image 0.46.0→0.47.0. Renovate already groups them. | None; let Renovate land them. | S | Stays current without manual work. |
| DEP-4 | L | `go.mod:34,56`; accepted in `docs/SPEC.md:1261` | Codegen tooling sits in the main module graph, which pulls in a second JWT major (`golang-jwt/jwt/v4 v4.5.2` beside `jwt/v5`), ClickHouse/YDB/Vertica/MSSQL/Azure drivers and the deprecated `github.com/golang/protobuf v1.5.4`. It inflates `go.sum` and hides DEP-2 among tooling noise. | Move the tools to their own module. | M | Smaller graph, quieter vulnerability scans. |

## Security (6)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| SEC-1 | L | `internal/store/session.go:107`, `db/queries/session.sql:16-17` | Session bearer tokens are stored in plaintext and looked up by value (`Token: token`, `WHERE token = sqlc.arg(token)`), while API tokens are hashed (`internal/auth/token.go:24`, `HashAPIToken`). A database read yields live sessions (idle 168h, lifetime 720h). | Store `SHA256(token)` and look up by hash, as API tokens do. | M | A database leak no longer hands out live sessions. |
| SEC-2 | L | `internal/httpapi/server.go:346-368`, `:102-104` | `/readyz` is unauthenticated and unthrottled (`allowAPI` skips non-`/api/` paths, `server.go:150-153`) and returns raw `err.Error()` for the database, `DATA_DIR`, `/tmp`, `TYPST_PATH` and storage failures, so filesystem paths and permission errors leak. The request-binding handler returns `err.Error()` as problem `detail` for any 4xx parse failure. | Return a generic detail, log the specifics. | S | Less internal disclosure to unauthenticated callers. |
| SEC-3 | L | `deploy/docker-compose.yml:12`, `deploy/docker-compose.backup.yml:11-15` | Secrets arrive as container environment (`env_file: .env` for `S3_SECRET_ACCESS_KEY`, `OIDC_CLIENT_SECRET`, `INITIAL_ADMIN_PASSWORD`, `MAIL_IMAP_PASSWORD`; Litestream credentials via `environment:`), so `docker inspect` and `/proc/<pid>/environ` expose them. The app already supports `*_FILE` variants (`deploy/.env.example:22,33,59,75,94`). | Use the `*_FILE` variants with Docker secrets, and keep Litestream credentials in `litestream.yml`. | M | Secrets leave the process environment. |
| SEC-4 | L | `deploy/docker-compose.yml:4` | `image: ...:${APP_VERSION:-latest}` falls back to a mutable tag when `APP_VERSION` is unset, so an operator who forgets the variable silently runs whatever `latest` points to. | Default to a pinned version or digest and fail when unset. | S | A deploy cannot change code by accident. |
| SEC-5 | L | `internal/httpapi/server.go:346-378` | Same handler: each unauthenticated, unthrottled `/readyz` request creates and removes temp files in `DATA_DIR` and the temp dir and runs two database queries, so anonymous callers can drive disk and DB work. SEC-2 covers only the error text. | Cache the check for a few seconds or gate it behind the throttle. | S | An anonymous health probe cannot load the host. |
| SEC-6 | M | `internal/httpapi/token.go:93`; `internal/httpapi/mailpoll.go:145` | The `mail:ingest` token is embedded in the forwarding address (`belege+<token>@domain`) and read back from the `To:` header of every forwarded mail, so the plaintext token travels through mail servers, logs and mailboxes. API tokens are hashed at rest (SEC-1 cites the asymmetry), and this exposure is not on the accepted-risk list. | Use an opaque address alias that maps to the token server side, or record the acceptance in the SPEC. | M | The ingest credential stops leaking through mail. |

Accepted risks, on record and not findings: the setup token is written to the log on every start
(`internal/httpapi/bootstrap.go:63`, SPEC 10.6); IMAP intake accepts mail with no or neutral
`Authentication-Results` verdict (`internal/httpapi/mail.go:191`) by design; `OIDC_ALLOW_EMAIL_LINKING`
links only on `email_verified` (`internal/httpapi/identity.go:72`).

## Cleanup (2)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| CLN-1 | M | `docs/SPEC.md:1144,1165-1179,1187`; `docs/development.md:31` | The documented CI layout no longer matches `ci.yml`: SPEC describes a `go` job with `govulncheck` instead of `go-lint`/`go-test`/`go-vuln`, omits `toolchain-parity` and `actionlint`, states a different concurrency expression (`ci-${{ github.event_name }}` vs `inputs.caller`), lists other timeouts for ten jobs (image 10 vs 25, e2e 10 vs 25, pdfa 5 vs 20, and so on), and still describes the `image` step as `load: true` although `ci.yml:291-294` extracts with `skopeo`. | Rewrite SPEC 18.1/18.2 and `development.md:31` from the workflow. | S | Contributors reason from a correct pipeline description. |
| CLN-3 | L | `renovate.json5:131` | The `svelte` group still lists `@testing-library/svelte`, removed from `web/package.json` when code-health 2.5 landed (`rg testing-library web/package.json pnpm-lock.yaml` is empty). | Remove the line. | S | No stale dependency configuration. |
