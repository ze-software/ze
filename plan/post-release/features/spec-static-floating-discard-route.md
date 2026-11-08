# Spec: static-floating-discard-route

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: one static prefix carrying both a forward alternative and a discard alternative at different distances. Existing single-action routes keep every current obligation. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

An operator wants a floating null route: a static route toward a gateway at a
good distance, and a blackhole (or reject) route for the SAME prefix at a worse
distance. While the gateway route is usable it wins. When its next hop goes away
(link down, BFD down, gateway unresolvable), the discard route takes over, so
traffic for the prefix is dropped locally instead of following a less specific
route (often the default route) and looping or leaking.

The usual shapes: an aggregate Ze announces in BGP that must stay in the FIB
when the internal route behind it fails, and a customer prefix that must be
dropped, not sent upstream, while the customer link is down.

Ze cannot express this today. One table holds one `route` entry per prefix,
and that entry holds exactly one case of `choice action`.

### VyOS reference

VyOS writes `protocols static route <p> next-hop <gw> distance <n>` and
`protocols static route <p> blackhole distance <m>` under the same prefix.
T9482 (vyos-1x `39507a51f`, `f819e1492`) found that FRR's staticd refuses a
blackhole and a next hop at the SAME distance for one prefix, and VyOS now
drops the blackhole in that case. Ze defines that case itself (AC-5) and never
drops an alternative in silence.

## Required Reading

- [ ] `docs/architecture/static-routes.md` - main-table routes through the Loc-RIB, named tables written directly
- [ ] `docs/guide/static-routes.md` - the operator surface
- [ ] `docs/architecture/core-design.md`, System RIB - cross-protocol arbitration by distance
- [ ] `plan/post-release/features/spec-static-routes-take-the-bgp-group-shape.md` - the merge trap around `choice action`

## Current Behavior

- [ ] `internal/plugins/static/yang/ze-static-conf.yang` - `list route` is keyed by `prefix` alone ("one table holds one entry for each prefix"); `choice action` has three cases, `forward` (`next`), `discard` (`blackhole`), `unreachable` (`reject`), one per entry
- [ ] `internal/plugins/static/model.go` - `staticRoute` carries one `Action`, one `NextHops` slice and one optional `Distance` (`HasDistance`); the per-route `distance` leaf already overrides `rib { distance { static } }`
- [ ] `internal/plugins/static/config.go` - `parseRoute` reads one action per entry, tests `blackhole` first and returns on the first hit
- [ ] `internal/plugins/static/locrib.go` - `insertPathLocked` and `removePathLocked` key a main-table route by family, prefix, `staticevents.ProtocolID` and one fixed `pathInstance`, so a second static path for the same prefix would replace the first

Named-table routes do not enter the Loc-RIB; they are written directly, so no
arbitration exists there today.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Per-route distance | `ze-static-conf.yang` leaf `distance`; `staticRoute.Distance` | Ranks the one entry against other protocols | Extends: each alternative of one prefix carries its own distance |
| Loc-RIB arbitration | `internal/core/rib/locrib`; sysrib `effectivePriority` | Picks the best path per prefix across protocols by distance | Extends: two static paths for one prefix coexist under distinct instances and are ranked by their own distance |
| Static next-hop liveness (BFD, link, resolvability) | static plugin link tracking, `checkNextHopResolvable` | Withdraws a forward route whose next hops are all down | Reused: the withdrawn forward path is what lets the discard path win |
| Group shape | `spec-static-routes-take-the-bgp-group-shape.md` | Proposes groups; keeps `choice action` on the member | Does not apply directly. It found that an entry holding both `next` and `blackhole` silently becomes a blackhole; this spec must not reintroduce that merge |
| Kernel route priority | leaf `metric` | Sets the netlink priority | Does not decide main-table ranking; the named-table design says whether metric or distance separates the two kernel routes |

## Design questions owed

1. **Config shape.** Either the list key widens (prefix plus a discriminator),
   or `blackhole` and `reject` become siblings of `next`, each with its own
   `distance`, so one entry holds a forward and a discard alternative. The
   second matches VyOS and FRR and keeps the prefix the key. The current choice
   must not be kept with a merge across it, because `parseRoute` resolves a
   mixed entry by test order and drops data silently.
2. **Loc-RIB identity.** Each alternative needs its own path instance, and the
   withdrawal of one must not remove the other.
3. **Named tables.** The direct-write path has no arbitration. Either named
   tables gain a local selection step, or the discard alternative is written as
   a second kernel route at a worse kernel priority. The answer holds on VPP
   too, or the VPP backend refuses the config at verify.
4. **Redistribution.** A redistribute consumer sees the forward case alone
   today. State what is redistributed while the discard alternative is active.

## Data Flow

### Entry Point
Config `static { table <t> { route <p> { ... } } }` through the static plugin's `OnConfigVerify` and `OnConfigApply`.

### Transformation Path
1. `parseRoute` builds one value per alternative.
2. Each alternative becomes a Loc-RIB path with its own instance and distance.
3. sysrib selects; the FIB plugin programs the winner.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| static plugin to sysrib | Loc-RIB insert in process, or the route-install RPC when forked | No |

### Integration Points
- Loc-RIB path instance per alternative; named-table direct write; VPP static backend.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with a gateway and a blackhole alternative for one prefix | → | static apply, Loc-RIB insert of both | [functional `.ci` test, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One prefix with a gateway alternative at distance 1 and a blackhole alternative at distance 250, gateway reachable | The FIB holds the gateway route; `show static route` lists both alternatives and marks the active one |
| AC-2 | Same config, the gateway link goes down (or its BFD session goes down) | The FIB holds a blackhole route for the prefix; a Linux integration test reads the kernel route type |
| AC-3 | The gateway comes back | The gateway route is reinstalled and the blackhole route leaves the FIB |
| AC-4 | The same with `reject` in place of `blackhole` | An unreachable route takes over and ICMP unreachable is returned |
| AC-5 | Forward and discard alternatives at the SAME distance | Config verify refuses it, naming the prefix, both distances and the leaf to change |
| AC-6 | The same pair in a named table | The same take-over holds, proven against the kernel |
| AC-7 | Another protocol offers the prefix at a distance between the two | The other protocol's route wins over the discard alternative once the gateway fails |
| AC-8 | A reload removes the discard alternative only | The forward route stays installed with no flap |
| AC-9 | VPP backend | The same behavior proven against VPP, or a verify-time refusal naming the leaf |

No interop test is owed: static routes have no protocol peer.

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A widened config shape breaks every existing static config in the repository | Convert them in the same change (`ai/META-DECISIONS.md`, pre-release config syntax) |
| R-2 | Two static paths for one prefix confuse redistribution and `show` consumers that assume one | Census every reader of static paths at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/plugins/static/config_test.go` | Parsing of two alternatives, refusal of equal distances | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/static/` | Floating blackhole takes over and gives back | |

## Files to Modify

- `internal/plugins/static/yang/ze-static-conf.yang`
- `internal/plugins/static/config.go`, `model.go`, `locrib.go`
- `docs/guide/static-routes.md`, `docs/architecture/static-routes.md`

## Implementation Steps

1. Settle design questions 1 to 4.
2. Tests first, then the YANG and parser, then the Loc-RIB identity, then named tables and VPP.

### Integration Checklist
- [ ] Every repository config that uses static routes still parses

### Documentation Update Checklist
- [ ] `docs/guide/static-routes.md` gains the floating null route

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test static` and the static Linux integration package
