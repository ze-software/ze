# Spec: wire-edit-3-deferred-ac9-dead-code -- delete the EBGP wire cache the AS-path fold made unreachable

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/7 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Handoff | verify |
| Updated | 2026-08-17 |

<!-- Handoff `verify`: the implementation session commits its work, sets Status to
     `verification`, and STOPS. A later Opus 5 session reviews that commit and closes
     the spec. The implementation session does NOT append plan/TEMPLATE-CLOSURE.md,
     does NOT run /ze-close, and does NOT git rm this file. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The AS-path fold (`ddf04953a`, `e2037e598`) moved eBGP AS_PATH prepending onto the
edit-set path. The per-`ReceivedUpdate` EBGP wire cache it replaced was left in the
tree. The fold spec `spec-wire-edit-3-aspath-fold` recorded its AC-9 as **Partial**
for that reason and closed. What survives is row 6 of
`plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, which homes the deletion here.

The work is to DELETE the cache. `ai/rules/no-layering.md` applies in its plainest
form: X was replaced by Y, so X goes. No deprecation comment, no build tag, no
"kept for reference" benchmark. Nothing user-visible changes. If anything does,
the fold was incomplete and that is the finding.

**This spec is the single owner of the deletion, as of 2026-08-05.**
`spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal` described the same
change and was removed rather than kept in step, because two specs for one change
means two closures, two reviews and a race over the same files.

### What is dead, re-verified 2026-08-11

| Symbol | Where | Verdict |
|--------|-------|---------|
| `EBGPWire` | `internal/component/bgp/reactor/received_update.go` | Zero non-test CALL sites tree-wide. Three non-test occurrences of the name survive, all in `internal/perf/allocgate.go`: two comments and one map key |
| `ebgpSlot` | `internal/component/bgp/reactor/received_update.go` | Called only by `EBGPWire` |
| `ebgpWireSlot` | `internal/component/bgp/reactor/received_update.go` | The slot struct. Used by the two fields, by `EBGPWire`'s store, and by four reads in `recent_cache.go` |
| `ebgpSlotASN4`, `ebgpSlotASN2` | `internal/component/bgp/reactor/received_update.go` | `atomic.Pointer[ebgpWireSlot]` fields on `ReceivedUpdate` |
| `ebgpMu` | `internal/component/bgp/reactor/received_update.go` | Guards the `EBGPWire` miss path only. One other user, a benchmark that also goes |
| `errEbgpWireBufferExhaustedPoolAt` | `internal/component/bgp/reactor/received_update.go` | Returned by `EBGPWire` only |
| `"BenchmarkEBGPWireCacheHitParallel"` in `AllocCeilings` | `internal/perf/allocgate.go` | A STRING key. The compiler cannot see it. See "The one thing the compiler will not catch" |

**The two slot fields have four non-test READERS**, two in `evictLocked` and two in
`Delete` (`internal/component/bgp/reactor/recent_cache.go`). They exist only to return
the slot's pooled buffer when a cache entry is evicted. `EBGPWire` is the sole writer
of those slots, and nothing outside a test calls it, so all four always find nil in a
running daemon. They are part of the same dead subsystem. A delete that removes the
fields without removing those four reads does not compile.

### The one thing the compiler will not catch

`internal/perf/allocgate.go` registers `BenchmarkEBGPWireCacheHitParallel` in
`AllocCeilings` with an allocation ceiling of 0. The registration is a map key, so it
is a string, and deleting the benchmark leaves the string behind with no diagnostic
from the build.

`CheckAllocCeilings` fails closed on a registered benchmark that is absent from the
benchmark output, so the leftover key IS caught: by `./le verify-deps alloc`, which runs
the reactor benchmarks for real and drives `TestAllocGateEnforce` with
`ZE_ALLOC_GATE_BENCH` pointing at the output.

**`go test -race ./internal/perf` does NOT catch it.** `TestAllocGateEnforce`
calls `t.Skip` when `ZE_ALLOC_GATE_BENCH` is unset, which is every plain `go test`
run. Every other test in that package reads the hand-written `sampleBenchOutput`
fixture, which still lists the benchmark, so all of them stay green with the stale key
in place. Run both targets. Only `./le verify-deps alloc` discriminates.

### What the delete must not silently discard

Deleting a test is normally forbidden (`ai/rules/testing.md`). It is legitimate HERE
for one specific reason, and the reason must hold per test: **the tests under deletion
are the only callers of the code under deletion, which is exactly why that code is
dead.** A test whose subject survives is NOT covered by that reason and must be
reworked instead. One test in scope is of that second kind, and it is named in "Files
to Modify".

The implementation agent MUST list every deleted `Test` and `Benchmark` function BY
NAME in its report, with the reason for each, so the reviewer can check that no test
covering surviving behavior went with them.

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 6.

## Required Reading

### Rules
- [ ] `ai/rules/no-layering.md` - when replacing X with Y, delete X
  → Decision: this spec is the deferred second half of that rule for the AS-path fold. Delete, never deprecate.
- [ ] `ai/rules/testing.md` - "Test Deletion and Weakening"
  → Constraint: "testing removed functionality" is a legitimate reason to delete a test. Each deleted test needs that reason stated individually, not one blanket sentence.
- [ ] `ai/rules/stale-comments.md` - a comment describing removed behavior goes with it
  → Constraint: five prose surfaces name this cache and outlive the code unless edited. They are listed in "Files to Modify". No gate reads them.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: "a test asserting the ABSENCE of something" passes when the mechanism is deleted. The Wiring Test section names the discriminating evidence for that reason.
- [ ] `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` row 6 - what survives of the fold spec's Partial AC-9
  → Constraint: AC-9's remaining half is exactly this delete. The row is already `deferred` and already names this spec.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 5.1.2, AS_PATH prepending on eBGP egress
  → Constraint: the obligation is met by the edit-set path, which this spec does not touch. Removing an unreachable second implementation must change no AS_PATH byte.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH NLRI encoding
  → Constraint: `rfc/requirements/rfc7911.md` proves `RFC7911-5-3` with two tagged tests in `forward_body_test.go`, `TestForwardSplitConvertsAddPathContext` and `TestForwardSplitSameContextKeepsRawSplit`, and pins each by line. The test this spec reworks sits between them, so the second pin moves.

**Key insights:**
- The delete is wider than the named symbols: four reads in `recent_cache.go` come with them, or the tree does not compile.
- The one reference the compiler cannot see is a string key in `internal/perf/allocgate.go`, and only `./le verify-deps alloc` catches it.
- One test caller, `TestForwardDoesNotRetranscodeASN2RewrittenWire`, tests SURVIVING code and must be reworked, not deleted.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/received_update.go` - defines `ebgpWireSlot`, `errEbgpWireBufferExhaustedPoolAt`, the `ebgpMu` mutex, the two `atomic.Pointer[ebgpWireSlot]` fields, `EBGPWire` and `ebgpSlot`. `EBGPWire` borrows a read-pool buffer, calls `wireu.RewriteASPath` to prepend the local ASN, wraps the result with `fwdContextIDWithASN4`, and publishes the wire plus its handle into the slot under double-checked locking
- [ ] `internal/component/bgp/reactor/recent_cache.go` - `evictLocked` and `Delete` each load both slot fields and call `ReturnReadBuffer` on any handle found. Four reads in total
- [ ] `internal/component/bgp/reactor/received_update_test.go` - six `TestReceivedUpdate_EBGPWire*` tests plus the helper `extractFirstASN`, which nothing else calls
- [ ] `internal/component/bgp/reactor/received_update_bench_test.go` - the whole file is `BenchmarkEBGPWireCacheHitParallel`
- [ ] `internal/component/bgp/reactor/received_update_bench_baseline_test.go` - the whole file is `ebgpWireMutexHit`, `errBenchSlotNotPrimed` and `BenchmarkEBGPWireCacheHitParallelMutexBaseline`, which reproduces the pre-lock-free mutex hit path by taking `ebgpMu` directly
- [ ] `internal/component/bgp/reactor/forward_body_test.go` - `TestForwardDoesNotRetranscodeASN2RewrittenWire` calls `EBGPWire` to BUILD a fixture, then asserts `buildFwdBody` does not transcode an already-rewritten ASN2 wire a second time
- [ ] `internal/perf/allocgate.go` - `AllocCeilings` registers the benchmark by string key with a ceiling of 0, and the package doc comment names EBGPWire as one of the gated hot paths
- [ ] `internal/perf/allocgate_test.go` - `sampleBenchOutput` carries a `BenchmarkEBGPWireCacheHitParallel-4` line, `TestParseAllocsPerOp` carries a matching `want` entry, and the file header comment names EBGPWire

