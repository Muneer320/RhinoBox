# syntax=docker/dockerfile:1.6

# ---- Builder ----
FROM golang:1.27-alpine AS builder
WORKDIR /src

# Certificates for module downloads and TLS database connections.
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download || go mod download || go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOMAXPROCS=2 go build -p 2 -ldflags="-s -w" -o /out/rhinobox ./cmd/rhinobox

# ---- Runtime ----
FROM alpine:3.23
WORKDIR /app

# Create non-root user and data directory
RUN adduser -D -u 10001 rhinobox && \
    mkdir -p /data && \
    chown rhinobox:rhinobox /data

ENV RHINOBOX_ADDR=:8090 \
    RHINOBOX_DATA_DIR=/data \
    RHINOBOX_MAX_UPLOAD_MB=512

COPY --chown=rhinobox:rhinobox --from=builder /out/rhinobox /app/rhinobox
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

USER rhinobox
VOLUME ["/data"]
EXPOSE 8090

ENTRYPOINT ["/app/rhinobox"]
