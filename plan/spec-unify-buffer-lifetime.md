# Spec: unify-buffer-lifetime

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-07-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `DESIGN-REVIEW.md` finding 2 (row "Attribute/buffer lifetime") and finding 6 ("Safety by convention on the hot path")
4. `ai/rules/memory-architecture.md`, `ai/rules/buffer-first.md` -- load-bearing memory divergences
5. Source: `internal/component/bgp/wireu/wire_update.go`, `internal/component/bgp/attrpool/pool.go`, `internal/component/bgp/attrpool/handle.go`, `internal/component/bgp/plugins/rib/forward_handle.go`, `internal/component/bgp/types/rawmessage.go`

## Task

DESIGN-REVIEW finding 2 flags that Ze holds route/attribute bytes past a call boundary through three separate, incompatible lifetime contracts:

1. **WireUpdate `Snapshot()` / `IsAsyncSafe()`** -- a per-message receive buffer copied eagerly on retain (`internal/component/bgp/wireu/wire_update.go:170-180`, `internal/component/bgp/types/rawmessage.go:42`).
2. **`attrpool` refcounted `Handle`** -- interned, deduplicated attribute bytes shared across many routes, lifetime managed by reference counting (`internal/component/bgp/attrpool/handle.go:29`, `pool.go:389,436,493`).
3. **`ribForwardHandle` copy-on-first-AddRef** -- one UPDATE's wire bytes handed to zero-or-more RIB Change subscribers, copied lazily only if someone retains (`internal/component/bgp/plugins/rib/forward_handle.go:30-77`).

