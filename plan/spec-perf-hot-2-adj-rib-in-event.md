# Spec: perf-hot-2-adj-rib-in-event

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/no-sprintf-alloc.md` - hot-path allocation rules
4. `ai/rules/memory-architecture.md` - data lifecycle and buffer strategy
5. `internal/component/bgp/plugins/adj_rib_in/rib.go` - main file being modified
6. `internal/component/bgp/nlri/iterator.go` - NLRIIterator used by new code path

## Task

Eliminate the intermediate `bgp.Event` construction and `wireNLRIsToAny()` boxing in
`AdjRIBInManager.handleReceivedStructured()`. The structured handler currently builds a
full `bgp.Event` with `FamilyOps` containing `[]any` prefix strings, then calls
`handleReceived()` which unboxes them via `ParseNLRIValue()`. This round-trip allocates
2.5M objects (71 MB) for boxing alone, plus another 3.6M objects (213 MB) in the
receiver for `RouteKey`, `RawRoute` structs, and hex encoding. The new structured path
will walk wire bytes directly using `nlri.NLRIIterator`, skipping the `bgp.Event`
intermediary entirely.

Part of spec set `perf-hot` (umbrella: `spec-perf-hot-0-umbrella.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-sprintf-alloc.md` - hot-path allocation rules
  -> Constraint: no `fmt.Sprintf`, no `Prefix.String()` in hot loops; use append-based alternatives
- [ ] `ai/rules/memory-architecture.md` - data lifecycle and buffer strategy
  -> Constraint: caller-owned buffers preferred; avoid per-call allocation
- [ ] `docs/architecture/plugin/rib-storage-design.md` - adj-rib-in storage model
  -> Decision: routes stored as raw hex for replay via "update hex" commands
- [ ] `ai/rules/buffer-first.md` - wire encoding constraints
  -> Constraint: zero-copy where possible; views into original buffer

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP UPDATE message format
  -> Constraint: NLRI format is [length][prefix-bytes], withdrawn same format
- [ ] `rfc/short/rfc7911.md` - ADD-PATH
  -> Constraint: path-id prepended before each NLRI when negotiated

**Key insights:**
- `handleReceivedStructured` is the hot path for internal (DirectBridge) plugins
- `handleReceived` is still needed for external text/JSON plugins
- `wireNLRIsToAny` converts wire bytes to `[]any{string}` only to have `ParseNLRIValue` convert them back to string
- `RawRoute` stores hex strings (AttrHex, NHopHex, NLRIHex) for replay commands
- Hex encoding is only needed for routes that get installed in ribIn, not for every NLRI in every UPDATE
- `NLRIIterator` returns views into the original buffer (zero-copy)
- The RPKI validation path (`r.validationEnabled`) must work identically

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:208-309` - `handleReceivedStructured`: builds `bgp.Event` with 4-6 map allocations, calls `wireNLRIsToAny` 1-4 times (one per NLRI section), then calls `dispatch` which routes to `handleReceived`
  -> Constraint: must continue to handle IPv4 unicast (body NLRI + Withdrawn) and multiprotocol (MP_REACH + MP_UNREACH) sections
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:326-366` - `wireNLRIsToAny`: walks wire bytes, converts each prefix to `netip.PrefixFrom().String()`, boxes into `[]any`. Uses stack-allocated `[16]byte` buffer for prefix parsing but allocates a string per prefix
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:418-531` - `handleReceived`: iterates `event.FamilyOps`, calls `ParseNLRIValue` to unbox `any` back to string, calls `RouteKey` to build map key, builds `RawRoute` with hex strings, stores in `ribIn` or `pending`
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:615-626` - `nhopToHex`: converts IP string to hex. Called once per add operation
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:631-655` - `splitRawNLRIHex`: splits concatenated hex into individual entries. Only for simple families. Called from `handleReceived` to get per-prefix NLRI hex
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:659-665` - `isSimplePrefixFamily`: IPv4/IPv6 unicast/multicast
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go:670+` - `prefixToWireHex`: fallback for when raw NLRI bytes are unavailable
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go:26-40` - `PendingRoute` struct and `pendingKey` function
- [ ] `internal/component/bgp/nlri/iterator.go:35-90` - `NLRIIterator` walks wire bytes, returns `(prefix []byte, pathID uint32, ok bool)` per NLRI. Zero-copy views into buffer
- [ ] `internal/component/bgp/route.go:69-76` - `RouteKey`: builds `family:prefix` or `family:prefix:pathID` string

**Behavior to preserve:**
- `handleReceived` (the text/JSON event path) must continue to work for external process plugins
- `RawRoute` struct and its fields (AttrHex, NHopHex, NLRIHex, ValidationState) are the storage format used by replay commands
- RPKI validation flow (`r.validationEnabled` branch) including `PendingRoute` construction, `applyEarlyDecision`, and `pendingKey`
- `ribIn` map structure: `map[string]*seqmap.Map[string, *RawRoute]` keyed by peer address then route key
- Route key format: `family:prefix` or `family:prefix:pathID`
- Complex family handling: VPN/EVPN/FlowSpec use the raw NLRI blob directly, not per-prefix split
- `seqCounter` monotonic increment per installed route
- Withdrawal behavior: `removePending` then `ribIn[peer].Delete(routeKey)`

**Behavior to change:**
- `handleReceivedStructured` will process wire bytes directly instead of building a `bgp.Event`
- `wireNLRIsToAny` will no longer be called from the structured path (kept for tests or removed if no other callers)
- Hex encoding of attributes and NLRIs happens per-installed-route, not per-NLRI-in-UPDATE
- Next-hop hex computed from wire bytes directly, not from string intermediary

## Data Flow (MANDATORY)

### Entry Point
- Wire UPDATE bytes arrive via `DirectBridge.DeliverStructured` -> `dispatchStructured` -> `handleReceivedStructured(se *rpc.StructuredEvent)`
- Input: `se.RawMessage.(*bgptypes.RawMessage)` containing `WireUpdate` and `AttrsWire`

### Transformation Path (current, wasteful)
1. `handleReceivedStructured` extracts wire sections (NLRI, Withdrawn, MP_REACH, MP_UNREACH)
2. For each section: calls `wireNLRIsToAny` which converts every wire prefix to `netip.Prefix.String()` and boxes into `[]any` (ALLOCATION: 2.5M objects)
3. Builds `bgp.Event` with `FamilyOps` map, `RawNLRIBytes` map, `AddPath` map (ALLOCATION: 4-6 maps)
4. Calls `dispatch` -> `handleReceived`
5. `handleReceived` iterates `FamilyOps`, calls `ParseNLRIValue` to unbox `any` -> string
6. Calls `GetRawAttributesHex` which lazily hex-encodes `RawAttributeBytes` (ALLOCATION: hex string)
7. Calls `GetRawNLRIHex` which lazily hex-encodes `RawNLRIBytes` (ALLOCATION: hex string)
8. Calls `splitRawNLRIHex` to split the hex back into per-prefix entries (ALLOCATION: `[]string`)
9. Builds `RouteKey` per prefix (ALLOCATION: string concat)
10. Builds `RawRoute` with hex strings (ALLOCATION: struct)
11. Stores in `ribIn` or `pending`

### Transformation Path (proposed, direct)
1. `handleReceivedStructured` extracts wire sections (same as before)
2. For each section: walks wire bytes directly using `nlri.NLRIIterator`
3. Per NLRI: extracts `wirePrefix` and `pathID` from iterator (zero-copy view)
4. Converts wire prefix to string once for route key and storage
5. Hex-encodes attributes and NLRI only for routes that are installed
6. Builds `RawRoute` and stores in `ribIn` or `pending`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Plugin | StructuredEvent with RawMessage | [ ] |
| Plugin -> Storage | RawRoute stored in seqmap | [ ] |

### Integration Points
- `nlri.NLRIIterator` - used for zero-copy wire byte walking
- `nlri.PrefixBytes` - computes byte count from prefix length
- `hex.EncodeToString` - for attribute and NLRI hex encoding (deferred to install time)
- `bgp.RouteKey` - still needed for route key generation (addressed in spec perf-hot-3)
- `attribute.AttributesWire.Packed()` - raw attribute bytes
- `bgpctx.Registry.Get` - encoding context for ADD-PATH flags

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `DirectBridge.DeliverStructured` with UPDATE event | -> | `handleReceivedStructured` direct wire path | `TestHandleReceivedStructuredDirectWire` |
| `DirectBridge.DeliverStructured` with ADD-PATH UPDATE | -> | `handleReceivedStructured` with addPath=true | `TestHandleReceivedStructuredAddPath` |
| `DirectBridge.DeliverStructured` with withdrawal | -> | `handleReceivedStructured` withdrawal path | `TestHandleReceivedStructuredWithdrawal` |
| `DirectBridge.DeliverStructured` with MP_REACH | -> | `handleReceivedStructured` multiprotocol path | `TestHandleReceivedStructuredMPReach` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv4 unicast UPDATE via structured path | Route stored in ribIn with correct AttrHex, NHopHex, NLRIHex, same as current behavior |
| AC-2 | IPv4 unicast withdrawal via structured path | Route removed from ribIn and pending, same as current behavior |
| AC-3 | MP_REACH_NLRI UPDATE via structured path | Route stored with correct family, hex fields, next-hop |
| AC-4 | MP_UNREACH_NLRI withdrawal via structured path | Route removed correctly |
| AC-5 | ADD-PATH UPDATE (pathID != 0) | Route key includes pathID, route stored under correct key |
| AC-6 | RPKI validation enabled | PendingRoute created with correct fields, applyEarlyDecision called |
| AC-7 | Complex family (VPN) UPDATE | Handled correctly (raw blob used, not per-prefix split) |
| AC-8 | Text/JSON event via handleReceived | Legacy path still works unchanged |
| AC-9 | Allocation reduction | pprof alloc profile shows wireNLRIsToAny eliminated from hot path |
| AC-10 | Replay commands | Routes stored via structured path produce identical "update hex" replay output |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleReceivedStructuredDirectWire` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | IPv4 unicast via new direct path stores correct RawRoute | |
| `TestHandleReceivedStructuredDirectWireWithdrawal` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | Withdrawal removes route from ribIn | |
| `TestHandleReceivedStructuredDirectWireAddPath` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | ADD-PATH route key includes pathID | |
| `TestHandleReceivedStructuredDirectWireMPReach` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | MP_REACH stores correct family and next-hop hex | |
| `TestHandleReceivedStructuredDirectWireRPKI` | `internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go` | RPKI validation creates PendingRoute correctly | |
| `TestHandleReceivedStructuredDirectWireReplay` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | Replay produces identical output as legacy path | |
| `TestHandleReceivedLegacyPath` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | Legacy handleReceived still works (regression) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length (IPv4) | 0-32 | 32 | N/A (0 is valid) | 33 (malformed wire) |
| prefix length (IPv6) | 0-128 | 128 | N/A (0 is valid) | 129 (malformed wire) |
| pathID | 0-4294967295 | 4294967295 | N/A (0 means no ADD-PATH) | N/A (uint32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `plugin-adj-rib-in-features.ci` | `test/plugin/` | Plugin features listing (unchanged) | |

### Interop Tests (MANDATORY for protocol features)
Not applicable. This is an internal refactor of the structured event path. No wire protocol behavior changes. The external plugin path (`handleReceived`) is preserved unchanged.

### Future (if deferring any tests)
- Allocation benchmark test comparing old vs new path (deferred, covered by PPROF=1 benchmark run)

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - rewrite `handleReceivedStructured` to walk wire bytes directly; keep `wireNLRIsToAny` if needed by tests or remove if unused
- `internal/component/bgp/plugins/adj_rib_in/rib_test.go` - add new tests for direct wire path, keep existing `handleReceived` tests as regression

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A |
| YANG validation constraints | No | N/A |
| YANG custom validators | No | N/A |
| CLI commands/flags | No | N/A |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A (internal refactor) |
| Pipe completeness | No | N/A |
| Env var registration | No | N/A |
| Doctor check for runtime dependencies | No | N/A |
| Prometheus counters/metrics | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A (internal refactor only) |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A (optimization, not architecture change) |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | N/A |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | Verify: grep `docs/` for `adj_rib_in/rib.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | N/A |

## Files to Create
- None (modifications to existing files only)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- add new direct-wire method skeleton
   - Tests: `TestHandleReceivedStructuredDirectWire` (fails because method is stub)
   - Files: `rib.go` (add `processStructuredUpdate` method skeleton)
   - Verify: test fails, skeleton compiles

2. **Phase: IPv4 unicast announce path** -- handle body NLRI section with NLRIIterator
   - Tests: `TestHandleReceivedStructuredDirectWire` (pass), `TestHandleReceivedStructuredDirectWireReplay` (pass)
   - Files: `rib.go` (implement NLRI walking, hex encoding at install time, RawRoute construction)
   - Verify: IPv4 unicast routes stored identically to legacy path

3. **Phase: Withdrawal + multiprotocol** -- handle Withdrawn, MP_REACH, MP_UNREACH sections
   - Tests: `TestHandleReceivedStructuredDirectWireWithdrawal`, `TestHandleReceivedStructuredDirectWireMPReach`
   - Files: `rib.go`
   - Verify: all NLRI sections handled

4. **Phase: ADD-PATH + RPKI** -- handle path-id and validation
   - Tests: `TestHandleReceivedStructuredDirectWireAddPath`, `TestHandleReceivedStructuredDirectWireRPKI`
   - Files: `rib.go`, `rib_validation_test.go`
   - Verify: path-id in route key, PendingRoute created correctly

5. **Phase: Switch over** -- replace `handleReceivedStructured` body with direct path
   - Tests: `TestHandleReceivedLegacyPath` (regression)
   - Files: `rib.go` (replace `handleReceivedStructured` body, keep `handleReceived` intact)
   - Verify: all tests pass, legacy path regression test passes

6. **Functional tests** -- verify existing functional tests still pass
7. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Hex encoding produces identical output to legacy path for same input |
| Correctness | Complex family (VPN/EVPN) handling preserved or explicitly deferred |
| Correctness | RPKI validation PendingRoute fields match legacy path exactly |
| Data flow | Wire bytes -> NLRIIterator -> hex encode at install -> ribIn. No intermediate Event |
| Hot-path allocation | No `Prefix.String()`, no `[]any` boxing, no `map[family.Family]` allocation per UPDATE |
| Rule: buffer-first | NLRIIterator returns views into original buffer, not copies |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `handleReceivedStructured` walks wire bytes directly | grep for `NLRIIterator` or direct wire walking in rib.go |
| `wireNLRIsToAny` not called from structured path | grep for `wireNLRIsToAny` in rib.go, verify only test/legacy callers |
| `handleReceived` preserved for legacy path | grep for `handleReceived` method, verify it exists unchanged |
| All tests pass | `make ze-unit-test` output |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Wire NLRI bytes validated by NLRIIterator bounds checking (prefix length vs data length) |
| Buffer safety | NLRIIterator views into buffer must not outlive the buffer; hex encoding creates owned copies for storage |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, check hex encoding |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong, redesign; if AC correct, fix implementation |
| Replay output differs | Compare hex encoding byte-by-byte with legacy path |
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

## Core Insight

The `handleReceivedStructured` -> `bgp.Event` -> `handleReceived` chain was built when
adj-rib-in only received text/JSON events from external plugins. When DirectBridge
structured events were added, the handler converted wire bytes into the legacy Event
format rather than processing them directly. This indirection is the single largest
allocation source in the plugin: 2.5M boxing allocations + 213 MB of downstream
unpacking per benchmark run.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Walk wire bytes directly in handleReceivedStructured | (a) Cache prefix strings across plugins, (b) Use netip.Prefix as map key instead of string | Direct walking eliminates the Event intermediary entirely; caching would add complexity and still allocate on first sight; netip.Prefix keys would require changing the RawRoute storage model |
| Keep handleReceived for legacy path | Remove it entirely | External process plugins still use the text/JSON event path; removing would break backward compatibility |
| Hex-encode at install time, not per-NLRI | Pre-compute all hex strings in a batch | Most UPDATEs in a benchmark carry the same attributes; encoding only on install avoids encoding for routes that are duplicates or withdrawals |

## Known Limitations
- `RouteKey` string allocation remains (addressed by spec perf-hot-3)
- `pendingKey` string allocation remains (addressed by spec perf-hot-3)
- Complex family (VPN/EVPN) NLRI handling still uses the allocating path because their wire format is not simple prefix+length

## RFC Documentation

No new RFC enforcement added. Existing NLRIIterator already documents RFC 4271 and RFC 7911 compliance.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Eliminate wireNLRIsToAny from hot path | pprof alloc profile | PPROF=1 benchmark showing function absent from top allocators |
| Preserve legacy handleReceived path | unit test | TestHandleReceivedLegacyPath passes |
| Identical storage for replay | unit test | TestHandleReceivedStructuredDirectWireReplay compares hex output |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
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

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
