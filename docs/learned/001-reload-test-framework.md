# 001 — Reload Test Framework

## Objective

Implement config reload via SIGHUP and extend the .ci test framework with signal, sleep, and tmpfs-update commands to enable deterministic reload testing.

## Decisions

- Used a callback pattern (`ReloadFunc`) instead of importing config directly from the reactor, because config → reactor is the allowed direction; reactor cannot import config (circular dependency).
- `ReloadFunc` returns full `*PeerSettings` (not a thin struct) to reuse the existing `configToPeer()` conversion and avoid duplication.
- Removed `ReloadPeerConfig` struct that was planned — full `PeerSettings` is simpler and avoids a translation layer.
- Implemented settings change detection (planned as "Future") since the diff algorithm was trivial and the omission would have silently ignored peer reconfiguration.

## Patterns

- Reactor stores `configPath` + a `ReloadFunc` callback; `StartWithContext()` wires SIGHUP → `Reload()`.
- `errors.Join()` used to collect AddPeer errors without aborting the entire reload.

## Gotchas

- SIGHUP handler was declared in `signal.go` but never wired into `StartWithContext()` — the callback existed but was never called.
- Phase 2 (.ci framework extensions: `cmd=signal:`, `cmd=sleep:`, `cmd=tmpfs-update:`) was deferred; no functional reload test was created. Core reload logic is implemented but untested end-to-end.
- Wrong error returned when `reloadFn` was nil (was `ErrNoConfigPath`, should be `ErrNoReloadFunc`).

## Files

- `internal/reactor/reactor.go` — `Reload()`, `SetReloadFunc()`, `SetConfigPath()`
- `internal/reactor/reload_test.go` — 8 unit tests for reload
- `internal/component/config/loader.go` — `CreateReactorWithPath()`, `createReloadFunc()`
