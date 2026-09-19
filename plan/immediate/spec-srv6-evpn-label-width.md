# Spec: srv6-evpn-label-width -- EVPN Label Storage Width for SRv6 Transposition

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-06-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9252.md` - Section 6: EVPN transposition (24-bit label)
4. `internal/component/bgp/plugins/rib/pool/srv6sid.go` - ApplyTransposition
5. `internal/component/bgp/plugins/nlri/` - EVPN NLRI label parsing

## Task

Implement RFC 9252 Section 6 SRv6 transposition for EVPN, so an EVPN route
whose Prefix-SID declares a Transposition Length gets the SID its peer
signaled rather than no SID at all.

**Corrected 2026-08-29.** This spec was written around a narrower premise:
that `labelWidthForSAFI` answers 24 for EVPN while the label pool stores
20-bit values, losing the top 4 bits. That premise is real but was
unreachable, because one layer below it no label reached the transposition
at all. The label side-data store (`FamilyRIB.labels`,
`internal/component/bgp/plugins/rib/storage/familyrib.go`) is created only
when `labeled && cidr`, and `labeled` is `fam.SAFI == family.SAFIMPLSLabel`.
EVPN is neither, so `LookupLabels` returned `InvalidHandle` for every EVPN
route and `ApplyTransposition` had no production caller.

That layer was fixed for the VPN families on 2026-08-29 by reading the label
out of the NLRI wire bytes the route is already keyed by, rather than storing
a second copy of them: `nlrisplit.TranspositionLabel`
(`internal/core/bgp/nlri/nlrisplit/transposition.go`) and `srv6SIDFromResult`
(`internal/component/bgp/plugins/rib/rib_bestchange.go`). RFC 9252 Section 7's
bound went in with it, so a Transposition Length wider than the label field
now makes the path ineligible for best-path selection.

`TranspositionLabel` answers only the VPN families. EVPN was left out
deliberately, and this spec is what closes it.

### Why EVPN is the harder half

EVPN does not have one label field. It has three carriers, and one of them is
not in the NLRI:

| Carrier | Route types | RFC 9252 | Width |
|---------|-------------|----------|-------|
| NLRI label field | 1 per-EVI, 2 (Label1 and Label2), 5 | S6.1.2, S6.2, S6.5 | 24 |
| PMSI Tunnel Attribute MPLS label | 3 | S6.3 | 24 |
| ESI Label extended community | 1 per-ES, carrying Argument bits | S6.1.1 | 24 |

Two further problems have no answer in the current code:

1. Route Type 2 carries Label1, bound to the SRv6 L2 Service TLV, and Label2,
   bound to the L3 Service TLV (S6.2). Choosing between them needs to know
   which TLV the SID came from. `pool.SRv6SIDResult`
   (`internal/component/bgp/plugins/rib/pool/srv6sid.go`) does not record it:
   `ExtractSRv6SIDFull` returns the first SID from either TLV type.
2. `nlri.ParseLabelStack` (`internal/core/bgp/nlri/rd.go`) returns the
   RFC 3107 20-bit value (`data[2]>>4`). RFC 9252 says of every EVPN label
   field that "the value is set in the 24 bits", so EVPN needs the raw
   three-octet value, not that one. This is the original 24-vs-20 finding,
   still true, and it belongs to whichever reader is written for EVPN.

Route Type 3's SID also merges the Route Type 1 ESI Filtering Argument by
bitwise OR (S6.3), which is a separate feature from transposition.

Doing part of EVPN is worse than doing none: a reader that handles Route Type
5 and guesses at Route Type 2 installs a confidently wrong SID for half the
population, which is the defect this whole line of work exists to remove.

### Investigation needed

1. Whether `pool.SRv6SIDResult` should record the Service TLV type its SID
   came from, and what that does to `ExtractSRv6SIDFull`'s callers.
2. Where the PMSI Tunnel Attribute reaches the RIB, if it does, since Route
   Type 3's label is not in the NLRI.
3. Whether real EVPN SRv6 deployments use `transposLen > 20`, which decides
   how much the 24-bit read matters against the 20-bit one.

### Key source files

- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - ApplyTransposition with labelWidth
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - labelWidthForSAFI
- `internal/core/bgp/nlri/nlrisplit/transposition.go` - TranspositionLabel, the VPN reader EVPN must join
- `internal/core/bgp/nlri/rd.go` - ParseLabelStack, which returns 20 bits where EVPN needs 24
- `internal/component/bgp/plugins/nlri/evpn/types.go` - EVPN route type layouts (EVPNType1/2/5, Labels())

## Required Reading

### Architecture Docs
- [ ] The SRv6 prefix-SID record (retired with the learned corpus) - SRv6 design decisions
- [ ] The SRv6 review-fixes record (retired with the learned corpus) - transposition errata 7652 fix

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9252.md` - Section 6: EVPN transposition rules
  -> Constraint: "Transposition Length MUST be less than or equal to 24 and less than or equal to the FL"
- [ ] `rfc/short/rfc7432.md` - EVPN NLRI label field format (if summary exists)

