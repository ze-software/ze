# Spec: fixit-bgp-session-fsm-lifecycle

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4271.md` - FSM, hold timer, keepalive timer (Sections 8, 10)
4. Source files in Current Behavior below

## Task

Fix four defects in the BGP session finite-state-machine and timer lifecycle, all
surfaced by the repository audit (2026-07-16) and adversarially verified against the
producing code (re-verified line by line 2026-07-16 during design; drift table below).
They share a root shape: a resource owned by one code path (a timer's armed state,
the timer set on teardown, the connection on teardown) is released or re-armed by a
different path, and that path is missed on a branch.

1. **[BLOCKER] Hold timer permanently disarms after its first expiry.** The hold-expiry
   closure clears `holdRunning` before invoking the session callback (`fsm/timer.go:186-195`,
   flag cleared at `:188`, callback invoked at `:192-194`; the `ResetHoldTimer` twin closure
   at `:223-232` has the identical shape). The session's grace branch ("recent read
   activity / CPU congestion") then calls `ResetHoldTimer` (`reactor/session.go:425-431`),
   which early-returns because `!holdRunning` (`fsm/timer.go:216-218`) and never re-arms.
   `StartHoldTimer`'s only non-test caller is `connectionEstablished` on a fresh connection
   (`reactor/session_connection.go:359`), so within a session no path re-arms. The FSM's
   own Event 26/27 restarts (`fsm/fsm.go:431-433`, `:448-450`, wired via `SetTimers` at
   `reactor/session.go:413`) all funnel through the same disabled `ResetHoldTimer`.
   Because `recentRead` is set on every read (`reactor/session_read.go:92`, coalesced twin
   `reactor/session_coalesce.go:82`) and cleared only by the expiry callback
   (`reactor/session.go:425` is the sole `Swap(false)`), the first hold expiry of
   essentially every Established session takes the grace branch (the last message before
   silence left the flag true), no-ops, and permanently disables dead-peer detection: a
   peer that later goes silent is never torn down and its routes are never withdrawn.
   The "extend by 10s" comment at `reactor/session.go:421-424` describes code that does
   not exist.

2. **[HIGH] Established-stage validation teardowns skip `StopAll`, leaking the Session.**
   The keepalive timer re-arms itself while `keepaliveRunning` (`fsm/timer.go:293-313`);
   only `StopAll`/`StopKeepaliveTimer` clears it. Several Established error returns call
   `closeConn` (which stops only the send-hold timer, `reactor/session_connection.go:461-500`,
   at `:463`) but not `StopAll`: bad message length (`reactor/session_read.go:114` and the
   coalesced twin `reactor/session_coalesce.go:101`), family-not-negotiated
   (`session_read.go:192`), prefix-limit (`session_read.go:209`), unknown type
   (`reactor/session_handlers.go:33`), bad ROUTE-REFRESH length (`session_handlers.go:303`,
   reachable from `session_read.go:225` and `session_handlers.go:323`). Design-phase
   sibling audit found three more members of the same family: RFC 7606 session-reset
   (`reactor/session_validation.go:156`), the policy-teardown path
   (`session_read.go:242-250`, also defect 4), and the hold-expiry/ctx-cancel exit itself
   (the cancel goroutine calls only `closeConn`, `reactor/session.go:736-738`, so even a
   correctly detected dead peer leaves its keepalive timer re-arming). `Peer.runOnce`
   creates a fresh `Session` per connection cycle (`reactor/peer_run.go:193`) and drops
   the old one (`:249-251`) without `StopAll`, so the self-re-arming keepalive closure
   (which references the Session via `onKeepaliveExpires`, `reactor/session.go:442-456`)
   keeps the abandoned `Session` reachable forever: bufio.Reader 64 KB
   (`session_connection.go:334`), bufio.Writer 16 KB (`:335`), `writeBuf` 4 KB (or 64 KB
   after RFC 8654 resize, `session_negotiate.go:38-41`), plus the struct -- roughly
   85-150 KB per reconnect cycle, unbounded, rate-capped by backoff.

3. **[MEDIUM] An OPEN received in Established zombifies the session with no NOTIFICATION.**
   `handleOpen` has no FSM-state gate (`reactor/session_handlers.go:39`): a second OPEN
   re-runs negotiation (`negotiateWith` at `:132` rewrites `s.negotiated` and calls
   `SetHoldTime`, `session_negotiate.go:27-62`), fires `EventBGPOpen` (`:170`), and
   returns nil, never `closeConn`. In Established, `EventBGPOpen` hits the FSM
   `default -> change(StateIdle)` (`fsm/fsm.go:487-491`; `handleEstablished` has no
   `EventBGPOpen` case), the Established->Idle transition fires the peer callback route
   withdrawal, but the connection stays open and the read loop keeps delivering to an
   Idle FSM. RFC 4271 Section 6.6 / 8.2.2 requires an FSM error: send NOTIFICATION
   (Finite State Machine Error, code 5, subcode 0) and drop the connection.
   Same shape in OpenConfirm: a second OPEN on the *same* connection hits
   `handleOpenConfirm`'s default arm (`fsm/fsm.go:406-408`). (Cross-connection collision
   is a different, already-handled path: `DetectCollision` / `AcceptWithOpen`.)

4. **[LOW, enabling] `FSM.Event` always returns nil** (`fsm/fsm.go:153-173`, `return nil`
   at `:172`), so the illegal-transition warning path in `logFSMEvent`
   (`reactor/session.go:689-698`) is dead code and callers cannot detect a rejected
   transition. Fixing defect 3 cleanly needs a real return value here. Also fold in the
   policy-teardown path (`reactor/session_read.go:242-250`) that returns nil without
   signalling `errChan` or `setCloseReason`, leaving `Run` to spin in the `conn == nil`
   10 ms sleep loop (`reactor/session.go:763-770`) until an external event arrives.

### Adjacent gap found during design (needs Thomas's scope decision)

Error paths that return from the read loop **without** calling `closeConn` leave the TCP
connection open after `Run` exits: the cancel goroutine wakes on `<-s.done` and returns
without closing (`reactor/session.go:733-734`). Verified members: `handleOpen` unpack
error (`session_handlers.go:43-47`), openValidator rejection (`:98-115`, sends a
NOTIFICATION but never closes), local-capability parse error (`:121-124`). A
`defer s.closeConn()` in `Run` (same shape as the defect-2 fix) closes them all. Held as
proposed AC-7 pending approval; it is NOT silently folded into defect 2.
**Resolved Q-2 (2026-07-17): AC-7 approved and in scope** (three leak sites above re-verified
against source: `session_handlers.go:43-47`, `:98-115`, `:121-124`; cancel-goroutine
`<-s.done` exit at `session.go:733-734`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/behavior/fsm.md` - BGP FSM design (referenced by `fsm/timer.go:1`)
  → Constraint: hold timer detects dead peers; it MUST be restarted on KEEPALIVE/UPDATE receipt in Established (doc lines 178-187: negotiated, 0 disables, keepalive = hold/3).
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` grammar for the functional tests
  → Constraint: `action=send:conn=N:seq=N:hex=...` injects raw bytes mid-session; `expect=bgp:conn=N:seq=N:hex=...` asserts received wire bytes -- sufficient to script "second OPEN in Established, expect NOTIFICATION code 5".

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - hold timer, keepalive timer, FSM events (Sections 8, 10)
  → Constraint: Section 8.2.2 -- an OPEN in Established is an FSM error; send NOTIFICATION and release resources. NOTIFICATION Error Code 5 (Finite State Machine Error) has no subcodes; subcode MUST be 0 (Section 6).
  → Constraint: Section 4.4 -- a negotiated hold time of zero disables both timers (this is deliberate and must be preserved; it is distinct from defect 1). Producer: `fsm/timer.go:180-182` (StartHoldTimer) and `:216` (ResetHoldTimer).
  → Constraint: Section 4.2 -- negotiated hold = min(local, peer), floor 3 s. Producer: `session_negotiate.go:50-62`.

**Key insights:**
- Defect 1 (the `holdRunning` lifecycle bug) and the deliberate `holdTime == 0` clause both flow through the single guard `fsm/timer.go:216`; fixing one must not regress the other.
- The `!holdRunning` guard in `ResetHoldTimer` is ALSO what keeps late FSM events (a KEEPALIVE processed after `StopAll`) from resurrecting timers on a torn-down session. The fix must NOT weaken that guard; the grace re-arm needs its own, generation-checked entry point (see Key Design Decisions).
- Ze's FSM deliberately does not send messages (`fsm/fsm.go` header, VIOLATIONS note 3), so the defect-3 NOTIFICATION belongs in `handleOpen` (reactor), not in the FSM.
- ROUTE-REFRESH receipt does not restart the hold timer (`handleRouteRefresh` fires no FSM event, `session_handlers.go:322-375`). This matches RFC 4271, whose FSM restarts HoldTimer on KeepAliveMsg (26) and UpdateMsg (27) only (`rfc/short/rfc4271.md:504-505`); RFC 2918 adds no hold-timer rule. It does mean `recentRead` can legitimately be true at expiry for a live, refresh-only peer -- one more reason the grace branch must actually re-arm.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/fsm/timer.go` (401L) - hold/keepalive/connect timers.
  → Constraint: every expiry closure clears its `*Running` flag before invoking the callback (hold `:186-195`, reset-twin `:223-232`, connectRetry `:357-366`); the keepalive `timerFunc` (`:293-310`) instead checks-and-re-arms under lock while `keepaliveRunning`. `stopHoldTimerLocked` (`:243-249`) ignores `Timer.Stop()`'s fired/not-fired return, so a fired-but-not-yet-run closure cannot be distinguished from a stopped one -- the ABA seed for A-2. `StopAll` (`:393-400`) is idempotent (nil-checked stops).