**Behavior to preserve:**
- Every forwarded route keeps its current bytes. An eBGP peer sees the same AS_PATH before and after this delete, produced by the edit-set path, per RFC 4271 Section 5.1.2.
- Pool accounting stays balanced. The eviction path releases fewer buffers after the delete because there are fewer buffers to release, never because a release was dropped.
- `buildFwdBody` still refuses to re-transcode a wire whose context already says 2-octet AS_PATH. That assertion lives in a test that calls `EBGPWire` only to build its fixture, and it must survive the delete.
- `TestForwardPoolBalanceLocalASOverride`, `TestForwardRSTranscodePoolBalance` and `TestReceivedUpdateAdoptedHandlesReturnedOnce` keep passing unchanged. They cover `adoptFwdHandle` and `returnFwdHandles`, a DIFFERENT lifetime mechanism that is not in scope.
- The remaining six entries of `AllocCeilings` keep their ceilings and their fixture lines.

**Behavior to change:** none. This is a removal of unreachable code.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A received UPDATE forwarded to an eBGP peer. It enters the reactor as a
`ReceivedUpdate` holding a `wireu.WireUpdate` that slices into a session read-pool
buffer, created on the inbound UPDATE path in `reactor_notify.go`.

### Transformation Path
1. The forward path builds the eBGP AS_PATH through the edit set. This is the live
   route since the AS-path fold, and this spec does not touch it.
2. `EBGPWire` offers a second, older route to the same result: borrow a pool buffer,
   call `wireu.RewriteASPath`, wrap with a width-corrected ContextID, publish the wire
   and its handle into one of two atomic slots, return the wire.
3. Nothing outside a `_test.go` file takes route 2, so both slots stay nil.
4. On eviction, `evictLocked` and `Delete` load both slots to release their pooled
   buffers, and always find nil.
5. After this spec, stages 2 to 4 do not exist. Stage 1 is unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ buffer pool | `ebgpWireSlot.handle` released by `evictLocked` and `Delete` | Yes. Four reads, re-read 2026-08-11 |
| Reactor ↔ wireu | `wireu.RewriteASPath`, reached by `EBGPWire` and, separately, by the edit-set path | Yes. `RewriteASPath` has other callers and STAYS |
| Reactor ↔ perf gate | `AllocCeilings` names the benchmark by string, so the boundary is untyped | Yes. Three non-test occurrences in `internal/perf/allocgate.go` |
| Reactor ↔ RFC requirement index | `rfc/requirements/rfc7911.md` pins the two `RFC7911-5-3` tagged tests in `forward_body_test.go` by line | Yes. `TestForwardSplitSameContextKeepsRawSplit` sits below the test this spec reworks, so its pin moves |

