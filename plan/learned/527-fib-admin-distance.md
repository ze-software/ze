# 527 -- Wire Admin Distance Config Into Route Selection

## Context

The `fib { admin-distance { } }` YANG config block existed with defaults (ebgp=20, ibgp=200, static=10, etc.) but nothing read these values at runtime. sysrib trusted incoming priority from Bus events verbatim. Additionally, the BGP RIB tagged all routes as "bgp" with no distinction between eBGP and iBGP, making per-type admin distance impossible.

The goal: make admin distance configurable and applied at the system RIB level, where cross-protocol route selection happens.

## Decisions

- Moved admin-distance from `fib { }` to `sysrib { }` YANG config, over keeping it under fib. Admin distance is a RIB concept (route selection), not FIB (forwarding). Every major NOS (Cisco, Junos, Arista, FRR) ties it to the RIB.
- Added per-change `protocol-type` JSON field ("ebgp"/"ibgp") over changing batch-level `metadata["protocol"]`. A single batch can contain winners from both eBGP and iBGP peers, so batch-level metadata cannot distinguish them.
- Routes map key stays "bgp" (one BGP route per prefix), over using "ebgp"/"ibgp" as separate keys. Separate keys would cause stale entries when best path switches between eBGP/iBGP peers.
- sysrib overrides priority via `effectivePriority()`, over having BGP RIB read sysrib config. Admin distance is sysrib's concern; protocol RIBs keep their hardcoded defaults.

## Consequences

- New protocols (OSPF, ISIS) just need to set `protocol-type` in their Bus events; sysrib handles the rest.
- Config reload is fully supported: OnConfigVerify/OnConfigApply + reapplyAdminDistances recalculates all stored route priorities and recomputes best paths.
- The `protocol-type` field is optional; absent means sysrib falls back to batch-level protocol metadata, preserving backward compatibility.
- `protocolRoute` stores both `incomingPriority` and `protocolType` so reload can recompute effective priority without re-receiving events.

## Gotchas

- A single BGP best-change batch can contain winners from different peer types (the triggering UPDATE is from one peer, but best-path recalculation picks winners across all peers). Batch-level metadata cannot carry per-change protocol type.
- `ze-fib-conf.yang` still exists with kernel-only config. Tests referencing the old admin-distance path need updating if they existed.

## Files

- `internal/plugins/sysrib/sysrib.go` -- parseAdminDistanceConfig, effectivePriority, processEvent override
- `internal/plugins/sysrib/register.go` -- ConfigRoots: ["sysrib"], OnConfigure
- `internal/plugins/sysrib/schema/ze-sysrib-conf.yang` -- admin-distance container
- `internal/plugins/fibkernel/schema/ze-fib-conf.yang` -- admin-distance removed
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- ProtocolType field, protocolType() helper
