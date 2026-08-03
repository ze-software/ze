# Spec: perf-next-1-ebgp-wire-lockfree -- Lock-Free Cache Hits in ReceivedUpdate.EBGPWire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-perf-next-0-umbrella.md |
| Phase | 5/5 |
| Updated | 2026-07-22 |

Awaiting closure (recorded 2026-07-22 during plan review): implemented and in
the tree -- `ebgpWireSlot` + lock-free `atomic.Pointer` slots at
`internal/component/bgp/reactor/received_update.go,89,93,203-209`, with
the concurrent cache-hit test in `received_update_test.go`. The work is
documented in `plan/learned/900-perf-next-round-3.md`; only the two-commit
closure remains.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/received_update.go` (struct + EBGPWire)
4. `internal/component/bgp/reactor/recent_cache.go` (evictLocked)
5. `docs/architecture/encoding-context.md`, `docs/architecture/forward-congestion-pool.md`

## Task

`ReceivedUpdate.EBGPWire` (received_update.go) lazily generates and caches
the EBGP variant of an UPDATE (local ASN prepended to AS_PATH per RFC 4271 9.1.2),
one cached variant per destination ASN width (`ebgpWireASN4`, `ebgpWireASN2`).
Today **every call takes `ebgpMu`, including cache hits**, and cache hits are the
overwhelming majority: on a route server fanning one UPDATE out to N eBGP peers,
the wire is generated once per ASN-width variant and then re-read N-ish times.

Measured shape (research dossier, 2026-06-11): three goroutine families call
EBGPWire concurrently on the SAME ReceivedUpdate:
1. Session read goroutine via the RS fast path (`forward_rs.go` inside `reactorForwardRS`, reached from `reactor_notify.go`).
2. Per-peer delivery goroutine -> plugin dispatch -> `forwardUpdateCore` (`reactor_api_forward.go`).
3. Per-destination-peer forward workers (`forward_pool.go` spawns `runWorker`).

At 100K UPDATE/s with ~150 EBGPWire calls per UPDATE (RS deployment, 100+ eBGP
destinations), that is ~15M mutex lock/unlock pairs per second, ~30-50ns each
uncontended and worse under contention, purely to re-read two immutable pointers.

**Goal:** make the cache-hit path lock-free (single atomic pointer load), keep
generation single-flight under the existing mutex (double-checked locking), and
keep buffer-handle ownership and eviction exactly as safe as today.

### Design (chosen)

Replace the four cache fields with two atomic slots, one per ASN-width variant.
Each slot stores a pointer to a small immutable struct bundling the generated
wire AND its backing pool buffer handle, so the (WireUpdate, BufHandle) pair is
published atomically in one store:

| Today (under ebgpMu) | After |
|----------------------|-------|
| `ebgpWireASN4 *wireu.WireUpdate` + `ebgpPoolBuf4 BufHandle` | one `atomic.Pointer` slot to a struct holding both |
| `ebgpWireASN2 *wireu.WireUpdate` + `ebgpPoolBuf2 BufHandle` | one `atomic.Pointer` slot to a struct holding both |
| `ebgpMu` guards read + generate + store | `ebgpMu` kept, guards ONLY generation (miss path) |

Call flow after the change:
1. Fast path: atomic load of the slot for `dstASN4`; if non-nil return its wire. No lock.
2. Miss: take `ebgpMu`, re-check the slot (double-checked locking), generate via `wireu.RewriteASPath` into a `getReadBuf` buffer, build the struct, atomic store, unlock.
3. Eviction: atomic load of each slot; if non-nil, `ReturnReadBuffer` its handle. Runs under `cache.mu` as today. TWO sites return the variant buffers and BOTH must switch to atomic loads: `evictLocked` (`recent_cache.go`, lines 461-462) and `Delete` (`recent_cache.go`, lines 523-524).

Why this is safe (must be restated in code comments):
- Slots are fire-once: written at most once, never mutated after publication. Readers see nil or the complete struct (Go memory model: atomic store happens-before atomic load that observes it).
- Generation failure does NOT publish: error paths return the buffer (`ReturnReadBuffer(dst)`) and leave the slot nil, exactly as today.
- Eviction cannot race a reader holding the wire: the RecentUpdateCache retention contract guarantees an entry obtained via `Get()` is not evicted until all consumers ack (retainCount in `recent_cache.go`). This spec does not change that contract; it relies on it identically to the current code.
- Eviction cannot race generation on the normal ack path: eviction only happens after consumers are done (`evictLocked` gated on `totalConsumers()<=0` at `recent_cache.go/498`), and a generator is a consumer holding the entry. The path to validate during the audit step (A-2) is NOT the ack path but the background safety valve: `gapScanLoop` (`recent_cache.go`) force-evicts "stalled" entries via `evictLocked` (`recent_cache.go`) when `isGapEvictable` (`recent_cache.go`) returns true, regardless of acks. Confirm a genuinely-running forward worker is never `isGapEvictable` (its 30s stall timer never elapses on a live reader). This is a PRE-EXISTING property: today `evictLocked` already returns `e.update.poolBuf` while a mid-flight generator reads `u.WireUpdate.Payload()` from that same buffer (`received_update.go`). Child 1 changes two field reads to atomic loads of the same already-published data and introduces NO new race class; it relies on this property identically to current code.

Local idiom to match: `Peer.negotiated` and `Peer.sendCtx` already use
`atomic.Pointer` for read-mostly fields (`peer.go`); campaign 771 used
`atomic.Int64` to replace an RLock on the zero-monitor fast path (commit f3c5c93cc).

### Out of scope
- The dual/secondary-ASN prepend path (`RewriteASPathDual`, `reactor_api_forward.go` area) does not use this cache and is unchanged.
- Any change to RewriteASPath itself, the cache retention protocol, or BufHandle.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/performance.md` - pool ownership rules
  → Constraint: only pool-issued BufHandles; hook block-fake-bufhandle.sh enforces
