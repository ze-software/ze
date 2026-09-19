# Spec: improve-3-event-replay-deferred-deterministic-scheduler

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

The capture/replay prerequisite was fulfilled when the parent closed on
2026-09-05 (`d74f428b47`, `99bbe13ab7`). Status remains `blocked` for the owner
decision on determinism scope below; the parent is no longer a live dependency.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/deterministic-simulation-analysis.md` - the event-queue layer this spec would build
4. `spec-improve-3-event-replay` - the source spec, R-3 and A-2
5. `internal/component/bgp/reactor/session.go`, `peer_run.go` - the clock seam

## Task

Deferred from `spec-improve-3-event-replay` (risk R-3, "Scope creep into full
deterministic simulation", mitigated by the ruling that "this spec = capture +
single-session replay only; simulation stays in the analysis doc"). The same spec's
Design Insights state the boundary in full: replay asserts OUTCOMES (FSM transitions
via `Peer.history`, RIB effect via dispatch show commands), not goroutine
interleavings, and exact interleaving reproduction needs the analysis doc's
event-queue layer, explicitly out of scope.

This spec holds that excluded work: a deterministic scheduler / event-queue layer
that would let a replayed session reproduce the exact goroutine interleaving of the
captured run, not merely the same end state. Without it, a bug that only manifests
under a specific interleaving (a race between the read path, timer expiry, and a
config commit) can be captured but not reliably re-triggered.

**Current prerequisite state (source read 2026-09-19).** Capture and replay exist:
`sessionCapture.writeItem` writes message, config and session events, and
`runReplay` in `internal/test/cli/cmd_replay.go` feeds messages through
`Session.ReadAndProcess` with a fixed fake clock. The parent's final A-2 was
confirmed for message-driven replay only; timer expiry was explicitly left
undone because replay never advances that clock. Its Work Not Done table
assigned timer-driven and multi-peer replay to this spec. Neither that closure
nor this source reading proves exact interleaving reproduction.

The following seam inventory is the historical 2026-07-16 research:

| Fact | Producer | Verified |
|------|----------|----------|
| Clock is injectable per session | `Session.SetClock` (`session.go`) | yes |
| Clock flows Peer -> Session at run time | `peer_run.go` calls `session.SetClock(p.clock)` | yes |
| Peer owns the clock | `Peer.SetClock` (`peer.go`), reactor fans out at `reactor.go` | yes |
| FSM transitions are recorded and assertable | `p.history.append(FSMTransition{...})` (`peer_run.go`) | yes |
| The global event ring is a counter trail, not a reproduction artifact | `EventRecord` holds Timestamp/Namespace/EventType only (`event_ring.go`) | yes |

Note the source spec's line citations have drifted by one to three lines
(`session.go` is now :463, `peer.go`/`:380` is now :382). The behavior is
unchanged; only the line numbers moved.

**Points to complete:**

1. Read `plan/deterministic-simulation-analysis.md` and extract the event-queue
   layer it proposes. improve-3 adopted only the Option-D clock-injection slice
   (Phase 1 of that doc's roadmap) and left the FSM event queue, fault injection,
   and scheduler layers behind. This spec picks up the scheduler layer.
2. Preserve the parent's final A-2 disposition: message-driven replay is covered,
   timer expiry is outstanding. Design the event ordering needed for hold and
   keepalive expiry; do not repeat the old check for an unvalidated parent A-2.
   The owner must decide whether the exact-interleaving scheduler is the chosen
   mechanism and how the inherited multi-peer requirement fits its scope.
3. Decide the scope of determinism: one session, one peer with concurrent timers,
   or the whole reactor including the forward pool (`forward_pool.go` has its
   own `SetClock`) and the listener (`listener.go`).
4. Account for the two inbound read paths. improve-3's A-1 resolved BROKEN: message
   coalescing is default on and has its own read path, so capture needs two tee
   points. A scheduler layer inherits that split.
5. Decide what "same interleaving" means as an assertion. A test that reproduces an
   interleaving must be able to FAIL when the interleaving differs, or it proves
   nothing.

**Known constraint:** `ai/rules/performance.md` applies. Any scheduler seam on the
session read path must cost approximately nothing when replay is off, the way
improve-3 specified capture costs one nil check when disabled.

## Required Reading

### Architecture Docs
- [ ] `plan/deterministic-simulation-analysis.md` - the event queue, scheduler, and fault-injection layers improve-3 left out
  → Decision: improve-3 adopted only the Option-D clock-injection slice; the layers this spec needs stay described there
- [ ] `spec-improve-3-event-replay` - source spec: R-3, A-2, Design Insights, Capture Format v1
  → Constraint: replay asserts outcomes, not interleavings; that boundary is what this spec moves
- [ ] `docs/architecture/core-design.md` - session/reactor layering and ownership
  → Constraint: Session is owned by Peer; a scheduler must respect that ownership rather than reach across it
- [ ] `ai/rules/performance.md` - the read path is hot
  → Constraint: disabled scheduler costs one nil check; no per-message allocation

### RFC Summaries (MUST for protocol work)
- No new wire behavior. RFC 4271 message handling is exercised, not changed.

**Key insights:**
- Capture/replay and the parent's A-2 disposition are available; the remaining blocker is the owner decision on exact-interleaving scope.
- Timer-driven and multi-peer replay remain owned here, as assigned by the parent's closure. Choosing a narrower first implementation cannot discard either obligation.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session.go` - `SetClock` (:463) injects the clock into a session; the session's own timing reads flow through it
- [ ] `internal/component/bgp/reactor/peer_run.go` - :194 wires the peer's clock into the session at run time; :436 appends an `FSMTransition` to `p.history`, the outcome surface replay asserts against
- [ ] `internal/component/bgp/reactor/peer.go` - `SetClock` (:382) is the peer-level clock seam
- [ ] `internal/component/bgp/reactor/reactor.go` - `SetClock` (:536) fans the clock out to child components
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `SetClock` (:401) drives worker idle timers, a second concurrency source a scheduler would have to own
- [ ] `internal/component/bgp/reactor/listener.go` - `SetClock` (:60) drives deadline calculations
- [ ] `internal/component/plugin/server/event_ring.go` - `EventRecord` (:13-17) stores Timestamp/Namespace/EventType, a trail rather than a reproduction artifact

