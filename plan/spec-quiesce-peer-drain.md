# Spec: quiesce-peer-drain

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/plugin/server/quiesce.go` - the QuiescerRegistry (Layer 1, committed)
4. `internal/component/bgp/reactor/peer.go:844` - `ShouldQueue()` (the drain condition)
5. `internal/component/bgp/reactor/peer_initial_sync.go:404` - where `sendingInitialRoutes` clears
6. `test/scripts/ze_api.py` - `wait_for_ack` (the sleep to remove)

## Task

Finish AC-6 of `test-sync-quiesce`: make `ze_api.wait_for_ack` sleepless by
covering the one drain path the forward-pool quiescer misses.

**Corrected diagnosis (from investigation):** `wait_for_ack`'s comment blamed
"ze-peer sending its own cmd=api", but ze-peer never executes `cmd=api` (those
`.ci` lines are documentation-only, ignored at `expect.go:93-94`). The real race
is **inside the ze reactor**: a plugin `send()` while a peer is not yet
Established, still in initial route sync (`sendingInitialRoutes != 0`), or has a
non-empty opQueue is diverted into the peer's **opQueue**
(`peer.go:844-850` `ShouldQueue`) instead of the forward pool. `ze-bgp:peer-flush`
/ the `bgp-forward-pool` quiescer drains ONLY the forward pool
(`reactor_api.go:892` `fwdPool.Barrier`), never the opQueue. The opQueue + EOR
drain later in `sendInitialRoutes` (`peer_initial_sync.go:348-405`, flag cleared
at `:404`). The `time.sleep(0.2*count)` masked that initial-sync drain latency.

**Fix:** register a **second** reactor quiescer, `bgp-peer-sync`, that blocks
until every established peer has finished initial sync (`!ShouldQueue()`), next
to the existing `bgp-forward-pool` quiescer. They drain independent queues
(opQueue drains DIRECT to the session; the forward pool drains post-establishment
routes), so `quiesceAll` running them concurrently is correct. Then
`wait_for_ack` drops its `time.sleep` and calls `quiesce()`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` § Quiesce Barrier - the Layer 1 barrier this extends
  → Constraint: subsystems register a Quiescer; the handler discovers them (no per-subsystem switch).

### RFC Summaries
N/A — no wire-format change (RFC 4724 EOR behavior is unchanged; we only wait for it).

**Key insights:**
- opQueue routes go DIRECT to the session (`peer_initial_sync.go:320-359`), NOT through the forward pool, so a second, independent barrier is required.
- No completion signal exists for `sendingInitialRoutes` (plain atomic, cleared at 4 sites: `:35,:62,:304,:404`), so the barrier polls the cheap `ShouldQueue()` condition, bounded by ctx (not a fixed sleep).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go:844` - `ShouldQueue()` = `State()!=Established || sendingInitialRoutes!=0 || len(opQueue)>0`.
  → Constraint: the drain condition per established peer is exactly `!ShouldQueue()`.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go:348-405` - drains opQueue direct to session, then `sendingInitialRoutes.Store(0)` (`:404`).
- [ ] `internal/component/bgp/reactor/reactor_api.go:891` - `FlushForwardPool` → `fwdPool.Barrier(ctx)`; `Reactor.Peers()` (`reactor.go:753`).
- [ ] `internal/component/plugin/server/quiesce.go:116-127` - `registerReactorQuiescer` registers `bgp-forward-pool`.
- [ ] `internal/component/plugin/types_bgp.go:379` - `ReactorLifecycle` interface (has `FlushForwardPool`); `coordinator.go:250` Coordinator delegate.
- [ ] `test/scripts/ze_api.py` `wait_for_ack` - peer-flush + `time.sleep(0.2*count)`.

**Behavior to preserve:**
- `ze-bgp:peer-flush`, `ze-system:quiesce` (forward-pool drain) keep working.
- Initial-sync ordering / RFC 4724 EOR unchanged — we only observe it.
- `wait_for_ack`'s external contract (returns True) preserved; only the sleep goes.

