# Spec: ospfv3-external-path-preference

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped OSPFv3 selects AS-external paths with the RFC 1583 rules, while RFC 5340 Section 4.8.5 requires the Section 16.4.1 rules of RFC 2328. A wrong exit choice can loop traffic in a multi-area domain. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

RFC 5340 Section 4.8.5 (`rfc/full/rfc5340.txt`): "AS Boundary Router (ASBR) and
forwarding address selection will proceed the same as if RFC1583Compatibility
is disabled. Furthermore, RFC1583Compatibility is not an OSPF for IPv6
configuration parameter."

Ze's OSPFv3 engine shares the OSPFv2 external computation, which implements
only the "enabled" (RFC 1583) behavior. So OSPFv3 never applies RFC 2328
Section 16.4.1: "Intra-area paths using non-backbone areas are always the most
preferred." It picks the least-cost ASBR entry across areas instead, and it
never breaks a tie between external paths on the intra-AS path type (Section
16.4 step (6)(c)). FRR's ospf6d follows RFC 5340 here, so Ze and FRR can choose
different exits for the same destination: the loop RFC 2328 Appendix C.1 says
the 16.4.1 rules prevent.

This spec builds the 16.4.1 rules once, in the shared computation, and turns
them on for OSPFv3 unconditionally. `plan/post-release/features/spec-ospf-rfc1583-compatibility.md`
then exposes the same rules to OSPFv2 behind the RFC1583Compatibility leaf.

Found while specifying the VyOS T9441 parity item (vyos-1x `a57ab8277`).

## Required Reading

- [ ] `docs/architecture/ospf/ospf-8-spf-rib.md` - SPF to Loc-RIB
- [ ] `rfc/short/rfc5340.md` - no row covers Section 4.8.5 yet; add one with `/ze-rfc` (RFC 5340 is enrolled)
- [ ] `rfc/full/rfc2328.txt` Sections 16.4 and 16.4.1; `rfc/full/rfc3101.txt` Section 2.5 step (3)

## Current Behavior

- [ ] `internal/plugins/ospf/spf/external.go` - `ComputeExternalWith` is documented as shared by the OSPFv3 strategy. Its ASBR loop keys every Type-5-capable area under the backbone and keeps the least-cost entry, ties to the largest Area ID, with no path-type pruning. `betterExternal` never compares intra-AS path types.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Shared external computation | `spf/external.go` `ComputeExternalWith` | RFC 1583 rules for both versions | Extends: a 16.4.1 mode, selected by the caller; OSPFv3 always passes it |
| Border-router entries | `BorderRouterEntry` (`AreaID`, `Metric`, path type) | Carry what 16.4.1 needs per area | Reused: pruning reads the entry's area and intra/inter type |
| RFC 3101 source preference | `betterExternal` | Type-7 against Type-5 tie-break | Unchanged, and ordered after step (6)(c) |

## Data Flow

### Entry Point
SPF run of an OSPFv3 instance.

### Transformation Path
1. ASBR entries pruned per 16.4.1, then least cost, then largest Area ID.
2. External candidates compared by type, type-2 metric, then 16.4.1 path type, then cost.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF plugin to Loc-RIB | `locrib.Path` insert | No |

### Integration Points
- The OSPFv3 strategy's call into `ComputeExternalWith`.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| OSPFv3 multi-area topology in the daemon | → | `ComputeExternalWith` 16.4.1 mode | [functional or interop test named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | OSPFv3: ASBR reachable intra-area through non-backbone area 1 at cost 20 and through the backbone at cost 10 | The area-1 entry is used |
| AC-2 | OSPFv3: two ASBRs, equal E1 cost, one intra-area non-backbone, one inter-area | Only the intra-area non-backbone path is installed |
| AC-3 | OSPFv2 with no new config | Unchanged behavior; existing tests unchanged |
| AC-4 | Interop: Ze and FRR ospf6d, AC-1 topology | Both choose the same exit; proven red with the fix reverted |
| AC-5 | RFC 5340 Section 4.8.5 row | Added to `rfc/short/rfc5340.md`, tagged test, discrimination record |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | The OSPFv2 path changes by accident | AC-3, and the mode is an explicit input |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/plugins/ospf/spf/external_test.go` | AC-1, AC-2, AC-3 | |

## Files to Modify

- `internal/plugins/ospf/spf/external.go`, the OSPFv3 strategy caller
- `rfc/short/rfc5340.md`

## Implementation Steps

1. Failing unit tests for AC-1 and AC-2.
2. Add the mode to `ExternalInput`; implement pruning and step (6)(c).
3. Interop scenario against FRR.

### Integration Checklist
- [ ] Both OSPF versions still build their external tables through the one function

### Documentation Update Checklist
- [ ] `docs/architecture/ospf/ospf-8-spf-rib.md` states which rules each version uses

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test ospfv3`, `./le test unit plugins`, `./le rfc check`
