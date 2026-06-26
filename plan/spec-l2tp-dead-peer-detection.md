# Spec: l2tp-dead-peer-detection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-06-26 |

**Resolved decisions (user, 2026-06-26):** knob = `hello-retries` (count, effective
timeout = `hello-retries × hello-interval`); default = **2**; proceed to
implementation now.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc2661.md` - HELLO Keepalive (§15), Retransmission/retention (§5.8)
4. `internal/component/l2tp/reactor.go` (handleTick), `tunnel_fsm.go` (Process, handleHelloTimer, teardownStopCCN), `tunnel.go` (lastActivity), `reliable.go` (Tick, OnReceive, Outstanding), `config.go` (Parameters, ExtractParameters)

## Task

L2TP dead-peer detection is too slow. When the remote LAC (xl2tpd in the
interop/evidence labs) dies, it never delivers a StopCCN to Ze: the wire trace
shows Ze receives SCCRQ→ICCN, then silence. xl2tpd's `death_handler` logs
"Server closing" but emits no StopCCN. Ze's only remaining dead-peer signal is
HELLO keepalive **retransmit exhaustion** in the reliable engine: a single
HELLO is sent after `hello-interval` of silence, then the reliable engine
retransmits it on the default schedule 1+2+4+8+16 = **31 seconds** before
declaring `TeardownRequired`. With the lab's `hello-interval 5`, total detection
is ~5 + 31 = **~36 s**, which blows the evidence harness's 20 s teardown window
(`scripts/evidence/effective-gokrazy-l2tp-ppp.py:732`,
`l2tp: subscriber routes withdrawn`) and the interop check's 30 s window
(`test/l2tp-interop/scenarios/01-ppp-ipv4/check.py:80`).

**Why this is not a one-line fix.** Two naive fixes are wrong:

1. **A hold-timer on `lastActivity`** would falsely tear down an alive-but-quiet
   peer. `lastActivity` is deliberately updated *only* when the engine delivers a
   non-ZLB message (`tunnel_fsm.go:61-67`); a ZLB ACK does **not** touch it (by
   design, so a peer replaying old Ns cannot suppress the HELLO timer). A peer
   that is alive but idle answers Ze's HELLOs with ZLB ACKs only, so its
   `lastActivity` ages forever. A hold-timer on `lastActivity` would kill it.

2. **Shortening the reliable-engine retransmit schedule** (lower `maxRetransmit`
   or `rtimeoutCap`) would weaken link-loss tolerance for *every* control
   message, including SCCRQ/SCCRP during setup and StopCCN/CDN during teardown.
   The 31 s figure is the RFC 2661 §5.8 retention/retransmission interval and
   must stay intact for those phases.

The correct fix is a dedicated dead-peer-detection (DPD) path driven by whether
the peer **acknowledges Ze's HELLO keepalives** (reliable-engine ACK state), kept
**separate** from the generic retransmit backoff, with a production-tunable
threshold expressed in `hello-interval` units.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `docs/architecture/wire/l2tp.md` - L2TP reliable transport, ZLB, HELLO
  → Constraint: HELLO is delivered reliably; the peer's ZLB ACK is the proof of liveness.
- [ ] `docs/guide/l2tp.md` - operator-facing config (`hello-interval`, reload)
  → Constraint: timer leaves live under root `l2tp {}`; `hello-interval` is hot-reloadable.
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision
  → Decision: a per-tunnel protocol timer belongs in YANG under `l2tp {}`, not an env var.
- [ ] `ai/rules/config-naming.md` - kebab-case leaf naming
  → Constraint: pair the new leaf with the existing `hello-interval` naming.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2661.md` §15 (HELLO Keepalive, lines 392-411)
  → Constraint: HELLO sent after a configurable silence period; if HELLO is not
    answered the tunnel is torn down (sessions cleared, retention window kept).
- [ ] `rfc/short/rfc2661.md` §5.8 (Retransmission/retention, lines 205-258)
  → Constraint: control-message retransmit MUST use exponential backoff and the
    full retransmission interval (31 s default) MUST be retained after teardown.
    DPD must NOT shorten this for setup/teardown messages.

**Key insights:**
- Liveness has two distinct signals that must not be conflated: (a) the peer
  delivered a control message (updates `lastActivity`, governs *when to probe*),
  and (b) the peer acknowledged something Ze sent, including a ZLB ACK of a HELLO
  (must govern *when to declare dead*). The current code only has (a).
- The reliable engine already knows when an ACK clears an outstanding message
  (`reliable.go:451` `acked := e.processNr(hdr.Nr)`); this is the hook for (b).
