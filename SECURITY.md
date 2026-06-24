# Security

## Threat model & defaults

symvibe runs **locally** and is a single-user developer tool.

- **Loopback only (default).** The board/API binds to `127.0.0.1` by default
  (`[server] host`). It is not authenticated — do not bind it to a public
  interface. Nothing in symvibe exposes opencode itself to the network.
- **The board can run code.** Pressing *Run* makes the backend execute an AI coding
  agent against your working tree (git operations, file edits, optionally `gh`
  calls for GitHub workflows, …). Treat the board like a terminal: only run it
  on repositories you trust. `gh` is optional; without it, GitHub-specific steps
  fail gracefully.

## Network access modes

`server.access` (env: `SYMVIBE_ACCESS`, CLI: `--access`) controls which
interfaces the board binds to and whether authentication is required.

| Mode      | Bind       | Auth required | TLS    | Use case                    |
|-----------|------------|---------------|--------|-----------------------------|
| loopback  | 127.0.0.1  | no            | no     | Local-only (default)        |
| lan       | 0.0.0.0    | **yes**       | **yes**| Same-network access (e.g. iPhone) |
| relay     | 0.0.0.0    | **yes**       | **yes**| Public/relay access         |

**Fail-closed:** `lan` and `relay` modes refuse to start unless `auth.enabled`
is `true`. Authentication uses Bearer tokens (see Pairing section). Without a
valid token, all `/api/*` and `/events` endpoints return 401.

## Pairing flow

Remote devices authenticate via the pairing protocol:

1. Run `symvibe pair` on the server to generate a one-time pairing code and
   display a QR code in the terminal.
2. The QR payload is `symvibe://pair?n=<hostname>&p=<port>&h=<host>&fp=<sha256>&c=<code>`.
3. The client scans the QR, calls `POST /api/pair/complete` with the code and a
   device name. The server returns a persistent Bearer token.
4. The client stores the token and sends it with every request via
   `Authorization: Bearer <token>` header or `?token=` query parameter.

Pairing codes are single-use and expire after 120 seconds.

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
