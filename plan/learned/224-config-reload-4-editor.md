# 224 — Config Reload 4: Editor Integration

## Objective

Make the config editor (`ze config edit`) trigger a live reload when the user commits changes, without requiring a PID file or SIGHUP.

## Decisions

- Socket RPC used instead of SIGHUP (spec originally suggested `syscall.Kill`) — socket RPC requires no PID file discovery, delivers a proper error response, and reuses the existing server-side reload handler.
- `commit-confirm` and `abort` operations ALSO trigger reload — not in the original spec scope but discovered during critical review: these operations also change the committed config and must propagate to the running daemon.

## Patterns

- Prefer RPC over signals for inter-process communication when a socket is already available — RPC gives bidirectional feedback (success/error), signals do not.
- Critical review during implementation is valuable: discovered that `commit-confirm`/`abort` needed the same treatment as `commit`, preventing a correctness gap.

## Gotchas

- The spec only mentioned `commit`. Implementing it without also doing `commit-confirm` and `abort` would have left those operations in a state where the editor and daemon were out of sync.

## Files

- `cmd/ze/config/cmd_edit.go` — editor commit triggering socket reload
- `internal/component/config/editor/reload.go` — NewSocketReloadNotifier
