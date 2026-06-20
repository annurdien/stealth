# Stealth

A Go-based alternative to FlareSolverr for Cloudflare challenge resolution.

## Features

- **REST API**: Standard HTTP methods and JSON routing.
- **Header Injection**: Custom headers passed via `fetch()`.
- **JSON Payload Support**: Returns JSON responses.
- **CDP Turnstile Solving**: Uses coordinate-based mouse clicks for CAPTCHAs.
- **Proxy Auth via CDP**: Supports headless mode proxy authentication.
- **Session Management**: In-memory sessions with TTL auto-expiry.
- **Graceful Shutdown**: SIGTERM handling for Chrome processes.

## Quick Start

```bash
# Run locally
go run ./cmd/stealth

# Run with Docker
docker compose up
```

Service starts on `http://localhost:8191`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `HOST` | `0.0.0.0` | Bind address |
| `PORT` | `8191` | HTTP port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `HEADLESS` | `true` | Run Chrome headless (`false` for visible window) |
| `PROMETHEUS_ENABLED` | `false` | Enable Prometheus metrics |
| `PROMETHEUS_PORT` | `8192` | Prometheus port |

## API Reference

### `GET /`
Service information.

### `GET /health`
Health check endpoint.

### `POST /v2/request`

Solves challenges and executes HTTP requests.

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
Create a session.
```json
{ "session": "my-id", "ttl": 300, "proxy": { "url": "http://ip:port" } }
```

### `GET /v2/sessions`
List active session IDs.

### `DELETE /v2/sessions/:id`
Destroy a session and close its browser instance.

## Architecture Flow

1. **Request** → Fiber Server → Session Manager → Solver Engine
2. **Phase 1**: Navigate base domain → Detect challenge → Solve Turnstile → Wait for selectors to clear
3. **Phase 2**: Inject `fetch()` → Execute with headers/body → Return JSON response
