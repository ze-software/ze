# 024 — Session Commands

## Objective

Implement per-process session state commands (`ack enable/disable/silence`, `sync enable/disable`, `reset`, `ping`, `bye`) that control API response behavior for individual process connections.

## Decisions

- Session state (ack, sync) is per-process, not per-command or per-socket — Ze uses Process-based API (stdin/stdout pipes), not socket connections, so state attaches to the `Process` struct.
- `ack disable` sends "done" for the disable command itself, then suppresses future responses — the current command still gets acknowledged before disabling.
- `ack silence` suppresses response for the silence command itself too — no response at all, immediate transition.
- `sync enable` makes response wait for wire transmission before ACKing; `sync disable` (default) ACKs immediately after RIB update.
- `ping` returns `pong <uuid>` where uuid is the daemon's UUID — health check with identity.

## Patterns

- Session commands match ExaBGP's command syntax exactly for compatibility.
- `CommandContext` already had a `Process *Process` pointer — session state methods added to `Process` directly.

## Gotchas

- None documented.

## Files

- `internal/plugin/process.go` — SetAck(), SetSync(), AckEnabled(), SyncEnabled() on Process
- `internal/plugin/handler.go` — 8 session command handlers + registration
- `internal/plugin/session_test.go` — session state and command tests
