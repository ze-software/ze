# Spec: llgr-1-capability

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9494.md` - LLGR RFC (capability wire format)
4. `internal/component/bgp/plugins/gr/gr.go` - GR plugin: decodeGR, extractGRCapabilities, RunDecodeMode
5. `internal/component/bgp/plugins/gr/register.go` - plugin registration
6. `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - YANG schema
7. `internal/component/bgp/attribute/community.go` - well-known community constants

## Task

Add LLGR capability (code 71) wire decode/encode, YANG config for long-lived-stale-time per AFI/SAFI, config extraction for Stage 3, CLI decode support, and well-known community constants. This is the foundation phase: no state machine or RIB changes.

Parent: `spec-llgr-0-umbrella.md`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin 5-stage protocol, capability declaration
  → Constraint: capabilities declared in Stage 3 via SetCapabilities
  → Decision: per-peer capabilities extracted from config JSON in OnConfigure (Stage 2)
- [ ] `docs/architecture/wire/capabilities.md` - capability wire format
  → Constraint: capability value is raw hex, plugin responsible for encoding/decoding

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9494.md` - LLGR capability code 71 wire format
  → Constraint: 7 bytes per AFI/SAFI tuple: AFI(2) + SAFI(1) + Flags(1) + LLST(3)
  → Constraint: MUST also advertise GR cap (code 64) or LLGR MUST be ignored
  → Constraint: F-bit in LLGR flags byte (bit 0) means forwarding state preserved
  → Constraint: LLST is 24-bit unsigned (0-16777215 seconds)
- [ ] `rfc/short/rfc4724.md` - GR capability code 64 wire format (for comparison)
  → Constraint: 4 bytes per tuple: AFI(2) + SAFI(1) + Flags(1), different from LLGR's 7

**Key insights:**
- LLGR cap 71 tuples are 7 bytes (vs GR cap 64's 4 bytes) due to 3-byte LLST field
- LLGR capability has no global header (unlike GR which has 2-byte restart-flags/time)
- LLGR F-bit semantics: if set, forwarding state preserved during LLGR period
- Config needs per-family LLST (not global like GR's restart-time)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go` - decodeGR parses cap 64: 2-byte header + 4-byte tuples. extractGRCapabilities reads config JSON per-peer. RunDecodeMode handles "decode capability 64 <hex>".
- [ ] `internal/component/bgp/plugins/gr/gr.go:handleOpenEvent` - finds cap code 64 in OPEN capabilities array, hex-decodes, stores as grPeerCap
- [ ] `internal/component/bgp/plugins/gr/register.go` - CapabilityCodes: []uint8{64}, RFCs: []string{"4724"}
- [ ] `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - presence container "graceful-restart" with leaf "restart-time" (uint16 0-4095, default 120)
- [ ] `internal/component/bgp/attribute/community.go` - Community type (uint32), well-known constants: CommunityNoExport (0xFFFFFF01), CommunityNoAdvertise (0xFFFFFF02), CommunityNoExportSubconfed (0xFFFFFF03), CommunityNoPeer (0xFFFFFF04). String() method has switch for named display.

**Behavior to preserve:**
- GR capability code 64 decode/encode unchanged
- extractGRCapabilities produces CapabilityDecl for code 64 (existing peers not affected)
- RunDecodeMode handles "decode capability 64 <hex>" (existing decode path)
- handleOpenEvent parses code 64 and stores grPeerCap (existing OPEN handling)
- Community String() switch for existing well-known communities

**Behavior to change:**
- register.go: add cap code 71 and RFC 9494
- gr.go: add decodeLLGR function for cap 71 wire format
- gr.go: handleOpenEvent also extracts cap code 71 from OPEN
- gr.go: extractGRCapabilities also produces CapabilityDecl for code 71 from config
- gr.go: RunDecodeMode handles "decode capability 71 <hex>"
- YANG: add long-lived-stale-time leaf list per AFI/SAFI under graceful-restart
- community.go: add CommunityLLGRStale and CommunityNoLLGR constants + String() cases

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config: YANG schema parsed from config file, long-lived-stale-time per family
- OPEN: capability code 71 hex value in received OPEN event JSON
- CLI: "decode capability 71 <hex>" command

### Transformation Path
1. Config parse: YANG -> config tree -> JSON section -> OnConfigure callback -> extractGRCapabilities -> CapabilityDecl for code 71 with encoded LLST per family
2. OPEN event: JSON `capabilities` array -> find code 71 -> hex decode -> decodeLLGR -> llgrPeerCap stored per-peer
3. CLI decode: "decode capability 71 <hex>" -> decodeLLGR -> JSON or text output

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Plugin | YANG schema parsed by config system, JSON delivered in OnConfigure | [ ] |
| OPEN JSON -> Plugin | handleOpenEvent extracts hex value from capabilities array | [ ] |
| Plugin -> Engine (Stage 3) | SetCapabilities with CapabilityDecl code=71 | [ ] |

### Integration Points
- `gr.go:handleOpenEvent` - extend to also look for code 71 in caps loop
- `gr.go:extractGRCapabilities` - extend to also produce code 71 CapabilityDecl
- `gr.go:RunDecodeMode` - extend to handle "decode capability 71"
- `gr.go:RunCLIDecode` - extend or add parallel for cap 71
- `register.go:init()` - CapabilityCodes and RFCs fields

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with long-lived-stale-time | -> | YANG parse + cap 71 extraction | `test/parse/graceful-restart-llgr.ci` |
| OPEN hex with cap 71 | -> | decodeLLGR | `test/decode/capability-llgr.ci` |
| CLI `ze plugin gr --capa <hex>` with cap 71 data | -> | RunCLIDecode extended | Unit test in `gr_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `long-lived-stale-time 3600` for ipv4/unicast under graceful-restart | LLGR capability (code 71) declared in Stage 3 with LLST=3600 for AFI=1/SAFI=1 |
| AC-2 | Received OPEN with capability code 71 hex | decodeLLGR returns per-family LLST and F-bit values |
| AC-3 | LLGR cap 71 with LLST=16777215 (max 24-bit) | Decoded correctly, no overflow |
| AC-4 | LLGR cap 71 with truncated tuple (<7 bytes remaining) | Gracefully ignored (partial tuple skipped) |
| AC-5 | LLGR cap 71 with zero families | Valid empty capability (no error) |
| AC-6 | "decode capability 71 <hex>" in decode mode | JSON output with name, families, LLST per family |
| AC-7 | CLI `--capa <cap71hex>` | Human-readable output showing LLST per family |
| AC-8 | CommunityLLGRStale constant | Value 0xFFFF0006, String() returns "LLGR_STALE" |
| AC-9 | CommunityNoLLGR constant | Value 0xFFFF0007, String() returns "NO_LLGR" |
| AC-10 | Config without long-lived-stale-time | Only GR cap 64 declared (no cap 71) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeLLGR_Basic` | `internal/.../gr/gr_test.go` | Decode LLGR cap with one family, verify LLST and F-bit | |
| `TestDecodeLLGR_MultipleFamilies` | `internal/.../gr/gr_test.go` | Decode with multiple AFI/SAFI tuples | |
| `TestDecodeLLGR_MaxLLST` | `internal/.../gr/gr_test.go` | LLST=16777215 (24-bit max) decoded correctly | |
| `TestDecodeLLGR_Empty` | `internal/.../gr/gr_test.go` | Empty capability (zero families) is valid | |
| `TestDecodeLLGR_TruncatedTuple` | `internal/.../gr/gr_test.go` | Partial tuple (<7 bytes remaining) gracefully ignored | |
| `TestDecodeLLGR_FBitSet` | `internal/.../gr/gr_test.go` | F-bit=1 parsed correctly (bit 0 of flags byte) | |
| `TestExtractGRCapabilities_LLGR` | `internal/.../gr/gr_test.go` | Config with LLST produces both cap 64 and cap 71 | |
| `TestExtractGRCapabilities_NoLLGR` | `internal/.../gr/gr_test.go` | Config without LLST produces only cap 64 | |
| `TestHandleOpenEvent_LLGR` | `internal/.../gr/gr_event_test.go` | OPEN with cap 71 stores LLGR peer capability | |
| `TestRunDecodeMode_LLGR` | `internal/.../gr/gr_test.go` | "decode capability 71 <hex>" returns correct JSON | |
| `TestCommunityLLGRStale` | `internal/.../attribute/community_test.go` | LLGR_STALE constant value and String() output | |
| `TestCommunityNoLLGR` | `internal/.../attribute/community_test.go` | NO_LLGR constant value and String() output | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LLST | 0-16777215 | 16777215 | N/A (0 is valid) | 16777216 (but wire is 24-bit, so can't exceed) |
| LLGR tuple size | 7 bytes per family | 7 | 6 (truncated, skip) | N/A |
| LLGR flags F-bit | bit 0 | 0x80 (F=1) | N/A | N/A (reserved bits ignored) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `graceful-restart-llgr` | `test/parse/graceful-restart-llgr.ci` | Config with long-lived-stale-time parses without error | |
| `capability-llgr` | `test/decode/capability-llgr.ci` | Decode LLGR capability hex produces correct JSON | |

### Future (if deferring any tests)
- None; all capability tests are in this phase

## Files to Modify

- `internal/component/bgp/plugins/gr/register.go` - add cap code 71, RFC "9494"
- `internal/component/bgp/plugins/gr/gr.go` - add decodeLLGR, extend handleOpenEvent, extend extractGRCapabilities, extend RunDecodeMode/RunCLIDecode
- `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - add long-lived-stale-time per family
- `internal/component/bgp/attribute/community.go` - add CommunityLLGRStale, CommunityNoLLGR constants + String() cases

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` |
| RPC count in architecture docs | [ ] | N/A (no new RPCs in this phase) |
| CLI commands/flags | [ ] | Existing `ze plugin gr --capa` extended |
| CLI usage/help text | [ ] | N/A (existing flag, new cap code) |
| API commands doc | [ ] | N/A (no new commands in this phase) |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/parse/graceful-restart-llgr.ci`, `test/decode/capability-llgr.ci` |

