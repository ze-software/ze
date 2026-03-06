# 370 — Editor Reload Probe

## Objective
Add daemon socket probe at startup so `ze config edit` provides conditional reload after commit.

## Decisions
- `ReloadNotifier` is optional `func() error` on `*Editor` — set only when daemon socket is reachable
- Socket probe at startup in `cmd/ze/config/cmd_edit.go` — if socket responds to ping, set notifier
- Three-way commit message: committed + reloaded / committed + daemon not running / committed + reload failed

## Patterns
- `NewSocketReloadNotifier(socketPath)` returns a function that sends reload RPC
- Probe uses dial + close — lightweight check without full RPC handshake
- Standalone mode (no daemon): notifier not set, commit message says "daemon not running"

## Gotchas
- Socket path must match the running daemon's actual socket path
- Probe at startup means changes to daemon state during editing session aren't detected

## Files
- `cmd/ze/config/cmd_edit.go` — socket probe, conditional notifier setup
- `internal/component/config/editor/editor.go` — SetReloadNotifier, HasReloadNotifier
- `internal/component/config/editor/model_commands.go` — three-way commit message
