# Spec: improve-3 -- Protocol Event Capture and Replay

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `plan/deterministic-simulation-analysis.md` -- prior research on replay/determinism
5. `internal/component/bgp/reactor/session_read.go` -- where inbound messages enter

## Task

When a production BGP session misbehaves, Ze has no way to capture what the peer sent
and feed it back into the same state machine on a developer's desk. The global event
ring stores only timestamp/namespace/event-type (a counter trail, not a reproduction
artifact), and adj-rib-in "replay" re-announces stored routes to a peer, which is a
different feature. Bug reproduction currently means reconstructing peer behavior by
hand.

Add an opt-in, per-session JSONL capture of protocol input events, plus a replay
command that feeds a captured stream back into the same processing path with a
deterministic clock. Start narrow: BGP session inbound messages (wire bytes + arrival
metadata) and config transaction events. Capture is off by default and enabled per
peer or globally via config; files are bounded. The existing research in
`plan/deterministic-simulation-analysis.md` (state capture, clock injection) feeds
this design; this spec implements the capture/replay slice only, not full
deterministic simulation.

## Required Reading

### Architecture Docs
- [ ] `plan/deterministic-simulation-analysis.md` - Sections on state capture and clock control
  → Decision: (fill during design -- which injection points this slice adopts)
- [ ] `docs/architecture/core-design.md` - session/reactor layering
  → Constraint: (fill during design)
- [ ] `ai/rules/buffer-first.md` - capture path must not allocate per message on hot path
  → Constraint: capture writer uses pooled buffers; disabled capture costs one nil check
- [ ] `ai/rules/config-surface.md` - capture enable knob placement (YANG vs env)
  → Decision: (fill during design)

### RFC Summaries (MUST for protocol work)
- No new wire behavior; RFC 4271 message handling is exercised, not changed.

