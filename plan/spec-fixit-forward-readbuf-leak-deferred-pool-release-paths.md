# Spec: fixit-forward-readbuf-leak-deferred-pool-release-paths

| Field | Value |
|-------|-------|
| Status | done |
| Scope | code |
| Depends | - |
| Phase | closed: code landed in `23da6d8a0`, review gate clean, record in this commit |
| Deferral shard | `plan/deferrals/fixit-forward-readbuf-leak.md` (all rows terminal, removed in commit B) |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Provenance.** Two rows of `plan/deferrals/fixit-forward-readbuf-leak.md`, dated
2026-07-21. The verification of that spec's read-pool fix found them, and they were
explicitly out of its scope. Both were homed on prose. Their source spec closed and its
file is gone. This file is their home.

**The one subject.** A forward-rail pool resource is acquired and then not returned
on a path that is not the happy one. Neither is the read-pool leak that spec fixed:
different pool, and only reachable on failure or shutdown.

**Item 1: outgoing peer-pool MOD buffer, body-build failure path.** `forwardUpdateCore`
gets a mod buffer, then continues the destination loop when `buildFwdBody` fails, and
drops the item without returning the buffer. `reactorForwardRS` (`forward_rs.go`) has
the same shape. RE-CONFIRMED against the current tree on 2026-08-05 and again on
2026-08-07, after the fan-out dedup rewrote the surrounding code. FIXED 2026-08-07.

**Item 2: `DispatchOverflow` on a stopped pool.** Verified live on 2026-08-03.
FIXED and committed separately on 2026-08-05 (`027f6b0b3`), with
`forward_pool_stopped_release_test.go` and a recorded coverage limit. This spec
re-confirmed the fix is in the tree and needs no further work.

-> Constraint: item 2 is a shutdown-window leak whose blast radius is a process that
is already stopping. It is not a reason to touch the pool's hot path. Its fix is
bounded to the stopped branch, and it is.

-> Decision: item 1 is fixed by reusing `fwdPool.releaseItem`, the function that
already expresses this exact obligation, rather than by writing a second release.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/rules/performance.md` - the forward rail is the hot path this fix sits on
  → Decision: the caller owns the buffer, and the callee never allocates its own.
    `buildModifiedPayload` returns a pool index precisely so the caller can release it.
  → Constraint: no per-iteration `defer` on a per-destination loop. A closure per
    destination allocates on the rail this repository optimizes hardest.

### RFC Summaries (Scope: protocol)

Not applicable. No wire-visible behavior changes: the fix returns a buffer that was
already being dropped, and the route is suppressed either way.

**Key insights:** (minimal context to resume after compaction)
- Two rails carry the identical loop. A fix to one leaves the other leaking on
  whichever path the deployment selects.
- The acquire site and the release site are separated by exactly one `continue`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore`
  builds one `fwdItem` per destination. When policy needs a rebuild it takes an
  Outgoing Peer Pool buffer and stores the index and the pool pointer on the item.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRS` is the same
  loop for the route-server rail, with the body cache expressed as slots rather than
  a map.
- [ ] `internal/component/bgp/reactor/forward_build.go` - `buildModifiedPayload` and
  `buildWithdrawalPayload` are the two acquire sites. Both are already clean: every
  failure return inside them hands the buffer back before returning, and only a
  success carries a non-zero index out.
- [ ] `internal/component/bgp/reactor/forward_dedup.go` - `copyMaterialization` is the
  third acquire site, reached on a fan-out dedup hit. Also clean: it returns the
  buffer when the copy does not fit.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody` reports `!ok`
  on a split failure, a parse failure, or a transcode failure.
  `fwdUpdateForDestination` rejects an unregistered destination context.
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `fwdPool.releaseItem` is the
  single release point for every pool resource an item holds.
  `fwdPool.DispatchOverflow` already calls it on both stopped branches.

**The leak, enumerated as paths.** Between the acquire and the pool that returns the
buffer there is exactly ONE exit per rail, and it leaked:

