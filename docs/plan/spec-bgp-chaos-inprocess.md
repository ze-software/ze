# Spec: bgp-chaos-inprocess (Phase 9 of 9) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-shrink.md`
**Next spec:** None (final phase)
**DST reference:** `docs/plan/deterministic-simulation-analysis.md` (Sections 5, 11)

**Status:** Blocked — requires Ze Clock and Network abstraction interfaces (not yet implemented).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design
3. `docs/plan/deterministic-simulation-analysis.md` Sections 5, 11 - seeded randomness, simulator architecture
4. Phase 6-8 done specs - event log, properties, shrinking
5. Ze clock abstraction implementation (wherever it lands)
6. Ze network abstraction implementation (wherever it lands)
7. `.claude/rules/planning.md` - workflow rules

## Task

Add an in-process execution mode to `ze-bgp-chaos` where the chaos tool and Ze's reactor run in the same Go process, communicating through mock network connections and advancing a virtual clock instead of waiting for real time.

This is the convergence point where the external chaos tool becomes a lightweight deterministic simulator. Same scenario generation, same validation, same properties, same shrinking — but running 100-1000x faster because there's no TCP overhead, no real timers, no wall-clock waiting.

**Scope:**
- `--in-process` flag that runs Ze reactor in-process with mock connections
- Virtual clock drives all timers (hold-timer, keepalive, connect-retry)
- Mock network layer connects chaos peers to Ze without TCP
- Seed controls everything: scenario generation, timer jitter, connection ordering
- Event log from in-process mode is identical in format to external mode (Phase 6)
- Properties (Phase 7) and shrinking (Phase 8) work unchanged
- Determinism verification: same seed → identical event log (byte-for-byte)

**Relationship to DST:**
This is the practical realization of the DST analysis's "Interface Injection" approach (Section 4.4). It combines:
- Clock abstraction (DST Phase 1) — virtual clock for timers
- Network abstraction (DST Phase 2) — mock connections for I/O
- Seeded randomness (DST Section 5) — seed controls all non-determinism
- Property testing (DST Section 9) — RFC properties checked continuously

What it intentionally does NOT include (deferred to full DST):
- Goroutine scheduler control (Go select remains non-deterministic)
- FSM event queue serialization
- Complete replay from event log (replay via validation model, not reactor re-execution)

This means in-process mode is "mostly deterministic" (Polar Signals terminology) — the scenario and validation are fully deterministic, but Go's select statement and goroutine scheduling introduce minor non-determinism in timing. For chaos testing purposes this is acceptable: the goal is fast property checking, not bit-exact replay of reactor internals.

**External dependencies:**
- Ze's Clock interface must exist (being implemented elsewhere)
- Ze's Dialer/Listener interfaces must exist (being implemented elsewhere)
- Reactor must accept injected Clock and network interfaces

## Required Reading

### Architecture Docs
- [ ] `docs/plan/deterministic-simulation-analysis.md` Section 4.4, 5, 11 - interface injection, seeded randomness, simulator
  → Decision: Interface injection for Clock, Dialer, Listener
  → Constraint: Default to real implementations; mock only in simulation mode
- [ ] `docs/architecture/core-design.md` - reactor lifecycle, peer management
  → Constraint: Reactor creates peers, peers create sessions — injection must flow through
- [ ] `docs/architecture/behavior/fsm.md` - FSM timers
  → Constraint: HoldTimer, KeepaliveTimer, ConnectRetryTimer all use Clock interface

### Source Code
- [ ] Ze Clock interface implementation (path TBD — being done elsewhere)
- [ ] Ze network abstraction implementation (path TBD — being done elsewhere)
- [ ] `internal/plugins/bgp/reactor/reactor.go` — reactor lifecycle, `New()` constructor
- [ ] `internal/plugins/bgp/reactor/peer.go` — peer creation, session management
- [ ] `internal/plugins/bgp/reactor/session.go` — TCP dialing, connection handling
- [ ] `internal/bgp/fsm/timer.go` — FSM timers (hold, keepalive, connect-retry)
- [ ] Phase 6-8 implementation (event log, properties, shrinking)

**Key insights:**
- The reactor currently creates real TCP connections and real timers — in-process mode must inject mocks at construction time
- Mock connections need bidirectional byte streams (not just channels) — `net.Pipe()` semantics but with optional fault injection
- Virtual clock must advance only when all goroutines are blocked on I/O or timers — otherwise events arrive before their time
- The chaos tool's peer simulators become the "other end" of mock connections — they write BGP messages directly into the byte stream
- RR plugin runs as a real goroutine inside the same process — it receives UPDATE events from reactor and issues forward commands
- Config generation still needed (reactor reads config) but uses in-memory config, not file I/O

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read when clock/network abstractions are available)
- [ ] `internal/plugins/bgp/reactor/reactor.go` — reactor construction and lifecycle
- [ ] `internal/plugins/bgp/reactor/session.go` — session creation with real net.Dial
- [ ] `internal/bgp/fsm/timer.go` — timer creation with real time.AfterFunc
- [ ] Ze Clock interface (path TBD)
- [ ] Ze network interface (path TBD)