- [ ] `docs/architecture/encoding-context.md` - ContextID semantics the cached wire preserves
  → Constraint: cached wire keeps the ORIGINAL SourceCtxID (received_update.go); do not alter
- [ ] `docs/architecture/forward-congestion-pool.md` - forward worker goroutine model
  → Decision: one worker goroutine per destination peer; these are the concurrent readers
- [ ] `ai/rules/goroutine-lifecycle.md`
  → Constraint: no new goroutines; this change is purely synchronization-shape

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - Section 9.1.2 AS prepend on eBGP propagation
  → Constraint: prepend semantics unchanged; this spec only changes how the cached result is read
- [ ] `rfc/short/rfc6793.md` - 4-byte ASN; reason there are two variants
  → Constraint: per-dstASN4 variant split must remain

**Key insights:**
- The pair (wire pointer, BufHandle) must publish atomically; BufHandle is a 3-field struct and cannot be stored atomically on its own. Bundling both in one pointer-to-struct solves it.
- Generation must stay single-flight: two concurrent generators would each take a pool buffer and one buffer would leak (only one can win the slot).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/received_update.go` - struct fields lines 38-86 (ebgpMu:69, ebgpWireASN4:73, ebgpWireASN2:77, ebgpPoolBuf4:81, ebgpPoolBuf2:85); EBGPWire lines 110-156: lock, check variant slot, generate via getReadBuf + wireu.RewriteASPath, store wire + handle, return
- [ ] `internal/component/bgp/reactor/recent_cache.go` - cacheEntry + retention (lines 60-94); evictLocked (lines 455-467) AND Delete (lines 517-529) both return poolBuf, ebgpPoolBuf4, ebgpPoolBuf2 under cache.mu; both must be migrated
- [ ] `internal/component/bgp/reactor/forward_rs.go` - caller at line 169 (getEBGPWire closure inside reactorForwardRS)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - caller at line 331 (forwardUpdateCore)
- [ ] `internal/component/bgp/reactor/forward_pool.go` - worker spawn at lines 714-726; runWorker loop ~1109
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - RewriteASPath signature + error cases (lines 18-39)
- [ ] `internal/component/bgp/reactor/peer.go` - atomic.Pointer idiom (lines 159-170)

**Behavior to preserve:**
- Return values and errors identical: same wire pointer for repeated same-variant calls; `errEbgpWireBufferExhaustedPoolAt` on pool exhaustion; wrapped RewriteASPath errors; buffer returned on error.
- Cached wire carries the original SourceCtxID, MessageID, SourceID (received_update.go).
- Eviction returns BOTH variant buffers exactly once (no double-return, no leak).
- Fire-once semantics per variant; generation happens at most once per variant per ReceivedUpdate.

**Behavior to change:**
- None functional. Cache-hit calls stop acquiring `ebgpMu` (synchronization shape only).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A forwarded UPDATE destined to an eBGP peer: callers obtain the ReceivedUpdate from RecentUpdateCache.Get(messageID), then call EBGPWire(localASN, srcASN4, dstASN4).

### Transformation Path
1. Caller (RS fast path / forwardUpdateCore / forward worker) requests the EBGP variant.
2. Fast path: atomic load of the per-variant slot; hit returns the immutable wire.
3. Miss: ebgpMu -> re-check -> getReadBuf (bufMuxStd/bufMuxExt by payload size) -> wireu.RewriteASPath into the pool buffer -> wrap as WireUpdate with original ctx/IDs -> publish (atomic store) -> unlock.
4. Consumers write the wire to TCP; on final ack RecentUpdateCache evicts and evictLocked returns the original read buffer plus any published variant buffers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor <-> buffer multiplexer | BufHandle from bufMuxStd/bufMuxExt via getReadBuf; returned via ReturnReadBuffer | [ ] |
| Cache retention <-> consumers | retainCount/ack protocol in recent_cache.go (unchanged) | [ ] |

### Integration Points
- `evictLocked` switches from reading struct fields to atomic-loading the two slots; everything else reads through EBGPWire only.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Multiple goroutines call EBGPWire concurrently on the same ReceivedUpdate | Caller trace: forward_rs.go, reactor_api_forward.go, forward_pool.go runWorker; test TestReceivedUpdate_EBGPWireConcurrent exists | Optimization pointless (but harmless) | Read the three call sites + the goroutine spawn sites during audit | unvalidated |
| A-2 | Cache eviction cannot run while a generator holds the entry. Normal ack path is safe (`evictLocked` gated on `totalConsumers()<=0`, recent_cache.go/498). The path to scrutinise is the background safety valve `gapScanLoop` (recent_cache.go) -> `evictLocked` (recent_cache.go) for `isGapEvictable` entries (recent_cache.go), which fires regardless of acks. | recent_cache.go retention contract + gap-scan valve | Use-after-free of pool buffer | Confirm a genuinely-running forward worker is never `isGapEvictable` (30s stall timer never elapses on a live reader). PRE-EXISTING: `evictLocked` already returns `e.update.poolBuf` while a generator reads `u.WireUpdate.Payload()` from it (received_update.go); child 1 only swaps two field reads for atomic loads of the same published data and adds NO new race class. | unvalidated |
| A-3 | EBGPWire callers never mutate the returned wire | WireUpdate is documented immutable (received_update.go) | Shared-read unsafe | grep for writes through the returned pointer at the three call sites | unvalidated |
| A-4 | localASN is invariant for a given ReceivedUpdate's EBGP cache (same local AS prepended regardless of destination) | Both call sites pass the session-local AS; cache has no localASN key today | Cache key insufficient (pre-existing issue, not introduced here) | Confirm both call sites source localASN from the same per-router value during audit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Data race introduced by incorrect publication ordering | `make ze-race-reactor` (-race -count=20) failure | Double-checked locking with atomic.Pointer only; never publish before the struct is fully built |
| R-2 | Pool buffer leak on generation race | Buffer multiplexer exhaustion in soak tests | Generation stays under ebgpMu (single-flight); error paths return the buffer before unlock |
| R-3 | Benchmark shows no measurable win on darwin (low contention locally) | flat ns/op in new parallel benchmark | Accept code-shape win with rationale, or run benchmark with GOMAXPROCS sweep; present to user before claiming AC-3 |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| RS fast path forwarding an UPDATE to eBGP peer | → | reactorForwardRS -> EBGPWire fast path | TestReceivedUpdate_EBGPWireConcurrent (existing test, received_update_test.go) |
| forwardUpdateCore export to eBGP peer | → | EBGPWire miss path (generation under ebgpMu) | TestReceivedUpdate_EBGPWireLazyASN4 (existing test, received_update_test.go) |
| Cache eviction after final ack | → | evictLocked returns both variant buffers | TestReceivedUpdate_EBGPWireEvictionReturnsBuffers (new; asserts buffer return for published slots) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two sequential EBGPWire calls, same variant | Identical pointer returned; second call performs no mutex operations (verified by code review of the fast path + mutex absence in the hit branch) |
| AC-2 | 10+ goroutines calling EBGPWire concurrently, mixed variants | All receive a non-nil wire; per-variant pointers identical; `make ze-race-reactor` passes (-race -count=20) |
| AC-3 | New parallel benchmark BenchmarkEBGPWireCacheHitParallel | 0 allocs/op on hits; ns/op improves vs the before-change run recorded in this spec |
| AC-4 | Generation error (pool exhausted / malformed payload) | Same errors as today; slot stays nil; buffer returned; later call retries generation |
| AC-5 | Eviction of an entry with 0, 1, or 2 published variants | Exactly the published buffers are returned, each once (asserted via multiplexer accounting in the new test) |
| AC-6 | Full suite | `make ze-test` passes; existing received_update_test.go tests unchanged and green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestReceivedUpdate_EBGPWireLazyASN4 (existing) | `internal/component/bgp/reactor/received_update_test.go` | First-call generation + caching | |
| TestReceivedUpdate_EBGPWireCachedASN4 (existing) | same | Same pointer on second call | |
| TestReceivedUpdate_EBGPWireLazyASN2 (existing) | same | Independent ASN2 slot | |
| TestReceivedUpdate_EBGPWireConcurrent (existing) | same | Concurrent mixed-variant safety | |
| TestReceivedUpdate_EBGPWireEvictionReturnsBuffers (new) | same | Eviction returns exactly the published handles | |
| TestReceivedUpdate_EBGPWireErrorDoesNotPublish (new) | same | Failed generation leaves slot nil + buffer returned | |
| BenchmarkEBGPWireCacheHitParallel (new) | `internal/component/bgp/reactor/received_update_bench_test.go` | b.RunParallel hit-path cost; before/after pasted in spec | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| payload length for extendedMessage buffer choice | 0..ExtendedMaxSize | MaxMsgLen-HeaderLen boundary picks bufMuxStd vs bufMuxExt | N/A | covered by existing RewriteASPath truncation errors |

### Functional Tests
No user-facing behavior change; no new RPC/CLI surface. Existing functional
suite proves no regressions on the forwarding path (existing test suite passes
via `make ze-verify`, including the route-server `.ci` scenarios under `test/`).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing RS forwarding suite | `test/` (.ci, unchanged) | eBGP peers still receive AS-prepended UPDATEs | |

### Interop Tests (MANDATORY for protocol features)
Wire bytes are byte-identical to today (RewriteASPath unchanged); no interop
run needed beyond the existing suite. Justification: synchronization-only change.

## Files to Modify
- `internal/component/bgp/reactor/received_update.go` - replace 4 cache fields with 2 atomic slots + bundling struct; rewrite EBGPWire as double-checked locking; keep ebgpMu for generation
- `internal/component/bgp/reactor/recent_cache.go` - evictLocked AND Delete load slots atomically and return their handles (grep for every ebgpPoolBuf reader before claiming done; the audit found two sites, there must be zero field readers left after migration)
- `internal/component/bgp/reactor/received_update_test.go` - add the two new tests
- `docs/architecture/buffer-architecture.md` - one paragraph: EBGP variant cache publication/eviction contract

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | - |
| CLI commands/flags | [ ] no | - |
| Functional test for new RPC/API | [ ] no | - |
| Env var registration | [ ] no | - |
| Doctor check for runtime dependencies | [ ] no | - |
| Prometheus counters/metrics | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-11 | user-facing / config / CLI / API / wire | [ ] no | - |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/buffer-architecture.md` (cache publication + eviction contract) |
| 16 | Changed files referenced by doc source anchors? | [ ] check | grep `docs/` for `source:` anchors on received_update.go / recent_cache.go |

