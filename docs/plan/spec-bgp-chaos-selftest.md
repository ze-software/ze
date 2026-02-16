# Spec: bgp-chaos-selftest (Phase 10 of 11) — SKELETON

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-inprocess.md` (Phase 9)
**Next spec:** `spec-bgp-chaos-integration.md` (Phase 11)
**DST reference:** `docs/plan/deterministic-simulation-analysis.md`

**Status:** Blocked — depends on Phase 9 (in-process mode with timer-aware FakeClock)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design
3. `internal/sim/clock.go` - Clock, Timer interfaces
4. `internal/sim/network.go` - Dialer, ListenerFactory interfaces
5. `internal/sim/fake.go` - FakeClock, FakeDialer, FakeListenerFactory
6. `internal/plugins/bgp/reactor/reactor.go` - SetClock, SetDialer, SetListenerFactory
7. `.claude/rules/planning.md` - workflow rules
8. Phase 9 done spec - mock components and clock driver

## Task

Add a self-testing chaos mode to Ze itself. When started with `--chaos-seed <N>`, Ze wraps its real Clock, Dialer, and ListenerFactory with chaos-injecting wrappers that introduce seed-driven random failures during normal operation.

Unlike the external `ze-bgp-chaos` tool (which tests Ze by misbehaving as a peer from outside) and the in-process mode (which runs the reactor inside the chaos tool), self-test mode makes **Ze misbehave against itself**. The daemon operates normally but its infrastructure randomly fails — connections drop, timers jitter, listener accepts fail — exposing internal resilience bugs (resource leaks, goroutine leaks, stuck FSM states, deadlocks).

**Key difference from ze-bgp-chaos:**

| Aspect | ze-bgp-chaos (external) | ze-bgp-chaos --in-process (Phase 9) | ze bgp server --chaos-seed (Phase 10) |
|--------|------------------------|--------------------------------------|---------------------------------------|
| Who fails | Peers misbehave | Mock infrastructure | Ze's own infrastructure |
| Who validates | Chaos tool | Chaos tool | External observer (ze-bgp-chaos or manual) |
| What it finds | Protocol handling bugs | Same + timing bugs | Internal resilience: leaks, deadlocks, recovery |
| Network | Real TCP | Mock (net.Pipe) | Real TCP with injected faults |
| Clock | Real time | Virtual (FakeClock) | Real time with jitter/jumps |

**Scope:**
- `--chaos-seed <N>` flag on `ze bgp server` — enables self-chaos mode
- `--chaos-rate <0.0-1.0>` — probability of fault per operation (default: 0.1)
- ChaosClock wrapping RealClock: timer jitter, occasional time jumps
- ChaosDialer wrapping RealDialer: random connection failures, resets, timeouts
- ChaosListenerFactory wrapping RealListenerFactory: random accept failures, bind delays
- Structured log of all injected faults (for correlation with observed behavior)
- All failures are recoverable — Ze should survive and continue operating

**What this does NOT include:**
- Route validation (use ze-bgp-chaos for that)
- Deterministic replay (real time + real TCP = non-deterministic)
- Timer-level simulation (that's Phase 9's FakeClock with time-wheel)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor lifecycle, peer session management
  → Constraint: Reactor must survive any single component failure and recover
  → Decision: Peers reconnect on session failure (connect-retry timer)
- [ ] `docs/architecture/behavior/fsm.md` - FSM state transitions, timers
  → Constraint: FSM must handle unexpected disconnects in any state
  → Constraint: Connect-retry timer drives reconnection after failure
- [ ] `docs/architecture/config/syntax.md` - config blocks
  → Decision: New config could go in `environment { chaos { } }` block

### Source Code
- [ ] `internal/sim/clock.go` — Clock, Timer interfaces
  → Constraint: ChaosClock must satisfy Clock interface exactly
- [ ] `internal/sim/network.go` — Dialer, ListenerFactory interfaces
  → Constraint: ChaosDialer must satisfy Dialer interface exactly
- [ ] `internal/sim/fake.go` — existing FakeClock, FakeDialer patterns
  → Decision: Chaos wrappers follow same delegation pattern but wrap real (not fake) implementations
- [ ] `internal/plugins/bgp/reactor/reactor.go` — New(), SetClock, SetDialer, SetListenerFactory
  → Constraint: Must call Set* before StartWithContext
  → Constraint: Propagation chain: Reactor → Peer → Session → FSM Timers
- [ ] `internal/plugins/bgp/reactor/session.go` — connection establishment via dialer
  → Constraint: Session.dial() uses s.dialer.DialContext()
- [ ] `internal/plugins/bgp/reactor/listener.go` — listener creation via factory
  → Constraint: Listener uses l.listenerFactory.Listen()
- [ ] `internal/plugins/bgp/fsm/timer.go` — FSM timers via clock
  → Constraint: All 5 timer calls use clock.AfterFunc()
- [ ] Phase 9 done spec — mock components, clock driver design
  → Decision: What timer-aware clock features exist to reuse

**Key insights:**
- Chaos wrappers delegate to real implementations (RealClock, RealDialer, RealListenerFactory) — not fakes
- The seed-driven PRNG decides per-call whether to inject a fault or pass through
- All faults must be survivable — Ze's FSM and reactor should recover from every injected failure
- Structured logging of injected faults enables root-cause analysis when Ze does NOT recover

## Current Behavior (MANDATORY)

**Source files read:** (before writing this spec)
- [ ] `internal/sim/clock.go` — Clock interface: Now, Sleep, After, AfterFunc, NewTimer
- [ ] `internal/sim/network.go` — Dialer interface: DialContext; ListenerFactory interface: Listen
- [ ] `internal/sim/fake.go` — FakeClock (Advance-based), FakeDialer (DialFunc-based), FakeListenerFactory (ListenFunc-based)
- [ ] `internal/plugins/bgp/reactor/reactor.go` — New() hardcodes RealClock/RealDialer/RealListenerFactory; SetClock/SetDialer/SetListenerFactory available before Start
- [ ] `cmd/ze/bgp/main.go` — CLI flag parsing for `ze bgp server`

**Behavior to preserve:**
- Default behavior (no --chaos-seed) is completely unchanged — real implementations used
- All existing tests pass unchanged
- Config parsing, plugin loading, peer lifecycle all unchanged
- BGP protocol behavior correct (chaos injects infrastructure faults, not protocol violations)

**Behavior to change:**
- With `--chaos-seed`: wrap real implementations with chaos wrappers before reactor.Start()

## Data Flow (MANDATORY)

### Entry Point
- CLI: `ze bgp server --chaos-seed 42 --chaos-rate 0.1 config.conf`
- Or config: `environment { chaos { seed 42; rate 0.1; } }`

### Transformation Path
1. Parse CLI flags / config for chaos settings
2. Create reactor with real implementations (normal path)
3. If chaos enabled: wrap with ChaosClock, ChaosDialer, ChaosListenerFactory
4. Call reactor.SetClock(chaosClock), reactor.SetDialer(chaosDialer), reactor.SetListenerFactory(chaosListenerFactory)
5. Start reactor (normal lifecycle)
6. During operation: each Clock/Dialer/ListenerFactory call passes through chaos wrapper
7. Wrapper uses seed-PRNG to decide: pass-through (1-rate) or inject fault (rate)
8. Injected faults logged via slogutil with subsystem "chaos"

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Reactor | SetClock/SetDialer/SetListenerFactory before Start | [ ] |
| Chaos wrapper → Real impl | Delegation (same call, maybe with fault) | [ ] |
| Chaos wrapper → Logger | slog.Debug for every injected fault | [ ] |

### Integration Points
- `internal/sim/` — new ChaosClock, ChaosDialer, ChaosListenerFactory types
- `cmd/ze/bgp/main.go` — `--chaos-seed` and `--chaos-rate` CLI flags
- `internal/slogutil/` — "chaos" subsystem logger (`ze.log.chaos`)
- Reactor SetClock/SetDialer/SetListenerFactory — already exist

### Architectural Verification
- [ ] No reactor changes (only injection via existing setters)
- [ ] No protocol changes (chaos injects infrastructure faults, not BGP violations)
- [ ] Default behavior identical (chaos wrappers only created when seed provided)
- [ ] All faults recoverable (no permanent damage to reactor state)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze bgp server config.conf` (no chaos) | Behavior identical to today — no chaos wrappers |
| AC-2 | `ze bgp server --chaos-seed 42 config.conf` | Chaos wrappers injected, faults occur, Ze continues operating |
| AC-3 | `--chaos-seed 42 --chaos-rate 0.0` | Chaos wrappers injected but rate=0 means no faults (passthrough) |
| AC-4 | `--chaos-seed 42 --chaos-rate 1.0` | Every operation faults — Ze should still not crash (graceful degradation) |
| AC-5 | ChaosDialer fault | Connection attempt fails with error, FSM retries via connect-retry timer |
| AC-6 | ChaosClock jitter | Timer fires slightly early/late, Ze handles gracefully |
| AC-7 | ChaosListenerFactory fault | Accept fails, listener retries |
| AC-8 | `ze.log.chaos=debug` | Every injected fault logged with: type, target, seed-state |
| AC-9 | Same seed, same config, two runs | Same sequence of fault decisions (deterministic PRNG) |
| AC-10 | `ze bgp server --chaos-seed 42` + `ze-bgp-chaos --peers 3` | Ze survives dual chaos: self-inflicted + external peer chaos |