This overlaps finding 6: all three are enforced by comment only, and all three fail SILENTLY (recycled bytes, another route's bytes, or freed bytes) rather than crashing.

The task is to decide, with evidence, whether these are true duplication (collapse into one type) or three legitimately different layers (keep separate, unify the enforcement discipline). Then fill the spec for the chosen direction: preserve all externally observable behavior (this is a refactor plus a debug-only safety layer), close the enforcement gaps, and give the three contracts one shared vocabulary.

## Required Reading

### Architecture Docs
- [ ] `DESIGN-REVIEW.md` -- findings 2 and 6, the source of this concern
  → Decision: finding 6 already prices the fix -- "a generation counter in Handle is one uint32 comparison per Get, and a debug-build poison pattern (instead of zeroing) plus a retained-pointer canary would make violations crash loudly." This spec realizes that, but adapts it to the packed 32-bit Handle reality.
  → Constraint: the three failure modes are all silent-wrong-data on route data, which the review says "deserve mechanical enforcement." Enforcement must be mechanical, not a new comment.
- [ ] `docs/architecture/pool-architecture.md` -- attrpool double-buffer, refcount, compaction model
  → Constraint: the Handle ABI is a packed 32 bits (1 bufferBit + 5 poolIdx + 26 slot); global deduplication and stable-across-compaction handles are load-bearing. A generation tag has no spare bits inside the existing uint32.
  → Decision: the pool already ships a debug/release build-tag validation split (`validate_debug.go` vs `validate_release.go`); that split is the mature enforcement pattern the other two contracts lack and should adopt.
- [ ] `ai/rules/memory-architecture.md`, `ai/rules/buffer-first.md` -- zero-copy, borrow-vs-own discipline
  → Constraint: zero-copy borrows are the norm on the hot path; the enforcement must not add a per-Get copy or a release-build allocation. Debug-only cost is acceptable; release-build cost is not.
- [ ] `ai/rules/no-fabrication.md`
  → Constraint: every behavioral claim in this spec cites the producing function file:line.

**Key insights:**
- The three contracts have mutually exclusive copy semantics (eager copy / never copy / lazy copy), each correct for its layer. Merging the types is wrong; unifying the enforcement discipline and vocabulary is right.
- The attrpool Handle is fully packed, so an always-on generation tag needs a 64-bit Handle (touching ~15 caller files). A debug-build "do not reuse freed slots" rule reaches the same detection with zero ABI change and zero caller migration.
- The debug/release split already exists in attrpool; the unification extends it outward, plus one shared poison helper and one documented boundary vocabulary.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/wire_update.go` - `Snapshot()` (:170-180) allocates and copies the whole payload into an owned WireUpdate; the type doc says "GC manages lifetime; no pool or reference counting." All accessor methods return slices into the payload.
- [ ] `internal/component/bgp/types/rawmessage.go` - `IsAsyncSafe()` (:42) returns `m.WireUpdate == nil`; an advisory boolean, not enforced. Doc: received-UPDATE `RawBytes` point at a buffer reused after the callback.
- [ ] `internal/component/bgp/attrpool/handle.go` - `Handle uint32` (:29), a packed bufferBit/poolIdx/slot with no free bits; no generation/epoch field.
- [ ] `internal/component/bgp/attrpool/pool.go` - `Intern` free-slot reuse (:347-370) resets `refCount=1; dead=false` on a recycled slot with NO generation bump; `Get` (:389-410) validates then returns a zero-copy slice; `Release` (:436-476) marks the slot dead and pushes it to `freeSlots`; `AddRef` (:493-517) increments. A stale handle to a re-interned slot passes `validateHandle` and resolves to a DIFFERENT route's bytes.
- [ ] `internal/component/bgp/attrpool/validate_debug.go` / `validate_release.go` - build-tag split; both reject invalid/wrong-pool/out-of-bounds/dead handles; debug attaches diagnostic detail. Neither detects stale-after-reuse (the slot is no longer dead).
- [ ] `internal/component/bgp/plugins/rib/forward_handle.go` - `ribForwardHandle` (:30-77): `source` valid only until the producing handler returns; first `AddRef` (:53-60) does a `sync.Once` copy into owned `buf` then nils `source`; `Release` (:64-66) is a bare decrement (no poison, no canary); `Bytes` (:76-78) returns `buf`, nil if AddRef never called.
- [ ] `internal/core/rib/locrib/forward_handle.go` - the `ForwardHandle` / `ForwardBytes` interface contract, all enforced by doc-comment ("MUST call AddRef before returning from the handler", "MUST NOT read Bytes after its matching Release").
- [ ] `internal/component/bgp/server/events.go` - the one `WireUpdate.Snapshot()` call site (:314): eager snapshot when a structured handler is present, because fire-and-forget delivery may free the pool buffer before consumers finish.
- [ ] `internal/core/redistevents/pool.go` - adjacent 4th contract: `ReleaseBatch` (:68-83) `clear()`s entries (zero, not poison), no refcount; safe only because EventBus Emit is synchronous per the dispatch loop, not an enforced pool invariant.

**Behavior to preserve:**
- Zero-copy on the hot path: `attrpool.Get` returns a slice into the pool buffer with no copy; `WireUpdate` accessors return slices into the payload; a subscriber-free UPDATE pays zero byte copies through `ribForwardHandle`.
- `attrpool.Handle` ABI stays 32 bits in release builds (packed layout, global dedup, stable-across-compaction handles unchanged); all existing handle-holders (`bestpath.go:116` ASPathHandle, `ribout_entry.go:40` AttrHandle, `routeentry.go` Bundle/ASPath) keep an opaque uint32.
- `Snapshot()` still deep-copies on retain; `ribForwardHandle` still copies lazily on first AddRef; the eager/lazy/never-copy semantics of the three layers are each retained.
- Release-build behavior and cost are byte-for-byte unchanged: no new per-Get comparison, no new allocation, no new branch on the release hot path.
- Existing tests keep passing: `TestRawMessageIsAsyncSafe`, `TestForwardHandleBytesLazyCopy`, `TestForwardHandleRefcount`, `TestReactorForwardRSBufferLifetime`, `TestDebugValidationCatchesDeadSlot`, `TestSlotReuseStaleIndexEntry`, and the redistribute `.ci` suite.

**Behavior to change:**
- None on the release hot path -- internal refactor plus a debug-build-only enforcement layer. The only observable change is in debug builds: previously-silent lifetime violations now surface as an error or a poisoned read.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A BGP UPDATE arrives on a peer's receive buffer in the reactor. Its bytes flow into three distinct retention paths, each with its own lifetime contract:
- the wire payload wrapped in a `WireUpdate` / `RawMessage` and delivered to event subscribers,
- individual path attributes interned into the shared `attrpool` and referenced from RIB route entries,
- the whole UPDATE's wire bytes offered to RIB `Change` subscribers via a `ribForwardHandle`.

### Transformation Path
1. Reactor reads UPDATE bytes into a receive/recent-update buffer (`internal/component/bgp/reactor/recent_cache.go`, `bufmux.go`); the buffer is reused after delivery (retainCount/Decrement).
2. Attribute bytes are interned per-attribute into `attrpool` (`internal/component/bgp/plugins/rib/storage/attrparse.go` -> `pool.*.Intern`), returning refcounted `Handle`s stored in `RouteEntry` / bestpath structs.
3. The RIB producer wraps `msg.RawBytes` in a `ribForwardHandle` (`rib_structured.go:106` -> `newForwardHandle`) attached to the `locrib.Change`; subscribers call `AddRef` inside the handler to retain past its return.
4. Structured event subscribers receive a `RawMessage`; when present, `server/events.go:314` calls `WireUpdate.Snapshot()` to own the bytes past fire-and-forget delivery.
5. On completion each path releases: `attrpool.Release` marks the slot dead and frees it for reuse; `ribForwardHandle.Release` drops a ref; the reactor recycles the receive buffer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor receive buffer ↔ event subscriber | `RawMessage.RawBytes` slice, valid until handler return; `Snapshot()` to own | [ ] |
| Wire attributes ↔ shared attrpool | per-attribute `Intern` -> refcounted `Handle`; zero-copy `Get` | [ ] |
| RIB producer ↔ Change subscriber | `locrib.Change.Forward` (`ForwardHandle`); `AddRef`/`Bytes`/`Release` | [ ] |
| Debug enforcement helper ↔ each contract | shared poison/canary helper called at each recycle/release site (build-tagged no-op in release) | [ ] |

### Integration Points
- `internal/component/bgp/attrpool` -- `intern`/`release`/`get` gain debug-only ABA detection and dead-slot poison; the existing `validate_debug.go` split is the anchor.
- `internal/component/bgp/plugins/rib/forward_handle.go` -- `Release`/`Bytes` gain a debug canary (poison-after-release).
- `internal/component/bgp/reactor/recent_cache.go` / `bufmux.go` -- receive buffer recycle point gains a debug poison call.
- `internal/core/redistevents/pool.go` -- `ReleaseBatch` upgrades its zeroing to poison in debug builds.
- One shared leaf helper (proposed `internal/core/memguard`) -- single `Poison`/`IsPoisoned` implementation, build-tagged; all four contracts reference it and one documented boundary vocabulary.

### Architectural Verification
- [ ] No bypassed layers (each contract keeps its own copy semantics; the enforcement is additive)
- [ ] No unintended coupling (the shared helper is a leaf utility called by each contract; the three types stay independent and are not merged)
- [ ] No duplicated functionality (one poison implementation replaces four ad-hoc zero/no-op patterns; one boundary vocabulary replaces three reverse-engineered contracts)
- [ ] Zero-copy preserved where applicable (release builds keep the exact zero-copy borrow paths; poison and generation checks are debug-build-only)
- [ ] Registration over hardcoding -- the enforcement helper is a leaf utility each contract calls directly; no new per-feature field, switch case, or factory is added to a core/shared struct, and no central registry needs to learn about the individual contracts (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `attrpool.Handle` is fully packed (1+5+26 bits) with no spare bit for an always-on generation tag | `internal/component/bgp/attrpool/handle.go:15-33` bit-layout comment | If wrong, a generation could be packed into the existing uint32 with no ABI widening, simplifying the fix | Re-read handle.go bit layout; confirm slot uses the full 26 bits (it does: `shardIDBits=4` + `shardSlotBits=22`) | **confirmed** (handle.go:15-33 + 42-54: all 32 bits used, 26-bit slot split 4+22, no spare bit) |
| A-2 | A debug-build "do not reuse freed slots" rule turns stale-after-reuse into the already-detected `ErrSlotDead` | `pool.go:347-360` (free-slot reuse) + `validate_debug.go:26-29` (dead check) | If a released slot must be reused even in debug (e.g. a test asserts reuse), this detector cannot be debug-only and needs the 64-bit generation instead | Grep attrpool tests for reuse assertions; add a debug-gated branch in `intern` and run the suite | **confirmed-with-refinement** — validate rejects dead slots in BOTH builds (validate_debug.go:26-29 + validate_release.go:24-27), so no-reuse → stale handle hits `ErrSlotDead`. Refinement: three existing tests assert release-only behavior and are made build-aware (skip/branch in debug), NOT the 64-bit fallback: `TestInternReuseDuringCompactionKeepsData` (compaction_test.go:276) and `TestSlotReuseStaleIndexEntry` (pool_test.go:966) hard-assert reuse; `TestForwardHandleBytesLazyCopy` (rib_bestchange_test.go) asserts buf-not-zeroed after Release. Reuse is a memory optimization, not mandatory in debug, so the design holds. |
| A-3 | The reactor owns the receive-buffer recycle point and can poison it in debug before reuse | `server/events.go:307-320` (buffer freed via cache Activate/Decrement) + `recent_cache.go` retainCount/Decrement | If the buffer is recycled outside a single reactor-owned site, poisoning needs to hook multiple places | Trace the Decrement -> reuse path in `recent_cache.go`/`bufmux.go`; place one debug poison call at the reuse point | **confirmed** — the single receive-buffer recycle site is `ReturnReadBuffer` (session.go:118), called by cache evictLocked/Delete and the forward paths; `returnReadBuffer` (session.go:572) delegates to it. Poison `h.Buf` there before the mux return. |
| A-4 | No production code depends on reading `attrpool` dead-slot bytes or post-Release `Bytes()` (the poison would only ever hit a bug) | `pool.go:459-473` (dead slot marked, bytes intact until compaction); `forward_handle.go:64-78` | If some path legitimately reads after release, poison would corrupt valid data -- but that path would already be a lifetime bug | Debug run of full `ze-test` with poison enabled; any failure is a real retained-pointer bug to fix at source | **pending full-debug-suite** — touched-package debug baseline is GREEN (attrpool/rib/reactor/redistevents/types/wireu all `ok` under `-tags debug`); final `go test -tags debug ./...` validates the rest. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Choosing the 64-bit Handle widening for production ABA enforcement doubles handle footprint in every RouteEntry/bestpath (millions of routes) | Memory benchmark regression in `ze-perf` RIB fill | Default to the debug-only no-reuse detector (zero ABI change); make 64-bit widening an explicit, separately-approved follow-up gated on a demonstrated production need |
| R-2 | Debug poison masks a race by changing timing, or a test relied on reading recycled/zeroed data | A debug-only test starts failing that passes in release | Treat as a real bug per `feedback_sleep_hides_races`; poison exposing a race is the intended outcome, fix the retained pointer |
| R-3 | Debug-build no-reuse of slots grows memory unboundedly in long-running debug/chaos tests | Debug chaos run OOMs where release does not | Cap the detector to a bounded ring of recently-freed slots, or reclaim after a generation window; the goal is catching the ABA in tests, not permanent retention |
| R-4 | The shared vocabulary doc drifts from the four contracts over time | Doc review finds a contract comment not referencing the vocabulary | Add a doc source-anchor from each contract's doc-comment to the vocabulary doc so `ze-doc-test` catches drift |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Structured handler retains received UPDATE bytes | → | `WireUpdate.Snapshot()` + debug receive-buffer poison at recycle | `TestReactorForwardRSBufferLifetime` (existing) + `TestWireUpdateBufferPoisonedAfterRecycle` (new, debug) |
| RIB Change subscriber retains wire bytes past handler | → | `ribForwardHandle.AddRef/Bytes/Release` + debug poison-after-release canary | `TestForwardHandleBytesLazyCopy` (existing) + `TestForwardHandleBytesAfterReleasePoisoned` (new, debug) |
| Attribute interned, released, slot re-interned for a different route | → | `attrpool.intern` debug no-reuse + `validateHandle` dead detection | `TestSlotReuseStaleIndexEntry` (existing regression) + `TestDebugStaleHandleAfterReuse` (new, debug) |
| `attrpool.Get` on a released handle | → | `validate_debug.go` dead-slot check | `TestDebugValidationCatchesDeadSlot` (existing) |
| Retained redist batch read after `ReleaseBatch` | → | `redistevents.ReleaseBatch` debug poison | `TestReleaseBatchPoisonsEntries` (new, debug) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Debug build: intern data, release the handle, intern different data (slot reuse), then `Get` the OLD handle | Returns an error (dead/stale), NOT another route's bytes; currently silent-wrong |
| AC-2 | Debug build: `ribForwardHandle.AddRef`, `Release`, then read `Bytes()` | Returns poison / trips an assertion, NOT the live copy |
| AC-3 | Debug build: retain `RawMessage.RawBytes` past handler return without `Snapshot()`, then read after the reactor recycles the buffer | Reads a poison pattern, detectable by the test; currently reads recycled bytes silently |
| AC-4 | Grep the tree for the poison/canary implementation and the boundary vocabulary | Exactly one shared poison helper; all four contracts' doc-comments reference one documented Boundary/Borrow/Retain/Own vocabulary |
| AC-5 | Release build: run `make ze-test` and a RIB-fill benchmark | Handle ABI is still uint32; no new per-Get cost; all existing tests pass; no benchmark regression |
| AC-6 | Debug build: `redistevents.ReleaseBatch` then inspect a retained batch | Entries are poison, not zero-valued (zero can masquerade as valid) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDebugStaleHandleAfterReuse` | `internal/component/bgp/attrpool/debug_test.go` | Stale handle after slot reuse errors in debug (AC-1) | |
| `TestForwardHandleBytesAfterReleasePoisoned` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | `Bytes()` after Release is poisoned in debug (AC-2) | |
| `TestWireUpdateBufferPoisonedAfterRecycle` | `internal/component/bgp/reactor/recent_cache_test.go` | Retained `RawBytes` poisoned after recycle in debug (AC-3) | |
| `TestReleaseBatchPoisonsEntries` | `internal/core/redistevents/pool_test.go` | Poison, not zero, in debug (AC-6) | |
| `TestReleaseBuildHandleABIUnchanged` | `internal/component/bgp/attrpool/handle_test.go` | Handle stays uint32; no generation field in release ABI (AC-5) | |
| `TestSlotReuseStaleIndexEntry` | `internal/component/bgp/attrpool/pool_test.go` | Existing behavior preserved (regression) | |
| `TestDebugValidationCatchesDeadSlot` | `internal/component/bgp/attrpool/debug_test.go` | Existing dead-slot detection still fires (regression) | |
| `TestRawMessageIsAsyncSafe` | `internal/component/bgp/types/types_test.go` | `IsAsyncSafe` contract preserved (regression) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-redist-bgp` | `test/ospf/ospf-redist-bgp.ci` | Routes redistributed OSPF->BGP still forward correctly through the RIB Change/forward path (no user-facing behavior change; existing test suite passes with no regressions) | |
| `isis-redist-bgp` | `test/isis/isis-redist-bgp.ci` | Routes redistributed IS-IS->BGP still forward correctly; internal refactor preserves behavior, existing test suite passes with no regressions | |

## Files to Modify
- `internal/component/bgp/attrpool/pool.go` - debug-only "do not reuse freed slots" in `intern` (turns stale-after-reuse into the already-detected `ErrSlotDead`); poison dead-slot bytes on `release`/`releaseBySlot` in debug
- `internal/component/bgp/attrpool/validate_debug.go` - document the stale-after-reuse guarantee; add a generation compare ONLY if the 64-bit widening follow-up is later approved (default: unchanged)
- `internal/component/bgp/plugins/rib/forward_handle.go` - debug canary: poison `buf` and flag on `Release`; `Bytes()` after final Release returns poison / asserts in debug
- `internal/component/bgp/reactor/recent_cache.go` - place one debug poison call at the receive-buffer reuse point (retainCount reaches recyclable)
- `internal/component/bgp/reactor/bufmux.go` - if the reusable block free-list is the actual recycle point, poison the block in debug before it re-enters `free`
- `internal/core/redistevents/pool.go` - `ReleaseBatch` poisons entries in debug builds instead of only `clear()`ing them
- `internal/core/memguard/poison.go` - NEW leaf helper: single build-tagged `Poison([]byte)` / `IsPoisoned([]byte)` (no-op in release); the one implementation all four contracts call
- `internal/component/bgp/types/rawmessage.go` - doc-comment on `IsAsyncSafe` references the shared Boundary/Borrow/Retain/Own vocabulary
- `internal/component/bgp/wireu/wire_update.go` - doc-comment on `Snapshot` references the shared vocabulary
- `docs/architecture/pool-architecture.md` - add (or link a new `docs/architecture/memory/lifetime-contracts.md`) the shared "held past boundary" vocabulary that all four contracts reference, with a source anchor per contract

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring / characterization (MANDATORY FIRST)** -- capture current behavior before changing it
   - Tests: add characterization tests that document today's SILENT failures -- `TestDebugStaleHandleAfterReuse` (currently returns another route's bytes), `TestForwardHandleBytesAfterReleasePoisoned` (currently returns live bytes), `TestWireUpdateBufferPoisonedAfterRecycle` (currently reads recycled bytes). Write them to FAIL against current code (they assert the future loud behavior).
   - Files: the four `*_test.go` files above; no production change yet.
   - Verify: the new tests fail for the right reason (silent-wrong behavior observed), existing tests pass.
2. **Phase: Shared poison helper + vocabulary** -- one enforcement toolkit
   - Tests: unit tests for `memguard.Poison`/`IsPoisoned` (build-tagged no-op verified in release).
   - Files: `internal/core/memguard/poison.go` (+ release/debug build-tag split), `docs/architecture/memory/lifetime-contracts.md` with the Boundary/Borrow/Retain/Own terms.
   - Verify: helper compiles under both build tags; release build inlines to no-op.
3. **Phase: attrpool ABA detection (debug no-reuse + dead-slot poison)** -- close the winner's own gap
   - Tests: `TestDebugStaleHandleAfterReuse`, `TestReleaseBuildHandleABIUnchanged`; regressions `TestSlotReuseStaleIndexEntry`, `TestDebugValidationCatchesDeadSlot`.
   - Files: `pool.go` (`intern` debug branch, `release`/`releaseBySlot` poison), `validate_debug.go` doc.
   - Verify: AC-1 test passes in debug; release ABI and cost unchanged; all attrpool tests green.
4. **Phase: ribForwardHandle canary** -- port the discipline onto contract C
   - Tests: `TestForwardHandleBytesAfterReleasePoisoned`; regressions `TestForwardHandleBytesLazyCopy`, `TestForwardHandleRefcount`.
   - Files: `forward_handle.go`.
   - Verify: AC-2 passes in debug; lazy-copy and refcount behavior preserved in release.
5. **Phase: WireUpdate receive-buffer poison + redistevents poison** -- port onto contracts A and the 4th
   - Tests: `TestWireUpdateBufferPoisonedAfterRecycle`, `TestReleaseBatchPoisonsEntries`; regression `TestRawMessageIsAsyncSafe`, `TestReactorForwardRSBufferLifetime`.
   - Files: `recent_cache.go`/`bufmux.go`, `redistevents/pool.go`; doc-comment updates in `rawmessage.go`, `wire_update.go`.
   - Verify: AC-3, AC-6 pass in debug; `Snapshot`/`IsAsyncSafe` behavior preserved in release.
6. **Functional tests** -- run `test/ospf/ospf-redist-bgp.ci`, `test/isis/isis-redist-bgp.ci` in both build modes; confirm no behavior change.
7. **Full verification** -- `make ze-verify`; run the debug-tagged suite (`-tags debug`) to exercise the enforcement.
8. **Complete spec** -- fill audit tables, write learned summary; two commits per planning.md.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation with file:line and a debug-tagged test |
| Types not merged | The three types keep independent, mutually-exclusive copy semantics; no merged type introduced |
| Release-build parity | Handle stays uint32; no new per-Get comparison or allocation on the release path; benchmark shows no regression (AC-5) |
| Debug-only cost | Poison, no-reuse, and canary are all behind `//go:build debug`; release build inlines them to no-ops |
| Single implementation | Exactly one poison helper (`memguard`) and one boundary vocabulary; grep proves no duplicate poison/zero patterns remain across the four contracts |
| Silent-to-loud | Each previously-silent failure (recycled / another-route / freed bytes) now errors or reads poison in debug (AC-1..AC-3, AC-6) |
| Registration over hardcoding | The poison helper is a leaf utility called directly by each contract; no new per-feature field, switch case, or factory added to a core/shared struct, and no central registry learns about individual contracts (`ai/rules/plugin-self-containment.md`) |
| Doc anchors | Each contract's doc-comment links the shared vocabulary via a source anchor so `ze-doc-test` catches drift |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated (debug-tagged tests for AC-1..AC-3, AC-6; release parity for AC-5; grep for AC-4)
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests) in the default build
- [ ] Debug-tagged suite (`go test -tags debug ./...`) passes and exercises the enforcement
- [ ] Feature code integrated (`internal/component/bgp/attrpool`, `internal/component/bgp/plugins/rib`, `internal/component/bgp/reactor`, `internal/core/redistevents`, `internal/core/memguard`)
- [ ] Release-build parity proven: Handle ABI unchanged, no per-Get cost, no benchmark regression
- [ ] Decision recorded: keep three types, unify enforcement + vocabulary (do NOT merge)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Shared boundary vocabulary doc written and anchored from all four contracts
- [ ] 64-bit Handle generation widening evaluated and explicitly deferred (or approved) with a memory-cost basis
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (redist `.ci` in both build modes)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep the three types separate; unify the enforcement discipline and vocabulary, do NOT merge | (a) One merged lifetime type for all three; (b) collapse WireUpdate.Snapshot into ribForwardHandle's lazy-copy | The three govern different layers with mutually exclusive copy semantics: WireUpdate copies EAGERLY (single known retainer), attrpool NEVER copies (dedup sharing across thousands of routes is the whole point -- copy-on-retain destroys the memory win), ribForwardHandle copies LAZILY (usually zero retainers, pay nothing). No single type can be simultaneously never-copy and copy-on-retain. A merge would also couple the server-event layer to the RIB-forward layer. The genuine overlap is the ENFORCEMENT (all fail silently, none has a generation guard, only attrpool has a debug/release split) and the VOCABULARY (each contract's "held past boundary" is reverse-engineered separately). |
| attrpool ABA guard via debug-build "do not reuse freed slots" (primary), 64-bit generation Handle as a deferred production option | (a) Pack a generation into the existing uint32 (no spare bits -- rejected, A-1); (b) always-on 64-bit Handle with generation compare per Get | The Handle is fully packed (1+5+26 bits), so an always-on generation needs a 64-bit Handle, doubling footprint across millions of route entries and touching ~15 caller files (bestpath, ribout_entry, routeentry, attrparse, all Get sites). The debug-no-reuse rule reaches the SAME detection -- a stale handle hits a still-dead slot and trips the existing `ErrSlotDead` -- with zero ABI change, zero caller migration, and zero release-build cost. It is a test-time detector, matching the repo's existing `validate_debug.go` philosophy. The 64-bit widening remains available as a separately-approved follow-up if production (not test) enforcement is ever required. |
| One shared `memguard` poison helper + one boundary vocabulary referenced by all four contracts | Four ad-hoc poison/zero patterns (status quo: attrpool leaves dead bytes, ribForwardHandle bare decrement, redistevents zeroes, WireUpdate nothing) | A single build-tagged implementation removes four divergent patterns and gives readers one vocabulary (Boundary/Borrow/Retain/Own) to learn once. This is the concrete code-level unification finding 2 asks for, without forcing a type merge. |

