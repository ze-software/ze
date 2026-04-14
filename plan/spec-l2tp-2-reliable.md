# Spec: l2tp-2 -- L2TP Reliable Delivery Engine

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-l2tp-1-wire |
| Phase | - |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
3. `docs/research/l2tpv2-implementation-guide.md sections 11-12`

## Task

Implement the L2TP reliable delivery engine for control messages: Ns/Nr
sequence numbering (modulo-65536), retransmission with exponential backoff,
sliding receive window, slow start and congestion avoidance (with correct
integer fractional counter), ZLB acknowledgment, duplicate detection and
re-acknowledgment, post-teardown state retention (~31 seconds).

This is a per-tunnel engine consumed by the tunnel state machine (phase 3).
No network I/O in this phase (message send/receive abstracted as interfaces).

Reference: docs/research/l2tpv2-implementation-guide.md sections 11
(reliable delivery), 12 (slow start and congestion control).

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 11 (Reliable Delivery)
  -> Constraint: Ns/Nr are 16-bit modulo-65536. `seqBefore(a,b)` = `int16(a-b) < 0`. ZLB does NOT increment Ns; next non-ZLB reuses the Ns value. Retransmit MUST update Nr to current value (Section 24.9).
  -> Constraint: max retransmits 5, schedule 1s/2s/4s/8s/16s. Timeout cap >= 8s. On max-exceeded: tunnel teardown (Section 11.5).
  -> Constraint: duplicate MUST be ACKed (Section 11.7 / 24.5). Out-of-order MAY be discarded OR queued (Section 11.4).
  -> Constraint: post-teardown retention ~31s = one full retransmit cycle, for both sender and receiver of StopCCN (Section 11.8).
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 12 (Slow Start / Congestion Control)
  -> Decision: `cwnd` starts at 1, capped at peer RWS. SSTHRESH initialized to peer RWS. Slow start (cwnd<ssthresh): cwnd++ per ACK. Congestion avoidance: integer fractional counter, cwnd++ only after cwnd ACKs. On retransmit: ssthresh=max(cwnd/2,1), cwnd=1.
  -> Constraint: effective send window is `min(cwnd, peer_rws)`. Never send more outstanding than this.
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 24 (Implementation Traps)
  -> Constraint: 24.3 ZLB Ns unchanged; 24.4 ignore Nr in data messages; 24.6 peer RWS=0 treated as 1; 24.9 retransmit updates Nr; 24.16 test wraparound near 65535/0.
- [ ] `docs/research/l2tpv2-ze-integration.md` Section 11 (Concurrency Model)
  -> Decision: single reactor goroutine reads UDP + dispatches synchronously to tunnels; single timer goroutine manages all deadlines via min-heap; both owned by phase 3. Phase 2 MUST be goroutine-free and tick-driven (caller passes `now`).
  -> Constraint: no goroutine-per-tunnel; reliable engine runs inline on reactor goroutine.