### Integration Points
- `internal/component/bgp/reactor/recent_cache.go` - the eviction path that must lose its four slot reads.
- `internal/perf/allocgate.go` - the untyped registration that must lose its entry.
- `internal/component/bgp/reactor/forward_body_test.go` - the one test caller whose subject survives.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The edit-set path is the sole surviving producer of an eBGP AS_PATH. A tree-wide grep for `.EBGPWire(` over `*.go` returns zero non-test call sites, 2026-08-11 |
| No unintended coupling | Yes | The removal drops a coupling rather than adding one: `internal/perf` stops naming a reactor benchmark that no longer exists |
| No duplicated functionality | Yes | This spec exists BECAUSE a duplicate survived. The check is that none remains: after the delete, `wireu.RewriteASPath` has one reachable caller chain, through the edit set |
| Zero-copy preserved where applicable | Yes | No buffer lifetime changes. The `poolBuf` and `fwdHandles` release paths are untouched. Only the two always-nil slot releases go |
| Registration over hardcoding (`ai/rules/plugins.md`) | N-A | Removal only. No command, view, family or handler is added. The one registry touched, `AllocCeilings`, loses an entry, which is the registration pattern working as designed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `EBGPWire` has zero non-test call sites. | A tree-wide grep for `.EBGPWire(` over `*.go` on 2026-08-11 returns nothing outside `_test.go`. The only non-test occurrences of the NAME are three lines in `internal/perf/allocgate.go`. | The delete is not a delete. STOP, report the caller, and do not proceed. | Re-run the same grep as step 1 of implementation | confirmed |
| A-2 | The four `recent_cache.go` slot reads always find nil in production. | The slots can only be non-nil after an `EBGPWire` store, and A-1 says nothing outside a test calls it. `EBGPWire` is the sole writer: it holds the one `slot.Store` site in the package. | A live path populates the slots, so A-1 is wrong. Same stop as A-1. | A-1 plus the sole-writer grep for `ebgpSlotASN` | confirmed |
| A-3 | Every property the deleted tests assert dies with its subject, except one. | Read of all nine functions: six in `received_update_test.go` and two benchmarks assert `EBGPWire`'s own lazy generation, caching, concurrency and error handling. `TestForwardDoesNotRetranscodeASN2RewrittenWire` is the exception: its subject is `buildFwdBody`. | Any test whose property outlives its subject is reworked, never deleted. | Per-test reason recorded in the implementation report | confirmed |
| A-4 | `./le verify-deps alloc` is the only check that catches a leftover `AllocCeilings` string key. | `TestAllocGateEnforce` skips unless `ZE_ALLOC_GATE_BENCH` is set, and `internal/le/verifydeps/actions.go` is the only thing that sets it. Every other test in `internal/perf` reads the `sampleBenchOutput` fixture. | If some other gate catches it too, no harm: the spec still requires `./le verify-deps alloc`. | Run `./le verify-deps alloc` and confirm it reds with the key left in | unvalidated (no gate may run this session) |
| A-5 | `extractFirstASN` in `received_update_test.go` has no caller after the six tests go. | Its one call site sits inside `TestReceivedUpdate_EBGPWireLazyASN4`, which is deleted. | It has another caller and stays. Harmless either way. | The lint run: `golangci-lint` `unused` flags a leftover | confirmed |
| A-6 | `testUpdatePayloadWithASPath` and `buildUpdatePayload` survive the delete. | Both have callers outside the deleted set: `forward_readbuf_leak_test.go` calls the first at four sites, `forward_modbuf_leak_test.go` calls the second. | Deleting either breaks four other tests. | The package build | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A pool-accounting test reds because the eviction path lost a real release. | `TestForwardPoolBalanceLocalASOverride` or `TestForwardRSTranscodePoolBalance` fails. | The release is removed because the buffer no longer exists. If a balance test reds, a live path WAS using the slot and A-1 is wrong: stop and report. |
| R-2 | The `AllocCeilings` string key survives the delete and nothing local notices. | Nothing in `go test -race ./internal/perf` fails, while `./le verify-deps alloc` reports the benchmark absent. | Run `./le verify-deps alloc` explicitly. It is a `./le verify current mode full` stage and is NOT in `./le verify current mode changed`, so a scoped dev loop misses it. |
| R-3 | `c_test_weakening` in `.claude/hooks/pretool-writeedit.py` refuses the edits with exit 2, and the session is stuck. | The hook names "deleting Test/Fuzz/Benchmark function" or "replacing test content with empty string". | Expected, not a defect. Add a row to `test/weakened.md` naming the test and why the deletion is correct, BEFORE the edit. The hook reads that file, so a row written afterwards buys nothing. |
| R-4 | The commit is refused because a weakened test has no row, or because it does not carry `test/weakened.md`. | `internal/le/commit/prepare.go create` names the test needing a row, or names the file to stage. | Add the row, and pass `--file test/weakened.md` beside the code. The file is replaced per commit and never accumulates, so there is no count to hold down and no ceiling to raise. |
| R-5 | `rfc/requirements/rfc7911.md` goes stale because a tagged test in `forward_body_test.go` moved, reddening `./le rfc check`. | `./le rfc check` reports a stale index or a stale per-RFC file. | Run `./le rfc index-update` and commit BOTH `ai/RFC-REQUIREMENTS.md` and every changed file under `rfc/requirements/` in the same commit (`ai/rules/testing.md`). |
| R-6 | The measured before-and-after numbers in `docs/architecture/perf-round-3.md` become unreproducible, since the baseline benchmark goes with the cache. | Nothing fails. The measurement is simply gone. | Intended. The optimization was correct and the traffic it was written for takes another route. Section 1 of that page is rewritten to record the outcome in the past tense rather than pointing at a benchmark that no longer exists. Recorded as a Known Limitation, not restored. |
| R-7 | The docs pages naming the cache outlive it, because no gate reads them. | Nothing fails. `check_source_anchor_stale_paths` in `internal/le/docwiring/checks.go` validates only that the anchored FILE exists, never that the named symbols do. | The Documentation Update Checklist below lists all five prose surfaces by path. Treat them as part of the delete, not as follow-up. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | eBGP peers receive a wrong AS_PATH, or pooled buffers leak on eviction. Both are wire-visible or resource-visible, so this is not a cosmetic delete. The likeliest real failure is milder: a leftover `AllocCeilings` string key reds `./le verify-deps alloc` for the next session that runs a full verify. |
| How is it reverted? | Single commit revert. Nothing persists to disk, to a peer, or to config. |
| Who else touches this path? | Any session working the reactor forward path, `recent_cache.go` eviction, or the pool accounting. `internal/perf/allocgate.go` is touched by any session adding a hot-path benchmark. This checkout is shared, so re-check `git status` on all six source files before editing. |