**Behavior to change:**
- Add `DrainPeerSync(ctx)` to the reactor + `ReactorLifecycle`; register `bgp-peer-sync` quiescer.
- `wait_for_ack` → `quiesce()` (no `time.sleep`).

## Data Flow (MANDATORY)

### Entry Point
- Test: `ze_api.quiesce()` → `ze-system:quiesce`.

### Transformation Path
1. `handleQuiesce` → `quiesceAll` runs all registered quiescers concurrently.
2. `bgp-forward-pool` → `FlushForwardPool` (forward-pool barrier).
3. `bgp-peer-sync` → `DrainPeerSync`: poll until every established peer has `!ShouldQueue()`, bounded by ctx.
4. Both drained → reply StatusDone.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → reactor | `DrainPeerSync(ctx)` polls `Peer.ShouldQueue()` | [ ] |
| Test → engine | `quiesce()` RPC | [ ] |

### Integration Points
- `internal/component/bgp/reactor` (new `DrainPeerSync`), `plugin.ReactorLifecycle`, `Coordinator`.
- `quiesce.go` `registerReactorQuiescer` (add 2nd quiescer).
- `ze_api.py` `wait_for_ack`.

### Architectural Verification
- [ ] No bypassed layers (barrier via the registry)
- [ ] No unintended coupling (quiescer is a reactor method value)
- [ ] No duplicated functionality (reuses ShouldQueue; independent of forward-pool barrier)
- [ ] Registration over hardcoding (2nd Quiescer registered)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `!PendingSync()` for all peers ⟹ opQueue routes are on the wire + EOR sent | `peer_initial_sync.go:401-404` drains opQueue then clears flag | Barrier returns too early | nexthop-278 sleepless 25/25 + full suite green | VALIDATED |
| A-2 | opQueue routes go direct to session (not forward pool), so the two quiescers are independent (concurrent-safe) | `peer_initial_sync.go:320-359` sendFn=session write | Need ordering between quiescers | code re-read + unit tests -race | VALIDATED |
| A-3 | A peer with an EMPTY queue in a down state is skipped; a not-yet-established peer WITH queued routes IS waited on | `PendingSync` gates on opQueue/flag, NOT state | Barrier hangs on a dead peer, or returns early on an establishing one | `TestPeersSyncedSkipsIdleNonEstablished` + nexthop (queued-before-establish) passes | VALIDATED (A-3 reworded: original "skip all non-established" was wrong) |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Poll busy-spins | high CPU in tests | 1ms ticker + ctx deadline; condition is a cheap RLock check |
| R-2 | A peer stuck mid-sync hangs the barrier | quiesce times out | per-quiescer ctx timeout (10s) already bounds it; names `bgp-peer-sync` |
| R-3 | Removing the sleep re-breaks a route test other than nexthop | plugin suite red | re-run the full route-forwarding subset after the change |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request quiesce` with an in-sync peer | → | `DrainPeerSync` waits for `!ShouldQueue()` | `TestDrainPeerSyncWaitsForInitialSync` |
| `.ci`: same-prefix re-announce + `quiesce()` (no sleep) | → | opQueue drained in order | `test/plugin/nexthop.ci` passes sleepless |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An established peer is mid initial-sync (`ShouldQueue()` true) | `DrainPeerSync` blocks until `!ShouldQueue()`, then returns nil |
| AC-2 | All peers already drained | `DrainPeerSync` returns nil immediately |
| AC-3 | A peer is not Established (down/connecting) | `DrainPeerSync` skips it (does not hang) |
| AC-4 | `bgp-peer-sync` registered | `request quiesce` drains BOTH forward pool and initial-sync |
| AC-5 | `wait_for_ack` migrated | contains no `time.sleep`; calls `quiesce()` |
| AC-6 | `nexthop.ci` (same-prefix ordered re-announce) with sleepless `wait_for_ack` | passes, repeatably (the R-3 regression is closed) |

## End-to-End User Stories

| # | User does | Path | Test |
|---|-----------|------|------|
| 1 | Test announces same prefix 3× then `wait_for_ack` | send → opQueue (in-sync) → DrainPeerSync waits → on-wire in order | `test/plugin/nexthop.ci` sleepless |
| 2 | `ze_api.quiesce()` after a send during establishment | quiesce → DrainPeerSync + FlushForwardPool → both drained | `TestDrainPeerSyncWaitsForInitialSync` + functional |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDrainPeerSyncWaitsForInitialSync` | `internal/component/bgp/reactor/quiesce_drain_test.go` | AC-1: blocks then returns when flag clears | |
| `TestDrainPeerSyncImmediateWhenDrained` | same | AC-2: returns nil at once | |
| `TestDrainPeerSyncSkipsNonEstablished` | same | AC-3: down peer does not hang | |
| `TestDrainPeerSyncRespectsContext` | same | R-2: ctx deadline bounds it | |
| `TestPeerSyncQuiescerRegistered` | `internal/component/plugin/server/quiesce_test.go` | AC-4: second quiescer present | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `nexthop` sleepless | `test/plugin/nexthop.ci` (via `wait_for_ack`) | same-prefix ordered re-announce passes without the sleep | |
| route-forwarding subset | `ze-test bgp plugin --pattern <route tests>` | no regression from the sleep removal | |

