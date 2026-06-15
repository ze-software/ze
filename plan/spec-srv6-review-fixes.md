# Spec: srv6-review-fixes -- SRv6 Prefix-SID RFC Compliance Fixes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9252.md` - SRv6 wire format, validation, transposition
4. `rfc/short/rfc8669.md` - Label-Index TLV layout
5. `plan/learned/776-srv6-prefix-sid.md` - original implementation decisions

## Task

Fix 8 confirmed bugs in the SRv6 Prefix-SID implementation found by external review.
All are RFC compliance issues: wire-format encoding errors, missing validation checks,
incorrect bit extraction, and state-machine gaps. The bugs fall into two groups:

**Wire-format correctness (5 bugs):** affect interop with any RFC-compliant peer.

**State-machine correctness (3 bugs):** cause stale FIB entries or missed updates
under SRv6 SID changes, resolution transitions, and table replay.

### Findings not in scope (valid but deferred)

| # | Finding | Why deferred |
|---|---------|--------------|
| 6 | SRv6 SID resolvability not in best-path | Architectural: moving into gatherCandidatesLocked creates circular dependency (best-path -> Loc-RIB -> best-path). Document as known limitation. |
| 8 | EBGP egress propagates Prefix-SID | SHOULD not MUST (RFC 8669 Section 4). Needs egress policy knob. Separate spec. |
| 10 | Labeled-unicast SRv6 skipped | Scope extension. SAFIMPLSLabel + SRv6 is uncommon. Separate spec. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern, component isolation
  -> Constraint: components are independent; sysrib operates after best-path selection
- [ ] `plan/learned/776-srv6-prefix-sid.md` - original design decisions
  -> Decision: lazy extraction from OtherAttrs at best-path emission time
  -> Decision: SID resolvability reuses NH resolver via Track/Resolve
  -> Decision: transposition at best-path emission only for VPN/EVPN SAFIs

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9252.md` - SRv6 Service TLV format, validation, transposition
  -> Constraint: SID Info Sub-TLV minimum length = 21 (Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1))
  -> Constraint: Endpoint Behavior is 2 octets, not 1
  -> Constraint: malformed SRv6 Service TLV triggers treat-as-withdraw (Section 3.4)
  -> Constraint: transposed bits are in high-order positions of label field (errata 7652)
  -> Constraint: path with no valid SRv6 SID is ineligible for best-path (Section 5)
- [ ] `rfc/short/rfc8669.md` - Label-Index TLV field layout
  -> Constraint: Label-Index TLV value = Reserved(1) + Flags(2) + LabelIndex(4) = 7 bytes
  -> Constraint: Label Index is 4 octets (uint32), max 2^32-1

**Key insights:**
- RFC 9252 SID Info value layout: Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) = 21 bytes minimum
- RFC 8669 Label-Index value layout: Reserved(1) + Flags(2) + LabelIndex(4) = 7 bytes
- Transposed bits occupy high-order positions of label field per errata 7652
- sysrib stores resolvedNH before checking SID resolvability, creating stale state

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/routeattr_prefixsid.go` - SRv6 encoder and Label-Index encoder
  -> Constraint: Behavior parsed as uint8 (line 292), emitted as 1 byte (line 331)
  -> Constraint: SID Info value is 20 bytes (missing trailing Reserved byte)
  -> Constraint: Label-Index emits Reserved(3)+Flags(1)+LabelIndex(3) (lines 65-76)
  -> Constraint: Label index max is 0xFFFFFF (24-bit), should be 0xFFFFFFFF (32-bit)