- DPD must only run for `L2TPTunnelEstablished` tunnels so setup (pre-Established)
  and teardown (Closed) keep the full 31 s reliable-retransmit budget.
- `ReactorParams` (reactor.go:62) is a distinct struct from `Parameters`
  (config.go:76); the reactor reads `r.params.HelloInterval` (a `ReactorParams`
  field). The new threshold must live on BOTH structs and be wired through the
  subsystem (Parameters → ReactorParams), mirroring `HelloInterval`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/reactor.go` (handleTick 462-552; ReactorParams 62-75)
  → Constraint: HELLO is sent from handleTick only when
    `state == Established && HelloInterval > 0 && engine.Outstanding() == 0` AND
    `now.Sub(lastActivity) >= HelloInterval` (reactor.go:505-509). Tick-driven
    teardown calls `teardownStopCCN(now, resultGeneralError)` only on
    `result.TeardownRequired` from the engine (reactor.go:490-494). Route
    withdrawal for tick teardowns runs via `notifyRouteObserverDown(tickTeardowns)`
    (reactor.go:543) and `(l2tp, tunnel-down)` is emitted with
    `Reason: "retransmit-timeout"` (reactor.go:531-538). `ReactorParams` holds the
    runtime copy of the timers (HelloInterval at line 69).
- [ ] `internal/component/l2tp/tunnel_fsm.go` (Process 58-87; teardownStopCCN
  254-278; handleStopCCN 561-585; handleHelloTimer 677-691)
  → Constraint: `lastActivity = now` only when `len(res.Delivered) > 0`
    (tunnel_fsm.go:61-67); ZLB ACKs are excluded on purpose. `teardownStopCCN`
    clears sessions, enqueues StopCCN (priority), transitions to Closed, calls
    `engine.Close(now)`. `handleStopCCN` is the prompt path that fires only if the
    peer actually sends StopCCN (xl2tpd does not).
- [ ] `internal/component/l2tp/reliable.go` (Tick 555-591; OnReceive 438-503;
  ReceiveResult 170-178; Outstanding 648; Close 628)
  → Constraint: `Tick` increments `attempts`; `attempts > maxRetransmit` →
    `TeardownRequired`. Defaults `RTimeout=1s`, `RTimeoutCap=16s`,
    `MaxRetransmit=5` → 1+2+4+8+16 = 31 s. `OnReceive` computes `acked` internally
    (line 451) but does NOT surface it in `ReceiveResult` (only Class, Delivered,
    NewSends). `Outstanding()` returns `len(rtmsQueue)`.
- [ ] `internal/component/l2tp/tunnel.go` (lastActivity 83-89)
  → Constraint: single `lastActivity time.Time`; read by reactor HELLO check and
    snapshot; no dead-peer field exists.
- [ ] `internal/component/l2tp/config.go` (Parameters 76-103; ExtractParameters
  114-284; DefaultHelloSecs 60)
  → Constraint: timer leaves parsed from root `l2tp {}`; `hello-interval` default
    60 s, range 1..3600 (config.go:173-182). `Parameters` is the parsed config;
    the subsystem maps it onto `ReactorParams`.
- [ ] `internal/component/l2tp/reactor_setters.go` (setHelloInterval 24-31) and
  `subsystem_reload.go` (110-117)
  → Constraint: hot reload is per-leaf: `subsystem_reload.go` compares prev/next
    and calls the matching `set*` setter under `tunnelsMu`.

**Behavior to preserve:**
- The reliable engine retransmit schedule and 31 s retention window
  (`reliable.go` Tick, `reliable_seq.go` defaults). DPD must NOT change them.
- `lastActivity` semantics (delivered-only) and the existing HELLO **trigger**
  (probe after `hello-interval` of silence). DPD adds teardown, not a new probe.
- The existing teardown plumbing: `teardownStopCCN` → `clearSessions` →
  `collectKernelEventsLocked` → `notifyRouteObserverDown` → `enqueueKernelEvents`.
  DPD reuses this exact chain (it only adds a new *trigger* for it).
- Existing `(l2tp, tunnel-down)` emission; DPD adds a new distinct reason string.

**Behavior to change:**
- Add a faster, ACK-driven dead-peer teardown for Established tunnels, governed by
  a new `hello-retries` leaf. Nothing else changes.

## Data Flow (MANDATORY)

### Entry Point
- Wire: inbound L2TP control datagrams arrive at the listener → reactor →
  `tunnel.Process(hdr, payload, now, ...)`.
- Time: reactor timer fires `handleTick(now)` on the engine deadline (bounded to
  at most `hello-interval` for Established tunnels, reactor.go:519-523).

### Transformation Path
1. **Liveness capture (new):** in `Process`, after `engine.OnReceive`, refresh a
   new tunnel field `lastLiveness = now` when the peer proved it is alive —
   i.e. `len(res.Delivered) > 0` **or** the receive acknowledged at least one of
   Ze's outstanding messages (`res.Acked > 0`, a newly-surfaced field on
   `ReceiveResult` set from `reliable.go:451`'s `acked`). A ZLB ACK of a HELLO
   therefore refreshes `lastLiveness` even though it does not refresh
   `lastActivity`.
2. **HELLO trigger (unchanged):** `handleTick` still sends one HELLO after
   `hello-interval` of `lastActivity` silence when `Outstanding() == 0`.
3. **Dead-peer check (new):** in `handleTick`, for an Established tunnel with
   `hello-retries > 0`, if `now.Sub(lastLiveness) >= hello-retries × hello-interval`,
   declare the peer dead and call the existing `teardownStopCCN(now, ...)`. This
   fires before the engine's 31 s exhaustion whenever `hello-retries × hello-interval < 31s`.
4. **Teardown (reused):** `teardownStopCCN` clears sessions, sends StopCCN,
   closes the engine (retention window preserved); the reactor collects kernel
   teardowns, withdraws subscriber routes, and emits `(l2tp, tunnel-down)` with a
   new reason (e.g. `keepalive-timeout`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ reliable engine | `OnReceive` surfaces `Acked` count | [ ] |
| Reliable engine ↔ tunnel FSM | `Process` refreshes `lastLiveness` on delivered-or-acked | [ ] |
| Tunnel FSM ↔ reactor | `handleTick` reads `lastLiveness`, triggers `teardownStopCCN` | [ ] |
| Reactor ↔ route observer | existing `notifyRouteObserverDown(tickTeardowns)` | [ ] |
| Config tree ↔ reactor | `ExtractParameters` → `Parameters.HelloRetries` → `ReactorParams.HelloRetries` → setter on reload | [ ] |

### Integration Points
- `teardownStopCCN(now, resultGeneralError)` — reused unchanged as the teardown.
- `notifyRouteObserverDown` / `collectKernelEventsLocked` — reused unchanged.
- `setHelloInterval` pattern in `reactor_setters.go` — mirrored by `setHelloRetries`.

### Architectural Verification
- [ ] No bypassed layers (DPD reuses `teardownStopCCN`, not a parallel teardown).
- [ ] No unintended coupling (reliable engine gains one read-only output field; it
      does not learn about DPD).
- [ ] No duplicated functionality (extends the existing HELLO/teardown paths).
- [ ] Zero-copy preserved where applicable (no new wire buffers; DPD is a timer).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A ZLB ACK of Ze's HELLO advances Nr and clears the HELLO from the engine's `rtmsQueue`, and `processNr` returns `acked > 0` for it | `reliable.go:451-477`, RFC 2661 §5.8 | DPD never resets for idle-but-alive peers → false teardowns | Unit test: feed a ZLB acking a sent HELLO, assert `lastLiveness` advanced | confirmed — `TestOnReceiveAckedCountSurfaced` + `TestDeadPeerZLBAckKeepsTunnelUp` PASS |
| A-2 | `lastActivity` is not updated by ZLB ACKs (so a second field is genuinely required) | `tunnel_fsm.go:61-67` comment + code | A second field would be redundant | Re-read confirmed; add regression test | confirmed — `TestDeadPeerZLBAckKeepsTunnelUp` asserts lastActivity unchanged while lastLiveness advances |
| A-3 | The HELLO trigger guard `Outstanding() == 0` means at most one HELLO is in flight, so `lastLiveness` aging is the right death signal during the idle keepalive phase | `reactor.go:505` | Multiple HELLOs could confuse counting (mitigated: design uses a timestamp, not a per-send counter) | Read confirmed | confirmed — design uses a timestamp, not a per-send counter; tests green |
| A-4 | Reusing `teardownStopCCN` from a DPD trigger correctly withdraws routes and emits tunnel-down (same as the retransmit-exhaustion path) | `reactor.go:490-543`, commit e231fbfdd | Routes leak on DPD teardown | Unit test asserting route withdraw on DPD teardown | confirmed — `TestDeadPeerKeepaliveTeardownWithdrawsRoute` asserts the route withdraw |
| A-5 | `hello-retries` default 2 keeps RFC-default deployments (60 s interval) tolerant (~120 s) while letting tuned deployments (5 s) detect in ~10 s (≈10 s margin under the 20 s evidence window) | task ("~2-3×"), evidence lab `hello-interval 5`, 20 s window, user decision | Default too aggressive → spurious teardowns; too lax → still fails the window | Functional/evidence test at `hello-interval 5` | confirmed (arithmetic + user decision); QEMU evidence run not executed locally |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | DPD reduces steady-state Established link-loss tolerance from 31 s to `hello-retries × hello-interval`; a transient blip longer than that tears down a live tunnel | tunnels flap on lossy links | Document as a tuning trade-off; default `hello-retries × hello-interval` ≫ 31 s at the RFC-default interval; gate strictly on `Established` so setup/teardown keep 31 s |
| R-2 | If `hello-retries × hello-interval > 31s` (e.g. default 3 × 60 s), the engine's 31 s exhaustion fires first and DPD adds nothing | detection still ~31 s at large intervals | Acceptable: large interval implies operator tolerates slow detection; document the interaction; the fast path engages exactly when configured for it |
| R-3 | Surfacing `Acked` on `ReceiveResult` touches the reliable engine's hot path | reliable unit tests change | Field is set from an already-computed value (`acked`); no new computation; cover with existing reliable tests |
| R-4 | Evidence window is 20 s with `hello-interval 5`; default `hello-retries 2` → ~10 s leaves ~10 s margin under QEMU jitter | evidence test flakes near boundary | Default 2 gives comfortable margin; functional/evidence config may tighten further; spec records the margin requirement |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `l2tp { hello-retries N; }` | → | `ExtractParameters` sets `Parameters.HelloRetries` | `TestExtractParameters_HelloRetries` (config_test.go) |
| SIGHUP reload changing `hello-retries` | → | `subsystem_reload.go` → `setHelloRetries` | `reload-hello-retries.ci` (mirrors `reload-hello-interval.ci`) |
| `handleTick` on an Established tunnel whose peer stopped ACKing HELLOs | → | DPD check → `teardownStopCCN` → route withdraw | `TestDeadPeerKeepaliveTeardownWithdrawsRoute` (reactor_test.go) |
| LAC process killed in the QEMU appliance | → | DPD teardown within the harness window | `scripts/evidence/effective-gokrazy-l2tp-ppp.py` (tightened) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Established tunnel; peer stops sending and stops ACKing HELLOs | Tunnel is torn down via `teardownStopCCN` after `hello-retries × hello-interval` from the last liveness signal, before the 31 s retransmit exhaustion |
| AC-2 | Established tunnel; peer is idle but answers every HELLO with a ZLB ACK | Tunnel is NOT torn down; `lastLiveness` is refreshed by each HELLO ACK even though `lastActivity` keeps aging |
| AC-3 | DPD-triggered teardown | Subscriber routes are withdrawn (`notifyRouteObserverDown`) and `(l2tp, tunnel-down)` is emitted with a dead-peer reason distinct from `retransmit-timeout` |
| AC-4 | `hello-retries` set to 0 | DPD is disabled; behavior is exactly today's (retransmit-exhaustion only) |
| AC-5 | Config `l2tp { hello-retries N; }` parsed | `Parameters.HelloRetries == N`; out-of-range rejected by YANG; default applied when unset |
| AC-6 | SIGHUP reload changes `hello-retries` | New value hot-applies via `setHelloRetries` (consistent with `hello-interval` reload semantics) |
| AC-7 | Reliable engine receives a ZLB acking a sent HELLO | `ReceiveResult.Acked > 0`; the 31 s retransmit schedule and retention window are unchanged |
| AC-8 | Tunnel in setup (pre-Established) or teardown (Closed) | DPD does not run; full 31 s reliable-retransmit budget is retained |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | LAC dies silently (no StopCCN) while a PPP session is up | wire silence → HELLO unanswered → `lastLiveness` ages → `handleTick` DPD → `teardownStopCCN` → routes withdrawn | `effective-gokrazy-l2tp-ppp.py` + `TestDeadPeerKeepaliveTeardownWithdrawsRoute` |
| 2 | Operator runs a quiet but healthy tunnel (no data, peer ACKs HELLOs) | HELLO sent → peer ZLB ACK → `Acked>0` → `lastLiveness` refreshed → no teardown | `TestDeadPeerIdleButAlivePeerStaysUp` |
| 3 | Operator tunes detection speed via config | `set l2tp hello-retries N` → reload → `setHelloRetries` → new threshold | `reload-hello-retries.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReliableOnReceive_AckedSurfaced` | `reliable_test.go` | ZLB acking a sent message yields `ReceiveResult.Acked > 0`; schedule unchanged (AC-7) | |
| `TestProcess_LastLivenessRefreshedByHelloAck` | `reactor_test.go`/`reactor_ppp_linux_test.go` | A ZLB ACK refreshes `lastLiveness` but not `lastActivity` (AC-2, A-1, A-2) | |
| `TestDeadPeerKeepaliveTeardownWithdrawsRoute` | `reactor_test.go` | DPD fires at `hello-retries × hello-interval`, before 31 s; routes withdrawn; tunnel-down reason set (AC-1, AC-3, A-4) | |
| `TestDeadPeerIdleButAlivePeerStaysUp` | `reactor_test.go` | Idle peer that ZLB-ACKs HELLOs is not torn down across multiple intervals (AC-2) | |
| `TestDeadPeerDisabledWhenRetriesZero` | `reactor_test.go` | `hello-retries 0` ⇒ no DPD teardown; only retransmit exhaustion (AC-4) | |
| `TestDeadPeerNotRunBeforeEstablished` | `reactor_test.go` | Pre-Established / Closed tunnels keep full 31 s budget (AC-8) | |
| `TestExtractParameters_HelloRetries` | `config_test.go` | Parse, default, range handling for `hello-retries` (AC-5) | |
| `TestReload_HelloRetries` | `subsystem_reload_test.go` | prev≠next triggers `setHelloRetries` (AC-6) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `hello-retries` | 0..255 (0 disables) | 255 | N/A (0 valid = disabled) | 256 (YANG uint8 rejects) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-hello-retries.ci` | `test/plugin/` | SIGHUP hot-applies a changed `hello-retries` (mirror of `reload-hello-interval.ci`) | |
| `l2tp-hello-retries-parse.ci` | `test/parse/` | Config parse + range validation for `hello-retries` | |
| (tighten) `effective-gokrazy-l2tp-ppp.py` | `scripts/evidence/` | LAC killed → subscriber routes withdrawn within the harness window with margin | |
| (tighten) interop `01-ppp-ipv4/check.py` | `test/l2tp-interop/` | LAC killed → withdrawal observed within window | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `01-ppp-ipv4` (existing, tighten) | `test/l2tp-interop/scenarios/` | xl2tpd | Peer death with no StopCCN is detected and routes withdrawn within the window | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/l2tp/config.go` - add `DefaultHelloRetries` const, `HelloRetries` field on `Parameters`, parse `hello-retries` from root `l2tp {}`.
- `internal/component/l2tp/reactor.go` - add `HelloRetries` to `ReactorParams`; in `handleTick`, add the Established-only DPD check that calls `teardownStopCCN` and uses a new tunnel-down reason.
- `internal/component/l2tp/subsystem.go` - map `Parameters.HelloRetries` onto `ReactorParams.HelloRetries` where the other timers are wired.
- `internal/component/l2tp/tunnel.go` - add `lastLiveness time.Time` field (doc its dual-signal purpose vs `lastActivity`).
- `internal/component/l2tp/tunnel_fsm.go` - in `Process`, refresh `lastLiveness` on delivered-or-acked; initialize `lastLiveness` at tunnel establishment.
- `internal/component/l2tp/reliable.go` - add `Acked int` to `ReceiveResult`, set it from `acked` in `OnReceive`.
- `internal/component/l2tp/reactor_setters.go` - add `setHelloRetries`.
- `internal/component/l2tp/subsystem_reload.go` - hot-apply `hello-retries` (prev/next compare → `setHelloRetries`).
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - add `hello-retries` leaf next to `hello-interval`.
- `scripts/evidence/effective-gokrazy-l2tp-ppp.py`, `test/l2tp-interop/scenarios/01-ppp-ipv4/check.py` - tighten/confirm teardown windows.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config leaf) | Yes | `internal/component/l2tp/yang/ze-l2tp-conf.yang` |
| YANG validation constraints | Yes | `type uint8 { range "0..255"; }` with default 2 |
| YANG custom validators | No | native range suffices |
| CLI commands/flags | No | config leaf is reached via the generic YANG editor |
| CLI grammar | No | no new verb |
| Editor autocomplete | Yes (automatic) | YANG type/default drives completion |
| Functional test for new RPC/API | Yes | `test/plugin/reload-hello-retries.ci`, `test/parse/l2tp-hello-retries-parse.ci` |
| Pipe completeness | No | no new output command |
| Env var registration | No | per `config-surface.md` this is a protocol timer ⇒ YANG only |
| Doctor check for runtime dependencies | No | no new file/socket/port/binary |
| Prometheus counters/metrics | Consider | optional `l2tp_tunnels_dead_peer_total` counter (see Key Design Decisions) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (dead-peer detection / `hello-retries`) |
| 2 | Config syntax changed? | Yes | `docs/guide/l2tp.md` (document `hello-retries`, interaction with `hello-interval`) |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | (HELLO/ZLB unchanged) verify `docs/architecture/wire/l2tp.md` keepalive prose |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2661.md` §15 keepalive note (DPD threshold) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` / `docs/architecture/testing/l2tp-interop.md` if windows change |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` (DPD vs accel-ppp/xl2tpd) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/l2tp.md` or l2tp subsystem doc (two liveness timestamps) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Only if counter added | `docs/plugin-development/metrics.md` |
| 15 | Registered event/reason changed? | Yes | document the new tunnel-down reason where reasons are listed |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep `docs/` for anchors on `reactor.go`/`reliable.go`/`tunnel_fsm.go` and update |
| 17 | Existing docs show config examples for this area? | Yes | verify `docs/guide/l2tp.md` examples |

## Files to Create
- `test/plugin/reload-hello-retries.ci` - SIGHUP hot-apply of `hello-retries`.
- `test/parse/l2tp-hello-retries-parse.ci` - parse + range validation.
- (unit tests added to existing `_test.go` files listed above)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review / fix / re-verify | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `HelloRetries` to `Parameters` and `ReactorParams`, `lastLiveness` to the tunnel, `Acked` to `ReceiveResult`, a stub DPD check in `handleTick`, and the failing wiring tests (`TestExtractParameters_HelloRetries`, `TestDeadPeerKeepaliveTeardownWithdrawsRoute`).
   - Verify: tests compile and fail because the DPD logic is a stub.
2. **Phase: Reliable ACK surfacing** — set `ReceiveResult.Acked` from `acked` in `OnReceive`; assert schedule/retention unchanged.
   - Tests: `TestReliableOnReceive_AckedSurfaced`.
3. **Phase: Liveness capture** — refresh `lastLiveness` in `Process` on delivered-or-acked; init at establishment.
   - Tests: `TestProcess_LastLivenessRefreshedByHelloAck`, `TestDeadPeerIdleButAlivePeerStaysUp`.
4. **Phase: DPD teardown** — implement the Established-only check in `handleTick`; reuse `teardownStopCCN`; new tunnel-down reason.
   - Tests: `TestDeadPeerKeepaliveTeardownWithdrawsRoute`, `TestDeadPeerDisabledWhenRetriesZero`, `TestDeadPeerNotRunBeforeEstablished`.
5. **Phase: Config + reload + YANG** — leaf, parse, default, setter, reload; `reload-hello-retries.ci`, `l2tp-hello-retries-parse.ci`.
6. **Phase: Functional/evidence** — tighten evidence + interop windows; confirm margin.
7. **RFC refs** — `// RFC 2661 Section 15` comment on the DPD trigger.
8. **Full verification + docs + learned summary.**

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with file:line |
| Feature completeness | Idle-but-alive peer survives (AC-2); dead peer detected fast (AC-1); setup/teardown unweakened (AC-8) |
| Correctness | DPD measured from `lastLiveness`, not `lastActivity`; reuses `teardownStopCCN` exactly |
| Naming | `hello-retries` kebab-case YANG; `HelloRetries` field; reason string distinct from `retransmit-timeout` |
| Data flow | reliable engine stays unaware of DPD; only surfaces `Acked` |
| YANG validation | `uint8` range with default; bare `type` is a red flag |
| Rule: no-layering | no parallel teardown path; engine schedule untouched |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `hello-retries` leaf | `grep hello-retries internal/component/l2tp/yang/ze-l2tp-conf.yang` |
| `Parameters.HelloRetries` + parse | `go test ./internal/component/l2tp -run TestExtractParameters_HelloRetries` |
| DPD teardown + route withdraw | `go test ./internal/component/l2tp -run TestDeadPeer` |
| `ReceiveResult.Acked` | `go test ./internal/component/l2tp -run TestReliableOnReceive_AckedSurfaced` |
| Reload | run `reload-hello-retries.ci` |
| Evidence window passes | run `scripts/evidence/effective-gokrazy-l2tp-ppp.py` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `hello-retries` bounded by YANG range; no overflow in `hello-retries × hello-interval` (use duration math, cap) |
| Resource exhaustion | DPD is a single timestamp compare per tick; no new allocation/goroutine |
| Liveness spoofing | A peer can keep a tunnel alive by ACKing HELLOs — expected (it is alive); replay of old Ns does not refresh `lastActivity` and only refreshes `lastLiveness` if it genuinely acks an outstanding message |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion/setup |
| Test fails behavior mismatch | Re-read Current Behavior; RESEARCH if misunderstood |
| Functional/evidence flake near window | Adjust test config margin (R-4), not the mechanism |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Hold-timer on `lastActivity` | ZLB ACKs do not refresh `lastActivity`; idle-but-alive peer would be falsely torn down | Separate `lastLiveness` refreshed by delivered-or-acked |
| Shorten reliable-engine retransmit (`maxRetransmit`/`rtimeoutCap`) | Weakens link-loss tolerance for setup/teardown; breaks RFC §5.8 retention | DPD path orthogonal to the engine; engine schedule untouched |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The bug is a missing liveness signal, not a wrong timer value. The system tracked
  "peer delivered a message" but not "peer acknowledged our keepalive." Dead-peer
  detection needs the latter; the HELLO trigger needs the former. Two signals,
  two fields.