### Interop Tests
N/A — no wire change.

## Files to Modify
- `internal/component/bgp/reactor/reactor_api.go` - add `DrainPeerSync` (poll `!ShouldQueue()` over `Peers()`).
- `internal/component/plugin/types_bgp.go` - add `DrainPeerSync` to `ReactorLifecycle`.
- `internal/component/plugin/coordinator.go` - Coordinator delegate for `DrainPeerSync`.
- `internal/component/plugin/server/quiesce.go` - register `bgp-peer-sync` quiescer.
- `test/scripts/ze_api.py` - `wait_for_ack` → `quiesce()`, drop the sleep; update its NOTE.
- `plan/deferrals.md` - resolve the AC-6 row (this spec is its destination).
- `docs/architecture/api/commands.md` - note the second quiescer in the Quiesce Barrier section.

### Integration Checklist
| Point | Needed? | File |
|-------|---------|------|
| New interface method on ReactorLifecycle | Yes | types_bgp.go + coordinator.go + mock reactors (test) |
| Functional test | Yes | nexthop.ci sleepless proves it |
| Prometheus | No | |

### Documentation Update Checklist
| # | Question | Applies? | File |
|---|----------|----------|------|
| 4 | API/RPC changed? | Partial | commands.md Quiesce Barrier (add bgp-peer-sync) |
| 10 | Test infra changed? | Yes | quiesce now drains initial-sync; wait_for_ack sleepless |
| (others) | | No | grep-verified at completion |

## Files to Create
- `internal/component/bgp/reactor/quiesce_drain_test.go` - unit tests for DrainPeerSync.

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| 1 Read | this file |
| 2 Audit | Files to Modify — confirm mock reactors need the new method |
| 3 Wiring | Wiring Test |
| 4 Implement (TDD) | phases below |
| 5 Verify | `make ze-verify-changed` + route suite |
| 13 Review | Review Gate |
| 14 Close | two-commit closure |

### Implementation Phases
1. **Phase: DrainPeerSync (TDD)** — add the reactor barrier + interface + Coordinator delegate + mock-reactor stubs; unit tests.
   - Tests: `TestDrainPeerSync*`.
2. **Phase: register quiescer** — add `bgp-peer-sync` in `registerReactorQuiescer`; `TestPeerSyncQuiescerRegistered`.
3. **Phase: wait_for_ack** — drop the sleep, call `quiesce()`; update NOTE + deferrals row.
4. **Functional** — re-run `nexthop.ci` (sleepless, must pass, repeated) + route-forwarding subset (no regression).
5. **Verify + close.**

