# Spec: perf-hot-0 -- Forwarding Hot-Path Allocation Reduction (Umbrella)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/core-design.md` -- component lifecycle, plugin bridge
4. `ai/rules/memory-architecture.md` -- allocation discipline, pool strategy
5. `ai/rules/no-sprintf-alloc.md` -- hot-path allocation rules
6. Child specs: `spec-perf-hot-1-*` through `spec-perf-hot-5-*`

## Task

Eliminate hot-path allocation overhead in ze's BGP forwarding pipeline. Profiling
(pprof CPU + alloc + GC trace, 100k routes, 5 iterations, PPROF=1) shows ze allocates
1.09 GB across 17.9M objects per benchmark run. GC pauses reach 53ms, directly
contributing to p99 latency variance. Bird's throughput stddev is 29k; ze's is 511k.

The root cause is string-keyed maps and redundant `Prefix.String()` conversions on
every NLRI in every UPDATE, across three independent plugin stages (adj-rib-in, rib,
route-server). Each stage independently converts wire bytes to prefix strings for map
keys, then discards the wire bytes. The same prefix is stringified 4 times per UPDATE.

### Profiling Evidence

CPU profile (2.69s sampled over 45s, 5.98% utilization, 2 cores):

| Function | Cumulative | Flat | Description |
|----------|-----------|------|-------------|
| `rib.handleReceivedStructured` | 56.5% | 3.0% | Best-path check per UPDATE |
| `rib.checkBestPathChange` | 36.4% | 0.4% | Per-NLRI candidate gathering |
| `runtime.mallocgc` | 14.9% | 3.4% | Heap allocation |
| `adj_rib_in.handleReceivedStructured` | 10.8% | 0% | Event construction |
| `runtime.gcBgMarkWorker` | 10.0% | 0% | GC mark phase |
| `rs.applyNLRIRecords` | 7.4% | 0.4% | Withdrawal tracking |
| `Prefix.String` | 7.8% | 0.4% | String conversion |
| `runtime.concatstrings` | 6.3% | 1.1% | String concat for map keys |

Allocation profile (17.9M objects, 1.09 GB total):

| Function | Objects | Bytes | What |
|----------|---------|-------|------|
| `Prefix.String` | 5.3M | 80 MB | String per prefix |
| `adj_rib_in.handleReceived` | 3.6M | 213 MB | RouteKey + RawRoute structs |
| `adj_rib_in.wireNLRIsToAny` | 2.5M | 71 MB | `[]any` boxing of prefix strings |
| `rs.applyNLRIRecords` | 2.5M | 112 MB | prefix.String + string concat |
| `adj_rib_in.pendingKey` | 1.4M | 4.5 MB | peerAddr + routeKey concat |
| `bgp.RouteKey` | 0.8M | 3.5 MB | family + prefix concat |
| bart tree nodes | 2.4M | 130 MB | RIB trie insertions (structural, not reducible) |

GC trace (peak per iteration):

| GC # | Pre | Post | Live | STW clock | Notes |
|------|-----|------|------|-----------|-------|
| gc 20 | 153 MB | 167 MB | 102 MB | 53 ms | Worst pause |
| gc 24 | 172 MB | 176 MB | 38 MB | 11 ms | Post-cleanup |

### Child Specs

Each child spec addresses one allocation site. They are independent of each other
(each touches different files) and ordered by allocation impact.

| # | Spec | Target | Objects Eliminated | Bytes Saved |
|---|------|--------|-------------------|-------------|
| 1 | `spec-perf-hot-1-prefix-string.md` | `wirePrefixToString` + `Prefix.String` in rib `storeSentEntries` / `removeSentNLRIs` | 5.3M | 80 MB |
| 2 | `spec-perf-hot-2-adj-rib-in-event.md` | `wireNLRIsToAny` + intermediate `bgp.Event` construction in adj-rib-in | 2.5M | 71 MB |
| 3 | `spec-perf-hot-3-adj-rib-in-keys.md` | `RouteKey` + `pendingKey` string concat in adj-rib-in `handleReceived` | 2.2M | 8 MB |
| 4 | `spec-perf-hot-4-rs-withdrawal-keys.md` | `prefix.String` + string concat in route-server `applyNLRIRecords` | 2.5M | 112 MB |
| 5 | `spec-perf-hot-5-rib-changesbyfamily.md` | `changesByFamily` map allocation in rib `handleReceivedStructured` | minor | ~2 MB |

