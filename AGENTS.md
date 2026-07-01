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

Go 1.26.x, **CGO-free** (`CGO_ENABLED=0`); only deps are `BurntSushi/toml` and
`spf13/cobra`. The web board is embedded via `go:embed all:dist`; the committed
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
- **opencode** (runtime peer, not imported) — `opencode run --format json`,
  `--agent/--model/--variant`, skills under `~/.config/opencode/skills`.