- [ ] `internal/component/bgp/message/rfc7606.go:725-783` - PrefixSID and SRv6 Service TLV validation
  -> Constraint: outer TLV loop has no leftover-byte check after loop (line 746)
  -> Constraint: Service TLV sub-TLV loop has no leftover-byte check (line 783)
  -> Constraint: SID Info Sub-TLV length check for 21 minimum is correct in validator
  -> Constraint: no sub-sub-TLV bounds validation inside SID Info Sub-TLV
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - SID extraction and transposition
  -> Constraint: extractSIDFromServiceTLV uses subLen >= 17 (line 83, should be >= 21)
  -> Constraint: ApplyTransposition reads from bit transposLen-1 downward (line 158)
  -> Constraint: ApplyTransposition called from rib_bestchange.go:866 with labels[0]
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go:768-821` - best-change short-circuit
  -> Constraint: short-circuit compares peer, NH, MED, eBGP only (lines 770-774)
  -> Constraint: SRv6SID assigned at line 819 but unreachable when short-circuit fires
  -> Constraint: lookupSRv6SIDForBest computed before short-circuit (line 717)
- [ ] `internal/plugins/sysrib/sysrib.go:399-414,531-588,724-734` - SID resolution and replay
  -> Constraint: resolvedNH stored at line 404 before SID check at line 411
  -> Constraint: handleNHChange returns nil when NH unchanged even if SID newly reachable (line 587)
  -> Constraint: replayBest emits all s.best routes without SID resolvability check (line 724-734)

**Behavior to preserve:**
- Lazy extraction from OtherAttrs at best-path emission time
- SID resolvability via NH resolver Track/Resolve mechanism
- Transposition only for VPN/EVPN SAFIs (needsTransposition check)
- IsSRv6Ineligible filtering in gatherCandidatesLocked
- EBGP ingress filtering via AcceptSRv6PrefixSID setting
- PrefixSID suppression on NH change in forward path
- Existing functional test wire expectations must be updated (not preserved)

**Behavior to change:**
- SID Info encoder: emit Flags(1) + Behavior(2) + Reserved(1) after SID for 21-byte minimum
- Behavior field: parse as uint16, emit as 2 bytes
- Label-Index encoder: emit Reserved(1) + Flags(2) + LabelIndex(4), accept uint32 range
- Validation: add leftover-byte checks in both TLV loops
- Validation: add sub-sub-TLV bounds check inside SID Info
- SID extraction: raise minimum subLen from 17 to 21
- Transposition: read label bits from high-order positions (bit labelWidth-1 downward)
- Best-change: include SRv6 SID in same-best short-circuit comparison
- sysrib: defer resolvedNH storage until after SID resolvability check passes
- sysrib: handle SID-becomes-reachable transition in handleNHChange
- sysrib: check SID resolvability in replayBest

## Data Flow (MANDATORY)

### Entry Point
- Config: `ParsePrefixSIDSRv6()` / `ParsePrefixSID()` produces wire bytes for local origination
- Wire: UPDATE attribute 40 arrives, validated by `validatePrefixSIDAttr`
- RIB: best-path emission extracts SID via `ExtractSRv6SIDFull` + `ApplyTransposition`
- sysrib: `selectBest` / `handleNHChange` / `replayBest` apply SID resolvability

### Transformation Path
1. Config string -> `ParsePrefixSIDSRv6` / `ParsePrefixSID` -> wire TLV bytes
2. Wire bytes -> `validatePrefixSIDAttr` -> accept/discard/withdraw
3. Wire bytes -> `ExtractSRv6SIDFull` -> `SRv6SIDResult` (SID + transposition params)
4. `SRv6SIDResult` + NLRI label -> `ApplyTransposition` -> final SID
5. Final SID -> `bestChangeEntry.SRv6SID` -> sysrib -> FIB

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Wire | `PrefixSID.Bytes` stored on route | [ ] |
| Wire -> Validator | `attrValidators[40]` registration | [ ] |
| Wire -> Pool | `ExtractSRv6SIDFull` reads OtherAttrs bytes | [ ] |
| RIB -> sysrib | `bestChangeEntry.SRv6SID` via EventBus | [ ] |
| sysrib -> FIB | `outgoingChange.SRv6SID` via EventBus | [ ] |

### Architectural Verification
- [ ] No bypassed layers (fixes stay within existing data flow)
- [ ] No unintended coupling (no new cross-component dependencies)
- [ ] No duplicated functionality (correcting existing code, not adding parallel paths)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ParsePrefixSIDSRv6("l3-service 2001:db8::1 0x48")` | -> | 21-byte SID Info value | `TestParsePrefixSIDSRv6_MinimumLength` |
| `ParsePrefixSID("777")` | -> | Reserved(1)+Flags(2)+LabelIndex(4) | `TestParsePrefixSID_FieldLayout` |
| Wire bytes with 1 trailing byte in outer TLV | -> | `validatePrefixSIDAttr` returns discard | `TestValidatePrefixSID_TrailingBytes` |
| `ExtractSRv6SIDFull` with subLen=20 | -> | returns invalid SID | `TestExtractSRv6SID_SubLenTooShort` |
| `ApplyTransposition` with 16-bit label in high bits | -> | correct SID reconstruction | `TestApplyTransposition_HighOrderBits` |
| Same-best with changed SRv6 SID | -> | `emitBestChange` returns change | `TestBestChange_SRv6SIDChange` |
| Route suppressed, SID becomes reachable | -> | sysrib emits Add | `TestSysRIB_SIDBecomesReachable` |
| `replayBest` with unresolvable SID | -> | route skipped | `TestSysRIB_ReplaySkipsUnresolvable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ParsePrefixSIDSRv6("l3-service 2001:db8::1 0x48")` | SID Info Sub-TLV length = 21; Behavior at bytes 18-19 as big-endian uint16 0x0048 |
| AC-2 | `ParsePrefixSIDSRv6("l3-service 2001:db8::1")` | SID Info Sub-TLV length = 21; trailing Reserved byte at offset 20 |
| AC-3 | `ParsePrefixSID("777")` | TLV value bytes = `00 00 00 00 00 03 09` (Reserved(1)=00, Flags(2)=0000, LabelIndex(4)=00000309) |
| AC-4 | `ParsePrefixSID("20000000")` | Accepted; 4-byte LabelIndex = 0x01312D00 |
| AC-5 | Prefix-SID attribute with 1-2 trailing bytes after last TLV | `validatePrefixSIDAttr` returns attribute-discard |
| AC-6 | Service TLV with 1-2 trailing bytes after last sub-TLV | `validateSRv6ServiceTLV` returns treat-as-withdraw |
| AC-7 | SID Info Sub-TLV with sub-sub-TLV length exceeding enclosing sub-TLV | `validateSRv6ServiceTLV` returns treat-as-withdraw |
| AC-8 | `ExtractSRv6SIDFull` with subLen 17 through 20 | Returns invalid SRv6SIDResult (SID not valid) |
| AC-9 | `ApplyTransposition(SID, label, 48, 16)` where label has 16-bit value in high-order positions of 20-bit field | SID function bits at offset 48 match the transposed value |
| AC-10 | Best-path winner unchanged (same peer, NH, MED, eBGP) but SRv6 SID changed | `emitBestChange` returns (entry, true) with new SID |
| AC-11 | Route stored in sysrib with unresolvable SID; SID later becomes reachable | sysrib emits outgoingChange with Action=Add |
| AC-12 | `replayBest` called; s.best contains route with unresolvable SRv6 SID | Route excluded from replay output |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePrefixSIDSRv6_MinimumLength` | `internal/component/bgp/config/routeattr_prefixsid_test.go` | SID Info value is 21 bytes | |
| `TestParsePrefixSIDSRv6_BehaviorTwoBytes` | `internal/component/bgp/config/routeattr_prefixsid_test.go` | Behavior > 0xFF encoded correctly as uint16 | |
| `TestParsePrefixSID_FieldLayout` | `internal/component/bgp/config/routeattr_prefixsid_test.go` | Reserved(1)+Flags(2)+LabelIndex(4) byte layout | |
| `TestParsePrefixSID_LargeIndex` | `internal/component/bgp/config/routeattr_prefixsid_test.go` | Label index > 16M accepted, 4-byte encoding | |
| `TestValidatePrefixSID_TrailingBytes` | `internal/component/bgp/message/rfc7606_test.go` | Trailing bytes in outer TLV rejected | |
| `TestValidateSRv6ServiceTLV_TrailingBytes` | `internal/component/bgp/message/rfc7606_test.go` | Trailing bytes in service TLV rejected | |
| `TestValidateSRv6ServiceTLV_SubSubTLVBounds` | `internal/component/bgp/message/rfc7606_test.go` | Sub-sub-TLV exceeding enclosing sub-TLV rejected | |
| `TestExtractSRv6SID_SubLenTooShort` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | subLen < 21 returns invalid SID | |
| `TestApplyTransposition_HighOrderBits` | `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` | Reads from high-order label bits | |
| `TestBestChange_SRv6SIDChange` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | SID change triggers emission | |
| `TestSysRIB_SIDBecomesReachable` | `internal/plugins/sysrib/sysrib_test.go` | Suppressed route installed when SID resolves | |
| `TestSysRIB_ReplaySkipsUnresolvable` | `internal/plugins/sysrib/sysrib_test.go` | Replay skips unresolvable SID routes | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Behavior (RFC 9252) | 0x0000-0xFFFF | 0xFFFF | N/A (unsigned) | N/A (uint16 max) |
| LabelIndex (RFC 8669) | 0-4294967295 | 4294967295 | N/A (unsigned) | 4294967296 (overflow) |
| SID Info Sub-TLV Length | 21-65535 | 21 (min valid) | 20 (treat-as-withdraw) | N/A |
| TransposLen for VPN | 0-20 | 20 | N/A | 21 (invalid per RFC) |
| TransposLen for EVPN | 0-24 | 24 | N/A | 25 (invalid per RFC) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-sid-srv6` | `test/encode/prefix-sid-srv6.ci` | SRv6 route encoded with correct 21-byte wire format | |

