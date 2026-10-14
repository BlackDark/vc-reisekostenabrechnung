#!/bin/sh
# Fails when mise.toml no longer matches the toolchain CI installs.
#
# CI is the source of truth. Reading the version out of the file CI actually
# reads (rather than duplicating the numbers here) means this keeps working when
# Renovate bumps only one side: the drift shows up as a red job instead of a
# confusing local failure. The go-lint job runs it, so no toolchain is needed.
set -eu
cd "$(dirname "$0")/.."

mise_val() {
	grep -E "^\"?$1\"?[[:space:]]*=" mise.toml | head -1 |
		sed -e 's/^[^=]*=[[:space:]]*"//' -e 's/"[[:space:]]*$//'
}

CI_FILE=.github/workflows/ci.yml
fail=0

check() {
	name=$1
	want=$2
	got=$3
	if [ -z "$got" ] || [ "$want" != "$got" ]; then
		echo "mise.toml drift: $name is '$got' locally, CI pins '$want'" >&2
		fail=1
	fi
}

# Go: the go directive is what actions/setup-go reads via go-version-file.
check go "$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' go.mod | head -1)" "$(mise_val go)"

# Node: .node-version is what actions/setup-node reads via node-version-file.
check node "$(tr -d ' \t\r\n' < .node-version)" "$(mise_val node)"

# pnpm: package.json's packageManager, written as pnpm@X.Y.Z.
check pnpm \
	"$(sed -n 's/.*packageManager.*pnpm@\([0-9][0-9.]*\).*/\1/p' package.json | head -1)" \
	"$(mise_val pnpm)"

# The remaining tools are pinned only inside the workflow, so read them back out
# of it. Each pattern occurs exactly once in ci.yml.
check golangci-lint \
	"$(grep -oE 'version: v[0-9][0-9.]*' "$CI_FILE" | head -1 | sed 's/version: v//')" \
	"$(mise_val golangci-lint)"
check govulncheck \
	"$(grep -oE 'govulncheck@v[0-9][0-9.]*' "$CI_FILE" | sed 's/govulncheck@v//')" \
	"$(mise_val 'go:golang.org/x/vuln/cmd/govulncheck')"
check typst \
	"$(grep -oE 'typst/typst:[0-9][0-9.]*@' "$CI_FILE" | sed 's/typst\/typst://; s/@$//')" \
	"$(mise_val typst)"
check actionlint \
	"$(grep -oE 'download/v[0-9][0-9.]*/actionlint' "$CI_FILE" | sed 's/download\/v//; s/\/actionlint//')" \
	"$(mise_val actionlint)"

if [ "$fail" -ne 0 ]; then
	echo "Fix mise.toml, or the workflow when CI moves first. See docs/development.md." >&2
	exit 1
fi
echo "mise.toml matches the CI toolchain"
