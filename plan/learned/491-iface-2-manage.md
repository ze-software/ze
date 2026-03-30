# 491 -- Interface Management + YANG + CLI

## Context

Ze could monitor OS interfaces (phase 1) but had no capability to create, delete, or configure them. Interface management was needed for DHCP, migration, and any scenario where Ze provisions its own network interfaces. This phase adds netlink-based interface lifecycle management, a YANG configuration schema (`ze-iface-conf`), and CLI commands (`ze interface`).

## Decisions

- YANG schema with `interface-physical` + `interface-unit` groupings over flat config: two-layer model mirrors JunOS IFD/IFL. Physical properties (MTU, description, disable) live on the interface; logical properties (addresses, VLAN, VRF, sysctl) live on units.
- CLI at `cmd/ze/interface/` with standard dispatch pattern over ad-hoc commands: `func Run(args []string) int` with `flag.NewFlagSet` per subcommand, matching existing `cmd/ze/bgp/` patterns.
- sysctl with testable `sysctlRoot` override over direct `/proc/sys` writes: unit tests can redirect sysctl writes to a temp directory without requiring root or `/proc` access.
- VyOS-aligned type-first grouping (ethernet, dummy, veth, bridge, loopback) over flat interface list: each type has its own YANG list, enabling type-specific constraints (e.g., veth requires peer name).

## Consequences

- Config validation happens at YANG level -- invalid MTU, VLAN ID out of range, or unknown interface type rejected before any netlink call.
- VLAN units create real OS subinterfaces (`<parent>.V`); non-VLAN units > 0 are logical grouping only (addresses go on parent).
- Phase 1 monitor detects all management changes automatically -- no special wiring needed between management and monitoring.

## Gotchas

- `validateIfaceName` initially checked only length (IFNAMSIZ=15). Needed character validation to prevent path traversal via sysctl writes (interface name appears in `/proc/sys/net/ipv4/conf/<name>/`). Added alphanumeric + hyphen + dot restriction.
- VLAN composite name (`<parent>.<vlan-id>`) can exceed IFNAMSIZ when parent is long. Validation must check the combined name length, not just the parent.
- Partial creation needs cleanup: if `LinkSetUp` fails after `LinkAdd` succeeds, the created-but-down interface must be deleted via `LinkDel`. Without cleanup, stale interfaces accumulate.
- sysctl for `accept_ra` must be `2` (not `1`) when `forwarding=true`, otherwise the kernel ignores Router Advertisements. This interaction is not obvious from either setting alone.

## Files

- `internal/component/iface/iface_linux.go` -- interface + unit create/delete/addr management
- `internal/component/iface/sysctl_linux.go` -- per-unit sysctl writes with testable root
- `internal/component/iface/schema/ze-iface-conf.yang` -- YANG config schema
- `cmd/ze/interface/` -- CLI: main.go, show.go, create.go, unit.go, addr.go
