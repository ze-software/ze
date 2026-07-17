# Spec: fixit-forward-readbuf-leak

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-perf-next-1-ebgp-wire-lockfree |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/memory-architecture.md`, `ai/rules/buffer-first.md` - pool lifecycle discipline
4. `internal/component/bgp/reactor/reactor_api_forward.go` (forwardUpdateCore, four leak sites)
5. `internal/component/bgp/reactor/forward_rs.go` (reactorForwardRS, two leak sites)
6. `internal/component/bgp/reactor/received_update.go` (ebgpWireSlot, the correct pattern)
7. `internal/component/bgp/reactor/recent_cache.go` (evictLocked/Delete, the return point)

## Task

**[BLOCKER]** The UPDATE forwarding / route-reflection / route-server path leaks a shared
read-pool buffer on every forwarded UPDATE under common local-AS configurations,
eventually failing all reads daemon-wide. Verified by the 2026-07-16 audit (verifier V6)
and re-verified against the current tree on 2026-07-16 (this design pass): six sites,
not four. All six citations below were re-read in the current working tree.

`forwardUpdateCore` and the RS fast path borrow a `BufHandle` via `getReadBuf` and, on the
success path, keep only the resulting `*wireu.WireUpdate`, discarding the handle needed to
return the buffer. `ReturnReadBuffer` is called only on the error branches. Verified sites
(borrow line / success-path wire-construction line where the handle is dropped):

| # | Site | Borrow | Wire built, handle dropped | Trigger |
|---|------|--------|---------------------------|---------|
| 1 | `reactor_api_forward.go` | 286 | 303 (returned 306) | export-filter wire override + EBGP prepend (per peer, per call) |
| 2 | `reactor_api_forward.go` | 357 | 376 (cached 380) | per-call `ebgpWireCache` miss: local-AS key not eligible for the ReceivedUpdate cache (second distinct localAS, or dual-AS `secondaryAS != 0`) |
| 3 | `reactor_api_forward.go` | 540 | 549 | RS-client ASN4->ASN2 transcode of an export-override wire (per peer) |
| 4 | `reactor_api_forward.go` | 560 | 571 (kept in `rsTranscodeWire` 574) | RS-client ASN4->ASN2 transcode, cached per call |
| 5 | `forward_rs.go` | 189 | 206 (cached 209) | RS fast path `ebgpWireCache` miss (same conditions as site 2) |
| 6 | `forward_rs.go` | 368 | 379 (kept in `rsTranscodeWire` 382) | RS fast path ASN4->ASN2 transcode, cached per call |

Citation drift from the skeleton: sites 5 and 6 moved 187 -> 189 and 366 -> 368; all four
`reactor_api_forward.go` sites are unchanged. The container `ebgpWireEntry` is `{wire,
failed}` with no handle field in both files (`reactor_api_forward.go:268-271`,
`forward_rs.go:131-134`). The comment at `reactor_api_forward.go:379` ("dst (pool buffer)
intentionally not returned: it backs wire for this call's lifetime") documents the bug:
nothing returns the buffer after the call's lifetime either.

The correct pattern is right next door: `ReceivedUpdate.EBGPWire` stores its handle in
`ebgpWireSlot{wire, handle}` (`received_update.go:24-27`, published at
`received_update.go:148`) and the cache returns it on eviction (`recent_cache.go:460-465`
in `evictLocked`, and `recent_cache.go:526-531` in `Delete`).

`getReadBuf` (`received_update.go:93-98`) draws from `bufMuxExt`/`bufMuxStd`
(`session.go:56-61`) -- the same bounded, globally shared pools the session read path
uses. A dropped handle permanently marks its slot in-use (`bufmux.go`: `bufBlock.inUse`
is only cleared by `put`, reachable only via `Return` with the dropped handle; the block
never becomes `fullyReturned`, so its budget bytes are never released). The pool is
byte-budgeted: auto-sized from peer weights when `ze.fwd.pool.maxbytes` is unset
(`reactor.go:445-448`, `updateBufMuxBudget`). Once every slot within budget has leaked,
`BufMux.getLocked` cannot grow (`growLocked` denied by `tryReserve`) and returns a nil
handle; every session's read then fails with `errReadBufferExhaustedPoolAtMaximum`
(`session_read.go:59-61`; also the coalesce path `session_coalesce.go:54-59`; error
defined `session_coalesce.go:21`) -- daemon-wide and permanent, because leaked slots
never return.

The fix is structural, not a one-line return: `buildFwdBody` aliases the borrowed buffer
zero-copy into the async worker writes (`forward_body.go:64` for the whole-payload path;
`wireu/split.go:34-79` sub-slices the same payload for the split path; the re-encode path
parses via `message.UnpackUpdate` over the same bytes), so the handle can only be
returned after the last dispatched write completes, never at end of call. See Key Design
Decisions: the chosen return point is cache-entry eviction, the same point that already
returns `poolBuf` and the two `ebgpWireSlot` handles.

**Coordination:** `plan/spec-perf-next-1-ebgp-wire-lockfree` (status in-progress) rewrote
`EBGPWire` and the eviction sites; its code is already committed on this branch (commit
`b5ad2cabe`, "perf(bgp): lock-free EBGPWire...") and the current tree reflects it. Only
that spec's closure (review gate, two-commit closure) remains. This spec is designed
against the committed lockfree code and composes with it (it adds a separate handle list;
it does not touch the two atomic slots). Sequencing decision recorded in Key Design
Decisions D-4.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/memory-architecture.md` - Incoming/Outgoing peer pools, refcount discipline
  → Constraint: every borrowed pool handle has exactly one matching return; a pool is not a counter.
  → Constraint: only pool-issued BufHandles (hook block-fake-bufhandle.sh enforces).