## Known Limitations
- The debug-only detectors (no-reuse, poison, canary) catch violations in the test/chaos suite, not in production release binaries. Production ABA enforcement would require the 64-bit Handle widening, deliberately deferred here on a memory-cost basis (R-1).
- The 4th contract (`redistevents` event-batch pool) is included for the shared poison/vocabulary but keeps its no-refcount design; its safety still rests on synchronous Emit, which this spec documents rather than changes.
- A minor optimization exists (events.go could use lazy copy like ribForwardHandle and skip the eager Snapshot when a structured handler does not retain) but is explicitly out of scope: it is an optimization, not a redundancy, and pursuing it would couple the server-event and RIB-forward layers.

## Implementation Summary

### What Was Implemented
- **Decision recorded:** the three (really four) contracts stay SEPARATE; the enforcement and vocabulary are unified. No type merged. Handle stays uint32 (64-bit widening deferred).
- **`internal/core/memguard`** (NEW leaf): the one build-tagged poison primitive. `poison_debug.go` (`Enabled=true`, real `Poison`/`IsPoisoned`, rotating `{0xDE,0xAD,0xBE,0xEF}` pattern) / `poison_release.go` (`Enabled=false`, no-op). Callers gate slice-arg construction on `if memguard.Enabled` so release builds dead-code-eliminate the guard, the poison, and the slice header.
- **Contract B (attrpool):** `const slotReuseEnabled` in the existing `validate_debug.go`(false)/`validate_release.go`(true) split; `intern` gates free-slot reuse (and the pool-full check) on it, so debug never reuses a freed slot → a stale handle trips the existing `ErrSlotDead`. Extracted `retireSlot` (DRYs `release`/`releaseBySlot`) which poisons dead-slot bytes under `if memguard.Enabled`.
- **Contract C (rib `forward_handle.go`):** `Release` poisons the owned `buf` on the final decrement (`if memguard.Enabled && n <= 0`); `h.refs.Add(-1)` still always runs, so release behavior is the unchanged bare decrement.
- **Contract A (reactor `session.go` `ReturnReadBuffer`):** poisons `h.Buf` at the single receive-buffer recycle site before the mux return.
- **Contract D (`redistevents` `ReleaseBatch`):** debug poisons entries via a struct sentinel (`poisonReleasedEntries`: scalar fields `0xDEADBEEF`, `Action=actionPoison`, netip fields left zero so their pointers stay nil/GC-safe) instead of `clear()`; release still `clear()`s. Byte-poison is unsafe here because `netip.Addr`/`Prefix` carry a `z` pointer.
- **Vocabulary doc** `docs/architecture/memory/lifetime-contracts.md` (Boundary/Borrow/Retain/Own) with a `<!-- source: -->` anchor per contract; doc-comments on `Snapshot` and `IsAsyncSafe` reference it.