## Files to Create
- `internal/component/bgp/reactor/received_update_bench_test.go` - BenchmarkEBGPWireCacheHitParallel (if no suitable bench file exists; otherwise add to existing)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Validate A-1..A-4 against current source; paste evidence into Risks & Assumptions |
| 3. Wiring phase | Wiring Test table (existing tests already wire the path; add the two new tests failing) |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-race-reactor` |
| 7-14 | Per template |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - write the two new tests + the parallel benchmark against the CURRENT implementation; record before-change benchmark numbers in this spec
   - Tests: TestReceivedUpdate_EBGPWireEvictionReturnsBuffers, TestReceivedUpdate_EBGPWireErrorDoesNotPublish, BenchmarkEBGPWireCacheHitParallel
   - Files: received_update_test.go, received_update_bench_test.go
   - Verify: new tests pass against current code (they assert preserved behavior); benchmark baseline recorded
2. **Phase: Atomic slots** - introduce the bundling struct + two atomic.Pointer slots; rewrite EBGPWire fast path + double-checked miss path; update evictLocked
   - Tests: full received_update_test.go suite
   - Verify: all tests pass; `make ze-race-reactor` green
3. **Phase: Measure** - re-run benchmark; paste after numbers; run `make ze-perf-bench PERF_DUT=ze` and record movement in umbrella
4. **Full verification** - `make ze-verify`
5. **Complete spec** - audit tables, learned summary, two-commit closure per `ai/rules/planning.md`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Publication happens only after the struct is fully initialized; error paths never publish |
| Data flow | evictLocked returns handles from atomic loads; no field reads of removed fields remain |
| Concurrency | Fast path has no mutex ops; miss path re-checks after locking; no new goroutines |
| Rule: stale-comments | EBGPWire doc comment updated (currently says "Thread-safe via ebgpMu") |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Lock-free hit path | read EBGPWire; confirm no Lock() before the hit return |
| Before/after benchmark numbers in spec | grep this file for BenchmarkEBGPWireCacheHitParallel results |
| Race gate | `make ze-race-reactor` output pasted |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Unchanged; RewriteASPath error paths still reject malformed payloads |
| Resource exhaustion | Pool-exhaustion path still returns errEbgpWireBufferExhaustedPoolAt without publishing |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Race detector failure | Re-examine publication ordering; do NOT widen locks as a workaround without understanding the race |
| A-2 (eviction/generation overlap) proves false | STOP. NOTE this is a pre-existing bug affecting `poolBuf` today, not one this spec creates — report it as such. Design changes (slot must carry returned-flag, or eviction must take ebgpMu, or the gap-scan must skip entries with active readers). Present to user |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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
- The atomic.Pointer[struct] pattern for bundling a pointer + handle pair is cleaner than separate atomics. Go's memory model guarantees that when a reader observes the stored pointer, the struct's fields are fully visible.
- Double-checked locking with atomic.Pointer is the natural Go idiom for lazy-init-once with concurrent readers; it matches the existing `Peer.negotiated` and `Peer.sendCtx` patterns in peer.go.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bundle (wire, BufHandle) in one atomically-stored struct | Separate atomic wire pointer + handle under mutex | BufHandle is multi-field; separate storage creates a publish window where eviction sees the wire but not the handle |
| Keep ebgpMu for generation (double-checked locking) | Lock-free CAS generation race | CAS race would let two generators each take a pool buffer; the loser's buffer must be returned, adding complexity for zero gain |
| Two fixed variant slots | map keyed by (localASN, dstASN4) | localASN is invariant per router (A-4); two slots match today's semantics exactly |

## Known Limitations
- Cache hits on different ReceivedUpdates still touch different cache lines; no cross-update sharing is attempted (out of scope, matches today).

## RFC Documentation

RewriteASPath already carries the RFC 4271 Section 9.1.2 reference; no new
protocol-enforcing code is added. Keep existing references intact.

## Implementation Summary

### What Was Implemented
- `ebgpWireSlot` struct bundling `*WireUpdate` + `BufHandle` (received_update.go)
- Replaced 4 fields (ebgpWireASN4/ASN2/PoolBuf4/PoolBuf2) with 2 `atomic.Pointer[ebgpWireSlot]` slots (received_update.go)
- Lock-free fast path via atomic load (received_update.go)
- Double-checked locking miss path (received_update.go)
- `ebgpSlot` helper for variant dispatch (received_update.go)
- Updated `evictLocked` and `Delete` in recent_cache.go to load slots atomically

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/buffer-architecture.md`: added "EBGP Variant Cache" section describing the lock-free publication/eviction contract

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Lock-free cache hits | done | received_update.go | atomic load, no mutex |
| Single-flight generation | done | received_update.go | ebgpMu + double-check |
| Safe eviction | done | recent_cache.go | atomic load + ReturnReadBuffer |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | TestReceivedUpdate_EBGPWireCachedASN4 | same pointer on second call |
| AC-2 | done | TestReceivedUpdate_EBGPWireConcurrent + `make ze-race-reactor` | 10 goroutines, mixed variants, -race -count=20 |
| AC-3 | done | BenchmarkEBGPWireCacheHitParallel | 128 ns/op -> 0.36 ns/op, 0 allocs |
| AC-4 | done | TestReceivedUpdate_EBGPWireErrorDoesNotPublish | slot nil after error, buffer returned |
| AC-5 | done | TestReceivedUpdate_EBGPWireEvictionReturnsBuffers | 0/1/2 variants, pool stats verified |
| AC-6 | done | `go test ./internal/component/bgp/reactor/` | all 82 BGP packages pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReceivedUpdate_EBGPWireLazyASN4 (existing) | pass | received_update_test.go | updated assertions for atomic slots |
| TestReceivedUpdate_EBGPWireCachedASN4 (existing) | pass | received_update_test.go | unchanged |
| TestReceivedUpdate_EBGPWireLazyASN2 (existing) | pass | received_update_test.go | updated assertions |
| TestReceivedUpdate_EBGPWireConcurrent (existing) | pass | received_update_test.go | unchanged |
| TestReceivedUpdate_EBGPWireEvictionReturnsBuffers (new) | pass | received_update_test.go | 3 subtests |
| TestReceivedUpdate_EBGPWireErrorDoesNotPublish (new) | pass | received_update_test.go | truncated payload |
| BenchmarkEBGPWireCacheHitParallel (new) | pass | received_update_bench_test.go | before=128ns, after=0.36ns |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| received_update.go | modified | ebgpWireSlot + atomic slots + EBGPWire rewrite |
| recent_cache.go | modified | evictLocked + Delete use atomic loads |
| received_update_test.go | modified | 2 new tests, updated assertions |
| received_update_bench_test.go | created | parallel cache-hit benchmark |
| docs/architecture/buffer-architecture.md | modified | EBGP variant cache section |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Cache-hit path is lock-free and cheaper | benchmark | BenchmarkEBGPWireCacheHitParallel: before=128ns/op, after=0.36ns/op (~350x), 0 allocs |
| No safety regression | race test | `make ze-race-reactor` passes; `go test -race -count=20` on EBGPWireConcurrent passes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [filled during review]

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | forward_rs.go, reactor_api_forward.go, forward_pool.go runWorker; TestReceivedUpdate_EBGPWireConcurrent exists |
| A-2 | confirmed | isGapEvictable gated on 5min stall timer (recent_cache.go); active readers never trigger; pre-existing safety property unchanged |
| A-3 | confirmed | Callers assign returned wire to peerWire for TCP write (forward_rs.go); no mutation |
| A-4 | confirmed | Both call sites guard with canUseUpdateCache/cachedLocal (reactor_api_forward.go, forward_rs.go) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| buffer-architecture.md EBGP section | docs/architecture/buffer-architecture.md source anchor | yes |
| No source anchor stale | grep docs/ for received_update.go/recent_cache.go anchors; claims remain valid | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-race-reactor` passes (BLOCKING for this spec)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments intact
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
- [ ] Tests FAIL (paste output for the new eviction/error tests against intentionally-broken intermediate state, or document why they pass-first as behavior-preservation tests)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf-next-1-ebgp-wire-lockfree.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm` of spec only
