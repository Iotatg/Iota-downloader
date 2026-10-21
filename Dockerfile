# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Iota Coder Downloader Bot
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/iota-downloader .

# ---------------------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata ffmpeg python3 py3-pip curl \
    && pip3 install --break-system-packages --no-cache-dir -U yt-dlp \
    && rm -rf /root/.cache/pip

RUN addgroup -S iota && adduser -S -G iota iota \
    && mkdir -p /app/downloads /app/logs /app/session \
    && chown -R iota:iota /app

WORKDIR /app

COPY --from=builder /out/iota-downloader /app/iota-downloader

USER iota

ENV OUTPUT_DIR=/app/downloads \
    LOG_DIR=/app/logs \
    MTPROTO_SESSION=/app/session/iota \
    PATH="/usr/local/bin:${PATH}"

ENTRYPOINT ["/app/iota-downloader"]
