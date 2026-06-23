# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-06-23

### Added
- Hybrid Request-Browser Architecture with cookie cache and Chrome TLS spoofing fallback.
- Initial release of Stealth.
- High-performance Go-based solver engine using `go-rod`.
- `POST /v2/request` endpoint for full request lifecycle management.
- Two-phase challenge clearance and native `fetch()` payload injection.
- CDP coordinate-based Turnstile solving mechanism.
- Headless proxy authentication support via CDP Fetch domain interception.
- In-memory session manager with TTL-based reaper (`/v2/sessions`).
- Graceful shutdown handling via SIGINT/SIGTERM.
- Dockerfile and `docker-compose.yml` for containerized deployments.
- GitHub Actions CI workflow for testing and formatting.
- GitHub Actions Docker workflow for automated container builds to GHCR.
- Comprehensive `README.md` with architecture details and API references.

### Changed
- Extracted `ExtractBaseURL` to the models package.

### Fixed
- Resolved critical and high bugs from codebase review.