- [ ] `plan/learned/594-l2tp-1-wire.md` -- phase 1 API surface
  -> Constraint: `WriteControlHeader(buf,off,length,tid,sid,ns,nr) int` encodes header with fixed flags 0xC802; offset 10-11 of a control message = Nr (rewrite-in-place on retransmit).
  -> Constraint: `GetBuf()`/`PutBuf()` provide 1500-byte pool buffers -- NOT suitable for long-lived retention. Retransmit queue owns heap-allocated copies sized to message length.
  -> Constraint: `MessageHeader` is a value type with `Ns`, `Nr`, `HasSequence`, `IsControl`, `PayloadOff`.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2661.md` Section 5.8 (reliable delivery), Section 26.2 (sequence comparison)
  -> Constraint: matches Section 11 of implementation guide; primary citation for AC tests.

**Key Insights** (minimal context to resume after compaction):
- Phase 2 is a per-tunnel pure-logic engine consumed by phase 3. No goroutines, no UDP, no timers-of-its-own.
- Engine owns: `nextSendSeq` (Ns), `nextRecvSeq` (expected peer Ns), `peer_Nr` (highest Nr acked by peer), CWND, SSTHRESH, fractional counter, rtms_queue (ordered by Ns: `{attemptCount, bytes}`), send_queue (window-throttled), recv_queue (ring buffer for reorder), `needsZLB` flag, `closedAt`.
- Engine is tick-driven: `Tick(now time.Time)` returns messages to retransmit and signals max-exceeded. `NextDeadline()` lets phase 3 aggregate tunnels in its global min-heap.
- Retransmit rewrites bytes 10-11 (Nr) in-place, not re-encoded. Single deadline per tunnel (entire rtms_queue shares one timer, replays together on expiry, doubles until rtimeout_cap).
- Sequence comparison: `int16(a-b) < 0`. Works cleanly across wraparound.
- Post-teardown: engine still ACKs duplicate StopCCN until `Expired(now)`. Retention = `sum(i=1..max_retransmit) min(rtimeout*2^(i-1), rtimeout_cap)` = 31s with defaults (1+2+4+8+16).
- Constants: `DEFAULT_RTIMEOUT = 1s`, `DEFAULT_RTIMEOUT_CAP = 16s`, `DEFAULT_MAX_RETRANSMIT = 5`, `DEFAULT_PEER_RCV_WND_SZ = 4` (initial optimistic value per accel-ppp / RFC 2661 S5.8 line 2615 "MUST accept a window of up to 4"), `DEFAULT_RECV_WINDOW = 16` (our advertised window, also the reorder ring-buffer size), `RECV_WINDOW_SIZE_MAX = 32768` (RFC S5.8 derivation: Ns half-space = 32768).
- Reorder policy: Ns within [nextRecvSeq+1, nextRecvSeq+recv_window) is queued in recv_queue; Ns beyond is discarded (peer retransmits); Ns < nextRecvSeq is a duplicate that MUST be ACKed via ZLB.
- Out-of-order ACK semantics: do NOT ACK immediately on queue. ACK fires when gap fills and Nr advances, or on the next piggyback opportunity.
- Decided 2026-04-14: full RFC-compliant scope (CWND + retention + reorder queue). Accel-ppp's shortcut (no CWND, no retention) would violate the S5.8 retention MUST.

## Current Behavior (MANDATORY)

**Source files read:**
- `internal/component/l2tp/header.go` (207L) -- `MessageHeader` value struct, `ParseMessageHeader`, `WriteControlHeader(buf, off, length, tid, sid, ns, nr) int`, `ControlHeaderLen=12`. Ns lives at bytes 8-9, Nr at bytes 10-11 of a control header. Retransmit rewrites Nr in-place.
- `internal/component/l2tp/pool.go` (39L) -- `GetBuf`/`PutBuf` return `*[]byte` of `BufSize=1500`. Pool contract: "caller MUST call PutBuf when done. Buffers held past the call chain of a single outbound message create cross-message aliasing bugs." Retransmit queue therefore cannot hold pool buffers -- it owns heap slices sized to each message length.
- `internal/component/l2tp/avp.go` / `avp_compound.go` -- no direct phase 2 consumption; phase 3 uses these to build message bodies, which phase 2 stores as opaque byte slices.
- `/home/thomas/Code/github.com/accel-ppp/accel-ppp/accel-pppd/ctrl/l2tp/l2tp.{c,h}` -- reference implementation. Per-tunnel `l2tp_conn_t` fields: `Ns, Nr, peer_Nr, peer_rcv_wnd_sz, rtimeout, rtimeout_cap, max_retransmit, retransmit, rtms_queue, send_queue, recv_queue`. Single `rtimeout_timer` per tunnel, period doubles on expiry (l2tp.c:2142-2144), caps at `rtimeout_cap` (l2tp.c:2144). `nsnr_cmp` at l2tp.c:242-264 uses unsigned arithmetic; ze uses signed `int16(a-b) < 0` (equivalent). Accel-ppp skips CWND/SSTHRESH and post-teardown retention; ze implements both per RFC 2661 S5.8.
- `tmp/rfc-ref/rfc2661.txt` (downloaded from rfc-editor.org) -- authoritative Section 5.8 text at lines 2527-2630, Appendix A at lines 4207-4247. Quoted in annotations above.

**Behavior to preserve:** Phase 1 public API unchanged. Phase 2 is purely additive.

**Behavior to change:** None. Phase 2 adds a new engine; no existing phase-1 code is modified.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
Phase 2 has **no user entry point** -- it is an internal library consumed by phase 3's tunnel reactor. Its "entry points" are Go API calls from phase 3: `NewEngine`, `Enqueue`, `OnReceive`, `Tick`, `NextDeadline`, `NeedsZLB`, `BuildZLB`, `Close`, `Expired`.

### Transformation Path
1. Phase 3 reactor reads a UDP datagram, calls `ParseMessageHeader` (phase 1), gets a `MessageHeader`.
2. Phase 3 looks up the tunnel by `hdr.TunnelID`, calls `engine.OnReceive(hdr, payload)`. Engine classifies the message (in-order / duplicate / reorder-queued / reorder-dropped), advances `peer_Nr`, clears ACKed messages from `rtms_queue`, updates CWND. Returns `Classification` and any now-deliverable queued messages for phase 3's tunnel state machine.
3. Phase 3's tunnel state machine processes delivered messages, chooses to send something (e.g. reply SCCRP). It encodes the message body via phase 1 writers into a heap buffer, then calls `engine.Enqueue(msgType, bytes)`. Engine assigns Ns, prepends/fixes header fields (or accepts pre-encoded header with Ns placeholder), adds to `send_queue`, and -- if window allows -- promotes to `rtms_queue` and returns the ready-to-send bytes.
4. Phase 3 emits the bytes on the UDP socket.
5. Phase 3's timer goroutine maintains a global min-heap of tunnel deadlines. For each tunnel it stores `engine.NextDeadline()`. When the nearest deadline fires, phase 3 calls `engine.Tick(now)`, receives the list of messages to retransmit (already Nr-rewritten), sends them, and re-queries `NextDeadline()`.
6. When the tunnel state machine decides to shut down, phase 3 calls `engine.Close(now)`. Engine remains alive to ACK retransmitted StopCCN frames until `engine.Expired(now) == true`, after which phase 3 reaps the tunnel.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Phase 3 reactor -> engine | Go method call on reactor goroutine, no locking | [ ] |
| Engine -> phase 3 (retransmit bytes) | Return value from `Tick(now)` -- slice of `[]byte` refs owned by engine | [ ] |
| Engine -> phase 3 (delivered messages) | Return value from `OnReceive` -- slice of in-order payloads | [ ] |
| Engine -> phase 3 (teardown trigger) | Distinguished error / sentinel from `Tick` when `retransmit >= max_retransmit` | [ ] |

### Integration Points
- None with external components in phase 2. Phase 3 will wire `engine.Tick` into the reactor's timer dispatch and `engine.OnReceive` into the UDP read path.

### Architectural Verification
- [ ] No bypassed layers (engine does not touch UDP, netlink, YANG, or the event bus)
- [ ] No unintended coupling (engine has no import outside `internal/component/l2tp/`)
- [ ] No duplicated functionality (reuses `WriteControlHeader` from phase 1 for ZLB emission; no re-implementation)
- [ ] Zero-copy preserved where applicable: engine accepts pre-encoded message bytes. For the retransmit queue, bytes are copied once from the caller's scratch buffer into a heap slice at the exact message length -- this is the copy-on-retention boundary documented in `rules/design-principles.md` "Zero-copy, copy-on-modify"

## Wiring Test (MANDATORY -- NOT deferrable)

Phase 2 is pure-logic, reactor-free, pre-user-entry-point. Per `rules/testing.md` line 30 ("Pure-logic, reactor-free code... belongs in Go unit tests, not in any `.ci` directory") the wiring test is a Go integration test. The phase 1 precedent for the same shape is `cmd/ze/l2tp/decode_test.go`. A `.ci` functional test for the L2TP subsystem becomes possible only in phase 3 when the UDP listener is wired; the deferral for the L2TP `.ci` test category already lives in `plan/deferrals.md` from phase 1 (open, destination `spec-l2tp-3-tunnel`).

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Go caller `NewEngine(...)` + full A/B conversation | -> | `reliable.go` end-to-end | `internal/component/l2tp/reliable_integration_test.go` `TestTunnelHandshakeWiring` |
| Go caller triggers dropped message | -> | retransmit path | Same file, `TestRetransmitOnDrop` |
| Go caller delivers out-of-order | -> | recv_queue path | Same file, `TestReorderDelivery` |
| Go caller closes tunnel, peer retransmits StopCCN | -> | retention path | Same file, `TestPostTeardownAckRetention` |

## Acceptance Criteria

Each AC has a unit or integration test whose assertion verifies the expected behavior directly (not a mechanism proxy). RFC citations point to `tmp/rfc-ref/rfc2661.txt` line numbers when quoted; the `rfc/short/rfc2661.md` summary is the in-repo reference.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `seqBefore(a, b)` across all 16-bit values | Returns `int16(a-b) < 0`. Cases: `seqBefore(1, 2) = true`; `seqBefore(65535, 0) = true` (wraparound); `seqBefore(0, 65535) = false`; `seqBefore(100, 100) = false` |
| AC-2 | `Enqueue(msg)` with empty send_queue and empty rtms_queue | Assigns `Ns = nextSendSeq`, increments `nextSendSeq` (mod 65536), adds to rtms_queue with deadline `now + rtimeout`, returns the bytes for transmission |
| AC-3 | `Enqueue(msg)` when outstanding == effective window (`min(cwnd, peer_rcv_wnd_sz)`) | Message queued in send_queue; NOT added to rtms_queue; does NOT return bytes for transmission. Drained automatically on subsequent ACK. |
| AC-4 | `OnReceive(hdr)` where `hdr.Ns == nextRecvSeq` | Message classified `InOrder`. Engine increments `nextRecvSeq`. Delivers payload plus any subsequently-unlocked messages from recv_queue. Marks `needsZLB=true`. |
| AC-5 | `OnReceive(hdr)` where `seqBefore(hdr.Ns, nextRecvSeq)` (duplicate) | Message classified `Duplicate`. `nextRecvSeq` unchanged. Marks `needsZLB=true`. RFC 2661 S5.8 line 2550: "receipt of duplicate messages MUST be acknowledged". |
| AC-6 | `OnReceive(hdr)` where `hdr.Ns > nextRecvSeq` and within recv_window | Message classified `ReorderQueued`. Stored in recv_queue ring buffer at slot `(hdr.Ns - nextRecvSeq) mod recv_window`. `nextRecvSeq` unchanged. `needsZLB` NOT set (no ACK until gap fills). |
| AC-7 | `OnReceive(hdr)` where `hdr.Ns > nextRecvSeq + recv_window` (beyond advertised window) | Message classified `Discarded`. Not queued, not ACKed. Peer will retransmit. |
| AC-8 | `OnReceive(hdr)` where `hdr.Nr` advances past outstanding messages | All rtms_queue entries with `seqBefore(entry.Ns, hdr.Nr)` removed. CWND updated per slow-start or congestion-avoidance. Retransmit counter reset. Next send_queue items promoted to rtms_queue if window allows. |
| AC-9 | `OnReceive(hdr)` where `!hdr.IsControl` (data message, S=1) | Engine ignores Nr field per RFC 2661 S5.8 and trap 24.4; no state change. |
| AC-10 | `OnReceive` of ZLB (control message with zero-length body) | Nr processed normally; `needsZLB` NOT set (no ACK-of-ACK); classifier distinguishes ZLB from non-ZLB. |
| AC-11 | `Tick(now)` where `now >= oldest_rtms_deadline` and `retransmit < max_retransmit` | Returns all rtms_queue entries with their Nr field rewritten to current `nextRecvSeq` (bytes 10-11 of the header). Doubles `rtimeout` up to `rtimeout_cap`. Increments `retransmit`. Schedules next deadline at `now + rtimeout`. |
| AC-12 | `Tick(now)` when rtms_queue empty | No-op. Returns empty slice. |
| AC-13 | `Tick(now)` when `retransmit == max_retransmit` on another expiry | Returns `TeardownRequired` signal; caller (phase 3) initiates tunnel teardown. |
| AC-14 | Upon retransmission firing (regardless of max) | `SSTHRESH = max(CWND/2, 1)`; `CWND = 1`; `cwndCounter = 0`. Per RFC 2661 Appendix A. |
| AC-15 | ACK advances peer_Nr during slow-start phase (CWND < SSTHRESH) | `CWND++`. Per RFC 2661 Appendix A. |
| AC-16 | ACK advances peer_Nr during congestion-avoidance phase (CWND >= SSTHRESH) | `cwndCounter++`; if `cwndCounter >= CWND`: `CWND++`, `cwndCounter = 0`. Per RFC 2661 Appendix A. |
| AC-17 | CWND would exceed `peer_rcv_wnd_sz` | Capped at `peer_rcv_wnd_sz`. Per RFC 2661 Appendix A: "CWND is never allowed to exceed the size of the advertised window". |
| AC-18 | Peer advertises `peer_rcv_wnd_sz = 0` | Engine treats as 1 (guide S24.6) and emits a warning log entry. |
| AC-19 | `NextDeadline()` with empty rtms_queue | Returns `time.Time{}` (zero value -- phase 3 treats as no deadline). |
| AC-20 | `NextDeadline()` with non-empty rtms_queue | Returns the earliest deadline among queued entries. |
| AC-21 | `NeedsZLB()` after receiving an in-order or duplicate non-ZLB message | Returns `true` until the engine emits a non-ZLB outbound message (piggyback ACK) OR `BuildZLB` is called. |
| AC-22 | `BuildZLB(buf, off)` | Writes a 12-byte control header with `Ns = nextSendSeq` (NOT incremented -- RFC S5.8 line 2556-2557), `Nr = nextRecvSeq`, empty body. Clears `needsZLB`. Returns 12. |
| AC-23 | `Close(now)` when tunnel never fully established | Engine transitions to closed state. `closedAt = now`. Rejects subsequent `Enqueue` calls. Continues to ACK duplicates via `BuildZLB`. |
| AC-24 | After `Close(now)`: `Expired(t)` for `t - closedAt < retentionDuration` | Returns `false`. Engine still ACKs duplicates. |
| AC-25 | After `Close(now)`: `Expired(t)` for `t - closedAt >= retentionDuration` | Returns `true`. Phase 3 reaps. `retentionDuration = sum(i=1..max_retransmit) of min(rtimeout * 2^(i-1), rtimeout_cap)` = 31s with defaults (1+2+4+8+16). Per RFC S5.8 line 2602-2605: "MUST be maintained and operated for the full retransmission interval". |
| AC-26 | `Enqueue` after `Close` | Returns `ErrEngineClosed`. |
| AC-27 | Wraparound: peer has sent Ns=65535, ze sends Ns=65534, peer acks with Nr=65535, then Nr=0 | `peer_Nr` advances correctly through wraparound; rtms_queue is cleared appropriately. |

## 🧪 TDD Test Plan

### Unit Tests

All in `internal/component/l2tp/reliable_test.go` unless noted. Table-driven with `t.Run(tt.name, ...)` per `rules/tdd.md`.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSeqBefore | reliable_test.go | AC-1 sequence comparison + wraparound | [ ] |
| TestEnqueueOpenWindow | reliable_test.go | AC-2 happy path enqueue | [ ] |
| TestEnqueueWindowFull | reliable_test.go | AC-3 send_queue holding | [ ] |
| TestOnReceiveInOrder | reliable_test.go | AC-4 in-order delivery + needsZLB | [ ] |
| TestOnReceiveDuplicate | reliable_test.go | AC-5 duplicate MUST ACK | [ ] |
| TestOnReceiveReorderQueued | reliable_test.go | AC-6 recv_queue insertion, no premature ACK | [ ] |
| TestOnReceiveReorderGapFill | reliable_test.go | AC-6 delivery chain when gap fills | [ ] |
| TestOnReceiveReorderBeyondWindow | reliable_test.go | AC-7 discard | [ ] |
| TestOnReceiveAckAdvance | reliable_test.go | AC-8 rtms_queue flush + window re-open | [ ] |
| TestOnReceiveDataMessage | reliable_test.go | AC-9 ignore Nr in data messages | [ ] |
| TestOnReceiveZLB | reliable_test.go | AC-10 no needsZLB for ZLB input | [ ] |
| TestTickRetransmit | reliable_test.go | AC-11 Nr rewrite + deadline schedule | [ ] |
| TestTickBackoffSchedule | reliable_test.go | AC-11 1s/2s/4s/8s/16s with cap | [ ] |
| TestTickEmpty | reliable_test.go | AC-12 no-op | [ ] |
| TestTickMaxAttempts | reliable_test.go | AC-13 TeardownRequired | [ ] |
| TestCongestionSlowStart | reliable_test.go | AC-15 CWND grows +1 per ACK | [ ] |
| TestCongestionAvoidance | reliable_test.go | AC-16 integer fractional counter | [ ] |
| TestCongestionResetOnRetransmit | reliable_test.go | AC-14 SSTHRESH/CWND reset | [ ] |
| TestCWNDCappedByPeerRWS | reliable_test.go | AC-17 cap | [ ] |
| TestPeerRWSZero | reliable_test.go | AC-18 treated as 1 + warn log | [ ] |
| TestNextDeadlineEmpty | reliable_test.go | AC-19 zero time | [ ] |
| TestNextDeadlineOldest | reliable_test.go | AC-20 earliest | [ ] |
| TestNeedsZLBLifecycle | reliable_test.go | AC-21 flag set/clear | [ ] |
| TestBuildZLBFormat | reliable_test.go | AC-22 wire bytes + Ns not consumed | [ ] |
| TestCloseTransitions | reliable_test.go | AC-23 state change + rejects enqueue | [ ] |
| TestExpiredBeforeAndAfter | reliable_test.go | AC-24, AC-25 retention window | [ ] |
| TestRetentionDurationComputation | reliable_test.go | AC-25 sum-of-schedule, not hardcoded 31s | [ ] |
| TestEnqueueAfterClose | reliable_test.go | AC-26 ErrEngineClosed | [ ] |
| TestWraparoundAckThrough65535To0 | reliable_test.go | AC-27 mod-65536 flush | [ ] |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Ns (uint16) | 0-65535 | 65535 (wraps to 0) | N/A (uint16) | N/A (uint16) |
| peer_rcv_wnd_sz | 1-32768 | 32768 | 0 (coerced to 1) | N/A -- accel-ppp uses RECV_WINDOW_SIZE_MAX=32768 per RFC S5.8 (half of 16-bit space) |
| max_retransmit | 1-255 (configurable) | 255 | 0 | N/A |
| rtimeout | 100ms-rtimeout_cap | rtimeout_cap | <100ms (rejected at config) | >rtimeout_cap (rejected) |
| rtimeout_cap | rtimeout..? | 300s (arbitrary sanity cap) | <rtimeout (rejected) | >300s (rejected) |
| CWND | 1-peer_rcv_wnd_sz | peer_rcv_wnd_sz | 0 (impossible: reset to 1) | >peer_rcv_wnd_sz (capped) |

### Integration / Wiring Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| TestTunnelHandshakeWiring | `internal/component/l2tp/reliable_integration_test.go` | Two engines simulate SCCRQ/SCCRP/SCCCN/ZLB conversation. All messages delivered in order. | [ ] |
| TestRetransmitOnDrop | same | One message dropped by the test harness; retransmit fires at 1s, message delivered, acked. | [ ] |
| TestReorderDelivery | same | Engine receives Ns=3 before Ns=2; Ns=2 arrives later; both delivered in order afterwards. | [ ] |
| TestPostTeardownAckRetention | same | Engine A sends StopCCN and Close()s; peer B retransmits StopCCN after its ZLB is "lost"; A ACKs via BuildZLB for 31s then Expired returns true. | [ ] |

### Fuzz (MANDATORY for external-input parsing)

Phase 2 does not parse wire bytes (phase 1 does). Fuzz is not structurally required, but a target covering OnReceive sequence robustness is cheap insurance:

| Test | Location | Validates | Status |
|------|----------|-----------|--------|
| FuzzOnReceiveSequence | `internal/component/l2tp/reliable_fuzz_test.go` | Never panic on any (Ns, Nr, peer_rcv_wnd_sz, flags) combination. Seed corpus covers wraparound edges. | [ ] |

### Future (if deferring any tests)
- None planned

## Files to Modify
- None. Phase 2 is purely additive; phase 1 files untouched.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc2661.md` -- extend "Reliable Delivery (Section 5.8)" section with CWND / retention / reorder citations already present; verify alignment with Appendix A text |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A (pre-subsystem-wiring) |
| 12 | Internal architecture changed? | [x] | `docs/architecture/wire/l2tp.md` -- add "Reliable delivery engine" section describing the public API surface phase 3 will consume |