## Chaos Wrapper Behavior

### ChaosClock

Wraps RealClock. Fault types:

| Fault | Effect | Frequency (at rate=0.1) | Recovery |
|-------|--------|------------------------|----------|
| Timer jitter | AfterFunc/NewTimer duration multiplied by 0.8-1.2 | 10% of timer creations | Timers fire slightly early/late |
| Now() jitter | Now() offset by ±50ms | 10% of Now() calls | Negligible for most code |
| Sleep extension | Sleep duration multiplied by 1.0-2.0 | 10% of Sleep calls | Slight delays |

**NOT included (too destructive for self-test):**
- Time jumps (backward clock) — would break everything
- Timer non-firing — would cause permanent FSM stalls

### ChaosDialer

Wraps RealDialer. Fault types:

| Fault | Effect | Frequency (at rate=0.1) | Recovery |
|-------|--------|------------------------|----------|
| Connection refused | DialContext returns error | 10% of dial attempts | FSM connect-retry timer fires, retries |
| Slow connect | DialContext sleeps 1-5s before proceeding | 5% of dial attempts | Connection succeeds after delay |
| Connection reset | DialContext succeeds but returned conn closes after 0-100 bytes | 5% of successful dials | Session detects EOF, FSM handles disconnect |

### ChaosListenerFactory

