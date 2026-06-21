# --- Build Stage ---
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o stealth ./cmd/stealth

# --- Runtime Stage ---
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        chromium \
        dumb-init \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user to run the service
RUN useradd --home-dir /app --shell /bin/sh stealth
WORKDIR /app

COPY --from=builder /build/stealth .
RUN chown -R stealth:stealth /app

USER stealth

# Chromium writes crash reports here — create it as the stealth user
RUN mkdir -p "/app/.config/chromium/Crash Reports/pending"

EXPOSE 8191

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8191/health || exit 1

# dumb-init prevents zombie Chromium processes after the main process exits
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/app/stealth"]
