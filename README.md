# Stealth

A high-performance Go alternative to FlareSolverr with native header injection, JSON payload support, and CDP-based Turnstile solving.

## Features

- **Clean REST API** — no legacy `cmd` field routing
- **Native Header Injection** — custom headers passed directly via `fetch()` injection
- **JSON Payload Support** — clean JSON responses, no HTML wrapping
- **CDP Turnstile Solving** — coordinate-based mouse click, not fragile Tab+Space
- **Proxy Auth via CDP** — works in headless mode (no Chrome extension hack)
- **Session Management** — in-memory sessions with optional TTL auto-expiry
- **Graceful Shutdown** — SIGTERM closes all Chrome processes cleanly

## Quick Start

```bash
# Run directly
go run ./cmd/stealth

# Or with Docker
docker compose up
```

The service starts on `http://localhost:8191`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `HOST` | `0.0.0.0` | Bind address |
| `PORT` | `8191` | HTTP port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `HEADLESS` | `true` | Run Chrome headless (`false` for visible window) |
| `PROMETHEUS_ENABLED` | `false` | Enable Prometheus metrics |
| `PROMETHEUS_PORT` | `8192` | Prometheus port |

## API

### `GET /`
Returns service info.

### `GET /health`
Returns `{"status":"ok"}`. Used by Docker health checks.

### `POST /v2/request`

Solve a Cloudflare challenge and execute an HTTP request.

**Request:**
```json
{
  "url": "https://example.com/api/data",
  "method": "GET",
  "maxTimeout": 60000,
  "session": "optional-session-id",
  "headers": { "X-Custom": "value" },
  "postData": "{\"key\":\"val\"}",
  "proxy": { "url": "http://ip:port", "username": "u", "password": "p" },
  "userAgent": "Mozilla/5.0...",
  "cookies": [{ "name": "x", "value": "y", "domain": "example.com" }],
  "returnOnlyCookies": false,
  "returnScreenshot": false,
  "disableMedia": true,
  "waitAfterMs": 0
}
```

**Response:**
```json
{
  "status": "ok",
  "message": "Challenge solved!",
  "startTimestamp": 1718000000000,
  "endTimestamp": 1718000005200,
  "version": "1.0.0",
  "solution": {
    "url": "https://example.com/api/data",
    "status": 200,
    "response": "{\"key\":\"val\"}",
    "cookies": [{ "name": "cf_clearance", "value": "...", "domain": ".example.com" }],
    "userAgent": "Mozilla/5.0..."
  }
}
```

### `POST /v2/sessions`
```json
{ "session": "my-id", "ttl": 300, "proxy": { "url": "http://ip:port" } }
```

### `GET /v2/sessions`
Returns all active session IDs.

### `DELETE /v2/sessions/:id`
Destroys a session and closes its browser.

## Architecture

```
Request → Fiber Server → Session Manager → Solver Engine
                                               ↓
                                    Phase 1: Navigate base domain
                                    → Detect Cloudflare challenge
                                    → Solve Turnstile (CDP coordinates)
                                    → Wait for selectors to clear
                                               ↓
                                    Phase 2: Inject fetch()
                                    → Execute with client headers/body
                                    → Return clean JSON response
```
