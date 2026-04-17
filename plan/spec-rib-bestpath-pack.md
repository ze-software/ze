# Spec: rib-bestpath-pack

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/learned/607-rib-bart-bestprev.md` -- prior phase; establishes bestPrev as BART-backed `Store[bestPathRecord]`, the shape this spec replaces
4. `plan/learned/534-rib-alloc.md` -- earlier phase that introduced per-attribute dedup and the BART adoption pattern
5. `internal/component/bgp/plugins/rib/rib_bestchange.go` -- the sole file materially rewritten

## Task

The 1M-prefix stress profile captured in `plan/learned/607-rib-bart-bestprev.md`
shows BART fringe nodes for `bestPathRecord` holding 56.5 MB (33% of inuse
heap) and GC scanning them at ~27% of CPU. The record is 72 bytes and carries
five GC-scannable pointers (`PeerAddr string`, `NextHop netip.Addr` string
zone, `ProtocolType string`, plus the struct header pointer tracking inside
BART).

Replace the struct with a named `uint64` packing four 16-bit fields, backed by
a shared (cross-family) interner that maps the three string/value fields
(`PeerAddr`, `NextHop`, `Metric`) to `uint16` indices. A single `resolve()`
function reconstitutes the full `bestChangeEntry` on the cold emission path
using reverse tables in the interner. The stored value contains zero
GC-traceable pointers; the hot comparison is a single `uint64` equality
instruction.

Layout (`bestPathRecord uint64`):

| Bits | Field | Meaning |
|------|-------|---------|
| [63:48] | MetricIdx | `uint16` index into `interner.metrics` |
| [47:32] | PeerIdx | `uint16` index into `interner.peers` |
| [31:16] | NextHopIdx | `uint16` index into `interner.nextHops` |
| [15:0] | Flags | bit 0 = `isEBGP`; bits 1-15 reserved for future use |

`uint16` cardinality (65,535 per table) is architecturally unreachable: the
largest Internet IXP carries ~2,000 peers, and realistic deployments see
tens to low hundreds of unique next-hops and MED values. No ze instance
will approach the cap; even so, the interner's `intern*` methods return
`(uint16, bool)` so the caller can gracefully skip tracking rather than
panic if the invariant is ever violated. `checkBestPathChange` drops the
single affected record with a logged error, the daemon keeps running, and
best-path emissions for all other records continue unaffected.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- RIB storage layering; already describes `Store[T]`
  -> Constraint: `bestPathRecord` is stored by value in `*Store[bestPathRecord]`; any type that satisfies Go generic constraints (including the new 16-byte compact struct) is a drop-in.
- [ ] `plan/learned/607-rib-bart-bestprev.md` -- prior phase
  -> Decision: single-mode `Store[T]` with hybrid (`trie` + `ap` Store pair) at the rib layer. This spec does not change the hybrid shape; it only changes the `T` from the current 72-byte pointer-carrying struct to a 16-byte pointer-free compact struct.
  -> Constraint: `r.mu` guards both `bestPrev` writes and reads; the interner inherits the same lock.
- [ ] `plan/learned/534-rib-alloc.md` -- establishes BART + per-attribute pool dedup patterns.

### Rules Applied
- [ ] `.claude/rules/design-principles.md`
  -> Constraint: "Do it right. Never trade correctness for speed of implementation." The packed record trades code density for measurable GC/heap wins; accessors enforce read-only semantics.
  -> Constraint: "Lazy over eager" -- the `resolve()` function defers full struct materialization until emission.
- [ ] `.claude/rules/no-layering.md`
  -> Constraint: the struct-based `bestPathRecord` is deleted; no parallel implementation or fallback flag.
- [ ] `.claude/rules/api-contracts.md`
  -> Constraint: the exported `ribevents.BestChangeEntry` payload shape is unchanged (callers outside `rib` see identical JSON and identical Go struct).
- [ ] `.claude/rules/goroutine-lifecycle.md`
  -> Constraint: no new goroutines; interner operations run under existing `r.mu`.
- [ ] `.claude/rules/spec-no-code.md`
  -> Constraint: this spec uses tables and prose only.

### RFC Summaries
Not applicable -- internal storage shape change.

**Key insights:**
- BART's `Table[T]` specializes cleanly over scalar `T` (a named `uint64`); per-entry overhead is dominated by BART's node metadata, not by `T`'s internal layout. The storage win comes primarily from dropping pointer fields; shrinking to exactly 8 bytes is an additional benefit.
- The `bestPathRecord` had a derivable field (`Priority`) alongside its source (`ProtocolType`). Removing the redundancy lets `Flags` carry both in a single bit.
- Interner cardinality for `PeerAddr`, `NextHop`, and `Metric` in realistic BGP tables is in the tens to low hundreds. The largest Internet IXP carries ~2,000 peers; `uint16` (65,535) gives 30x headroom over the upper bound observed anywhere in production. No ze instance will approach this cap.
- Overflow handling is defensive (non-panic, log + skip the record) rather than load-bearing: the condition is architecturally unreachable, but the path exists so a mis-deployment degrades gracefully rather than crashing.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` -- defines `bestPathRecord` struct (72 bytes, five pointer fields), `checkBestPathChange` comparison by field, `replayBestPaths` reading the struct fields directly, `bestCandidateNextHopAddr`, `extractMPNextHopAddr`, `nextHopString`, `parseNextHopAddr`.
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- `RIBManager` declares `bestPrev map[family.Family]*bestPrevStore`; no interner field yet.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- `Candidate` struct carries `PeerAddr string`, `PeerASN`, `LocalASN`, MED accessor; `SelectBest` picks a winner.
- [ ] `internal/component/bgp/plugins/rib/storage/store_bart.go`, `store_map.go` -- generic `Store[T]`; T is instantiated twice in this file (RouteEntry and bestPathRecord).
- [ ] `internal/component/bgp/plugins/rib/events/` -- `BestChangeEntry` payload type (stable external shape).
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange_test.go`, `rib_test.go` -- constructor helpers.

**Behavior to preserve:**
- `BestChangeBatch` / `BestChangeEntry` JSON payload shape unchanged (kebab-case keys, `action` / `prefix` / `next-hop` / `priority` (20 or 200 int) / `metric` (uint32) / `protocol-type` (`ebgp` / `ibgp`)).
- Replay ordering: one batch per family.
- Same-best short-circuit: no emission when peer/next-hop/priority/metric match previous.
- Mixed-mode (ADD-PATH + non-ADD-PATH peers in same family) still routes via the two-backend `bestPrevStore`.
- `FamilyRIB` public API unchanged.
- `maprib` build tag semantics preserved.
- All existing `rib_bestchange_test.go` assertions pass unchanged.

**Behavior to change:**
- `bestPathRecord` becomes a named `uint64` with bit-field accessors (three `uint16` interner indices plus a 16-bit Flags field); the string / `netip.Addr` fields are gone.
- `RIBManager` gains a `bestPathInterner` field shared across all families.
- `checkBestPathChange` interns inputs, packs into `uint64`, compares by single `uint64` equality, stores by value.
- `replayBestPaths` and the emission path in `checkBestPathChange` use a new `resolve()` helper to rebuild `bestChangeEntry` from the packed record + interner reverse tables.
- `bestPathRecord.Priority` + `.ProtocolType` fold into the Flags bit 0 (`isEBGP`); the emitted payload still carries `priority: 20|200` and `protocol-type: "ebgp"|"ibgp"` via the resolve helper.
- On the unreachable overflow case (>65,535 unique peers / next-hops / metrics), the interner's `intern*` methods return `(0, false)`; `checkBestPathChange` logs one `slog.Error` and returns `(zero, false)` for that single record. No panic, no crash, no silent corruption -- the daemon keeps running and all other records continue to be tracked correctly.

## Data Flow (MANDATORY)

### Entry Point
`handleReceivedStructured` in `rib_structured.go` receives decoded BGP UPDATEs and calls `checkBestPathChange(fam, nlriBytes, addPath)` per affected prefix, under `r.mu` write lock.

### Transformation Path
1. `gatherCandidates(fam, nlriBytes)` walks per-peer RIBs to build a `Candidate` list.
2. `SelectBest(candidates)` picks the winner.
3. `bestCandidateNextHopAddr` extracts `netip.Addr` for the winner's next-hop.
4. **New:** intern `PeerAddr`, `NextHop`, `Metric` via the shared `bestPathInterner`, obtaining three `uint16` indices. If any `intern*` returns `(_, false)` (overflow), log one `slog.Error`, return `(zero, false)` and skip the record.
5. **New:** pack the three indices + flags (`isEBGP`) into a single `uint64` `bestPathRecord`.
6. Look up previous `bestPathRecord` in the appropriate backend (trie or ap map).
7. **New:** compare by `uint64` equality -- single instruction, no field walk, no allocation.
8. On change, call `resolve(interner, action, prefix)` to materialize the full `bestChangeEntry` using reverse tables.
9. Insert the packed `uint64` into the backend; emit the batch after lock release.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Structured event | existing path, unchanged | existing tests |
| RIBManager <-> storage.Store[T] | `Store[bestPathRecord]` -- T changes from 72-byte pointer-carrying struct to a named `uint64` (8 bytes, zero pointers) | `store_test.go` (generic surface) |
| Change detection -> EventBus | `resolve()` produces `bestChangeEntry`; EventBus payload shape unchanged | `test/plugin/fib-rib-event.ci` |

### Integration Points
- `r.mu` guards `bestPrev` AND `bestPathInterner`; no lock change.
- `EventBus` handle `ribevents.BestChange` unchanged.
- `Store[T]` generic unchanged (drop-in instantiation with named `uint64`).

### Architectural Verification
- [ ] No bypassed layers -- inserts still go Wire -> Structured -> RIBManager -> store.
- [ ] No unintended coupling -- interner is a private type on RIBManager.
- [ ] No duplicated functionality -- replaces the struct, does not coexist.
- [ ] Zero-copy preserved -- `Store[T]` stores the 8-byte `uint64` by value; no pointers.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE arriving at RIB → best-path change emission | → | `RIBManager.checkBestPathChange` with packed `uint64` interned record | `test/plugin/fib-rib-event.ci` (existing; refactor preserves) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 1M non-ADD-PATH prefix stress run | `bart NewFringeNode[bestPathRecord]` heap flat drops from 56.5 MB to under 12 MB; total inuse heap drops by at least 40 MB vs Phase-4b |
| AC-2 | All 11 existing `rib_bestchange_test.go` tests | Pass unchanged; assertions on `BestChangeEntry` payload shape (priority: 20, protocol-type: "ebgp", etc.) continue to hold |
| AC-3 | Mixed-mode family (ADD-PATH + non-AP peers) | Packed records still route to correct backend (trie vs ap); no cross-backend collision |
| AC-4 | `bestPathRecord` is a named `uint64` with bit-field accessors | `grep 'type bestPathRecord uint64' internal/.../rib_bestchange.go` finds the declaration; no struct-based definition remains |
| AC-5 | Replay path (`replayBestPaths`) | Emits identical `BestChangeBatch` payloads via `resolve()`; one batch per family |
| AC-6 | Interner dedups values | First insert of a unique value appends to reverse table; repeated inserts return the cached uint16 index. Verified by unit test |
| AC-7 | Interner overflow path is non-panic | `intern*` returns `(0, false)` when the table is saturated; `checkBestPathChange` logs `slog.Error` once and returns `(zero, false)` for that single record. Verified by unit test that drives one interner to cap and confirms no panic fires |
| AC-8 | `make ze-verify-fast` | Passes (disregarding the pre-existing addpath MP_REACH mismatch documented in `plan/known-failures.md`) |
| AC-9 | `go test -race -tags maprib ./internal/component/bgp/plugins/rib/...` | Passes |
| AC-10 | GC share of CPU | `gcBgMarkWorker` cumulative drops by at least 5 pp vs Phase-4b (target: 27% -> under 22%) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBestPathRecordPackUnpack` | `rib_bestchange_test.go` | round-trip: `packBestPath(metric, peer, nh, flags)` then accessors return same values; boundary (all zero, all max uint16) | |
| `TestBestPathRecordEquality` | `rib_bestchange_test.go` | two records with identical fields compare equal via `uint64 ==`; any differing field produces inequality | |
| `TestBestPrevInternerDedup` | `rib_bestchange_test.go` | inserting the same peer / next-hop / metric twice returns the same index; reverse table length grows only on new values | |
| `TestBestPrevInternerReverse` | `rib_bestchange_test.go` | `peers[idx]` / `nextHops[idx]` / `metrics[idx]` return the original value for every interned index | |
| `TestBestPrevInternerOverflow` | `rib_bestchange_test.go` | driving an interner table past 65535 entries causes `intern*` to return `(0, false)` WITHOUT panicking; subsequent `checkBestPathChange` calls log once and return `(zero, false)` for the affected record | |
| `TestBestPathResolve` | `rib_bestchange_test.go` | `resolve(interner, action, prefix)` returns a `bestChangeEntry` whose `Priority` (20 or 200), `ProtocolType` ("ebgp"/"ibgp"), `NextHop`, `Metric` reflect the packed record and interner reverse tables | |
| (existing) `TestRIBBestChange*` x11 | `rib_bestchange_test.go` | Payload shape preserved: action, prefix, next-hop, priority (20 or 200), metric, protocol-type ("ebgp" or "ibgp") | |
| (existing) `TestRIBReplayOnSubscribe` | `rib_bestchange_test.go` | Replay still emits a batch via `resolve()`; batch.Replay=true | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interner idx (per table) | 0..65535 | 65535 | N/A (unsigned) | 65536 -> `intern*` returns `(0, false)`; caller logs + skips record; no panic |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-rib-event` | `test/plugin/fib-rib-event.ci` | Downstream subscriber receives best-change events (existing; refactor must preserve) | |

## Files to Modify

- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- rewrite `bestPathRecord` type; add `bestPrevInterner` type + methods (`internPeer`, `internNextHop`, `internMetric`, reverse lookups, `resolve`); rewrite `checkBestPathChange` and `replayBestPaths` to use packed records.
- `internal/component/bgp/plugins/rib/rib.go` -- add `bestPathInterner *bestPrevInterner` field on `RIBManager`; initialize in `NewRIBManager`; update comment on `r.mu` guarded fields.
- `internal/component/bgp/plugins/rib/rib_test.go` -- update test constructor to initialize the new field.
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go` -- update test constructor; add new unit tests listed above.
- `docs/architecture/plugin/rib-storage-design.md` -- update the "Generic `Store[T]` and bestPrev Consolidation" section with a one-paragraph note on packed records + interner.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | -- |
| CLI commands/flags | [ ] No | -- |
| Editor autocomplete | [ ] No | -- |
| Functional test for new RPC/API | [ ] No | existing `fib-rib-event.ci` suffices |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | -- |
| 2 | Config syntax changed? | [ ] No | -- |
| 3 | CLI command added/changed? | [ ] No | -- |
| 4 | API/RPC added/changed? | [ ] No | -- |
| 5 | Plugin added/changed? | [ ] No | -- |
| 6 | Has a user guide page? | [ ] No | -- |
| 7 | Wire format changed? | [ ] No | -- |
| 8 | Plugin SDK/protocol changed? | [ ] No | -- |
| 9 | RFC behavior implemented? | [ ] No | -- |
| 10 | Test infrastructure changed? | [ ] No | -- |
| 11 | Affects daemon comparison? | [ ] No | -- |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/plugin/rib-storage-design.md` -- paragraph on packed-record bestPrev + interner |

## Files to Create

None. All changes live in files already covered by `plan/learned/607-rib-bart-bestprev.md`.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. Full verification | `make ze-verify-fast` + `-tags maprib` variant |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every finding |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

1. **Phase 1 -- packed `bestPathRecord` + interner type + unit tests**
   - Tests: `TestBestPathRecordPackUnpack`, `TestBestPathRecordCompare`, `TestBestPrevInternerDedup`, `TestBestPrevInternerOverflow`, `TestBestPathResolve`.
   - Files: `rib_bestchange.go` (type + methods); `rib_bestchange_test.go` (tests).
   - Verify: tests fail (types not implemented) -> implement -> tests pass.
2. **Phase 2 -- wire through `checkBestPathChange` and `replayBestPaths`**
   - Tests: existing `TestRIBBestChange*` (11 cases) and `TestRIBReplayOnSubscribe` pass unchanged.
   - Files: `rib_bestchange.go` (rewrite hot path + replay); `rib.go` (add `bestPathInterner` field + init); `rib_test.go`, `rib_bestchange_test.go` (constructor updates).
   - Verify: `go test -race ./internal/component/bgp/plugins/rib/...` passes.
3. **Phase 3 -- full verification + 1M stress re-profile**
   - `make ze-verify-fast` (both default and `-tags maprib`).
   - 1M-prefix stress run; capture `tmp/profiles/phase5-*`; produce comparison table vs Phase-4b baseline (56.5 MB bestPrev heap, 27% GC).
4. **Phase 4 -- architecture doc paragraph**
   - Update `docs/architecture/plugin/rib-storage-design.md` with the packed-record note.
5. **Completion** -- fill audit tables, Pre-Commit Verification, write learned summary to `plan/learned/NNN-rib-bestpath-pack.md`, two-commit sequence.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test or profile artefact |
| Correctness | Payload shape preserved: priority emits 20/200 as int; protocol-type emits "ebgp"/"ibgp" as string; both derived from Flags bit 0 |
| Correctness | Same-best short-circuit: `uint64 ==` matches field-by-field equivalence of the pre-refactor record |
| Correctness | Mixed-mode family (AP + non-AP peers) stores and retrieves packed records correctly from the appropriate backend |
| Correctness | Overflow path returns `(0, false)` without panicking; caller logs once and continues. Verified by deliberately driving one interner past 65535. |
| Naming | Accessor methods idiomatic (`MetricIdx()`, `PeerIdx()`, `NextHopIdx()`, `Flags()`, `IsEBGP()`) |
| Data flow | Interner is private to `rib_bestchange.go` (not exported); resolve is the only external path out of the packed form |
| Rule: no-layering | Old struct definition fully deleted; no dual-path code |
| Rule: api-contracts | `ribevents.BestChangeEntry` public shape unchanged |
| Rule: zero-copy / copy-on-modify | `Store[bestPathRecord]` stores the 8-byte `uint64` by value; `resolve()` does one allocation per event emit |
| Rule: spec-no-code | This spec contains no code snippets |
| Rule: maprib build | `go test -tags maprib` still passes |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `bestPathRecord` is a named `uint64` | `grep 'type bestPathRecord uint64' internal/component/bgp/plugins/rib/rib_bestchange.go` |
| Old pointer-carrying struct gone | `grep 'type bestPathRecord struct' internal/component/bgp/plugins/rib/rib_bestchange.go` returns no match |
| Interner defined + returns (uint16, bool) from intern* | `grep 'type bestPrevInterner struct\|func.*internPeer\|func.*internNextHop\|func.*internMetric' internal/component/bgp/plugins/rib/rib_bestchange.go` |
| `resolve()` method exists | `grep 'func (r bestPathRecord) resolve' internal/component/bgp/plugins/rib/rib_bestchange.go` |
| No `panic` in overflow path | `grep -n 'panic' internal/component/bgp/plugins/rib/rib_bestchange.go` returns no match inside the interner or overflow handling code |
| Unit tests for pack/unpack, equality, interner, resolve, overflow | `grep -c '^func Test' internal/component/bgp/plugins/rib/rib_bestchange_test.go` increases by at least 5 |
| `make ze-verify-fast` passes | output in `tmp/ze-verify.log` |
| `-tags maprib` passes | pasted `go test -tags maprib` output |
| 1M stress profile delta captured | Comparison table in learned summary (bestPrev fringe-node heap target: below 20 MB; GC share target: below 22%) |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Interner inputs are already-validated peer addresses, parsed next-hops, and uint32 MED values from `Candidate` fields; no raw wire bytes reach the interner |
| Resource exhaustion | Interner indices are `uint16` (65,535 entries per table). Realistic cardinality is <10^4 (largest IXP: ~2k peers). Cap is architecturally unreachable; if it is ever hit, `intern*` returns `(0, false)`, caller logs once and drops the record -- no panic, no crash, no silent corruption |
| Error leakage | No user-visible error paths; `slog.Error` on overflow carries no user input |
| Lock ordering | Interner accessed only under `r.mu` (same as bestPrev); no new lock |
| No panics | All interner operations return normally; `grep panic` in the modified file returns no match |
| Index bounds | Reverse-table lookups (`peers[idx]`, `nextHops[idx]`, `metrics[idx]`) use `idx` from the interner's own `intern*` return value, bounded by insertion -- safe by construction |
| Bit-field extraction | Shift/mask operations in pack/unpack accessors are pure bitwise; defined behavior on any `uint64` value |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Pack/unpack test fails | Re-check bit layout and shift constants |
| Best-change test fails (priority / protocol-type mismatch) | Check `resolve()` derivation from Flags bit 0 |
| Mixed-mode test fails | Verify `store.pick(addPath)` still routes correctly; interner shared across backends |
| Profile shows no improvement | Re-check that the stored `T` is `uint64` (named, not a struct); verify BART fringe-node size is ~8 bytes per entry |
| Overflow test triggers a panic | Fix the intern path to return `(0, false)` and let the caller log + skip; no panic is permitted anywhere in the new code |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| | | | |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| | | | |

## Design Insights

- A named `uint64` with bit-field accessors is the clearest idiomatic Go way to express a packed record; BART's `Table[T]` specializes naturally over scalars. The single-instruction `cmpq` compare is a bonus over struct equality.
- Interning peer / next-hop / metric exploits the low cardinality observed in real BGP tables (dozens to hundreds per field type). The memory overhead of the reverse tables is sub-kilobyte at realistic scale.
- The biggest win is GC, not record size: eliminating all pointer fields from the stored value means BART fringe nodes become opaque bytes to GC, reducing the 5M pointer-trace workload at 1M entries to zero.
- `Priority` was redundant with `ProtocolType`; dropping the former and folding the latter into Flags bit 0 is a net simplification.
- `uint16` indices are safe because BGP deployment realities cap peer / next-hop / metric cardinality well below 65,535 (the largest Internet IXP carries ~2,000 peers -- 30x below the uint16 cap). The overflow handler exists only as defense-in-depth: it logs and skips the single affected record rather than panicking, so a mis-deployment degrades gracefully.

## RFC Documentation

Not applicable -- internal storage shape change.

## Implementation Summary

### What Was Implemented
- (Filled at completion)

### Bugs Found/Fixed
- (Filled at completion)

### Documentation Updates
- (Filled at completion)

### Deviations from Plan
- (Filled at completion)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace bestPathRecord with named uint64 (four 16-bit fields) | | | |
| Introduce bestPrevInterner (peers, nextHops, metrics) returning (uint16, bool) | | | |
| Fold Priority + ProtocolType into Flags bit 0 | | | |
| Preserve BestChangeEntry payload shape | | | |
| No panic anywhere in the new code | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| | | |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| | | |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `make ze-verify-fast` passes (pre-commit gate)
- [ ] `make ze-test` passes (full suite, run once before closing spec)
- [ ] `go test -tags maprib ./internal/component/bgp/plugins/rib/...` passes
- [ ] 1M-prefix stress profile re-run; comparison table in learned summary
- [ ] Architecture doc updated

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (interner private to rib_bestchange.go)

### TDD
- [ ] Tests written before implementation
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the interner cap (overflow returns `(0, false)` without panicking)
- [ ] Functional `.ci` test continues to pass

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Pre-Commit Verification filled with fresh evidence
- [ ] Write learned summary to `plan/learned/NNN-rib-bestpath-pack.md`
- [ ] Summary included in the two-commit sequence
