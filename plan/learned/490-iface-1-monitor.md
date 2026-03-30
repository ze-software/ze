# 490 -- Interface Monitor Plugin

## Context

Ze had no interface event system. BGP could not react to address changes -- it assumed configured IPs existed at startup and never verified. The monitor plugin opens a netlink multicast socket on Linux, receives kernel events for link state and address changes, classifies them into hierarchical Bus topics, and publishes JSON payloads. This is the read-only foundation that all subsequent phases build on.

## Decisions

- Netlink multicast (async) over polling: subscribes to `RTMGRP_LINK`, `RTMGRP_IPV4_IFADDR`, `RTMGRP_IPV6_IFADDR` for immediate event delivery without periodic overhead.
- `sync.Map` for interface index tracking over regular map: needed to distinguish interface creation from state-change events (first-seen index = created, subsequent = state change).
- `isLinkUp` checks both `OperState == OperUnknown` and `IFF_UP` flag: virtual interfaces (dummy, veth) report `OperUnknown` even when administratively up, so `OperState == OperUp` alone misses them.
- VLAN subinterface resolution in monitor: OS events for `eth0.100` are resolved back to parent name + unit with matching `vlan-id` so Bus payloads carry Ze-level unit identifiers.

## Consequences

- All interface changes (external or Ze-managed) produce Bus events, enabling uniform BGP reactions regardless of who created the interface.
- Monitor is a long-lived goroutine (one per plugin lifetime), not per-event. Channel-based worker pattern per `goroutine-lifecycle.md`.
- IPv6 tentative addresses (DAD incomplete) are filtered out -- only confirmed addresses produce `addr/added`.

## Gotchas

- Original `handleLinkUpdate` had dead code: up/down branches were unreachable because the creation check consumed both cases. Fixed by restructuring to check creation first, then state separately.
- `stoppableContext` leaked goroutines when cancel was not returned to the caller. Required returning the cancel func so the monitor's stop path could invoke it.
- IPv4-mapped IPv6 addresses (e.g., `::ffff:10.0.0.1`) needed `Unmap()` before comparison or Bus payload encoding, otherwise matching against peer `LocalAddress` failed.
- Bus `cleanup()` deadlocked when `unsubscribeBus` was called while holding `r.mu`. Fix: move `unsubscribeBus` before `r.mu.Lock` in shutdown sequence.

## Files

- `internal/component/iface/iface.go` -- shared types, Bus topic constants, payload encoding
- `internal/component/iface/register.go` -- plugin registration via `init()`
- `internal/component/iface/monitor_linux.go` -- netlink multicast monitor goroutine
