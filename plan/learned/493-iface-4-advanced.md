# 493 -- DHCP, Migration, Mirroring, SLAAC

## Context

Ze could monitor and manage interfaces (phases 1-2) and BGP could react to address events (phase 3), but there was no DHCP client for dynamic address acquisition, no orchestrated migration protocol for make-before-break IP moves, and no traffic mirroring capability. This phase adds DHCPv4/v6 clients, a 5-phase migration orchestrator, tc-based traffic mirroring, and SLAAC sysctl control.

## Decisions

- DHCPv4 via `nclient4`, DHCPv6 via `nclient6.RapidSolicit` over custom implementations: `insomniacslk/dhcp` is the standard Go DHCP library. Known limitation: DHCPv6 `RapidSolicit` does not support proper Renew -- leases must re-solicit on expiry.
- `TC_ACT_PIPE` for mirror action over `TC_ACT_STOLEN`: PIPE continues packet processing after mirroring, so the original traffic path is unaffected. STOLEN would consume the packet.
- 5-phase migration with Bus-based BGP readiness confirmation over timer-based: the orchestrator subscribes to `bgp/listener/ready` and only proceeds to phase 4 (remove old IP) after BGP confirms sessions are established on the new address. Timeout aborts cleanly.
- DHCP client as long-lived goroutine per interface over per-lease: follows `goroutine-lifecycle.md`. One goroutine handles discover/solicit, renew, rebind, and expiry for the interface's lifetime.
- SLAAC via kernel sysctl only (no RA sender): Ze is a BGP daemon, not a router advertisement daemon. `autoconf=1` enables kernel-native SLAAC; monitor detects resulting addresses.

## Consequences

- DHCP-acquired addresses produce both DHCP-specific events (`dhcp/lease-acquired`) and standard events (`addr/added`), so BGP reacts identically to DHCP and static addresses.
- Migration abort at any phase cleans up the new interface -- the old interface is never touched until BGP readiness is confirmed.
- Mirror requires `CAP_NET_ADMIN`; DHCP requires `CAP_NET_RAW`. These capabilities must be documented in deployment guides.

## Gotchas

- `renewV4`/`renewV6` initially discarded the new lease data returned by the renewal call, causing stale timers after renewal. The renewed lease must replace the old one to get correct T1/T2 values.
- IA_PD parsing loop was initially uncapped -- a malicious DHCP server could send unbounded prefix delegations. Added a cap (max 16 prefixes per solicit).
- `stoppableContext` needed to return its cancel function to prevent goroutine leak -- same issue as phase 1 monitor, recurring pattern.
- Migration phase 5 incorrectly used the `new-type` config flag to decide whether to delete the old interface. The correct check is whether the old interface is Ze-managed (has `managed=true` in state), not what type the new interface is.

## Files

- `internal/component/iface/dhcp_linux.go` -- DHCPv4/v6 client with lease lifecycle
- `internal/component/iface/mirror_linux.go` -- tc mirred setup/teardown via netlink
- `cmd/ze/interface/migrate.go` -- `ze interface migrate` CLI handler
- `internal/component/iface/schema/ze-iface-conf.yang` -- DHCP, mirror, SLAAC YANG sections