## Wiring Test (MANDATORY -- NOT deferrable)

A deletion spec has an inverted wiring test: what must be proved is ABSENCE, and
`ai/rules/interop-and-goal-validation.md` warns that a test asserting absence usually
passes for the wrong reason. "The tests still pass" is worthless evidence here,
because deleting the mechanism leaves the same absence.

So the evidence is stated as three separate facts, and the third is the discriminator.
It is the one check that goes RED if the delete is done carelessly, and it is the only
one that can.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A received UPDATE forwarded to an eBGP peer | → | the edit-set AS-path fold, now the only route to an eBGP AS_PATH | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci`, unchanged and still green: the peer's AS_PATH is byte-identical before and after |
| A recent-cache entry is evicted | → | `evictLocked` and `Delete` release `poolBuf` and `fwdHandles`, with no slot reads left behind | `TestForwardPoolBalanceLocalASOverride` and `TestForwardRSTranscodePoolBalance`, both unchanged and still green |
| A benchmark registered in `AllocCeilings` no longer exists | → | the fail-closed missing-benchmark branch of `CheckAllocCeilings` | `TestAllocGateEnforce` driven by `./le verify-deps alloc`. THE DISCRIMINATOR: leave the string key `"BenchmarkEBGPWireCacheHitParallel"` in the map and this goes RED with "absent from benchmark output". Nothing else in the tree does |

**Proving the third row discriminates (`ai/rules/interop-and-goal-validation.md`).**
Before claiming the delete complete, run `./le verify-deps alloc` once with the
map entry deliberately still present and confirm it fails naming that benchmark.
Then remove the entry and confirm it passes. Paste both outcomes; without that
pair the row is an assertion of absence and proves nothing.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A tree-wide grep over `*.go` for `EBGPWire`, `ebgpWireSlot`, `ebgpSlot`, `ebgpMu` and `errEbgpWireBufferExhaustedPoolAt` | Zero matches |
| AC-2 | A grep for `EBGPWire` over `docs/` and `internal/perf/` | Zero matches. No comment, doc paragraph or map key names the removed cache |
| AC-3 | `go test -race ./internal/component/bgp/reactor` and `go test -race ./internal/perf` | Both pass. The reactor package compiles with the four `recent_cache.go` slot reads gone |
| AC-4 | `./le verify-deps alloc` with the `AllocCeilings` entry left in place | RED, naming `BenchmarkEBGPWireCacheHitParallel` as absent from benchmark output. This is the discriminating evidence for AC-2 |
| AC-5 | `./le verify-deps alloc` with the entry removed | Green. The six surviving registered benchmarks are all present and within their ceilings |
| AC-6 | An eBGP peer receiving a forwarded route, via `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Identical wire bytes before and after the delete. The AS_PATH is produced by the edit-set path, per RFC 4271 Section 5.1.2 |
| AC-7 | `go test -race ./internal/component/bgp/reactor/...` | Passes. The eviction path and the `ReceivedUpdate` struct lose shared state, so the race surface shrinks and must not have moved |
| AC-8 | Each deleted `Test` or `Benchmark` function | Named individually in the implementation report, with the reason "its subject was removed" and the symbol that subject was |
| AC-9 | `TestForwardDoesNotRetranscodeASN2RewrittenWire` | Still exists, still asserts the same AS_PATH of 65000 then 65001, and builds its fixture without `EBGPWire`. Its subject `buildFwdBody` survives, so the test survives |
| AC-10 | `./le rfc check` | Passes. `rfc/requirements/rfc7911.md` pins a tagged test that moves, so `./le rfc index-update` has been re-run and both its outputs committed |

## 🧪 TDD Test Plan