### Critical Review Checklist
| Check | For this spec |
|-------|---------------|
| Correctness | barrier returns only when opQueue drained; skips non-established; ctx-bounded |
| Concurrency | poll uses RLock; two quiescers independent (opQueue direct vs forward pool) |
| No busy-spin | 1ms ticker, ctx deadline |
| No-layering | wait_for_ack sleep fully removed; deferrals row resolved |
| Registration | 2nd Quiescer via registry, not a handler switch |

### Deliverables Checklist
| Deliverable | Verification |
|-------------|--------------|
| `DrainPeerSync` on reactor + interface | grep + unit tests pass |
| `bgp-peer-sync` quiescer | `TestPeerSyncQuiescerRegistered` |
| `wait_for_ack` sleepless | `grep time.sleep test/scripts/ze_api.py` shows none in wait_for_ack |
| nexthop sleepless | `ze-test bgp plugin 278` PASS (repeated) |

### Security Review Checklist
| Check | Look for |
|-------|----------|
| Resource | poll bounded by ctx (per-quiescer 10s); cheap condition |
| DoS | quiesce already authorized + bounded |

### Failure Routing
| Failure | Route |
|---------|-------|
| nexthop still flaky sleepless | A-1 broken → barrier returns too early; re-check opQueue drain |
| barrier hangs | A-3 broken → non-established peer not skipped |

## Mistake Log
### Wrong Assumptions
| Assumed | True | How | Impact |
|---------|------|-----|--------|
| (Layer 1) wait_for_ack sleep covers ze-peer cmd=api interleaving | ze-peer never runs cmd=api; the race is ze's opQueue (initial-sync) | this investigation | corrected the fix from "peer-side" to a ze-side initial-sync quiescer |
| `wait_for_ack`/`quiesce()` invoke the barrier via `_call_engine("ze-bgp:peer-flush"/"ze-system:quiesce")` | those wire methods are `unknown method` for a plugin -- `dispatchPluginRPC` (`dispatch.go:79-94`) routes ONLY `ze-plugin-engine:*` engineOps + codec RPCs; api-yang RPCs dispatch via command path. Reachable call: `api.dispatch("request quiesce")` | probe plugin that `runtime_fail`s with the literal RPC results | THE root cause: peer-flush/quiesce barriers were silent no-ops (`except RuntimeError: pass`); the sleep did all the work, so every sleep-removal reintroduced flakiness (the yo-yo) and produced the false "AC-6 unachievable" conclusion |
### Failed Approaches
| Approach | Why | Replacement |
|----------|-----|-------------|
| `wait_for_ack`/`quiesce()` call `_call_engine("ze-system:quiesce")` | not a plugin-callable engine op -> `unknown method`, swallowed silently | `api.dispatch("request quiesce")` (dispatch-command path) |
### Escalation Candidates
| Mistake | Freq | Rule | Action |
|---------|------|------|--------|
| Assuming an api-yang RPC is plugin-callable via `_call_engine(wire-method)` | 2+ (peer-flush and quiesce) | plugin RPC vs command dispatch | recorded in `plan/learned/1118`; consider a plugin-protocol doc note |

## Design Insights
- The opQueue (initial-sync ordering buffer) and the forward pool are DISTINCT async paths; a complete "routes on the wire" barrier must drain both.

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|-------------|-----------|
| Second independent quiescer (concurrent) | One quiescer doing peer-sync then forward-pool sequentially | The two queues are independent (opQueue → direct session; forward pool → post-establishment), so concurrent draining is correct and simpler |
| Poll `ShouldQueue()` | Add a completion channel signaled at flag-clear | `sendingInitialRoutes` clears at 4 sites; a channel is fiddly and error-prone. A 1ms ctx-bounded condition-poll is simple, returns as soon as drained, and is not a fixed sleep |

## Known Limitations
- The barrier waits only for peers already/becoming Established; a peer that never establishes is skipped (correct — it has no wire).
- Poll granularity ~1ms; a test asserting sub-ms timing would not be served (none do).

## Implementation Summary
### What Was Implemented
- `DrainPeerSync(ctx)` reactor barrier (`reactor_api.go:918`) polling `!PendingSync()`
  (`peer.go:859`) over `Peers()`; `peersSynced`/`waitForCondition` helpers.