### Design Principles

| Principle | Detail |
|-----------|--------|
| Wire bytes as keys | Wire NLRI bytes already uniquely identify a prefix. Use them directly as map keys instead of converting to strings |
| Eliminate round-trips | Where a plugin stage converts wire bytes to strings only to have the next stage parse them back, process wire bytes directly |
| Use `netip.Prefix` as key | Where a typed prefix is needed for map keys, `netip.Prefix` is value-typed (no allocation), unlike `string` from `Prefix.String` |
| No new abstractions | Each child spec modifies existing data paths. No new registries, pools, or frameworks |
| Preserve external API | JSON output, plugin RPC events, and CLI output remain unchanged. Only internal map key representations change |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin bridge, event delivery
  -> Constraint: StructuredEvent is the delivery unit; changes must preserve its contract
- [ ] `ai/rules/memory-architecture.md` -- allocation discipline, caller-owned buffers
  -> Decision: wire bytes are caller-owned; map keys must not alias mutable buffers
- [ ] `ai/rules/no-sprintf-alloc.md` -- hot-path allocation ban
  -> Constraint: no `Prefix.String`, `fmt.Sprintf`, or string concat on forwarding hot path

**Key insights:**
- Wire NLRI bytes (from `nlrisplit.Split` and `NLRIIterator`) are already unique per prefix
- `netip.Prefix` is a 24-byte value type; using it as a map key avoids heap allocation
- The adj-rib-in `handleReceivedStructured` builds a `bgp.Event` with `wireNLRIsToAny` then `handleReceived` unboxes the same data; the intermediate `bgp.Event` is avoidable for the structured path
- Route-server `appendUnicastRecords` already stores `netip.Prefix` in `nlriRecord.prefix`; the allocation happens later in `applyNLRIRecords` when it calls `.String()` for map keys

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` -- handleReceivedStructured: per-UPDATE handler; calls checkBestPathChange per NLRI; wirePrefixToString for ribOut keys; changesByFamily map allocated per UPDATE
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` -- checkBestPathChange: gatherCandidates iterates all peers; parsePrevKey converts wire bytes to netip.Prefix (already allocation-free)
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` -- gatherCandidatesLocked: iterates r.bgpPeers map under RLock; extractCandidate allocates Candidate struct per peer
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` -- handleReceivedStructured: builds bgp.Event with wireNLRIsToAny (allocating []any with string-boxed prefixes); handleReceived: iterates FamilyOps calling RouteKey + pendingKey per NLRI
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go` -- applyNLRIRecords: iterates nlriRecords calling prefix.String() per unicast prefix for withdrawal map keys; appendUnicastRecords already stores netip.Prefix without allocation
- [ ] `internal/component/bgp/route.go` -- RouteKey: string concat family + ":" + prefix (+ optional pathID via textbuf)

**Behavior to preserve:**
- JSON output format from rib plugin (prefix strings in API responses)
- Plugin RPC StructuredEvent contract (PeerAddress, RawMessage, SourcePeerStr fields)
- adj-rib-in route replay via "update hex" commands (hex-encoded wire format)
- Route-server peer-down withdrawal behavior (correct prefix tracking)
- Best-path selection algorithm and change detection
- External plugin backward compatibility (route-meta JSON format)

**Behavior to change:**
- Internal map key representations from `string` to `[]byte` or `netip.Prefix`
- adj-rib-in structured path: process wire bytes directly instead of via bgp.Event round-trip
- rib ribOut maps: key by wire bytes instead of wirePrefixToString result
- Route-server withdrawal maps: key by netip.Prefix instead of prefix.String() concat
- rib handleReceivedStructured: eliminate per-UPDATE changesByFamily map allocation

## Data Flow (MANDATORY)

### Entry Point
- Wire UPDATE bytes arrive at reactor via BGP session
- Reactor creates RawMessage with WireUpdate, delivers via DirectBridge to plugins

### Transformation Path
1. Reactor receives wire UPDATE, creates RawMessage with WireUpdate reference
2. DirectBridge delivers StructuredEvent to each plugin (rib, adj-rib-in, route-server)
3. Each plugin independently parses wire NLRIs from WireUpdate
4. Current: each plugin converts wire bytes to prefix strings for internal map keys
5. Target: each plugin uses wire bytes or netip.Prefix directly as map keys
6. Output path (JSON, CLI, web) converts to strings only at the display boundary

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to Plugin | StructuredEvent via DirectBridge (unchanged) | [ ] |
| Plugin internal maps | Wire bytes or netip.Prefix as keys (changed) | [ ] |
| Plugin to JSON output | Prefix.String at serialization boundary (unchanged) | [ ] |
| Plugin to CLI/web | String conversion at display time (unchanged) | [ ] |

### Integration Points
- `rib.handleReceivedStructured` -- wire NLRI iteration, best-path check, ribOut storage
- `adj_rib_in.handleReceivedStructured` -- structured event dispatch to handleReceived
- `rs.processForward` -- NLRI record extraction, withdrawal tracking
- `bgp.RouteKey` -- route key construction (adj-rib-in consumer)
- `nlrisplit.Split` / `NLRIIterator` -- wire NLRI parsing (shared across plugins)

### Architectural Verification
- [ ] No bypassed layers (plugins still receive StructuredEvent via DirectBridge)
- [ ] No unintended coupling (each child spec modifies one plugin's internal maps)
- [ ] No duplicated functionality (extends existing wire-byte parsing, does not recreate)
- [ ] Zero-copy preserved where applicable (wire bytes used directly as keys)

## Wiring Test (MANDATORY -- NOT deferrable)

Umbrella-level wiring tests. Each child spec has its own detailed wiring tests.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| 100k UPDATE forwarding benchmark | -> | All 5 optimizations active | `test/perf/run.py` with `PPROF=1` (allocation count < 5M) |
| Peer-down withdrawal replay | -> | Route-server withdrawal with prefix-keyed maps | `test/plugin/rs-peer-down.ci` |
| RIB best-path change after UPDATE | -> | rib handleReceivedStructured with no changesByFamily map | `test/plugin/rib-best-path.ci` |
| adj-rib-in route storage from structured event | -> | Direct wire-byte processing without bgp.Event | `test/plugin/adj-rib-in-structured.ci` |

## Acceptance Criteria

Umbrella-level outcomes. Each child spec has detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100k route benchmark with PPROF=1 | Total allocation < 300 MB (from 1.09 GB) |
| AC-2 | 100k route benchmark with PPROF=1 | Object count < 5M (from 17.9M) |
| AC-3 | 100k route benchmark with GCTRACE=1 | Max GC pause < 15ms (from 53ms) |
| AC-4 | 100k route benchmark, 5 iterations | Throughput stddev < 100k (from 511k) |
| AC-5 | All existing functional tests | All pass unchanged (no behavioral regression) |
| AC-6 | JSON API output from rib and adj-rib-in | Prefix strings identical to current output |

## 🧪 TDD Test Plan

### Unit Tests

Umbrella-level. Each child spec defines its own unit tests.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRibOutWireKey` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | ribOut stores and retrieves entries by wire-byte key | |
| `TestAdjRibInStructuredDirect` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | Structured path processes wire NLRIs without bgp.Event | |
| `TestWithdrawalPrefixKey` | `internal/component/bgp/plugins/rs/server_inventory_test.go` | Withdrawal map uses netip.Prefix key correctly | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rs-peer-down` | `test/plugin/rs-peer-down.ci` | Existing: route-server withdraws routes on peer down | |
| `rib-best-path` | `test/plugin/rib-best-path.ci` | Existing: best-path selection produces correct forwarding | |
| `adj-rib-in-structured` | `test/plugin/adj-rib-in-structured.ci` | Existing: adj-rib-in stores and replays routes correctly | |

## Files to Modify

Umbrella-level summary. Each child spec lists its specific files.

- `internal/component/bgp/plugins/rib/rib_structured.go` -- wire-byte keys for ribOut, eliminate changesByFamily map
- `internal/component/bgp/plugins/adj_rib_in/rib.go` -- direct wire-byte processing, eliminate bgp.Event round-trip
- `internal/component/bgp/plugins/rs/server_inventory.go` -- netip.Prefix keys for withdrawal maps
- `internal/component/bgp/route.go` -- wire-byte based RouteKey variant

## Files to Create
- No new files expected. All changes modify existing files.

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are independent and can be
implemented in any order. Ordered by allocation impact for maximum early benefit.

1. **Phase: Prefix String Elimination (perf-hot-1)** -- wire-byte keys in rib ribOut maps
   - Tests: `TestRibOutWireKey`, existing rib functional tests
   - Files: `rib_structured.go` (storeSentEntries, removeSentNLRIs, wirePrefixToString removal)
   - Verify: ribOut correctly stores and retrieves entries; 5.3M fewer objects in profile

2. **Phase: adj-rib-in Event Elimination (perf-hot-2)** -- direct structured processing
   - Tests: `TestAdjRibInStructuredDirect`, existing adj-rib-in functional tests
   - Files: `adj_rib_in/rib.go` (handleReceivedStructured bypasses bgp.Event construction)
   - Verify: route storage and replay unchanged; 2.5M fewer objects in profile

3. **Phase: adj-rib-in Key Optimization (perf-hot-3)** -- wire-byte or numeric route keys
   - Tests: adj-rib-in unit tests, existing functional tests
   - Files: `adj_rib_in/rib.go` (handleReceived), `route.go` (RouteKey variant)
   - Verify: route keying and validation lookup unchanged; 2.2M fewer objects in profile

4. **Phase: Route-Server Withdrawal Keys (perf-hot-4)** -- netip.Prefix keyed maps
   - Tests: `TestWithdrawalPrefixKey`, existing rs functional tests
   - Files: `rs/server_inventory.go` (applyNLRIRecords, withdrawal map type change)
   - Verify: peer-down withdrawal behavior unchanged; 2.5M fewer objects in profile

5. **Phase: changesByFamily Elimination (perf-hot-5)** -- pre-allocated or direct publish
   - Tests: existing rib best-path functional tests
   - Files: `rib_structured.go` (handleReceivedStructured changesByFamily removal)
   - Verify: best-path change batching unchanged; minor object reduction

6. **Full verification** -- `make ze-verify` + benchmark with PPROF=1

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 5 child specs implemented, all ACs demonstrated |
| Correctness | JSON API output unchanged; withdrawal behavior unchanged; best-path selection unchanged |
| Data flow | Wire bytes used as keys do not alias mutable buffers (must copy or use value types) |
| Hot-path rule | No Prefix.String, fmt.Sprintf, or string concat on forwarding path after optimization |
| Rule: no-partial-completion | Each child spec verified independently before claiming umbrella done |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Allocation reduction measured | `PPROF=1` benchmark, compare alloc profile before/after |
| GC pause reduction measured | `GCTRACE=1` benchmark, compare max STW before/after |
| Throughput variance reduction | Benchmark 5-iteration stddev comparison |
| No behavioral regression | `make ze-test` passes, existing functional tests unchanged |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Wire-byte map keys from untrusted BGP peers must be bounded by NLRI length validation |
| Memory safety | Wire-byte keys must not alias pool buffers that may be recycled |

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

String-keyed maps on the forwarding hot path are the dominant allocation source.
Wire bytes and value-typed `netip.Prefix` are free alternatives that carry the same
information. The optimization is purely internal; external APIs remain string-based.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Wire bytes as map keys over interned strings | String interning pool, hash-based dedup | Wire bytes are already unique, available, and require zero conversion. Interning adds complexity and still allocates on first sighting |
| netip.Prefix as map key where prefix semantics needed | Custom prefix struct, uint128 encoding | netip.Prefix is stdlib, value-typed, and already used throughout the codebase. No new type needed |
| Per-child-spec implementation over single large change | Monolithic optimization pass | Each child spec is independently testable and reviewable. Easier to bisect if a regression appears |

## Known Limitations
- bart tree node allocations (2.4M objects, 130 MB) are structural to the trie and not reducible without replacing the RIB data structure
- Non-unicast families (VPN, EVPN) still use string-based NLRI representations in adj-rib-in and route-server because their wire format is complex; these are rare in the benchmark workload

## RFC Documentation

Not applicable. These are internal optimization changes with no protocol behavior impact.

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Allocation < 300 MB | Benchmark | PPROF=1 alloc profile comparison |
| Object count < 5M | Benchmark | PPROF=1 alloc_objects comparison |
| GC pause < 15ms | Benchmark | GCTRACE=1 max STW comparison |
| Throughput stddev < 100k | Benchmark | 5-iteration throughput stddev |
| No behavioral regression | Functional tests | `make ze-test` output |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled during review]

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
- [ ] AC-1..AC-6 all demonstrated across child specs
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (each child spec is a concrete, targeted change)
- [ ] No speculative features (every child addresses a profiled allocation site)
- [ ] Single responsibility per child spec
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (children are independent)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf-hot.md`