This is a deletion, so there is no new behavior to drive test-first. The plan is the
inverse: name the tests that must stay green untouched, and the one that must be
reworked in place. Any of them going red means the delete removed something live.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardPoolBalanceLocalASOverride` (existing, unchanged) | `internal/component/bgp/reactor/forward_readbuf_leak_test.go` | AC-3, AC-7. Pool balance across the eviction path after the two slot releases go | |
| `TestForwardRSTranscodePoolBalance` (existing, unchanged) | `internal/component/bgp/reactor/forward_readbuf_leak_test.go` | AC-3, AC-7. RS transcode borrow-and-return balance, the other eviction consumer | |
| `TestReceivedUpdateAdoptedHandlesReturnedOnce` (existing, unchanged) | `internal/component/bgp/reactor/received_update_test.go` | AC-3. `adoptFwdHandle` and `returnFwdHandles` are a separate lifetime mechanism and must be untouched by the delete | |
| `TestForwardDoesNotRetranscodeASN2RewrittenWire` (existing, REWORKED) | `internal/component/bgp/reactor/forward_body_test.go` | AC-9. Same assertions, fixture rebuilt without `EBGPWire` | |
| `TestAllocGateEnforce` (existing, unchanged) | `internal/perf/allocgate_test.go` | AC-4, AC-5. The discriminator. Runs only under `./le verify-deps alloc` | |
| `TestParseAllocsPerOp`, `TestAllocGateCeiling`, `TestAllocGateMissingFailsClosed` (existing, fixture edited) | `internal/perf/allocgate_test.go` | AC-3. `sampleBenchOutput` and the `want` map lose their EBGPWire line together, or the parsed-count assertion fails | |

### Boundary Tests (numeric inputs)
N-A. This spec removes code and adds no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-ebgp-shared.ci` (existing, unchanged) | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | An eBGP peer receives a forwarded route with the correct AS_PATH. No regression is the whole point, so the test must not be edited: an edited test proves nothing about a delete | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A, with a reason | `test/interop/scenarios/` | none | No wire-visible behavior changes. `ai/rules/interop-and-goal-validation.md` exempts a change with no wire-visible effect, and the existing eBGP interop scenarios already cover the surviving edit-set path. A new scenario here would be vacuous by that rule's own first trap: it would pass identically with the cache present or absent | |

## Files to Modify

Feature code:
- `internal/component/bgp/reactor/received_update.go` - remove `ebgpWireSlot`, `errEbgpWireBufferExhaustedPoolAt`, the `ebgpMu` field, both `atomic.Pointer[ebgpWireSlot]` fields, `EBGPWire` and `ebgpSlot`. Correct the `ReceivedUpdate` memory-contract doc comment and the `adoptFwdHandle` and `returnFwdHandles` comments, all of which name the ebgpSlot handles as a second release class
- `internal/component/bgp/reactor/recent_cache.go` - remove the four slot reads, two in `evictLocked` and two in `Delete`, and correct the `evictLocked` doc comment that says "any EBGP patched versions"
- `internal/perf/allocgate.go` - remove the `BenchmarkEBGPWireCacheHitParallel` entry from `AllocCeilings` and its comment, and drop EBGPWire from the package doc comment's hot-path list

Tests deleted, because their only subject is deleted:
- `internal/component/bgp/reactor/received_update_test.go` - delete `TestReceivedUpdate_EBGPWireLazyASN4`, `TestReceivedUpdate_EBGPWireCachedASN4`, `TestReceivedUpdate_EBGPWireLazyASN2`, `TestReceivedUpdate_EBGPWireConcurrent`, `TestReceivedUpdate_EBGPWireEvictionReturnsBuffers`, `TestReceivedUpdate_EBGPWireErrorDoesNotPublish`, and the helper `extractFirstASN` which nothing else calls. KEEP `buildUpdatePayload` and `testUpdatePayloadWithASPath`: both have callers in other files
- `internal/component/bgp/reactor/received_update_bench_test.go` - delete the WHOLE file. It is `BenchmarkEBGPWireCacheHitParallel` and nothing else
- `internal/component/bgp/reactor/received_update_bench_baseline_test.go` - delete the WHOLE file. It is `ebgpWireMutexHit`, `errBenchSlotNotPrimed` and `BenchmarkEBGPWireCacheHitParallelMutexBaseline`, all of which reach `ebgpMu` and the slots directly

Test reworked, NOT deleted:
- `internal/component/bgp/reactor/forward_body_test.go` - `TestForwardDoesNotRetranscodeASN2RewrittenWire` tests `buildFwdBody`, which survives. Every one of its assertions stays. Only the fixture changes: build the already-rewritten ASN2 wire directly with `wireu.RewriteASPath` into a plain byte slice, then `wireu.NewWireUpdate` with `fwdContextIDWithASN4` and a false ASN4 flag. That is exactly what `EBGPWire` did, minus the pool borrow and the slot publication, and both of those are what is being deleted

Test fixture:
- `internal/perf/allocgate_test.go` - remove the `BenchmarkEBGPWireCacheHitParallel-4` line from `sampleBenchOutput` AND its entry in the `want` map inside `TestParseAllocsPerOp`, in one edit. Removing either alone fails that test. Drop EBGPWire from the file's `PREVENTS:` header comment

Prose that outlives the code unless edited, and that no gate reads:
- `docs/architecture/buffer-architecture.md` - delete the whole "EBGP Variant Cache (ReceivedUpdate)" section, including its `source:` anchor comment and the sentence naming this spec
- `docs/architecture/perf-round-3.md` - rewrite section 1, "Lock-Free EBGPWire Cache Hits", in the past tense: the optimization landed, was measured, and was then deleted with its subsystem. Remove the `ebgpWireSlot` and `EBGPWire` names from the page's `received_update.go` anchor comment, keeping the two other anchors untouched
- `docs/functional-tests.md` - the alloc-gate paragraph lists "bufmux / forward-pool / EBGPWire" as the gated hot paths. Drop EBGPWire

