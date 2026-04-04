# 489 -- Interface Management (Umbrella)

## Context

Ze had no interface lifecycle management. BGP assumed configured IPs always existed and could not react to address availability changes. Make-before-break IP migration was not possible -- removing an old IP required manually ensuring BGP had already bound the new one. The iface spec set (phases 1-4) adds OS interface monitoring, management, BGP reactions, and advanced features (DHCP, migration, mirroring) as a cohesive capability layered on the Bus.

## Decisions

- JunOS-style two-layer units over VyOS flat model: physical/logical split, unit 0 = parent, VLAN units create OS subinterfaces via netlink. Non-VLAN units > 0 are logical groupings only (no OS subinterface).
- Plugin at `internal/component/iface/` over subsystem: iface is cross-cutting infrastructure, not BGP-specific. The "delete the folder" test confirms it should survive independent of BGP.
- Bus-mediated communication over direct coupling: BGP never imports the iface plugin. All coordination flows through hierarchical Bus topics under `interface/`.
- `vishvananda/netlink` over shell commands: pure Go netlink for all interface operations (create, delete, addr, mirror). No `exec.Command("ip", ...)`.
- `insomniacslk/dhcp` for DHCP client: BSD-3-Clause, standard Go DHCP library, supports both DHCPv4 and DHCPv6 with Prefix Delegation.
- 5-phase migration protocol with strict ordering: new interface created, IP added, BGP confirms ready on Bus, only then old IP removed. Phase 4 blocked until phase 3 confirmation.

## Consequences

- All future protocol consumers (not just BGP) can subscribe to `interface/` Bus topics for address awareness.
- Linux-only initially via `_linux.go` suffixes; macOS/BSD can be added without restructuring.
- Migration safety depends on BGP publishing `bgp/listener/ready` -- any subsystem using migration must publish an equivalent readiness signal.
- DHCP-acquired addresses flow through the same `addr/added`/`addr/removed` path as static addresses, so BGP reacts identically to both.

## Gotchas

- VLAN composite names (e.g., `eth0.4094`) can exceed Linux IFNAMSIZ (15 chars) -- validation must check the combined name, not just the parent.
- IPv6 DAD must complete before publishing `addr/added` -- the `IFA_F_TENTATIVE` flag must be checked.
- Monitor must resolve VLAN subinterface names back to Ze unit identifiers for correct Bus event payloads.
- sysctl `accept_ra` must be set to `2` (not `1`) when `forwarding=true` to still accept Router Advertisements.

## Files

- `internal/component/iface/` -- plugin core (iface.go, register.go, monitor_linux.go, iface_linux.go, sysctl_linux.go, dhcp_linux.go, mirror_linux.go)
- `internal/component/iface/schema/ze-iface-conf.yang` -- YANG config schema
- `internal/component/bgp/reactor/reactor_iface.go` -- BGP interface event handler
- `cmd/ze/iface/` -- CLI subcommands (show, create, addr, unit, migrate)