| Exit from the destination loop, after the item is built | Returns the buffer? |
|--------------------------------------------------------|---------------------|
| `buildFwdBody` reports `!ok` | NO, before this fix |
| Body cache hit, then dispatch | Yes, via the pool |
| Body built, then dispatch | Yes, via the pool |

Every other `continue` in both loops sits BEFORE the item is built, so no buffer is
outstanding when it runs. The RS rail's second `continue` belongs to the inner
body-slot search, not the destination loop.

**Behavior to preserve:**
- The destination is still suppressed when its body cannot be built. The fix returns
  a buffer; it does not make a failing destination succeed.
- `dispatchedCount` and the `errNoEstablishedPeersToForwardTo` and
  `errAllDestinationsSuppressed` verdicts are unchanged.
- No allocation is added to the success path.

**Behavior to change:**
- The Outgoing Peer Pool buffer is returned when `buildFwdBody` fails, on both rails.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE reaches `reactorAPIAdapter.ForwardUpdate` (plugin rail) or
  `reactorForwardRS` (route-server fast path), carrying wire bytes and a message id.
- Format at entry: an UPDATE body as received, wrapped in a `wireu.WireUpdate` that
  carries the source encoding context.

### Transformation Path
1. Per destination, egress policy accumulates edits into a shared `ModAccumulator`.
2. When the accumulator is non-empty, `buildModifiedPayload` (or
   `buildWithdrawalPayload`, or `copyMaterialization` on a dedup hit) rebuilds the
   body into a buffer taken from that destination's Outgoing Peer Pool.
3. The buffer index and its pool are stored on the `fwdItem`.
4. `buildFwdBody` splits or transcodes the body for the destination's context.
5. The item is appended to the pending list and handed to `fwdPool`, whose
   `safeBatchHandle` returns the buffer after the write.

Stage 4 is where the item can be dropped, and it is the stage this spec fixes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Forward rail ↔ Outgoing Peer Pool | `peerPool.Get` / `peerPool.Return`, reached through `fwdPool.releaseItem` | Yes |
| Forward rail ↔ forward pool workers | `fwdItem` handed to `TryDispatch` / `DispatchOverflow` | Yes |

### Integration Points
- `fwdPool.releaseItem` - the existing single release point, now also called on the
  body-build failure exit of both rails.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix calls `fwdPool.releaseItem`, the same function the pool workers and `DispatchOverflow` call. No second release is written. |
| No unintended coupling (components stay isolated) | Yes | Both call sites already hold the `fwdPool` pointer they use. |
| No duplicated functionality (extends existing, does not recreate) | Yes | `releaseItem` is reused rather than re-implemented per rail. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Nothing is copied. The success path is untouched. |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No new command, view, family, or handler. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `buildModifiedPayload`, `buildWithdrawalPayload` and `copyMaterialization` never return a non-zero buffer index beside a failure | read all three producers in `forward_build.go` and `forward_dedup.go` | a second leak would exist inside the builders, not at the call site | reading each function's returns | confirmed |
| A-2 | The `buildFwdBody` failure exit is the ONLY exit between acquire and dispatch | read both destination loops end to end | a remaining exit would still leak | enumerated in the table above | confirmed |
| A-3 | `fwdPool` is non-nil wherever the new call runs | both rails already dereference it to fetch the Outgoing Peer Pool and to dispatch | a nil dereference on a rail that never rebuilds | full package test | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A double return, if a future edit also releases at the same exit | `peerPool.Return` reports an out-of-range or double return | `releaseItem` zeroes the index and the pool pointer after returning, so a second call is a no-op |
| R-2 | Cost added to the forward rail | benchmark regression | the call runs only on the failure exit, never on the success path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A double return would corrupt the destination's buffer accounting and could hand one buffer to two writes. `releaseItem` zeroing its fields is what prevents it. |
| How is it reverted? | Single commit revert. Two call sites and one test file. |
| Who else touches this path? | The fan-out dedup work rewrote both loops recently. The read-pool spec owns the sibling buffer on the same items. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `forwardUpdateCore` with a destination whose body build fails | → | `fwdPool.releaseItem` on the failure exit | `TestForwardUpdateCoreReturnsModBufOnBodyFailure` |
| `reactorForwardRS` with a client whose body build fails | → | `fwdPool.releaseItem` on the failure exit | `TestForwardRSReturnsModBufOnBodyFailure` |
| `DispatchOverflow` on a stopped pool | → | `fwdPool.releaseItem` on the stopped branch | `TestDispatchOverflowReleasesItemWhenStopped` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `forwardUpdateCore` rebuilds a destination's payload into an Outgoing Peer Pool buffer, then `buildFwdBody` reports `!ok` | The pool's free count is the same after the call as before it |
| AC-2 | `reactorForwardRS` reaches the same state | The pool's free count is the same after the call as before it |
| AC-3 | The same destination succeeds instead | The dispatched item carries a non-zero pool buffer index, proving AC-1 and AC-2 assert over a real loan |
| AC-4 | An item reaches `DispatchOverflow` after the pool stopped | The item's pool buffer is returned and `done()` still runs |