RFC index, regenerated and never hand-edited:
- `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/rfc7911.md` - regenerate with `./le rfc index-update` after `forward_body_test.go` changes, and commit both with the change

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Removal only. No config surface is touched |
| YANG validation constraints | No | No leaf added or changed |
| YANG custom validators | No | No leaf added or changed |
| CLI commands/flags | No | The cache is unexported behavior with no CLI surface |
| CLI grammar (keyword before value) | No | No CLI change |
| Editor autocomplete | No | No YANG leaf change |
| Functional test for new RPC/API | No | No new RPC or API. Existing `.ci` coverage must stay green unchanged |
| Pipe completeness | No | No command output produced or changed |
| Env var registration | No | No env var added |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port or binary. The delete removes a memory-only cache |
| Prometheus counters/metrics | No | The cache exported no metric. `announce_metrics.go` names no ebgp cache counter |
| BGP family surface (new SAFI / capability / attribute) | No | No SAFI, capability or attribute added or removed. AS_PATH handling on egress is unchanged and stays with the edit-set path |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing user-facing is added or removed. `docs/features.md` never named the cache |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No CLI surface touched |
| 4 | API/RPC added/changed? | No | The cache was reachable from no API |
| 5 | Plugin added/changed? | No | Reactor-internal, no plugin boundary crossed |
| 6 | Has a user guide page? | No | No `docs/guide/` page names the cache. Verified by a grep for `EBGPWire` over `docs/` |
| 7 | Wire format changed? | No | Zero wire-visible change. That is AC-6 |
| 8 | Plugin SDK/protocol changed? | No | Unexported reactor internals only |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4271 Section 5.1.2 stays met by the edit-set path. No `rfc/short/` row and no `docs/features/rfc-status.md` row changes. The RFC INDEX is regenerated for a moved line pin only, which is bookkeeping and not a claim change |
| 10 | Test infrastructure changed? | **Yes** | `docs/functional-tests.md` names EBGPWire in the alloc-gate hot-path list. Drop it |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not name the cache |
| 12 | Internal architecture changed? | **Yes** | `docs/architecture/buffer-architecture.md`, delete the "EBGP Variant Cache" section. `docs/architecture/perf-round-3.md`, rewrite section 1 |
| 13 | Route metadata keys added/changed? | No | No metadata key involved |
| 14 | Prometheus counters added/changed? | No | The cache exported no counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `AllocCeilings` is a registration, but it is a test-gate registry, not a product inventory. No page in `docs/plugin-overview.md`, `docs/features/plugins.md` or `docs/guide/status.md` lists it |
| 16 | Any changed source file referenced by existing doc source anchors? | **Yes** | Two anchors name `internal/component/bgp/reactor/received_update.go` AND the symbols being deleted, one in `docs/architecture/buffer-architecture.md` and one in `docs/architecture/perf-round-3.md`. `check_source_anchor_stale_paths` validates only that the FILE exists, so neither goes red on its own. Both must be edited by hand |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No doc shows a config, CLI or API example reaching this code |

## Implementation Steps

**Before anything: re-check `git status` on all six source files.** This checkout is
shared. If any of them carries another session's uncommitted edits, STOP and report:
deleting symbols under a live edit discards that session's work
(`ai/rules/never-destroy-work.md`). All six were clean on 2026-08-11.

1. **Phase: Wiring (MANDATORY FIRST) -- prove the discriminator before deleting anything.**
   - Tests: `TestAllocGateEnforce`, driven by `./le verify-deps alloc`
   - Files: none edited in this phase
   - Verify: re-run A-1's tree-wide grep for `.EBGPWire(` over `*.go` and confirm zero non-test call sites. Do NOT proceed on a stale verdict; if a live caller exists, stop and report it. Then run `./le verify-deps alloc` and record that it is green today. That is the baseline the third Wiring Test row compares against
2. **Phase: delete the production symbols in one compiling step.**
   - Tests: `go test -race ./internal/component/bgp/reactor` must compile and pass
   - Files: `received_update.go` and `recent_cache.go`, edited TOGETHER. The four `recent_cache.go` reads must go in the same step as the fields, or the package does not build
   - Verify: the package builds. Comments naming the removed slots are corrected in the same edit (`ai/rules/stale-comments.md`), not left for a later pass
3. **Phase: retire the test callers whose subject is gone.**
   - Tests: `go test -race ./internal/component/bgp/reactor`
   - Files: `received_update_test.go`, six `TestReceivedUpdate_EBGPWire*` functions plus `extractFirstASN`, and both bench files in full
   - Verify: `c_test_weakening` in `.claude/hooks/pretool-writeedit.py` WILL refuse these edits with exit 2. That is expected. Add a row to `test/weakened.md` naming the test and why the deletion is correct, BEFORE the edit: the hook reads that file, so a row written afterwards buys nothing. For the two bench files, reduce each to its `package reactor` line, then let the commit remove them: `rm` and `git rm` on a `_test.go` are both blocked, so the working route is `internal/le/commit/prepare.go create --remove <path>`, whose generated script does the `git rm`. The commit MUST carry `test/weakened.md` itself (`--file test/weakened.md`), or `commit_helper.py` refuses it: a row left in the working tree puts the weakening in history with no reason beside it
4. **Phase: rework the one test whose subject survives.**
   - Tests: `TestForwardDoesNotRetranscodeASN2RewrittenWire`
   - Files: `forward_body_test.go`
   - Verify: every assertion is unchanged and the AS_PATH is still 65000 then 65001. Only the fixture construction changes, from `EBGPWire` to a direct `wireu.RewriteASPath` plus `wireu.NewWireUpdate` with `fwdContextIDWithASN4` and a false ASN4 flag. No assertion is removed, so this test needs no row in `test/weakened.md` and none should be written. Then run `./le rfc index-update` and commit both its outputs, because `rfc/requirements/rfc7911.md` pins a tagged test that this edit moves
