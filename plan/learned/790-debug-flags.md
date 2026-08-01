# 790: Debug Flags

## Context

Runtime debug flags stored in ZeFS, inspired by VyOS filesystem debug flags
(`touch /tmp/vyos.ifconfig.debug`). Ze uses zefs keys instead of files, with
CLI commands to toggle and a three-tier resolution system.

## Decisions

- **Three-tier resolution:** global override (`state/debug/all`) > per-subsystem
  key (`state/debug/{subsystem}`) > default (off). Global "on" overrides all
  per-subsystem settings; global "off" or absent allows per-subsystem keys to take effect.
- **Hierarchical matching:** `debug enable bgp` covers all bgp.* subsystems
  (bgp.reactor, bgp.fsm, bgp.routes, etc.), mirroring the existing env var hierarchy.
- **Restore on disable:** re-derives level from `getLogEnv()` (env var / config),
  not from a stored "previous level". Config is the source of truth for non-debug levels.
- **Offline CLI command:** `ze debug` opens the zefs blob store directly (like `ze data`).
  Applies `SetLevel()` to the current process but cannot notify a running daemon.
  For immediate daemon effect, use the CLI session (online RPC path, future work).
- **`storage.BlobStoreFrom()` accessor:** added to expose the underlying `*zefs.BlobStore`
  from `storage.Storage` without exporting `blobStorage`. Used by the hub startup to
  pass the blob store to `slogutil.ApplyDebugFlags()`.
- **`DebugStore` interface:** matches `*zefs.BlobStore` (`ReadFile` + `List`), not
  `storage.Storage` (whose `List` returns `([]string, error)`). Keeps the resolution
  logic simple; an adapter would be needed if the hub ever drops blob storage.

## Consequences

- Debug flags persist across daemon restarts (stored in zefs).
- The `state/debug/` namespace is the first use of `state/` keys in zefs
  (existing keys use `meta/` and `file/`). Future runtime state keys should
  follow this prefix.
- `levelDebug` constant added to slogutil to satisfy goconst; all debug level
  string references within slogutil now use it.

## Gotchas

- The offline `ze debug enable` writes to zefs and calls `SetLevel()` on its own
  process. It does NOT notify a running daemon. The daemon picks up the change on
  next restart (via `ApplyDebugFlags` in hub startup). The RPC path for live daemon
  notification is not yet implemented.
- `ValidateSubsystem` accepts hierarchical prefixes (e.g., "bgp") even if "bgp"
  is not itself a registered subsystem, as long as "bgp.*" subsystems exist. This
  is intentional for the parent-enables-children pattern.

## Files

- `pkg/zefs/keys.go` -- `KeyDebugAll`, `KeyDebugSubsystem` registrations
- `internal/core/slogutil/debug.go` -- `ResolveDebugStates`, `ApplyDebugFlags`, `RestoreLevel`, `ValidateSubsystem`, `SubsystemsMatching`
- `internal/core/slogutil/debug_test.go` -- 9 unit tests for resolution logic
- `internal/plugins/debug/register.go` -- CLI command registration
- `internal/plugins/debug/debug.go` -- `Run`, `cmdEnable`, `cmdDisable`, `cmdShow`
- `internal/plugins/debug/debug_test.go` -- 9 unit tests for CLI commands
- `internal/component/config/storage/storage.go` -- `BlobStoreFrom()` accessor
- `cmd/ze/hub/main.go` -- `ApplyDebugFlags` call in Phase 1a of startup
- `cmd/ze/main.go` -- `zedebug` import + dispatch case
- `docs/guide/command-reference.md` -- `ze debug` section
- `docs/features.md` -- Debug Flags feature row
