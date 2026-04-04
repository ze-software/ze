# 526 -- Interface Backend Split

## Context

The iface component was a monolith: YANG schema, types, validation, and all OS operations (netlink management, monitoring, bridge, sysctl, mirror, DHCP) lived together in `internal/component/iface/`. The only separation was Go build tags (`_linux.go` / `_other.go`). Adding a new backend (e.g., systemd-networkd, FreeBSD) would have required restructuring the entire component. The goal was to follow the FIB pattern (fibkernel/fibp4): component owns configuration and orchestration, backends handle OS-specific operations.

## Decisions

- Backend interface in the component (`iface/backend.go`) over a shared `pkg/` interface, because only the component dispatches to backends. 32 methods covering lifecycle, address, link state, properties, query, bridge, sysctl, mirror, and monitoring.
- Backend registry (`RegisterBackend`/`LoadBackend`) in the component over registry.Registration fields, because backends are not full plugins with their own SDK lifecycle -- they're called synchronously by the component.
- DHCP as a separate plugin (`ifacedhcp`) over keeping it in the component, because DHCP is a protocol client with its own goroutine lifecycle, not a backend concern. It uses netlink directly for lifetime-aware address operations (AddrReplace with ValidLft/PreferedLft) that the Backend interface doesn't cover.
- Dispatch functions in component over requiring callers to use `GetBackend()` directly, because many test files and CLI commands call package-level functions like `CreateDummy(name)`. The dispatch layer preserves this API while routing through the backend.
- `RemoveMirror` added to Backend interface (not just `SetupMirror`) because mirror cleanup is a real operation needed by tests and config application.
- YANG `backend` leaf with default `"netlink"` over explicit selector, because existing configs work unchanged.

## Consequences

- Future backends (networkd, FreeBSD, container) can be added by implementing the Backend interface and calling `RegisterBackend` in `init()`. No iface component changes needed.
- DHCP is independently loadable -- non-DHCP deployments don't load the DHCP code.
- Integration tests that override backend-internal variables (`sysctlRoot`, `bridgeSysfsRoot`) had to move to the backend package. Component-level tests use the dispatch layer.
- The dispatch layer means the component has ~250 lines of thin delegation functions. These are not identity wrappers (they add nil-backend error checking) but are mechanical.

## Gotchas

- `RegisterBackend` must be called before `LoadBackend`. The netlink package's `init()` registers, then the component's `OnConfigure` loads. Import order matters -- `all/all.go` must import `ifacenetlink` before the engine starts.
- DHCP v4/v6 files use `netlink.AddrReplace` with lifetime fields directly -- the Backend interface's `AddAddress(name, cidr)` is too simple for DHCP lease installation. A future `ReplaceAddressWithLifetime` method would clean this up.
- Monitor tests that call `handleLinkUpdate` directly must be in the backend package (unexported method). The component's `Monitor` type is now a thin wrapper around `StartMonitor`/`StopMonitor`.
- `unparam` lint catches test helpers with parameters that are always the same value across tests. Adding a second test case with a different value (e.g., VLAN interface name) fixes it.

## Files

- `internal/component/iface/backend.go` -- Backend interface + registry
- `internal/component/iface/dispatch.go` -- package-level functions delegating to backend
- `internal/component/iface/validate.go` -- exported ValidateIfaceName + validation helpers
- `internal/plugins/ifacenetlink/` -- netlink backend (10 source files)
- `internal/plugins/ifacedhcp/` -- DHCP plugin (5 source files)
- `internal/component/iface/schema/ze-iface-conf.yang` -- added `backend` leaf
