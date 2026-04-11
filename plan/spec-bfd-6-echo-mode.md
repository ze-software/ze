# Spec: bfd-6-echo-mode

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-bfd-5-authentication |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec
2. `.claude/rules/planning.md`
3. `rfc/short/rfc5880.md` §6.4 (Echo function) and §6.8.9 (Reception of Echo)
4. `rfc/short/rfc5881.md` §5 (single-hop Echo port 3785)
5. `plan/learned/555-bfd-skeleton.md`, 556, Stage 2-5 learned summaries
6. Source files: `internal/plugins/bfd/engine/loop.go`, `internal/plugins/bfd/session/machine.go`, `internal/plugins/bfd/transport/udp.go`

## Task

Stage 6 implements BFD Echo mode. Echo mode lets the local end send a self-directed packet that the remote forwards back over the same path; the RTT measurement and the sheer fact of receipt confirm forwarding-plane liveness with lower control-plane load than Async Control packets.

Echo has specific constraints:

- **Single-hop only** (RFC 5881 §5, RFC 5883 §4 explicitly prohibits multi-hop echo).
- **UDP port 3785.**
- When echo is active and the peer advertises `RequiredMinEchoRxInterval > 0`, the local end MAY send echo packets at that rate.
- The session's async Control packet rate MAY be slowed to the bfd.RequiredMinRxInterval advertised by the remote (§6.8.9).
- Detection time can be driven by echo RTT instead of async control, per §6.8.4 when echo is in use.

Stage 6 delivers:

1. New transport socket bound to UDP 3785 (single-hop only).
2. Echo packet format -- 24-byte Control-like envelope per RFC 5880 §6.4 (the RFC says "the format is local matter" -- ze chooses the 24-byte Control layout with a distinct magic to distinguish from real Control packets on 3784).
3. Per-session echo scheduler: if the profile sets `echo enabled` AND the peer advertises `RequiredMinEchoRxInterval > 0`, the engine schedules echo sends at the negotiated interval.
4. Echo receiver: returns the same bytes to the sender; the local sender matches received echoes against outstanding ones by a session-local ID and updates detection timing.
5. YANG profile knob: `echo { enabled true; desired-min-echo-tx-us 50000 }`.
6. Metrics: `ze_bfd_echo_rx_total`, `ze_bfd_echo_tx_total`, `ze_bfd_echo_rtt_us` histogram.
7. Slowed async control: when echo is active, async TX slows to the peer's advertised `RequiredMinRxInterval`.

**Explicitly out of Stage 6 scope:**