## Core Insight
Liveness and activity are different facts. `lastActivity` (delivered-only)
answers "should I probe?"; `lastLiveness` (delivered **or** acked, incl. ZLB)
answers "is the peer dead?". Conflating them is exactly the bug both naive fixes
fall into.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Detect death via a `lastLiveness` timestamp refreshed by delivered-or-acked, torn down at `hello-retries × hello-interval` | (a) per-send "unanswered HELLO" counter; (b) hold-timer on `lastActivity`; (c) shorten reliable retransmit | (a) breaks against the `Outstanding()==0` single-HELLO guard and tick jitter; (b) false-kills idle-but-alive peers; (c) weakens setup/teardown. The timestamp is simplest, ACK-driven, and orthogonal to the engine. |
| Threshold leaf name `hello-retries` (uint8, default 2, 0=disabled) | `dead-peer-threshold`; `keepalive-retries`; absolute `dead-peer-timeout` (duration) | Pairs with `hello-interval`; expressing as a multiple of the interval matches the task's "2-3×" and avoids a second absolute timer to misconfigure. **CONFIRMED by user 2026-06-26.** |
| Default `hello-retries = 2` | 3 (looser); 5 (RFC-ish) | 2 detects at ~10 s for tuned deployments (5 s interval) — ~10 s margin under the 20 s evidence window — and stays tolerant (~120 s) at the RFC-default 60 s interval. **CONFIRMED by user 2026-06-26.** |
| Gate DPD strictly on `L2TPTunnelEstablished` | run in all states | Preserves the 31 s reliable budget for setup (pre-Established) and teardown (Closed), per the task's explicit constraint |
| Reuse `teardownStopCCN` + `notifyRouteObserverDown` | new teardown path | Commit e231fbfdd already routes withdrawal through this chain; reuse avoids divergence |
| New tunnel-down reason (e.g. `keepalive-timeout`) distinct from `retransmit-timeout` | reuse `retransmit-timeout` | Operators must distinguish "peer ignored keepalives" from "control message lost"; observability |

