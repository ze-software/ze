# Spec: dhcp-learned-routes-table-and-next-hop

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds capabilities absent from the supported release: a DHCP-learned default route ranked by distance and placed in a chosen table, and a static route whose next hop follows a DHCP lease. The shipped DHCP default route keeps every current obligation. |
| RFC gaps | - |
| Depends | plan/post-release/features/spec-vrf.md (a learned route in a VRF's table needs the VRF device) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

Two operator needs, both common on CPE and dual-WAN boxes:

1. **The learned default goes where the operator says.** Into a named routing
   table (policy routing, a management table, a VRF), at an administrative
   distance that ranks it against static and protocol routes, or not at all
   while the lease is still used for the address.
2. **A static route whose next hop is "whatever router the DHCP lease on
   eth1 gave me".** The gateway changes when the lease changes, and the route
   goes away when the lease does. Today the operator must hard-code a gateway
   the ISP may change.

### VyOS reference

VyOS `interfaces ethernet <n> dhcp-options default-route-distance <d>`,
`dhcp-options no-default-route`, the interface's `vrf`, and
`protocols static route <p> dhcp-interface <ifname>` (include
`dhcp-interface-multi.xml.i`). T9278 (vyos-1x `599ebc0e2`, `e091ee722`)
fixed the DHCP client hooks that write these routes into a VRF table.

## Required Reading

- [ ] `docs/guide/static-routes.md`, `docs/architecture/static-routes.md`
- [ ] `docs/architecture/forked-route-install.md` - how a route producer reaches sysrib
- [ ] `docs/architecture/core-design.md`, System RIB - distance ranking
- [ ] `plan/post-release/features/spec-vrf.md` - VRF device and table isolation

## Current Behavior

- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - `applyV4Route` and `installV4Route` write `0.0.0.0/0` via the lease's router straight into the kernel main table with `iface.AddRoute(..., c.config.RouteMetric, rtproto.Iface)`. The route bypasses the Loc-RIB, so no distance ranks it and no table can be chosen.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - leaf `route-priority` (default 254) sets the kernel metric of learned defaults; there is no distance, no table and no "no default route" leaf for DHCP (PPP has `no-default-route`).

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| DHCPv4 default route install | `dhcp_v4_linux.go` `installV4Route`, `removeV4Route` | Direct kernel write, main table, `rtproto.Iface` | Replaces: the learned default becomes a Loc-RIB path of its own protocol with a distance, so sysrib ranks it (no second direct path kept, `ai/rules/no-layering.md`) |
| Learned-default metric | leaf `route-priority`, link-down +1024 | Kernel priority between Ze's own writes | Re-judged at design: once sysrib ranks by distance, state what the metric still means |
| Lease events | `iface.TopicDHCPLeaseAcquired` and the other lease topics | Announce lease changes on the bus | Reused: the static plugin subscribes to learn the router of a named interface's lease |
| Static next hops | static plugin `nextHop` (address, interface) | Fixed gateway or interface | Extends: a third next-hop form naming a DHCP interface, resolved from lease events, withdrawn while no lease is held |
| Named tables | `routing-table` registry, static named-table direct write | Static routes in named tables | Extends: the learned default can target a named table |
| PPP `no-default-route` | `ze-iface-conf.yang` | Suppresses the PPP default | The DHCP leaf reuses the same name and meaning |

## Data Flow

### Entry Point
DHCP client lease events; config under the unit's `dhcp` container and under `static`.

### Transformation Path
1. A lease names its router.
2. The DHCP client inserts or withdraws its default path with the configured distance and table, unless suppressed.
3. A static route with a DHCP-interface next hop resolves to the same router and is withdrawn while there is no lease.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface DHCP plugin to sysrib | Loc-RIB insert or route-install RPC | No |
| iface to static plugin | lease events on the EventBus | No |

### Integration Points
- DHCP client, sysrib, static plugin next-hop resolution, routing-table registry.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Lease acquired on eth1, static route with DHCP-interface eth1 | → | static resolution to the lease router | [Linux integration test with a DHCP server in a netns, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DHCP with a distance of 200 and a static default at 10 | The static default is in the FIB; the learned one is in the Loc-RIB only; the learned one takes over when the static one is withdrawn |
| AC-2 | DHCP with a named table | The learned default is in that table and not in main (kernel read-back) |
| AC-3 | DHCP with no default route | Address and resolvers installed, no default route anywhere |
| AC-4 | Static route 198.51.100.0/24 with a DHCP-interface next hop on eth1 | Installed via the lease's router; follows a router change at renew; withdrawn on lease loss and on unit disable |
| AC-5 | DHCP-interface names an interface with no DHCP client configured | Config verify refuses it, naming the interface and the leaf that enables DHCP |
| AC-6 | DHCPv6 and RA-learned defaults | Same distance, table and suppression leaves apply, or the design states the separate owner |
| AC-7 | The iface plugin runs forked | AC-1 to AC-4 hold |
| AC-8 | Existing DHCP functional and integration tests | Re-validated against the new path in the same change (`ai/META-DECISIONS.md`, a behavior change converts its users) |

No interop test beyond a standard DHCP server (the existing DHCP integration harness) is owed: the change is route placement, not wire.

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Moving the default into the Loc-RIB changes which writer owns the kernel route; `fib-kernel` reconciliation could fight leftovers tagged `rtproto.Iface` | Census every reader of `rtproto.Iface` routes at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | static plugin config tests | DHCP-interface next-hop parsing and refusal | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/static/`, `test/dhcp/` | Config parse and refusal paths | |

## Files to Modify

- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` and the DHCPv6 and RA equivalents
- `internal/component/iface/yang/ze-iface-conf.yang`
- `internal/plugins/static/` (next-hop form, YANG, lease subscription)
- `docs/guide/static-routes.md`, the DHCP client section of `docs/guide/configuration.md`

## Implementation Steps

1. Learned default through the Loc-RIB with distance, table and suppression.
2. DHCP-interface next hop in static.
3. Re-validate every DHCP test the repository carries.

### Integration Checklist
- [ ] A dual-WAN config with two DHCP uplinks ranks both defaults by distance

### Documentation Update Checklist
- [ ] Guide sections for the new leaves and the next-hop form

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test static`, `./le test functional dhcp`, `./le test integration iface`
