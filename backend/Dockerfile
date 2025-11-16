# syntax=docker/dockerfile:1.6

# ---- Builder ----
FROM golang:1.23-alpine AS builder
WORKDIR /src

# Install build deps (git already included)
RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/rhinobox ./cmd/rhinobox

# ---- Runtime ----
FROM alpine:3.20
WORKDIR /app

# Create non-root user and data directory
RUN adduser -D -u 10001 rhinobox && \
    mkdir -p /data && \
    chown rhinobox:rhinobox /data

ENV RHINOBOX_ADDR=:8090 \
    RHINOBOX_DATA_DIR=/data \
    RHINOBOX_MAX_UPLOAD_MB=512

COPY --chown=rhinobox:rhinobox --from=builder /out/rhinobox /app/rhinobox

USER rhinobox
VOLUME ["/data"]
EXPOSE 8090

ENTRYPOINT ["/app/rhinobox"]
