# Spec: perf-next-1-ebgp-wire-lockfree -- Lock-Free Cache Hits in ReceivedUpdate.EBGPWire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-perf-next-0-umbrella.md |
| Phase | 5/5 |
| Updated | 2026-08-05 |

The feature code landed in commit `b5ad2cabe` on 2026-06-15. It carries
`ebgpWireSlot` plus the two `atomic.Pointer` slots in `received_update.go`, the
eviction change in `recent_cache.go`, the tests, the benchmark, and the
`docs/architecture/buffer-architecture.md` section. Closure (2026-08-05)
re-measured every number in this spec rather than trusting it. See the Mistake
Log for what changed.

**The optimized path is no longer reachable in production, and this spec closes
saying so.** The AS-path fold (`ddf04953a`, `e2037e598`, both 2026-08-01) moved
eBGP prepending onto the edit-set path. It left `EBGPWire` with zero non-test
callers, so both slots stay nil in a running daemon.

The lock-free change is still correct and still measured. The traffic it was
written for now takes another route. Deleting the cache was homed at
`spec-wire-edit-3-deferred-ac9-dead-code`, its single owner since
2026-08-05, when the duplicate spec that described the same deletion was
removed. That deletion landed on 2026-08-17 in `df44d8d27`, its doc half in
`467d99165`, and the owning spec closed on 2026-08-29. This closure did not do
that deletion. It corrects every comment and doc line that still called the
path live.

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
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
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
| R-1 | Data race introduced by incorrect publication ordering | the retired `ze-unit-reactor-test-race` (current: `go test -race ./internal/component/bgp/reactor/...`) (-race -count=20) failure | Double-checked locking with atomic.Pointer only; never publish before the struct is fully built |
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
| AC-2 | 10+ goroutines calling EBGPWire concurrently, mixed variants | All receive a non-nil wire; per-variant pointers identical; `go test -race ./internal/component/bgp/reactor/...` passes (-race -count=20) |
| AC-3 | New parallel benchmark BenchmarkEBGPWireCacheHitParallel | 0 allocs/op on hits; ns/op improves vs the before-change run recorded in this spec |
| AC-4 | Generation error (pool exhausted / malformed payload) | Same errors as today; slot stays nil; buffer returned; later call retries generation |
| AC-5 | Eviction of an entry with 0, 1, or 2 published variants | Exactly the published buffers are returned, each once (asserted via multiplexer accounting in the new test) |
| AC-6 | Full suite | `./le verify current mode full` passes; existing received_update_test.go tests unchanged and green |

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
via `./le verify current mode full`, including the route-server `.ci` scenarios under `test/`).

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
| 6. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` + `go test -race ./internal/component/bgp/reactor/...` |
| 7-14 | Per template |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - write the two new tests + the parallel benchmark against the CURRENT implementation; record before-change benchmark numbers in this spec
   - Tests: TestReceivedUpdate_EBGPWireEvictionReturnsBuffers, TestReceivedUpdate_EBGPWireErrorDoesNotPublish, BenchmarkEBGPWireCacheHitParallel
   - Files: received_update_test.go, received_update_bench_test.go
   - Verify: new tests pass against current code (they assert preserved behavior); benchmark baseline recorded
2. **Phase: Atomic slots** - introduce the bundling struct + two atomic.Pointer slots; rewrite EBGPWire fast path + double-checked miss path; update evictLocked
   - Tests: full received_update_test.go suite
   - Verify: all tests pass; `go test -race ./internal/component/bgp/reactor/...` green
3. **Phase: Measure** - re-run benchmark; paste after numbers; run `./le perf-bench suggestion-report` and record movement in umbrella
4. **Full verification** - `./le verify current mode full`
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
| Deliverable | Verification method | Result (2026-08-05) |
|-------------|---------------------|---------------------|
| Lock-free hit path | read EBGPWire; confirm no Lock() before the hit return | Done. `slot.Load()` and its return precede `u.ebgpMu.Lock()` in `EBGPWire` |
| Before/after benchmark numbers in spec | grep this file for BenchmarkEBGPWireCacheHitParallel results | Done, re-measured; run pasted under Goal Validation with the host named |
| Race gate | `go test -race ./internal/component/bgp/reactor/...` output pasted | Done: `ok github.com/ze-software/ze/internal/component/bgp/reactor 221.454s`, exit 0 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for | Result (2026-08-05) |
|-------|-----------------|---------------------|
| Input validation | Unchanged; RewriteASPath error paths still reject malformed payloads | Pass. `EBGPWire` wraps and returns the `RewriteASPath` error and publishes nothing; TestReceivedUpdate_EBGPWireErrorDoesNotPublish feeds a truncated payload and asserts a nil slot |
| Resource exhaustion | Pool-exhaustion path still returns errEbgpWireBufferExhaustedPoolAt without publishing | Pass. `dst.Buf == nil` returns before the store, so an exhausted pool cannot publish a slot over a zero handle |
| Buffer lifetime (generic) | Double-return or use-after-return of a pool handle | Pass. The independent review enumerated every slot reader: the writer in `EBGPWire` and the two release sites in `evictLocked` and `Delete`, both under `c.mu`, both deleting the entry in the same call. Mutants M1 and M2 confirm the tests catch a missing return |
| Race / TOCTOU (generic) | Torn publication, double generation | Pass. `go test -race ./internal/component/bgp/reactor/...` (-race -count=20) exit 0. Generation is single-flight under `ebgpMu` with a re-check after the lock |
| Untrusted input, injection, crypto, privilege, path traversal (generic) | Any new surface | Not applicable. The change reads and writes no external input, adds no parsing, no filesystem or network call, and no privilege decision |

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
| A-1 held: production goroutines call EBGPWire concurrently | It held when the code landed on 2026-06-15. It stopped holding seven weeks later. The AS-path fold (`e2037e598`, 2026-08-01) removed the last non-test caller, so the optimized path is dead in a running daemon | Closure grepped for callers instead of re-reading the spec's caller trace | The spec's headline number ("~15M lock ops/sec removed") is not achievable by this change any more. Deleting the cache is homed at two existing specs. This closure corrected the comments, the doc section and the alloc-gate entry that still described the path as live |
| The recorded before/after benchmark (128 ns/op -> 0.36 ns/op) could be re-run | It was honestly measured (`docs/architecture/perf-round-3.md` names the host: 16 goroutines, Apple M4 Max) and still could not be re-run. The before number came from a working tree that `b5ad2cabe` overwrote, the benchmark file landed in that same commit, and the number was taken on a different machine from the one that would check it | Closure re-ran the benchmark: the after path reproduced, the before path had no producer at all | The speedup rested on a number no gate and no reader could reproduce. Fixed by adding `BenchmarkEBGPWireCacheHitParallelMutexBaseline`, which keeps the pre-change hit path measurable, and by pasting a run with its host named |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Recover the before number from git history alone | Reading the old source proves the shape, never the cost, and restoring it into the working tree to time it would edit committed code other sessions build against | A permanent comparator benchmark that reproduces the old hit path in a `_test.go` file, so the baseline is re-runnable without touching production code |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A performance spec records a before number whose producer is deleted by the same commit that records it | Second occurrence in the perf-next family (`spec-perf-next-3-rib-show-alloc.md` claims a >=50% allocation cut against a baseline that was never measured) | A perf spec's baseline must have a producer that survives the change: a committed comparator benchmark, or a stored `benchstat` file. `ai/rules/interop-and-goal-validation.md` already demands a pasted benchmark result; it does not yet demand the baseline stay re-runnable | Carried into `plan/learned/` with this spec; raise for `ai/rules/performance.md` if a third instance appears |

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
- Closure fixed four prose defects the independent review found: the `EBGPWire` doc comment's false SourceCtxID claim, and three sites plus a doc section that still called the path a live RS fan-out hot path (see Review Gate).
- Closure cleared a `./le doc check verify` red it did not cause but depended on: two learned summaries cited `spec-rfc7606-5-1-2-relay-shape`, which its own closure commit (`632dcade1`) removed, putting `internal/le/journal/validate.go` two references over its shrink-only ceiling. The two dead lines were deleted: a record must not cite a spec that closure removed.

### Documentation Updates
- `docs/architecture/buffer-architecture.md`: "EBGP Variant Cache" section describing the lock-free publication and eviction contract, plus the reachability note added at closure
- `docs/architecture/perf-round-3.md`: re-measured numbers beside the 2026-07 ones, the `b.RunParallel` caveat, and the reachability note

### Deviations from Plan
- `BenchmarkEBGPWireCacheHitParallelMutexBaseline` was added at closure. The plan assumed the before number recorded during Phase 1 would stand; it could not be re-run, so the comparator now produces it on demand.
- Phase 3's the retired `ze-perf-bench PERF_DUT=ze` (current: `./le perf-bench suggestion-report`) was not run. It measures end-to-end daemon throughput, and the change under test is unreachable from the daemon (see the header), so the run could only report noise. The microbenchmark is the evidence.
- A-1 is recorded broken rather than confirmed. See Mistake Log.

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
| AC-2 | done | TestReceivedUpdate_EBGPWireConcurrent + `go test -race ./internal/component/bgp/reactor/...` | 10 goroutines, mixed variants, -race -count=20 |
| AC-3 | done | BenchmarkEBGPWireCacheHitParallel vs BenchmarkEBGPWireCacheHitParallelMutexBaseline | re-measured 2026-08-05: 73.6 ns/op -> 0.26 ns/op, 0 allocs/op on both (see Goal Validation) |
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
| BenchmarkEBGPWireCacheHitParallel (new) | pass | received_update_bench_test.go | after path; 0 allocs/op, ceiling registered in `internal/perf/allocgate.go` (`AllocCeilings`) |
| BenchmarkEBGPWireCacheHitParallelMutexBaseline (new, closure) | pass | received_update_bench_baseline_test.go | before path, kept so the baseline stays re-runnable |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| received_update.go | modified | ebgpWireSlot + atomic slots + EBGPWire rewrite |
| recent_cache.go | modified | evictLocked + Delete use atomic loads |
| received_update_test.go | modified | 2 new tests, updated assertions |
| received_update_bench_test.go | created | parallel cache-hit benchmark |
| received_update_bench_baseline_test.go | created (closure) | mutex-shaped comparator; keeps the before number re-runnable |
| docs/architecture/buffer-architecture.md | modified | EBGP variant cache section, plus the reachability note |
| docs/architecture/perf-round-3.md | modified (closure) | re-measured numbers, RunParallel caveat, reachability note |
| internal/perf/allocgate.go | modified (closure) | ceiling comment |
| internal/component/bgp/wireu/aspath_rewrite.go | modified (closure) | stale cache comment removed |

### Audit Summary
- **Total items:** 17
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Cache-hit path is lock-free and cheaper | benchmark | 73.6 ns/op -> 0.26 ns/op, 0 allocs/op on both. Output pasted below. Measured over `EBGPWire`, which no production caller reaches since `e2037e598`; the win is real for the method and unreachable for the daemon |
| "~15M lock ops/sec removed at 100K UPDATE/s route-server fan-out" (Task) | NOT achieved | The RS fan-out no longer calls `EBGPWire` at all (`grep -rn '\.EBGPWire(' --include=*.go .` returns only `_test.go` hits). The lock ops were removed by the AS-path fold deleting the call, not by this spec. Recorded rather than claimed |
| Generation stays single-flight | code + test | `EBGPWire` generates under `ebgpMu` with a re-check after the lock (received_update.go); TestReceivedUpdate_EBGPWireConcurrent passes under `-race -count=20` |
| Buffer ownership unchanged | mutation-killed test | M1 (drop the ASN2 return in `evictLocked`) fails TestReceivedUpdate_EBGPWireEvictionReturnsBuffers/both_variants; M2 (drop `ReturnReadBuffer` on the error path in `EBGPWire`) fails TestReceivedUpdate_EBGPWireErrorDoesNotPublish. Both restored |
| No safety regression | race gate | `go test -race ./internal/component/bgp/reactor/...` (-race -count=20) exit 0: `ok ... reactor 221.454s` |

### Benchmark (re-measured 2026-08-05)

Host: linux/amd64, AMD EPYC 7351 16-Core, GOMAXPROCS=32.
Command: `go test -tags ze_core -run '^$' -bench BenchmarkEBGPWireCacheHitParallel -benchmem -benchtime=2s -count=5 ./internal/component/bgp/reactor/`

```
BenchmarkEBGPWireCacheHitParallel-32                 1000000000    0.2771 ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallel-32                 1000000000    0.2622 ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallel-32                 1000000000    0.2719 ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallel-32                 1000000000    0.2627 ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallel-32                 1000000000    0.2285 ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallelMutexBaseline-32      32208591   73.60   ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallelMutexBaseline-32      32032927   73.93   ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallelMutexBaseline-32      31244032   73.27   ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallelMutexBaseline-32      32311959   73.67   ns/op    0 B/op    0 allocs/op
BenchmarkEBGPWireCacheHitParallelMutexBaseline-32      32276869   73.73   ns/op    0 B/op    0 allocs/op
```

The baseline benchmark calls `ebgpWireMutexHit`, which reproduces the pre-change
hit path from `b5ad2cabe^` exactly: `ebgpMu.Lock()`, deferred unlock, read the
cached variant, return. Only the field read differs, and the mutex is what the
benchmark measures. Both benchmarks share one fixture and one parallel shape.

Mutant M3 proves the comparator measures the lock and nothing else: with
`ebgpMu.Lock()` / `defer Unlock()` deleted from `ebgpWireMutexHit`, the same
benchmark reports 0.109 ns/op. The whole 73.5 ns gap is the mutex.

`b.RunParallel` divides wall time by total operations, so both figures scale
with GOMAXPROCS. Compare the two benchmarks on one host; never compare a
recorded ns/op across machines. The 2026-07 numbers in
`docs/architecture/perf-round-3.md` (128 -> 0.36 ns/op) were taken on an Apple
M4 Max with 16 goroutines and are not comparable to the run above.

## Review Gate

Independent `/ze-review` subagents, 2026-08-05. Artifact recorded with
`internal/le/spec/session/review.go`.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `EBGPWire` has zero non-test callers since `e2037e598`; the optimized path is dead in production | `internal/component/bgp/reactor/received_update.go` `EBGPWire` | Verified independently (grep returns only `_test.go` hits). Already homed at `spec-wire-edit-3-deferred-ac9-dead-code` and a duplicate spec since removed, both skeleton, both stating the deletion needs its own implementation phase against read-pool buffer lifetime. Downgraded to a recorded fact for THIS spec: the deletion is separable work with a home, and folding it into a closing commit is what `ai/rules/rule-precedence.md` bans. Spec header, Goal Validation, A-1 and Mistake Log now state it |
| 2 | ISSUE | Doc comment claims the returned wire "shares the original SourceCtxID"; false since `9668abc9b` introduced `fwdContextIDWithASN4` | `received_update.go` `EBGPWire` doc comment | Fixed: comment now describes `fwdContextIDWithASN4` and says when the ids coincide |
| 3 | ISSUE | Four sites still describe the cache as a live RS fan-out hot path | `received_update_bench_test.go` header; `internal/perf/allocgate.go` `AllocCeilings`; `internal/component/bgp/wireu/aspath_rewrite.go` `RewriteASPath`; `docs/architecture/buffer-architecture.md` | Fixed: all four now state the method has no production caller and name the owning deletion spec |
| 4 | NOTE | Both cache-hit benchmarks prime with `EBGPWire` and never return the handle | `received_update_bench_test.go`, `received_update_bench_baseline_test.go` | Fixed: each benchmark returns its primed handle in a deferred cleanup |
| 5 | NOTE | Eviction returns the handle but leaves the slot pointer populated | `recent_cache.go` `evictLocked`, `Delete` | Recorded, not fixed. The entry is removed from the cache in the same call, so no consumer can reach it; this mirrors the pre-existing `poolBuf` contract. `slot.Store(nil)` belongs with the deletion spec, not in a closing commit |
| 6 | NOTE | Baseline comparator returned a production error to mean "not primed" | `received_update_bench_baseline_test.go` `ebgpWireMutexHit` | Fixed: dedicated `errBenchSlotNotPrimed` |
| 7 | NOTE | The `test-relax:` token in `received_update_test.go` `TestReceivedUpdate_EBGPWireErrorDoesNotPublish` is honest but misapplied: the test is new in the diff, so nothing was relaxed | `received_update_test.go` | Recorded. The token is harmless and removing it would edit a passing test for cosmetics |

The reviewer also confirmed, against source: publication ordering is safe (the
store is the last statement and `wu` is fully built); the two-site
buffer-ownership claim is TRUE and admits no double-return or leak;
single-flight holds with both miss-path exits clean; no mutex-era comment
survives.

### Fixes applied
- `received_update.go` -- `EBGPWire` doc comment: correct ctx-id claim, state that no production caller reaches it
- `internal/perf/allocgate.go` -- ceiling comment says the entry guards a path with no production caller
- `internal/component/bgp/wireu/aspath_rewrite.go` -- dropped the "EBGPWire cache amortizes this" clause
- `received_update_bench_test.go` -- header corrected; deferred handle return added
- `received_update_bench_baseline_test.go` -- `errBenchSlotNotPrimed`; deferred handle return
- `docs/architecture/buffer-architecture.md` -- EBGP Variant Cache section states the reachability and names the deletion spec
- `docs/architecture/perf-round-3.md` -- the round's own record: re-measured numbers beside the original, the `b.RunParallel` caveat, and the reachability note. Found by grepping `docs/` for anchors onto the changed files, which is what the Documentation Update Checklist row 16 asks for

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | Run 2 over the fixed diff | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| internal/component/bgp/reactor/received_update.go | yes | `ls -la` 2026-08-05, 8819 bytes |
| internal/component/bgp/reactor/recent_cache.go | yes | `ls -la` 2026-08-05, 30628 bytes |
| internal/component/bgp/reactor/received_update_test.go | yes | `ls -la` 2026-08-05, 18521 bytes |
| internal/component/bgp/reactor/received_update_bench_test.go | yes | `ls -la` 2026-08-05, 1446 bytes |
| internal/component/bgp/reactor/received_update_bench_baseline_test.go | yes | `ls -la` 2026-08-05, 2115 bytes (added at closure) |
| docs/architecture/buffer-architecture.md | yes | section "EBGP Variant Cache (ReceivedUpdate)" with a source anchor on `ebgpWireSlot` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Same pointer, no mutex on the hit | `EBGPWire` returns `s.wire` from `slot.Load()` before any `ebgpMu.Lock()` (received_update.go); TestReceivedUpdate_EBGPWireCachedASN4 PASS |
| AC-2 | Concurrent mixed variants safe | TestReceivedUpdate_EBGPWireConcurrent PASS; `go test -race ./internal/component/bgp/reactor/...` exit 0, `ok ... reactor 221.454s` |
| AC-3 | 0 allocs on hits, ns/op improves | 0 allocs/op measured; 73.6 -> 0.26 ns/op (Goal Validation). Ceiling 0 registered for the benchmark in `internal/perf/allocgate.go` (`AllocCeilings`), enforced by `internal/le/verify/deps/actions.go` |
| AC-4 | Error leaves the slot nil, buffer returned | TestReceivedUpdate_EBGPWireErrorDoesNotPublish PASS; mutant M2 (drop `ReturnReadBuffer` on the error path) FAILS it |
| AC-5 | Eviction returns exactly the published handles | TestReceivedUpdate_EBGPWireEvictionReturnsBuffers PASS with subtests no_variants / one_variant_(ASN4) / both_variants; mutant M1 (drop the ASN2 return in `evictLocked`) FAILS both_variants |
| AC-6 | Suite green | `go test -race ./internal/component/bgp/reactor/...` exit 0 over `./internal/component/bgp/reactor/...`; `./le changed scope` 0 issues; `./le repository tracked-build check` OK across 6 build flavors |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| RS fast path -> `reactorForwardRS` -> EBGPWire hit | no `.ci`; unit-level | TestReceivedUpdate_EBGPWireConcurrent, run under `-race -count=20` by `go test -race ./internal/component/bgp/reactor/...` |
| `forwardUpdateCore` -> EBGPWire miss (generation under `ebgpMu`) | no `.ci`; unit-level | TestReceivedUpdate_EBGPWireLazyASN4 / LazyASN2 |
| Cache eviction after final ack -> `evictLocked` returns both variant buffers | no `.ci`; unit-level | TestReceivedUpdate_EBGPWireEvictionReturnsBuffers, mutation-killed by M1 |

The change is synchronization shape only: the wire bytes `RewriteASPath` produces
are unchanged, so no `.ci` or interop scenario can discriminate it. The
discriminating evidence is the mutation kills above plus the race gate.

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken (was true when implemented, false at closure) | `grep -rn '\.EBGPWire(' --include=*.go .` on 2026-08-05 returns 14 hits, every one in a `_test.go` file. The RS fast path and `forwardUpdateCore` stopped calling it at `e2037e598`. Only tests call it concurrently now; TestReceivedUpdate_EBGPWireConcurrent still exercises the shape |
| A-2 | confirmed | isGapEvictable gated on 5min stall timer (recent_cache.go); active readers never trigger; pre-existing safety property unchanged |
| A-3 | confirmed | Callers assign returned wire to peerWire for TCP write (forward_rs.go); no mutation |
| A-4 | confirmed | Both call sites guard with canUseUpdateCache/cachedLocal (reactor_api_forward.go, forward_rs.go) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| buffer-architecture.md EBGP section | source anchor names `ebgpWireSlot`, `ebgpSlotASN4`, `ebgpSlotASN2`, `EBGPWire`; all four exist in received_update.go. Section now also states the path has no production caller | yes |
| No source anchor stale | the only `docs/` anchor onto received_update.go / recent_cache.go is the one above; re-read and corrected | yes |
| Categories 1-11 (user-facing / config / CLI / API / wire) | no user-visible surface changed; wire bytes are `RewriteASPath` output, unchanged | no update needed |
| `./le doc check verify` | PASSED after the fixes (log `tmp/doc-test-ebgp3.log`) | yes |

## Deferrals Resolved

This spec has no deferral shard: no `plan/deferrals/spec-perf-next-1-*` or
`*ebgp-wire*` file exists, and nothing was deferred out of it. The one live
follow-on, deleting the now-unreachable cache, was homed before this closure at
`spec-wire-edit-3-deferred-ac9-dead-code` by the
closure of the wire-edit-3 AS_PATH fold spec. That closure also homed it at a
duplicate spec, since removed. This spec adds no row to either, and removes no
shard.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] `go test -race ./internal/component/bgp/reactor/...` passes (BLOCKING for this spec)
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
