# syntax=docker/dockerfile:1
# Image `kiosk`: Cage (Wayland kiosk compositor) + Chromium, supervised by the
# Go agent kioskd. Build context is the repository root.
# kioskd is cross-compiled on the build platform; the final stage has RUN steps
# (apt, useradd), so an arm64 build on an amd64 runner needs QEMU.

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS kioskd
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/kioskd ./cmd/kioskd

FROM debian:trixie-slim
LABEL org.opencontainers.image.title="kiosk" \
      org.opencontainers.image.source="https://github.com/Thazzr/MirrorNextGen"
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      cage chromium wlr-randr seatd libgl1-mesa-dri \
      fonts-dejavu-core fonts-noto-color-emoji ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 1000 kiosk \
 && useradd --uid 1000 --gid kiosk --create-home --shell /usr/sbin/nologin kiosk \
 && for g in video render; do \
      if getent group "$g" >/dev/null; then usermod -aG "$g" kiosk; fi; \
    done
COPY --from=kioskd /out/kioskd /usr/local/bin/kioskd
ENTRYPOINT ["/usr/local/bin/kioskd"]