## Known Limitations
- DPD reduces steady-state Established link-loss tolerance to
  `hello-retries × hello-interval`. This is the intended, configurable trade-off
  of dead-peer detection; setup and teardown retain the full 31 s budget.
- When `hello-retries × hello-interval > 31s` (e.g. defaults 2 × 60 s = 120 s), the
  reliable engine's 31 s exhaustion fires first and DPD does not accelerate
  detection. The fast path engages only when configured for it.

## RFC Documentation
- `// RFC 2661 Section 15: "HELLO ... is used as a keepalive ..."` above the DPD
  trigger in `handleTick`.
- `// RFC 2661 Section 5.8` retained on the reliable engine retransmit/retention
  code (unchanged) — note in review that DPD does not alter it.

## Implementation Summary

### What Was Implemented
- `reliable.go`: `ReceiveResult.Acked` field, populated from `processNr` in all
  post-ack `OnReceive` return paths (ZLB, delivered, duplicate, reorder,
  discard). Retransmit schedule/retention untouched.
- `tunnel.go`: new `lastLiveness` field (delivered-or-acked clock).
- `tunnel_fsm.go`: `Process` refreshes `lastLiveness` on
  `len(Delivered) > 0 || Acked > 0`; `handleSCCCN` seeds it at establishment.
