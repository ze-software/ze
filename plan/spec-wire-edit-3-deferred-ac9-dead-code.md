# Spec: wire-edit-3-deferred-ac9-dead-code -- delete the EBGP wire cache the AS-path fold made unreachable

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The AS-path fold that landed in `ddf04953a` and `e2037e598` moved eBGP AS-path
prepending onto the edit-set path. The per-`ReceivedUpdate` EBGP wire cache it
replaced was left in the tree. `plan/learned/1319-wire-edit-3-aspath-fold.md` recorded
its AC-9 as **Partial** for this reason, and closed. The learned summary is
`plan/learned/1319-wire-edit-3-aspath-fold.md`.

The work is to delete the cache. It is dead code with no behavioral effect, and
deleting it is a separate reviewable change rather than a tail on the fold.

### What is dead, verified 2026-08-02

| Symbol | Where | Verdict |
|--------|-------|---------|
| `EBGPWire` | `internal/component/bgp/reactor/received_update.go` | Zero non-test callers. Every call site is in `received_update_test.go`, `received_update_bench_test.go` or `forward_body_test.go` |
| `ebgpWireSlot` | `internal/component/bgp/reactor/received_update.go` | Used only by the two slot fields and by `EBGPWire`'s store |
| `ebgpSlotASN4`, `ebgpSlotASN2` | `internal/component/bgp/reactor/received_update.go` | Fields on `ReceivedUpdate` |

**One nuance the deferral row omitted, and it changes the scope of the delete.**
The two slot fields DO have non-test readers, at four sites in
`internal/component/bgp/reactor/recent_cache.go`. Those reads exist only to
return the slot's pooled buffer when a cache entry is evicted. Because nothing
outside a test ever calls `EBGPWire`, and `EBGPWire` is the only writer, those
four reads always find nil in a running daemon. They are part of the same dead
subsystem and must be removed with it. A delete that removes the fields without
removing those reads will not compile.

A stale comment also survives: `internal/component/bgp/wireu/aspath_rewrite.go`
says "The EBGPWire cache amortizes this, but the fast path is free." Once the
cache is gone that sentence is false and must go
(`ai/rules/stale-comments.md`).

### What the delete must not silently discard

The tests that call `EBGPWire` are not scaffolding. `received_update_test.go`
covers concurrent generation of both ASN widths, cache identity on repeat calls,
and error paths. `received_update_bench_test.go` measures parallel cache-hit
reads on the RS fan-out path. `forward_body_test.go` uses it to build a rewritten
wire for a body comparison. Before any of them is deleted, decide for each
whether the property it asserts is now covered on the edit-set path, and say so
(`ai/rules/no-test-deletion.md`). A test deleted because its subject was deleted
is legitimate. A test deleted because it was in the way is not.

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 6.

## Required Reading

- [ ] `ai/rules/no-test-deletion.md` - deleting a test is legitimate only when the functionality it tests is removed
  → Constraint: each deleted test needs a stated reason, per test, not one blanket sentence.
- [ ] `ai/rules/no-layering.md` - when replacing X with Y, delete X
  → Decision: this spec is the deferred second half of that rule for the AS-path fold.
- [ ] `ai/rules/stale-comments.md` - a comment describing removed behavior is removed with it
  → Constraint: `wireu/aspath_rewrite.go` names the cache and must be corrected.
- [ ] `plan/learned/1319-wire-edit-3-aspath-fold.md` - the fold's closure record and the Partial AC-9
  → Constraint: AC-9's remaining half is exactly this delete.

