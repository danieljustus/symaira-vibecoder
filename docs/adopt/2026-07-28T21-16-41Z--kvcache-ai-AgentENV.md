<!-- review: timestamp=2026-07-28T21:16:41Z  repo=danieljustus/symaira-vibecoder  head=875fc43 -->
<!-- adopt: source=kvcache-ai/AgentENV  source_ref=6296bc4be7ad79eb3a278eb5264ef011c341adf5  source_url=https://github.com/kvcache-ai/AgentENV  depth=clone  license=MIT -->

# Adoption Report — symaira-vibecoder ← kvcache-ai/AgentENV — 2026-07-28

## Sources

| Field | Value |
|---|---|
| SOURCE | `kvcache-ai/AgentENV` (https://github.com/kvcache-ai/AgentENV) |
| Ref analyzed | `6296bc4` (`main`) |
| Language / License | Rust (4.7 MB) + Go (350 KB) + Shell / MIT |
| Health | 1408 stars, 126 forks, last push 2026-07-28, **created 2026-07-23 — five days old**, one release `v0.1.0` (2026-07-25), 1.5 GB disk |
| Scope | all facets, full clone |
| TARGET | `danieljustus/symaira-vibecoder` @ `875fc43` |

## Verdict

AgentENV runs agent sandboxes on Firecracker microVMs at cluster scale; symvibe is a
single CGO-free Go binary serving a localhost board that drives `opencode`. No product
overlap — nothing from their sandbox, storage or scheduling layers survives the move.
The one area where they are genuinely ahead of us is **what they do with a coverage
number after computing it**: we compute a total in `check-coverage.sh`, compare it to a
threshold, print it and throw it away, while they publish a machine-readable
`coverage.json` plus a badge to an orphan branch. That gap is a ~15-line increment on
code we already have, and it is the highest-value takeaway. A dispatch-only mutation
workflow is a plausible second step.

Health caveat: five days old, no maintenance track record. Every finding below is
justified by our own gap, not by upstream's popularity; confidence capped at `medium`.

## What we already do as well or better

- Coverage measured and gated in CI → `.github/scripts/check-coverage.sh` + the `coverage` job at `.github/workflows/ci.yml:74` already enforce a threshold from `.github/coverage-config.json`; upstream's coverage run is explicitly non-blocking.
- Static analysis and supply-chain checks → we run `codeql.yml`; AgentENV has no code scanning at all.
- Release automation → `.goreleaser.yml` + `.github/workflows/release.yml` (350 lines) is more reproducible than their hand-rolled release job.
- Curated release notes and prerelease gating → our `06-gh-prerelease`/`07-gh-release` flow beats their `cliff.toml` commit-dump changelog.
- Machine-readable CLI output where it matters → `cmd/symvibe/doctor.go:73` already ships `--json`.

## Findings

- [ ] **[UX/DX] Publish the coverage total as a badge and a machine-readable baseline on an orphan branch**
  - **Status quo:** `.github/scripts/check-coverage.sh:17-29` already computes `TOTAL` from `go tool cover -func`, prints it, compares it against the threshold in `.github/coverage-config.json` and then discards it — the number never leaves the job log. Consequence: `README.md` carries CI/Release/Go/License badges but no coverage badge, and any later coverage review has to re-derive the previous number by hand. Upstream `.github/workflows/coverage.yml:60-135` keeps the same computation but adds a `publish-coverage-data` job that force-writes `badge.json`, `badge.svg` and `coverage.json` to an orphan `coverage-data` branch on pushes to `main`, so history stays clean and the README badge (their line 10) reads straight from the branch.
  - **Proposed solution:** Pattern adoption, no code copy. Extend `.github/scripts/check-coverage.sh` to also emit `coverage.json` (total, threshold, commit SHA) and a shields-compatible `badge.json`/`badge.svg`, and add a `publish-coverage-data` job to `.github/workflows/ci.yml` — gated on `github.ref == 'refs/heads/main'` and on the coverage job succeeding — that commits those three files to an orphan `coverage-data` branch. Add the badge to `README.md`.
  - **Effort/Impact:** Low effort / medium impact. We already have the number and the gate; this is publication only. Nothing on `main` changes, and it is reversible by deleting the branch and the job.

- [ ] **[Architecture] Add a dispatch-only, non-blocking mutation-testing workflow**
  - **Status quo:** The threshold in `.github/coverage-config.json` tells us how much code is executed by tests, but nothing tells us whether those tests would *fail* if the code broke — and the risk concentrates in exactly the packages that decide autonomous behavior (`internal/engine/`, `internal/runner/`, `internal/config/cycle.go`, whose `StepStatus.IsTerminal()` at line 117 gates whether the autonomous walk may pass over a step). A line-coverage gate cannot catch a wrong terminal-state predicate. Upstream `.github/workflows/mutation-tests.yml:1-51` shows a viable shape for an expensive tool: `workflow_dispatch` only, package matrix with `fail-fast: false`, `timeout-minutes: 720`, `continue-on-error: true`, artifacts always uploaded, and a closing step that reports the outcome as informational instead of failing the run.
  - **Proposed solution:** Pattern adoption. Add `.github/workflows/mutation.yml` with that shape, matrixed over `internal/engine`, `internal/runner` and `internal/config`, driven by a Go mutation tool (`gremlins` is the nearest `cargo-mutants` analogue). Survivors are read from artifacts, never gate a PR. The workflow shape is the transferable part; the tool choice needs a spike first.
  - **Effort/Impact:** Medium effort / medium impact. Targets the code where a silent test gap costs most (autonomous cycle progression). Confidence `medium` — Go mutation tooling is much less mature than `cargo-mutants`, so verify a tool completes on our tree before writing YAML.

## Considered and rejected

- **Firecracker microVMs, overlaybd image loading, ublk storage, uffd snapshots, P2P artifact transport** (`src/sandbox/`, `storage/`, `src/p2p/`) — gate 1 (Transferable): Linux/KVM cluster infrastructure; symvibe is a single localhost Go binary. Most of the repo dies here.
- **Running the agent's work inside a disposable sandbox** — gate 1 (Transferable): conceptually adjacent to symvibe driving `opencode` on the user's own repos, but their isolation model requires a Linux KVM host, and symvibe's entire value is operating on the *real* working tree. Sandboxing would defeat the product.
- **TTY-aware output format resolution** (`crates/aenv/src/output.rs:19-27`) — gate 3 (Better): no meaningful delta here. Our CLI surface is a single `doctor --json` (`cmd/symvibe/doctor.go:73`); the primary interface is the web board, so a shared table/JSON renderer would abstract over one call site.
- **`adev`, their in-repo dev CLI** (`adev/src/`) — gate 4 (Worth it): amortizes over 12 workflows and 6 crates; our `Makefile` covers one Go module comfortably.
- **Composite actions to dedupe CI setup** (`.github/actions/rust-ci-setup/`) — gate 4 (Worth it): we have 3 workflows and a two-step setup. Revisit past ~6 workflows.
- **mdbook docs site on GitHub Pages** (`.github/workflows/docs.yml`) — gate 4 (Worth it): adds a Pages surface to maintain; `README.md` + `docs/` render fine on GitHub for a solo repo. Rule 9 (scale fit).
- **`git-cliff` changelog generation** (`cliff.toml`) — gate 3 (Better): our `07-gh-release` notes are curated; cliff produces a grouped commit dump.
- **`alibaba/open-code-review` on `pull_request_target`** (`.github/workflows/open-code-review.yml`) — gate 4 (Worth it): grants fork-PR-triggered runs access to repo secrets, and pins the action to `@main`. We already have CodeQL plus the `01-code-review` skill.
- **`curl … | sudo bash` installers** (`scripts/install.sh`) — gate 3 (Better): our goreleaser assets and Homebrew tap are strictly better.
- **Benchmark workflows and `benches/`** (`.github/workflows/benchmark.yml`) — gate 4 (Worth it): they defend a sub-100 ms product claim; symvibe's latency is dominated by `opencode` runs we do not control.

## Open questions

- Is there a Go mutation tool that completes on this tree in reasonable time? Settle with a single local run against `internal/config` before writing finding 2's workflow.
- Does an orphan `coverage-data` branch survive the `gh-branch-clean` "reduce to main" policy and branch protection? It must be explicitly exempted, or the badge breaks on the next cleanup.
- Upstream is five days old — no evidence that any of this survives maintenance. Judge on our own cost/benefit only.

**First step:** teach `.github/scripts/check-coverage.sh` to emit `coverage.json` + `badge.json` and publish them to a `coverage-data` branch — the number already exists, only the publication is missing.