## End-to-End User Stories

Not applicable. This spec adds no user-facing operation. It reclaims a buffer on an
internal failure path; the operator-visible outcome (the route is suppressed for that
destination, and says so in the log) is unchanged.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardModBufTakenOnRebuild` | `internal/component/bgp/reactor/forward_modbuf_leak_test.go` | AC-3: the fixture reaches the acquire site, so the two leak tests are not vacuous | passing |
| `TestForwardUpdateCoreReturnsModBufOnBodyFailure` | `internal/component/bgp/reactor/forward_modbuf_leak_test.go` | AC-1 | passing |
| `TestForwardRSReturnsModBufOnBodyFailure` | `internal/component/bgp/reactor/forward_modbuf_leak_test.go` | AC-2 | passing |
| `TestDispatchOverflowReleasesItemWhenStopped` | `internal/component/bgp/reactor/forward_pool_stopped_release_test.go` | AC-4 | passing, landed 2026-08-05 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `peerPool` free count | 0-64 | 64 | N/A | N/A |

The assertion is an equality against the pre-call free count, not a threshold, so
there is no boundary to walk: any drift of one fails it.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| None | - | No user-facing behavior changes. The fix reclaims an internal pool buffer on a failure path that already suppressed the route; nothing an operator can observe through the CLI, config, or wire changes. Pool bookkeeping has no user entry point, which is the "pure internal logic" case in `ai/rules/testing.md`. | N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| None | - | - | No wire-visible change: the destination is suppressed with and without the fix, and the bytes any peer receives are identical. An interop test here would be vacuous by the first row of the vacuity table in `ai/rules/interop-and-goal-validation.md`. | N-A |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_forward.go` - release the item on the
  `buildFwdBody` failure exit of `forwardUpdateCore`
- `internal/component/bgp/reactor/forward_rs.go` - the same on `reactorForwardRS`