- `reactor.go`: `ReactorParams.HelloRetries`; `handleTick` Established-only
  dead-peer check (`now - lastLiveness >= HelloRetries * HelloInterval`)
  reusing `teardownStopCCN`; tunnel-down reason `keepalive-timeout`; operator
  log "dead peer; keepalive timeout teardown".
- `config.go` + `subsystem_reload.go`: `DefaultHelloRetries = 2`,
  `Parameters.HelloRetries`, parse `hello-retries` in BOTH parsers
  (`ExtractParameters` and `extractFromProvider`).
- `reactor_setters.go` + `subsystem_reload.go`: `setHelloRetries` + hot-apply.
- `subsystem.go`: map `Parameters.HelloRetries` -> `ReactorParams.HelloRetries`.
- `yang/ze-l2tp-conf.yang`: `hello-retries` leaf (uint8 0..255, default 2).
- Observability: `ConfigSnapshot.HelloRetries`, `show l2tp config` JSON key,
  web `/l2tp` config-form row.
- Tests: 1 reliable unit (`TestOnReceiveAckedCountSurfaced`), 4 reactor units
  (`TestDeadPeer*`), 1 config (`TestConfig_HelloRetries`), 1 reload
  (`TestReloadAppliesHelloRetries`), 2 parse `.ci`, 1 plugin reload `.ci`.

