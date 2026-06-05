# Spec: perf-hot-3 -- Compact Keys for adj-rib-in Hot Path

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/bgp/plugins/adj_rib_in/rib.go` -- handleReceived, RawRoute, ribIn map
4. `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` -- pendingKey, PendingRoute, earlyDecisions
5. `internal/component/bgp/route.go` -- RouteKey function
6. `internal/core/seqmap/seqmap.go` -- generic Map[K, V] (already supports non-string keys)
7. `spec-perf-hot-0-umbrella.md` -- parent spec with profiling evidence

## Task

Replace string-based `RouteKey()` and `pendingKey()` on adj-rib-in's structured hot
path with compact, allocation-free value-type keys. Profiling shows these two functions
produce 2.2M object allocations (0.8M RouteKey + 1.4M pendingKey) totaling ~8 MB per
benchmark run. Each call concatenates strings that heap-allocate. The underlying maps
(`r.ribIn` keyed by seqmap string key, `r.pending` and `r.earlyDecisions` keyed by
pendingKey string) all use string keys that could be replaced with a fixed-size
comparable struct.

Part of spec set `perf-hot` (umbrella: `spec-perf-hot-0-umbrella.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- adj-rib-in raw hex storage design
  -> Constraint: ribIn stores raw hex for replay via "update hex" commands
- [ ] `ai/rules/memory-architecture.md` -- caller-owned buffers, pool strategy
  -> Constraint: value types preferred over pointer types for map keys on hot paths
- [ ] `ai/rules/enum-over-string.md` -- numeric keys not strings on hot paths
  -> Decision: family index as uint16 instead of string in key

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP UPDATE processing
  -> Constraint: route key uniqueness is (AFI, SAFI, NLRI prefix, path-ID when ADD-PATH)
- [ ] `rfc/short/rfc7911.md` -- ADD-PATH path-ID is part of the route key
  -> Constraint: path-ID must be part of the compact key

