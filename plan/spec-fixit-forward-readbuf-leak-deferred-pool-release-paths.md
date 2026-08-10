# Spec: fixit-forward-readbuf-leak-deferred-pool-release-paths

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | code |
| Depends | - |
| Phase | implementation complete, review not run |
| Deferral shard | `plan/deferrals/fixit-forward-readbuf-leak.md` |
| Updated | 2026-08-07 |

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
| 16 | Any changed source file referenced by existing doc source anchors? | No | Checked: no `source:` anchor names either changed file |
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