**Behavior to preserve:**
- Production sessions keep running on the real clock and real goroutine scheduling; a scheduler layer is replay-only and off by default.
- The clock injection chain (reactor -> peer -> session) keeps its current shape and ownership.
- Both inbound read paths (coalesced and non-coalesced) keep working; coalescing stays default on.
- `Peer.history` keeps recording FSM transitions in the same form, since existing assertions read it.
- Hot-path cost when replay is disabled stays at a nil check, per `ai/rules/performance.md`.

**Behavior to change:**
- Add a replay-only event-queue / scheduler seam so a captured run can be re-executed with the same ordering, not only the same outcome.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A replay invocation over a capture file produced by `spec-improve-3-event-replay` (JSONL, one event per line, with a version header).
- The session inbound read paths in `internal/component/bgp/reactor/`, where captured wire bytes are fed back.

### Transformation Path
1. Replay reads the versioned capture stream. Current v1 records messages, config operations and session events, but no timer-expiry event; the design must establish the additional ordering data needed before claiming a captured timer interleaving can be reproduced.
2. A scheduler owns the event queue and decides which runnable goroutine or timer advances next, instead of the Go runtime deciding.
3. The injected clock advances only when the scheduler says so, so a timer fires at a queue position rather than at a wall-clock moment.
4. Events feed the same session processing path as production, through both read paths.
5. The replayed run's ordering is compared against the captured ordering, and FSM transitions in `Peer.history` are compared against the captured outcome.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Capture file -> replay harness | JSONL, versioned header (format owned by improve-3) | [ ] |
| Replay harness -> session read paths | the two tee points improve-3 identified | [ ] |
| Scheduler -> clock | the existing `SetClock` chain (reactor -> peer -> session) | [ ] |
| Scheduler -> concurrency sources | forward pool workers, listener deadlines, timers | [ ] |

