# Security

## Threat model & defaults

symvibe runs **locally** and is a single-user developer tool.

- **Loopback only.** The board/API binds to `127.0.0.1` by default
  (`[server] host`). It is not authenticated — do not bind it to a public
  interface. Nothing in symvibe exposes opencode itself to the network.
- **The board can run code.** Pressing *Run* makes opencode execute an AI coding
  agent against your working tree (git operations, file edits, `gh` calls, …).
  Treat the board like a terminal: only run it on repositories you trust.

## `--dangerously-skip-permissions`

By default (`[runner] skip_permissions = true`) symvibe passes
`--dangerously-skip-permissions` to `opencode run`. This is **required** for
unattended operation: without a TTY, opencode would otherwise block forever on a
tool-permission prompt and the step would hang.

The trade-off: opencode will auto-approve any permission **not explicitly
denied** for that run. To restrict what an agent may do, configure opencode's own
permission rules (e.g. via your `oh-my-openagent` / opencode agent permissions),
or set `skip_permissions = false` (or `SYMVIBE_SKIP_PERMISSIONS=0`) and run
opencode interactively instead.

A per-step wall-clock budget (`[runner] request_timeout`, default 30m) bounds a
runaway or hung step, which is then marked `failed`.

## Reporting a vulnerability

Email the maintainer (see the repository owner) rather than opening a public
issue for anything exploitable. Include reproduction steps and affected version
(`symvibe version`).
