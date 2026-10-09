# syntax=docker/dockerfile:1

# ---- SPA: once on the build platform, architecture-independent ----
FROM --platform=$BUILDPLATFORM node:24.21.0-trixie-slim@sha256:173f125896c3b47ddf056734c7ea789d04595a6a08769a8f78e0df642781fb66 AS web
# renovate: datasource=npm depName=pnpm
ARG PNPM_VERSION=12.10.1
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

# ---- Go: cross-compile without emulation ----
FROM --platform=$BUILDPLATFORM golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS build
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist internal/webui/dist
# nodynamic keeps gen2brain/avif and webp on wazero. The default build dlopens libavif and is not static.
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -tags nodynamic -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/reisekosten ./cmd/reisekosten \
 && mkdir -p /out/data

# ---- Typst: static binary from the official multi-arch image ----
FROM ghcr.io/typst/typst:0.15.1@sha256:032e292249bcd378480cc7c142cfa324b63ef8aadeb88d7e7230320c4c9c422f AS typst

# ---- Runtime: COPY only, so arm64 needs no emulation ----
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=typst /bin/typst /usr/local/bin/typst
COPY assets/fonts/ /usr/share/fonts/app/
COPY --from=build /out/reisekosten /usr/local/bin/reisekosten
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV DATA_DIR=/data TYPST_PATH=/usr/local/bin/typst HOME=/tmp
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s CMD ["/usr/local/bin/reisekosten", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/reisekosten"]
CMD ["serve"]