### Integration Points
- `Session.SetClock` / `Peer.SetClock` / `Reactor.SetClock`, the existing injection chain.
- `Peer.history` (`peer_run.go`) as the outcome oracle.
- The capture format and replay harness delivered by `spec-improve-3-event-replay`.
- `internal/core/clock` fake clock, already used by reactor tests.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (the scheduler must not leak into production session code beyond one seam)
- [ ] No duplicated functionality (extends the existing clock seam, does not recreate it)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding: the replay scheduler registers rather than adding a per-feature field or switch to the reactor

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Exact interleaving reproduction is worth its cost | improve-3 R-3 deferred it rather than cancelling it | The spec should be cancelled instead of built | User decision at pickup, plus a real bug that outcome-replay could not reproduce | unvalidated |
| A-2 | The reactor's concurrency sources can all be driven from one scheduler | Every one has a `SetClock` seam (reactor.go, peer.go, session.go, forward_pool.go, listener.go) | Partial determinism only; scope narrows to a single session | Enumerate goroutine spawns in the reactor at design time | unvalidated |
| A-3 | The capture format can represent the ordering the scheduler needs | v1 carries per-file `seq` for message/config/session events, but no timer-expiry event | A format extension is required before captured timer ordering can be asserted | Compare `internal/core/capture` and `sessionCapture.writeItem` with the chosen scheduler event model | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scheduler seam leaks into the production hot path | Benchmarks regress on disabled replay | One nil check when off; benchmark gate |
| R-2 | Determinism is claimed but not proven, so the layer gives false confidence | No test can fail on a wrong interleaving | Require a test that reproduces a known race and fails when ordering changes |
| R-3 | Scope creeps into full deterministic simulation, the exact failure improve-3 guarded against | Design review | Scope to the scheduler layer only; fault injection stays in the analysis doc |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| replay of a capture with concurrent timer and message events | -> | scheduler drives the event queue instead of the Go runtime | `test/replay/bgp-interleaving-replay.ci` (fill during design) |
| replay disabled (production path) | -> | no scheduler in the read path | `TestSessionReadPathNoSchedulerWhenDisabled` (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A capture containing an interleaving-sensitive sequence is replayed twice | Both runs produce the identical event ordering, not merely the same final state |
| AC-2 | A capture is replayed after the ordering is perturbed | The comparison FAILS, proving the assertion has teeth |
| AC-3 | Replay is disabled | Session read paths are unchanged and the hot path cost is a nil check |
| AC-4 | A timer-driven transition (hold/keepalive) is replayed | It fires at the captured queue position, completing the timer-expiry remainder explicitly excluded from the parent's final A-2 |
| AC-5 | The multi-peer replay remainder inherited at parent closure | The owner-approved design names and proves its replay boundary; it stays owned here until implemented or transferred to an existing spec with explicit scope approval |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchedulerReplayDeterministicOrdering` | `internal/component/bgp/reactor/replay_test.go` | AC-1: two replays yield identical ordering | |
| `TestSchedulerReplayDetectsPerturbedOrder` | `internal/component/bgp/reactor/replay_test.go` | AC-2: the assertion fails on a wrong interleaving | |
| `TestSchedulerDisabledLeavesReadPathUntouched` | `internal/component/bgp/reactor/replay_test.go` | AC-3: no production impact | |
| `TestSchedulerReplayTimerExpiry` | `internal/component/bgp/reactor/replay_test.go` | AC-4: timer fires at the captured position | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-interleaving-replay` | `test/replay/bgp-interleaving-replay.ci` | A developer replays a captured session and reproduces the original interleaving | |

### Future (if deferring any tests)
- (fill during design)

## Files to Modify
- `internal/component/bgp/reactor/` - the scheduler seam on the session read paths and the clock chain (exact files chosen at design; improve-3's tee points are the anchor)
- `plan/deterministic-simulation-analysis.md` - mark the scheduler layer as implemented once it lands

## Implementation Steps

### Implementation Phases

1. **Phase: Owner scope decision (MANDATORY FIRST)** - use the parent's final A-2 and a concrete interleaving-sensitive case to choose the scheduler boundary. Account for timer-driven and multi-peer replay before implementation; capture/replay availability is no longer the gate.
2. **Phase: Wiring** - register the replay scheduler seam; write the failing determinism test.
3. **Phase: Event queue** - (fill during design)
4. **Phase: Scheduler ordering** - (fill during design)
5. **Functional test** - `.ci` proving a developer-visible reproduction.
6. **Full verification** - `./le verify current mode full`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Hot path | Disabled replay costs one nil check (`ai/rules/performance.md`) |
| Assertion has teeth | AC-2 proves the ordering check can fail |
| Registration over hardcoding | The scheduler registers; no per-feature field or switch added to the reactor |
| Scope | Fault injection and full simulation stay out (improve-3 R-3) |

## Known Limitations
- Exact interleaving scope remains an owner decision. No scheduler implementation is authorised by the prerequisite's completion.
- The current replay reports config operations without re-applying them and does not advance the fake clock; the design must distinguish those limits from missing capture infrastructure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Owner decision recorded for exact ordering, timer expiry and inherited multi-peer replay; capture/replay prerequisite recorded as fulfilled
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Learned summary written at closure

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `improve-3-event-replay.md`, 2026-07-10

Deferred by spec-improve-3-event-replay (R-3).

Exact goroutine-interleaving reproduction in event replay (deterministic scheduler / event-queue layer)