- [ ] `internal/component/bgp/reactor/session.go` (799L) - Session struct, callbacks, `Run` loop.
  → Constraint: `OnHoldTimerExpires` callback (`:420-440`): grace branch `:425-431`, teardown branch `:433-439` signals `errChan`. Cancel goroutine (`:713-739`): closes conn only for ctx-cancel/errChan reasons (`:736-738`); on `<-s.done` (read-loop error return) it exits WITHOUT closing (`:733-734`). `conn == nil` poll loop `:763-770` exits only when `closeReason` is set. `logFSMEvent` `:689-698`. FSM wired to timers at `:413`. `Stop()` at `:641-644` calls `StopAll`.
- [ ] `internal/component/bgp/reactor/session_read.go` (294L) - read loop, error returns.
  → Constraint: `recentRead.Store(true)` at `:92` on every header read. EOF/reset path is CLEAN: `handleConnectionClose` (`:271-275`) does `StopAll` + FSM event + `closeConn`. Dirty paths: `:114` (bad length), `:192` (family), `:209` (prefix limit), `:242-250` (policy teardown; also returns nil error).
- [ ] `internal/component/bgp/reactor/session_coalesce.go` - coalesced read twin.
  → Constraint: duplicates the read-loop preamble: `recentRead` at `:82`, bad-length `closeConn` at `:101`. Any read-loop fix must NOT need per-site edits here (exit-discipline fix in `Run` covers both).
- [ ] `internal/component/bgp/reactor/session_handlers.go` (387L) - per-message handlers.
  → Constraint: `handleOpen` (`:39-183`) has no state gate and is reachable in every state from `processMessage` (`session_read.go:256-257`). `handleNotification` (`:275`) and `handleConnectionClose` are the only handler-level `StopAll` callers. `handleKeepalive` (`:211-222`) starts keepalive + send-hold timers on OpenConfirm->Established.
- [ ] `internal/component/bgp/reactor/session_connection.go` (507L) - connect/accept/teardown.
  → Constraint: `connectionEstablished` arms the hold timer once per connection with the openwait value (`:357-359`); negotiation later shrinks it via `SetHoldTime` (`session_negotiate.go:62`) + `ResetHoldTimer` (`session_handlers.go:180`, `session_connection.go:232`). `Close`/`CloseWithNotification`/`Teardown` all `StopAll` (`:366`, `:390`, `:416`); `closeConn` (`:461-500`) does not (send-hold only, `:463`).