- [ ] `ai/rules/buffer-first.md` - buffer ownership across zero-copy forwarding
  → Constraint: modification-on-egress copies into the outgoing pool; the incoming handle must still be balanced.
- [ ] `docs/architecture/forward-congestion-pool.md` - forward worker goroutine model
  → Decision: one worker goroutine per destination peer; `safeBatchHandle` is where item resources are released.
- [ ] `docs/architecture/buffer-architecture.md` - EBGP variant cache publication/eviction contract (added by perf-next-1)
  → Constraint: eviction is the single return point for entry-owned read-pool handles; this spec extends that contract to per-forward handles.

**Key insights:**
- The buffer is aliased zero-copy into async dispatch, so the return point is after the last dispatched write, not at function return -- this is why the naive fix is a use-after-free.
- Every `forwardUpdateCore` / `reactorForwardRS` invocation operates on a cache-resident `ReceivedUpdate` (callers: `ForwardUpdate` `reactor_api_forward.go:178/226`; `ForwardUpdatesDirect` `reactor_api_forward_batch.go:111/131`; RS fast path `reactor_notify.go:553`), and each dispatched item holds a cache retain (`RetainN` at `reactor_api_forward.go:654`, `forward_rs.go:469`) released only after its bytes are written. Cache-entry eviction is therefore strictly after the last write -- a proven-safe return point.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore` (244-673); `getEBGPWire` closure (278-382) with sites 1-2; RS-client transcode block (535-580) with sites 3-4; `ebgpWireEntry{wire,failed}` (268-271); dispatch loop (653-666): `RetainN` then per-item `done` closure releasing the cache retain
  → Constraint: sites 1 and 3 are per-peer (up to one borrow per matching peer per call); sites 2, 4, 5, 6 are per-call (one borrow per cache key or transcode)
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRS` (83-492); sites 5-6; direct-write fast path `tryDirectWriteNoFlush` (28-72) consumes bodies synchronously; dispatch block (468-489): direct write calls `done()` inline (475) and returns only the outgoing peer-pool buffer (476-478), else falls back to `TryDispatch`/`DispatchOverflow`
- [ ] `internal/component/bgp/reactor/received_update.go` - `getReadBuf` (93-98); `ebgpWireSlot{wire,handle}` (24-27); `EBGPWire` (115-151) publishes wire+handle atomically (148) -- the correct handle-retaining pattern; `ReceivedUpdate` struct (47-89) with `poolBuf` (60) and the two atomic slots (84-88)
- [ ] `internal/component/bgp/reactor/recent_cache.go` - retention: `cacheEntry.totalConsumers` (122-124); ack-driven eviction `ackEntryLocked` (439-453); `evictLocked` (459-471) returns `poolBuf` + both slot handles (460-465); `Delete` (521-537) returns the same set (526-531); `RetainN` (546+); gap-scan safety valve (`isGapEvictable` 135-149, `runGapScan` 228-253) force-evicts entries stalled > 5 min
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `fwdItem` (51-63): carries `peerBufIdx`/`peerPoolRef`/`overflowBuf`, no read-pool handle; `releaseItem` (441-451, skeleton cited 439-449) returns outgoing-pool + overflow resources only; `safeBatchHandle` (878-897): deferred loop (880-887) calls `done()` + `releaseItem` per item AFTER the handler returns -- the async completion point; `fwdBatchHandler` (102-194) writes `rawBodies` into the session `bufWriter` and flushes; `DispatchOverflow` supersede path releases the superseded item inline (684-693); `Stop` drains overflow with `done()` + `releaseItem` (766-776)
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody` (37-107): same-context path appends `peerWire.Payload()` zero-copy into `rawBodies` (64) or split sub-slices (55-62); mismatch path parses the same bytes (69) and may alias them in `updates`
- [ ] `internal/component/bgp/reactor/bufmux.go` - `BufHandle` (21-25); `bufBlock.get`/`put` in-use accounting (56-85); `BufMux.Get`/`Return` (235-288); budget `tryReserve` (141-157); exhaustion returns zero handle (`getLocked` 244-264)
- [ ] `internal/component/bgp/reactor/session.go` - `bufMuxStd`/`bufMuxExt` globals (56-61); `ReturnReadBuffer` (110-136) routes by handle length; budget wiring (67-84)
- [ ] `internal/component/bgp/reactor/session_read.go` - read loop borrow (59-61) fails with `errReadBufferExhaustedPoolAtMaximum` when the shared pool is exhausted
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeRawUpdateBody` (312-344): copies body into the session write scratch then `bufWriter.Write` -- the body bytes are consumed synchronously; passes `body` to the sent-event callback (336) under the pre-existing lifetime contract (memguard Contract A, `docs/architecture/memory/lifetime-contracts.md`)

