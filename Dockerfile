# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

# ---- SPA: once on the build platform, architecture-independent ----
FROM --platform=$BUILDPLATFORM node:24.21.0-trixie-slim@sha256:173f125896c3b47ddf056734c7ea789d04595a6a08769a8f78e0df642781fb66 AS web
# renovate: datasource=npm depName=pnpm
ARG PNPM_VERSION=12.16.1
ENV PNPM_HOME=/pnpm PATH=/pnpm:$PATH CI=true
RUN npm install -g pnpm@${PNPM_VERSION}
WORKDIR /src
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY web/package.json web/
COPY e2e/package.json e2e/
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store pnpm fetch
COPY web/ web/
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm install --offline --frozen-lockfile --filter web && pnpm --filter web build

# ---- Go: cross-compile without emulation, or take the CI artifact ----
# CI passes --build-context gobin=<dir> holding one prebuilt binary per
# architecture, so the compile happens once in a cached CI job instead of inside
# every build. Without that context this stage compiles here, which keeps
# compose and local builds working from source alone. The binary and the data
# directory are written to /<arch>, which is the layout the context uses.
FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS gobin
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist internal/webui/dist
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    VERSION="$VERSION" COMMIT="$COMMIT" GOOS="$TARGETOS" GOARCH="$TARGETARCH" OUT="/$TARGETARCH" \
    ./scripts/build-binary.sh

# ---- Typst: static binary from the official multi-arch image ----
FROM ghcr.io/typst/typst:0.15.1@sha256:032e292249bcd378480cc7c142cfa324b63ef8aadeb88d7e7230320c4c9c422f AS typst

# ---- Runtime: COPY only, so arm64 needs no emulation ----
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
# COPY expands this in the source path, which is how one prebuilt context serves
# both architectures without an emulated assembly step.
ARG TARGETARCH
COPY --from=typst /bin/typst /usr/local/bin/typst
COPY assets/fonts/ /usr/share/fonts/app/
COPY --from=gobin /${TARGETARCH}/reisekosten /usr/local/bin/reisekosten
COPY --from=gobin --chown=65532:65532 /${TARGETARCH}/data /data
USER 65532:65532
ENV DATA_DIR=/data TYPST_PATH=/usr/local/bin/typst HOME=/tmp
VOLUME ["/data"]
EXPOSE 8080
# The app answers in milliseconds, so a 30s probe only burns a process per
# container per half minute for nothing. 60s with a 30s start period still lets
# scripts/smoke.sh see the container turn healthy well inside its budget.
HEALTHCHECK --interval=60s --timeout=5s --start-period=30s CMD ["/usr/local/bin/reisekosten", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/reisekosten"]
CMD ["serve"]
