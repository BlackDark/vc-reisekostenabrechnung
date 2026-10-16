#!/bin/sh
# Fails when a tool version is pinned in two places that disagree.
#
# CI is the source of truth. Reading the version out of the file CI actually
# reads (rather than duplicating the numbers here) means this keeps working when
# Renovate bumps only one side: the drift shows up as a red job instead of a
# confusing local failure. The toolchain-parity job runs it, so no toolchain is
# needed.
set -eu
cd "$(dirname "$0")/.."

CI_FILE=.github/workflows/ci.yml
DOCKERFILE=Dockerfile
E2E_PACKAGE=e2e/package.json
fail=0

mise_val() {
	grep -E "^\"?$1\"?[[:space:]]*=" mise.toml | head -1 |
		sed -e 's/^[^=]*=[[:space:]]*"//' -e 's/"[[:space:]]*$//'
}

# pin joins the matches of one pattern into a single value, so a pin that a file
# states more than once is also compared with itself.
pin() {
	sort -u | tr '\n' ' ' | sed 's/ $//'
}

check() {
	name=$1
	want=$2
	got=$3
	if [ -z "$got" ] || [ "$want" != "$got" ]; then
		echo "mise.toml drift: $name is '$got' locally, CI pins '$want'" >&2
		fail=1
	fi
}

# agree holds two files that pin the same tool. A pattern that stops matching
# fails too, because that would leave the tool pinned in one of them only.
agree() {
	name=$1
	want=$2
	got=$3
	if [ -z "$want" ] && [ -z "$got" ]; then
		echo "toolchain drift: $name is pinned nowhere this script reads" >&2
		fail=1
	elif [ "$want" != "$got" ]; then
		echo "toolchain drift: $name is '$want' in one place and '$got' in another" >&2
		fail=1
	fi
}

# Go: the go directive is what actions/setup-go reads via go-version-file.
check go "$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' go.mod | head -1)" "$(mise_val go)"

# Node: .node-version is what actions/setup-node reads via node-version-file.
check node "$(tr -d ' \t\r\n' < .node-version)" "$(mise_val node)"

# pnpm: package.json's packageManager, written as pnpm@X.Y.Z, and the same
# version in the setup steps, in the container that runs the end-to-end tests,
# and in the image build.
pnpm=$(sed -n 's/.*packageManager.*pnpm@\([0-9][0-9.]*\).*/\1/p' package.json | head -1)
check pnpm "$pnpm" "$(mise_val pnpm)"
agree 'pnpm setup steps' "$pnpm" \
	"$(grep -A2 'pnpm/action-setup' "$CI_FILE" | sed -n 's/^ *version: \([0-9][0-9.]*\)$/\1/p' | pin)"
agree 'pnpm end-to-end container' "$pnpm" \
	"$(grep -oE 'npm install -g pnpm@[0-9][0-9.]*' "$CI_FILE" | sed 's/.*@//' | pin)"
agree 'pnpm image build' "$pnpm" \
	"$(sed -n 's/^ARG PNPM_VERSION=\([0-9][0-9.]*\).*/\1/p' "$DOCKERFILE" | pin)"

# The remaining tools are pinned inside the workflow, so read them back out of
# it. golangci-lint and actionlint use the version the action or the tarball
# carries; govulncheck, Typst and Playwright are pinned more than once.
check golangci-lint \
	"$(grep -oE 'version: v[0-9][0-9.]*' "$CI_FILE" | head -1 | sed 's/version: v//')" \
	"$(mise_val golangci-lint)"
govulncheck=$(grep -oE 'govulncheck@v[0-9][0-9.]*' "$CI_FILE" | sed 's/govulncheck@v//')
check govulncheck "$govulncheck" "$(mise_val 'go:golang.org/x/vuln/cmd/govulncheck')"
agree 'govulncheck cache key' "$govulncheck" \
	"$(sed -n 's/.*key: govulncheck-\([0-9][0-9.]*\)-.*/\1/p' "$CI_FILE" | pin)"
typst=$(grep -oE 'typst/typst:[0-9][0-9.]*@' "$CI_FILE" | sed 's/typst\/typst://; s/@$//')
check typst "$typst" "$(mise_val typst)"
agree 'typst image build' "$typst" \
	"$(sed -n 's|^FROM ghcr.io/typst/typst:\([0-9][0-9.]*\)@.*|\1|p' "$DOCKERFILE" | pin)"
check actionlint \
	"$(grep -oE 'download/v[0-9][0-9.]*/actionlint' "$CI_FILE" | sed 's/download\/v//; s/\/actionlint//')" \
	"$(mise_val actionlint)"

# Playwright is not a tool mise installs: the end-to-end workspace pins it in its
# own package.json, and the container the tests run in has to carry that version.
agree playwright \
	"$(grep -oE 'playwright:v[0-9][0-9.]*' "$CI_FILE" | sed 's/.*:v//')" \
	"$(sed -n 's/.*"@playwright\/test":[[:space:]]*"\([0-9][0-9.]*\)".*/\1/p' "$E2E_PACKAGE" | pin)"

if [ "$fail" -ne 0 ]; then
	echo "Fix the pins, or mise.toml when CI moves first. See docs/development.md." >&2
	exit 1
fi
echo "every tool version pin agrees"
