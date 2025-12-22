# FSM Active Design Plan

**Status:** Planning → **CLOSED (No Action)**
**Created:** 2025-12-22
**Priority:** ~~Medium~~ **None** (original critique was based on misunderstanding)

---

## Original Critique

> "The FSM is purely a state transition tracker. It does not handle I/O, timers,
> or message sending directly... this design forces the Reactor to handle all the
> complexity of timer management and I/O, leading to the bloat observed in pkg/reactor."

---

## Critical Analysis: The Critique is Incorrect

### Finding 1: Timers ARE in the FSM Package

The original critique stated FSM doesn't handle timers. **This is false.**

```
pkg/bgp/fsm/
├── fsm.go      # State transitions (440 LOC)
├── timer.go    # Timer management (360 LOC)  ← TIMERS ARE HERE
├── state.go    # State/Event definitions
└── *_test.go
```

The FSM **package** absolutely handles timer management. The FSM **struct** delegates
to a separate `Timers` struct within the same package. This is intentional separation
of concerns WITHIN the package, not a design flaw.

### Finding 2: Reactor Bloat is NOT from Timer Complexity

The critique claims passive FSM causes reactor bloat. Analysis of actual code:

| File | LOC | Timer-related | Encoding-related |
|------|-----|---------------|------------------|
| `session.go` | 950 | ~50 LOC | ~100 LOC |
| `peer.go` | 1726 | ~20 LOC | **~1350 LOC** |

**The bloat is from encoding logic, not timer management.**

Timer code in session.go (lines 76-95) is ~20 lines of callback wiring.
The actual timer implementation is in `fsm/timer.go`.

### Finding 3: Critique Conflates Two Unrelated Issues

1. **FSM design** → Clean, well-separated, timers in same package
2. **Reactor bloat** → Caused by encoding logic in peer.go (see `peer-encoding-extraction.md`)

These are separate concerns. "Fixing" FSM design would not reduce reactor bloat.

---

## Actual Architecture (Corrected)

```
┌─────────────────────────────────────────────────────────────┐
│                      pkg/bgp/fsm/                            │
│  ┌─────────────────┐          ┌─────────────────┐           │
│  │    fsm.go       │          │   timer.go      │           │
│  │  State Machine  │          │  Timer Manager  │           │
│  │  - Transitions  │          │  - HoldTimer    │           │
│  │  - RFC 4271 §8  │          │  - Keepalive    │           │
│  │  - Callbacks    │          │  - ConnRetry    │           │
│  └─────────────────┘          └─────────────────┘           │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ uses
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   pkg/reactor/session.go                     │
│                                                              │
│  - Orchestrates FSM events                                   │
│  - Wires timer callbacks (~20 LOC)                           │
│  - Message I/O                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Why Current Design is Correct

### 1. Separation Within Package (Good)

FSM struct: pure state transitions, testable without time dependencies
Timers struct: time-based logic, separately testable
Both in same package: cohesive module

### 2. RFC 4271 Compliance

FSM documents deviations explicitly (excellent practice):
- `fsm.go:5-19`: Lists all violations with RFC section references
- Section 8.2.1.3 permits omitting optional attributes

### 3. ExaBGP Alignment

ExaBGP uses same pattern:
- `src/exabgp/bgp/neighbor/fsm.py` - state machine
- `src/exabgp/reactor/` - orchestration

### 4. Testability

```go
// FSM testable without time
func TestFSMTransitions(t *testing.T) {
    fsm := fsm.New()
    fsm.Event(EventManualStart)
    assert.Equal(t, StateConnect, fsm.State())
}

// Timers testable without FSM
func TestHoldTimerExpiry(t *testing.T) {
    timers := fsm.NewTimers()
    // ...
}
```

---

## Verdict: NO ACTION REQUIRED

The original critique is based on incorrect understanding of the codebase:

| Claim | Reality |
|-------|---------|
| "FSM doesn't handle timers" | Timers in `pkg/bgp/fsm/timer.go` |
| "Forces complexity onto Reactor" | Timer wiring is ~20 LOC |
| "Leading to bloat in reactor" | Bloat is from encoding, not timers |

**The FSM design is sound. No changes needed.**

---

## Real Issue: Reactor Bloat

The actual problem (encoding logic in peer.go) is addressed in:
→ `plan/peer-encoding-extraction.md`

---

## Related Files

| File | LOC | Role |
|------|-----|------|
| `pkg/bgp/fsm/fsm.go` | 440 | State machine |
| `pkg/bgp/fsm/timer.go` | 360 | Timer management |
| `pkg/bgp/fsm/state.go` | ~50 | State/Event definitions |
| `pkg/reactor/session.go` | 950 | Orchestration |

---

## Decision

- [x] **CLOSED** - Original critique was incorrect
- [x] No changes to FSM design needed
- [x] Reactor bloat addressed separately in `peer-encoding-extraction.md`