## Files to Create

| File | Concern | Target LOC |
|------|---------|-----------|
| `internal/component/l2tp/reliable.go` | Engine struct, NewEngine, Enqueue, OnReceive, Tick, Close/Expired, classification types | ~450 |
| `internal/component/l2tp/reliable_window.go` | CWND/SSTHRESH/slow-start/congestion-avoidance helpers | ~80 |
| `internal/component/l2tp/reliable_reorder.go` | recv_queue ring buffer with insert/pop-in-order | ~120 |
| `internal/component/l2tp/reliable_seq.go` | `seqBefore`, retention-duration computation, constants | ~60 |
| `internal/component/l2tp/reliable_test.go` | Unit tests (29 entries above) | ~700 |
| `internal/component/l2tp/reliable_integration_test.go` | Four integration tests (wiring) | ~350 |
| `internal/component/l2tp/reliable_fuzz_test.go` | FuzzOnReceiveSequence + seed corpus | ~60 |

All files use `// Design: docs/architecture/wire/l2tp.md -- L2TP reliable delivery engine` + `// Related:` cross-refs per `rules/design-doc-references.md` / `rules/related-refs.md`. Source files carry `// RFC 2661 Section 5.8` + `// RFC 2661 Appendix A` comments above enforcing code per `rules/rfc-compliance.md`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Standard flow |

