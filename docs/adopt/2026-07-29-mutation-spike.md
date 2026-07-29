# Mutation-testing spike — gremlins (2026-07-29)

Issue: #116 — spike a Go mutation tool before adding the dispatch-only
mutation-testing workflow (`.github/workflows/mutation.yml`).

## Tool chosen

`gremlins` (github.com/go-gremlins/gremlins), installed via
`go install github.com/go-gremlins/gremlins/cmd/gremlins@latest` into
`$GOPATH/bin` (kept out of the repo; no go.mod/go.sum changes).

## Did it complete on this tree?

**Yes.** Ran from `internal/config` (foundation SHA
`6557715dd59f3b172f0fd40ca60a65acd257e0a5`, Go 1.26.5 darwin/arm64):

```
Mutation testing completed in 17 seconds 776 milliseconds
Killed: 103, Lived: 52, Not covered: 39
Timed out: 2, Not viable: 0, Skipped: 0
Test efficacy: 66.45%
Mutator coverage: 79.90%
```

Exit code 0. No fallback alternative was needed, so no other tool was tried.

## Survivors in `StepStatus.IsTerminal()` (internal/config/cycle.go:117–123)

**None.** All mutants generated inside `IsTerminal()`'s body were KILLED.
For context, the surviving (LIVED) mutants in `cycle.go` overall are
elsewhere: CONDITIONALS_NEGATION at lines 140, 297, 303 and
CONDITIONALS_BOUNDARY at line 188 (five sites). The NOT COVERED mutants in
`cycle.go` (lines 144, 145, 153, 157, 223, 226) are also outside
`IsTerminal()`.

## Consequences for the workflow

- The tool runs on this tree, so `.github/workflows/mutation.yml` was added.
- The workflow is `workflow_dispatch` only, `continue-on-error: true`,
  `fail-fast: false` matrix over `internal/engine`, `internal/runner`,
  `internal/config`, artifacts uploaded with `if: always()`, and a closing
  informational summary step. Mutation survivors never gate a PR.
- Baseline (2026-07-29, internal/config): test efficacy 66.45%, mutator
  coverage 79.90% — useful as a reference point for future dispatch runs.
