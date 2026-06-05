# Spec: perf-hot-4-rs-withdrawal-keys

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
3. `ai/rules/memory-architecture.md` - allocation rules
4. `internal/component/bgp/plugins/rs/server_inventory.go` - NLRI extraction and withdrawal map update
5. `internal/component/bgp/plugins/rs/server_withdrawal.go` - processForward and text path
6. `internal/component/bgp/plugins/rs/server_handlers.go` - handleStateDown and sendBatchedWithdrawals
7. `internal/component/bgp/plugins/rs/server.go` - RouteServer struct and withdrawalInfo type

## Task

Replace string-keyed withdrawal maps in the route-server plugin with value-type struct keys
(`family.Family` + `netip.Prefix`) to eliminate per-NLRI `Prefix.String()` calls and string
concatenation on the forwarding hot path.

Profiling shows `applyNLRIRecords` allocates 87 MB flat and 2.5M objects per benchmark run
(100k routes, 5 iterations). Every unicast NLRI in every UPDATE triggers `rec.prefix.String()`
(heap allocation) and string concatenation for the map key (`familyName + "|" + key`). The
`nlriRecord` struct already carries a parsed `netip.Prefix` (value type, zero-alloc), but
`applyNLRIRecords` converts it back to string solely for use as a map key.

This is spec 4 of the `perf-hot` set (umbrella: `spec-perf-hot-0-umbrella.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/memory-architecture.md` - allocation rules for hot paths
  -> Constraint: no heap allocation per UPDATE on hot paths; use value types for map keys
- [ ] `ai/rules/no-sprintf-alloc.md` - hot path definition
  -> Constraint: processForward and applyNLRIRecords are hot path (called per UPDATE)
- [ ] `ai/rules/enum-over-string.md` - numeric keys over string keys on hot paths
  -> Decision: use struct key with value-type fields instead of string concatenation

### RFC Summaries (MUST for protocol work)
Not applicable. This is an internal data structure change; no wire format or protocol behavior changes.

**Key insights:**
- `nlriRecord.prefix` is already a `netip.Prefix` value type parsed from wire bytes in `appendUnicastRecords`
- `applyNLRIRecords` calls `rec.prefix.String()` only to build a string map key, then discards the string after insertion
- The withdrawal map is read on peer-down (cold path) in `handleStateDown` -> `sendBatchedWithdrawals`
- `sendBatchedWithdrawals` iterates entries grouped by `info.Family` string and uses `info.Prefix` in withdrawal commands
- The text path (`updateWithdrawalMapText`) uses string NLRIs from parsed text events and must continue working
- `family.Family` is `struct { AFI AFI; SAFI SAFI }` (comparable, hashable)
- `netip.Prefix` is a Go stdlib value type (comparable, hashable)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server.go:113-145` - `withdrawalInfo` struct (Family string, Prefix string); `RouteServer.withdrawals` field: `map[string]map[string]withdrawalInfo`, keyed by sourcePeer -> routeKey
  -> Constraint: `withdrawalInfo.Prefix` is used in `sendBatchedWithdrawals` for "update text nlri <family> del <prefix>" commands
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go:180-206` - `applyNLRIRecords`: for unicast records (nlriStr == ""), calls `rec.prefix.String()` then builds `familyName + "|" + key` as map key; for non-unicast (nlriStr != ""), uses `familyName + "|" + nlriKey(rec.nlriStr)` as map key
  -> Constraint: non-unicast path uses nlriStr (string NLRI), not netip.Prefix
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go:17-26` - `nlriRecord` struct: has `prefix netip.Prefix` (unicast) and `nlriStr string` (non-unicast)
  -> Decision: unicast records already carry parsed prefix as value type
- [ ] `internal/component/bgp/plugins/rs/server_withdrawal.go:139-164` - `updateWithdrawalMapText`: text path uses string NLRIs; must continue to work with whatever map type is chosen
  -> Constraint: text path must store entries compatible with the new map structure
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go:75-127` - `handleStateDown` extracts entries under withdrawalMu, passes to `sendBatchedWithdrawals`; groups by `info.Family` string, builds "update text nlri <fam> del <prefix>" commands
  -> Constraint: withdrawal command generation needs family string and prefix string; these can be derived from value types on this cold path
- [ ] `internal/component/bgp/plugins/rs/server_test.go:40,86-91,117-119,148-166` - tests construct and inspect `withdrawals` map directly by string keys like `"ipv4/unicast|10.0.0.0/24"`
  -> Constraint: tests must be updated to use new key type

**Behavior to preserve:**
- Withdrawal commands on peer-down produce identical "update text nlri <family> del <prefix>" strings
- Text-path withdrawal map updates (`updateWithdrawalMapText`) continue working
- Non-unicast NLRI withdrawal tracking (nlriStr path) continues working
- Concurrent safety: `withdrawalMu` protects all map access

**Behavior to change:**
- Hot-path `applyNLRIRecords` for unicast records: eliminate `Prefix.String()` and string concat
- Inner withdrawal map key type: from `string` ("family|prefix") to a struct key containing `family.Family` + `netip.Prefix`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Wire UPDATE bytes arrive at reactor, forwarded to route-server plugin via DirectBridge
- `processForward` in `server_withdrawal.go` calls `extractWireNLRIRecords` before forwarding

### Transformation Path
1. Wire bytes -> `extractWireNLRIRecords` -> `appendUnicastRecords` -> `nlriRecord` with `prefix netip.Prefix` (zero-alloc)
2. After forwarding: `applyNLRIRecords` reads `nlriRecord.prefix`, **currently** calls `.String()` to build map key -> **change**: use `prefix` directly as part of struct key
3. On peer-down: `handleStateDown` -> `sendBatchedWithdrawals` iterates map, groups by family, builds withdrawal command strings (**cold path**, string conversion acceptable here)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> RS plugin | DirectBridge structured event delivery | [ ] |
| RS plugin internal | withdrawalMu-protected map access | [ ] |
| RS plugin -> Engine | "update text nlri ... del ..." RPC commands | [ ] |

### Integration Points
- `processForward` calls `applyNLRIRecords` after forwarding, under `withdrawalMu.Lock`
- `handleStateDown` reads and clears the map under `withdrawalMu.Lock`, then generates withdrawal commands in a goroutine
- `updateWithdrawalMapText` (text path) also writes to the withdrawal map under `withdrawalMu.Lock`
- `newTestRouteServer` in tests initializes the withdrawal map

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `processForward` with wire UPDATE | -> | `applyNLRIRecords` with struct-keyed map | `TestApplyNLRIRecords_StructKey_Unicast` |
| `processForward` with text UPDATE | -> | `updateWithdrawalMapText` with struct-keyed map | `TestUpdateWithdrawalMapText_StructKey` |
| `handleStateDown` | -> | `sendBatchedWithdrawals` reading struct-keyed map | `TestHandleStateDown_StructKeyWithdrawals` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Unicast UPDATE with 100 NLRIs processed via wire path | All 100 entries inserted in withdrawal map with zero `Prefix.String()` calls during `applyNLRIRecords` |
| AC-2 | Unicast withdrawal UPDATE via wire path | Matching entries removed from withdrawal map using struct key lookup (no string conversion) |
| AC-3 | Non-unicast UPDATE via wire path (VPN/EVPN with nlriStr) | Entries stored and retrieved correctly using string-based sub-key for non-unicast NLRIs |
| AC-4 | Text-path UPDATE (legacy fork mode) | `updateWithdrawalMapText` stores entries compatible with the new map structure |
| AC-5 | Peer-down triggers `handleStateDown` | `sendBatchedWithdrawals` generates identical withdrawal commands as before (string conversion on cold path) |
| AC-6 | Mixed unicast + non-unicast in same UPDATE | Both stored correctly in the same per-peer map |
| AC-7 | Benchmark allocation profile | `applyNLRIRecords` no longer appears in pprof alloc top-20 for unicast workload |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyNLRIRecords_StructKey_Unicast` | `internal/component/bgp/plugins/rs/server_test.go` | AC-1: unicast insert uses struct key, no string conversion | |
| `TestApplyNLRIRecords_StructKey_Withdraw` | `internal/component/bgp/plugins/rs/server_test.go` | AC-2: unicast delete uses struct key lookup | |
| `TestApplyNLRIRecords_StructKey_NonUnicast` | `internal/component/bgp/plugins/rs/server_test.go` | AC-3: non-unicast entries use string sub-key | |
| `TestUpdateWithdrawalMapText_StructKey` | `internal/component/bgp/plugins/rs/server_test.go` | AC-4: text path writes compatible entries | |
| `TestSendBatchedWithdrawals_StructKey` | `internal/component/bgp/plugins/rs/server_test.go` | AC-5: cold-path command generation produces correct strings | |
| `TestApplyNLRIRecords_StructKey_Mixed` | `internal/component/bgp/plugins/rs/server_test.go` | AC-6: mixed unicast + non-unicast in same peer map | |

### Boundary Tests (MANDATORY for numeric inputs)
Not applicable. No new numeric inputs introduced.

### Functional Tests
Existing performance benchmark (`test/perf/run.py` with `PPROF=1`) validates AC-7 after implementation.
No new functional test files needed; existing `test/plugin/` route-server tests cover the user-visible behavior.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing RS functional tests | `test/plugin/` | Route server forwards and withdraws correctly | |

### Interop Tests (MANDATORY for protocol features)
Not applicable. No wire protocol changes. Internal data structure optimization only.

### Future (if deferring any tests)
- None. All tests implementable in this spec.

## Files to Modify
- `internal/component/bgp/plugins/rs/server.go` - Change `withdrawalInfo` type and `withdrawals` map type on `RouteServer` struct
- `internal/component/bgp/plugins/rs/server_inventory.go` - Change `applyNLRIRecords` to use struct key instead of string key
- `internal/component/bgp/plugins/rs/server_withdrawal.go` - Change `updateWithdrawalMapText` to write compatible entries
- `internal/component/bgp/plugins/rs/server_handlers.go` - Change `handleStateDown`/`sendBatchedWithdrawals` to read new map structure; generate strings on cold path
- `internal/component/bgp/plugins/rs/server_test.go` - Update all test assertions that inspect `withdrawals` map by string key

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | - |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | - |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Files to Create
- None. All changes are modifications to existing files.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- confirm existing callers of applyNLRIRecords |
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
| 14. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Data types** -- define struct key type and update withdrawalInfo
   - Define a `withdrawalKey` struct with `fam family.Family` and `prefix netip.Prefix` fields (unicast path)
   - Decide handling for non-unicast: either a separate map or a union key with optional string field
   - Change `withdrawals` field type on `RouteServer` from `map[string]map[string]withdrawalInfo` to use new key type
   - Update `newRouteServer` initialization
   - Tests: `TestApplyNLRIRecords_StructKey_Unicast` (write failing test first)
   - Files: `server.go`
   - Verify: test fails because applyNLRIRecords still writes string keys

2. **Phase: Wire path (applyNLRIRecords)** -- eliminate Prefix.String() on hot path
   - Change `applyNLRIRecords` to use `withdrawalKey{fam, prefix}` for unicast records
   - Store `withdrawalInfo` with family and prefix as value types; generate string only on cold-path read
   - Preserve non-unicast (nlriStr) path
   - Tests: `TestApplyNLRIRecords_StructKey_Withdraw`, `TestApplyNLRIRecords_StructKey_NonUnicast`, `TestApplyNLRIRecords_StructKey_Mixed`
   - Files: `server_inventory.go`
   - Verify: all applyNLRIRecords tests pass

3. **Phase: Text path (updateWithdrawalMapText)** -- make text path compatible
   - Change `updateWithdrawalMapText` to write entries using the new key type
   - Text path provides string NLRIs; parse prefix from "prefix X.X.X.X/N" format to build struct key, or use string sub-key for non-parseable NLRIs
   - Tests: `TestUpdateWithdrawalMapText_StructKey`
   - Files: `server_withdrawal.go`
   - Verify: text path tests pass

4. **Phase: Cold path (sendBatchedWithdrawals)** -- generate strings on peer-down
   - Change `sendBatchedWithdrawals` to read new map structure
   - Group by `fam` (use `fam.String()` on cold path), generate prefix strings on cold path
   - Tests: `TestSendBatchedWithdrawals_StructKey`
   - Files: `server_handlers.go`
   - Verify: withdrawal command generation produces identical strings

5. **Phase: Test migration** -- update existing tests
   - Update `TestHandleUpdate_ZeBGPFormat`, `TestHandleUpdate_Withdraw_ZeBGPFormat`, `TestHandleUpdate_MultiFamilyMixed` to inspect new map structure
   - Update `newTestRouteServer` initialization
   - Files: `server_test.go`
   - Verify: all existing tests pass

6. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Non-unicast path still works; text path produces compatible entries |
| Naming | New struct type follows ze naming conventions |
| Data flow | `Prefix.String()` is not called anywhere in `applyNLRIRecords` for unicast records |
| Rule: no-sprintf-alloc | No `fmt.Sprintf` on modified hot paths |
| Rule: enum-over-string | Map keys use value types, not string concatenation |
| Allocation profile | Run `PPROF=1` benchmark; confirm applyNLRIRecords drops from alloc profile |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `withdrawalKey` struct type defined | `grep 'type withdrawalKey' server.go` |
| `applyNLRIRecords` uses struct key for unicast | `grep -v 'String()' server_inventory.go` in applyNLRIRecords function |
| `sendBatchedWithdrawals` reads new map structure | `grep 'withdrawalKey' server_handlers.go` |
| All RS unit tests pass | `go test ./internal/component/bgp/plugins/rs/...` |
| No Prefix.String() in hot path | pprof allocs profile with `PPROF=1` benchmark |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `netip.Prefix` values from wire bytes are already validated by `appendUnicastRecords`; no new input paths |
| Map size | Per-peer withdrawal map grows with announced routes; same bound as before (max-prefix config) |
| Concurrent access | All map access protected by `withdrawalMu`; no change to locking discipline |

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

## Core Insight

The `nlriRecord` struct already carries the optimal representation (`netip.Prefix`, a 20-byte
value type parsed from wire bytes). The allocation waste comes from converting this back to a
string solely for use as a map key. The fix is to use the value type directly as the key, and
defer string conversion to the cold path (peer-down withdrawal generation).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Struct key with `family.Family` + `netip.Prefix` | (a) Interned string pool, (b) Byte-slice key via encoding | Struct key is zero-alloc (both fields are value types, Go handles hashing natively); interning adds complexity; byte-slice keys require custom hashing |
| Keep non-unicast as string sub-key within same map | (a) Separate map for non-unicast, (b) Union key with discriminant | Non-unicast is rare (VPN/EVPN); separate map adds complexity for negligible gain; union key with string field is simpler |
| Generate strings on cold path only | (a) Cache string in withdrawalInfo, (b) Pre-format during applyNLRIRecords | Cold path (peer-down) happens at most once per peer session; string generation there is negligible |

## Known Limitations
- Non-unicast (VPN, EVPN, BGP-LS) NLRIs still allocate strings via `nlriStr`; this is acceptable because non-unicast traffic is rare in the benchmark and not on the critical path for unicast forwarding
- Text-path NLRIs (legacy fork mode) still use string parsing; optimization of that path is out of scope

## RFC Documentation

Not applicable. No RFC-governed behavior changed.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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
| Eliminate Prefix.String() in applyNLRIRecords hot path | benchmark + pprof | PPROF=1 benchmark shows applyNLRIRecords absent from alloc top-20 |
| Withdrawal commands on peer-down remain identical | unit test | TestSendBatchedWithdrawals_StructKey output matches expected commands |
| Non-unicast and text paths continue working | unit test | TestApplyNLRIRecords_StructKey_NonUnicast, TestUpdateWithdrawalMapText_StructKey |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

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
- [ ] AC-1..AC-7 all demonstrated
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