## Files to Create
- `internal/component/bgp/reactor/forward_modbuf_leak_test.go` - the three unit tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No CLI surface |
| CLI grammar (keyword before value) | No | No CLI surface |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | No | No new RPC or API |
| Pipe completeness | No | No command output |
| Env var registration | No | No new env var |
| Doctor check for runtime dependencies | No | No new file, socket, service, module, port, or certificate |
| Prometheus counters/metrics | No | The pool's existing stats already expose in-use counts |
| BGP family surface (new SAFI / capability / attribute) | No | No family, capability, or attribute touched |

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
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | Three tests added, no new runner or format |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | The release obligation is unchanged; one exit now honors it |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and none is stale | Anchors DO name both changed files (`docs/architecture/core-design.md`,717-718; `docs/architecture/update-building.md`,414; `docs/guide/bgp-policy.md`,102; `docs/comparison.md`,338; `docs/architecture/bgp/structural-forwarding.md`,9). Each claim is about the accumulator hoist, the body cache, or the RS rail existing. None describes the buffer's release, so none is falsified by adding one. The one anchor that DOES name `releaseItem` (`docs/architecture/forward-congestion-pool.md`,465) states the overflow handle returns on processing complete, which this change makes more true, not less. No edit needed. The earlier "no anchor names either changed file" in this row was wrong and is corrected here |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No operator-facing surface in this area |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach the acquire site from the rail's own entry point
   - Tests: `TestForwardModBufTakenOnRebuild`
   - Files: `forward_modbuf_leak_test.go`
   - Verify: an IBGP destination with next-hop-self and a registered Outgoing Peer
     Pool carries a non-zero buffer index on its dispatched item. Without this the
     two leak tests below would pass over a pool that never lent anything.
2. **Phase: The failure exit** -- drive `buildFwdBody` to `!ok` with a buffer outstanding
   - Tests: `TestForwardUpdateCoreReturnsModBufOnBodyFailure`, `TestForwardRSReturnsModBufOnBodyFailure`
   - Files: `reactor_api_forward.go`, `forward_rs.go`
   - Verify: both tests fail with the fix absent and pass with it present.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both rails carry the release, not one |
| Feature completeness | The three acquire sites all feed the one exit that is now guarded |
| Correctness | The release runs before the `continue`, and `releaseItem` zeroes the item so a later release is a no-op |
| Naming | The call is `releaseItem`, the existing name for this obligation, not a new spelling |
| Data flow | Nothing is added to the success path, and the suppression verdict is unchanged |
| Rule: `ai/rules/performance.md` | No `defer` and no closure inside the per-destination loop |
| Rule: `ai/rules/testing.md` | Each test was seen RED with the fix reverted, and the control proves the loan is real |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Release on the general rail | `make ze-test-pkg PKG=./internal/component/bgp/reactor RUN=TestForwardUpdateCoreReturnsModBufOnBodyFailure` |
| Release on the route-server rail | `make ze-test-pkg PKG=./internal/component/bgp/reactor RUN=TestForwardRSReturnsModBufOnBodyFailure` |
| The tests are not vacuous | `make ze-test-pkg PKG=./internal/component/bgp/reactor RUN=TestForwardModBufTakenOnRebuild` |
| No race introduced | `make ze-race-reactor` |
| Lint clean | `golangci-lint run ./internal/component/bgp/reactor/...` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | None added. The fix runs after every validation the rail already performs |
| Resource exhaustion | This IS the resource-exhaustion fix. A peer that reliably triggers a body-build failure could previously drain a destination's 64 buffers, after which every rebuild for that destination fell back to `sync.Pool` and allocated |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **A leak claim is about paths, so the fix is only as good as the enumeration.**
  Three functions acquire this buffer and all three were already clean. The single
  defect was at the call site, on the one exit between acquire and hand-off. Reading
  the acquire sites first is what made the fix two lines rather than a rewrite.
- **A pool-balance test is vacuous by default.** A pool that never lent a buffer
  also ends at its baseline, so "back to baseline" proves nothing until a loan is
  proven. The control test exists for that and nothing else.
- **The two rails are copies, and a fix to one is half a fix.** Both loops were
  written to stay behaviorally identical, and their own comments say so.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Call `fwdPool.releaseItem` at the failure exit | Write an inline `peerPoolRef.Return(peerBufIdx)` at each site | `releaseItem` already owns this obligation, covers the overflow handle too, and zeroes the item so a double return cannot happen. A second spelling would drift from it. |