### Bugs Found/Fixed
- No production bug found (the debug suite passed, confirming A-4: no production path reads dead-slot or post-release bytes). The silent lifetime hazards the spec targets are now caught loudly in debug.

### Documentation Updates
- NEW `docs/architecture/memory/lifetime-contracts.md` (shared vocabulary + 4 contract anchors).
- Doc-comments: `wireu/wire_update.go` `Snapshot`, `types/rawmessage.go` `IsAsyncSafe`, `rib/forward_handle.go` `Release`/`Bytes`, `attrpool` `validate_*.go` `slotReuseEnabled`, `redistevents/pool.go` `ReleaseBatch`/`poisonReleasedEntries`.

### Deviations from Plan
- **Phase order:** memguard (spec Phase 2) was built before the characterization tests (spec Phase 1) because the contract-C/D characterization tests import `memguard.IsPoisoned`. Per-phase TDD (write the failing debug test, then implement) was used within Phases 3-5 instead of a single up-front Phase 1. Same red-then-green discipline.
- **Test build-awareness (A-2 refinement):** three existing tests assert release-only behavior that the debug enforcement intentionally changes; made build-aware rather than the 64-bit fallback. `TestInternReuseDuringCompactionKeepsData` and `TestSlotReuseStaleIndexEntry` skip in debug (conditional `if !slotReuseEnabled`, annotated `// test-relax:`); `TestForwardHandleBytesLazyCopy` tail and `TestRouteChangeBatchPoolResetsOriginAS` branch on `memguard.Enabled` (added coverage, no assertion removed).
- **Contract-A test assertion:** `TestWireUpdateBufferPoisonedAfterRecycle` checks `IsPoisoned` on the whole recycled slot (not a sub-slice) because the repeating 4-byte pattern is phase-aligned to the slot's index 0; a retained sub-slice at a non-period-aligned offset reads poison out of phase. The test asserts the slot is poisoned end-to-end and the retained borrow no longer reads its live bytes.
- **Contract D poison mechanism:** struct sentinel, not `memguard.Poison`, because `netip` pointers preclude safe byte-poison (documented in the vocabulary doc and Known Limitations). Still gated on `memguard.Enabled`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Decide duplication vs distinct layers, with evidence | Done | Key Design Decisions + vocabulary doc | Distinct layers (mutually-exclusive copy semantics); keep separate |
| Preserve all externally observable behavior | Done | release paths unchanged; all release tests green | Poison/no-reuse are debug-only |
| Close the enforcement gaps (silent → loud) | Done | AC-1..AC-3, AC-6 tests | Each contract now errors/poisons in debug |
| One shared vocabulary | Done | `docs/architecture/memory/lifetime-contracts.md` | Boundary/Borrow/Retain/Own, 4 anchors |
| One shared poison helper | Done | `internal/core/memguard` | Contracts A/B/C; D uses gated struct sentinel (netip pointers) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDebugStaleHandleAfterReuse` (debug PASS) | stale handle → `ErrSlotDead`, slot not reused |
| AC-2 | Done | `TestForwardHandleBytesAfterReleasePoisoned` (debug PASS) | `Bytes()` after Release is poisoned |
| AC-3 | Done | `TestWireUpdateBufferPoisonedAfterRecycle` (debug PASS) | recycled receive buffer poisoned; borrow no longer reads live bytes |
| AC-4 | Done | grep: one `memguard.Poison`; 4 doc anchors reference the vocabulary | D's struct sentinel documented as the netip exception |
| AC-5 | Done | `TestReleaseBuildHandleABIUnchanged` (uint32); release tests green; lint clean both tags | no per-Get cost; poison behind `if memguard.Enabled` |
| AC-6 | Done | `TestReleaseBatchPoisonsEntries` (debug PASS) | entries poisoned (0xDEADBEEF), not zero |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDebugStaleHandleAfterReuse` | Done | `attrpool/debug_test.go` | AC-1 |
| `TestForwardHandleBytesAfterReleasePoisoned` | Done | `rib/rib_bestchange_test.go` | AC-2 (build-aware, both builds) |
| `TestWireUpdateBufferPoisonedAfterRecycle` | Done | `reactor/recent_cache_test.go` | AC-3 |
| `TestReleaseBatchPoisonsEntries` | Done | `redistevents/pool_test.go` | AC-6 |
| `TestReleaseBuildHandleABIUnchanged` | Done | `attrpool/handle_test.go` | AC-5 ABI, both builds |
| `TestSlotReuseStaleIndexEntry` | Preserved (skips in debug) | `attrpool/pool_test.go` | regression; release-only reuse |
| `TestDebugValidationCatchesDeadSlot` | Preserved | `attrpool/debug_test.go` | dead-slot regression, still fires |
| `TestRawMessageIsAsyncSafe` | Preserved | `types/types_test.go` | contract preserved |
| memguard `TestPoisonRoundTrip` / `TestPoisonEmptyAndNil` / `TestIsPoisonedRejectsPartialMatch` | Done | `memguard/memguard_test.go` | primitive, both builds |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `attrpool/pool.go` | Done | reuse gate, `retireSlot` poison |
| `attrpool/validate_debug.go` / `validate_release.go` | Done | `slotReuseEnabled` const |
| `rib/forward_handle.go` | Done | Release poison canary |
| `reactor/session.go` (`ReturnReadBuffer`) | Done | receive-buffer poison (the actual recycle site; `recent_cache.go`/`bufmux.go` not needed) |
| `redistevents/pool.go` | Done | struct-sentinel poison |
| `internal/core/memguard/poison.go` | Done (as `memguard.go` + `poison_debug.go` + `poison_release.go`) | build-tag split |
| `types/rawmessage.go`, `wireu/wire_update.go` | Done | vocabulary doc-comments |
| `docs/architecture/memory/lifetime-contracts.md` | Done | new vocabulary doc |

