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

`labelWidthForSAFI` returns 24 for EVPN (RFC 9252 Section 6.2: "Transposition
Length MUST be less than or equal to 24"). But `pool.ResolveLabels` stores MPLS
labels as 20-bit values (see `route_labeled.go`: "MaxMPLSLabel is the maximum
valid MPLS label value (20 bits)").

If an EVPN peer advertises `transposLen > 20`, `ApplyTransposition` reads bits
23..20 from the stored label, which are always 0. The high 4 bits of the
transposed value are lost.

### Investigation needed

1. How does the EVPN NLRI parser extract labels? Does it store the full 3-byte
   (24-bit) field or only the 20-bit MPLS label portion?
2. For SRv6 transposition in EVPN, RFC 9252 Section 6 says the full 24-bit
   label field is used. Confirm whether this means the 3-byte NLRI field
   including TC/S bits, or a 24-bit label value.
3. Do real-world EVPN SRv6 deployments use `transposLen > 20`?

### Key source files

- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - ApplyTransposition with labelWidth
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - labelWidthForSAFI
- `internal/component/bgp/plugins/rib/pool/labels.go` - InternLabels, ResolveLabels
- `internal/component/bgp/route/route_labeled.go` - MaxMPLSLabel (20 bits)
- `internal/component/bgp/plugins/nlri/` - EVPN NLRI label parsing

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
- [ ] `internal/component/bgp/route/route_labeled.go` - MaxMPLSLabel = 20 bits
- [ ] `internal/component/bgp/plugins/rib/pool/labels.go` - labels stored as uint32
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - ApplyTransposition uses labelWidth param

**Behavior to preserve:**
- VPN transposition with 20-bit labels (working correctly after review-fixes)
- EVPN label storage and forwarding for non-SRv6 use cases

**Behavior to change:**
- EVPN labels may need to store the full 24-bit field when SRv6 transposition is active

## Data Flow (MANDATORY)

### Entry Point
- EVPN UPDATE with PrefixSID containing SRv6 Service TLV and SID Structure with transposLen > 20

### Transformation Path
1. EVPN NLRI parsed, label extracted (currently 20 bits?)
2. Label stored via InternLabels
3. Best-path change triggers lookupSRv6SIDForBest
4. ApplyTransposition called with label and labelWidth=24
5. Bits 23..20 read from label, which are 0 if only 20 bits stored

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NLRI parser -> label pool | InternLabels([]uint32) | [ ] |
| Label pool -> transposition | ResolveLabels -> ApplyTransposition | [ ] |

### Integration Points
- `ApplyTransposition` (`internal/component/bgp/plugins/rib/pool/srv6sid.go`) - consumes the stored label with `labelWidth`; where the 24-bit read happens
- `labelWidthForSAFI` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) - already returns 24 for EVPN; the width the storage must honor
- `InternLabels` / `ResolveLabels` (`internal/component/bgp/plugins/rib/pool/labels.go`) - label storage that may need to keep the full 24-bit field
- `MaxMPLSLabel` (`internal/component/bgp/route/route_labeled.go`) - the 20-bit cap in conflict with EVPN transposition
- EVPN NLRI label parsing (`internal/component/bgp/plugins/nlri/`) - source of the label value; investigation item 1

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
| AC-1 | EVPN route with SRv6 transposLen=16, label has value in high 16 of 24 bits | Correct SID reconstruction |
| AC-2 | EVPN route with SRv6 transposLen=24 (full 24-bit field) | All 24 bits transposed into SID |
| AC-3 | VPN route with SRv6 transposition | No regression (20-bit label still works) |

## 🧪 TDD Test Plan

### Unit Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyTranspositionEVPNHighBits` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-1: transposLen=16 with value in high bits of the 24-bit field reconstructs the SID correctly | planned |
| `TestApplyTranspositionEVPNFull24Bits` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-2: transposLen=24 transposes all 24 bits into the SID | planned |
| `TestApplyTranspositionVPN20BitRegression` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | AC-3: VPN transposition with 20-bit labels unchanged | planned |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EVPN label | 0-16777215 (24-bit) | 16777215 | N/A | 16777216 |
| transposLen (EVPN) | 0-24 | 24 | N/A | 25 |

### Functional Tests
<!-- Planned names derived from ACs; refine during design (spec is skeleton). -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `evpn-srv6-transposition-24bit` | `test/decode/evpn-srv6-transposition-24bit.ci` | AC-1/AC-2: EVPN UPDATE with SRv6 SID Structure and transposLen > 20 decodes with correct reconstructed SID | planned |
| `vpn-srv6-transposition-regression` | `test/decode/vpn-srv6-transposition-regression.ci` | AC-3: VPN route with SRv6 transposition still decodes correctly (20-bit label) | planned |

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

(fill during design)

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
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
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
- [ ] **Commit B:** `git rm plan/spec-srv6-evpn-label-width.md`
