# Codebase audit, 2026-10-17

Baseline: main 0604fce (v1.1.0). Method: the codebase-audit checklist, three read-only subagents
(code + comments, CI/CD + cleanup, dependencies + security), then one skeptical review pass.
Related: `docs/ideas/code-health.md` holds the earlier pass (ids 1.x, 2.x, 3.x, landed or decided
against). Those items are not repeated here.

Grade: B. The tree is small for what it does, pinned end to end, and documented more than most. The
real problems are one dead distribution channel (CI-1), duplicated security and error-mapping paths in
`internal/httpapi` (CODE-1, CODE-2), and no license file for a product meant to be self-hosted (DEP-1).

Numbers: 652 tracked files, 53,985 Go lines (106 non-test + 44 test), 17,694 web lines, 19 CI jobs,
0 reachable known CVEs, 7 findings: 0 high, 2 medium, 5 low. By area: code 0, comments 0, CI/CD 1,
dependencies 2, security 2, cleanup 2.

Top 5 by benefit / effort:

(none left)
(none left)
(none left)
(none left)
Tool coverage: ripgrep, govulncheck 1.8.0, `go vet`, `go test`, `go mod tidy -diff`, `go mod verify`,
shellcheck, actionlint, `pnpm audit`, `pnpm outdated`, docker. Not available, so unverified: gitleaks,
trivy, osv-scanner, semgrep, hadolint, tokei/cloc, license tooling; `yamllint` fails to start.
Worktree leftovers (`out/`, `e2e/test-results/`, `e2e/playwright-report/`, `node_modules/`) are
git-ignored and untracked, so they are not committed clutter.

Areas: code 0 findings, CI/CD 1 -> `docs/audit/code.md`, `docs/audit/cicd.md`. The rest below.

review: kept 34, dropped 3 (COM-5, CLN-4, CLN-5), merged 2 (COM-2 into CODE-3, CLN-2 into DEP-1),
downgraded 0; 2 findings added from the review (SEC-5, SEC-6).
Dropped: COM-5 wording nit, CLN-4 speculative (the tool header ties it to an open ADR 0005),
CLN-5 the research docs reference those files.

## Comments (0)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |

## Dependencies (2)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| DEP-1 | M | repository root; `docs/SPEC.md:1024` | No license file is tracked (`git ls-files \| rg -i license` is empty) although the SPEC lists `LICENSE` in the layout and the product is published for self-hosting. Without one, users get no grant and no terms. | Add the intended license and its SPDX identifier; the SPEC layout entry then matches (absorbs CLN-2). | S | Legal clarity for a self-hosted product. |
| DEP-4 | L | `go.mod:34,56`; accepted in `docs/SPEC.md:1261` | Codegen tooling sits in the main module graph, which pulls in a second JWT major (`golang-jwt/jwt/v4 v4.5.2` beside `jwt/v5`), ClickHouse/YDB/Vertica/MSSQL/Azure drivers and the deprecated `github.com/golang/protobuf v1.5.4`. It inflates `go.sum` and hides DEP-2 among tooling noise. | Move the tools to their own module. | M | Smaller graph, quieter vulnerability scans. |

## Security (2)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| SEC-3 | L | `deploy/docker-compose.yml:12`, `deploy/docker-compose.backup.yml:11-15` | Secrets arrive as container environment (`env_file: .env` for `S3_SECRET_ACCESS_KEY`, `OIDC_CLIENT_SECRET`, `INITIAL_ADMIN_PASSWORD`, `MAIL_IMAP_PASSWORD`; Litestream credentials via `environment:`), so `docker inspect` and `/proc/<pid>/environ` expose them. The app already supports `*_FILE` variants (`deploy/.env.example:22,33,59,75,94`). | Use the `*_FILE` variants with Docker secrets, and keep Litestream credentials in `litestream.yml`. | M | Secrets leave the process environment. |
| SEC-4 | L | `deploy/docker-compose.yml:4` | `image: ...:${APP_VERSION:-latest}` falls back to a mutable tag when `APP_VERSION` is unset, so an operator who forgets the variable silently runs whatever `latest` points to. | Default to a pinned version or digest and fail when unset. | S | A deploy cannot change code by accident. |

Accepted risks, on record and not findings: the setup token is written to the log on every start
(`internal/httpapi/bootstrap.go:63`, SPEC 10.6); IMAP intake accepts mail with no or neutral
`Authentication-Results` verdict (`internal/httpapi/mail.go:191`) by design; `OIDC_ALLOW_EMAIL_LINKING`
links only on `email_verified` (`internal/httpapi/identity.go:72`).

## Cleanup (2)

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| CLN-1 | M | `docs/SPEC.md:1144,1165-1179,1187`; `docs/development.md:31` | The documented CI layout no longer matches `ci.yml`: SPEC describes a `go` job with `govulncheck` instead of `go-lint`/`go-test`/`go-vuln`, omits `toolchain-parity` and `actionlint`, states a different concurrency expression (`ci-${{ github.event_name }}` vs `inputs.caller`), lists other timeouts for ten jobs (image 10 vs 25, e2e 10 vs 25, pdfa 5 vs 20, and so on), and still describes the `image` step as `load: true` although `ci.yml:291-294` extracts with `skopeo`. | Rewrite SPEC 18.1/18.2 and `development.md:31` from the workflow. | S | Contributors reason from a correct pipeline description. |
| CLN-3 | L | `renovate.json5:131` | The `svelte` group still lists `@testing-library/svelte`, removed from `web/package.json` when code-health 2.5 landed (`rg testing-library web/package.json pnpm-lock.yaml` is empty). | Remove the line. | S | No stale dependency configuration. |
