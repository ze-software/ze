# Spec: isis-segment-routing-mpls

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: IS-IS Segment Routing over MPLS (RFC 8667). No release profile lists RFC 8667, so no summary is written or enrolled until Thomas brings it into scope (`ai/META-DECISIONS.md`). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

Segment Routing lets an IS-IS domain carry MPLS label-switched paths with no
LDP or RSVP-TE: each router advertises an SR Global Block (SRGB) and Prefix-SID
indices for its loopbacks, and Adjacency SIDs for its links; every router then
programs label forwarding from the IGP alone. Ze already does this for OSPF
(RFC 8665, RFC 8666). Ze's IS-IS neither originates nor consumes SR-MPLS
sub-TLVs; it only exports SRv6 SIDs it receives into BGP-LS.

### VyOS reference

VyOS `protocols isis segment-routing global-block`, `local-block`,
`prefix <p> index|absolute <n>` on top of FRR isisd. T9485 (vyos-1x
`c33e993f1`) enables SR-MPLS only when MPLS segment-routing options are set, so
an SRv6-only IS-IS instance does not turn SR-MPLS on. Ze must keep the two
apart the same way (AC-8).

### RFC text read (`rfc/full/rfc8667.txt`)

- Section 2.1: "A Prefix-SID sub-TLV is associated to a prefix advertised by a node and MAY be present in any of the following TLVs:" TLV-135, 235, 236 and 237. Type 3, length 5 or 6, flags R, N, P, E, V, L.
- Section 2.2: Adj-SID and LAN-Adj-SID sub-TLVs. Section 2.4: SID/Label Binding TLV (mapping server). Section 3: SR-Capabilities, SR-Algorithm, SR Local Block and SRMS Preference sub-TLVs of the Router Capability TLV (242).
- The design phase walks every MUST of Sections 2 and 3 before an AC is final.

## Required Reading

- [ ] `docs/architecture/isis/isis-9-spf-rib.md`, `isis-4-component-config.md`
- [ ] `docs/architecture/mpls/mpls-kernel.md` - how label entries reach the kernel, and its test traps
- [ ] `ai/digests/isis.md`, `ai/digests/mpls-signaling.md`
- [ ] `docs/architecture/wire/ospf.md` - the OSPF SR config resolve, the shape to share

## Current Behavior

- [ ] `internal/plugins/ospf/sr_config.go` - OSPF resolves a `segment-routing` container into `sr.SRConfig` (SRGB, SRLB, prefix SIDs) from package `internal/plugins/ospf/sr`. That package lives inside the OSPF plugin, so IS-IS cannot import it (`./le arch tier check`, plugin import ownership).
- [ ] `internal/plugins/isis/bgpls_export.go` - IS-IS reads SID sub-TLVs only to re-encode them for BGP-LS (`sidValue`, `endSID`); it installs no label and originates no SR sub-TLV.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| OSPF SR model and label computation | `internal/plugins/ospf/sr` (SRGB, Prefix-SID, actions), `sr_install.go`, `sr_fib.go` | Computes push, swap and pop from OSPF SPF and SR TLVs | Extends: the protocol-neutral part (SRGB arithmetic, label actions, mapping-server resolution) moves to an `internal/core/` leaf both IGPs import; the TLV codecs stay per protocol |
| MPLS FIB events | `internal/core/mplsfib` `(mpls-fib, entry)` | The only path from a protocol to kernel labels | Reused: IS-IS emits label entries the same way; no netlink in `isis` |
| IS-IS SPF and Loc-RIB install | `internal/plugins/isis/spf` | Inserts IP routes | Extends: each SPF route gains its label actions |
| IS-IS LSP TLV codecs | `internal/plugins/isis/packet` | Router Capability and reachability TLVs | Extends: Prefix-SID, Adj-SID, LAN-Adj-SID, SR-Capabilities, SR-Algorithm, SRLB, Binding TLV |
| BGP-LS export of SIDs | `bgpls_export.go` | Exports SRv6 SIDs | Extends: exports SR-MPLS SIDs from the same decoded values (RFC 9085) |

## Data Flow

### Entry Point
Config `isis { segment-routing { ... } }` and received LSPs carrying SR sub-TLVs.

### Transformation Path
1. Config resolves into the shared SR model.
2. Own LSP carries SR-Capabilities in TLV 242 and Prefix-SIDs on loopback reachability; Adj-SIDs per adjacency.
3. SPF plus neighbors' SRGBs produce label actions, emitted as `(mpls-fib, entry)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IS-IS plugin to fib-kernel | `(mpls-fib, entry)` events | No |

### Integration Points
- Shared SR leaf in `internal/core/`; IS-IS LSP origination; IS-IS SPF.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IS-IS config with SRGB and a Prefix-SID | → | LSP origination and label install | [functional `.ci` plus QEMU label read-back, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SRGB 16000-23999 and Prefix-SID index 1 on the loopback | Own LSP carries SR-Capabilities and the Prefix-SID sub-TLV with N-flag; hex asserted |
| AC-2 | A received Prefix-SID from a neighbor two hops away | Ze installs push and swap entries using the next hop's SRGB; read back from the kernel in QEMU |
| AC-3 | Adjacency up | An Adj-SID is allocated from the SRLB and advertised; pop entry installed |
| AC-4 | Prefix-SID index outside a neighbor's SRGB | No label entry for that prefix, with a log line naming prefix, index and neighbor |
| AC-5 | Malformed SR sub-TLV | Handled as RFC 8667 and RFC 1195 require, never as a zero value |
| AC-6 | The OSPF SR tests after the extraction to `internal/core/` | Unchanged and green |
| AC-7 | Interop: Ze and FRR isisd with SR-MPLS | Labels agree in both directions; an MPLS ping or traffic test crosses Ze; proven red with the label install reverted |
| AC-8 | SRv6 configured without MPLS segment-routing options | SR-MPLS stays off: no SR-Capabilities MPLS flags, no labels |
| AC-9 | BGP-LS export | SR-MPLS SIDs appear per RFC 9085 |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Extracting the OSPF SR package regresses OSPF | AC-6, and the extraction lands as its own commit first |
| R-2 | Stock kernels refuse some MPLS attributes | `docs/architecture/mpls/mpls-kernel.md` traps; probe as RSVP-TE does |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/plugins/isis/packet/` tests | SR sub-TLV encode and decode | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/isis/` | SR config parses and the LSP carries the sub-TLVs | |

## Files to Modify

- `internal/plugins/ospf/sr` (moved to an `internal/core/` leaf), `internal/plugins/ospf/sr_config.go`
- `internal/plugins/isis/packet/`, `internal/plugins/isis/spf/`, the IS-IS YANG
- `docs/guide/isis.md`, `docs/architecture/isis/`

## Implementation Steps

1. Extract the protocol-neutral SR model; OSPF tests stay green.
2. IS-IS codecs, origination, label computation.
3. FRR interop scenario.

### Integration Checklist
- [ ] `./le arch tier check` passes after the extraction

### Documentation Update Checklist
- [ ] `docs/guide/isis.md` gains a segment routing section

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test isis`, `./le test wire isis`, `./le test ospf`, `./le test integration interop`
