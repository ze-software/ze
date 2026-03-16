# 228 — Config Reload 8: Daemon RPCs to ze-system

## Objective

Move `daemon-reload`, `daemon-shutdown`, and `daemon-status` RPCs from the `ze-bgp` module to `ze-system`. Config reload notifies GR, hostname, RIB, and any plugin with `WantsConfigRoots` — it is not BGP-specific.

## Decisions

- No dual registration needed (Ze has no users) — update sender and receiver atomically in the same commit.
- `ze-system:shutdown` was an exact duplicate of `ze-bgp:daemon-shutdown` — deleted the duplicate rather than keeping an alias.
- The editor's `NewSocketReloadNotifier` updated from `ze-bgp:daemon-reload` to `ze-system:daemon-reload` as part of the same commit.

## Patterns

- `RPCRegistration.WireMethod` is the YANG-derived `module:rpc-name` on the wire; `CLICommand` is the human-readable dispatch text — these are separate and both must be updated when moving RPCs.
- Existing comprehensive `TestRPCRegistrationExpectedMethods` and `TestRPCRegistrationPerModule` tests cover wire method validation better than individual per-method tests would.
- Architecture docs (`wire-format.md`, `ipc_protocol.md`, `commands.md`) had stale examples referencing `ze-bgp:daemon-*` — always grep docs when renaming wire methods.

## Gotchas

- 19 files total needed updating: 7 source + 9 test/functional + 3 architecture docs. Scope was larger than the spec's initial "Files to Modify" list.
- `internal/ipc/method_test.go` and `internal/ipc/dispatch_test.go` had stale test examples with old wire method names — caught during critical review.
- YANG schema changes ripple through multiple test files that assert exact RPC lists and counts.

## Files

- `internal/component/plugin/bgp.go` — removed 3 daemon handler registrations
- `internal/component/plugin/system.go` — added 3 daemon handlers, removed duplicate `handleSystemShutdown`
- `internal/component/bgp/schema/ze-bgp-api.yang` — removed daemon RPC definitions
- `internal/ipc/schema/ze-system-api.yang` — added daemon RPCs, removed standalone shutdown
- `internal/component/config/editor/reload.go` — wire method updated to `ze-system:daemon-reload`
