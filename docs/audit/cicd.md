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
| CI-1 | M | `.github/workflows/ci.yml:576-578`; docs at `docs/operations.md:49`, `docs/SPEC.md:1179` | `publish-edge` is skipped on every push to main, so the documented `:edge` channel does not publish. Observed: runs 39015758274 (0604fce), 38967834674 (c9eb601) and 38966067709 (8a5520c) all report `ci-ok` success and `publish-edge` skipped, and none of the last 40 `ci.yml` runs has a successful `publish-edge`. On run 39015758274 the job `pr-titel` (guard `github.event_name == 'pull_request'`) was also skipped, which proves `github.event_name == 'push'` holds, so its `github.ref == 'refs/heads/main'` comparison is the operand that evaluates false although the run heads `main`; the cause needs one instrumented run. Docs still promise "`:edge` is the latest `main` build" (`operations.md:49`) and "Multi-Arch-Push `:edge`" (`SPEC.md:1179`), and the job's tags are `edge` plus `sha-<short>` (`ci.yml:600-603`). | Capture the evaluation in a real run, make the guard provably match (drop the ref test; `on.push.branches` already limits it), assert an `edge` tag after publish, then correct the docs if the channel is meant to exist. | S | Restores a documented distribution channel that never published. |
| CI-2 | L | `.github/workflows/release.yml:96,108,126` | The release `publish` job runs `docker/build-push-action` three times: multi-arch OCI archive (08:09:19-08:11:22), an amd64-only `load` build for Trivy (08:11:22-08:11:30), and the final multi-arch push (08:11:38-08:12:11). The amd64 load build repeats work the OCI archive already contains. | Extract amd64 from the OCI archive with `skopeo copy` (same pattern as `ci.yml:291-294`) for the scan, and delete the second build. | S | One buildx invocation less per release. |
| CI-3 | L | `.github/workflows/ci.yml:506-521` | `actionlint` has no path filter, so it runs on every pull request including docs-only ones, and it also runs `go test ./scripts/workflowperm ./scripts/ghrepo`, which is unrelated to linting workflows. Evidence: docs-only run 38967689085 ran `actionlint` (19s) while every filtered job was skipped. | Filter it to `.github/workflows/**` and the two script packages, or move the Go tests to a filtered job. | S | Less runner time on non-workflow changes; one job, one purpose. |
| CI-4 | L | `.github/workflows/release.yml:22-34` | The reusable `ci` call passes `secrets: inherit`, so the release pipeline receives every repository secret although this workflow only uses the automatic `GITHUB_TOKEN`. | Drop `secrets: inherit`. | S | Least privilege for the release pipeline. |
| CI-5 | L | `Dockerfile:1` | `# syntax=docker/dockerfile:1` is a floating frontend tag while every base image and action is digest-pinned, so the build is not reproducible. | Pin the frontend by digest and let Renovate keep it current. | S | Reproducible builds, consistent with the rest of the tree. |
| CI-6 | L | `.github/workflows/ci.yml:247-306`, `:576-620` | Every push to main builds the multi-platform image twice: `image` builds amd64+arm64 without pushing, and `publish-edge` rebuilds the same two platforms with push. Currently masked by CI-1, which skips `publish-edge`. | After CI-1, let the `image` job push on main, or reuse its OCI archive, instead of a second buildx run. | M | One fewer full multi-arch build per merge. |
| CI-7 | L | `.github/workflows/release.yml:1-16` | No `concurrency` group, so a tag push and a manual `workflow_dispatch` for the same tag can run at once and race on tags and signing. | Add a group keyed on `env.RELEASE_TAG` with `cancel-in-progress: false`. | S | No duplicate or racing publishes. |

## Unverified

- Required checks per branch protection: `gh api .../branches/main/protection` and `/actions/permissions`
  return 403 for this token.
- The reason CI-1's guard evaluates false: a skipped job emits no log, and the condition log was not
  reachable through the API.
- `actionlint` could not run its shellcheck and pyflakes passes in this environment (mise shims unset);
  `actionlint -shellcheck= -pyflakes=` passed on all four workflows. `yamllint` fails to start
  (`ModuleNotFoundError`).
- Base-image CVEs: no trivy or osv-scanner available, so images were not scanned.