**Key insights:**
- seqmap is generic (`Map[K comparable, V any]`), already supports non-string keys
- `RouteKey` and `pendingKey` are only called on the hot path in `handleReceived` (lines 460, 501, 520); command handlers (rib_commands.go) use string-based RouteKey from parsed CLI arguments (cold path)
- `pending` and `earlyDecisions` maps both use `pendingKey` string as key
- `buildReplayCommands` iterates ribIn via `seqmap.Since` and ignores the key entirely (line 594: `_ string`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/route.go` (76L) -- RouteKey builds `family + ":" + prefix` or uses textbuf for pathID > 0
  -> Constraint: RouteKey is called from other packages (rib_commands.go, reactor/peersettings.go); the function must remain for non-hot-path callers
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` (627L) -- handleReceived calls RouteKey per NLRI at lines 460 and 520; ribIn declared as `map[string]*seqmap.Map[string, *RawRoute]`; buildReplayCommands discards the key (`_ string` at line 594)
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` (179L) -- pendingKey concatenates `peerAddr + "|" + routeKey`; PendingRoute stores routeKey as string field; pending map is `map[string]*PendingRoute`; earlyDecisions map is `map[string]*EarlyDecision`
- [ ] `internal/core/seqmap/seqmap.go` (133L) -- generic Map[K comparable, V any]; key type is parameterized; no string assumption
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` -- accept-routes and reject-routes parse string arguments, build RouteKey from strings, then call pendingKey (cold path, does not need optimization)

**Behavior to preserve:**
- `RouteKey()` function in `internal/component/bgp/route.go` must continue to exist for cold-path callers
- RPKI validation commands (accept-routes, reject-routes) must still be able to look up pending routes by family, prefix, and pathID parsed from string arguments
- `buildReplayCommands` must continue to replay routes correctly; it only uses the RawRoute value, not the key
- `sweepExpiredPending` and `clearPeerPending` must iterate pending routes correctly
- `earlyDecisions` map must match lookup keys between storeEarlyDecision and applyEarlyDecision

**Behavior to change:**
- `handleReceived` hot path uses compact struct key instead of string RouteKey/pendingKey
- ribIn seqmap parameterized on compact key type instead of string
- pending and earlyDecisions maps keyed by compact struct instead of string

## Data Flow (MANDATORY)

### Entry Point
- Wire UPDATE bytes arrive via `handleReceivedStructured` which builds a `bgp.Event` and calls `handleReceived`
- `handleReceived` iterates `event.FamilyOps`, parses each NLRI to (prefix, pathID), then needs a map key

### Transformation Path
1. Wire NLRI bytes parsed to `(prefix string, pathID uint32)` by `bgp.ParseNLRIValue`
2. Currently: `RouteKey(famStr, prefix, pathID)` builds a string key
3. Currently: `pendingKey(peerAddr, routeKey)` builds a compound string key
4. Key used to insert/lookup in `r.ribIn[peerAddr]` seqmap and `r.pending` map
5. On withdrawal: key used to delete from both maps

After change:
1. Wire NLRI bytes parsed to `(prefix string, pathID uint32)` by `bgp.ParseNLRIValue`
2. Prefix string parsed to `netip.Prefix` via `netip.ParsePrefix`
3. Compact key constructed as value type: `{Family family.Family, Prefix netip.Prefix, PathID uint32}`
4. For pending: compound key `{PeerAddr netip.Addr, Route compactRouteKey}` (peerAddr parsed once per event, not per NLRI)
5. Key used in retyped maps with zero string allocation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RPKI command -> pending map | Command handler builds compact key from parsed string args | [ ] |
| handleReceived -> seqmap | Compact key replaces string key | [ ] |
| handleReceived -> pending map | Compact pending key replaces string pending key | [ ] |
| buildReplayCommands -> seqmap iteration | Key type changes but value is discarded (`_`) | [ ] |

### Integration Points
- `seqmap.Map[K, V]` -- retyped from `seqmap.Map[string, *RawRoute]` to `seqmap.Map[compactRouteKey, *RawRoute]`
- `PendingRoute.routeKey` field -- changes from string to compact type
- `rib_commands.go` accept/reject handlers -- must convert string args to compact key for pending lookup
- `clearPeerPending` -- currently iterates pending map checking `pr.peerAddr`; with compound key, can check `key.PeerAddr` directly

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Wire UPDATE to adj-rib-in plugin | -> | handleReceived with compact keys | TestCompactRouteKeyHandleReceived |
| RPKI accept-routes command | -> | acceptRoutesCommand lookup via compact pending key | TestCompactPendingKeyAcceptRoutes |
| Peer-up replay | -> | buildReplayCommands iterating seqmap with compact key | TestCompactKeyBuildReplay |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k IPv4 unicast routes received by adj-rib-in | Zero RouteKey() calls on the handleReceived path; all map lookups use compact struct key |
| AC-2 | RPKI validation enabled, accept-routes command with (peer, family, prefix, pathID) | Command correctly looks up pending route using compact key built from parsed string arguments |
| AC-3 | RPKI validation enabled, reject-routes command | Command correctly removes pending route using compact key |
| AC-4 | Peer goes down then up, buildReplayCommands called | Replay iterates compact-keyed seqmap, produces identical "update hex" commands as before |
| AC-5 | clearPeerPending called after peer-down | All pending and earlyDecision entries for the peer are removed |
| AC-6 | sweepExpiredPending with timed-out entries | Expired entries promoted correctly using compact key |
| AC-7 | Benchmark allocation profile | RouteKey and pendingKey no longer appear in hot-path alloc profile (may still appear in cold command-handler path) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCompactRouteKeyEquality` | `adj_rib_in/compact_key_test.go` | Same (family, prefix, pathID) produces equal keys; different values produce unequal keys | |
| `TestCompactRouteKeyAsMapKey` | `adj_rib_in/compact_key_test.go` | Compact key works correctly as Go map key (insert, lookup, delete) | |
| `TestCompactPendingKeyEquality` | `adj_rib_in/compact_key_test.go` | Compound (peerAddr, routeKey) keys match correctly | |
| `TestCompactKeyHandleReceived` | `adj_rib_in/rib_test.go` | handleReceived stores and retrieves routes with compact keys | |
| `TestCompactKeyWithdrawal` | `adj_rib_in/rib_test.go` | Withdrawal deletes the correct route by compact key | |
| `TestCompactKeyPendingRPKI` | `adj_rib_in/rib_validation_test.go` | Pending route stored and looked up with compact pending key | |
| `TestCompactKeyAcceptRoutes` | `adj_rib_in/rib_validation_test.go` | accept-routes command converts string args to compact key and finds pending route | |
| `TestCompactKeyRejectRoutes` | `adj_rib_in/rib_validation_test.go` | reject-routes command converts string args to compact key and removes pending route | |
| `TestCompactKeyEarlyDecision` | `adj_rib_in/rib_validation_test.go` | Early decision stored and matched using compact key | |
| `TestCompactKeyBuildReplay` | `adj_rib_in/rib_test.go` | buildReplayCommands iterates compact-keyed seqmap and produces correct commands | |
| `TestCompactKeyClearPeerPending` | `adj_rib_in/rib_validation_test.go` | clearPeerPending removes entries for the target peer only | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PathID | 0 to 2^32-1 | 4294967295 | N/A (uint32) | N/A (uint32) |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 (netip.Prefix rejects) |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 (netip.Prefix rejects) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing adj-rib-in functional tests | `test/plugin/*.ci` | Verify no regression in route storage and replay | |

### Interop Tests (MANDATORY for protocol features)
N/A. This is an internal optimization with no wire-format or protocol behavior change.

### Future (if deferring any tests)
- None deferred

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib.go` -- change ribIn seqmap key type; change handleReceived to build compact keys; update newSeqMap; update buildReplayCommands signature
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` -- change pending and earlyDecisions map key types; change PendingRoute.routeKey field; update pendingKey to build compact type; update promoteToInstalled, sweepExpiredPending, clearPeerPending, removePending, applyEarlyDecision, storeEarlyDecision
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` -- update accept-routes and reject-routes to build compact key from parsed string args
- `internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go` -- update test helpers to use compact key types

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create
- `internal/component/bgp/plugins/adj_rib_in/compact_key.go` -- compact route key type definition and construction helpers
- `internal/component/bgp/plugins/adj_rib_in/compact_key_test.go` -- unit tests for key equality, map behavior

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

1. **Phase: Compact Key Type** -- define the value-type key structs
   - Tests: TestCompactRouteKeyEquality, TestCompactRouteKeyAsMapKey, TestCompactPendingKeyEquality
   - Files: compact_key.go, compact_key_test.go
   - Verify: keys compare equal/unequal correctly; work as Go map keys

2. **Phase: Retype ribIn seqmap** -- change seqmap parameterization from string to compact key
   - Tests: TestCompactKeyHandleReceived, TestCompactKeyWithdrawal, TestCompactKeyBuildReplay
   - Files: rib.go (ribIn type, newSeqMap, handleReceived add path, handleReceived del path, buildReplayCommands)
   - Verify: routes stored and retrieved by compact key; replay produces identical commands

3. **Phase: Retype pending and earlyDecisions** -- change validation maps to compact pending key
   - Tests: TestCompactKeyPendingRPKI, TestCompactKeyEarlyDecision, TestCompactKeyClearPeerPending
   - Files: rib_validation.go (pending map, earlyDecisions map, PendingRoute.routeKey, pendingKey func, all helper functions)
   - Verify: pending routes stored and found by compact key; early decisions match correctly

4. **Phase: Update command handlers** -- accept-routes and reject-routes build compact key from string args
   - Tests: TestCompactKeyAcceptRoutes, TestCompactKeyRejectRoutes
   - Files: rib_commands.go (acceptRoutesCommand, rejectRoutesCommand)
   - Verify: commands parse string args to compact key and find/remove pending routes

5. **Phase: Update existing tests** -- migrate existing test code to new key types
   - Tests: all existing rib_validation_test.go tests
   - Files: rib_validation_test.go
   - Verify: all existing tests pass with compact keys

6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Compact key equality matches string RouteKey equality semantics exactly (same family+prefix+pathID = same key) |
| Naming | Compact key type uses idiomatic Go naming; no exported types needed (package-internal) |
| Data flow | No string RouteKey or pendingKey calls remain on handleReceived hot path |
| Rule: enum-over-string | Family stored as value type, not string, in the compact key |
| Rule: no-sprintf-alloc | No fmt.Sprintf in compact key construction |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| compact_key.go exists | `ls internal/component/bgp/plugins/adj_rib_in/compact_key.go` |
| compact_key_test.go exists | `ls internal/component/bgp/plugins/adj_rib_in/compact_key_test.go` |
| No RouteKey calls in handleReceived | `grep -n 'RouteKey' internal/component/bgp/plugins/adj_rib_in/rib.go` shows zero hits in handleReceived |
| No pendingKey string concat on hot path | `grep -n 'pendingKey' internal/component/bgp/plugins/adj_rib_in/rib.go` shows zero hits in handleReceived |
| All tests pass | `make ze-unit-test` output |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Compact key constructed from parsed netip.Prefix; invalid prefixes rejected by netip.ParsePrefix |
| Map key collision | Verify compact key struct has no padding that could cause spurious inequality |

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

## Core Insight

The seqmap is already generic on key type, so retyping from string to a compact struct
key requires no changes to the seqmap package itself. The change is entirely within
adj_rib_in. The key insight is that `buildReplayCommands` discards the key (`_ string`),
so the key type change has zero impact on the replay path.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Value-type struct key over byte-array key | Fixed [20]byte array, hash-based uint64 key | Struct key is readable, compiler-checked, and Go maps handle struct keys efficiently. Byte array loses type safety. Hash has collision risk. |
| Keep RouteKey() function for cold paths | Remove it entirely, inline compact key everywhere | Command handlers parse string arguments and need string-to-key conversion. RouteKey() serves the cold path without change. |
| netip.Prefix in key (not raw bytes) | Raw [17]byte (1 prefix-len + 16 addr), string | netip.Prefix is a comparable value type, 24 bytes, no allocation. Raw bytes would need manual comparison. |
| netip.Addr for peerAddr in pending key | Keep string peerAddr | peerAddr is already parsed from wire; netip.Addr is 24 bytes comparable, avoids string alloc. The cold command path can parse the string argument once. |

## Known Limitations
- Non-unicast families (VPN, EVPN) use complex NLRI keys that do not fit netip.Prefix. These routes go through `wireNLRIsToAny` which returns string representations. The compact key optimization applies only to unicast families (IPv4/IPv6 unicast/multicast). Non-unicast routes will continue using string keys on the handleReceived path.
- If `spec-perf-hot-2` removes the `handleReceived` call from the structured path entirely, this spec's hot-path impact is reduced to the legacy event path only. The compact key types would still benefit any remaining callers.

## RFC Documentation

No RFC-specific documentation needed. Route key uniqueness follows RFC 4271 Section 9.1
(AFI/SAFI + NLRI) and RFC 7911 (path-ID as part of route identity).

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
| Eliminate RouteKey string allocation on hot path | Benchmark alloc profile | RouteKey absent from pprof alloc profile of handleReceived |
| Eliminate pendingKey string allocation on hot path | Benchmark alloc profile | pendingKey absent from pprof alloc profile |
| No regression in route storage/replay | Functional test | Existing adj-rib-in functional tests pass |
| No regression in RPKI validation | Unit test | Validation tests pass with compact keys |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled after review]

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
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled, 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `rules/quality.md`, no failures)

### Quality Gates (SHOULD pass, defer with user approval)
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

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes, all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
