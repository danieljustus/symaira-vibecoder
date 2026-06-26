# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-06-26

### Added

- **Template export & import** — export cycles as reusable templates (JSON), import with automatic capability check and remap dialog (#53, #54)
- **POST /api/cycle/assist** — AI-guided cycle editing: send a cycle + natural-language instruction, get back a validated, capability-checked cycle (#55)
- **Community template library** — searchable panel in the board to browse, filter, and import shared Baukasten cycles from a configurable index (#56)
- Runtime status separated from cycle definition for export-safe model (#53)

### Fixed

- **Security: go/path-injection** — added `safeCyclePath()` to prevent path traversal in `LoadCycle`/`SaveCycle` (#57)

### Closed Issues

- #47 Trenne Laufzeit-Status von der Cycle-Definition
- #48 Template-Export/Import mit Manifest und automatisch abgeleiteten requires
- #49 Capability-Check beim Template-Import
- #50 Board-UI: Template exportieren/importieren
- #51 POST /api/cycle/assist endpoint
- #52 Community template library
- #57 go/path-injection (security alert)

## [0.2.0] - 2026-06-25

### Added

- **SwiftUI client** — native macOS app with onboarding, cycle board, run controls, demo mode, widget, and intents
- **SymvibeKit Swift package** — reusable Swift library for cycle management
- **Auth middleware** — Bearer token authentication for API endpoints
- **Device registry** — track and manage connected devices with QR pairing
- **TLS certificate generation** — self-signed certs for HTTPS serving
- **Network access modes** — loopback, LAN, and relay options
- **mDNS discovery** — automatic local network service advertisement
- **Anthropic API runner** — alternative LLM backend via Anthropic API
- **`/api/version` endpoint** — report running version to clients
- **Dependency checks** — `symvibe doctor` command for opencode/git/gh availability
- **Community files** — CODE_OF_CONDUCT.md, CONTRIBUTING.md, issue templates, PR template
- **Dependabot** — automated dependency version updates
- **GitHub repository audit** — setup, security, CI/CD documentation

### Changed

- Canonical Apache-2.0 license text
- README improvements with badges and features section
- Test coverage improved from 27.3% to 49.4%
- Bumped Go dependencies (BurntSushi/toml)
- Bumped GitHub Actions (3 updates)

### Fixed

- `@preconcurrency` annotation for UserNotifications import in PushManager.swift
- Server registry test race condition

## [0.1.0] - 2026-06-23

### Added

- Initial release
- Editable cycle board on `127.0.0.1`
- Autonomous phase/step walking via `Runner` interface
- Per-step model bindings and live status over SSE
- TOML cycle persistence
- Model resolver with override > category > default fallback chain
- opencode discovery (skills/agents/models)
- `symvibe serve` command
- `symvibe doctor` command
- Embedded web board via `go:embed`
- SSE event streaming
- REST API for cycle read/edit and run control

[Unreleased]: https://github.com/danieljustus/symaira-vibecoder/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/danieljustus/symaira-vibecoder/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/danieljustus/symaira-vibecoder/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/danieljustus/symaira-vibecoder/releases/tag/v0.1.0
