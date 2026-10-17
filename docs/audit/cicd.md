# Audit: CI/CD and deployment, 2026-10-17 (main 0604fce)

Inventory: `ci.yml` 19 jobs, `codeql.yml` 2 analyze jobs, `release.yml` (reusable `ci` call, `publish`,
notes), `release-please.yml`. Durations from run 39015758274 (main push, 0604fce, 10m04s total):
changes 5s, actionlint 15s, toolchain-parity 4s, go-lint 65s, go-test 162s, go-vuln 32s, codegen 71s,
web 40s, image 470s, smoke 21s, e2e desktop 93s / mobile 96s / mobile-webkit 116s, pdfa 102s,
scan 42s, backup 23s, ci-ok 4s; publish-edge skipped (CI-1). Release `publish` about 3m50s
(run 39015778960). Every action is pinned by commit SHA and every image by digest, except the
Dockerfile frontend (CI-5). Branch protection is unreadable with the available token (API 403), so
which jobs are required is unverified; `docs/SPEC.md:1178` names `ci-ok` as the required check.

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| CI-6 | L | `.github/workflows/ci.yml:247-306`, `:576-620` | Every push to main builds the multi-platform image twice: `image` builds amd64+arm64 without pushing, and `publish-edge` rebuilds the same two platforms with push. Currently masked by CI-1, which skips `publish-edge`. | After CI-1, let the `image` job push on main, or reuse its OCI archive, instead of a second buildx run. | M | One fewer full multi-arch build per merge. |

## Unverified

- Required checks per branch protection: `gh api .../branches/main/protection` and `/actions/permissions`
  return 403 for this token.
- The reason CI-1's guard evaluates false: a skipped job emits no log, and the condition log was not
  reachable through the API.
- `actionlint` could not run its shellcheck and pyflakes passes in this environment (mise shims unset);
  `actionlint -shellcheck= -pyflakes=` passed on all four workflows. `yamllint` fails to start
  (`ModuleNotFoundError`).
- Base-image CVEs: no trivy or osv-scanner available, so images were not scanned.
