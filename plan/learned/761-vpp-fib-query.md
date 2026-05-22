# 761 -- VPP FIB Query

## Context

`show ip route lookup <ip>` hardcoded netlink's RouteGet, bypassing the Backend interface entirely. When the VPP backend was active, operators got kernel FIB answers instead of VPP FIB answers. VPP's FIB is authoritative on that backend, so returning kernel data misled operators into thinking routes were missing or wrong. The goal was to move RouteLookup into the Backend interface so it dispatches correctly to whichever backend is active.

## Decisions

- Added `RouteLookup(dest netip.Addr) (map[string]any, error)` to the Backend interface over keeping the platform-specific bypass files, because the bypass pattern violated the Backend abstraction that every other iface operation uses.
- Used `IPRouteLookupV2` with `Exact=0` (LPM mode) over `IPRouteDump` with post-filter, because the VPP API does the longest-prefix-match in hardware and returns a single route.
- Returned `map[string]any` matching the existing JSON key schema over introducing a new struct, because the handler (`handleRouteLookup`) and all consumers already expect this shape.
- Reused `routeV2ToKernelRoute` and helpers from `fib.go` over duplicating conversion logic, because ListKernelRoutes already validated that path.
- Emptied `route_lookup_linux.go` and `route_lookup_other.go` over deleting them immediately, because `git rm` is forbidden from Bash. The commit script will remove them.

## Consequences

- Every Backend implementation (netlink, VPP, stub, test mocks) must now implement RouteLookup. Adding a new backend requires this method.
- The dispatch.go RouteLookup function is now platform-independent (no build tags), simplifying the package structure.
- Per-VRF route lookup is deferred: `IPRouteLookupV2.TableID` is hardcoded to 0. When VRF support lands, RouteLookup will need a table parameter.
- ECMP multi-path display shows only the first path, matching ListKernelRoutes behavior.

## Gotchas

- Three test mocks needed RouteLookup stubs (`fakeBackend` in config_test.go, `mockMigrateBackend` in migrate_linux_test.go, `stubBackend` in backend_other.go). The `stubBackend` in rate_test.go uses embedding so it inherited the method automatically but would panic if called.
- The mock channel's `ReceiveReply` needed a type switch to handle both `SwInterfaceClearStatsReply` and `IPRouteLookupV2Reply`, not just an if/ok check.

## Files

- `internal/component/iface/backend.go` -- added RouteLookup to Backend interface
- `internal/component/iface/dispatch.go` -- added package-level RouteLookup dispatch
- `internal/component/iface/route_lookup_linux.go` -- emptied (logic moved to netlink backend)
- `internal/component/iface/route_lookup_other.go` -- emptied (logic moved to Backend stubs)
- `internal/plugins/iface/vpp/fib.go` -- VPP RouteLookup via IPRouteLookupV2
- `internal/plugins/iface/vpp/fib_test.go` -- 4 tests: hit, miss, IPv6, channel-not-ready
- `internal/plugins/iface/netlink/route_linux.go` -- moved netlink RouteLookup here
- `internal/plugins/iface/netlink/backend_other.go` -- added RouteLookup stub
- `internal/component/iface/config_test.go` -- added RouteLookup stub to fakeBackend
- `internal/component/iface/migrate_linux_test.go` -- added RouteLookup stub to mockMigrateBackend
- `docs/features/interfaces.md` -- added RouteLookup to Backend method table