- Demand mode (RFC 5880 §6.6). Already in the cancelled bucket.
- Multi-hop echo -- prohibited by RFC 5883.
- Adaptive echo rate based on measured RTT jitter.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/bfd.md`
- [ ] `.claude/rules/plugin-design.md`

### RFC Summaries

- [ ] `rfc/short/rfc5880.md` -- §6.4 Echo function, §6.8.9 Reception of Echo, detection time formula
  → Constraint: Echo is single-hop only.
  → Constraint: "A BFD implementation MAY choose to slow down periodic transmission of BFD Control packets if bfd.RemoteMinRxInterval is smaller than bfd.EchoReceiveInterval" (non-binding, but we implement it because it is the whole point).
- [ ] `rfc/short/rfc5881.md` -- §5 single-hop port 3785

### Source files

- [ ] `internal/plugins/bfd/engine/loop.go` -- tick path
- [ ] `internal/plugins/bfd/session/machine.go` -- detection time calculation
- [ ] `internal/plugins/bfd/transport/udp.go` -- second socket per Loop

## Current Behavior (MANDATORY)

**Source files read:** (filled during /implement)

**Behavior to preserve:**

- Stage 1-5 lifecycle, YANG surface, auth code, and metrics.
- Async Control TX rate unchanged when echo is disabled.

**Behavior to change:**

- `bfd.DesiredMinEchoTxInterval` and `bfd.RequiredMinEchoRxInterval` start appearing in transmitted and parsed Control packets. Today the codec writes `0` for these; Stage 6 actually populates them.
- New echo loop goroutine shares the express-loop goroutine (no new goroutine -- echo TX is scheduled by the same tick).
- Second socket per single-hop Loop: ztransport.UDP grows an optional companion socket on port 3785 that the Loop drives.

## Data Flow

### Entry Point

- Async Control negotiation: peer advertises `RequiredMinEchoRxInterval > 0` → local enables echo.
- YANG `bfd { profile { echo { enabled true; desired-min-echo-tx-us 50000 } } }`.

### Transformation Path

1. Session Init reads profile echo config → `m.vars.DesiredMinEchoTxInterval`.
2. Build() populates this in outgoing Control packets.
3. Receive() captures peer's `RequiredMinEchoRxInterval`.
4. Tick: if both > 0 AND session is Up AND echo is configured locally, schedule echo packets on the echo socket.
5. Peer's echo receiver loops them back; our socket receives, match against outstanding ID, record RTT.
6. Detection time: use `RequiredMinEchoRxInterval * detect-multiplier` in place of async detection when echo is active.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Transport ↔ Engine | new echo RX channel on the second UDP socket | [ ] |
| Engine ↔ Session | Machine grows echo state vars + outstanding ID tracker | [ ] |

### Integration Points

- `transport.UDP` extended to drive two sockets OR a new `transport.Echo` added.
- `engine.Loop` handles echo in its tick.
- Metrics registered via Stage 4 telemetry registry.

### Architectural Verification

- [ ] Single-writer invariant preserved: echo TX/RX runs in the express loop
- [ ] No per-packet goroutine
- [ ] No new public API surface

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `bfd { profile { echo enabled } }` + both ends support echo | → | Engine schedules echo TX on port 3785 | `test/plugin/bfd-echo-handshake.ci` |
| Echo RTT exceeds detect time | → | Session transitions Down with diag `Echo Function Failed` | `internal/plugins/bfd/engine/echo_test.go` |
| Echo metrics | → | `ze_bfd_echo_tx_total`, `ze_bfd_echo_rx_total`, `ze_bfd_echo_rtt_us` populated | `test/plugin/bfd-echo-metrics.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Local echo enabled; peer advertises `RequiredMinEchoRxInterval = 50 ms` | Local schedules echo packets at 50 ms intervals |
| AC-2 | Echo packets loop back successfully | RTT recorded; session stays Up |
| AC-3 | Echo packets stop looping back (path failure) | Session transitions Down with `Echo Function Failed` |
| AC-4 | Peer advertises `RequiredMinEchoRxInterval = 0` | Local does NOT send echo; async Control continues |
| AC-5 | Multi-hop session with echo enabled in profile | Config parse rejects -- echo is single-hop only |
| AC-6 | Echo active + async rate slowed | Async Control TX at `max(1s, peer.RequiredMinRxInterval)` |
| AC-7 | Out-of-order echo reply | Matched by ID; RTT from its send timestamp |
| AC-8 | Unknown echo ID received | Dropped; counter incremented |
| AC-9 | Echo on port 3785 receives a malformed packet | Dropped; no crash |
| AC-10 | `plan/deferrals.md` row `spec-bfd-6-echo-mode` | Marked done |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEchoSchedulerEnabled` | `internal/plugins/bfd/engine/echo_test.go` | AC-1: peer advertises echo → scheduler fires | |
| `TestEchoSchedulerDisabledByPeer` | `internal/plugins/bfd/engine/echo_test.go` | AC-4 | |
| `TestEchoDetectionFailure` | `internal/plugins/bfd/engine/echo_test.go` | AC-3 | |
| `TestEchoOutOfOrderReply` | `internal/plugins/bfd/engine/echo_test.go` | AC-7 | |
| `TestEchoUnknownIDDrop` | `internal/plugins/bfd/engine/echo_test.go` | AC-8 | |
| `TestAsyncRateSlowedUnderEcho` | `internal/plugins/bfd/engine/echo_test.go` | AC-6 | |
| `TestEchoMultiHopRejected` | `internal/plugins/bfd/config_test.go` | AC-5 | |
| `FuzzEchoPacket` | `internal/plugins/bfd/packet/echo_fuzz_test.go` | AC-9 | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| echo-id | 0-4294967295 | 4294967295 | N/A | N/A (wraps) |
| desired-min-echo-tx-us | 1-4294967295 | 4294967295 | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-echo-handshake` | `test/plugin/bfd-echo-handshake.ci` | Two ze; both opt into echo; session Up; echo counters advance | |
| `bfd-echo-failover` | `test/plugin/bfd-echo-failover.ci` | Echo path broken; session Down in detect time | |
| `bfd-echo-metrics` | `test/plugin/bfd-echo-metrics.ci` | RTT histogram bucket populated | |

### Future
- None.

## Files to Modify

- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- echo profile block
- `internal/plugins/bfd/config.go` -- parse echo block, reject on multi-hop
- `internal/plugins/bfd/packet/control.go` -- populate echo intervals
- `internal/plugins/bfd/session/machine.go` -- echo state vars, outstanding IDs, detection-time formula
- `internal/plugins/bfd/engine/loop.go` -- tick schedules echo
- `internal/plugins/bfd/engine/echo.go` (new) -- echo TX/RX pieces
- `internal/plugins/bfd/transport/udp.go` -- optional companion socket on 3785 OR `transport.Echo` as separate struct
- `internal/plugins/bfd/metrics.go` -- echo counters
- `plan/deferrals.md` -- close row
- Docs per table below

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG | [ ] Yes | `ze-bfd-conf.yang` |
| CLI | [ ] No | - |
| Functional test | [ ] Yes | three `.ci` tests |
| Metrics | [ ] Yes | `metrics.go` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File |
|---|----------|----------|------|
| 1 | User-facing feature? | [ ] Yes | `docs/features.md` |
| 2 | Config syntax? | [ ] Yes | `docs/guide/configuration.md` |
| 3 | CLI? | [ ] No | - |
| 4 | API/RPC? | [ ] No | - |
| 5 | Plugin? | [ ] Yes | `docs/guide/plugins.md` |
| 6 | User guide? | [ ] Yes | `docs/guide/bfd.md` |
| 7 | Wire format? | [ ] Yes | `docs/architecture/bfd.md` |
| 8 | Plugin SDK? | [ ] No | - |
| 9 | RFC behavior? | [ ] Yes | `rfc/short/rfc5880.md` §6.4, rfc5881.md §5 echo |
| 10 | Test infrastructure? | [ ] No | - |
| 11 | Daemon comparison? | [ ] Yes | `docs/comparison.md` |
| 12 | Internal architecture? | [ ] Yes | `docs/architecture/bfd.md` |
| 13 | Route metadata? | [ ] No | - |

## Files to Create

- `internal/plugins/bfd/engine/echo.go`
- `internal/plugins/bfd/engine/echo_test.go`
- `internal/plugins/bfd/packet/echo_fuzz_test.go`
- `test/plugin/bfd-echo-handshake.ci`
- `test/plugin/bfd-echo-failover.ci`
- `test/plugin/bfd-echo-metrics.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files tables |
| 3. Implement | Phases below |
| 4. Verify | `make ze-verify` |
| 5-12 | as usual |

### Implementation Phases

1. **Phase: YANG + config parse** -- echo profile block, multi-hop rejection.
2. **Phase: Echo transport socket** -- second UDP bound to 3785 for single-hop Loops.
3. **Phase: Echo TX scheduler** -- tick schedules echo at `max(local desired, peer required-min-echo-rx)`.
4. **Phase: Echo RX matcher** -- outstanding ID set, RTT recording.
5. **Phase: Detection time** -- when echo active, detection driven by echo RTT.
6. **Phase: Async rate slowing** -- apply to `TransmitInterval`.
7. **Phase: Metrics** -- counters + histogram.
8. **Phase: Functional tests**.
9. **Phase: Docs**.
10. **Phase: Close spec**.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Correctness | Echo scheduler only fires when both ends have echo enabled + active |
| Naming | `echoScheduler`, `outstandingEcho`, `echoInterval` |
| Data flow | Echo is per-session; express loop remains single-writer |
| Rule: no-layering | No "echo disabled" fallback path; either the profile enables it or it's off |
| Rule: proximity | Echo code in `engine/` + `transport/`, not scattered |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Echo socket bound | `ss -ulnp \| grep 3785` shows ze process (from .ci) |
| Echo TX observed | `bfd-echo-handshake.ci` asserts `ze_bfd_echo_tx_total > 0` |
| Echo detection failure | `bfd-echo-failover.ci` passes |
| Multi-hop rejection | `TestEchoMultiHopRejected` passes |
| Fuzz clean | 30 s fuzz run |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Echo port spoofing | Local-source-only enforcement: reject echo packets whose source is not our own local address |
| Amplification | Do not reflect any packet whose source address is not the session peer |
| Resource exhaustion | Outstanding-ID set bounded (`min(DetectMult*2, 16)`) |
| Fuzz | `FuzzEchoPacket` 30 s |

### Failure Routing

| Failure | Route to |
|---------|----------|
| Echo socket bind fails (privilege) | Log + disable echo for that loop; do not stop the loop |
| Fuzz panic | Fix parser |
| 3 fix attempts fail | STOP |

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

## Design Insights

- Echo format "is a local matter" per RFC 5880. Picking the 24-byte Control layout keeps codec reuse.
- Outstanding-ID set is a small fixed-size ring; overflow drops the oldest outstanding measurement (which is the same as a detection failure for that slot).

## RFC Documentation

- `// RFC 5880 Section 6.4: "The Echo function allows..."` above the echo scheduler
- `// RFC 5881 Section 5: "...Echo packets... destination UDP port 3785..."` above the socket bind
- `// RFC 5883 Section 4: "...echo packets MUST NOT be used..."` above the multi-hop rejection

## Implementation Summary

### What Was Implemented
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes (includes `make ze-test` -- lint + all ze tests)
- [ ] Feature code integrated
- [ ] Functional tests pass
- [ ] Docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC 5880 §6.4 / 5881 §5 / 5883 §4 annotations added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Echo is single-hop only
- [ ] No new goroutine per packet
- [ ] Bounded outstanding-ID set

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests
- [ ] Fuzz target

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written
- [ ] Summary in commit