### Audit Summary
- **Total items:** 6 ACs + 5 requirements + planned files/tests
- **Done:** all ACs, all requirements, all planned files (with the noted file-name/location adaptations)
- **Partial:** none
- **Skipped:** none (the two reuse tests are *conditionally skipped in debug only* by design, not dropped; they run and pass in release)
- **Changed:** phase order, three tests made build-aware, contract-D sentinel mechanism (all in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/memguard/{memguard,poison_debug,poison_release,memguard_test}.go` | Yes | written this session; tests pass both builds |
| `docs/architecture/memory/lifetime-contracts.md` | Yes | new vocabulary doc, 4 anchors |
| `attrpool/{pool,validate_debug,validate_release,debug_test,handle_test,pool_test,compaction_test}.go` | Yes | modified |
| `rib/{forward_handle,rib_bestchange_test}.go` | Yes | modified |
| `reactor/{session,recent_cache_test}.go` | Yes | modified |
| `redistevents/{pool,pool_test}.go` | Yes | modified |
| `{wireu/wire_update,types/rawmessage}.go` | Yes | doc-comment updates |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | stale handle → ErrSlotDead | `TestDebugStaleHandleAfterReuse` PASS (debug) |
| AC-2 | Bytes() after Release poisoned | `TestForwardHandleBytesAfterReleasePoisoned` PASS (debug) |
| AC-3 | recycled receive buffer poisoned | `TestWireUpdateBufferPoisonedAfterRecycle` PASS (debug) |
| AC-4 | one poison helper + vocabulary | `rg "func Poison"` → memguard only; 4 contracts ref lifetime-contracts.md |
| AC-5 | Handle uint32, no release cost | `TestReleaseBuildHandleABIUnchanged` PASS; lint clean both tags; release tests green |
| AC-6 | entries poisoned not zeroed | `TestReleaseBatchPoisonsEntries` PASS (debug) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| redistribute OSPF→BGP forwards through RIB Change/forward path | `test/ospf/ospf-redist-bgp.ci` | Yes — `make ze-ospf-test` PASS (ospf-redist-bgp 60 checks); release behavior unchanged |
| redistribute IS-IS→BGP forwards | `test/isis/isis-redist-bgp.ci` | Yes — `make ze-isis-test` PASS (isis-redist-bgp 11 checks) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | handle.go:15-33/42-54 — 32 bits fully packed, no spare bit |
| A-2 | confirmed-with-refinement | validate rejects dead in both builds; 3 release-only tests made build-aware (not 64-bit fallback) |
| A-3 | confirmed | `ReturnReadBuffer` (session.go:118) is the single recycle site; poison placed there |
| A-4 | confirmed | previously-failing pkgs (config/doctor/plugin-all/dnsserver) + all touched pkgs pass under `-tags "debug ze_core <features>"` in BOTH builds → no production path reads dead/post-release bytes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| vocabulary doc anchors resolve to real symbols | 4 `<!-- source: -->` anchors → memguard/attrpool/rib/reactor/redistevents/wireu/types | Yes |
| Snapshot / IsAsyncSafe doc-comments reference vocabulary | wire_update.go / rawmessage.go edited | Yes |
| `ze-doc-test` | run in Phase 6 verification | Yes — PASS (after `make ze-discovery-index` regenerated the 3 indexes) |

## Review Gate

Result: **0 BLOCKER, 0 ISSUE.**

| Pass | Source | Findings | Disposition |
|------|--------|----------|-------------|
| Automated | `make ze-validate` | 1 real (`memguard.IsPoisoned` no non-test caller) + 20 pre-existing exports in touched files | Fixed via `IsPoisonedForTest` rename (`*ForTest` convention, validate.py:372); 20 pre-existing = false positives (ze-validate is post-verify, not in ze-verify) |
| Automated | `audit-test-relaxation.py` | 3 documented `[RELAXED]` | All legitimate: 2 reuse tests skip in debug (release-only reuse path, still run in release), 1 redistevents assertion made robust to sync.Pool non-determinism (exact value still checked deterministically on backing[0]) |
| Manual | own pass (logic/concurrency/perf/altitude) | 0 BLOCKER/ISSUE | release path byte-identical (all poison behind `if memguard.Enabled`); retireSlot extraction faithful; guards correct |
| Independent | fresh-eyes agent (correctness/concurrency/hidden-behavior) | 0 BLOCKER/ISSUE, 3 NOTE | All 3 NOTEs addressed (comment/doc accuracy: redistevents "0.0.0.0/0" rationale corrected, debug churn ceiling documented, migration-case canary gap noted) |

Verification: full debug suite (proper tags) PASS; touched packages PASS release+debug under `-race`; ospf/isis redist functional PASS; tier-check PASS; doc-test PASS; lint clean both tags. Pre-existing `internal/plugins/ospf` `TestVirtualLinkResolutionDrivesRuntime` data race (`virtual_link.go:160`) reproduces in a pure release build without this change — out of scope.
