# Spec: ospf-rfc1583-compatibility

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Owed RFC 2328 work after the first release: the RFC1583Compatibility parameter and the Section 16.4.1 preference rules. OSPFv2's current behavior equals the RFC default ("enabled"), so the shipped default is conformant; the OSPFv3 side is a shipped defect owned by `plan/release/fixes/spec-ospfv3-external-path-preference.md`. |
| RFC gaps | RFC2328-C.1-1 |
| Depends | plan/release/fixes/spec-ospfv3-external-path-preference.md (it builds the 16.4.1 rules this spec puts behind a switch) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

RFC 2328 Appendix C.1 defines the global parameter RFC1583Compatibility,
"enabled" by default. With it "disabled", Section 16.4 prunes the ASBR's routing
table entries per Section 16.4.1 before choosing one, and step (6)(c) breaks
ties between external paths on the intra-AS path type. Ze has no such
parameter: every OSPFv2 instance behaves as "enabled".

An operator running a domain whose routers all implement Section 16.4.1 sets
the parameter "disabled" everywhere, which is what C.1 asks for:
"Otherwise, all routers should have RFC1583Compatibility set to "disabled",
preventing all routing loops." Ze cannot join such a domain consistently today,
so a mixed domain can loop on AS-external routes originated from several areas.

### VyOS reference

VyOS `protocols ospf compatibility rfc1583`, rendered as FRR's
`compatible rfc1583` (T9441, vyos-1x `a57ab8277`). FRR decides it in
`ospf_find_asbr_route` (`ospfd/ospf_ase.c`, prunes when
`OSPF_RFC1583_COMPATIBLE` is unset) and `ospf_route_cmp` (`ospfd/ospf_route.c`).
BIRD's equivalent is the OSPF `rfc1583compat` option; read its source before
an interop test asserts BIRD's choice.

### RFC text read (`rfc/full/rfc2328.txt`)

- Section 16.4 step (3): "If RFC1583Compatibility is set to "disabled", prune the set of routing table entries for the ASBR as described in Section 16.4.1."
- Section 16.4 step (6)(c): "If the new AS external path is still indistinguishable from the current paths in the N's routing table entry, and RFC1583Compatibility is set to "disabled", select the preferred paths based on the intra-AS paths to the ASBR/forwarding addresses, as specified in Section 16.4.1."
- Section 16.4.1: "Intra-area paths using non-backbone areas are always the most preferred." and "The other paths, intra-area backbone paths and inter-area paths, are of equal preference."
- Appendix C.1: "Set to "enabled" by default."
- RFC 3101 Section 2.5 step (3) repeats the pruning for Type-5 LSAs on an NSSA border router (`rfc/full/rfc3101.txt`).

## Required Reading

- [ ] `docs/architecture/ospf/ospf-8-spf-rib.md` - how SPF results reach the Loc-RIB
- [ ] `rfc/short/rfc2328.md` - Section 16.4 prose, Appendix C table, row RFC2328-C.1-1
- [ ] `plan/release/fixes/spec-ospfv3-external-path-preference.md` - the shared 16.4.1 implementation

## Current Behavior

- [ ] `internal/plugins/ospf/spf/external.go` - `ComputeExternalWith` keeps, per ASBR, the least-cost border-router entry and breaks ties on the largest Area ID, folding every Type-5-capable area into one key; no 16.4.1 pruning. `betterExternal` orders candidates by route type, metric, E2 forwarding distance and RFC 3101 source preference, never by intra-AS path type. The function is shared by OSPFv2 and OSPFv3.
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` - no RFC1583Compatibility leaf.
- [ ] `internal/plugins/ospf/config.go` - the OSPFv2 config resolver that would carry the new leaf.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| External route computation | `spf/external.go` `ComputeExternalWith`, `keepBestExternal`, `betterExternal` | RFC 1583 preference for both OSPF versions | Extends: OSPFv2 takes the 16.4.1 rules when the new leaf says disabled |
| 16.4.1 rules | built by `spec-ospfv3-external-path-preference` | Do not exist yet | Reused: this spec selects them per OSPFv2 instance |
| NSSA Type-7 selection | `betterExternal`, RFC 3101 Section 2.5 | Source preference among equivalent LSAs | Extends: Type-5 pruning on an NSSA border router follows the same switch |
| OSPF instance config | `ze-ospf-conf.yang`, `config.go` | Per-instance leaves | Extends: one leaf per OSPFv2 instance, default enabled |

## Data Flow

### Entry Point
Config leaf under the OSPFv2 instance; a reload re-runs the external computation.

### Transformation Path
1. The config resolver sets the instance's compatibility mode.
2. `ComputeExternalWith` receives the mode in `ExternalInput`.
3. ASBR entry selection and step (6)(c) apply 16.4.1 when disabled.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF plugin to Loc-RIB | `locrib.Path` insert, route-install RPC when forked | No |

### Integration Points
- `ExternalInput`, OSPFv2 strategy only (OSPFv3 always behaves as "disabled", RFC 5340 Section 4.8.5).

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| OSPFv2 config with compatibility disabled | → | `ComputeExternalWith` 16.4.1 branch | [functional `.ci` named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No leaf set | Behavior unchanged ("enabled", the Appendix C.1 default); existing external-route tests stay green unchanged |
| AC-2 | Disabled; one ASBR reachable through a non-backbone intra-area path at cost 20 and through the backbone at cost 10 | The non-backbone intra-area entry is used (16.4.1 first rule), not the cheaper backbone one |
| AC-3 | Disabled; two ASBRs advertise the same Type-1 prefix at equal X+Y, one reached intra-area non-backbone, one inter-area | Only the non-backbone intra-area path is installed (step (6)(c)) |
| AC-4 | Enabled, with the AC-2 and AC-3 topologies | The least-cost entry and ECMP, as today |
| AC-5 | A reload flips the leaf | External routes are recomputed and the FIB follows, with no adjacency restart |
| AC-6 | `show ospf` | Prints the instance's RFC1583Compatibility mode |
| AC-7 | Interop: Ze and FRR (`compatible rfc1583` off) in a multi-area topology with an ASBR reachable through two areas | Both select the same exit; the scenario is proven red with the 16.4.1 branch reverted |
| AC-8 | The RFC2328-C.1-1 row | Tagged test and discrimination record through `./le rfc record` |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A mixed domain loops when operators set the leaf on some routers only | The guide states the C.1 advice; a doctor check is added only if the wire carries evidence of the peer's mode |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/plugins/ospf/spf/external_test.go` | AC-2, AC-3, AC-4 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/ospf/` | The leaf parses, show prints it, reload recomputes | |

## Files to Modify

- `internal/plugins/ospf/yang/ze-ospf-conf.yang`, `internal/plugins/ospf/config.go`
- `internal/plugins/ospf/spf/external.go`
- `docs/guide/ospf.md`, `rfc/short/rfc2328.md` (row annotation)

## Implementation Steps

1. Land the 16.4.1 rules through `spec-ospfv3-external-path-preference`.
2. Add the leaf and carry it into `ExternalInput` for OSPFv2.
3. Interop scenario against FRR.

### Integration Checklist
- [ ] OSPFv3 never reads the leaf

### Documentation Update Checklist
- [ ] `docs/guide/ospf.md` explains the leaf and the C.1 advice

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test ospf`, `./le test unit plugins`, `./le rfc check`
