<!-- review: timestamp=2026-07-29T10:35:23Z  repo=danieljustus/symaira-vibecoder  head=875fc43886104c2b973f97f5d317df3842b358d8 -->
<!-- adopt: source=VasiHemanth/tokentelemetry  source_ref=d1c2f378386832e4afa835e7a041701408dbf3b9  source_url=https://github.com/VasiHemanth/tokentelemetry  depth=clone  license=MIT -->

# Adoption Report — symaira-vibecoder ← VasiHemanth/tokentelemetry — 2026-07-29

## Sources

| Field | Value |
|---|---|
| SOURCE | `VasiHemanth/tokentelemetry` (https://github.com/VasiHemanth/tokentelemetry) |
| Ref analyzed | `d1c2f37` (main) |
| Language / License | Python (FastAPI backend) + TypeScript (Next.js frontend) / MIT |
| Health | 289 stars, 41 forks, last push 2026-07-28 (today-1), not archived, active release notes in `UPDATE.json`, ADRs and design docs maintained |
| Scope | all facets, full clone |
| TARGET | `danieljustus/symaira-vibecoder` @ `875fc43` (Go binary + SwiftUI client) |

## Verdict

Worth learning from, in a narrow but well-aimed way. The repos are structurally opposite — TokenTelemetry *observes* coding agents by reading their logs after the fact; symvibe *drives* them and owns the live event stream — so nothing about their scanners, SQLite rollup or dashboard transfers. What does transfer is that they have already solved, and documented the rationale for, the two problems symvibe is walking into: (a) how to price a coding-agent run without making a network call on the user's machine, and (b) how to let agent-derived data leave the process without leaking prompts, paths and credentials. The single highest-value takeaway is the first: symvibe runs paid backends autonomously and unattended and currently discards the usage numbers that flow past it, so a cycle's cost is unknowable after the fact.

## What we already do as well or better

- Local-first, loopback-only, no cloud → `cmd/symvibe/serve.go` binds `127.0.0.1` with TLS + paired-token auth (`internal/auth/middleware.go`); we never had a remote mode to retrofit.
- Multi-agent support via per-agent adapters (`backend/summarizers/*.py`) → our `Runner` interface with six backends (`internal/runner/runner.go:70`) is the same seam, applied to driving rather than reading.
- Durable run history as a decided schema → their ADR-0002 SQLite rollup is answered by `docs/ARCHITECTURE.md:115` "RunState: structured learnings, not just a log", which is the stronger design (typed learnings rows, not a raw log).
- Decision provenance in-repo → their `docs/adr/` vs. our dated, issue-linked decision sections in `docs/ARCHITECTURE.md` plus the `docs/adopt/` + `.github/review/` trail. Comparable depth for a solo repo.
- Single-command install → `scripts/install.sh` + Homebrew tap + GoReleaser; a single CGO-free binary beats their Python+Node install story.
- CI concurrency cancellation and least-privilege `permissions:` → `.github/workflows/ci.yml:18-25` already does both.

## Findings

- [ ] **[Architecture] Capture per-run token usage and price it from a bundled, CI-refreshed pricing table**
  - **Status quo:** symvibe drives paid backends unattended (`SkipPerms`, autonomous cycle walk in `internal/engine/engine.go:280`) and throws the usage data away: `parseSSELine` in `internal/runner/api.go:220` handles `message_delta` only for stop reasons and never reads its `usage` block, and `internal/runner/events.go` maps opencode lines without a usage field at all. `RunEvent` (`internal/runner/runner.go:37`) has no token or cost field, so no cost figure exists anywhere — not in the board, the activity log, the recipe result, or a run record. `docs/ARCHITECTURE.md:104` lists "live token streaming" as roadmap but says nothing about turning tokens into money. Upstream solves the pricing half deliberately: `backend/pricing.py:1-40` is a two-tier lookup (provider+model, then flat model) reading a **bundled** `pricing_data.json` with **zero network I/O at import or runtime**, and `.github/workflows/pricing-sync.yml` refreshes that file weekly from models.dev by opening a PR — so freshness is a maintainer/CI concern, never an outbound call from a user's machine. Their `DESIGN.md` also warns from measured data that pricing a subagent with the parent's model is simply wrong.
  - **Proposed solution:** Pattern adoption (their MIT license would permit copying, but the pricing table is data we should curate ourselves for the models we actually resolve). Add `Usage` (input / output / cached-read tokens + model) to `RunEvent`, populate it in `api.go` from `message_delta.usage` and in `local_api.go` from the OpenAI-compatible `usage` object — both already parse the frame that carries it. Keep opencode `unknown` rather than estimating (their explicit "do NOT estimate" rule for Cursor). Add a small `internal/pricing` package with an embedded table keyed by `provider/model` and a flat fallback, `go:embed`ed and network-free at runtime, refreshed by a scheduled workflow that opens a PR — same shape as `pricing-sync.yml`. Cost per step then lands on the terminal `EventDone` and, when `RunState` ships, in the run record next to the learnings rows. Budgets/alerts are explicitly *not* in this finding.
  - **Effort/Impact:** Medium effort / high impact. Reversible — `Usage` is additive on an existing struct and the pricing package is standalone; the two API-shaped backends can ship first, opencode later once `opencode serve`'s `/event` demux (already on the roadmap) exposes usage. Confidence `high` for the pricing mechanism (motivation documented in-repo), `medium` for opencode coverage.

- [ ] **[Security] Stop shipping raw backend payloads out of the process unredacted**
  - **Status quo:** Every `RunEvent` carries `Raw` — the verbatim backend line (`internal/runner/runner.go:42`, set at `internal/runner/events.go:55-66` and `internal/runner/api.go:203-228`) — which means whole prompts, file contents, tool arguments and any provider error text containing a key. `internal/recipe/service.go:108` collects those events into `RecipeResult.Trace` and returns them to the MCP caller, and `writeTrace` persists them into the caller's workspace at mode `0o644` (`internal/recipe/service.go:287`). This is the same defect class CodeQL already flagged once in this repo — `go/clear-text-logging` alerts #15/#16, fixed in `c1833ef` by simply deleting the field from one `slog.Warn` in `cmd/symvibe/serve.go`, leaving the much larger trace surface untouched. Upstream treats this as an invariant rather than a bug fix: `backend/telemetry.py:15-20` is "content-free by construction" — an explicit per-event key allowlist, every value forced through a scalar/enum coercion, everything else dropped — with `backend/test_telemetry_redaction.py` as a dedicated guardrail test asserting a list of sensitive strings can never appear; their most recent commit (`d1c2f37`) extends the same ban to session URLs and machine identifiers.
  - **Proposed solution:** Pattern adoption. Give `RunEvent.Raw` a single redaction chokepoint in `internal/runner` applied before an event leaves the runner: drop or truncate `Raw` by default and keep it only behind an explicit debug opt-in, plus a pattern scrub (`sk-…`, `Bearer …`, `api_key=…`, `?token=`) on `Text`/`Err`, which today already carry `input_json_delta` tool arguments verbatim to the SSE feed (`internal/runner/api.go:212` → `internal/engine/engine.go:288`). Add a table-driven guardrail test in the shape of theirs — a list of sensitive literals that must not survive a round trip through the trace serializer. Tighten `writeTrace` to `0o600` in the same change.
  - **Effort/Impact:** Low-to-medium effort / high impact. Fully reversible (one function plus a config flag); closes the general case of an alert class this repo has already been hit by, and shrinks what a compromised or over-broad MCP caller can read out of `POST /api/recipe/run`. Confidence `high`.

- [ ] **[Security] Add a blocking dependency-vulnerability gate to CI**
  - **Status quo:** `.github/workflows/ci.yml` runs lint, gofmt, vet, test, race and build — nothing checks dependencies for known CVEs, and `govulncheck` appears nowhere in the repo. The two mechanisms we do have are both non-blocking: `.github/dependabot.yml` opens PRs after the fact, and `.github/workflows/codeql.yml` is code analysis on a *weekly schedule*, not on PRs. That gap has already cost us once — `c1833ef` had to bump `golang.org/x/net` 0.48.0 → 0.55.0 for CVE-2026-25680 as a security-alert cleanup rather than a merge that CI refused. Upstream added exactly this gate for exactly this reason, and says so in the workflow header of `.github/workflows/security-audit.yml`: "issue #91 — vulnerable runtime deps … were merged because nobody ran `npm audit` before shipping", with runtime deps blocking on high/critical and the dev tree advisory-only.
  - **Proposed solution:** Pattern adoption (their YAML is npm-specific; the Go analogue is one step). Add a `vuln` job to `.github/workflows/ci.yml` running `govulncheck ./...`, which is stricter than a manifest audit because it reports only vulnerabilities actually reachable from our call graph — so it blocks on real exposure instead of on every advisory in `go.sum`. Keep it blocking on PRs, and adopt their split posture by leaving anything unreachable-but-known to Dependabot.
  - **Effort/Impact:** Low effort / medium-high impact. One job, no new repo dependency, trivially reversible. A vulnerable `golang.org/x/net` (still an indirect dep via mdns) can be merged with a green CI today; after, it cannot. Confidence `high`.

## Considered and rejected

- **Budgets & alerts per project/agent (`backend/harness_config.py:205`, observational, 80%/100% thresholds)** — gate 4 (Worth it): sound design, but it is downstream of usage capture existing at all. Revisit after the cost finding lands.
- **Durable SQLite history rollup (ADR-0002, `backend/history_store.py`)** — gate 2 (New): already decided and better specified for our shape at `docs/ARCHITECTURE.md:115` (typed learnings rows + raw log), from the `google/mantis` report.
- **`docs/adr/` as a separate decision-record directory** — gate 3 (Better): equivalent depth already exists as dated, issue-linked decision sections in `docs/ARCHITECTURE.md` plus `docs/adopt/` and `.github/review/`; no consequence named for the split.
- **Anonymous product telemetry to a Cloudflare Worker (`backend/telemetry.py`)** — gate 4 (Worth it): adds an outbound network path, a hosted Worker to maintain, and a trust conversation, to a tool whose selling point is that it never phones home. The redaction *technique* is adopted above; the sink is not.
- **`.claude/hooks/protect-sensitive-files.py` (PreToolUse deny on `.env`/`*.db` writes)** — gate 3 (Better): the motivation is a tracked `.env` with real credentials in their repo root; we have none, and no incident of an agent clobbering secrets here.
- **`llms.txt` (LLM-oriented repo summary)** — gate 3 (Better): a discoverability play for a product with a marketing site; no pain in this repo it resolves.
- **`UPDATE.json` in-app release highlights** — gate 3 (Better): `CHANGELOG.md` plus GitHub releases already carry this; showing it inside the board is a feature request, not a delta on existing code.
- **Docker/Podman + Compose packaging (`compose.yml`, `Makefile` up/down/logs)** — gate 4 (Worth it): it exists upstream because installing Python *and* Node on a host is painful. We ship one CGO-free binary; a container adds a maintained surface for no user gain.
- **Fumadocs documentation site (ADR-0003, `website/`)** — gate 1 (Transferable) on scale fit: a Next.js docs site plus static search index is a second toolchain for a repo whose README already covers the surface.
- **Per-agent log scanners with a 30s TTL cache (`backend/main.py`, `scan_cache.py`)** — gate 1 (Transferable): they reconstruct sessions by reading other tools' transcripts; we own the live event stream and never need to scan anything.
- **Cost-anomaly detection / model-efficiency analytics (`backend/insights.py`)** — gate 4 (Worth it): needs a history corpus we do not have yet and a dashboard we do not want to own.
- **Energy/CO₂ estimation (`backend/power_meter.py`)** — gate 3 (Better): no pain point in this repo, and the numbers are modelled, not measured.

## Open questions

- Does `opencode run --format json` emit token usage on any event type at all, or only via `opencode serve`'s `/event` stream? Nothing in the SOURCE answers this — they read opencode's SQLite, not its stdout. Settled by capturing one real `--format json` run and grepping for a usage-shaped object; that result decides whether the default backend gets cost numbers in phase one or has to report `unknown`.
- Which pricing source to bundle. Upstream uses models.dev plus a hand-curated override table because the two disagree. Settled by diffing models.dev against the actual `provider/model` strings our `config.Resolver` emits, and seeing how many resolve.
- Whether any MCP caller currently depends on `RunEvent.Raw` being present in `RecipeResult.Trace`. Settled by checking the SymDesk/vault caller before redaction changes the contract — if it does, the debug opt-in has to be reachable from the request.

**First step:** add `govulncheck ./...` as a blocking job in `.github/workflows/ci.yml` — one commit, closes a gap this repo has already been bitten by.
