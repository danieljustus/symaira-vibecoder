# Agent Instructions — symvibe (symaira-vibecoder)

A graphical "Vibe Coding" Baukasten. A small, standalone Go binary serves an
editable cycle board on `127.0.0.1` and **drives opencode** (never forks it)
through a swappable `Runner` interface to walk the cycle's phases/steps
autonomously, with per-step model bindings and live status over SSE.

- **License:** Apache-2.0 (public core, like the other Symaira cores).
- **Standalone-first:** the board runs with no opencode installed (read-only,
  Run disabled). opencode is a *runtime peer*, detected on PATH — never a
  compile-time dependency on opencode internals.

## Build & Test

```bash
make build        # CGO_ENABLED=0 single binary, embeds web/dist + the seed cycle
make test         # unit tests
make test-race    # race detector (engine concurrency)
make lint         # gofmt + go vet
make run          # build + symvibe serve (opens the board)
./symvibe doctor  # check opencode/git/gh availability and config sanity
```

Go 1.26.x, **CGO-free** (`CGO_ENABLED=0`); direct deps are `BurntSushi/toml`,
`spf13/cobra`, and `hashicorp/mdns` (LAN discovery). The web board is embedded via `go:embed all:dist`; the committed
`web/dist/index.html` is a dependency-free board, so `go build` never needs Node.

## Architecture & Key Competencies

```
browser (embedded board)  ──REST + SSE──▶  internal/server (net/http + SSE + embed)
                                               │ Engine API
                                          internal/engine  (autonomous scheduler, sensors, status FSM, bus)
                                               │ config.Resolver        │ runner.Runner (swappable)
                                          internal/config               internal/runner ──exec──▶ opencode
                                          (Cycle/Phase/Step, model        (OpenCodeRunner:
                                           Resolver, TOML persist,         `opencode run --format json`)
                                           discovery)
```

- **internal/config** — the single source of truth for the Baukasten
  (`Cycle/Phase/Step`, statuses, `NextRunnable`), TOML persistence under the data
  dir, the model `Resolver` (override > category > default + fallback chain), and
  opencode discovery (skills/agents/models).
- **internal/engine** — walks `NextRunnable`, evaluates `AutoSkip` sensors,
  enforces the status transition table, drives the runner, and publishes events.
  Owns `pending→in_progress→done/failed`, so status is correct regardless of how
  the backend streams.
- **internal/runner** — `Runner` interface + `OpenCodeRunner`. Does **not** pass
  `--pure` (it disables zen auth); passes `--dangerously-skip-permissions` when
  `runner.skip_permissions` so unattended runs don't block.
- **internal/server** — REST for cycle read/edit + run control, one SSE stream at
  `/events`, embedded board with SPA fallback. Binds loopback only.

## Conventions (see ../ECOSYSTEM.md)

- Config: `~/.config/symvibe/config.toml` (TOML). Cycles: `~/.local/share/symvibe/cycles/<id>.toml`.
- Env prefix `SYMVIBE_*`. Exit codes: 0 ok, 1 error, 2 usage/config.
- Stdio hygiene: logs go to **stderr** via `log/slog`; stdout is for user output.
- Strict SemVer; binary is `symvibe`.

## Key Dependencies

- `github.com/spf13/cobra` — CLI.
- `github.com/BurntSushi/toml` — config + cycle persistence.
- `github.com/hashicorp/mdns` — LAN service discovery/advertisement.
- **opencode** (runtime peer, not imported) — `opencode run --format json`,
  `--agent/--model/--variant`, skills under `~/.config/opencode/skills`.

## symaira-appkit (Welle 4, Teiladoption)

- The macOS app target (`client/`, target `Symvibe`) consumes
  **symaira-appkit** pinned exact (`0.2.0`, `client/project.yml`):
  SymairaToolKit for binary discovery in `SymvibeApp/EngineManager.swift`.
  The engine supervision itself stays local (DaemonKit v0.2 candidate).
- **iOS boundary:** symaira-appkit declares macOS only. Do NOT add appkit
  products to the iOS targets or to SymvibeKit (iOS 17 + macOS 14) — the
  local `KeychainHelper` in SymvibeKit stays until appkit ships iOS support.
- **App target builds green (fixed 2026-07-28):** the former pre-existing
  breakage in `Views/BoardView.swift` / `Views/PairingQRView.swift` /
  `Views/StepEditorView.swift` (committed unbuilt in PR #24) is repaired:
  cross-platform `Color.cardSeparator`/`Color.cardBackground` helpers replace
  the ambiguous `Color(.separator)`/`Color(.background)` (resolved to SwiftUI
  shape styles, not system colors), `NSSize(width:y:)` became
  `NSSize(width:height:)`, and `.navigationBarTitleDisplayMode` is iOS-only
  now. SymvibeKit models (`Cycle`, `Phase`, `Step`, `StepModelOverride`,
  `AutoSkip`) got explicit `public init`s so the app target can construct
  them across the module boundary. `PairingPayload.parse` was fixed for
  custom-scheme URLs (`url.host == "pair"`, not `url.path`) plus
  form-encoding (`+` → space) and gained `baseURL(for:)`.
  `Sources/SymvibeApp/Info.plist` now contains the full CFBundle key set as
  build variables — before, the processed bundle plist lacked
  CFBundleExecutable/-Identifier/-Version. `swift test`: 44/44 green.
- **Local build:** `make build` (embeds `symvibe` into the app), then
  `cd client && xcodegen generate` and
  `DEVELOPER_DIR=/Applications/Xcode-beta.app/Contents/Developer xcodebuild -project Symvibe.xcodeproj -scheme Symvibe -configuration Release -derivedDataPath build`.
  The CommandLineTools-only toolchain cannot run XCTest; use the same
  `DEVELOPER_DIR` for `swift test`. Result: adhoc-signed
  `build/Build/Products/Release/Symvibe.app` (installed at
  `/Applications/Symvibe.app`).
- SSE, TLS pinning, push, and the widget are untouched by design.