**Key insights:**
- The delete is wider than the four named symbols: four reads in `recent_cache.go` come with them.
- Nothing user-visible changes. If anything does, the fold was incomplete and that is the finding.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/received_update.go` - defines `ebgpWireSlot`, the two slot fields, `EBGPWire` and `ebgpSlot`; `EBGPWire` prepends the local ASN and publishes the result into the slot
- [ ] `internal/component/bgp/reactor/recent_cache.go` - reads both slot fields at four sites to release pooled buffers on eviction
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - carries the stale comment naming the cache
- [ ] `internal/component/bgp/reactor/forward_body_test.go` - a test caller that builds a rewritten wire through `EBGPWire`

**Behavior to preserve:** every forwarded route keeps its current bytes. An eBGP peer must see the same AS_PATH before and after this delete, produced by the edit-set path. Pool accounting must stay balanced: the eviction path releases fewer buffers after the delete because there are fewer to release, not because a release was dropped.

**Behavior to change:** none. This is a removal of unreachable code.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A received UPDATE forwarded to an eBGP peer, arriving as a `ReceivedUpdate` in the reactor.

### Transformation Path
1. Today the fold builds the eBGP AS_PATH through the edit set, on the forward path.
2. `EBGPWire` offers a second, older route to the same result: prepend, cache in a slot, return a `WireUpdate`.
3. Nothing outside a test takes route 2.
4. On eviction, `recent_cache.go` inspects both slots to release their pooled buffers, and always finds nil.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ buffer pool | `ebgpWireSlot.handle` released on eviction | Yes, four sites read on 2026-08-02 |
| Reactor ↔ wireu | AS-path rewrite, now reached through the edit set | No, confirm at design time |

### Integration Points
- `internal/component/bgp/reactor/recent_cache.go` - the eviction path that must lose its slot reads.
- `internal/component/bgp/wireu/aspath_rewrite.go` - the comment to correct.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | fill during design: confirm the edit-set path is the only producer of an eBGP AS_PATH |
| No unintended coupling | No | fill during design |
| No duplicated functionality | No | this spec exists BECAUSE a duplicate survived; the check is that none remains |
| Zero-copy preserved where applicable | No | fill during design: the eviction release must stay balanced |
| Registration over hardcoding (`ai/rules/plugin-self-containment.md`) | N-A | removal only |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `EBGPWire` has zero non-test callers. | Tree-wide grep over `internal/`, `cmd/` and `pkg/` on 2026-08-02: every call site is a `_test.go` file. | The delete is not a delete. Stop and re-plan. | re-run the grep immediately before deleting | unvalidated |
| A-2 | The four `recent_cache.go` slot reads always find nil in production. | They can only be non-nil after an `EBGPWire` store, and A-1 says nothing calls it. | A live path populates the slots, so A-1 is wrong. | add a temporary counter, or reason it through with A-1 confirmed | unvalidated |
| A-3 | Every property the deleted tests assert is covered on the edit-set path. | Not yet checked. This is the assumption most likely to be wrong. | Port the uncovered property to an edit-set test BEFORE deleting its old test. | read each test and name its edit-set counterpart | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A pool-accounting test goes red because the eviction path lost a release. | `TestForwardPoolBalance*` or a readbuf-leak test fails. | The release is removed because the buffer no longer exists. If a balance test reds, a live path was using the slot and A-1 is wrong. |
| R-2 | The benchmark's coverage of parallel cache-hit reads disappears with no replacement, so a future regression on the edit-set path is unmeasured. | Nothing fails; the measurement is just gone. | Decide explicitly whether the edit-set path needs the equivalent benchmark. Record the decision either way. |
| R-3 | Deleting the tests is treated as bookkeeping and no per-test reason is recorded. | A commit removing several `Test*` functions with one blanket sentence. | `ai/rules/no-test-deletion.md`: one stated reason per deleted test. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | eBGP peers receive a wrong AS_PATH, or pooled buffers leak on eviction. Both are wire-visible or resource-visible, so this is not a cosmetic delete. |
| How is it reverted? | Single commit revert. |
| Who else touches this path? | Any session working the reactor forward path, the recent cache, or the pool accounting. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A received UPDATE forwarded to an eBGP peer | → | the edit-set AS-path fold, with no second route through a cache | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci`, which must stay green (no new feature) |
| A recent-cache entry is evicted | → | pooled buffers released, with no slot reads left behind | existing `TestForwardPoolBalanceLocalASOverride` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A grep for `EBGPWire`, `ebgpWireSlot`, `ebgpSlotASN4` and `ebgpSlotASN2` across the tree | No match outside git history |
| AC-2 | `make ze-verify` | Passes, with no pool-accounting or readbuf-leak regression |
| AC-3 | An eBGP peer receiving a forwarded route | Identical wire bytes before and after the delete |
| AC-4 | Each deleted test | Carries a stated reason naming either the removed functionality or its edit-set replacement |
| AC-5 | `internal/component/bgp/wireu/aspath_rewrite.go` | Carries no comment referring to a cache that no longer exists |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardPoolBalanceLocalASOverride` (existing) | `internal/component/bgp/reactor/` | AC-2, pool balance across the eviction path | |
| edit-set AS-path coverage (name at design time) | `internal/component/bgp/reactor/` | AC-3, and the properties A-3 must account for | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-ebgp-shared.ci` (existing) | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | an eBGP peer receives a forwarded route with the correct AS_PATH; no regression is the whole point | |

## Files to Modify
- `internal/component/bgp/reactor/received_update.go` - remove `EBGPWire`, `ebgpSlot`, `ebgpWireSlot` and both slot fields
- `internal/component/bgp/reactor/recent_cache.go` - remove the four slot reads in the eviction path
- `internal/component/bgp/wireu/aspath_rewrite.go` - correct the stale comment
- `internal/component/bgp/reactor/received_update_test.go` - remove or port the tests, with a reason each
- `internal/component/bgp/reactor/received_update_bench_test.go` - decide on the benchmark (R-2)
- `internal/component/bgp/reactor/forward_body_test.go` - rework the caller

## Implementation Steps

1. Re-run the grep for A-1. Do not proceed on a stale verdict.
2. Read each test that calls `EBGPWire` and name its edit-set counterpart, or state that the property is uncovered (A-3). Port anything uncovered FIRST.
3. Delete the production symbols and the four eviction reads together, so the tree compiles in one step.
4. Correct the stale comment.
5. Rework or remove the test callers, one stated reason each.
6. Run `make ze-race-reactor`: this touches reactor code that shares state across goroutines (`ai/rules/testing.md`).
7. Run `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | The four `recent_cache.go` reads went with the fields, not after them |
| Correctness | Wire bytes to an eBGP peer are unchanged; the delete is invisible on the wire |
| Data flow | The edit-set path is the sole producer of the eBGP AS_PATH, confirmed by reading it |
| Rule: `ai/rules/no-test-deletion.md` | One stated reason per deleted test |
| Rule: `ai/rules/stale-comments.md` | No surviving comment names the removed cache |
| Registration over hardcoding | N-A, removal only |

## RFC Documentation (Scope: protocol)

AS_PATH prepending on eBGP egress is RFC 4271 Section 5.1.2. This spec removes an
unreachable second implementation of it and must not change the behavior the
surviving implementation produces. If the delete changes any AS_PATH byte, stop:
the two implementations disagreed, and that disagreement is the finding.

## Known Limitations
- This removes the cache. It does not add a gate that would have caught an unreachable exported symbol at commit time; `make ze-verify-wiring-docs` already owns that surface (`ai/rules/wiring-completeness.md`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] `make ze-verify` passes
- [ ] `make ze-race-reactor` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