**Behavior to preserve:**
- All Phase 1-8 chaos tool functionality
- External TCP mode remains the default
- Event log format identical between modes
- Properties and shrinking work unchanged

**Behavior to change:**
- Add `--in-process` mode that uses mock clock + mock network
- Reactor instantiated in-process instead of as separate OS process

## Data Flow (MANDATORY)

### Entry Point
- Same as external mode: seed + CLI flags → scenario generator → peer profiles + chaos schedule
- Difference: reactor started in-process with injected mocks

### Transformation Path

**External mode (existing):**
```
chaos tool ──TCP──> Ze process (reactor + RR plugin)
```

**In-process mode (new):**
```
chaos tool ──mock conn──> reactor (same process)
                            ↕
                         RR plugin (same process)
                            ↕
                      virtual clock (drives timers)
```

1. Scenario generated from seed (same as external)
2. Reactor instantiated with mock Clock, mock Dialer, mock Listener
3. RR plugin started as goroutine (using Unix socket pair or in-process channels)
4. Chaos peer simulators connected via mock connections (no TCP)
5. Virtual clock advanced to drive timers (hold, keepalive, connect-retry)
6. Events recorded to same event log format
7. Properties checked continuously
8. On violation: auto-shrink (Phase 8) runs at in-process speed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Chaos peers ↔ Reactor | Mock connections (bidirectional byte streams) | [ ] |
| Reactor ↔ RR Plugin | Unix socket pair or in-process pipe | [ ] |
| Timer system ↔ Virtual clock | Clock interface injection | [ ] |
| Event log ↔ Reporter | Same channel as external mode | [ ] |

### Integration Points
- Ze's Clock interface — injected into reactor, peer, session, FSM timers
- Ze's Dialer/Listener interface — injected into session (mock returns mock conns)
- Ze's reactor `New()` — must accept optional Clock and network overrides
- Ze's RR plugin — runs as goroutine, communicates via socket pair
- Phase 6 event log — identical format
- Phase 7 properties — identical interface
- Phase 8 shrinking — identical algorithm, much faster iteration