**Key insights:**
- Ze already streams synthetic UPDATEs in-process (`ze-test peer --mode inject`);
  replay closes the loop with REAL captured traffic.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/plugin/server/event_ring.go` - `EventRecord` holds Timestamp/Namespace/EventType only (:13-17); `Append` stores those three fields (:47-49); useful as a trail, not for reproduction
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `replayCommand` re-sends STORED ROUTES from other peers to a target peer (:279-304); route replay, not event replay
- [ ] `internal/component/bgp/reactor/session_read.go` - `readAndProcessMessage` (:57) is the single inbound point where captured bytes would be teed
- [ ] `internal/component/bgp/reactor/session.go` - `Run` loop (:706) drives reads; clock injected via `s.clock` (:767) -- a seam replay can reuse
- [ ] `internal/component/config/transaction/orchestrator.go` - transaction events to capture (`Execute`, :198-234)

**Behavior to preserve:** (unless user explicitly said to change)
- Zero hot-path cost when capture is disabled (single nil/flag check).
- Event ring, adj-rib-in replay, and `ze-test peer` inject mode unchanged.
- No change to message processing semantics under capture.

**Behavior to change:** (only if user explicitly requested)
- None; capture and replay are additive, opt-in features.

## Data Flow (MANDATORY)

### Entry Point
- Capture: inbound BGP message bytes at `readAndProcessMessage` after a complete
  message is read; config transaction events at coordinator phase transitions.
- Replay: `ze bgp replay <capture-file>` (dispatch key per CLI grammar) feeding a
  session instance in a harness process, not the live daemon.

### Transformation Path
1. Capture enabled per peer via config: session tees each complete inbound message (header + body bytes, arrival timestamp, peer identity) to a JSONL writer.
2. Writer appends one JSON object per event to a bounded per-session file (size cap + rotation), off the hot path via a buffered channel or equivalent.
3. Config transaction events (verify/apply/commit/rollback with txID) append to the same format under their own namespace.
4. Replay reads the JSONL stream, constructs a session with a deterministic clock and a stub connection, and feeds the captured bytes through the SAME `readAndProcessMessage` path.
5. Replay output (FSM transitions, RIB effect, NOTIFICATIONs) is observable through existing show/diag surfaces for comparison.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session hot path ↔ capture writer | non-blocking hand-off, drop-with-counter on overflow | [ ] |
| Capture file ↔ replay harness | JSONL schema versioned in the file header line | [ ] |
| Replay ↔ session | stub net.Conn + injected clock (existing `s.clock` seam) | [ ] |

### Integration Points
- `readAndProcessMessage` (`session_read.go:57`) - tee point.
- `s.clock` (`session.go:767` usage) - deterministic clock seam for replay.
- `TxCoordinator.Execute` - transaction event source.
- spec-improve-4 conformance fixtures consume this capture format as their event-stream input.

### Architectural Verification
- [ ] No bypassed layers (replay uses the real read/process path, not a parallel decoder)
- [ ] No unintended coupling (capture writer owned by reactor; format pkg shared with replay tool)
- [ ] No duplicated functionality (extends event trail; does not replace event ring)
- [ ] Zero-copy preserved (capture copies bytes once at tee point, only when enabled)
- [ ] Registration over hardcoding -- replay CLI registers via existing dispatch (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | One tee point (`readAndProcessMessage`) sees all inbound session bytes including coalesced path | `session.go:773-777` shows coalesced variant `readAndProcessCoalesced` | Two tee points needed | Read coalesced path during design | unvalidated |
| A-2 | The `s.clock` seam is sufficient for deterministic replay of timer-driven behavior | clock injected in session (`session.go:767`) | Replay diverges on hold/keepalive timing; need broader clock control from the simulation analysis | Prototype replay of a captured session with timer expiry | unvalidated |
| A-3 | JSONL per-message capture keeps up at stress rates when enabled | buffered writer design | Capture must sample or be documented as debug-rate only | Stress test with `ze-test peer --mode inject` during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Capture files contain operator config/routing data (sensitive) | design review | document handling; store under a diag directory with clear ownership; no auto-upload |
| R-2 | Format churn breaks old captures | first schema change | version field in header line; replay rejects unknown versions with a clear error |
| R-3 | Scope creep into full deterministic simulation | design review | this spec = capture + single-session replay only; simulation stays in the analysis doc |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer config enables capture | → | tee in readAndProcessMessage writes JSONL | TestSessionCaptureWritesEvents |
| ze bgp replay <file> | → | replay harness drives session read path | test/replay/bgp-capture-replay.ci |
| config commit with capture on | → | transaction events appended | TestTransactionEventCapture |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Capture disabled (default) | No file writes; hot path unchanged (benchmark-guarded) |
| AC-2 | Capture enabled, peer sends OPEN/KEEPALIVE/UPDATE | Each message appears as one JSONL event with bytes + metadata |
| AC-3 | Replay of a captured session | Same FSM transitions and RIB effect as the original run (deterministic clock) |
| AC-4 | Capture file reaches size cap | Rotation/stop per config; daemon unaffected |
| AC-5 | Replay of a truncated/corrupt file | Clear error naming the offending line; no panic |
| AC-6 | Config transactions during capture | verify/apply/commit/rollback events with txID recorded |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator hits a session bug, enables capture, reproduces, ships the file | capture -> JSONL -> developer replays -> same failure observed | test/replay/bgp-capture-replay.ci |
| 2 | Developer bisects a fix against a captured stream | replay before/after fix | test/replay/bgp-capture-replay.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSessionCaptureWritesEvents | `internal/component/bgp/reactor/capture_test.go` | tee correctness, bytes round-trip | |
| TestCaptureFormatRoundTrip | capture format package test | encode/decode, version handling | |
| TestReplayDrivesSession | replay harness test | captured stream -> FSM transitions | |
| TestTransactionEventCapture | `internal/component/config/transaction/` test | tx events recorded | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| capture file size cap | (fill during design) | (fill) | (fill) | (fill) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| bgp-capture-replay | `test/replay/bgp-capture-replay.ci` | capture a session, replay it, compare outcome | |

### Interop Tests (MANDATORY for protocol features)
- No new wire behavior; capture of an FRR/BIRD peer session can piggyback on an
  existing interop scenario during implementation (decide at design).

## Files to Modify
- `internal/component/bgp/reactor/session_read.go` - tee point (and coalesced path per A-1)
- `internal/component/config/transaction/orchestrator.go` - transaction event emission
- BGP peer YANG schema - capture enable knob
- CLI dispatch - `ze bgp replay` command registration

## Files to Create
- `internal/component/bgp/reactor/capture.go` - capture writer
- capture format package (location per module tiers during design) - JSONL schema
- replay harness + CLI command
- `test/replay/bgp-capture-replay.ci` - functional test

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - capture knob + writer skeleton + replay command registered; failing wiring tests
2. **Phase: capture format + session tee** (including coalesced path)
3. **Phase: replay harness** with deterministic clock via existing seam
4. **Phase: transaction event capture**
5. Functional test, stress check (A-3), `make ze-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 with file:line |
| Correctness | replay uses the real processing path; no parallel decoder |
| Performance | disabled capture adds no allocation on hot path (`ai/rules/buffer-first.md`) |
| Registration over hardcoding | replay command registered via dispatch registry (`ai/rules/plugin-self-containment.md`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | replay parses untrusted files: bounds, version, no panic on corrupt input |
| Resource exhaustion | file size caps, writer backpressure drops with counter |
| Data sensitivity | captured routing data documented; operator-controlled location |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Capture wire bytes, not decoded structs | decoded-event capture (serialize the internal message types) | bytes replay through the real decoder and survive internal refactors |

## Known Limitations
- Single-session replay only; multi-peer/topology replay and full deterministic
  simulation remain in `plan/deterministic-simulation-analysis.md` scope.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