### Bugs Found/Fixed
- None new. The fix closes the "Ze not acting on peer death promptly" issue
  noted in commit e231fbfdd.

### Documentation Updates
- `docs/guide/l2tp.md`: config example, reload table row, new "Dead-peer
  detection" section (lastActivity vs lastLiveness).
- `docs/features.md`: L2TP row dead-peer detection clause + source anchor.
- `docs/functional-tests.md`: two parse-test rows.
- `rfc/short/rfc2661.md` §15: Ze dead-peer threshold note + source anchors.

### Deviations from Plan
- Default set to 2 (not the spec's initial recommendation of 3) per user
  decision 2026-06-26 — gives ~10s detection at `hello-interval 5`, ~10s
  margin under the 20s evidence window.
- Added observability surfaces (ConfigSnapshot/show-config/web row) beyond the
  original Files-to-Modify list, for discoverability and to let the reload
  `.ci` observe the value.
- Evidence script `effective-gokrazy-l2tp-ppp.py` not modified: with default
  `hello-retries 2` and its `hello-interval 5`, detection is ~10s, already
  within the existing 20s window (no tightening required).

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Detect a silently-dead LAC before the ~31s retransmit exhaustion and withdraw routes | unit test | `TestDeadPeerKeepaliveTeardownWithdrawsRoute` PASS (single tick at +11s, `hello-retries 2` x `hello-interval 5s`, tears down + withdraws route + logs keepalive-timeout). Evidence harness `effective-gokrazy-l2tp-ppp.py` (hello-interval 5, default retries 2 → ~10s) sits within its 20s window; QEMU run not executed locally |
| Do not tear down idle-but-alive peers | unit test | `TestDeadPeerZLBAckKeepsTunnelUp` PASS (peer only ZLB-ACKs HELLOs over 12s > 10s deadline; lastLiveness refreshed, lastActivity NOT refreshed, tunnel stays Established) |
| Do not weaken setup/teardown link-loss tolerance | unit test | `TestDeadPeerNotRunBeforeEstablished` PASS (pre-Established tunnel with stale lastLiveness not torn down); reliable schedule tests (`reliable_seq_test.go`, `TestOnReceiveAckedCountSurfaced`) unchanged/green |

## Review Gate

### Run 1 (manual adversarial self-review; /ze-review skill not invocable in this session)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | DPD reduces steady-state Established link-loss tolerance to `hello-retries x hello-interval` | reactor.go handleTick | Acknowledged; documented as the intended trade-off (Known Limitations, R-1) |
| 2 | NOTE | `reactor.go` is 804 lines (>600 review threshold, <1000 split) | reactor.go | Acknowledged; +~30 lines this change; no split warranted (<1000) |

Checked and cleared (no issue): duration overflow (`uint8 x <=3600s` fits int64);
`deadPeer` precedence vs `result.TeardownRequired` (DPD only when not already
tearing down); `!lastLiveness.IsZero()` guard; all reads/writes of
`lastLiveness` under `tunnelsMu` (passes `-race`); `Acked` set in every post-ack
return path; data-message early return leaves `Acked=0` (Nr untrusted).

### Fixes applied
- None required (only NOTEs).

### Final status
- [ ] Manual adversarial self-review shows 0 BLOCKER, 0 ISSUE (NOTEs only)
- [ ] All NOTEs recorded above

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/l2tp/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `hello-retries`
- [ ] Functional tests for end-to-end behavior
- [ ] Interop test confirms peer-death detection
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-l2tp-dead-peer-detection.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-l2tp-dead-peer-detection.md` only