**Behavior to preserve:**
- Zero-copy forwarding (no new copy of the UPDATE body on the forward path).
- The `EBGPWire` atomic-slot path (perf-next-1), which is already correct -- do not regress it, do not touch the two slots.
- Pool sizing and the `errReadBufferExhaustedPoolAtMaximum` fail-closed behavior on genuine exhaustion.
- Error-path returns at all six sites (`ReturnReadBuffer` on rewrite/transcode failure) stay immediate.
- Per-call `ebgpWireCache` semantics: same wire reused across peers within one call.

**Behavior to change:**
- The six leak sites: retain each successfully-borrowed handle on the cache entry and return it at eviction.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A received UPDATE is forwarded to an EBGP/RS peer whose facts require a rewritten or transcoded wire: local-AS override (second distinct localAS in one call), dual-AS prepend (`secondaryAS != 0`, the default when a local-as override has neither no-prepend nor replace-as), export-filter wire override, or ASN4->ASN2 RS-client transcode.
- Three callers, all cache-resident: `ForwardUpdate` (plugin/API), `ForwardUpdatesDirect` (bgp-rs batch), `reactorForwardRS` (reactor-native RS fast path, `reactor_notify.go:553`).

### Transformation Path
1. `forwardUpdateCore` (or `reactorForwardRS`) computes per-destination facts and calls `getEBGPWire` / the transcode block.
2. A rewrite/transcode branch calls `getReadBuf` to borrow a pool buffer and builds a `*WireUpdate` over it (`RewriteASPath`/`RewriteASPathDual`/`TranscodeASPath`). Today the handle is dropped here on success.
3. NEW: the site immediately hands the handle to the `ReceivedUpdate` (adopt method appends it to an entry-owned handle list under a leaf mutex).
4. `buildFwdBody` aliases that buffer zero-copy into `rawBodies` (`forward_body.go:64`, split path via `wireu/split.go`); items are appended to `pending`.
5. `RetainN(updateID, len(pending))` then dispatch: pool path (`TryDispatch`/`DispatchOverflow`) or RS direct write. Each item's `done()` releases one retain only after its bytes are written (`safeBatchHandle` deferred loop `forward_pool.go:880-887`; RS inline `forward_rs.go:475`).
6. When plugin acks + retains reach zero, `ackEntryLocked`/`Decrement` calls `evictLocked`, which NEW: drains the entry's handle list via `ReturnReadBuffer`, alongside `poolBuf` and the two `ebgpWireSlot` handles. `Delete` does the same.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Read pool <-> forward path | `getReadBuf` borrow; NEW return via entry handle list at eviction | [ ] |
| Forward path <-> async workers | zero-copy alias into `rawBodies` (`forward_body.go:64`); consumed by `writeRawUpdateBody` (`session_write.go:312`) | [ ] |
| Forward path <-> cache retention | `RetainN` before dispatch; `done()` per item after write; eviction gated on `totalConsumers() <= 0` (`recent_cache.go:450`, 122-124) | [ ] |
| Read pool <-> session read | shared `bufMuxStd`/`bufMuxExt` (`session.go:56-61`); exhaustion blocks reads (`session_read.go:59-61`) | [ ] |

