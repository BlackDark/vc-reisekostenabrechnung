#!/bin/sh
# Builds the server binary for one target platform.
#
# The image build, the CI build job and the release job all call this, so the
# build tags, trimpath and ldflags cannot drift apart. VERSION and COMMIT are
# compiled in: `reisekosten version` and the status endpoint report them. OUT
# receives the binary and the empty data directory that the runtime image copies,
# which is also the layout the prebuilt build context uses.
set -eu
: "${VERSION:?}" "${COMMIT:?}" "${GOOS:?}" "${GOARCH:?}" "${OUT:?}"

cd "$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
mkdir -p "$OUT/data"
# nodynamic keeps gen2brain/avif and webp on wazero. The default build dlopens
# libavif and is not static.
CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
	go build -tags nodynamic -trimpath \
		-ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT" \
		-o "$OUT/reisekosten" ./cmd/reisekosten