## Files to Create

- `test/parse/graceful-restart-llgr.ci` - config parsing test
- `test/decode/capability-llgr.ci` - capability decode test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Community Constants** -- add LLGR_STALE and NO_LLGR to community.go
   - Tests: `TestCommunityLLGRStale`, `TestCommunityNoLLGR`
   - Files: `attribute/community.go`, `attribute/community_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Wire Decode** -- add decodeLLGR function for cap 71
   - Tests: `TestDecodeLLGR_Basic`, `TestDecodeLLGR_MultipleFamilies`, `TestDecodeLLGR_MaxLLST`, `TestDecodeLLGR_Empty`, `TestDecodeLLGR_TruncatedTuple`, `TestDecodeLLGR_FBitSet`
   - Files: `gr.go` (new decodeLLGR function and llgrResult/llgrFamily types)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: OPEN Parsing** -- extend handleOpenEvent for cap 71
   - Tests: `TestHandleOpenEvent_LLGR`
   - Files: `gr.go` (handleOpenEvent), `gr_event_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: YANG + Config** -- add long-lived-stale-time to YANG, extend extractGRCapabilities
   - Tests: `TestExtractGRCapabilities_LLGR`, `TestExtractGRCapabilities_NoLLGR`
   - Files: YANG schema, `gr.go` (extractGRCapabilities, new parseLLGRCapValue)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: CLI Decode** -- extend RunDecodeMode and RunCLIDecode for cap 71
   - Tests: `TestRunDecodeMode_LLGR`
   - Files: `gr.go` (RunDecodeMode, RunCLIDecode)
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Registration** -- update register.go
   - Tests: existing TestAllPluginsRegistered (verify count unchanged)
   - Files: `register.go`
   - Verify: registration succeeds