### Interop Tests
Interop deferred: correctness fixes to existing wire format. Functional test covers encoding.
Interop testing deferred to `spec-fib-depth-4-srv6` (FIB backends).

## Files to Modify
- `internal/component/bgp/config/routeattr_prefixsid.go` - fix SRv6 SID Info encoding (21 bytes, 2-byte behavior) and Label-Index encoding (field layout, uint32 index)
- `internal/component/bgp/message/rfc7606.go` - add leftover-byte checks and sub-sub-TLV bounds validation
- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - raise subLen minimum to 21, fix transposition bit extraction
- `internal/component/bgp/plugins/rib/pool/srv6sid_test.go` - update existing transposition tests for high-order bits
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - add SRv6 SID to same-best short-circuit
- `internal/plugins/sysrib/sysrib.go` - fix resolvedNH ordering, SID-becomes-reachable, replay filtering
- `test/encode/prefix-sid-srv6.ci` - update expected hex for corrected encoding

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| CLI grammar | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Doctor check for runtime dependencies | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | Yes | wire format corrected to match RFC; no Ze wire doc exists yet |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | already in rfc/short/ |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create
- No new files (all tests go in existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8-10. Fix/re-verify | Loop until clean |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 14. Summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wire-format encoding (findings 1, 11)** -- fix encoder output
   - Tests: `TestParsePrefixSIDSRv6_MinimumLength`, `TestParsePrefixSIDSRv6_BehaviorTwoBytes`, `TestParsePrefixSID_FieldLayout`, `TestParsePrefixSID_LargeIndex`
   - Files: `routeattr_prefixsid.go`
   - Changes:
     - `ParsePrefixSIDSRv6`: parse behavior as uint16, emit Flags(1) + Behavior(2) + Reserved(1) = 4 bytes after SID
     - `ParsePrefixSID` / `parsePrefixSIDWithSRGB`: emit Reserved(1) + Flags(2) + LabelIndex(4), accept uint32 range
   - Verify: encoder produces 21-byte SID Info value, correct Label-Index field layout

2. **Phase: Validation hardening (finding 2)** -- add missing checks
   - Tests: `TestValidatePrefixSID_TrailingBytes`, `TestValidateSRv6ServiceTLV_TrailingBytes`, `TestValidateSRv6ServiceTLV_SubSubTLVBounds`
   - Files: `rfc7606.go`
   - Changes:
     - `validatePrefixSIDAttr`: after TLV loop, check `off != length`, return attribute-discard
     - `validateSRv6ServiceTLV`: after sub-TLV loop, check `off != len(value)`, return treat-as-withdraw
     - `validateSRv6ServiceTLV`: validate sub-sub-TLV bounds within SID Info Sub-TLV
   - Verify: trailing bytes and overlong sub-sub-TLVs rejected

3. **Phase: SID extraction minimum length (finding 3)** -- raise subLen check
   - Tests: `TestExtractSRv6SID_SubLenTooShort`
   - Files: `srv6sid.go`
   - Changes: `extractSIDFromServiceTLV` line 83: change `subLen >= 17` to `subLen >= 21`
   - Verify: subLen 17-20 returns invalid SID, subLen 21+ extracts correctly

4. **Phase: Transposition bit extraction (finding 4)** -- fix label bit positions
   - Tests: `TestApplyTransposition_HighOrderBits` (new), update existing tests
   - Files: `srv6sid.go`, `srv6sid_test.go`
   - Changes:
     - Add `labelWidth uint8` parameter to `ApplyTransposition`
     - Extract bit i from `label >> (labelWidth - 1 - i)` instead of `label >> (transposLen - 1 - i)`
     - Update caller in `rib_bestchange.go:866` to pass label width (20 for VPN, 24 for EVPN)
     - Update existing tests to place transposed values in high-order bits
   - Verify: 16-bit value in top 16 of 20-bit label correctly transposes into SID

5. **Phase: Best-change SRv6 SID comparison (finding 5)** -- prevent silent drops
   - Tests: `TestBestChange_SRv6SIDChange`
   - Files: `rib_bestchange.go`
   - Changes: add SRv6 SID comparison to same-best short-circuit (lines 768-775). The SRv6 SID is already computed at line 717 before the short-circuit, so compare `srv6SID` against previous record's SID.
   - Verify: SID-only change emits bestChangeEntry

6. **Phase: sysrib SID resolution state machine (findings 7, 9)** -- fix suppression and replay
   - Tests: `TestSysRIB_SIDBecomesReachable`, `TestSysRIB_ReplaySkipsUnresolvable`
   - Files: `sysrib.go`
   - Changes:
     - `selectBest` (new route, prev==nil path at line 399): move `s.resolvedNH[key] = resolved` after SID resolvability check; when SID is unresolvable, do not store resolvedNH
     - `handleNHChange` (line 572+): when SRv6 SID is valid and resolvable but no previous resolvedNH exists (suppressed route), emit Add instead of returning nil
     - `replayBest` (line 724): check `srv6SIDResolvable(route.srv6SID)` when `route.srv6SID.IsValid()`; skip unresolvable routes
   - Verify: suppressed route installs when SID resolves; replay skips unresolvable

7. **Phase: Update functional test fixture**
   - Files: `test/encode/prefix-sid-srv6.ci`
   - Changes: recompute expected hex to match corrected 21-byte SID Info encoding
   - Verify: `make ze-functional-test` passes

8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit, learned summary, spec closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 12 ACs have implementation with file:line |
| Correctness | SID Info value exactly 21 bytes without structure, 30 with; Label-Index exactly 7 bytes |
| Wire compat | Our encoder output passes our own validator (round-trip) |
| Transposition | High-order bit extraction matches RFC 9252 errata 7652 |
| State machine | SID-becomes-reachable path has no stale resolvedNH state |
| Replay | Unresolvable routes excluded from replay batch |
| Rule: buffer-first | No new allocations on wire-parsing hot path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| SID Info encoder produces 21-byte minimum value | `go test -run TestParsePrefixSIDSRv6_MinimumLength` |
| Label-Index encoder uses correct field layout | `go test -run TestParsePrefixSID_FieldLayout` |
| Validator rejects trailing bytes | `go test -run TestValidatePrefixSID_TrailingBytes` |
| subLen minimum raised to 21 | grep `subLen >= 21` in srv6sid.go |
| Transposition reads high-order bits | `go test -run TestApplyTransposition_HighOrderBits` |
| Best-change includes SRv6 SID | grep `srv6SID` in short-circuit block of rib_bestchange.go |
| sysrib resolvedNH deferred past SID check | read sysrib.go selectBest path |
| Replay filters unresolvable | grep `srv6SIDResolvable` in replayBest |
| Functional test updated and passes | `make ze-functional-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All TLV length checks reject overlong/underlong values before reading data |
| Buffer bounds | No out-of-bounds reads on malformed SID Info with subLen < 21 |
| Resource exhaustion | No new allocations on the hot path; validation short-circuits early |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, transposition scheme.

## Implementation Summary

### What Was Implemented
- SID Info Sub-TLV: 21-byte minimum encoding, Behavior as uint16
- Label-Index TLV: Reserved(1)+Flags(2)+LabelIndex(4) layout, uint32 range
- Trailing-byte validation in outer TLV and Service TLV loops
- Sub-sub-TLV bounds validation within SID Info
- SID extraction minimum raised from 17 to 21
- Transposition reads label bits from high-order positions (errata 7652)
- Best-change short-circuit bypassed when SRv6 SID present
- sysrib resolvedNH deferred past SID resolvability check
- replayBest skips routes with unresolvable SRv6 SIDs

### Bugs Found/Fixed
All 8 spec findings confirmed and fixed.

### Documentation Updates
- RFC header comments added to routeattr_prefixsid.go and srv6sid.go
- Functional test hex updated for corrected encoding

### Deviations from Plan
- Implemented directly without formal /ze-implement workflow (user request)
- Bug 5 (best-change short-circuit): used flag-based bypass instead of interning SRv6 SID

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Wire-format encoding matches RFC 9252/8669 | unit test | encoder tests with byte-level verification |
| Malformed TLVs rejected per RFC error handling | unit test | validation tests with trailing bytes, short lengths, overlong sub-sub-TLVs |
| Transposition correct per errata 7652 | unit test | high-order bit extraction test |
| SID changes trigger FIB updates | unit test | best-change and sysrib state tests |
| SID resolution transitions handled | unit test | becomes-reachable and replay tests |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Interop tests N/A (correctness fixes, deferred to spec-fib-depth-4-srv6)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-srv6-review-fixes.md`
- [ ] Summary included in commit