### Architectural Verification
- [ ] Reactor code unchanged (only injection points used)
- [ ] Mock connections behave like real TCP (ordered bytes, EOF, errors)
- [ ] Virtual clock advances correctly (timers fire in order, no wall-clock leaks)
- [ ] RR plugin operates normally (receives events, issues forwards)
- [ ] No `time.Now()`, `net.Dial()`, `net.Listen()` calls leak past mocks
- [ ] Event log indistinguishable from external mode (same format, same fields)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--in-process --seed 42 --peers 4` | Runs to completion, produces event log + summary |
| AC-2 | 30-second external run | Equivalent in-process run completes in <1 second |
| AC-3 | Same seed, in-process mode | Event log has same event types and ordering as external (timing may differ) |
| AC-4 | Route propagation | Routes forwarded by RR plugin to correct peers |
| AC-5 | Hold-timer expiry chaos | Virtual clock advanced past hold-time → session torn down |
| AC-6 | Disconnect + reconnect chaos | Mock connection closed, new mock connection established |
| AC-7 | Properties pass | All Phase 7 properties checked, pass for correct scenarios |
| AC-8 | `--auto-shrink --in-process` | Violation → shrink completes in seconds (not minutes) |
| AC-9 | 50-peer in-process run | Completes without deadlock or resource exhaustion |
| AC-10 | `--in-process` without clock/network abstractions | Clear error: "in-process mode requires Ze with clock/network injection" |
| AC-11 | External mode unchanged | `ze-bgp-chaos` without `--in-process` works exactly as before |

## Mock Components

### Mock Clock

Uses Ze's VirtualClock implementation (from clock abstraction spec):
- `Now()` returns simulated time
- `AfterFunc(d, cb)` schedules callback at simulated time + d
- `Advance(d)` moves time forward, firing timers in order
- Advancement strategy: advance to next pending timer when all goroutines blocked

### Mock Network

Uses Ze's mock network implementation (from network abstraction spec):
- `MockDialer.DialContext()` returns one end of a `MockConn` pair
- `MockListener.Accept()` returns the other end
- `MockConn.Read/Write()` are bidirectional byte streams (like `net.Pipe()`)
- Optional: fault injection hooks for partial reads, delays, resets

### Mock Connection Pair

Each chaos peer ↔ reactor connection needs a pair of connected mock conns:

| End | Holder | Reads From | Writes To |
|-----|--------|------------|-----------|
| Client end | Chaos peer simulator | Reactor's writes | Reactor's reads |
| Server end | Reactor session | Chaos peer's writes | Chaos peer's reads |

For passive peers (Ze connects out): MockDialer returns client end, chaos peer holds server end.
For active peers (tool connects in): MockListener returns server end, chaos peer holds client end.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInProcessBasic` | `inprocess/inprocess_test.go` | 2 peers, route exchange via RR, no chaos | |
| `TestInProcessClock` | `inprocess/inprocess_test.go` | Virtual clock drives hold-timer correctly | |
| `TestInProcessDisconnect` | `inprocess/inprocess_test.go` | Mock conn close → reactor detects disconnect | |
| `TestInProcessReconnect` | `inprocess/inprocess_test.go` | New mock conn after disconnect → session re-established | |
| `TestInProcessChaos` | `inprocess/inprocess_test.go` | Chaos events execute via mock connections | |
| `TestInProcessProperties` | `inprocess/inprocess_test.go` | Properties checked, pass for correct behavior | |
| `TestInProcessEventLog` | `inprocess/inprocess_test.go` | Event log format identical to external mode | |
| `TestInProcessSpeed` | `inprocess/inprocess_test.go` | 4-peer 30s scenario completes in <1s | |
| `TestInProcessDeterminism` | `inprocess/inprocess_test.go` | Same seed → same property results | |
| `TestInProcessShrink` | `inprocess/inprocess_test.go` | Shrinking works at in-process speed | |
| `TestInProcessNoLeakedTime` | `inprocess/inprocess_test.go` | No real `time.Now()` calls during in-process run | |
| `TestInProcessFallback` | `inprocess/inprocess_test.go` | Missing Ze abstractions → clear error message | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Peers (in-process) | 1-50 | 50 | 0 | 51 |
| Virtual time advance | 1ms granularity | N/A | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-inprocess-basic` | `test/chaos/inprocess-basic.ci` | 4 peers, seed 42, in-process, no chaos → PASS | |
| `chaos-inprocess-chaos` | `test/chaos/inprocess-chaos.ci` | 4 peers, chaos-rate 0.3, in-process → PASS | |
| `chaos-inprocess-shrink` | `test/chaos/inprocess-shrink.ci` | In-process with auto-shrink | |

## Files to Create

- `cmd/ze-bgp-chaos/inprocess/runner.go` — in-process execution engine
- `cmd/ze-bgp-chaos/inprocess/runner_test.go`
- `cmd/ze-bgp-chaos/inprocess/mockpeer.go` — mock connection management for chaos peers
- `cmd/ze-bgp-chaos/inprocess/mockpeer_test.go`
- `cmd/ze-bgp-chaos/inprocess/clockdriver.go` — virtual clock advancement strategy
- `cmd/ze-bgp-chaos/inprocess/clockdriver_test.go`

## Files to Modify

- `cmd/ze-bgp-chaos/main.go` — add `--in-process` flag, dispatch to in-process runner
- `cmd/ze-bgp-chaos/orchestrator.go` — abstract over external/in-process execution
- `cmd/ze-bgp-chaos/peer/simulator.go` — accept mock conn instead of TCP conn
- `cmd/ze-bgp-chaos/peer/session.go` — use injected conn (mock or TCP)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | No | Already has ze-bgp-chaos target |
| Ze Clock interface | Yes (external dep) | Wherever clock abstraction lands |
| Ze network interface | Yes (external dep) | Wherever network abstraction lands |
| Reactor constructor | Maybe | May need option pattern for injection |

## Implementation Steps

1. **Verify Ze abstractions exist** — Clock and network interfaces available
   → If not: this spec blocks until they are

2. **Read Phase 6-8 learnings** — event log, properties, shrinking APIs
   → Review: What needs to change for in-process execution?

3. **Design connection pair management** — how mock conns are created and paired
   → Review: Does Ze's mock network support paired connections?

4. **Write basic in-process test** (2 peers, route exchange, no chaos)
   → Run: Tests FAIL

5. **Implement in-process runner** — reactor instantiation with mocks
   → Run: Tests PASS

6. **Write clock driver tests** — virtual clock advancement
   → Run: Tests FAIL

7. **Implement clock driver** — advance when all blocked, fire timers in order
   → Run: Tests PASS

8. **Write chaos in-process tests** — disconnect, reconnect via mock conns
   → Run: Tests FAIL

9. **Implement chaos via mock connections**
   → Run: Tests PASS

10. **Wire event log, properties, shrinking** — verify they work unchanged

11. **Performance test** — 30s scenario in <1s

12. **Wire into CLI** — `--in-process` flag

13. **Verify** — `make lint && make test`

## Spec Propagation Task

**MANDATORY at end of this phase (final chaos phase):**

1. **Update `docs/plan/spec-bgp-chaos.md`** (master design) with:
   - Final 9-phase architecture as-built
   - External vs in-process mode comparison
   - Performance observations
   - Known limitations and future work (full DST: scheduler control, FSM queue)

2. **Update `docs/architecture/`** if architectural insights discovered

3. **Consider adding to MEMORY.md:**
   - In-process testing patterns
   - Mock connection pair management
   - Virtual clock advancement strategies
   - Performance characteristics

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| --in-process flag | | | |
| Virtual clock drives timers | | | |
| Mock network connections | | | |
| Seed-controlled execution | | | |
| Event log format identical | | | |
| Properties work unchanged | | | |
| Shrinking at in-process speed | | | |
| 100x+ speedup over external | | | |
| No leaked real time/network calls | | | |
| Graceful fallback if abstractions missing | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Master design doc updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-inprocess.md`