7. **Functional tests** -- create .ci files
   - Files: `test/parse/graceful-restart-llgr.ci`, `test/decode/capability-llgr.ci`

8. **RFC refs** -- add `// RFC 9494 Section X.Y` comments

9. **Full verification** -- `make ze-verify`

10. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 has implementation with file:line |
| Correctness | LLST is 24-bit (3 bytes big-endian), F-bit is bit 0 of flags byte (0x80 mask) |
| Naming | Community constants follow existing pattern (CommunityXxx), JSON keys kebab-case |
| Data flow | Config -> Stage 3 cap declaration, OPEN -> peer cap storage (same path as GR) |
| Rule: no-layering | LLGR decode is separate function (not modification of GR decode) |
| Rule: buffer-first | Cap 71 encoding for config uses hex string (same as cap 64), no []byte allocation |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| decodeLLGR function | grep for "func decodeLLGR" in `gr.go` |
| Cap 71 in register.go | grep for "71" in `register.go` |
| LLGR YANG config | grep for "long-lived-stale-time" in YANG file |
| CommunityLLGRStale | grep for "CommunityLLGRStale" in `community.go` |
| CommunityNoLLGR | grep for "CommunityNoLLGR" in `community.go` |
| Parse .ci test | ls `test/parse/graceful-restart-llgr.ci` |
| Decode .ci test | ls `test/decode/capability-llgr.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | decodeLLGR: check minimum length, handle truncated tuples without panic |
| LLST overflow | 24-bit field: uint32 sufficient, no overflow in Duration conversion (max ~194 days fits in int64 nanoseconds) |
| Hex decode | hex.DecodeString errors handled (same pattern as existing GR code) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

Add `// RFC 9494 Section 3: "<quoted requirement>"` above LLGR capability decode.
MUST document: wire format (7-byte tuples), F-bit semantics, LLST range, GR prerequisite.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-llgr-1-capability.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