5. **Phase: close the untyped reference and PROVE the discriminator.**
   - Tests: `TestAllocGateEnforce`, `TestParseAllocsPerOp`
   - Files: `internal/perf/allocgate.go`, `internal/perf/allocgate_test.go`
   - Verify: run `./le verify-deps alloc` with the `AllocCeilings` entry STILL PRESENT and confirm it reds naming the absent benchmark (AC-4). Paste that output. Then remove the entry, the `sampleBenchOutput` line and the `want` entry, re-run, and confirm green (AC-5). Paste that too. Without both outputs the absence proves nothing
6. **Phase: correct the prose the delete stranded.**
   - Tests: `./le doc-check verify`
   - Files: `docs/architecture/buffer-architecture.md`, `docs/architecture/perf-round-3.md`, `docs/functional-tests.md`
   - Verify: a grep for `EBGPWire` over `docs/` and `internal/` returns nothing (AC-1, AC-2). No gate reads these pages, so the grep IS the check
7. **Phase: gates, then stop.**
   - Run `go test -race ./internal/component/bgp/reactor/...`. This touches reactor state shared across goroutines, so `ai/rules/testing.md` requires it. Paste the output
   - Run `go test -race ./internal/component/bgp/reactor`, `go test -race ./internal/perf`, `./le verify-deps alloc`, `./le rfc check`, `./le doc-check verify`, `./le changed scope`, `./le test-weakened check`
   - Run `./le verify current mode full` once, at the end, in the foreground. Never poll
   - Commit with `internal/le/commit/prepare.go create`, passing `--remove` for the two deleted bench files. Run `./le repository-tracked-build check` immediately after the script, because the commit carries Go
   - **Handoff is `verify`: set Status to `verification` in this file, report, and STOP.** Do not append `plan/TEMPLATE-CLOSURE.md`, do not run `/ze-close`, do not `git rm` this spec. A later Opus 5 session reviews the commit and closes it

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | The four `recent_cache.go` reads went WITH the fields, in the same edit, not after them |
| Correctness | Wire bytes to an eBGP peer are unchanged. The delete is invisible on the wire |
| Data flow | The edit-set path is the sole surviving producer of an eBGP AS_PATH, confirmed by reading it rather than by inference |
| Untyped reference | `AllocCeilings` lost its string key, and the retired `ze-alloc-check` (current: `./le verify-deps alloc`) was shown RED with it and green without it |
| Test deletion | Every deleted `Test` and `Benchmark` is named in the report with its own reason. No test covering surviving behavior went with them |
| Test survival | `TestForwardDoesNotRetranscodeASN2RewrittenWire` still exists with every assertion intact. Deleting it would have removed live coverage of `buildFwdBody` |
| Rule: `ai/rules/no-layering.md` | Nothing was deprecated, tagged out, or kept "for reference". The baseline benchmark went too |
| Rule: `ai/rules/stale-comments.md` | No surviving comment or doc paragraph names the removed cache. A grep for `EBGPWire` over `docs/` and `internal/` is empty |
| Rule: `ai/rules/testing.md` | `./le rfc index-update` was re-run and BOTH outputs committed, because a tagged test's line pin moved |
| Registration over hardcoding | N-A, removal only. The one registry touched loses an entry, which is the pattern working |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No symbol of the cache survives | A tree-wide grep over `*.go` for `EBGPWire`, `ebgpWireSlot`, `ebgpSlot`, `ebgpMu` and `errEbgpWireBufferExhaustedPoolAt` returns nothing |
| No prose names the cache | A grep for `EBGPWire` over `docs/` and `internal/` returns nothing |
| The two bench files are gone from git | `git log -1 --stat` shows both under deletions |
| The discriminator was exercised in both directions | Two pasted the retired `ze-alloc-check` (current: `./le verify-deps alloc`) outputs, one red with the key, one green without |
| Reactor concurrency is unchanged | Pasted `go test -race ./internal/component/bgp/reactor/...` output |
| The RFC index matches the moved pin | `./le rfc check` green, with `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/rfc7911.md` in the same commit |
| Every deleted test is accounted for | A named list in the implementation report, one reason per test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A. No input path is added or changed. The removed code parsed nothing the surviving edit-set path does not already parse |
| Resource exhaustion | The delete removes two potential pool-buffer holders per cache entry. It can only reduce pool pressure, never increase it. Confirm with `TestForwardPoolBalanceLocalASOverride` that the eviction path still returns every borrowed buffer |
| Error leakage | `errEbgpWireBufferExhaustedPoolAt` is removed with its only producer. No error string changes for any reachable path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A non-test `EBGPWire` call site turns up in step 1 | STOP. A-1 is broken. Report the caller and do not delete |
| Compilation error in the reactor package | Step 2 was split. Delete the fields and the four `recent_cache.go` reads in one edit |
| A pool-balance test reds | R-1. A live path was using the slot, so A-1 is wrong. Stop and report, do not adjust the test |
| `c_test_weakening` refuses an edit with exit 2 | Expected. Add the test's row to `test/weakened.md` FIRST, then re-run the edit |
| `commit_helper.py create` refuses the commit | R-4. Either a weakened test has no row, or the commit does not carry `test/weakened.md`. The message names which |
| `./le verify-deps alloc` reds after the delete | The string key survived in `AllocCeilings`. That is the gate working |
| `./le rfc check` reports a stale index | R-5. Run `./le rfc index-update` and commit both outputs |
| Any `.ci` test needs editing to pass | STOP. Editing a functional test to accommodate a delete proves the delete changed behavior. Report it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Do not weaken a test to reach green |

## Design Insights