Wraps RealListenerFactory. Fault types:

| Fault | Effect | Frequency (at rate=0.1) | Recovery |
|-------|--------|------------------------|----------|
| Bind failure | Listen returns error | 10% of Listen calls | Reactor retries listener creation |
| Accept delay | Accept() sleeps 1-3s before returning | 5% of Accept calls | Inbound connection delayed but succeeds |

### PRNG Design

- Single `math/rand.Rand` seeded from `--chaos-seed`
- Each fault decision: `rng.Float64() < rate`
- Fault type selection: weighted random from fault table
- Fault parameters (jitter multiplier, delay duration): rng-derived
- Thread-safe: mutex-protected (multiple goroutines call wrappers concurrently)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChaosClockPassthrough` | `internal/sim/chaos_test.go` | Rate=0 → all calls pass through to RealClock | |
| `TestChaosClockJitter` | `internal/sim/chaos_test.go` | Rate=1 → timer durations jittered within bounds | |
| `TestChaosClockDeterministic` | `internal/sim/chaos_test.go` | Same seed → same jitter sequence | |
| `TestChaosDialerPassthrough` | `internal/sim/chaos_test.go` | Rate=0 → all dials pass through to RealDialer | |
| `TestChaosDialerFault` | `internal/sim/chaos_test.go` | Rate=1 → connection failures injected | |
| `TestChaosDialerDeterministic` | `internal/sim/chaos_test.go` | Same seed → same fault sequence | |
| `TestChaosListenerPassthrough` | `internal/sim/chaos_test.go` | Rate=0 → all listens pass through | |
| `TestChaosListenerFault` | `internal/sim/chaos_test.go` | Rate=1 → bind failures injected | |
| `TestChaosConcurrency` | `internal/sim/chaos_test.go` | Concurrent calls from multiple goroutines are safe | |
| `TestChaosLogging` | `internal/sim/chaos_test.go` | Injected faults produce structured log entries | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| chaos-rate | 0.0-1.0 | 1.0 | N/A (0.0 is valid = disabled) | values >1.0 clamped to 1.0 |
| chaos-seed | 1-MaxUint64 | MaxUint64 | 0 (means disabled) | N/A |
| Jitter multiplier | 0.8-1.2 | 1.2 | N/A (clamped) | N/A (clamped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-selftest-survives` | `test/plugin/chaos-selftest-survives.ci` | Ze with --chaos-seed runs, peers connect, routes propagate despite self-inflicted faults | |
| `chaos-selftest-logging` | `test/plugin/chaos-selftest-logging.ci` | ze.log.chaos=debug produces fault log entries | |
| `chaos-selftest-rate-zero` | `test/plugin/chaos-selftest-rate-zero.ci` | --chaos-rate 0 behaves identically to no chaos | |