| No `defer` | A deferred release inside the loop body | The acquire and release span one loop ITERATION, not the function, so a `defer` would fire too late. Expressing it per iteration needs a closure, which allocates once per destination on the hottest loop in the repository. |
| A separate test file | Add cases to `forward_readbuf_leak_test.go` | That file is about the shared READ pool. This is the Outgoing Peer Pool: different pool, different lifecycle. Item 2 got its own file for the same reason. |
| Drive the failure with an unregistered destination context | Malformed NLRI, or an oversized payload forcing a split failure | It is a single knob that cannot be satisfied by accident, it needs no malformed wire bytes, and it exercises the real "encoding update for forward" failure the rail logs. |

## Known Limitations

- The general rail's fix is proven through `forwardUpdateCore` only. `ForwardUpdate`,
  the plugin-facing wrapper, reaches the same loop, and no test drives the leak
  through that outer entry point.
- `buildFwdBody` has three failure returns (split, parse, transcode). The tests drive
  the transcode one. The `continue` they prove is shared by all three, so the release
  is reached identically, but only one is exercised.

## RFC Documentation (Scope: protocol)

Not applicable. No protocol behavior is implemented or changed.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Problem record written as a row in `plan/journal/<class>.md` (commit `2cff2050a`
      retired `plan/learned/NNN-<name>.md`; the row is the closure artifact
      `spec_closure_stem` reads)
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `internal/component/bgp/reactor/reactor_api_forward.go`,761: `a.r.fwdPool.releaseItem(&item)`
  on the `buildFwdBody` failure exit of `forwardUpdateCore`, before the `continue`.
- `internal/component/bgp/reactor/forward_rs.go`,461: `r.fwdPool.releaseItem(&item)`
  on the same exit of `reactorForwardRS`.
- `internal/component/bgp/reactor/forward_modbuf_leak_test.go`: three tests, one
  control and one per rail.
- Item 2 needed no code. `fwdPool.DispatchOverflow` already calls `releaseItem` on
  both stopped branches (`forward_pool.go`,648 and,670), landed by `027f6b0b3` with
  `forward_pool_stopped_release_test.go`. This spec re-read the function and
  confirmed it.
- The code landed in commit `23da6d8a0`. This closure carries the record only.

### Bugs Found/Fixed
- The Outgoing Peer Pool buffer was dropped, not returned, when `buildFwdBody`
  reported `!ok`, on both forwarding rails. Covered by
  `TestForwardUpdateCoreReturnsModBufOnBodyFailure` and
  `TestForwardRSReturnsModBufOnBodyFailure`.

### Documentation Updates
- None. Item 16 of the Documentation Update Checklist was re-checked against the
  tree and corrected: anchors do name both changed files, and no anchored claim
  describes the buffer's release, so none is falsified. `make ze-doc-test` not run:
  no doc file changed.

### Deviations from Plan
- The spec's Closure checklist named `plan/learned/NNN-<name>.md`. Commit
  `2cff2050a` retired that corpus in favour of a row in `plan/journal/<class>.md`.
  The checklist text is corrected in this commit and the row is the artifact.
- The deferral shard's third row is rehomed rather than left pointing at this spec
  (see Deferrals Resolved).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Documentation checklist row 16 asserted that no doc `source:` anchor names either changed file | Eight anchors name `reactor_api_forward.go` or `forward_rs.go` | grep over `docs/` and `ai/` at closure | row 16 corrected with the real anchor list and the reason each claim is unaffected |
