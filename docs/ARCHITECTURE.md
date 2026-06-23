# symvibe — Architecture

symvibe is a single, CGO-free Go binary that serves an editable **Baukasten**
(cycle board) on `127.0.0.1` and **drives opencode** to run the cycle's
phases/steps autonomously. It owns the orchestration; opencode is a runtime peer
reached through a swappable `Runner` interface (so a fork, Claude Code, or a
direct-API backend can drop in later without touching the rest).

## Layers

```
┌──────────────────────────────────────────────────────────────────────┐
│  Browser — embedded board (web/dist/index.html, dependency-free)       │
│  phases ▸ draggable step cards ▸ discreet status glyphs ▸ activity log  │
└───────────────▲──────────────────────────────────┬────────────────────┘
   REST /api/*  │ GET / (go:embed)                  │ GET /events (SSE)
┌───────────────┴──────────────────────────────────▼────────────────────┐
│  internal/server  (net/http ServeMux, SSE, embed serving)              │
│   handlers_cycle · handlers_run · handlers_meta · sse                  │
└───────────────────────────────┬───────────────────────────────────────┘
                                 │ Engine API
┌────────────────────────────────▼──────────────────────────────────────┐
│  internal/engine  (the "brain")                                        │
│   • walks config.Cycle.NextRunnable()  ← knows where it is             │
│   • sensors.go: cheap git/gh probes → AutoSkip                         │
│   • status.go: guarded transition table                               │
│   • bus.go: in-proc pub/sub → SSE fan-out                             │
└──────────┬───────────────────────────────────┬────────────────────────┘
   config.Resolver / config.Cycle              │ runner.Runner (interface)
┌──────────▼──────────────┐        ┌───────────▼────────────────────────┐
│  internal/config        │        │  internal/runner                    │
│  Cycle/Phase/Step,       │       │  OpenCodeRunner (MVP):              │
│  StepStatus, NextRunnable│       │   `opencode run --format json`      │
│  Resolver (override >    │       │   --agent/--model/--variant --dir   │
│   category > default),   │       │   --dangerously-skip-permissions    │
│  TOML persistence,       │       │   → newline-JSON → RunEvent stream  │
│  Discover{Skills,Agents, │       │  (future: ClaudeCode / API / fork)  │
│   Models}                │       └───────────┬─────────────────────────┘
└──────────────────────────┘                  │ os/exec
                                   ┌───────────▼─────────────────────────┐
                                   │  opencode 1.17.x (runtime peer)     │
                                   └─────────────────────────────────────┘
```

## The load-bearing flow — "Run step 1.1 (00-sync)"

1. Board `POST /api/run/step {step_id:"1.1"}` → `202 {run_id}` (engine kicked in
   a goroutine; non-blocking).
2. Engine loads the cycle, runs the step's `AutoSkip` sensor (if any). Predicate
   holds → `skipped` + reason; else continue.
3. Engine sets `in_progress`, persists the cycle TOML, publishes `step_status`.
4. `config.Resolver.BuildRunSpec` → `{model, variant, agent}` (override >
   category > default). Engine composes the message (`Run the $00-sync skill.`)
   and calls `runner.RunStep`.
5. `OpenCodeRunner` execs `opencode run … --format json`, scans newline-JSON into
   normalized `RunEvent`s. Each becomes a `log` event on the bus → SSE → the
   activity feed; the card glyph flips live.
6. Terminal `EventDone` (Err=="" → done, else failed) → engine sets the final
   status, persists, publishes. A failed step **halts** an autonomous cycle.

## How "autonomous & smart" works

- **Knows where it is** — `Cycle.NextRunnable()` is a pure selector over the
  *persisted* `Step.Status`, not an in-memory cursor. It walks past
  done/skipped, returns the first `pending` step, and **halts** at the first
  failed/blocked/needs_review/in_progress step (so a cycle never runs past a
  problem, and a crashed `in_progress` is reset to `pending` on resume via
  `ResetStuck`). Resume, "run from here", and "run only this" all fall out of it.
- **Knows what to skip** — each step may carry an `auto_skip = { sensor, when }`
  rule. Sensors are cheap probes returning an integer (`git-dirty`,
  `open-issues`, `open-prs`, …); `when` is a predicate (`==0`, `>0`, `>=3`). The
  engine runs the sensor *before* the step; a satisfied predicate → `skipped`
  with a recorded reason. A sensor error fails *open* (runs the step) and logs —
  it never silently skips work.

## Per-step / per-category models

A model binding is `{id, temperature, variant, fallback_models}` — the same shape
as `oh-my-openagent.json`. `Resolver.Resolve(step)` applies precedence
**step override > category binding > default category** and records which rule
won (`Source`) for the GUI. `BuildRunSpec` materializes the primary attempt onto
`--model/--variant`; on failure the engine walks `ResolvedModel.Chain()` via
`NextAttempt` (opencode runs once per `--model`; symvibe does the chaining).
*Temperature is advisory in the MVP — opencode exposes no per-request override.*

## Key decisions (and why)

- **Drive opencode, don't fork it.** Everything symvibe needs is on the headless
  CLI; a fork would mean owning opencode's whole provider/tool/session surface.
  The `Runner` interface keeps a fork available later as just another adapter.
- **No `--pure`.** Verified: `--pure` disables zen-provider auth ("Model is
  disabled"). The runner loads the user's real providers/agents.
- **MVP transport = `opencode run --format json` subprocess.** Simplest path that
  makes "Run actually drives opencode" true. `opencode serve` + `/event` SSE
  (session reuse, mid-run abort, richer subagent surfacing) is the roadmap
  upgrade behind `runner.mode = serve`.
- **TOML on disk, JSON on the wire**, same snake_case names — one mental model,
  no case-translation bugs.
- **Status persisted in the cycle TOML** (atomic temp+rename); SQLite run-history
  is roadmap, not MVP.

## Roadmap (not in the MVP)

- `internal/runner/serve.go` — `opencode serve` + `/event` demux: live token
  streaming, `/abort` cancel, child-session→subagent correlation.
- Intra-phase parallelism for `parallel_safe` review steps (cap from
  `max_parallel_subagents`).
- SQLite `RunState` for cross-execution history / audit.
- `needs_review` halt + an explicit human-ack affordance for high-risk steps
  (release, force-merge).
- A richer React/Vite board under `web/` (replaces the embedded vanilla board via
  `make web`).