## Files to Create
- `internal/sim/chaos.go` — ChaosClock, ChaosDialer, ChaosListenerFactory
- `internal/sim/chaos_test.go` — unit tests for chaos wrappers

## Files to Modify
- `cmd/ze/bgp/main.go` — add `--chaos-seed` and `--chaos-rate` flags, wire into reactor
- `test/plugin/` — functional tests for self-test mode

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| CLI flags | Yes | `cmd/ze/bgp/main.go` — `--chaos-seed`, `--chaos-rate` |
| Config syntax | Maybe | `environment { chaos { } }` — defer to user preference |
| Subsystem logger | Yes | `ze.log.chaos` via slogutil |
| Reactor changes | No | Uses existing SetClock/SetDialer/SetListenerFactory |
| Functional tests | Yes | `test/plugin/chaos-selftest-*.ci` |

## Implementation Steps

1. **Read Phase 9 learnings** — timer-aware FakeClock design, mock connection patterns
   → Review: What chaos-wrapper patterns did Phase 9 establish?

2. **Write ChaosClock tests** — passthrough, jitter, determinism
   → Run: Tests FAIL

3. **Implement ChaosClock** — wraps RealClock with seed-PRNG fault injection
   → Run: Tests PASS

4. **Write ChaosDialer tests** — passthrough, fault injection, determinism
   → Run: Tests FAIL

5. **Implement ChaosDialer** — wraps RealDialer with seed-PRNG fault injection
   → Run: Tests PASS

6. **Write ChaosListenerFactory tests** — passthrough, fault injection
   → Run: Tests FAIL

7. **Implement ChaosListenerFactory** — wraps RealListenerFactory with seed-PRNG
   → Run: Tests PASS

8. **Wire CLI flags** — `--chaos-seed` and `--chaos-rate` in `cmd/ze/bgp/main.go`

9. **Write functional tests** — Ze with chaos survives peer connections

10. **Run functional tests** — verify Ze operates correctly under self-chaos

11. **Verify** — `make lint && make test && make functional`

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
| --chaos-seed CLI flag | | | |
| --chaos-rate CLI flag | | | |
| ChaosClock wrapping RealClock | | | |
| ChaosDialer wrapping RealDialer | | | |
| ChaosListenerFactory wrapping RealListenerFactory | | | |
| Seed-deterministic fault sequence | | | |
| Structured fault logging | | | |
| Ze survives all injected faults | | | |
| Default behavior unchanged | | | |
| Functional tests pass | | | |

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
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Integration completeness: chaos wrappers proven to work via functional test (see `rules/integration-completeness.md`)

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Master design doc updated
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests verify Ze survives self-chaos

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-selftest.md`