- A registration keyed by a STRING is invisible to the compiler in both directions. Removing the registered thing leaves a live-looking entry, and only the gate that actually RUNS the registered thing can tell. This is the second time the alloc gate's fail-closed missing-benchmark branch has earned its keep.
- Absence is not evidence. For a deletion, the only useful test is one that goes RED when the delete is done carelessly. Here exactly one exists, so the spec names it as the discriminator and requires it to be exercised in both directions rather than merely observed green.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Delete `BenchmarkEBGPWireCacheHitParallelMutexBaseline` with the rest | Keep it as a re-runnable historical baseline | It reaches `ebgpMu` and both slots directly, so it cannot compile once they go. Keeping it would mean keeping the subsystem, which is the gradual migration `ai/rules/no-layering.md` forbids. The measurement is recorded in `docs/architecture/perf-round-3.md` |
| Rework `TestForwardDoesNotRetranscodeASN2RewrittenWire`, do not delete it | Delete it with the other `EBGPWire` callers | Its subject is `buildFwdBody`, which survives. It called `EBGPWire` only to build a fixture. Deleting it would remove live coverage under cover of a dead-code delete, which is the exact failure `ai/rules/testing.md` guards |
| Name `./le verify-deps alloc` as the discriminator, not `go test -race ./internal/perf` | Rely on the `internal/perf` unit tests | `TestAllocGateEnforce` skips without `ZE_ALLOC_GATE_BENCH`, and every other test in that package reads a hand-written fixture. The unit run stays green with the stale key. Only the gate that runs the real benchmarks discriminates |
| Rewrite `docs/architecture/perf-round-3.md` section 1 rather than delete it | Delete the section | The page is a record of a measurement campaign, and the optimization was real and correct. Deleting it would erase the history that explains why the cache existed. It is rewritten in the past tense instead |

## Known Limitations

- The before-and-after numbers in `docs/architecture/perf-round-3.md` stop being re-runnable, because the baseline comparator benchmark goes with the cache. That is accepted: the measured path is unreachable, so a re-run would measure nothing the daemon does.
- Two `.ci` comments, in `test/plugin/asn4-transcode-pooled-buffer.ci` and `test/plugin/bgp-rs-asn4-transcode.ci`, and one comment block in `internal/component/bgp/reactor/forward_readbuf_leak_test.go`, name a `getEBGPWire` function. That symbol is ALREADY gone: the AS-path fold removed it, and no Go file defines or calls it today. Those comments are stale for a different reason and are OUT of scope here, because this spec's delete does not create or worsen them. Do not fold the fix into this commit: `ai/rules/rule-precedence.md` says an unrelated fix in a closing commit costs the commit its single focus. Write one row in `plan/journal/` instead and move on.
- This spec removes the cache. It adds no gate that would have caught an unreachable exported method at commit time; `./le doc-wiring` already owns that surface.

## RFC Documentation (Scope: protocol)

AS_PATH prepending on eBGP egress is RFC 4271 Section 5.1.2. This spec removes an
unreachable SECOND implementation of it. The surviving implementation is the edit-set
path, and this spec must not change a single byte it produces.

The removed method carried an RFC 4271 Section 9.1.2 comment. That citation goes with
the code. No new RFC comment is added anywhere, because no enforcing code is added:
the obligation is met, and was already met, by the edit-set path.

**If the delete changes any AS_PATH byte, STOP.** That means the two implementations
disagreed, and the disagreement is a wire-visible conformance finding that outranks
this spec (`ai/rules/rfc-compliance.md`, and rung 2 of `ai/rules/rule-precedence.md`).
Report it rather than adjusting a test around it.

`rfc/requirements/rfc7911.md` proves `RFC7911-5-3` with two tagged tests in
`forward_body_test.go`, and pins each by line. The test this spec reworks sits between
them, so `./le rfc index-update` must be re-run and BOTH `ai/RFC-REQUIREMENTS.md` and
`rfc/requirements/rfc7911.md` committed with the change. No requirement gains or loses
a polarity, so no ratchet fires.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: three rows, each a concrete test name, none deferred
- [ ] The discriminator was exercised in BOTH directions: the retired `ze-alloc-check` (current: `./le verify-deps alloc`) pasted red with the `AllocCeilings` entry, and green without it
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] `go test -race ./internal/component/bgp/reactor/...` passes, output pasted
- [ ] `./le verify-deps alloc` passes
- [ ] `./le rfc check` passes, with both `./le rfc index-update` outputs committed
- [ ] `./le test-weakened check` passes
- [ ] Every deleted `Test` and `Benchmark` named in the report with its own reason
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard row 6 resolved: no live row without a destination

### TDD
- [ ] Tests written (N-A for new tests: this is a deletion. The obligation is the reworked `TestForwardDoesNotRetranscodeASN2RewrittenWire`, whose assertions are unchanged)
- [ ] Tests FAIL (paste output: `./le verify-deps alloc` red with the `AllocCeilings` entry still present)
- [ ] Tests PASS (paste output: `./le verify-deps alloc` green after the entry is removed)
- [ ] Boundary tests for all numeric inputs (N-A, no numeric input added)
- [ ] Functional `.ci` tests for end-to-end behavior: `bgp-rs-fastpath-ebgp-shared.ci`, unchanged and green
- [ ] Interop tests for protocol features (N-A: no wire-visible change, reason in the Interop Tests table)

### Handoff (Handoff = `verify`)
- [ ] Status set to `verification` in this file
- [ ] Commit prepared with `internal/le/commit/prepare.go create`, using `--remove` for the two deleted bench files
- [ ] `./le repository-tracked-build check` run after the commit script, because the commit carries Go
- [ ] Report delivered and the session STOPPED. Closure belongs to a later Opus 5 session, which reviews the commit
