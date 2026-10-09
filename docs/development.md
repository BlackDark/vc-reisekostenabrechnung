# Development

Go 1.27, Node 24, pnpm 12, Svelte 5, SQLite. Lint with golangci-lint and Biome.

```sh
make check
```

`make check` runs `go test ./...` and the web checks (Biome, svelte-check, Vitest, the bundle budget). `make generate` refreshes sqlc, the OpenAPI server, and the TypeScript client.

Commits and pull request titles follow [Conventional Commits](https://www.conventionalcommits.org/). release-please opens the release pull request from those titles. Do not tag a release by hand. Merging the release-please pull request creates the tag; `release.yml` then builds, scans, signs, and publishes the image.

## End-to-end tests

Playwright runs against the built image. CI starts it with `scripts/e2e-up.sh` and the Playwright container. Projects are desktop (1280×800) and mobile (Pixel 7). On `main` and on release runs, `E2E_WEBKIT=1` adds an iPhone WebKit project.

`e2e/tests/seiten.spec.ts` opens every route in `web/src/router.ts`, fails on `console.error` or `pageerror`, and checks that a phone-width page does not scroll sideways. Screenshots land in `e2e/screenshots/<viewport>/`. CI uploads them as the `screenshots` artifact, including a Fast 4G LCP sample for the login page (`lcp-fast4g.txt`). The 2 second LCP figure in the specification is a budget, not a failing check.

Copy a local run into the README gallery:

```sh
pnpm screenshots:readme
```

That copies desktop and mobile PNGs to `docs/assets/screenshots/` and runs `oxipng` when it is on `PATH`.

## Other checks

`govulncheck` runs in the `go` job. Trivy scans the image in the `scan` job (fixable CRITICAL and HIGH fail the job; the SARIF upload is visible in code scanning). CodeQL is informational. `pdfa` checks the sample exports with veraPDF and rejects a file that is not PDF/A-3b.
