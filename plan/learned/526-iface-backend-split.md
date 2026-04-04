# 526 -- Interface Backend Split

## Context

The iface component was a monolith: YANG schema, types, validation, and all OS operations (netlink management, monitoring, bridge, sysctl, mirror, DHCP) lived together in `internal/component/iface/`. The only separation was Go build tags. Adding a new backend (e.g., systemd-networkd, FreeBSD) would have required restructuring the entire component. Config was YANG-validated but never applied to the OS -- `OnConfigure` discarded the parsed config. The goal was to follow the FIB pattern (fibkernel/fibp4), split DHCP out, and wire config all the way to the OS.

## Decisions

- Backend interface (33 methods) in the component over a shared `pkg/` interface, because only the component dispatches to backends. Backends are not full SDK plugins -- they're called synchronously.
- `RegisterBackend`/`LoadBackend` registry in the component. `LoadBackend` closes the previous backend before creating a new one (prevents goroutine leaks on reload).
- Per-OS backend default via build-tagged constants (`default_linux.go`: "netlink"). Config `backend` leaf overrides.
- DHCP as a separate plugin (`ifacedhcp`) using `ReplaceAddressWithLifetime` and `RemoveAddress` from the dispatch layer -- zero netlink dependency in DHCP code.
- Declarative config application: desired state from config, diff against OS via `ListInterfaces`, four phases (create, properties/sysctl/mirror, reconcile addresses, delete stale).
- Config reload via verify/apply pipeline: `OnConfigVerify` parses and stores, `OnConfigApply` re-applies declaratively.
- Errors collected and returned (not silently logged). Single error shown on CLI status line, multiple errors via `errors` command which now shows both validation issues and reload errors.

## Consequences

- Future backends: implement `Backend`, call `RegisterBackend` in `init()`. No component changes.
- Config is now applied: interfaces created, addresses set, sysctl written, mirrors configured.
- Config reload re-reconciles declaratively: stale addresses removed, missing interfaces created, deleted interfaces cleaned up.
- Dispatch layer (~250 lines) preserves package-level API (`CreateDummy`, `AddAddress`, etc.) for callers and tests.
- Integration tests that override backend-internal variables (`sysctlRoot`, `bridgeSysfsRoot`) must live in the backend package. `withNetNS` calls `ensureBackendForIntegration` so all integration tests get a backend.

## Gotchas

- `LoadBackend` MUST close the previous backend. The original implementation silently overwrote `activeBackend`, leaking the old monitor goroutine. Found in deep review.
- Integration test `collectingBus` type was moved to the backend package but still needed by component monitor integration tests. Had to be duplicated. Found in deep review.
- `ensureBackendForIntegration` was only called by sysctl tests. 4 other integration test files (manage, monitor, mirror, migrate) silently failed. Fixed by calling it from `withNetNS`. Found in deep review.
- DHCP v4/v6 originally used `netlink.AddrReplace` with lifetime fields directly. Adding `ReplaceAddressWithLifetime` to the Backend interface let DHCP drop its netlink import entirely.
- `unparam` lint catches test helpers with parameters always receiving the same value. Adding a second test case with a different value (e.g., VLAN iface name) fixes it.
- Reload errors were silently logged. The CLI/web showed "reload failed: ..." as a one-line status but the `errors` command only showed YANG validation errors. Extended `errors` to show both.
- `// Detail:` refs in `iface.go` pointed at files that moved to the backend. Stale refs blocked the hook and needed cleanup.

## Files

- `internal/component/iface/backend.go` -- Backend interface + registry (33 methods, `LoadBackend` closes previous)
- `internal/component/iface/config.go` -- config parsing, declarative application, sysctl, mirror
- `internal/component/iface/dispatch.go` -- package-level functions delegating to backend
- `internal/component/iface/register.go` -- OnConfigure/OnConfigVerify/OnConfigApply, error propagation
- `internal/component/iface/default_linux.go` / `default_other.go` -- per-OS backend default
- `internal/plugins/ifacenetlink/` -- netlink backend (10 source files + 4 test files)
- `internal/plugins/ifacedhcp/` -- DHCP plugin (5 source files, zero netlink dependency)
- `internal/component/cli/model_commands.go` -- `errors` command shows reload errors, `tryReload` helper
- `internal/component/iface/schema/ze-iface-conf.yang` -- `backend` leaf
- `docs/features/interfaces.md`, `docs/guide/plugins.md`, `docs/architecture/core-design.md` -- updated
