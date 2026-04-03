# syntax=docker/dockerfile:1
# Multi-platform build support for linux/amd64 and linux/arm64
FROM --platform=${BUILDPLATFORM} golang:1.23-alpine AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=dev" \
    -o /out/proxyd ./cmd/proxyd

# Runtime image - use compatible base
FROM alpine:3.21
RUN adduser -D -H -u 10001 proxy
WORKDIR /app

COPY --from=builder /out/proxyd /usr/local/bin/proxyd
RUN mkdir -p /data && chown -R proxy:proxy /data

USER proxy

ENV PROXY_DB_PATH=/data/proxy-center.db
ENV PROXY_HTTP_LISTEN=:8080
ENV PROXY_SOCKS_LISTEN=:1080
ENV PROXY_WEB_LISTEN=:8090
ENV PROXY_ADMIN_USER=admin
ENV PROXY_ADMIN_PASS=change-me-now
ENV PROXY_EGRESS_MODE=direct
ENV PROXY_EGRESS_POOL=
ENV PROXY_HEALTH_TICK=10s
ENV PROXY_LOG_DOMAINS=true

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O - http://127.0.0.1:8090/healthz >/dev/null 2>&1 || exit 1

EXPOSE 8080 1080 8090
ENTRYPOINT ["proxyd"]
