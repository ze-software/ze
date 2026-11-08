# Spec: bgp-evpn-instance-route-targets

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: EVPN instances (EVI) with Route-Target import and export, including a `both` form. Ze carries EVPN routes as a BGP speaker and reflector today; the PE procedures of RFC 7432 that need an instance stay owed gaps (`rfc/short/rfc7432.md`). |
| RFC gaps | - |
| Depends | plan/post-release/features/spec-vrf-0-umbrella.md (phase 9 for the IP-VRF side of a symmetric IRB instance) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

An EVPN PE groups its MAC and IP routes into EVPN instances. Each instance
has a Route Distinguisher and Route-Targets: routes it originates carry the
export RTs, and received routes whose RTs match an import RT are imported into
it. Operators nearly always use the same RT for both, so every implementation
offers a `both` shorthand.

Ze decodes and encodes all EVPN route types and reflects them, but it has no
EVPN instance: no RD or RT per instance, no RT-based import, and so no place
for an RT `both` setting to apply.

### VyOS reference

VyOS `protocols bgp address-family l2vpn-evpn vni <n> route-target
import|export|both <rt>` and the per-VRF `l2vpn-evpn route-target both`.
T9450 (vyos-1x `83c42081d`) renders `both` as one import plus one export line.
FRR parses it in `parse_rtlist` (`bgpd/bgp_evpn_vty.c`, `RT_TYPE_BOTH` sets the
import and the export list). Ze's config should store the two lists, with
`both` a way of writing them, so `show` and a later edit of one direction
never disagree with what is applied.

## Required Reading

- [ ] `rfc/short/rfc7432.md` - which PE procedures Ze lacks
- [ ] `docs/architecture/wire/nlri-evpn.md` - the EVPN NLRI plugin
- [ ] `plan/post-release/features/spec-bgp-evpn-route-type-match.md` - route-type policy, a separate need
- [ ] `plan/post-release/features/spec-vrf-0-umbrella.md` - phase 9, the L3VPN RT model to share

## Current Behavior

- [ ] `internal/component/bgp/plugins/nlri/evpn/types.go` - the EVPN NLRI plugin models the route types (RFC 7432, RFC 9136); it holds no instance.
- [ ] `rfc/short/rfc7432.md` - records that checking the Type 4 fields "does not implement segment membership, automatic import rules or DF election".

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| EVPN NLRI codec | `internal/component/bgp/plugins/nlri/evpn` | Encodes and decodes route types 1 to 5 | Reused: the instance originates and imports through it |
| Extended community codec | `internal/core/bgp/attribute` | Reads and writes Route-Target communities | Reused for import matching and export stamping |
| L3VPN RT model | `spec-vrf-0-umbrella.md` phase 9 (planned) | Per-VRF RD, import and export RTs, `both` | Shared: one RT import/export model for L3VPN and EVPN, not two |
| Route reflection | `internal/component/bgp/plugins/rr` | Reflects EVPN routes | Unchanged |

## Design questions owed

1. **Data plane.** An EVI is useful only with a forwarding plane: VXLAN (RFC
   8365) or MPLS. Name it, its interface model and its owner spec before an AC
   about forwarding is final. A control-plane-only EVI must not be announced as
   EVPN support.
2. **Auto-derived RT.** FRR derives `ASN:VNI` when no RT is set. Decide whether
   Ze does, from RFC 8365 Section 5.1.2.1, and state it.

## Data Flow

### Entry Point
Config of an EVPN instance (VNI or EVI id, RD, RT import, export, or both); received EVPN UPDATEs.

### Transformation Path
1. Locally learned MAC and IP entries become routes carrying the instance's RD and export RTs.
2. Received routes whose RTs intersect the import set enter the instance.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP to the instance | RIB events or a plugin of its own, decided at design | No |

### Integration Points
- EVPN NLRI plugin, bgp-rib, the shared RT model, the data plane chosen in question 1.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| EVI config with RT both | → | export stamping and import matching | [functional `.ci` with hex UPDATEs, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Instance with `both 65000:100` | Originated routes carry RT 65000:100; received routes carrying it are imported; hex asserted |
| AC-2 | Instance with import 65000:1 and export 65000:2 | Each direction uses its own list |
| AC-3 | `both` written, then the export list edited | The config shows two lists and applies exactly what it shows |
| AC-4 | Received route with no matching RT | Not imported into any instance; still reflected if Ze is a reflector |
| AC-5 | Interop: Ze and FRR with `route-target both` on the same VNI | Type 2 and Type 3 routes are exchanged and imported on both sides; proven red with the import check reverted |
| AC-6 | Forwarding over the data plane question 1 names | Traffic crosses the EVI in a lab; owned here or by the named spec |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A second RT model beside L3VPN's | One model, shared with phase 9 of the VRF umbrella |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | the instance package | RT intersection and export stamping | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/plugin/` | Import and export through the real daemon | |

## Files to Modify

- The instance plugin or package chosen at design, its YANG
- `docs/guide/configuration.md` EVPN section

## Implementation Steps

1. Answer the design questions; settle the shared RT model with the VRF umbrella.
2. Instance config, export stamping, import matching.
3. FRR interop scenario (extends `test/interop/scenarios/bgp-evpn-frr`).

### Integration Checklist
- [ ] Reflector behavior for EVPN unchanged

### Documentation Update Checklist
- [ ] Guide section for EVPN instances

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test functional plugin`, `./le test integration interop` with `bgp-evpn-frr`