**Key insights:**
- (fill during design)

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/bgp/plugins/rib/storage/familyrib.go` - the label store is created only for `labeled && cidr`, so EVPN never had one
- [x] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `srv6SIDFromResult`, `labelWidthForSAFI`, `isSRv6Ineligible`
- [x] `internal/core/bgp/nlri/nlrisplit/transposition.go` - `TranspositionLabel`, which returns ok=false for EVPN
- [x] `internal/core/bgp/nlri/rd.go` - `ParseLabelStack` returns the 20-bit RFC 3107 value

**Behavior to preserve:**
- VPN transposition, working since 2026-08-29 and covered by
  `TestSRv6TranspositionRestoresFunctionBitsFromNLRILabel`
- EVPN routes that declare no transposition, which are untouched by any of this
- RFC 9252 Section 7 ineligibility for a Transposition Length wider than its label field

**Behavior to change:**
- An EVPN route declaring a transposition currently yields NO SRv6 SID. It must
  yield the reconstructed one.

## Data Flow (MANDATORY)

### Entry Point
- EVPN UPDATEs for Route Types 1 per-EVI, 2, 3 and 5 whose SRv6 Service TLV declares transposition, with zero-length transposition as the unchanged control and length 25 as the invalid-width case.

### Transformation Path
1. Preserve the Service TLV association of the extracted SID so Route Type 2 can select Label1 for the L2 Service TLV and Label2 for the L3 Service TLV.
2. Read the raw 24-bit carrier from the route's existing wire representation: the route-type-specific NLRI field for Types 1 per-EVI, 2 and 5, or the PMSI Tunnel Attribute for Type 3.
3. Carry that value to `srv6SIDFromResult` / `ApplyTransposition` at best-path change. Extend the existing wire-reader approach; do not add an EVPN `InternLabels` side store.
4. Reconstruct the SID at width 24, including bits lost by a 20-bit MPLS-label read.
5. Reject a path whose Transposition Length exceeds the permitted field width through the existing eligibility mechanism. Preserve VPN width-20 and no-transposition behaviour.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NLRI or attribute wire -> carrier selection | Route type and Service TLV identify the raw 24-bit field; Type 3 requires the PMSI attribute | [ ] |
| Carrier selection -> SID reconstruction and eligibility | `srv6SIDFromResult`, `ApplyTransposition` and the existing invalid-width check | [ ] |

### Integration Points
- `ApplyTransposition` (`internal/component/bgp/plugins/rib/pool/srv6sid.go`) consumes the selected carrier at width 24.
- `srv6SIDFromResult` and `labelWidthForSAFI` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) already provide the reconstruction seam and EVPN width.
- `nlrisplit.TranspositionLabel` (`internal/core/bgp/nlri/nlrisplit/transposition.go`) reads VPN labels from NLRI bytes today; extend the wire-reader approach for the appropriate EVPN carriers.
- `pool.SRv6SIDResult` / `ExtractSRv6SIDFull` must retain the Service TLV association required by AC-3.
- Type 3's PMSI carrier and Type 1 per-ES Argument handling remain the explicit design investigations in Task; the obsolete label-pool proposal supplies neither.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EVPN Route Type 5 with SRv6 transposLen=16 | SID reconstructed from the 24-bit NLRI label field |
| AC-2 | EVPN Route Type 5 with transposLen=24 | All 24 bits of the field transposed into the SID |
| AC-3 | EVPN Route Type 2 carrying Label1 and Label2 | The label bound to the Service TLV the SID came from is the one used |
| AC-4 | EVPN Route Type 1 per-EVI with transposition | SID reconstructed from the label at that route type's offset |
| AC-5 | EVPN Route Type 3 with ingress replication | SID reconstructed from the PMSI Tunnel Attribute label |
| AC-6 | VPN route with SRv6 transposition | No regression: the 2026-08-29 tests stay green |
| AC-7 | EVPN route with transposLen=25 | Path ineligible for best-path (RFC 9252 Section 7) |

## 🧪 TDD Test Plan

### Unit Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyTranspositionEVPNHighBits` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-1: transposLen=16 with value in high bits of the 24-bit field reconstructs the SID correctly | planned |
| `TestApplyTranspositionEVPNFull24Bits` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-2: transposLen=24 transposes all 24 bits into the SID | planned |
| `TestEVPNType2SelectsServiceTLVLabel` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-3: differing Label1/Label2 values reconstruct the SID from the matching Service TLV | planned |
| `TestEVPNType1PerEVISIDTransposition` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-4: Route Type 1 per-EVI uses its raw 24-bit label field | planned |
| `TestEVPNType3PMSISIDTransposition` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-5: Route Type 3 uses the PMSI Tunnel Attribute carrier | planned |
| `TestApplyTranspositionVPN20BitRegression` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-6: VPN transposition with 20-bit labels unchanged | planned |
| `TestEVPNTranspositionTooWideIsIneligible` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-7: length 25 cannot become best, with length 24 as the valid control | planned |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EVPN label | 0-16777215 (24-bit) | 16777215 | N/A | 16777216 |
| transposLen (EVPN) | 0-24 | 24 | N/A | 25 |

### Functional Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `evpn-srv6-transposition-24bit` | Functional RIB scenario; concrete carrier to be selected during design | AC-1 through AC-5 and AC-7: UPDATE ingestion through RIB best-path selection yields the reconstructed SID for Types 1 per-EVI, 2, 3 and 5, and excludes length 25. A decode-only assertion is insufficient | planned |
| `vpn-srv6-transposition-regression` | Functional RIB scenario plus `TestSRv6TranspositionRestoresFunctionBitsFromNLRILabel` | AC-6: existing VPN transposition remains unchanged | planned |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | | | | |

## Files to Modify
- (fill during design)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| (fill during design) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| (fill during design) | | | |

## Files to Create
- (fill during design)

## Implementation Steps

The detailed design must resolve the Task's open carrier questions before
implementation. The implementation sequence must cover Service TLV association
and the raw carrier readers first, then RIB reconstruction for Route Types 1
per-EVI, 2, 3 and 5. Verification must exercise UPDATE ingestion through
best-path selection for AC-1 through AC-7, including VPN regression, the
no-transposition control and invalid-width rejection. The former
`InternLabels`/`ResolveLabels` proposal is superseded by direct wire reads.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated through their current test mappings
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (or N/A confirmed)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-srv6-evpn-label-width.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/immediate/spec-srv6-evpn-label-width.md`