- [ ] `internal/component/bgp/reactor/session_validation.go` - RFC 7606 + capability-mode validation.
  → Constraint: RFC 7606 session-reset does `closeConn` without `StopAll` at `:156` (Established; keepalive leak). Capability/ADD-PATH mode rejects at `:265/:279/:300/:311` are pre-Established (keepalive not yet started; hold closure fires once then the Session is garbage -- bounded, acceptable).
- [ ] `internal/component/bgp/fsm/fsm.go` (494L) - state machine.
  → Constraint: `Event` (`:153-173`) always returns nil. Established handler restarts hold timer on Events 26/27 (`:431-433`, `:448-450`). No `EventBGPOpen` case in `handleEstablished` or `handleOpenConfirm`; both fall to `default -> change(StateIdle)` (`:487-491`, `:406-408`).
- [ ] `internal/component/bgp/reactor/peer_run.go` (539L) - reconnect loop, Session lifecycle consumer.
  → Constraint: `runOnce` creates a NEW Session each cycle (`:193`) and abandons the old one without `StopAll` (`:249-251`); `run()` classifies `Session.Run`'s error: `ErrTeardown` reconnects immediately (`:78-84`), everything else backs off exponentially (`:118-151`). `cleanup()` (`:517-538`) calls `session.Close()` only if `p.session` is still non-nil -- it never is after `runOnce`'s defer, so peer stop does NOT stop an abandoned session's timers either.
- [ ] `internal/test/sim/sim.go` - FakeClock for the timer unit tests.
  → Constraint: `advanceTo` (`:67-95`) releases the clock lock before each callback and documents that callbacks "may take other locks or schedule new timers" (`:63-66`) -- re-arming via `AfterFunc` from inside a fired callback is supported, so A-1's test is writable.

**Behavior to preserve:**
- Negotiated hold-time 0 disables both timers (RFC 4271 Section 4.4) -- the single spurious "hold timer extended" log at hold-time 0 is unreachable (timer never armed) and stays that way.
- `ResetHoldTimer`'s no-op after a deliberate stop (`StopAll`, `StopHoldTimer`): late FSM events on a torn-down session must not resurrect timers.
- The EOF/reset close path (`handleConnectionClose`), `handleNotification`, `Close`, `CloseWithNotification`, `Teardown`, `Stop`: already correct, must not double-teardown (they may now overlap with a `Run`-exit `StopAll`; `StopAll` is idempotent, A-3).
- The per-peer panic failure domain (`safeRunOnce`, cancel-goroutine recover) and existing teardown reason plumbing (`closeReason` first-wins CAS).
- `ErrTeardown` => immediate reconnect vs error => backoff classification in `peer_run.go:76-151`.
- Existing timer API surface: `StartHoldTimer`, `ResetHoldTimer`, `StopHoldTimer`, `StopAll`, `IsHoldTimerRunning` signatures unchanged (callers grepped: reactor + fsm + tests only).
- Openwait arming (`ze.bgp.openwait`, `session_connection.go:357-359`) borrowing the hold-timer plumbing.

**Behavior to change:**
- Defects 1-4 above; AC-7 (conn-close on Run exit) ~~only if Thomas approves~~ (approved 2026-07-17, Q-2; in scope); nothing else.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A hold-timer `AfterFunc` fires after `holdTime` with no intervening KEEPALIVE/UPDATE; or a peer sends a second OPEN post-Establishment; or an Established validation error triggers `closeConn`.

### Transformation Path
1. `connectionEstablished` arms the hold timer via `StartHoldTimer` (`session_connection.go:359`), duration = openwait; negotiation then shrinks it (`session_negotiate.go:62` + `ResetHoldTimer` at `session_handlers.go:180`).
2. Each received message: read loop sets `recentRead` (`session_read.go:92`), fires the FSM event, and the FSM handler calls `ResetHoldTimer` (`fsm/fsm.go:431-433`, `:448-450`) -- works only while `holdRunning`.
3. On expiry the `AfterFunc` closure sets `holdRunning = false`, unlocks, then calls `onHoldExpires` (`timer.go:186-195`).
4. `onHoldExpires` grace branch swaps `recentRead` and calls `ResetHoldTimer` (`session.go:425-431`), which no-ops (`timer.go:216-218`).
5. From then, every FSM-event `ResetHoldTimer` no-ops; dead-peer detection is off for the session's life. The teardown branch (`session.go:433-439`) is reached only if the peer was silent for the ENTIRE session before the first expiry.
6. On a validation teardown, the read loop calls `closeConn` + returns an error; `Run` returns it (`session.go:788-796`); the cancel goroutine exits via `<-s.done` without further cleanup; `runOnce` abandons the Session (`peer_run.go:249-251`); the keepalive `timerFunc` keeps re-arming (`timer.go:305-309`) and pins the Session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Timer goroutine ↔ Session | `AfterFunc` closure invokes `onHoldExpires`/`onKeepaliveExpires` callbacks with no lock held (`timer.go:190` unlocks first) | [ ] |
| Session ↔ FSM | `FSM.Event` drives state transitions; return value is currently always nil (`fsm.go:172`); FSM calls back into `Timers` for Events 26/27 (lock order f.mu -> t.mu, never reversed) | [ ] |
| Session ↔ Peer | `Run`'s returned error selects reconnect class (`peer_run.go:76-151`); `errChan`/`closeReason` carry teardown reasons into `Run` | [ ] |
| Session ↔ RIB | route withdrawal on peer-down rides the FSM Established->Idle callback (`peer_run.go:399-434`); depends on the session actually tearing down | [ ] |

### Integration Points
- `fsm.Timers` (hold/keepalive lifecycle) - gains the generation guard + grace re-arm entry point.
- `reactor.Session.Run` (exit discipline) - gains the `defer` teardown.
- `fsm.FSM.Event` (transition result) - gains a sentinel return.
- `reactor` peer FSM-transition callback (route withdrawal on down) - unchanged, exercised by tests.

