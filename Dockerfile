# syntax=docker/dockerfile:1
# Image `mirror`: Go binary with the embedded React app on distroless.
# Frontend and binary are built on the build platform (cross-compiled, no
# cgo), so the final stage only copies files and an arm64 build needs no QEMU.

FROM --platform=$BUILDPLATFORM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY api/ /src/api/
COPY web/ ./
# vite.config.ts writes to ../server/internal/web/dist, i.e. /src/server/internal/web/dist.
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS server
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/server/internal/web/dist/ ./internal/web/dist/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildTime=$BUILD_TIME" \
      -o /out/mirror ./cmd/mirror \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.title="mirror" \
      org.opencontainers.image.source="https://github.com/Thazzr/MirrorNextGen"
COPY --from=server /out/mirror /mirror
COPY --from=server --chown=65532:65532 /out/data /data
ENV MIRROR_ADDR=:8080 \
    MIRROR_DATA_DIR=/data
EXPOSE 8080
VOLUME /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/mirror", "healthcheck"]
USER nonroot
ENTRYPOINT ["/mirror"]