### Implementation Phases

Phase 2 is a single implementation phase for `/ze-implement` purposes (no sub-phases). Internal ordering when building:

1. **Sequence primitives** -- `reliable_seq.go`: `seqBefore`, retention-duration helper, constants. Write tests first (AC-1, AC-27). Verify wraparound.
2. **Engine skeleton** -- `reliable.go`: `Engine` struct (state fields from Current Behavior), `NewEngine(config)`, error sentinels. No logic yet.
3. **Send path** -- `Enqueue`, send_queue -> rtms_queue promotion, Ns assignment. Tests AC-2, AC-3.
4. **Receive path** -- `OnReceive` classification tree: in-order, duplicate, reorder-queued, reorder-beyond-window, data (ignore Nr), ZLB. Tests AC-4 through AC-10.
5. **Reorder queue** -- `reliable_reorder.go`: ring buffer, insert-at-offset, pop-in-order-from-head. Tests AC-6 gap-fill chain.
6. **ACK processing** -- rtms_queue flush on Nr advance; window re-open; CWND update hook. Test AC-8.
7. **CWND / slow start / congestion avoidance** -- `reliable_window.go`. Tests AC-14 through AC-17.
8. **Retransmit** -- `Tick`, Nr rewrite in-place (bytes 10-11 of cached message), deadline doubling, max-attempts teardown. Tests AC-11, AC-12, AC-13.
9. **ZLB** -- `NeedsZLB`, `BuildZLB`. Uses `WriteControlHeader` from phase 1. Tests AC-21, AC-22.
10. **Close / retention** -- `Close`, `Expired`, post-teardown duplicate-ACK path. Tests AC-23 through AC-26.
11. **Peer RWS edge cases** -- RWS=0 coerce (AC-18), RWS cap at 32768 (guide S11.1 derivation).
12. **Integration tests** -- four scenarios in `reliable_integration_test.go`.
13. **Fuzz** -- `FuzzOnReceiveSequence`, run 5s clean before commit.
14. **Documentation** -- update `docs/architecture/wire/l2tp.md` with the reliable-delivery section; update `rfc/short/rfc2661.md` with CWND/retention/reorder citations.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-27 has a test; every unit test maps to an AC |
| Correctness | Wraparound works near 65535/0; CWND never exceeds `min(computed, peer_rcv_wnd_sz)`; retransmit rewrites Nr at offset 10-11 (not a new buffer); ZLB does not consume Ns |
| Rule: buffer-first | No `append` in encoding; retransmit Nr rewrite is byte-level `PutUint16` into existing slice |
| Rule: goroutine-lifecycle | Engine has zero goroutines; caller passes `now` on every call |
| Rule: no-layering | No "legacy" placeholder code; CWND and retention implemented directly |
| RFC citations | Every MUST behavior has a `// RFC 2661 Section 5.8 line <N>` comment above it (S5.8 = full RFC lines 2527-2630; App A = lines 4207-4247 in the downloaded `tmp/rfc-ref/rfc2661.txt`) |
| Observable state | Engine exposes enough via accessors/stats for phase 3 logging: current CWND, SSTHRESH, outstanding count, retransmit count |
| Concurrency | Engine is NOT safe for concurrent use -- documented on the type -- phase 3's reactor owns it exclusively |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Engine passes all 29 unit tests | `go test -race ./internal/component/l2tp/... -run 'TestSeqBefore\|TestEnqueue\|TestOnReceive\|TestTick\|TestCongestion\|TestCWND\|TestPeerRWS\|TestNextDeadline\|TestNeedsZLB\|TestBuildZLB\|TestClose\|TestExpired\|TestRetention\|TestWraparound' -v` |
| Four integration tests pass | `go test -race ./internal/component/l2tp/... -run 'TestTunnelHandshakeWiring\|TestRetransmitOnDrop\|TestReorderDelivery\|TestPostTeardownAckRetention' -v` |
| Fuzz runs clean for 5s | `go test -race -fuzz=FuzzOnReceiveSequence -fuzztime=5s ./internal/component/l2tp/...` |
| `make ze-verify` passes | With log at `tmp/ze-verify-<session>.log` |
| Line counts within targets | Files under projected LOC; no file exceeds 600 lines (`rules/file-modularity.md`) |
| No allocation in hot path | Inspect `Tick` and `OnReceive`: zero `make`/`append` calls inside. Retransmit rewrites bytes in-place |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `OnReceive` trusts the parsed `MessageHeader` (phase 1 already validated wire bytes). Engine validates: `hdr.TunnelID != 0` for non-SCCRQ is phase 3's responsibility; peer_rcv_wnd_sz coerced; reorder window bounded |
| Resource exhaustion | rtms_queue bounded by `min(cwnd, peer_rcv_wnd_sz) <= 32768`; recv_queue bounded by advertised recv_window (we send 16 by default); send_queue is effectively bounded by application-layer rate (tunnel/session setup is rare) but add a sanity cap -- propose 256 -- and document it |
| Retransmit amplification | Attacker cannot force unbounded retransmits: max_retransmit=5 terminates, retention bounded at ~31s default |
| Post-teardown state | Expired tunnels MUST be reaped by phase 3 to prevent leak; engine's `Expired(now)` exposes the signal; document on the type that phase 3 is responsible |
| Wraparound abuse | Peer injecting Ns=32768-higher-than-current cannot fool engine into delivering -- signed int16 comparison handles half-space correctly |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## RFC Documentation

Add `// RFC 2661 Section X.Y` above enforcing code.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