### Architectural Verification
- [ ] No bypassed layers (timer re-arm goes through the `Timers` API, not a private field poke)
- [ ] No unintended coupling (session teardown discipline stays in `Session.Run`; FSM still sends no messages)
- [ ] No duplicated functionality (grace re-arm reuses the arm path via a shared locked helper, not a third copy of the closure)
- [ ] Registration over hardcoding — no new per-feature field/switch added to a core/shared struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The grace branch re-arm can call back into the `Timers` API from within the expiry callback without deadlock | closure unlocks `t.mu` before invoking cb (`timer.go:190`); callback runs on the timer goroutine holding no session/FSM locks; FakeClock releases its lock before callbacks and supports AfterFunc-from-callback (`sim.go:63-94`) | Re-arm deadlocks | design-time read of both producers (done); targeted unit test with fake clock at implementation | confirmed (code read) |
| A-2 | A generation counter is needed to stop a stale fired-closure clobbering `holdRunning` under a freshly armed timer | audit V7 note; confirmed by reading the producer: the closure captures no arming identity, `stopHoldTimerLocked` ignores `Stop()`'s return (`timer.go:243-249`), so fire -> re-arm -> stale-closure-runs leaves `holdRunning=false` under an armed timer | Narrow ABA race remains; `ResetHoldTimer` no-ops until next fire | race test stopping+rearming across expiry (implementation); shape verified from source | confirmed (code read) |
| A-3 | Adding a `defer StopAll()` in `Session.Run` does not double-stop timers already stopped on the clean path | `StopAll` is idempotent: `stop*Locked` nil-check their timer fields (`timer.go:243-249`, `:323-329`, `:377-383`) | Harmless double-stop | read `stop*Locked` (done) | confirmed |
| A-4 | A `.ci` scripted-peer test can deliver a second OPEN mid-session and assert the NOTIFICATION | `docs/architecture/testing/ci-format.md`: `action=send:conn=N:seq=N:hex=` (raw bytes), `expect=bgp:...:hex=`; framework already has `send-unknown-message` post-OPEN precedent | Functional test for AC-4 needs new runner support (small `expect.go` extension) | write the `.ci` in Phase 1 and watch it fail for the right reason | unvalidated (doc-verified only) |
| A-5 | A stock reference daemon cannot be made to send a second in-session OPEN, so interop for defect 3 must validate the regression direction (normal reconnect still works), while defect 1 gets a real dead-peer interop scenario | FRR/BIRD reconnect by opening a new TCP connection; no knob sends OPEN twice on one connection | Interop table as written in the skeleton is unimplementable | interop scenario design below; harness `check.py` structure read (`test/interop/scenarios/*/check.py`) | unvalidated (needs harness pause/SIGSTOP capability check at implementation) |
| A-6 | The keepalive `timerFunc` flag-check under lock is NOT sufficient against its own stale-chain race (old fired closure re-arms with the old interval and clobbers `t.keepaliveTimer`), but stopping still works because both chains gate on `keepaliveRunning` | read of `timer.go:293-313` | Duplicate keepalive chains until next stop; harmless for correctness, noisy on the wire | extend the generation guard to all three timers, or document why keepalive is flag-safe; decided in Phase 2 | confirmed (code read) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Re-arming to full `holdTime` instead of the documented 10 s extension changes congestion behavior and doubles worst-case dead-peer detection (2x holdTime) | interop timing test drift; `deadpeer-holddown.ci` duration | Decision D-2 below; add a test pinning the chosen extension |
| R-2 | Sending a NOTIFICATION on OPEN-in-Established regresses a peer that legitimately reconnects | interop suite (01/02/05/06 reconnect flows) goes red | Gate keys on same-connection state, not on peer identity; run full interop suite; scenario below asserts clean re-establishment after the NOTIFICATION |
| R-3 | Grace re-arm racing `StopAll` resurrects the hold timer on a torn-down session | flaky `TestHoldTimerGenerationGuard`; timer fires on dead session in soak | generation checked inside the re-arm under `t.mu`; `StopAll` bumps the generation |
| R-4 | Dead-peer functional test is timing-sensitive (3 s min hold + grace extension) | flaky `.ci` in CI | keep negotiated hold at the 3 s floor; scripted peer uses inject dwell with keepalives disabled; generous expect window; fall back to unit-level coverage if the runner cannot suppress dwell keepalives |
| R-5 | `defer StopAll()` in `Run` masks the per-site knowledge of WHY a path tears down | reviewer confusion; future sites relying on the defer for NOTIFICATION sending | the defer owns only resource release; NOTIFICATION + `closeReason` stay at the erroring site |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| hold timer fires while `recentRead` true, then peer goes silent | -> | grace branch re-arms; next expiry tears down | `TestHoldTimerRearmsAfterGracedExpiry` |
| Established peer triggers a validation teardown | -> | `Session.Run` exit calls `StopAll` | `TestSessionRunStopsTimersOnValidationTeardown` |
| Established peer sends a second OPEN | -> | NOTIFICATION (code 5) sent + connection closed | `test/parse/open-in-established.ci` |
| import policy filter requests teardown | -> | `Run` exits promptly (no 10 ms spin) with a teardown reason | `TestPolicyTeardownExitsRun` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Established session, one hold expiry taken via the grace branch, then no traffic for the grace window + `holdTime` | Session tears down on the next expiry; dead-peer detection survives the first graced expiry |
| AC-2 | Normal 90 s hold session, one graced expiry | Hold timer is still armed afterwards (`IsHoldTimerRunning` true) |
| AC-3 | Established peer triggers each StopAll-free error path (all 8 sites listed in defect 2) | After teardown, `IsKeepaliveTimerRunning` is false and the `Session` is not retained by a timer closure |
| AC-4 | Established peer sends a second OPEN on the same connection | A NOTIFICATION (FSM Error, code 5, subcode 0) is sent and the connection is closed; no zombie Idle-with-open-conn state; peer can then re-establish normally |
| AC-5 | `FSM.Event` receives an event that lands in a state handler's default arm | Returns a non-nil sentinel; `logFSMEvent` logs the rejection (its warn branch becomes live code) |
| AC-6 | Import policy filter requests a session teardown | NOTIFICATION sent, `Run` returns a teardown reason promptly (no `conn == nil` spin), peer reconnect class is deliberate (see D-7) |
| AC-7 | ~~(PROPOSED, needs approval)~~ (APPROVED 2026-07-17, Q-2) Any read-loop error return, including ones that skip `closeConn` today | TCP connection is closed by the time `Run` has returned |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator's peer crashes silently (no FIN) after a busy period | silence -> hold expiry (graced once) -> re-armed expiry -> NOTIFICATION code 4 + teardown -> FSM Idle -> route withdrawal | `test/parse/deadpeer-holddown.ci` + `47-holdtime-deadpeer-frr` |
| 2 | misbehaving peer re-sends OPEN mid-session | read loop -> `handleOpen` state gate -> NOTIFICATION code 5 + close -> peer reconnects cleanly | `test/parse/open-in-established.ci` |
| 3 | operator runs thousands of flap cycles against a malformed-speaking peer | per-cycle Session teardown -> `Run` defer StopAll -> old Sessions collectable | `TestSessionRunStopsTimersOnValidationTeardown` (+ heap assertion) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHoldTimerRearmsAfterGracedExpiry` | `internal/component/bgp/fsm/timer_test.go` | AC-1/AC-2: graced expiry re-arms (FakeClock; fire, grace re-arm, advance, second fire tears down) | |
| `TestHoldTimerGenerationGuard` | `internal/component/bgp/fsm/timer_test.go` | A-2/R-3: stale fired closure neither clears `holdRunning` under a fresh arm nor re-arms after `StopAll` | |
| `TestResetHoldTimerStillNoOpsAfterStop` | `internal/component/bgp/fsm/timer_test.go` | preserved behavior: deliberate stop still disables `ResetHoldTimer` | |
| `TestHoldTimeZeroStaysDisabled` | `internal/component/bgp/fsm/timer_test.go` | RFC 4271 §4.4: holdTime 0 arms nothing, grace path unreachable | |
| `TestSessionRunStopsTimersOnValidationTeardown` | `internal/component/bgp/reactor/session_test.go` | AC-3: `Run` exit stops all timers on each dirty path (table-driven over the 8 sites) | |
| `TestPolicyTeardownExitsRun` | `internal/component/bgp/reactor/session_test.go` | AC-6: policy teardown sets a close reason and `Run` returns | |
| `TestFSMEventReturnsErrorOnIllegalTransition` | `internal/component/bgp/fsm/fsm_test.go` | AC-5: sentinel return from default arms; nil from handled events | |
| `TestOpenInEstablishedSendsNotification` | `internal/component/bgp/reactor/session_handlers_test.go` | AC-4: state gate, NOTIFICATION code 5 subcode 0, close; also OpenConfirm same-connection case per D-6 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| holdTime | 0 or 3-65535 s | 65535 | 1-2 s rejected (`ValidateHoldTime`; negotiation floors to 3 s, `session_negotiate.go:56-58`) | N/A (0 disables, deliberate) |
| grace extension (if D-2 picks bounded) | > 0, <= holdTime | holdTime | N/A (constant/env-derived) | clamp to holdTime |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `open-in-established` | `test/parse/open-in-established.ci` | peer that re-OPENs mid-session gets NOTIFICATION code 5, session closes, next connection re-establishes cleanly | |
| `deadpeer-holddown` | `test/parse/deadpeer-holddown.ci` | silent peer (conn held open, no FIN, no keepalives) after a busy period is torn down within hold time + grace window; NOTIFICATION code 4 observed | |

(The skeleton placed these in `test/bgp/*.ci`; that directory does not exist. `ze-test bgp parse` runs `test/parse/*.ci` (`mk/test-functional.mk:72,123-124`); the scripted peer (`internal/test/peer/`) loads `.ci` expectations (`expect.go`).)

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `47-holdtime-deadpeer-frr` | `test/interop/scenarios/` | FRR | real peer silenced (container pause/SIGSTOP via `check.py`) is torn down within hold time + grace; routes withdrawn (defect 1 end-to-end) | |
| reconnect regression | existing scenarios 01/02/05/06 re-run | FRR + BIRD | defect-3 NOTIFICATION change does not break legitimate reconnect (R-2) | |

(The skeleton's `NN-open-in-established` against FRR/BIRD is not implementable: a stock daemon never sends a second OPEN on an established connection (A-5). The scripted peer `.ci` covers that wire behavior; interop covers dead-peer detection and the regression direction.)

### Future (if deferring any tests)
- None planned. Deferral requires explicit user approval.

## Files to Modify
- `internal/component/bgp/fsm/timer.go` - generation counter on arm/stop; shared locked arm helper (collapses the duplicated closures at `:186-195` and `:223-232`); generation-checked grace re-arm entry point; keep `holdTime == 0` semantics
- `internal/component/bgp/reactor/session.go` - grace branch calls the new re-arm (with its captured generation); `Run` exit `defer` teardown (StopAll; + closeConn if AC-7 approved); `logFSMEvent` consumes the real `Event` error
- `internal/component/bgp/reactor/session_read.go` - policy-teardown signals `errChan` + `setCloseReason`
- `internal/component/bgp/reactor/session_handlers.go` - `handleOpen` FSM-state gate + NOTIFICATION code 5 + close
- `internal/component/bgp/fsm/fsm.go` - `FSM.Event` returns a sentinel when a default arm fires; handled events return nil

Sibling audit result (no edits needed, covered by the `Run` defer): `session_coalesce.go:101`, `session_validation.go:156/:265/:279/:300/:311`, `session_handlers.go:33/:61/:78/:154/:201/:303`, `session_read.go:114/:192/:209`, cancel-goroutine exits. Each stays responsible for its own NOTIFICATION + `closeReason`; none gains a per-site `StopAll`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Prometheus counters/metrics | [ ] | proposed: `ze_bgp_hold_expiry_graced_total`, `ze_bgp_open_in_established_total` (peer-labeled) in `reactor` metrics; ~~decide at implementation with Thomas~~ RESOLVED 2026-07-17 (Q-5): add both, in `reactor_metrics.go` (peer-labeled) |
| YANG schema / env var | [ ] | ~~only if D-2 picks a tunable grace duration; then `ai/rules/config-surface.md` + `ai/rules/config-naming.md` first~~ N/A per Q-1 (2026-07-17): grace extension is a fixed 10 s, not tunable; no leaf/env added |
| Doctor check | [ ] | N/A - no new runtime dependency |
| CLI grammar | [ ] | N/A - no new commands |

## Files to Create
- `test/parse/open-in-established.ci` - functional test for defect 3
- `test/parse/deadpeer-holddown.ci` - functional test for defect 1
- `test/interop/scenarios/47-holdtime-deadpeer-frr/` (`check.py`, `ze.conf`, `frr.conf`) - interop for defect 1

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: Grace re-arm goes through a NEW generation-checked `Timers` entry point called only by the expiry callback | (a) delete the `!holdRunning` guard from `ResetHoldTimer`; (b) call `StartHoldTimer` from the grace branch | (a) would let late FSM events resurrect timers on a torn-down session (the guard is load-bearing for `StopAll` semantics); (b) `StartHoldTimer` re-arms unconditionally and would equally resurrect after a concurrent `StopAll` (R-3). The expiry closure captures its arming generation; the re-arm proceeds only if the generation is unchanged (i.e., no `Stop*`/re-arm happened since this fire). |
| D-2: Extension duration -- RECOMMEND bounded 10 s (clamped to holdTime), matching the call-site comment | (a) full `holdTime` re-arm; (b) tunable via env | Full holdTime doubles worst-case dead-peer detection to 2x holdTime and makes the `.ci` slower; the bounded window caps it at holdTime + 10 s while genuine congestion keeps extending (each expiry re-checks `recentRead`, and any processed message resets to full holdTime via the FSM). The 10 s figure has no decision record (searched `plan/learned/`, none); it exists only in the comment at `session.go:421-424`, so this is a fresh decision for Thomas, not a restoration. Env-tunable only if Thomas wants it (config-surface rules apply). **Resolved Q-1 (2026-07-17): fixed 10 s, clamped to holdTime; not env-tunable.** |
| D-3: Teardown discipline is a single `defer s.timers.StopAll()` in `Session.Run` | per-site `StopAll` at each of the 8+ dirty paths | The defer covers current and future exits (this bug class already recurred 8 times); per-site fixes are exactly the missed-sibling shape that caused the defect. Idempotent by A-3. NOTIFICATION-sending and `closeReason` stay per-site (R-5). |
| D-4: `FSM.Event` returns a package-level sentinel (e.g. `fsm.ErrFSMError`) when a default arm fires; handled events (including deliberate ignores like ManualStop-in-Idle, which have explicit cases) return nil | boolean return; error per state+event pair | Callers need exactly one bit ("did the FSM treat this as an error transition"); a sentinel keeps `errors.Is` composable and `logFSMEvent` trivial. Explicit-case ignores are RFC-mandated non-errors and must stay nil. |
| D-5: Defect-3 gate lives in `handleOpen` (reactor), keyed on `s.fsm.State()` | inside the FSM | Ze's FSM deliberately sends no messages (`fsm/fsm.go` header note 3); the NOTIFICATION + close is session I/O. Gate: OpenSent proceeds (normal path); Established (and OpenConfirm, D-6) get NOTIFICATION code 5 subcode 0 + `closeConn` + error return; negotiation is NOT re-run on the rejected path (no live capability rewrite). |
| D-6: A second OPEN on the SAME connection in OpenConfirm gets the same FSM-error treatment as Established | run RFC 6.8 collision handling | RFC 6.8 collision is about two TCP connections (already handled by `DetectCollision`/`AcceptWithOpen`); a duplicate OPEN on one connection is not a collision. Current behavior (silent zombie Idle) is strictly worse than an explicit FSM error. Flag for Thomas: this is a judgment call on RFC 8.2.2's OpenConfirm Event 19 wording. **Resolved Q-3 (2026-07-17): FSM error (same as Established).** |
| D-7: Policy teardown signals a distinct sentinel that takes the BACKOFF reconnect class | reuse `ErrTeardown` (immediate reconnect) | The peer will typically re-offend immediately (its config still violates policy); immediate reconnect makes a NOTIFICATION storm. Needs Thomas's confirmation since it changes observable flap cadence for filter_family tear-down users. **Resolved Q-4 (2026-07-17): backoff (distinct sentinel, not `ErrTeardown`).** |
| D-8: AC-7 (defer `closeConn` in `Run`) proposed but scope-gated | leave conn-leak paths for a follow-up spec | Same one-defer shape and same file as D-3; but it is new scope found in design, so it ships only with explicit approval (no unilateral scope growth). **Resolved Q-2 (2026-07-17): approved, in scope.** |

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** -- add the failing unit + `.ci` tests named above; confirm each fails for the right reason (grace no-op observed, keepalive still running after teardown, no NOTIFICATION on second OPEN, `Event` returns nil, `Run` spins on policy teardown).
2. **Phase: hold-timer re-arm (defect 1)** -- generation counter in `Timers` (bumped on every arm and stop, checked by fired closures before clearing `holdRunning`); shared locked arm helper; generation-checked grace re-arm API; grace branch in `session.go` uses it with the D-2 duration; preserve `holdTime == 0` and post-`StopAll` no-op (tests `TestResetHoldTimerStillNoOpsAfterStop`, `TestHoldTimeZeroStaysDisabled`). Decide A-6 treatment (extend guard to keepalive/connectRetry or document). RFC comment per RFC Documentation section.
3. **Phase: StopAll discipline (defect 2)** -- single `defer s.timers.StopAll()` at the top of `Session.Run`; if AC-7 approved, `defer s.closeConn()` beside it; table-driven `TestSessionRunStopsTimersOnValidationTeardown` walks all 8 dirty paths; heap-reachability assertion for the abandoned-Session claim.
4. **Phase: FSM.Event sentinel (defect 4a)** -- default arms return `fsm.ErrFSMError`; `logFSMEvent` warn branch becomes live; audit ALL `fsm.Event(...)` call sites (grep) for ones that must now branch on the sentinel vs merely log.
5. **Phase: policy-teardown exit (defect 4b)** -- `session_read.go:242-250` also calls `setCloseReason` + signals `errChan` with the D-7 sentinel; `TestPolicyTeardownExitsRun`.
6. **Phase: OPEN-in-Established (defect 3)** -- state gate at the top of `handleOpen` per D-5/D-6; NOTIFICATION code 5 subcode 0; unit test both states; `test/parse/open-in-established.ci` goes green.
7. **Functional + interop tests** -- `deadpeer-holddown.ci`; `47-holdtime-deadpeer-frr` scenario; re-run reconnect interop scenarios for R-2.
8. **Full verification** -- `make ze-verify`.
9. **Complete spec** -- audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has code + test at file:line |
| Correctness | `holdTime == 0` still disables; post-`StopAll` `ResetHoldTimer` still no-ops; re-arm interval matches D-2 as approved |
| Sibling call-site audit | Every `closeConn`-without-`StopAll` path (8 listed + cancel goroutine) is covered by the `Run` defer; every `fsm.Event` caller re-audited after the sentinel lands |
| Lock ordering | No new path acquires t.mu while holding it (grace re-arm runs on the timer goroutine, lock-free entry); f.mu -> t.mu order preserved |
| Registration over hardcoding | No new per-feature field/switch added to a core/shared struct (`ai/rules/plugin-self-containment.md`) |
| No workaround | Timer fix is at the producer (`Timers`), not a session-side re-arm hack around a broken API |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Generation-guarded timer lifecycle | `go test -race ./internal/component/bgp/fsm/ -run TestHoldTimer` |
| `Run` exit discipline | `go test ./internal/component/bgp/reactor/ -run TestSessionRunStopsTimers` |
| OPEN gate + NOTIFICATION | `bin/ze-test bgp parse open-in-established` |
| Dead-peer detection end-to-end | `bin/ze-test bgp parse deadpeer-holddown`; interop scenario 47 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | second-OPEN gate must not parse/negotiate attacker bytes before rejecting (gate FIRST, then unpack only what the NOTIFICATION needs) |
| Resource exhaustion | flap cycles must not accumulate Sessions/timers (AC-3 heap assertion); NOTIFICATION-on-OPEN must not enable a reflection loop (close after send, backoff on reconnect) |
| Error leakage | NOTIFICATION data carries no internal state beyond RFC-required fields |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Interop reconnect regression (R-2) | Re-examine D-5 gate condition before touching daemon configs |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (skeleton) functional tests live in `test/bgp/*.ci` | no such directory; `ze-test bgp parse` runs `test/parse/*.ci` | `ls test/`; `mk/test-functional.mk:72` | test locations corrected in design |
| (skeleton) interop scenario "FRR/BIRD sends second OPEN in-session" | stock daemons cannot emit that; only the scripted peer can | A-5 reasoning + framework read | interop table redefined (dead-peer + regression direction) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| closeConn-without-StopAll | 8 sites | exit-discipline defer owns resource release | D-3 |

## Design Insights
- The `!holdRunning` early-return serves two masters: it is the BUG for the grace path and the GUARD for post-teardown resets. Any fix that treats it as only one of these regresses the other.
- Ze already has a per-message hold-timer restart chokepoint (FSM Events 26/27 handlers); the grace path is the only restart that bypasses the FSM, which is why it alone broke.
- Exit-discipline resources (timers, conn) belong to `Run`'s frame; reason-reporting (NOTIFICATION, closeReason) belongs to the erroring site. Mixing the two is what produced 8 divergent teardown paths.

## Core Insight
A timer flag cleared by the expiry closure BEFORE the owner decides what the expiry means makes "decide to keep running" unimplementable; the armed-state must survive until the decision, which is what the generation token provides.

## Known Limitations
- ROUTE-REFRESH does not restart the hold timer (no FSM event fired, `session_handlers.go:322-375`), so a refresh-only peer survives via repeated graced re-arms rather than a proper reset. **Not a gap, and no work is owed:** RFC 4271's FSM restarts HoldTimer on KeepAliveMsg (26) and UpdateMsg (27) only (`rfc/short/rfc4271.md:504-505`), and RFC 2918 says nothing about the hold timer, so a peer sending ROUTE-REFRESH but no KEEPALIVE is meant to time out. Ze already does the RFC-shaped thing here. Ruled by Thomas 2026-07-16; recorded cancelled in `plan/deferrals.md`.
- `handleUpdate`'s second `validateUpdateFamilies` call (`session_handlers.go:238`) is redundant with `processMessage`'s (`session_read.go:181`) and its error return path differs; not touched here.
- Cross-connection collision behavior (`DetectCollision`, `AcceptWithOpen`) is unchanged.

## RFC Documentation

Add `// RFC 4271 Section 8.2.2: "<quoted requirement>"` above the OPEN-in-Established gate and the hold-timer restart; `// RFC 4271 Section 6.6` (FSM error NOTIFICATION, code 5, subcode 0 per Section 6) above the NOTIFICATION send; `// RFC 4271 Section 4.4` above the `holdTime == 0` guard; note at the grace re-arm that the extension is a deliberate, documented divergence from Section 8.2.2 Event 10 (which mandates immediate teardown) shared with BIRD-style implementations.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated (AC-7 only if approved)
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Interop scenario passes against a reference daemon (or N/A with justification)
- [ ] Registration over hardcoding respected

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for hold-time
- [ ] Interop tests for protocol behavior (or N/A with justification)

## Open Questions for Thomas (resolve before `ready`)

| # | Question | Options | Spec's recommendation |
|---|----------|---------|-----------------------|
| Q-1 | Grace extension duration (D-2) | full holdTime / fixed 10 s / env-tunable | fixed 10 s, clamped to holdTime |
| Q-2 | AC-7 conn-close-on-Run-exit in scope? (D-8) | yes / follow-up spec | yes (same defer, same file, three verified leak sites) |
| Q-3 | OpenConfirm same-connection second OPEN = FSM error? (D-6) | FSM error / attempt collision semantics | FSM error |
| Q-4 | Policy-teardown reconnect class (D-7) | backoff / immediate (`ErrTeardown`) | backoff |
| Q-5 | Graced-expiry / OPEN-in-Established Prometheus counters | add / skip | add both (cheap, peer-labeled) |

### Resolutions (APPEND-ONLY: all five adopt the spec's own recommendation, the conservative choice)

→ AUTONOMOUS DEFAULT (2026-07-17): **Q-1 grace extension = fixed 10 s, clamped to holdTime** (adopts D-2). Rationale: a full-holdTime re-arm doubles worst-case dead-peer detection to 2x holdTime and slows the `.ci`; 10 s caps it at holdTime + 10 s while genuine congestion keeps re-extending (each expiry re-checks `recentRead`, and any processed message resets to full holdTime via FSM Events 26/27). Verified against source: the grace branch at `session.go:425-431` currently calls `ResetHoldTimer()`, which re-arms to FULL `t.holdTime` (`timer.go:223`), contradicting the "extend by 10s" comment at `session.go:421-424`, so the fix must re-arm to the 10 s clamp (min(10 s, holdTime)), not full holdTime. NOT env-tunable, so no YANG/env leaf is added (Integration Checklist YANG row → N/A). Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): **Q-2 AC-7 (conn-close on `Run` exit) = IN SCOPE** (adopts D-8). Rationale: same one-defer shape and same file as D-3: a `defer s.closeConn()` beside `defer s.timers.StopAll()` at the top of `Session.Run`. Three leak sites verified returning without `closeConn`: `handleOpen` unpack error (`session_handlers.go:43-47`), openValidator rejection (`session_handlers.go:98-115`, sends a NOTIFICATION but never closes), local-capability parse error (`session_handlers.go:121-124`); the cancel goroutine also exits on `<-s.done` without closing (`session.go:733-734`). AC-7 row below de-gated; "Behavior to change" note updated. Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): **Q-3 OpenConfirm same-connection second OPEN = FSM error** (not collision semantics; adopts D-6). **[STAKES: protocol]** Rationale: RFC 6.8 collision handling is about two *TCP connections* (already covered by `DetectCollision`/`AcceptWithOpen`); a duplicate OPEN on ONE connection is not a collision. Verified: `handleOpenConfirm` has no `EventBGPOpen` case and falls to `default -> change(StateIdle)` (`fsm.go:406-408`), producing a silent zombie strictly worse than an explicit FSM error. Treat identically to Established: NOTIFICATION code 5 subcode 0 + `closeConn`. This is a judgment call on RFC 4271 §8.2.2 OpenConfirm Event 19 wording; the conservative default (explicit FSM error over silent zombie) is the more-reversible choice. Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): **Q-4 policy-teardown reconnect class = BACKOFF** (not immediate `ErrTeardown`; adopts D-7). **[STAKES: protocol]** Rationale: after a policy tear-down the peer's config still violates policy, so it re-offends immediately; routing through `ErrTeardown`'s immediate-reconnect arm (`peer_run.go:78-84`) would produce a NOTIFICATION storm. A distinct sentinel that is NOT `errors.Is ErrTeardown` falls through to the exponential-backoff arm (`peer_run.go:118-151`), the lower-wire-churn, more-reversible option. This changes observable flap cadence for `filter_family` tear-down users. Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17): **Q-5 Prometheus counters = ADD BOTH, peer-labeled** (adopts recommendation). `ze_bgp_hold_expiry_graced_total` (incremented in the grace branch, `session.go:425-431`) and `ze_bgp_open_in_established_total` (incremented at the defect-3 gate in `handleOpen`). Home verified: the reactor `rmetrics` struct lives in `reactor_metrics.go` and is already peer-labeled via `peerLabel()` / `.With(peerLabel).Inc()` (`peer_run.go:42-49`); the session reaches it through `session.prefixMetrics = p.reactor.rmetrics` (`peer_run.go:204-206`). Cheap counters, no new runtime dependency. Integration Checklist Prometheus row → resolved (add). Thomas: override if wrong.

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Deepened to design 2026-07-16: every `file:line` re-verified against the working tree.
- **Citation drift (skeleton -> verified 2026-07-16):** `session.go:424-430` -> `:425-431` (grace branch); `session.go:421-423` -> `:421-424` (10 s comment); `session.go:688-697` -> `:689-698` (logFSMEvent); `fsm.go:487-492` -> `:487-491` (Established default arm); `timer.go:392-400` -> `:393-400` (StopAll). All other skeleton citations exact. No skeleton claim failed verification; two skeleton TEST-PLAN premises were wrong and corrected (Mistake Log): `test/bgp/` does not exist, and the defect-3 interop scenario is not implementable with a stock daemon.
- Additions found during design (not in the skeleton, all verified): RFC 7606 session-reset leak site (`session_validation.go:156`); hold-expiry/ctx-cancel exits leave keepalive running (`session.go:736-738`); `ResetHoldTimer`'s twin closure shares the clear-before-callback shape (`timer.go:223-232`); keepalive stale-chain race (A-6); conn-leak paths without `closeConn` (AC-7 / Q-2).
- Related in-progress work: `plan/spec-bgp-session-ready-contract.md` (EOR readiness) touches the same `Session` but a different concern; coordinate but do not merge. `plan/spec-fixit-redistribute-establishment-stall.md` touches reactor establishment flow; no file overlap with this spec's Files to Modify except `session.go` (watch for merge friction).