### Integration Points
- `ReceivedUpdate`: new entry-owned handle list + adopt method, sibling of the existing `poolBuf`/`ebgpSlot` ownership (`received_update.go:58-88`).
- `RecentUpdateCache.evictLocked` (`recent_cache.go:459-471`) and `Delete` (521-537): drain the list -- the ONLY two eviction paths (grep confirmed: all evictions route through these).
- The six borrow sites adopt on success; error paths unchanged.

### Architectural Verification
- [ ] No bypassed layers (return goes through `ReturnReadBuffer`, the same pool API eviction already uses)
- [ ] Zero-copy preserved (no added body copy; adopt stores a 3-field handle)
- [ ] No duplicated functionality (extends the existing entry-owned-handle pattern of `poolBuf`/`ebgpWireSlot`; does not invent a second lifecycle)
- [ ] Registration over hardcoding -- N/A (internal buffer lifecycle), stated for completeness (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The async workers have a well-defined completion point where per-item resources are released | RESOLVED this pass: `safeBatchHandle` deferred loop (`forward_pool.go:880-887`) runs `done()` + `releaseItem` after `fwdBatchHandler` returns; RS direct write completes synchronously and calls `done()` inline (`forward_rs.go:475`); overflow supersede (684-693), `Stop` (766-776), and pool-stopped `DispatchOverflow` (600-604, 617-623) all call `done()` | n/a | source read, lines cited | confirmed (design pass) |
| A-2 | Returning at cache-entry eviction is safe: no reader still aliases the buffer | Each dispatched item holds a retain until after its bytes are consumed (`writeRawUpdateBody` copies synchronously, `session_write.go:312-344`); eviction requires `totalConsumers() <= 0` (`recent_cache.go:450`, 122-124, 502). Pre-existing caveat: the gap-scan valve (`recent_cache.go:135-149`, 228-253) force-evicts entries stalled > 5 min regardless of retains -- the identical pre-existing exposure `poolBuf` and the ebgp slots already have (perf-next-1 A-2 confirmed a live worker never trips it). This spec adds no new race class. | Use-after-free | `-race` forwarding test + pool-balance test; re-confirm gap-valve reasoning during audit | unvalidated |
| A-3 | The six sites are the complete set of leaking `getReadBuf` callers | RESOLVED this pass: grep of non-test `getReadBuf` callers yields exactly seven: `received_update.go:133` (EBGPWire, correct -- handle stored in slot, returned at eviction) plus the six sites above. The session read path uses a different function (`Session.getReadBuffer`, `session.go:571-583`) with its own balanced lifecycle (`session_read.go:59/67-71`, `session_coalesce.go:54/170`). | A leak remains | grep re-run at implementation audit; each caller has a matching adopt-or-return | confirmed (design pass) |
| A-4 | Handles adopted onto the entry are returned exactly once | Eviction paths are exactly `evictLocked` + `Delete` (grep: no other `entries.Delete` with buffer ownership); list is drained-and-emptied under a mutex so a hypothetical double-evict returns nothing twice | Double return (pool corruption, guarded by `bufBlock.put` double-return check) | unit test asserting one return per adopt across evict + Delete | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Conflicts with spec-perf-next-1's remaining closure work (same files: `received_update.go`, `recent_cache.go`) | merge conflict / review-gate churn on those files | Sequence after perf-next-1 closes (D-4); its code is already committed (`b5ad2cabe`), so the surface is small |
| R-2 | Returning too early frees a buffer still aliased into a worker write | `-race` / crash / corrupt UPDATE bytes under forwarding load | Return ONLY at eviction (D-1); never add a return in `forwardUpdateCore`/`reactorForwardRS`; `-race` forwarding test |
| R-3 | Handle accumulation on long-retained entries (GR retain, slow consumer): per-call borrows now live until eviction | pool in-use grows with retained-entry count under churn | Accepted and documented (Known Limitations): normal eviction is immediate on last ack; growth is bounded by forwards-per-entry x distinct wire keys; same class as the existing ebgp slots |
| R-4 | Lock-order mistake between `cache.mu` (held in `evictLocked`) and the new handle-list mutex | deadlock under concurrent forward + evict | The list mutex is a leaf: adopt sites hold no other lock; eviction takes it strictly inside `cache.mu`; document the order in the struct comment |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| sustained forwarding to an EBGP peer with a local-AS override (dual-AS default, site 2/5) | -> | adopt at borrow, return at eviction | `TestForwardPoolBalanceLocalASOverride` |
| export-filter wire override to an EBGP peer (site 1) and its RS-client transcode (site 3) | -> | per-peer adopt sites balanced | `TestForwardPoolBalanceExportOverride` |
| RS-client ASN4->ASN2 transcode forwarding (sites 4/6) | -> | transcode adopt sites balanced | `TestForwardRSTranscodePoolBalance` |
| route-server fan-out under load | -> | pool in-use recovers after eviction | `test/plugin/forward-pool-balance.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | N forwarded UPDATEs through each of the six sites, entries evicted | pool in-use count (`bufMuxStd.Stats()` / `bufMuxExt.Stats()`) returns to baseline; no net leak |
| AC-2 | Dual-AS prepend config (local-AS override, default prepend) forwarding at churn | pool in-use stable over time; no `errReadBufferExhaustedPoolAtMaximum` |
| AC-3 | `-race` forwarding load exercising all six sites through dispatch + eviction | no data race, no use-after-free on the adopted buffers |
| AC-4 | Plain uniform-AS EBGP (no override) | unchanged -- rides the `EBGPWire` atomic slot, no regression, no double return of slot handles |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardPoolBalanceLocalASOverride` | `internal/component/bgp/reactor/forward_update_test.go` | AC-1/AC-2: sites 2 (and 5 via shared helper path); asserts `bufMuxStd.Stats()` in-use before == after eviction (pattern: `received_update_test.go:407-446`) | |
| `TestForwardPoolBalanceExportOverride` | `internal/component/bgp/reactor/forward_update_test.go` | AC-1: sites 1 and 3 (export override + override transcode), per-peer borrows | |
| `TestForwardRSTranscodePoolBalance` | `internal/component/bgp/reactor/forward_rs_test.go` | AC-1: sites 4, 5, 6 on the RS fast path | |
| `TestForwardBufferReturnAfterDispatch` | `internal/component/bgp/reactor/forward_update_test.go` | AC-3: handle NOT returned while an item is pending (retain held), returned after final `done()` + eviction; run under `-race` | |
| `TestReceivedUpdateAdoptedHandlesReturnedOnce` | `internal/component/bgp/reactor/received_update_test.go` | A-4: adopt K handles, evict (or Delete); each returned exactly once; empty list no-op | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pool in-use slots | 0..budget | budget | N/A | exhaustion returns nil handle; site marks `failed` / skips peer (fail-closed, unchanged) |
| adopted handles per entry | 0..(forward calls x distinct wire keys + override peers) | all returned at eviction | N/A | N/A (unbounded list by design; growth documented in Known Limitations) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `forward-pool-balance` | `test/plugin/forward-pool-balance.ci` | route server forwards sustained UPDATEs with local-AS override / dual-AS peers; sessions keep reading (no `errReadBufferExhaustedPoolAtMaximum`), forwarding stays healthy over the run | |

(Note: the skeleton placed this under `test/bgp/`; that directory does not exist -- BGP
forwarding `.ci` tests live in `test/plugin/` (e.g. `rs-fastpath.ci`,
`forward-backpressure.ci`).)

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-rs-forward-longrun` (next free NN) | `test/interop/scenarios/` | GoBGP or BIRD | long-running RS fan-out with local-AS override stays healthy (no read stall); wire bytes unchanged | |

## Files to Modify
- `internal/component/bgp/reactor/received_update.go` - add entry-owned adopted-handle list (leaf mutex + slice of `BufHandle`) and an adopt method; document the ownership contract next to `poolBuf`/`ebgpSlot`
- `internal/component/bgp/reactor/recent_cache.go` - `evictLocked` and `Delete` drain the adopted-handle list via `ReturnReadBuffer` (alongside the existing three returns in each)
- `internal/component/bgp/reactor/reactor_api_forward.go` - adopt the handle on the success path at sites 1 (286/303), 2 (357/376), 3 (540/549), 4 (560/571); fix the false comment at 379
- `internal/component/bgp/reactor/forward_rs.go` - adopt at sites 5 (189/206) and 6 (368/379)
- NOT modified (per D-1): `forward_pool.go` (`fwdItem`/`releaseItem` untouched), `forward_body.go`, the two `ebgpWireEntry` types (stay `{wire, failed}`)

## Files to Create
- `test/plugin/forward-pool-balance.ci` - functional pool-balance test
- `test/interop/scenarios/NN-rs-forward-longrun/` - interop long-run scenario

## Key Design Decisions

| ID | Decision | Alternatives Considered | Rationale |
|----|----------|------------------------|-----------|
| D-1 | Return point: hang every borrowed handle on the cache entry (`ReceivedUpdate`), returned when `evictLocked`/`Delete` fire | (a) refcounted handle carrier threaded through `fwdItem` and released in `releaseItem`; (b) return at end of `forwardUpdateCore` | (b) is a use-after-free: bodies alias the buffer into async writes (`forward_body.go:64`). (a) must cover at least five distinct release paths (`safeBatchHandle` 880-887, overflow supersede 684-693, `Stop` 766-776, pool-stopped `DispatchOverflow` 600-604 which today runs `done()` without `releaseItem`, and the zero-items-dispatched early return) plus shared-wire refcounts across items -- leak-prone. Eviction is ONE existing point, already returns `poolBuf` + both ebgp slot handles, and is provably after the last write: every dispatched item holds a retain (`RetainN`) released only post-write, and eviction requires `totalConsumers() <= 0` |
| D-2 | Storage: mutex-guarded `[]BufHandle` on `ReceivedUpdate` + adopt method; sites call adopt immediately after successful wire construction | handle field on `ebgpWireEntry` + on `fwdItem` (skeleton hypothesis); two fixed fields | Sites 1/3 are per-peer (N handles per call) and sites 2/5 are per-key (map) -- a slice is the only shape that fits all six; keeping `fwdItem` untouched avoids changing dispatch/release plumbing and leaves `forward_pool.go` out of the diff |
| D-3 | Lock: a dedicated leaf mutex on `ReceivedUpdate` for the list (adopt appends; eviction drains inside `cache.mu`) | reuse `ebgpMu`; atomic list | Leaf mutex gives a trivial no-inversion proof (adopters hold no other lock; `cache.mu -> listMu` is the only nesting); reusing `ebgpMu` couples unrelated lifecycles; atomics buy nothing for a drain-once slice |
| D-4 | Sequencing: implement AFTER spec-perf-next-1 closes; not folded in | fold into perf-next-1; implement concurrently | perf-next-1's code is already committed (`b5ad2cabe`); only its spec closure remains. Folding a correctness fix into a perf spec's closure mixes concerns and bloats its review gate. Both specs touch `received_update.go`/`recent_cache.go`, so serializing removes the conflict window. ~~Confirm with Thomas~~ RESOLVED 2026-07-17 (AUTONOMOUS DEFAULT, see Notes): sequence after perf-next-1 closes; not folded in |
| D-5 | Error paths keep their existing immediate `ReturnReadBuffer`; adopt happens only on success | adopt before rewrite, return via eviction even on failure | Failure means no wire references the buffer; immediate return (current behavior) keeps the pool tight and the diff minimal |

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- add the pool-balance invariant tests that FAIL on the current leak: `TestForwardPoolBalanceLocalASOverride`, `TestForwardPoolBalanceExportOverride`, `TestForwardRSTranscodePoolBalance` (assert in-use returns to baseline after eviction; they fail red today because the six sites drop handles).
2. **Phase: entry-owned handle list** -- add the list + leaf mutex + adopt method to `ReceivedUpdate`; drain in `evictLocked` AND `Delete`; `TestReceivedUpdateAdoptedHandlesReturnedOnce` green.
3. **Phase: adopt at the six sites** -- one site at a time, red-to-green against the phase-1 tests; fix the false comment at `reactor_api_forward.go:379`.
4. **Phase: sibling-site sweep** -- re-grep every `getReadBuf` and `ReturnReadBuffer` caller; assert each borrow is adopted or returned on every path; record the sweep output in the audit. Also record (not fix, unless Thomas approves scope) the adjacent observations listed in Notes.
5. **Phase: race + functional** -- `TestForwardBufferReturnAfterDispatch` under `-race`; `test/plugin/forward-pool-balance.ci`; interop long-run scenario.
6. **Full verification** -- `make ze-verify`.
7. **Complete spec** -- audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All six sites adopt their handle; grep proves no other `getReadBuf` leak (A-3 sweep pasted) |
| Correctness | Return happens ONLY in `evictLocked`/`Delete`; no return added in the forward functions; zero-copy preserved; error paths still return immediately |
| Sibling call-site audit | Every `getReadBuf` caller balanced; every `ReturnReadBuffer` site accounted for |
| No regression | `EBGPWire` slot path untouched (AC-4); slot handles returned exactly once (existing eviction lines unchanged) |
| Locking | Leaf-mutex order documented; no new lock taken under `ebgpMu`; `-race` clean |
| Registration over hardcoding | N/A -- internal buffer lifecycle (`ai/rules/plugin-self-containment.md`) |

## Known Limitations
- Adopted handles live until entry eviction, not until the last write that uses them. Normal operation evicts immediately on the last ack/release, so the window is short; long-retained entries (GR retain, stalled consumer) hold their per-forward buffers for the retention duration -- the same class of exposure as the existing `ebgpWireSlot` handles, now extended to per-call variants. Bounded by forwards-per-entry x distinct wire keys (+ override/transcode peers).
- The gap-scan safety valve can force-evict a stalled entry (> 5 min) while a pathological worker still holds aliases; this pre-existing exposure (shared with `poolBuf` and the ebgp slots, analyzed and accepted in spec-perf-next-1 A-2) is unchanged by this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `-race` forwarding test clean
- [ ] Registration over hardcoding respected (N/A stated)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Coordination with `spec-perf-next-1-ebgp-wire-lockfree` recorded (D-4 confirmed by Thomas)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for pool exhaustion
- [ ] Interop tests for long-run forwarding (or N/A with justification)

## Notes
- Skeleton captured from the 2026-07-16 repository audit (verifier V6). Deepened to design 2026-07-16: every `file:line` re-verified against the current tree (drift: `forward_rs.go` 187->189 and 366->368; `releaseItem` 439-449 -> 441-451; all other citations exact). A-1 and A-3 resolved by source read; A-2 and A-4 carry validation methods into implementation.
- Open question for Thomas (D-4): confirm sequencing AFTER perf-next-1 closure rather than folding this fix into that spec's remaining work.
  → AUTONOMOUS DEFAULT (2026-07-17): confirmed. Sequence AFTER spec-perf-next-1 closes; keep this correctness fix as its own self-contained spec, do NOT fold it into perf-next-1's remaining closure work. Rationale: this is the smaller, self-contained option (decision protocol for scope/scheduling picks the smaller self-contained choice). perf-next-1's code is already committed (b5ad2cabe) so only its spec closure remains; folding a correctness fix into a perf spec's review gate mixes concerns; both specs touch received_update.go and recent_cache.go, so serializing removes the conflict window. Matches D-4's existing recommendation. Thomas: override if wrong.
- Adjacent observations found during verification, OUT of this spec's scope (read, not exhaustively traced; flag for triage, do not fix here without approval):
  1. Outgoing peer-pool buffer leak on the body-build failure path: `forwardUpdateCore` acquires a mod buffer (`reactor_api_forward.go:589-605`), then `continue`s at 627-628 when `buildFwdBody` fails, dropping the item without `Return`; same shape in `forward_rs.go:392-411` / 436-437.
  2. Pool-stopped `DispatchOverflow` calls `done()` but not `releaseItem` (`forward_pool.go:600-604`, 617-623) -- shutdown-window-only leak of item resources.