| approach | The Closure checklist directed the lesson to `plan/learned/NNN-<name>.md` | That corpus was retired by `2cff2050a`; the artifact is a `plan/journal/<class>.md` row | the closure gate reads the journal row for the spec stem | checklist text corrected, journal rows written |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Item 1: return the mod buffer on the body-build failure exit of the general rail | Done | `reactor_api_forward.go`,761 | `releaseItem` before the `continue` |
| Item 1: the same on the route-server rail | Done | `forward_rs.go`,461 | identical shape |
| Item 2: `DispatchOverflow` releases the item on a stopped pool | Done | `forward_pool.go`,648 and,670 | landed by `027f6b0b3`, re-read and confirmed here |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForwardUpdateCoreReturnsModBufOnBodyFailure` | free count equal to the pre-call count |
| AC-2 | Done | `TestForwardRSReturnsModBufOnBodyFailure` | same assertion on the RS rail |
| AC-3 | Done | `TestForwardModBufTakenOnRebuild` | the dispatched item carries a non-zero `peerBufIdx`, so AC-1 and AC-2 assert over a real loan |
| AC-4 | Done | `TestDispatchOverflowReleasesItemWhenStopped` | first stopped branch; the second is recorded as a coverage limit in the test file |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestForwardModBufTakenOnRebuild` | Done | `forward_modbuf_leak_test.go`,165 | control |
| `TestForwardUpdateCoreReturnsModBufOnBodyFailure` | Done | `forward_modbuf_leak_test.go`,200 | |
| `TestForwardRSReturnsModBufOnBodyFailure` | Done | `forward_modbuf_leak_test.go`,222 | |
| `TestDispatchOverflowReleasesItemWhenStopped` | Done | `forward_pool_stopped_release_test.go`,27 | landed 2026-08-05 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor_api_forward.go` | Done | one call added |
| `internal/component/bgp/reactor/forward_rs.go` | Done | one call added |
| `internal/component/bgp/reactor/forward_modbuf_leak_test.go` | Done | created |

### Audit Summary
- **Total items:** 14
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The Outgoing Peer Pool buffer is returned when the body build fails, on BOTH rails | unit, mutation-discriminated | `make ze-test-pkg PKG=./internal/component/bgp/reactor RUN='TestForwardModBufTakenOnRebuild\|TestForwardUpdateCoreReturnsModBufOnBodyFailure\|TestForwardRSReturnsModBufOnBodyFailure\|TestDispatchOverflowReleasesItemWhenStopped'` -> `ok github.com/ze-software/ze/internal/component/bgp/reactor 1.574s`. The independent review discriminated each site by mutation: reverting `reactor_api_forward.go`,761 reds the general test while the RS test stays green, and reverting `forward_rs.go`,461 reds the RS test while the general one stays green |
| The pool-balance assertions are not vacuous | unit control | `TestForwardModBufTakenOnRebuild` asserts a non-zero `peerBufIdx` on the dispatched item, so the pool provably lent a buffer before the balance is asserted |
| An item reaching `DispatchOverflow` after the pool stopped returns its buffer | unit, mutation-discriminated | `TestDispatchOverflowReleasesItemWhenStopped`; reverting the first stopped branch in `forward_pool.go` reds it |
| No regression under the race detector | package stress | `make ze-race-reactor` (`-race -count=20`): all four tests of this spec green on every repetition. Two other tests are red and neither is this spec's (see Pre-Commit Verification, and the `--unverified` attribution on the commit) |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 2026-07-21: Outgoing peer-pool MOD buffer leak on the body-build failure path | done | Fixed by this spec. `reactor_api_forward.go`,761 and `forward_rs.go`,461, proven by the two rail tests |
| 2026-07-21: pool-stopped `DispatchOverflow` does not `releaseItem` | done | Fixed earlier by `027f6b0b3`; re-read and confirmed at `forward_pool.go`,648 and,670 |
| 2026-08-07: `TestForwardAdoptedHandleHeldUntilLastWrite` red under `make ze-race-reactor` | done, rehomed | This spec was its only destination and closure deletes it. The knowledge now lives in `plan/journal/registry-contamination.md`, which carries the test name, the observation, the ruled-out hypothesis and the next step. Re-observed at closure, together with a SECOND test in the same package failing on the same empty read pool |

All three rows are terminal, so `plan/deferrals/fixit-forward-readbuf-leak.md`
holds no live work. It is NOT removed, and the reason is a gate boundary rather
than a choice. `deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`)
reads the shard at HEAD, by design, and at the moment commit B is PREPARED that
HEAD still carries the live row. Commit A resolves it, and commit A has not run
yet, so the gate refuses commit B's removal. The shard survives with every row
terminal, which the gate's own message calls the correct end state. The gate's
blind spot is recorded in `plan/journal/gate-excludes-part-of-its-population.md`.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-forward-readbuf-leak-deferred-pool-release-paths-640fa955-f03a-45e8-a58f-4b367f5859e6.md` (5 files pinned) |
| `review_gate.py check` | `review_gate: OK (3 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 1 |
| Reviewer lenses used | leak-path enumeration (every exit between acquire and hand-off, both rails), mutation discrimination per call site, duplication of the release obligation |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE. The review found no product defect and required no code change | - | - |

The review recorded one NOTE, kept out of the code deliberately: `releaseItem` has
two open-coded copies, the supersede path (`forward_pool.go`,745-751) and the RS
direct-write path (`forward_rs.go`,506-507). Neither leaks today. Folding them in
needs its own change and would widen this closure, so it is recorded as a journal
row (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`) and the code is left
alone.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_modbuf_leak_test.go` | Yes | `ls -la` reports 9849 bytes; `git ls-files` reports it tracked, added by `23da6d8a0` |
| `internal/component/bgp/reactor/forward_pool_stopped_release_test.go` | Yes | added by `027f6b0b3` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the general rail releases on the failure exit | `grep -n releaseItem reactor_api_forward.go` -> `761: a.r.fwdPool.releaseItem(&item)`; `TestForwardUpdateCoreReturnsModBufOnBodyFailure` green |
| AC-2 | the RS rail releases on the same exit | `grep -n releaseItem forward_rs.go` -> `461: r.fwdPool.releaseItem(&item)`; `TestForwardRSReturnsModBufOnBodyFailure` green |
| AC-3 | the loan is real, so AC-1 and AC-2 are not vacuous | `TestForwardModBufTakenOnRebuild` green in the same run |
| AC-4 | the stopped pool releases the item | `grep -n releaseItem forward_pool.go` -> `648` and `670`; `TestDispatchOverflowReleasesItemWhenStopped` green |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `forwardUpdateCore` with a destination whose body build fails | none: a Go unit test, `forward_modbuf_leak_test.go`,200 | Yes. The test drives the rail's own entry point with an unregistered destination context, which is what makes `buildFwdBody` report `!ok` |
| `reactorForwardRS` with a client whose body build fails | `forward_modbuf_leak_test.go`,222 | Yes, same knob on the RS rail |
| `DispatchOverflow` on a stopped pool | `forward_pool_stopped_release_test.go`,27 | Yes. Read at closure: it stops the pool, dispatches an item holding a buffer, then asserts the pool's free count |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | all three acquire sites return the buffer on every failure return; only a success carries a non-zero index out (`forward_build.go`, `forward_dedup.go`) |
| A-2 | confirmed | the exit enumeration in Current Behavior was re-walked by the independent reviewer on both rails |
| A-3 | confirmed | both rails dereference `fwdPool` before the new call to fetch the Outgoing Peer Pool; the package tests pass under `-race -count=20` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user-facing, config, CLI, API, wire, SDK, RFC, metrics or inventory surface changed | the diff is two `releaseItem` calls and one test file; no exported symbol, no command, no leaf, no wire byte | Yes |
| No anchored doc claim is stale | grep over `docs/` and `ai/` for `reactor_api_forward.go`, `forward_rs.go`, `forward_pool.go`: eight anchors name a changed file, each about the accumulator hoist, the body cache or the RS rail existing. The `releaseItem` anchor (`docs/architecture/forward-congestion-pool.md`,465) says the handle returns on processing complete, which the fix strengthens | Yes |

## Core Insight

A resource-balance assertion is vacuous by default. A pool that never lent a buffer
ends at its baseline exactly like a pool that lent one and got it back, so
"the free count is unchanged" proves nothing until the loan is proven separately.
The control test is not politeness; without it the two leak tests assert over an
untouched pool.