- `PendingSync()` = `sendingInitialRoutes != 0 || len(opQueue) > 0` (no state gate,
  unlike `ShouldQueue`) so a not-yet-established peer with queued routes IS waited on.
- `DrainPeerSync` added to `plugin.ReactorLifecycle` (`types_bgp.go`), `Coordinator`
  delegate (`coordinator.go`), and 3 `mockReactor` test stubs.
- `bgp-peer-sync` quiescer registered next to `bgp-forward-pool`
  (`quiesce.go:131`); `request quiesce` now drains BOTH concurrently.
- `ze_api.py`: `quiesce()` and `wait_for_ack()` invoke the barrier via
  `api.dispatch("request quiesce")` (the reachable dispatch-command path);
  `wait_for_ack`'s `time.sleep` removed.
- Unit tests `quiesce_drain_test.go` (7, -race); `quiesce_test.go` expects 2 quiescers.
### Bugs Found/Fixed
- **Root cause of the whole sleep saga:** `wait_for_ack` called
  `_call_engine("ze-bgp:peer-flush")`, which is `unknown method` for a plugin (api-yang
  RPCs dispatch via command path, not as engine ops), silently swallowed -- the barrier
  never ran and the sleep did all the work. Fixed by dispatching the command instead.
### Documentation Updates
- `plan/learned/1118-quiesce-peer-drain.md` (the invocation finding).
- `plan/deferrals.md` AC-6 row -> done.
- TODO at closure: `docs/architecture/api/commands.md` Quiesce Barrier (note bgp-peer-sync).
### Deviations from Plan
- Plan step 3 said "`wait_for_ack` -> `quiesce()`". `quiesce()` itself had to be fixed
  first (it used the same broken `_call_engine` path). Both now dispatch the command.
- A-3 wording ("skip non-established peers") was too strong: nexthop needs the
  not-yet-established-but-queued peer to BE waited on; `PendingSync` (no state gate)
  handles both. Only a down peer with an EMPTY queue is skipped.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestPeersSyncedWaitsForSyncingPeer` | blocks until `!PendingSync` |
| AC-2 | done | `TestPeersSyncedTrueWhenAllDrained` | returns immediately |
| AC-3 | done | `TestPeersSyncedSkipsIdleNonEstablished` | down+empty-queue peer skipped |
| AC-4 | done | probe: `request quiesce` -> `quiesced:[bgp-forward-pool,bgp-peer-sync]`; `quiesce_test.go` | 2 quiescers |
| AC-5 | done | `grep time.sleep` in `wait_for_ack` shows none | dispatches `request quiesce` |
| AC-6 | done | nexthop.ci sleepless 25/25 + full plugin suite 472/474 (2 reds = scratch files) | the yo-yo is closed |
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
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| wait_for_ack sleepless without regression | functional | nexthop.ci (278) sleepless PASS 25/25 repeated; full plugin suite 472/474 (only reds were 2 scratch debug files, since deleted); `grep time.sleep` in `wait_for_ack` shows none |
| barrier actually runs (not a silent no-op) | probe | probe plugin: `dispatch("request quiesce")` -> `{status:done, quiesced:[bgp-forward-pool,bgp-peer-sync]}`; `_call_engine("ze-system:quiesce")` -> `unknown method` (the old broken path) |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what review reported] | file:line | fixed/deferred/acknowledged |
### Fixes applied
### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
### Final status
- [ ] review shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Claim | Source evidence | Verified |
|-------|-----------------|----------|

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 demonstrated
- [ ] Wiring Test complete
- [ ] review gate clean
- [ ] `make ze-test` passes (or scoped)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered
### Quality Gates
- [ ] Implementation Audit complete
### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Minimal coupling
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste)
- [ ] Tests PASS (paste)
- [ ] Functional tests for end-to-end behavior
### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-quiesce-peer-drain.md`
- [ ] **Commit A:** code + tests + docs + spec + learned + counter
- [ ] **Commit B:** `git rm plan/spec-quiesce-peer-drain.md`
