<!-- review: timestamp=2026-08-03T09:55:11Z  repo=danieljustus/symaira-vibecoder  head=6ac39b30f769356b818e99bd65c99c3cb412ed9a -->
<!-- adopt: source=Comfy-Org/ComfyUI  source_ref=14b05228cef127ce529bc0c08660770d4af3e9a8  source_url=https://github.com/Comfy-Org/ComfyUI  depth=clone  license=GPL-3.0 -->

# Adoption Report — symaira-vibecoder ← Comfy-Org/ComfyUI — 2026-08-03

## Sources

| Field | Value |
|---|---|
| SOURCE | `Comfy-Org/ComfyUI` (https://github.com/Comfy-Org/ComfyUI) |
| Ref analyzed | `14b05228` (master) |
| Language / License | Python / **GPL-3.0 — pattern-level adoption only** (TARGET is Apache-2.0) |
| Health | 123,327 stars, last push 2026-08-03, not archived, release `v0.30.0` published 2026-08-03 (very active) |
| Scope | all facets, full clone (history deepened to 2,001 commits for rationale hunting) |
| TARGET | `danieljustus/symaira-vibecoder` @ `6ac39b30` (Go 1.26 + SwiftUI client) |

## Verdict

The languages and domains have nothing in common — Python diffusion backend vs. a
CGO-free Go orchestrator — but the **deployment shape is identical**: a local HTTP
server on a loopback port that serves an embedded browser UI, pushes live execution
events to it, and executes expensive, side-effectful work on the user's machine when
that UI says "run". Every finding below comes from that shared shape, and nothing from
their model loading, node graph, execution cache or queue survived the gates.

The single highest-value takeaway is **security**: ComfyUI learned (and fixed, with the
reasoning left in a code comment) that binding to `127.0.0.1` does *not* stop an
arbitrary website from POSTing to your local server. `symvibe`'s loopback mode has
exactly that hole today, and the button it exposes starts an autonomous AI coding agent
against the user's working tree. That one is worth doing before the other two.

## What we already do as well or better

- ComfyUI's `--enable-cors-header` / `--listen` / `--tls-keyfile` flag surface for
  widening access (`comfy/cli_args.py:63-67`) → `symvibe`'s `server.access` mode table
  (`loopback|lan|relay`) is strictly better: it couples binding, auth and TLS into one
  decision and **fails closed** when `lan`/`relay` is chosen without auth
  (`SECURITY.md` "Network access modes", `cmd/symvibe/serve.go:165`). ComfyUI lets you
  bind `0.0.0.0` with no authentication at all.
- Their SSE/WS client re-sync on reconnect — the server re-sends `status` and, for the
  owning client, the currently executing node (`server.py:284-290`) → we already prime a
  fresh SSE subscriber with the live `run_state` (`internal/server/sse.go:44`).
- Their capability negotiation between frontend and backend over the socket's first
  message (`comfy_api/feature_flags.py`, `server.py:300-315`) → covered by our version
  handshake and the corekit binding landed in `297f59c`, plus `GET /api/version`
  (`internal/server/handlers_meta.go`).
- Their canonical `openapi.yaml` + Spectral naming rules as the cross-language wire
  standard (`openapi.yaml`, `.spectral.yaml`) → we enforce the same class of guarantee
  where it actually bites us, across Go **and** Swift, with
  `.github/wire-contract.json` + `internal/config/contract_test.go` +
  `.github/scripts/check-wire-contract.sh` in the CI gate. I verified every path used by
  `client/Sources/SymvibeKit/APIClient.swift` and `web/dist/index.html` against
  `internal/server/server.go:52-100` — no drift exists.
- Their per-advisory regression tests (`tests-unit/security_test/test_ghsa_779p_*.py`,
  one file per published GHSA fix) → equivalent discipline already in
  `internal/runner/redact_test.go` for the clear-text-logging fix (`c1833ef`), plus
  CodeQL and `govulncheck` gates that ComfyUI has no counterpart for.
- Their `-S`-style dependency minimalism and "prefer fewer dependencies" rule
  (`AGENTS.md` "Engineering Style") → we run on three direct dependencies and a
  dependency-free embedded board; nothing to learn here.

## Findings

- [ ] **[Security] Reject cross-site browser requests to the loopback board**
  - **Status quo:** In the default `loopback` mode `internal/auth/middleware.go:16-19`
    bypasses authentication for *every* request whose `RemoteAddr` is loopback, and no
    handler checks `Origin`, `Sec-Fetch-Site` or `Host` (`rg -n -i 'origin|csrf|sec-fetch'`
    over `internal/` returns only git-remote matches). `POST /api/run`
    (`internal/server/handlers_run.go:28`) reads **no request body**, so any page the
    user visits can fire `fetch('http://127.0.0.1:4317/api/run', {method:'POST', mode:'no-cors'})`
    — a CORS-simple request that browsers send regardless — and start an autonomous
    cycle that runs a coding agent with `--dangerously-skip-permissions` against the
    user's repo. A DNS-rebinding page is worse: `RemoteAddr` is still loopback, the
    attacker's origin matches its own host, so responses are readable too. `SECURITY.md`
    documents the threat model as "loopback only … do not bind it to a public interface",
    which is precisely the assumption this breaks — the vector is unconsidered, not
    consciously accepted. Upstream fixed the same class in `server.py:159-197`
    (`create_origin_only_middleware`): reject `Sec-Fetch-Site: cross-site` outright, then
    reject any request whose `Host` is loopback but whose `Origin` hostname differs. The
    motivation is stated in-code — "prevent the case where a random website can queue
    comfy workflows by making a POST to 127.0.0.1 which browsers don't prevent" — and in
    commit `76b75f3` ("Fix some issue with insecure browsers").
  - **Proposed solution:** Pattern adoption (GPL-3.0 source — no code copy; the mechanism
    is three header comparisons, reimplement in Go). Add an origin guard to
    `internal/auth/` in front of the existing middleware in
    `internal/server/server.go:44-50`: 403 when `Sec-Fetch-Site` is `cross-site`, and 403
    when `Origin` is present, `Host` resolves to loopback, and the two hostnames differ.
    Requests without an `Origin` header pass — that keeps the SwiftUI client, `curl` and
    the recipe/MCP callers working, since only browsers send `Origin`. Apply it in every
    access mode (it is orthogonal to the Bearer-token gate), and add a `server.allow_origin`
    escape hatch mirroring their `--enable-cors-header` for anyone embedding the board
    elsewhere. Table-driven handler test alongside `internal/server/server_test.go`.
  - **Effort/Impact:** Low effort / high impact. ~40 lines plus tests, entirely additive
    and reversible (drop the wrapper), no new dependency; closes a remote-code-execution-
    adjacent path that is reachable today from any browser tab.

- [ ] **[UX/DX] Boot the real server in CI and fail on startup errors**
  - **Status quo:** The CI smoke step is `./symvibe version` (`.github/workflows/ci.yml:71`)
    — it never starts `serve`, so nothing in the pipeline exercises the wiring in
    `cmd/symvibe/serve.go`: port binding, `deriveBindHost`, mDNS advertisement
    (`internal/server/mdns.go`), TLS setup for `lan`/`relay`, the `go:embed` board FS
    (`web/embed.go`) or config discovery. `internal/server/server_test.go` uses `httptest`
    and starts below all of that, and `cmd/symvibe/main_test.go` only covers `version`,
    `doctor` and pure helpers. A change that makes `symvibe serve` panic or serve a broken
    board ships green — which is the same class of regression the four consecutive CI
    fixes at `6ac39b3`, `9496301`, `f9ed1bc` and `875fc43` were chasing in the release
    path, just one layer up. Upstream runs `.github/workflows/test-launch.yml`: start the
    server, wait for the port, then **grep its captured console output for
    `Exception|Error` and fail the job**, uploading the log as an artifact either way.
  - **Proposed solution:** Pattern adoption (a 20-line shell job; do not copy their YAML).
    Add a `launch` job to `.github/workflows/ci.yml`: `./symvibe serve --no-open &`, poll
    `GET /api/version` until it answers or a timeout expires, assert `GET /` returns the
    embedded board, then `kill` and fail if stderr contains `panic:` or `level=ERROR`
    (our `log/slog` stderr convention makes this a cleaner check than their `grep -v`
    filter chain, which they already have to whitelist known-noisy lines out of). Upload
    the log with `if: always()`. Fits the existing fast-PR-gate shape and needs no new
    tooling.
  - **Effort/Impact:** Low effort / medium impact. Under an hour, fully reversible (delete
    the job), no new dependency; converts "the binary starts and serves the board" from an
    untested assumption into a gate on every PR.

- [ ] **[UX/DX] Keep a bounded log ring buffer with a replay endpoint so reconnecting clients backfill**
  - **Status quo:** Runner output reaches the UI only as transient `log` events on the
    in-process bus, which drops them for any subscriber whose 256-slot channel is full
    (`internal/engine/bus.go:66-70`) and never stores them. `internal/engine/ledger.go`
    persists structured `run_start`/`step_start`/`step_end` evidence but no log lines, and
    `internal/server/sse.go:44` primes a new subscriber with `run_state` only. So every
    line emitted while a client is disconnected is gone permanently — and the SwiftUI
    client reconnects constantly by design (`client/Sources/SymvibeKit/SSEClient.swift:23`
    reconnects with exponential backoff; iOS backgrounds the app during multi-minute
    steps), after which `ActivityStore` silently shows a gap with no marker. Upstream keeps
    a fixed-size `deque(maxlen=300)` of log entries (`app/logger.py:98-105`) and exposes
    `GET /internal/logs/raw` returning `{entries, size}` (`api_server/routes/internal/internal_routes.py:26-33`),
    so the frontend terminal backfills on connect while `TerminalService` pushes deltas
    live to subscribed clients (`api_server/services/terminal_service.py:48-59`).
  - **Proposed solution:** Pattern adoption. Add a bounded ring buffer (say 500 entries,
    matching `ActivityStore`'s `maxLines`) next to the bus in `internal/engine/bus.go`,
    fed on `Publish` for `log`/`error` events, and a `GET /api/logs` handler in
    `internal/server/` returning the buffer with the current `run_id`. Have `SSEClient`
    and the board fetch it once after each (re)connect and merge by the existing `ts`
    field. Deliberately *not* adopting their client-scoped subscription model
    (`sockets_metadata`, per-`sid` routing) — `symvibe` is single-user with one active
    run, so a single shared buffer is the whole mechanism. Memory is bounded and the run
    ledger stays the durable record.
  - **Effort/Impact:** Medium effort / medium impact. Touches the bus, one new handler and
    both clients; reversible (the endpoint can ship first and clients adopt it
    independently); no new dependency. Confidence `medium` — the mechanism is visible and
    wired in upstream, but I could not find a commit or issue stating why they added it,
    so the delta rests on our own reconnect behavior, which is directly observable.

## Considered and rejected

- **Their canonical `openapi.yaml` + Spectral lint job** (`openapi.yaml`, `.spectral.yaml`,
  `.github/workflows/openapi-lint.yml`) — gate 4 (Worth it): a hand-maintained spec is a
  second source of truth that drifts from `internal/server/server.go:52-100` with no
  mechanical link, and Spectral means adding Node to a pipeline whose stated property is
  that `go build` never needs it (`AGENTS.md` "Build & Test"). I checked for the defect it
  would catch and found none — every path in `APIClient.swift` and `web/dist/index.html`
  exists in the mux.
- **Their HTTP cache-control middleware** (`middleware/cache_middleware.py`) — gate 3
  (Better): plausible on paper for a stale board after `brew upgrade`, but no defect is
  demonstrated. We serve one self-contained `index.html` from `go:embed`, whose zero
  ModTime means no `Last-Modified` and therefore no heuristic browser caching to begin
  with.
- **`PromptQueue` — queue submissions instead of rejecting them while busy**
  (`execution.py`, `server.py:get_queue_info`) — gate 3 (Better): our `409 ErrBusy` +
  `s.busy()` edit lock (`internal/server/server.go:104`) is a deliberate choice for a
  single-cycle orchestrator walking a persisted board; a queue would need a second source
  of truth for step status. No recorded pain asks for it.
- **Per-client progress isolation** (`tests/execution/test_progress_isolation.py`,
  `sockets_metadata` routing) — gate 1 (Transferable): solves multi-tenant event routing;
  `symvibe` is single-user with one active run, so every subscriber wants every event.
- **Versioned frontend as an installable package with `--front-end-version` /
  `--front-end-root`** (`app/frontend_management.py`) — gate 1 (Transferable): the
  single-binary, Node-free `go:embed` board is an explicit architectural decision
  (`docs/ARCHITECTURE.md` "Key decisions", `AGENTS.md`), and their pattern needs a package
  registry and a runtime download path.
- **Per-advisory security regression tests** (`tests-unit/security_test/test_ghsa_779p_*.py`)
  — gate 3 (Better): a good convention, but it needs published advisories to hang tests
  on; we have none, and the one shipped security fix already has its regression test
  (`internal/runner/redact_test.go`).
- **`Close stale issues` workflow** (`.github/workflows/stale-issues.yml`) — gate 4 (Worth
  it) / scale fit: tuned for a 123k-star inbound support queue. This repo currently has
  zero open issues; automated nagging has no addressee.
- **`Detect Unreviewed Merge` SOC 2 workflow** (`.github/workflows/detect-unreviewed-merge.yml`)
  — gate 1 (Transferable): structurally requires a second reviewer and a separate
  org-level tracking repo. Solo repo.
- **`Check AI Co-Authors` gate** (`.github/workflows/check-ai-co-authors.yml`, blocking
  PRs that carry AI co-author trailers) — gate 1 (Transferable): the inverse of this
  project's premise, which is autonomous agent-authored PRs.
- **CRLF line-ending check** (`.github/workflows/check-line-endings.yml`) — gate 2 (New):
  `.gitattributes`-free but Go-only and macOS/Linux-developed; `gofmt -l` in
  `.github/workflows/ci.yml:57` already fails on any Go file that is not canonically
  formatted.
- **Their `AGENTS.md` "No Internet Requests" hard boundary** — gate 1 (Transferable):
  `symvibe`'s core function is driving remote model APIs and fetching the template
  library; the rule is meaningful for a local inference engine, incoherent here.
- **`merge_json_recursive` config-merge helper** (`utils/json_util.py`) — gate 2 (New):
  our layered precedence (`override > category > default`, plus file → env → flag) is
  already explicit and tested in `internal/config/resolve.go` and
  `internal/config/config.go`.
- **gzip response compression middleware** (`server.py:100-113`) — gate 4 (Worth it):
  sized for multi-megabyte node-definition JSON over a LAN; our largest response is a
  cycle TOML rendered as JSON, and it buys nothing on loopback.

## Open questions

- Is `POST /api/run` with no body reachable cross-origin *in practice* on the browsers you
  target? The reasoning above says yes (no preflight for a simple POST, response
  unreadable but the side effect lands), but the settling evidence is a two-file repro: a
  page served from `http://localhost:9999` firing `fetch(..., {mode:'no-cors'})` at a
  running `symvibe serve`, checked against the run ledger. Worth doing as the first step
  of the security finding, since it also becomes the regression test.
- Upstream's log ring buffer has no commit or issue explaining why it was introduced (I
  searched `-S "logs/raw"` across 2,001 commits and their issue tracker). If a cheap
  signal exists that reconnect gaps actually bother you in the iOS client — a session
  where `ActivityStore` visibly lost lines — that would raise the third finding's
  confidence from `medium` to `high`; without it, it rests on code reading alone.

**First step:** add the cross-site origin guard in front of
`internal/auth/middleware.go`, with the cross-origin `POST /api/run` repro as its test.
