# Spec: improve-5 -- Panic Containment at Network-Input Task Boundaries

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Scope | protocol |
| Release reason | Untrusted network input must not let one session panic terminate the whole control plane. |
| Depends | - |
| Phase | 10/10 |
| Updated | 2026-10-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/post-release/engineering/spec-improve-0-umbrella.md` -- set context
4. `internal/component/bgp/reactor/session.go` -- `Session.Run`, where the new BGP boundary goes
5. `internal/component/bgp/reactor/peer_run.go` -- `safeRunOnce`, the existing outer boundary
6. `ai/rules/goroutine-lifecycle.md` -- worker goroutine rules
7. The audit table in "Design Insights" -- every network-input task and its phase

## Task

A peer MUST NOT be able to panic the daemon (`docs/contributing/ze-go-style.md`,
"Assertions, in a language that has none"). For BGP the process already survives:
`Session.Run` runs inline inside `runOnce`, which `safeRunOnce` wraps in a recover
(added 2026-03-06, `986ff479dd`). What BGP lacks is the rest of the policy: the
peer is never told why the session ended (the `closeConn` defer in `Run` closes the
socket during the unwind, before the outer recover fires, so no NOTIFICATION can be
sent), no metric counts the panic, the log names neither the message type nor a
command, no crash record exists for `ze show crashes`, and no test proves that a
panic in the READ path (rather than the dialer) is contained.

Every sibling network-input task outside BGP has no task-level recover at all: the
L2TP, IKE, OSPFv2, OSPFv3, IS-IS, BFD, VRRP, RSVP-TE, LDP, RTR, BMP, RADIUS, TACACS+,
DHCP, TFTP and CoA receive paths, the plugin RPC readers and connection handlers, the
SSH server and the gNMI server. A panic in any of them still ends the process, and
with it every BGP session and the appliance's control plane.

This spec installs one containment policy, implemented once in a shared guard, at
every network-input task boundary: recover at the task, never inside a decode
function; log the context and the stack at ERROR; count it in one metric; write a
crash record; end only the unit the input belongs to (the session, the neighbor, the
request) through that unit's existing teardown path; and keep every other unit and
the process running.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived worker rules
  → Constraint: the guard is a deferred call at the top of an existing goroutine or an existing per-event dispatch; it starts no goroutine and never restarts a goroutine from inside its own recover
- [ ] `ai/rules/principles.md` - no silently wrong value
  → Constraint: a recovered panic is never converted into a zero, a nil or a dropped packet without a log line, a metric increment and a crash record; a reader that stops after a panic must end its unit visibly, never leave an interface that silently receives nothing
- [ ] `ai/rules/user-facing-errors.md` - every message says what, why, and the fixing command
  → Constraint: the ERROR line says the unit was ended because of an internal Ze defect, names what restarts it and when, and names `ze show crashes` as the command that shows the report to attach to a bug
- [ ] `docs/contributing/ze-go-style.md` - "Assertions, in a language that has none"
  → Constraint: `panic("BUG: ...")` stays the tool for a state only a Ze defect reaches; the guard is the backstop that makes that rule's first sentence true, never a reason to panic on peer input
- [ ] `docs/architecture/core-design.md` - session/reactor layering
  → Constraint: the recover sits where session teardown is already defined, so a contained panic ends the session the way a fatal read error does
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - "`safeRunOnce` and panic recovery"
  → Constraint: `safeRunOnce` stays as the outer boundary for panics outside `Session.Run` (dial, OPEN setup); the page gains the inner boundary
- [ ] `docs/architecture/behavior/fsm.md` - `sessionDown` steps and the receive-publication failure
  → Constraint: `sessionDown` already runs on every exit from `runOnce`, so a `Run` that returns an error instead of unwinding reuses it unchanged
- [ ] `docs/architecture/diagnostics/crash-capture.md` - crash files and `show crashes`
  → Constraint: a recovered panic is recorded through `internal/core/crashlog` so the operator finds it where process crashes are listed

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/full/rfc4271.txt` - Section 4.5, Section 6.7, Section 8.1.2
  → Constraint: Section 4.5: "If no appropriate Error Subcode is defined, then a zero (Unspecific) value is used for the Error Subcode field." Section 6.7: "In the absence of any fatal errors (that are indicated in this section), a BGP peer MAY choose, at any given time, to close its BGP connection by sending the NOTIFICATION message with the Error Code Cease." A recovered panic is no fatal error of Section 6, so the NOTIFICATION is Cease, sent through `teardownAutomatic` (Event 8, AutomaticStop)
- [ ] `rfc/full/rfc4486.txt` - Section 3 subcode list
  → Constraint: the eight subcodes cover prefix limits, administrative actions, collisions and "Out of Resources (e.g., memory)"; none names an internal processing failure, so Section 4.5's Unspecific (0) applies

**Key insights:**
- This changes failure blast radius only; message processing semantics are untouched.
- FRR and BIRD contain nothing per session: BIRD's `bug()` (`sysdep/unix/log.c`) logs and calls `abort()`, and FRR's bgpd guards its invariants with `assert()` (25 in `bgpd.c`), so either daemon dies whole and is restarted by its supervisor (watchfrr, systemd). Ze runs every protocol in one process on an appliance, so the per-unit boundary is Ze's own design, and interop is unaffected because the peer sees only a standard Cease.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/session.go` - `Session.Run` drives `readAndProcessMessage` / `readAndProcessCoalesced` inline with no recover of its own; its recover covers only the cancel goroutine; its `defer s.closeConn()` closes the socket during any unwind, so the outer recover can no longer send a NOTIFICATION
- [ ] `internal/component/bgp/reactor/peer_run.go` - `runOnce` ends with `return session.Run(p.ctx)`; `safeRunOnce` recovers, logs "session panic recovered" with a 4 KB stack, returns an error, and the run loop's exponential backoff applies (the delay resets only on a nil error)
- [ ] `internal/component/bgp/reactor/session_read.go` - `onMessageReceived` is called inline for every received message: the test seam a panicking callback uses
- [ ] `internal/component/bgp/reactor/peer_session_end.go` - `failReceivePublication` tears the session down through `teardownAutomatic` with Cease / Out of Resources after a recovered delivery-worker panic
- [ ] `internal/component/bgp/reactor/session_connection.go` - `teardownAutomatic`: Event 8 AutomaticStop, Cease NOTIFICATION, close reason, errChan signal
- [ ] `internal/component/bgp/reactor/session_write.go` - `sendNotificationWithin` takes `writeMu` through `writeMessageWithin`
- [ ] `internal/component/bgp/reactor/panic_recovery_test.go` - `panicDialer` and `panicReceiver`: the existing test-only injection idiom (a fake behind an existing interface or func field)
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - `ze_peer_*` counter vectors: the metric naming precedent
- [ ] `internal/component/plugin/process/process.go` - the internal plugin runner goroutine recovers, but goroutines a plugin starts (its read loops) are outside that recover
- [ ] `internal/core/dnsserver/handler.go` - `Authoritative` recovers per query: the request-class precedent
- [ ] `internal/core/crashlog/crashlog.go` - `HandlePanic` and `HandleCaughtPanic` write crash files that `ze show crashes` lists
- [ ] `internal/component/firewall/metrics.go` - `applyResultPanic`: a panic counted as its own result, never as success

**Behavior to preserve:**
- Normal error paths (parse errors, NOTIFICATION handling, FSM events) unchanged;
  the guard fires only on an actual panic.
- Buffer pool discipline: the `kept` defer in `session_read.go` still returns
  buffers when the unwind stops at the new boundary.
- Existing recovers (cancel goroutine, `safeRunOnce`, delivery worker, monitor,
  listener, engine events, DNS per query) stay; the new BGP boundary is inside
  `safeRunOnce`, which keeps covering the dial and setup path.
- The process exits on a panic in a goroutine no guard covers, exactly as today:
  the guard is added per task, never as a process-wide catch.

**Behavior to change:**
- A recovered BGP read-path panic sends Cease / Unspecific before the socket closes.
- The recovered delivery-worker panic sends Cease / Unspecific instead of
  Out of Resources (same RFC 4271 Section 4.5 reasoning); the "no receiver" branch
  of `failReceivePublication` keeps Out of Resources.
- Every recovered panic is logged by the shared guard, counted, and crash-recorded.
- A panic in a sibling network-input task ends its unit instead of the process.

## Data Flow (MANDATORY)

### Entry Point
- BGP: untrusted peer bytes enter at `Session.Run` -> `readAndProcessMessage` /
  `readAndProcessCoalesced`; a panic below (decode, attribute parsing, RIB and plugin
  callbacks) unwinds through `Run` to `safeRunOnce` today.
- Siblings: each protocol's receive goroutine (the audit table in "Design Insights")
  and the worker that consumes what it queues; a panic in either ends the process today.

### Transformation Path
1. The guard is deferred once at the top of the task: `Session.Run` for BGP, each
   receive goroutine, each per-connection handler, and the per-event dispatch call of
   each protocol worker (the same shape as the engine event dispatch recover). It is
   never placed in a decode function or a per-attribute loop.
2. On a panic the guard captures the panic value and stack, logs at ERROR with the
   component, the task, the unit identity (peer address, interface, neighbor, client
   address), the last message type when the task tracks one, the panic value and the
   stack; increments `ze_task_panics_total{component,task}`; and writes a crash record
   through `internal/core/crashlog`.
3. The unit ends through its existing teardown path. BGP: Cease / Unspecific through
   `teardownAutomatic`, then `Run` returns an error and `runOnce`'s `sessionDown`
   runs as for any session error. Other classes: see "Key Design Decisions".
4. The unit restarts through the backoff its component already owns; the guard adds
   no restart loop of its own.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Panic unwind -> session teardown | the guard in `Run` runs before the `closeConn` defer, sends the Cease and returns an error; `sessionDown` follows | [ ] |
| Session -> reactor | `runOnce` returns an error; the run loop's existing backoff decides the retry | [ ] |
| Guard -> metrics, logs, crash files | one shared guard owns the log line, the counter and the crash record | [ ] |
| Protocol worker -> neighbor state | the worker's dispatch guard raises the protocol's existing neighbor-down event for the unit the event came from | [ ] |

### Integration Points
- `Session.Run` (`session.go`) - BGP boundary location.
- `teardownAutomatic` (`session_connection.go`) - reused for the post-panic Cease.
- `failReceivePublication` (`peer_session_end.go`) - its panic caller passes Unspecific.
- `metrics.Registry.CounterVec` (`internal/core/metrics`) - re-registration returns the same vector, so every component registers `ze_task_panics_total` with the same name and labels.
- `internal/core/crashlog` - gains one exported recorder for a recovered panic.

### Architectural Verification
- [ ] No bypassed layers (post-panic close uses the normal teardown path)
- [ ] No unintended coupling (no recover sprinkled in decode internals)
- [ ] No duplicated functionality (one guard, one log shape, one metric)
- [ ] Registration over hardcoding -- the metric registers through each component's existing registry

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No lock is held across a call that can panic in a guarded task, so the teardown after a recover does not block | Ze's lock discipline uses defer unlock on most paths | The teardown deadlocks the unit, or a shared reactor lock stalls every peer | Phase 2 audit of every lock taken in the read path (`s.mu`, `s.writeMu`, `p.mu`, `reactor.mu`); each is released by defer or never held across a callback; AC-4 proves the second peer still progresses | validated in phase 2 (Lock audit A-1 below; two reactor-wide holds fixed) |
| A-2 | One deferred call per task (per session, per goroutine, per dispatched event) costs nothing measurable | a defer at function entry is open-coded by the Go compiler | Hot path slows | The guard is never per message inside `readAndProcess*`; `Run` defers once per session; AC-9 | validated by placement |
| A-3 | Shared state touched mid-panic (RIB, pools, LSDB) stays consistent enough for the other units | pools use get/return discipline; a link-state LSDB resyncs when the adjacency restarts | Containment gives false safety | Each phase names the shared state its task mutates and the unit teardown that resyncs it; a site that cannot be contained is listed in Known Limitations | validated with limits: each row's end resyncs through the unit's own teardown (Cease and reconnect, KillNbr, adjacency teardown, LSP soft-state expiry plus admission release, BFD handshake, VRRP Shutdown/Startup, StopCCN, PADT, IKE Delete); the residuals are in Known Limitations |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Recover masks real bugs (peers flap instead of crash reports) | `ze_task_panics_total` > 0 | ERROR log with stack, crash record listed by `ze show crashes`, metric; no knob downgrades it |
| R-2 | A peer that triggers the same panic on every session gets a free reconnect loop | the same peer label recurring in the log | BGP: Event 8 increments ConnectRetryCounter and the run loop's backoff doubles to its cap (no reset on error); siblings reuse their own backoff |
| R-3 | Boundary placed too low (inside decode) | review | the guard is called only from the task entry points listed in the audit table |
| R-4 | The NOTIFICATION write blocks on `writeMu` held by the panicking frame | a session that never closes after a panic | the guard sends the Cease only when it acquires the write path without waiting (TryLock) and within `holdExpiryNotifyDeadline`; otherwise it logs that no NOTIFICATION was sent and the socket closes |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE read by `Session.Run` over `net.Pipe`, `onMessageReceived` panics | → | guard in `Run` -> Cease / Unspecific -> `sessionDown` -> backoff | TestSessionReadPanicSendsCeaseAndCloses |
| same, coalesced read path | → | same guard | TestSessionReadPanicCoalesced |
| two peers started through `Peer.StartWithContext`, peer A's callback panics | → | peer A reconnects, peer B keeps receiving | TestPeerReadPanicIsolation |
| delivery worker receiver panics (`panicReceiver`) | → | `failReceivePublication` with Unspecific | TestDeliveryPanicSendsCeaseUnspecific |
| each sibling task's real entry (socket, pipe or handler) with its existing callback or decoder seam panicking | → | shared guard -> unit teardown | the test named in that task's audit-table row |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A panic in BGP message processing for one peer | The process stays up; the peer receives a NOTIFICATION Cease (6) / Unspecific (0); `Run` returns an error; `sessionDown` runs; the peer retries after the run loop's backoff |
| AC-2 | Any recovered panic, every class | One ERROR line naming component, task, unit, message type where tracked, panic value and stack, saying the unit was ended by an internal Ze defect and pointing to `ze show crashes`; `ze_task_panics_total{component,task}` increments by one; a crash record appears in `crashlog.ListCrashes` |
| AC-3 | Read buffers held when the BGP panic fires | Returned to the pool (pool counters balance after the test) |
| AC-4 | Peer B established while peer A panics | Peer B keeps exchanging messages and its routes stay installed |
| AC-5 | The delivery worker recovers a receiver panic | The Cease carries Unspecific; a missing receiver still sends Out of Resources |
| AC-6 | The write path is held by the panicking frame | The guard does not block: the socket closes, the log line says no NOTIFICATION was sent |
| AC-7 | A panic in each sibling task of the audit table | The process stays up; only that unit (session, neighbor, request, connection) ends through its existing path; the component's other units continue |
| AC-8 | Audit table | Every network-input task in Ze is a row with its class, its boundary location and its test; no row reads "lacks" at closure |
| AC-9 | No panic | Behavior unchanged; no guard is placed per message inside a read loop |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A buggy or hostile peer triggers a processing panic on an appliance | guard -> Cease -> peer down -> backoff; every other protocol stays up | TestPeerReadPanicIsolation |
| 2 | Operator investigates | reads the ERROR line, runs `ze show crashes`, attaches the report | crash-record assertion in TestSessionReadPanicSendsCeaseAndCloses |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestGuardRecordsPanic | `internal/core/taskguard/guard_test.go` | AC-2: log fields, metric increment, crash record, re-panics nothing | |
| TestGuardNoPanicIsSilent | `internal/core/taskguard/guard_test.go` | AC-9: no log, no metric, no record without a panic | |
| TestSessionReadPanicSendsCeaseAndCloses | `internal/component/bgp/reactor/session_panic_test.go` | AC-1, AC-2, AC-3 (wire bytes of the NOTIFICATION asserted) | |
| TestSessionReadPanicCoalesced | `internal/component/bgp/reactor/session_panic_test.go` | AC-1 on the coalesced path | |
| TestSessionReadPanicWriteLockHeld | `internal/component/bgp/reactor/session_panic_test.go` | AC-6 | |
| TestPeerReadPanicIsolation | `internal/component/bgp/reactor/session_panic_test.go` | AC-4 | |
| TestDeliveryPanicSendsCeaseUnspecific | `internal/component/bgp/reactor/panic_recovery_test.go` | AC-5 | |
| The 21 sibling tests in the audit table's Test column, TestRTRReadPanicContained to TestGNMIHandlerPanicContained | the package of each audit row | AC-7 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- control-flow change) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | - | A panic can only be injected through a test seam, and no seam may enter a shipped binary, so no `.ci` scenario can reach it. The Go tests drive the real entry points (`Session.Run` over `net.Pipe`, `Peer.StartWithContext`, each protocol's socket or handler); the existing functional suites guard the no-panic path (AC-9) | |

### Interop Tests (MANDATORY for protocol features)
- The only wire-visible change is the Cease / Unspecific NOTIFICATION. Its encoder
  and the peers' acceptance of a Cease are already exercised by the existing BGP
  interop scenarios that end sessions with a Cease. TestSessionReadPanicSendsCeaseAndCloses
  asserts the exact NOTIFICATION octets Ze writes. Whether a lab scenario that injects
  a panic is also owed is the owner question in "Owner Questions".
- `bgp-panic-cease` (`test/interop/scenarios/bgp-panic-cease`, checker
  `internal/le/interoplab/bgp/check_panic_cease.go`, interop.md inventory row 60):
  Ze runs the `ze_le` daemon, FRR 10.7.1's UPDATE triggers the injected read-path
  panic, FRR records a RECEIVED Cease / Unspecific, its session returns (2
  established, 1 dropped), and the BIRD 3.4.0 session never drops.
  -> Evidence: green on 2026-10-27, twice (session scratch `lab-green4.log`, `lab-green5.log`,
  `INTEROP_SCENARIO=bgp-panic-cease ./le test integration interop`, "1 passed,
  0 failed"), after 27d7e4f0ea moved the seam to `processValidatedMessage`.
  -> Evidence: red on 2026-10-27, `ceaseAfterPanic` with its
  `tryNotificationWithin` send replaced by `var err error` (GOFLAGS overlay):
  assertion 5 timed out, ze logged "an internal Ze defect ended this unit" for
  172.30.0.3, FRR's `lastResetDueTo` read "Peer closed the session" (code 15).
  -> Evidence: shipped-binary proof on the staged lab binaries:
  `go tool nm test/interop/ze-linux | grep -c reactor.injectReadPanic` = 0,
  `strings ... | grep -c bgp-read-panic-` = 0, `grep -c /run/ze-inject` = 0;
  `ze-le-linux` gives 1, 1 and 2.
  -> Decision: the seam sits in `processValidatedMessage`, not `processMessage`:
  the coalescing reader hands a plain IPv4 announcement straight to
  `processValidatedMessage`, so the first lab run armed a trigger no UPDATE
  consumed (TestInjectReadPanicReachedByBothReaders, red with the old placement).

## Files to Modify
- `internal/component/bgp/reactor/session.go` - guard deferred in `Session.Run`, after the `closeConn` defer so it runs first; tracks the last received message type in a field the read path sets and only `Run`'s goroutine reads
- `internal/component/bgp/reactor/session_connection.go` - a non-blocking teardown variant for the guard (R-4)
- `internal/component/bgp/reactor/peer_session_end.go` - `failReceivePublication` takes the subcode from its caller
- `internal/component/bgp/reactor/peer_run.go` - delivery-worker recover passes Unspecific and calls the shared guard; `safeRunOnce` calls the shared guard
- `internal/component/bgp/reactor/session_read.go` - read path records the message type before dispatch
- `internal/core/crashlog/crashlog.go` - an exported recorder for a recovered panic (crash file plus syslog, the process keeps running)
- `internal/component/bgp/plugins/rpki/rtr_session.go`, `internal/component/bgp/plugins/bmp/receiver.go`, `internal/component/bgp/plugins/bmp/bmp.go` - phase 3
- `internal/plugins/ldp/session.go`, `internal/plugins/rsvpte/transport_linux.go` - phase 4
- `internal/plugins/ospf/transport/backend_linux.go`, `internal/plugins/ospf/v3/transport/backend_linux.go`, `internal/plugins/isis/transport/backend_linux.go`, and the worker of each that consumes `recvCh` - phase 5
- `internal/component/bfd/transport/udp.go`, `internal/plugins/vrrp/transport/backend_linux.go`, and their consuming workers - phase 6
- `internal/component/l2tp/listener.go`, `internal/component/l2tp/pppoe/kernel_linux.go`, `internal/component/ike/transport/udp.go`, `internal/component/ike/engine/inbound.go` - phase 7
- `internal/component/radius/client.go`, `internal/component/tacacs/client.go`, `internal/plugins/dhcpserver/register.go`, `internal/plugins/tftpserver/socket_linux.go`, `internal/component/l2tp/plugins/authradius/coa.go` - phase 8
- `pkg/plugin/rpc/conn.go`, `pkg/plugin/rpc/mux.go`, `internal/component/plugin/ipc/tls.go`, `internal/component/plugin/server/managed_serve.go`, `internal/component/ssh/ssh.go`, `internal/component/gnmi/server.go` - phase 9
- `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/diagnostics/crash-capture.md` - the BGP boundary, the Cease subcode, the recovered-panic record
- `docs/guide/monitoring.md` - `ze_task_panics_total`, beside the `ze_peer_*` counters it already lists
- The design page of each sibling phase, edited in that phase where it describes the receive task: `docs/architecture/plugin/rib-storage-design.md` (phase 2, delivery worker), `docs/architecture/ldp/mpls-ldp.md` (phase 4), `docs/architecture/ospf/ospf-3-ip-transport.md` (phase 5), `rfc/short/rfc5881.md` read for the BFD single-hop transport (phase 6, no requirement row changes), `docs/research/l2tpv2-ze-integration.md` (phase 7, read only), `docs/architecture/api/ipc_protocol.md` (phase 9)

## Files to Create
- `internal/core/taskguard/guard.go` - the shared guard: log line, metric, crash record; returns whether a panic was recovered so the caller runs its unit teardown
- `internal/core/taskguard/guard_test.go` - AC-2, AC-9
- `internal/component/bgp/reactor/session_panic_test.go` - AC-1, AC-3, AC-4, AC-6

## Implementation Steps

Each phase is sized at about 60 tool calls, lands as its own commit once its scoped
tests pass, and proves its boundary with a test that panics through an existing seam
at the real entry point and shows the process and the other units survive.

1. **Phase 1: Wiring and guard.** Write TestSessionReadPanicSendsCeaseAndCloses
   first (it fails: no NOTIFICATION today). Create `internal/core/taskguard` and the
   crashlog recorder; AC-2 and AC-9 tests. Scoped: `./le test unit` for
   `internal/core/taskguard` and `internal/core/crashlog`.
2. **Phase 2: BGP boundary.** Guard in `Session.Run`, non-blocking Cease / Unspecific,
   message-type tracking, delivery-worker subcode, `safeRunOnce` through the guard;
   lock audit for A-1 written into "Design Insights"; AC-1, AC-3 to AC-6; page edits.
   Scoped: the `internal/component/bgp/reactor` package with `-race`.
3. **Phase 3: RTR and BMP.** Per-session class; AC-7 tests TestRTRReadPanicContained,
   TestBMPReceiverPanicContained.
4. **Phase 4: LDP and RSVP-TE.** Per-session class (LDP session) and per-neighbor
   worker class (RSVP-TE); TestLDPReadPanicContained, TestRSVPReadPanicContained.
5. **Phase 5: OSPFv2, OSPFv3, IS-IS.** Reader goroutine and consuming worker each
   guarded; the worker's guard raises the neighbor or adjacency down event for the
   packet's source; TestOSPFReadPanicContained, TestOSPFv3ReadPanicContained,
   TestISISReadPanicContained.
6. **Phase 6: BFD and VRRP.** TestBFDReadPanicContained, TestVRRPReadPanicContained.
7. **Phase 7: L2TP, PPPoE and IKE.** Per-tunnel and per-SA units;
   TestL2TPReadPanicContained, TestPPPoEReadPanicContained, TestIKEInboundPanicContained.
8. **Phase 8: request servers and AAA clients.** Per-request class;
   TestRADIUSClientReadPanicContained, TestTACACSClientPanicContained,
   TestDHCPRequestPanicContained, TestTFTPRequestPanicContained, TestCoAPanicContained.
9. **Phase 9: plugin IPC, SSH, gNMI.** Per-connection class; a panic ends that plugin
   connection or client session through its existing close; TestRPCReadLoopPanicContained,
   TestMuxReadLoopPanicContained, TestPluginAcceptorPanicContained,
   TestManagedServerPanicContained, TestSSHHandlerPanicContained,
   TestGNMIHandlerPanicContained.
10. **Closure.** Audit table shows no "lacks" row (AC-8); `./le verify changed`;
    learned summary; the two-commit closure.

Each phase starts by locating the existing seam its test panics through (an interface
the task calls, a func field, a fake transport the package's tests already build, as
`panicDialer` and `panicReceiver` do). A phase that finds no seam builds the fake
behind an existing interface in the test file; it never adds a hook to production code.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-9 with file:line; every audit row has its test |
| Correctness | guard at task entry only; unit teardown reused; no recover in decode internals |
| Performance | no per-message defer in a read loop (AC-9) |
| Registration over hardcoding | the metric registers through each component's registry with one name and label set |
| Rule: no silently wrong value | a recovered panic always logs, counts and records; a reader that stops ends its unit visibly |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | unchanged; this is blast-radius control, not validation |
| DoS surface | a repeated-panic peer meets the component's backoff (R-2) |
| Error leakage | the panic value and stack go to the local log and crash file only; the wire carries only the Cease code and subcode, no Data |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The skeleton said a BGP read-path panic kills the daemon | `safeRunOnce` (`peer_run.go`) had recovered it since 986ff479dd (2026-03-06); what BGP lacked was the NOTIFICATION, the metric, the crash record and a read-path test | Disposition at 043c682d24 | Reworked in place into the shared policy; the Task section states the true premise |
| approach | The `ze_le` injection seam sat in `processMessage` | The coalescing reader (`session_coalesce.go`) hands a plain IPv4 announcement straight to `processValidatedMessage`, so the first lab run armed a trigger no UPDATE consumed and the interop scenario went red for the wrong reason | Lab run 3 log: trigger file stayed, FRR stayed Established | 27d7e4f0ea moved the seam; TestInjectReadPanicReachedByBothReaders is red on the old placement |
| assumption | A-1: no lock is held across a call that can panic | False in six components: the reactor `r.mu` and AIGP `state.mu` (BGP), RTR `s.mu` and `lease.mu`, L2TP `tunnelsMu`, RSVP-TE `lsp.mu` (about 20 sites), OSPF `e.mu` on the receive path and the graceful-restart `m.mu`, IS-IS circuit `c.mu`. A panic there would have left the lock held and wedged every unit sharing it, so containment would have turned a crash into a hang | Each phase's lock audit; reds by overlay (timeouts, TryLock failures) | Every hold fixed by `defer` with a lock test; the table in "Lock audit (A-1)" |
| assumption | The phase 5 lock audit: OSPF self-origination never runs on the receive worker | A received Grace-LSA (`grInspectV6Update` -> `helperEnter`) and an LSDB content change (`helperExit`) originate on the receive worker, and a neighbor reaching Full runs `exitRestart` there | Closure review, by tracing every `originateSelfLSAs` caller | The origination `e.mu` holds read clean; the graceful-restart `m.mu` holds fixed at closure with TestGRLockReleasedAfterContainedPanic |
| approach | Existing recovers were counted as boundaries | DNS `Authoritative`/`Recursive` and the OSPF opaque, RI and ext-sub-TLV wrappers swallowed a panic with no log, count or record whenever the caller passed no `onPanic`, which every test caller and `ext_subtlv.go` `Render` did | Phase 1 and phase 5 reading | b505e144ab and b90a703ab5 route them through taskguard; TestQueryPanicNeverSilent, TestOSPFRenderPanicNeverSilent |
| approach | PPPoE `handlePADR` initialised `PppoxFD` to its zero value | Fd 0 is a valid descriptor: a `Remove` before `createTransport` closed the daemon's stdin | Phase 7 PPPoE test | 558c13f782 initialises it to -1 |
| assumption | RSVP-TE's LSP role zero value was a role | `lspRole` zero was `RoleIngress`, so the panic end's `tearLSPLocal` on an LSP a handler had just created issued `removePush` for a tunnel endpoint it never installed | Phase 4 review of the end path | e4bf3104a6: zero is `RoleUnspecified`, `tearLSPLocal` withdraws only installed state |
| approach | The original audit listed readers and connection handlers only | Per-request work dispatched to another goroutine escaped it: `dispatchPluginRPC` (one goroutine per plugin request), the LDP discovery reader, the RADIUS accounting goroutine and the commit-confirm worker that runs every gNMI, SSH, web and API config write | Phase 9, phase 8 and closure review | Each got a row, a boundary and a test |
| approach | Existing tests changed without a judge before the commit | e4bf3104a6 changed two existing tests: `linkdown_test.go` TestHandleLinkDownIngressTornDown's fixture gained `lsp.Reserved = true` (assertions unchanged), and `rfc3032_fsm_test.go` gained the `{RoleUnspecified, "unspecified"}` case. ze-test-judge judged both after the commit landed, not before | Phase 4 part 3 record | Judged after the fact; a test change is judged before the commit that carries it |
| escalation | The IS-IS quotes cite `rfc/full/iso-iec-10589.txt` Section 10.6 | That file is a project-written "Independent Reconstruction" (`RFC-SYNTHETIC-ISIS`), not ISO/IEC 10589, so the quoted sentence is the project's restatement, not the standard's text | Phase 5 part 4 | Owner decision: see Owner Questions |

## Design Insights

### Network-input task audit (AC-8), at 043c682d24

Classes: **session** (one peer connection; the unit is that session), **neighbor**
(a shared protocol worker; the unit is the neighbor or adjacency the packet came
from), **request** (one query or exchange; the unit is that request), **connection**
(one plugin or management client). A reader goroutine that only queues packets is
guarded too: when it recovers it ends its interface's receive task through the
component's existing interface-down path, never leaving a socket nobody reads.

| Task (file, function) | Class | Has a boundary today | Phase | Test |
|-----------------------|-------|----------------------|-------|------|
| BGP `Session.Run` read path (`reactor/session.go`) | session | outer only: `safeRunOnce` (`peer_run.go`), no NOTIFICATION, no metric | 2: DONE, task `read`, `ceaseAfterPanic` | TestSessionReadPanicSendsCeaseAndCloses, TestSessionReadPanicCoalesced, TestSessionReadPanicWriteLockHeld, TestPeerReadPanicIsolation |
| BGP `safeRunOnce` (`peer_run.go`) | session | has it | 2: DONE, task `connect` | TestPeerRunRecoversPanic |
| BGP session cancel goroutine (`session.go`) | session | has it | 2: DONE, task `cancel` | existing |
| BGP delivery worker (`peer_run.go`) | session | has it, Out of Resources | 2: DONE, task `delivery`, Unspecific | TestDeliveryPanicSendsCeaseUnspecific |
| BGP listener handler `safeHandle` (`reactor/listener.go`) | connection | has it | 2: DONE, task `accept` | TestListenerHandlerRecoversPanic |
| BGP listener cancel goroutine (`reactor/listener.go`) | internal | has it, logs at ERROR with stack | none: closes the listener on context end, reads no network input | existing |
| BGP reactor monitor, notify, signal, forward pool and build (`reactor.go`, `reactor_notify.go`, `signal.go`, `forward_pool.go`, `forward_build.go`) | internal | has it | none: not network input | existing |
| Plugin engine event dispatch (`plugin/server/engine_event.go`) | internal | has it | none | existing |
| Internal plugin runner goroutine (`plugin/process/process.go` `startInternal`) | connection | had a log-only recover | 9: DONE 895f153c5e, task `plugin/run`, recover replaced; the plugin ends as one whose run returned | TestInternalPluginRunnerPanicContained |
| Plugin-to-engine request (`plugin/server/dispatch.go` `dispatchPluginRPC`) | request | lacked: found in phase 9, missing from the 043c682d24 audit | 9: DONE 07e6474dfa, task `plugin/engine-op`, request answered with a plain `SendError`, not applied; the connection and other requests continue | TestDispatchPluginRPCPanicContained |
| DirectBridge request (in-process plugin to engine) | request | runs synchronously on the calling goroutine | none of its own: the calling task's boundary contains it (`plugin/run`, or the network reader's own row, for example `bmp/receive`); a goroutine a plugin starts for internal work is not network input. Stated in `process-protocol.md` | covered by the caller's row |
| DNS per query, incl. DoT and DoH, as112, geodns (`core/dnsserver/handler.go` `Authoritative`, `recursive.go` `Recursive`) | request | had it, and SWALLOWED the panic silently when `onPanic == nil` | 1: DONE b505e144ab, tasks `dns/authoritative-query`, `dns/recursive-query`; no reply, the client retries | TestQueryPanicNeverSilent |
| Commit-confirm worker (`config/confirm/confirm.go` `Window.do`) running a gNMI Set, an SSH, web or API commit, an accept, an abort, a managed push | request | lacked: found at closure; every config write runs on this one worker, outside the gNMI, SSH and web boundaries on the caller's goroutine | closure: DONE (uncommitted), task `config-confirm/write` in `Window.write`; the caller gets a plain error saying the write may be partly applied, the worker serves the next write and the deadline | TestConfirmWritePanicContained |
| OSPF plugin-registered opaque consumers and builders, show decoders (`ospf/opaque_registry.go`, `ri_registry.go`, `ext_subtlv.go`, `decode_view*.go`) | internal | had recovers that swallowed silently when no `onPanic` was passed | 5: DONE b90a703ab5, tasks `ospf/consumer`, `ospf/show-decode` | TestOSPFRenderPanicNeverSilent, TestOSPFBuilderPanicRecorded |
| Web UI, looking glass (`net/http` servers) | request | has it: `net/http` recovers per request | none | - |
| RTR `RTRSession.readLoop` (`bgp/plugins/rpki/rtr_session.go`) | session | lacked | 3: DONE 24ff93be83, task `rpki/rtr-read`; Error Report Internal Error (RFC 8210 Sections 5.11, 12; none for an Error Report or before a PDU); the next cache in preference order loads, this one after its Retry Interval; End of Data lock fix `applyEndOfData` | TestRTRReadPanicContained, TestRTRReadPanicSkipsErrorReportForErrorReport |
| BMP receiver session `BMPPlugin.handleSession` (`bgp/plugins/bmp/bmp.go`) | session | lacked | 3: DONE 24ff93be83, task `bmp/receive`; the session's own defers close it, withdraw the router and remove it; nothing sent (RFC 7854 Section 3.2); the router reconnects | TestBMPReceiverPanicContained |
| BMP `acceptLoop`, sender `holdConnection`, `drainLoop`, `statisticsLoop`; RTR dial-cancel goroutine and `rtrDataLease.expire` | internal | none | none: parse no network input (the sender discards the collector's bytes uninterpreted) | - |
| LDP `runSession` / `Session.ReadLoop` (`plugins/ldp/register.go`, `session.go`) | session | lacked | 4: DONE d9fa580dce, task `ldp/session`; Internal Error Notification (RFC 5036 Section 3.5.1.2.7), Stop and onDone teardown; the next Hello restarts it | TestLDPReadPanicContained |
| LDP discovery reader `readDiscoveryLoop` (`plugins/ldp/register.go`) | neighbor | lacked: missing from the 043c682d24 audit | 4: DONE d9fa580dce, task `ldp/discovery`; the Hello is dropped, the same socket reads on | TestLDPDiscoveryPanicContained |
| RSVP-TE `rawTransport.readLoop` (`plugins/rsvpte/transport_linux.go`) | neighbor | lacked | 4: DONE 1f2ec7260a, task `rsvp-te/receive`; the datagram is dropped, the same socket reads on | TestRSVPReadPanicContained |
| RSVP-TE signaling worker `engine.run` / `handlePacket` (`plugins/rsvpte/engine.go`) | neighbor | lacked: one worker for every LSP, so a panic ended all RSVP signaling | 4: DONE 1f2ec7260a, e4bf3104a6, task `rsvp-te/signaling`; `tearLSPLocal` of the LSPs the message names (soft-state expiry path, RFC 2205 Sections 2.3, 2.4), admission released, only installed state withdrawn; restart by upstream PATH refresh or, at the head-end, the next commit | TestRSVPSignalingPanicContained, TestRSVPSignalingPanicTearsOnlyInstalledState |
| OSPFv2 `linuxInterface.readLoop`, `Transport.rxLoop` (`ospf/transport`) | neighbor | lacked | 5: DONE a736ea0800, tasks `ospf/interface-read`, `ospf/interface-fan-in`; the interface's receive ends through the link-down path | TestOSPFReadPanicContained, TestOSPFFanInPanicContained |
| OSPFv3 `linuxInterface.readLoop`, `Transport.rxLoop` (`ospf/v3/transport`) | neighbor | lacked | 5: DONE e89eae05b1, tasks `ospfv3/interface-read`, `ospfv3/interface-fan-in` | TestOSPFv3ReadPanicContained, TestOSPFv3FanInPanicContained |
| OSPF engine receive worker `receivePackets` (`ospf/instance.go`, `panic_guard.go`), both families | neighbor | lacked | 5: DONE bce5fe4915, task `ospf/receive`; the packet's neighbor goes Down through KillNbr (RFC 2328 Sections 10.2, 10.3), the worker resumes | TestOSPFReceivePanicContained |
| IS-IS `linuxCircuit.readLoop`, `Transport.rxLoop` (`isis/transport`) | neighbor | lacked | 5: DONE 4222fb02a9, tasks `isis/interface-read`, `isis/interface-fan-in` | TestISISReadPanicContained, TestISISFanInPanicContained |
| IS-IS engine receive worker `receivePDUs` (`isis/panic_guard.go`) | neighbor | lacked | 5: DONE f02d11be14, task `isis/receive`; the sender SNPA's adjacencies on that circuit go Down (`TeardownNeighbor`), the worker resumes | TestISISReceivePanicContained, TestISISTeardownNeighbor |
| BFD `UDP.readLoop` (`bfd/transport/udp.go`) | neighbor | lacked | 6: DONE 3417534caf, task `bfd/receive`; the packet is dropped, its slot returned | TestBFDReceivePanicContained |
| BFD express loop `Loop.serve` (`bfd/engine/loop.go`) | neighbor | lacked (engine recovered only a send on a closed channel, kept: a benign Unsubscribe race, not a defect) | 6: DONE 3417534caf, task `bfd/express`; the session goes Down with diag 0 and restarts by the three-way handshake | TestBFDReadPanicContained |
| VRRP `linuxInstance.readLoopV4`, `readLoopV6` (`vrrp/transport/backend_linux.go`) | neighbor | lacked | 6: DONE 0b9e085901, task `vrrp/receive` | TestVRRPReceivePanicContained |
| VRRP advertisement dispatcher `rxLoop` (`vrrp/panic_guard.go` `serveAdvertisements`) | neighbor | lacked: plugin-wide goroutine | 6: DONE 0b9e085901, task `vrrp/advertisement`; the one virtual router restarts through Shutdown and Startup (RFC 9568 Sections 6.4.2, 6.4.3) | TestVRRPReadPanicContained |
| VRRP `instance.run`, OSPF and IS-IS timers, LDP KeepAlive sender and adjacency expiry, RSVP-TE refresh and cleanup tickers | internal | none | none: timer or config driven, no network input | - |
| L2TP control dispatch `reactor.handle` (`l2tp/reactor.go`); `readLoop` and `tunnelReadLoop` only copy and queue | session (tunnel) | lacked (drain recovered only pool and auth handler calls) | 7: DONE 2de545d809, tasks `l2tp/control`, `l2tp/auth-handler`, `l2tp/pool-handler`; StopCCN Result Code 1 (RFC 2661 Sections 4.4.2, 6.4) | TestL2TPReadPanicContained |
| PPPoE discovery reader (`l2tp/pppoe/subsystem.go` `dispatchDiscovery`) | session | lacked | 7: DONE 558c13f782, tasks `pppoe/discovery`, `pppoe/auth-handler`, `pppoe/pool-handler`; before PADS the admission is removed, after PADS a PADT (RFC 2516 Section 5.5) | TestPPPoEReadPanicContained |
| IKE inbound dispatch and established SA (`ike/engine/panic_guard.go` `deliverDatagram`, `handleOwnedInboundGuarded`); `UDPTransport.Run` only copies | session (IKE SA) | lacked | 7: DONE 91502e4f4a, tasks `ike/inbound` (datagram dropped), `ike/established` (Delete then re-establish, RFC 7296 Section 2.4) | TestIKEInboundPanicContained, TestIKEEstablishedPanicSendsDelete |
| RADIUS client `Client.readLoop` (`radius/client.go`) | request | lacked | 8: DONE 2974d67589, task `radius/read`; the answer is dropped, the request retransmits or fails over | TestRADIUSClientReadPanicContained |
| TACACS+ client requests (`tacacs/client.go` `Authenticate`, `SendAuthorization`, `SendAccounting`) | request | lacked | 8: DONE dfe8925c99, task `tacacs/request`; answered as an unreachable server (RFC 8907 Section 5.4.3); one deferred close-and-evict in `trySend` (Section 4.4) | TestTACACSClientPanicContained |
| DHCP server `serveMulti` (`plugins/dhcpserver/register.go`) | request | lacked | 8: DONE c5e7232a16, task `dhcp/request`; no reply, the client retransmits (RFC 2131 Section 4.1); no DHCPv6 path exists | TestDHCPRequestPanicContained |
| TFTP listener `serve` and transfer `runTransfer` (`plugins/tftpserver/handler.go`) | request | lacked | 8: DONE 1c189da8be, tasks `tftp/request`, `tftp/transfer`; ERROR code 0 (RFC 1350 Section 2), slot and generation released by defer | TestTFTPRequestPanicContained, TestTFTPTransferPanicContained |
| RADIUS CoA listener (`l2tp/plugins/authradius/coa.go`) | request | lacked | 8: DONE cef77391f7, task `radius-coa/request`; NAK with Error-Cause 506 once authenticated (RFC 5176 Section 3.5), else dropped | TestCoAPanicContained |
| L2TP RADIUS login `doRADIUS` (`authradius/handler.go`) | request | had a log-only recover: not counted, not recorded | 8: DONE 2c0b7e2e01, task `radius-auth/request`; login denied "internal error" | TestRADIUSLoginPanicContained |
| L2TP RADIUS accounting goroutine `runAccounting` (`authradius/acct.go`) | request | lacked: missing from the 043c682d24 audit | 8: DONE 2c0b7e2e01, task `radius-auth/accounting`; that session's Start and Interim stop, Stop still sent (RFC 2866 Section 2) | TestRADIUSAccountingPanicContained |
| Plugin RPC `Conn.readLoop`, `MuxConn.readLoop` (`pkg/plugin/rpc`) | connection | lacked (per-call recovers in `sdk_dispatch.go`, `bridge.go` only) | 9: DONE 629502dff5, tasks `plugin/rpc-read`, `plugin/rpc-route`; the connection closes, the engine releases the plugin as one that exited | TestRPCReadLoopPanicContained, TestMuxReadLoopPanicContained |
| Plugin TLS acceptor `PluginAcceptor.handleConn` (`plugin/ipc/tls.go`) | connection | lacked | 9: DONE 3294897b53, task `plugin/connect-back` | TestPluginAcceptorPanicContained |
| Managed server `ManagedServer.handleConn` (`plugin/server/managed_serve.go`) | connection | lacked | 9: DONE 07e6474dfa, task `plugin/managed-client` | TestManagedServerPanicContained |
| SSH session handler and login checks (`ssh/panic_guard.go` `panicMiddleware`, `containAuth`) | connection | lacked a Ze boundary (charm.land/ssh `recoverAndLog` recovered log-only) | 9: DONE 56e4e6a9e1, tasks `ssh/session` (stderr message, exit 1), `ssh/auth` (login refused) | TestSSHHandlerPanicContained, TestSSHAuthPanicContained |
| gNMI unary RPC and Subscribe stream (`gnmi/panic_guard.go`) | request | lacked: grpc-go does not recover handler panics | 9: DONE 5b0daf545c, task `gnmi/rpc`; code Internal with a plain message; a Set's draft discarded by defer | TestGNMIHandlerPanicContained |
| NTP (`plugins/ntp`) | none | no Go receive loop in the package | none | - |

### Lock audit (A-1)
Phase 2 records here, for every lock the BGP read path takes (`s.mu`, `s.writeMu`,
`s.sendHoldMu`, `s.openMu`, `p.mu`, `reactor.mu`), whether it is released by
`defer` or never held across a call that can panic. A lock that fails both is fixed
in phase 2, because a held shared lock would stall every peer (AC-4).

| Lock | Scope | Read-path holds | Verdict |
|------|-------|-----------------|---------|
| `s.mu` | session | Field snapshots (`RLock`, read, `RUnlock`, no call inside), and the OPEN path's `Lock; s.peerOpen = open; Unlock` | Never held across a call that can panic |
| `s.openMu` | session | `session_read.go` and `session_handlers.go` (three sites) | Always `defer Unlock` |
| `s.writeMu` | session | Read-path writes go through `writeMessageWithin` (`defer`). `closeConn`'s flush held it with no `defer` | FIXED: `flushForClose` releases by `defer`. `connectionEstablished`'s hold covers field writes and env reads only |
| `s.sendHoldMu` | session | `startSendHoldTimer`, `stopSendHoldTimer` (`defer`); `sendHoldTimerCheck` holds it around a nil check and `Timer.Reset` only, on the timer's goroutine, not the read path | Never held across a call that can panic |
| FSM `f.mu` | session | All `defer`, except the callback drainer, which unlocks before the callback | A panicking callback leaves `draining` set on that session's FSM only, which ends with the unit |
| `p.mu` | peer | Snapshot reads in `notifyMessageReceiver` and the delivery worker | Never held across a call |
| reactor `r.mu` | reactor-wide | `notifyMessageReceiver` held `RLock` with no `defer` across about 120 lines that parse peer bytes (`wireUpdate.IsEOR`, `updateDelayEndOfRIB`, `typedNLRIHeld.endOfRIB`, health, forward weights) | FIXED: `reactorLockHeld` flag with a deferred conditional `RUnlock` |
| AIGP `state.mu` | reactor-wide | `commitAIGPWrites` unlocked without `defer` | FIXED: `defer Unlock` |

The sibling phases ran the same audit on each guarded task's path.

| Component | Lock | Finding | Verdict |
|-----------|------|---------|---------|
| RTR | `s.mu`, `lease.mu` | End of Data unlocked both by hand around `cache.Replace`, `ApplyDelta` and the ASPA cache | FIXED 24ff93be83: `applyEndOfData` releases both by `defer`; red: test timed out in `connectAndSync`'s state defer |
| BMP, LDP, DHCP, TFTP, TACACS+, gNMI, SSH, plugin IPC | component locks | Every hold on the task's path is deferred or copies fields only | Clean |
| L2TP | `tunnelsMu` | `reactor.handle` held it across `Process` with no `defer` | FIXED 2de545d809: `tunnelsHeld` flag and deferred conditional unlock; red: teardown deadlocked |
| RSVP-TE | `lsp.mu` | About 20 holds across calls on peer-derived data with no `defer` | FIXED f11f9eb9dc: `(*LSP).locked` and `*Locked` helpers; red: `lsp.mu` still held after the recovered panic |
| OSPF | engine `e.mu` | Receive-path holds in `handleHello`, `handleLSUpdate`, `handleLSAck`, `handleNeighborPacket`, `verifyPacket`, `neighborTopologyFacts`, `interfaceAreaType`, `graceRestarterV4`, `sendVirtualLink` | FIXED 3007251b43: deferred helpers (`receiveTarget` and others); red: TryLock failed, then shutdown hung |
| OSPF | graceful-restart `m.mu` | `helperEnter`, `updateGrace`, `helperExit` and `exitRestart` held it across metric calls and `time.AfterFunc` with no `defer`, and the receive worker reaches them (a received Grace-LSA, an LSDB content change, a neighbor reaching Full) | FIXED at closure (uncommitted): `addHelperSession`, `removeHelperSession`, `leaveRestart` and `updateGrace` release by `defer`; TestGRLockReleasedAfterContainedPanic, red on HEAD's two files in all three cases |
| OSPF | `e.mu` in self-origination | The phase 5 premise "origination never runs on the receive worker" is false: `handleLSUpdate` -> `grInspectV6Update` -> `onGraceReceived` -> `helperEnter` -> `originateSelfLSAs`, and the content-change observer -> `helperExit` -> `originateSelfLSAs` | Read at closure: `originateSelfLSAs`, `origination_v6*.go`, `ri.go`, `te_originate.go`, `ext_link.go`, `ext_prefix.go`, `opaque.go` hold `e.mu` only to copy fields, maps and the next LSID, with no call out: clean |
| IS-IS | circuit `c.mu` | `RunElection`'s second hold ran `st.Elect` on Hello-derived candidates with no `defer` | FIXED f02d11be14: `(*Circuit).elect`; red: `c.mu` still held. The engine's 34 `e.mu` and the `circuitsMu` sites copy fields only |
| BFD, VRRP | session and instance locks | `handleInbound`, `tick`, `onPacket` release by `defer` | Clean |
| Commit-confirm worker | none | Window state is worker-only, and `api/config_session.go` `(*ConfigSessionManager).Commit` copies the hooks under `m.mu` before `WriteOutside` | Clean |

A-1 is validated by this audit and by TestPeerReadPanicIsolation, where a
second peer keeps exchanging messages after the first one's read path panicked.
AC-6 consequence: after the fixes the panicking frame can no longer hold
`writeMu` when the guard runs, so TestSessionReadPanicWriteLockHeld models the
remaining holder, another writer, and `tryNotificationWithin` bounds the wait.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 Recover at the task entry: once per goroutine or per dispatched event, never in decode | per decode call; crash-fast (FRR, BIRD) | user brief; one deferred call per task costs nothing measurable (A-2); FRR and BIRD abort the whole daemon (`bug()` -> `abort()`, `assert()`), which on a single-process appliance is a device outage |
| D-2 BGP: an inner guard in `Session.Run`, keeping `safeRunOnce` as the outer one | widen `safeRunOnce` only | the outer recover fires after `Run`'s `closeConn` defer, so it can never send the NOTIFICATION; the two guard different tasks (read loop versus dial and setup), which is no layering |
| D-3 BGP NOTIFICATION Cease / Unspecific through `teardownAutomatic` | Out of Resources; no NOTIFICATION | RFC 4271 Section 6.7 allows Cease at any time absent a fatal Section 6 error; Section 4.5 sends Unspecific when no subcode fits; RFC 4486 has none for an internal failure. Event 8 (AutomaticStop) increments ConnectRetryCounter |
| D-4 The delivery-worker panic also sends Unspecific | keep Out of Resources | the same condition gets one answer; the "no receiver" branch keeps Out of Resources |
| D-5 The Cease is best effort and never blocks (TryLock, `holdExpiryNotifyDeadline`) | blocking send | the panicking frame may hold `writeMu`; a blocked guard would leak the session |
| D-6 One shared guard in `internal/core/taskguard` | a copy per package | one log shape, one metric, one crash record: one declaration (`ai/rules/principles.md`) |
| D-7 Metric `ze_task_panics_total{component,task}` | per-peer label | bounded cardinality across every protocol; the peer, interface or client is in the log line |
| D-8 Log at ERROR with component, task, unit, message type, panic value, stack, the restart behaviour, and `ze show crashes` | WARN; stack only | `ai/rules/user-facing-errors.md`: what, why, and the command |
| D-9 Crash record through `internal/core/crashlog` | log only | the operator finds a contained panic where crashes are listed |
| D-10 Restart and backoff stay the component's own: BGP's run-loop exponential backoff (no reset on error), each protocol's neighbor or session restart | a quarantine after N panics; a guard-owned restart loop | the backoff exists and already bounds a repeat offender; a second retry policy would layer over it |
| D-11 One spec, phases by protocol class | one spec per protocol | one policy, one guard, one audit table; each phase lands alone; the per-protocol RFC reading for the neighbor-down event belongs to its phase |

## Known Limitations
- A panic in a goroutine no guard covers still ends the process; the audit table is
  the inventory of network-input tasks, and internal workers keep their own recovers.
- Shared state a panic leaves half-updated (A-3) is resynced only by the unit
  restart; a site that phase work shows cannot be resynced is listed here by that
  phase, and containment never claims it.
- RSVP-TE: a label allocated but not yet recorded on the LSP when the panic
  fires is not returned; it is never reused (phase 4).
- Commit-confirm worker: a write that panics after part of the change applied
  leaves that part applied. The caller is told so in plain words, and the
  operator compares the running configuration before writing again.
- A Ze defect that repeats on every tick or every packet re-panics each time:
  each one is logged, counted and crash-recorded, bounded only by the crash
  record rotation (`crashlog.rotateRecoveredFiles`) and the component's backoff (R-2).
- Third-party goroutines Ze does not own keep their own log-only recovers:
  charm.land/ssh `recoverAndLog` (PTY copy and request goroutines) and the
  wish/bubbletea window-change goroutine.
- No SDK `Go` helper was built for goroutines an external plugin starts: they
  belong to the plugin author, and no AC asks for one.

## RFC Documentation

The BGP guard's Cease carries `// RFC 4271 Section 6.7` and `// RFC 4271 Section 4.5`
quotes above the statement that sends it. Each sibling phase quotes the RFC section
of the neighbor-down or session-end event it raises (OSPF RFC 2328 Section 10.2,
IS-IS ISO 10589, BFD RFC 5880 Section 6.8, VRRP RFC 5798, LDP RFC 5036, IKEv2 RFC 7296,
L2TP RFC 2661).

## Owner Questions

| Question | Options | Reading this spec proceeds under |
|----------|---------|----------------------------------|
| Is a lab interop scenario that injects a panic owed, given it needs an injection compiled only under a non-shipped build tag (`ze_le`) in the binary the lab runs? | (a) the Go wire test plus the existing Cease interop coverage is the evidence; (b) add a `ze_le`-only injection and a `bgp-panic-cease` scenario | Decided (b), by `ai/rules/interop-and-goal-validation.md`: a protocol behaviour change owes an interop test, and the Cease-on-panic is one. The injection compiles only under the `ze_le` build tag, which no shipped binary sets, so no production build carries a hook. Phase 2 owns the injection and the `bgp-panic-cease` scenario |
| The IS-IS end path (`isis/panic_guard.go` `endReceivePanic`) quotes "If the holding timer expires before another accepted IIH arrives, the IS MUST purge the adjacency and report adjacencyStateChange Down." from `rfc/full/iso-iec-10589.txt` Section 10.6. That file is a project-written "Independent Reconstruction", not ISO/IEC 10589. Is it an acceptable source for an RFC-style quote? | (a) keep it, and label every such quote as from the reconstruction; (b) obtain the ISO text and re-quote; (c) drop the quote and cite the section only | Open for Thomas. The code does not depend on the answer: the teardown follows the adjacency state machine either way |

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-27 |
| Checked at | 043c682d24 |
| Verdict | REWORK |
| Evidence | `Session.Run` is called inline by `runOnce` (`peer_run.go`, `return session.Run(p.ctx)`), which `safeRunOnce` wraps in a recover added by `986ff479dd` (2026-03-06) |
| Evidence | `TestPeerRunRecoversPanic` panics only the dialer; no test panics the read path |
| Evidence | `Run`'s `defer s.closeConn()` runs before the outer recover, so no NOTIFICATION is sent |
| Evidence | Recover search over the sibling packages: only DNS per query, L2TP pool and auth callbacks, OSPF registry decoders and plugin per-call bridges recover |

The skeleton's premise, that a BGP read-path panic kills the daemon, was already false
when it was written: the outer recover predates it. The goal still holds. BGP owes
the rest of the policy (NOTIFICATION, metric, crash record, a read-path test), and
every sibling receive task still lacks any boundary, so a panic there ends the
process. The spec was reworked in place into the policy, the shared guard and the
phased audit above.

## Implementation Summary

### What Was Implemented
- `internal/core/taskguard` (bfe10ac9c4): one boundary, `Task.Contain`, deferred
  directly at a task entry; logs at ERROR, counts
  `ze_task_panics_total{component,task}`, writes a recovered crash record
  (`crashlog.RecordRecoveredPanic`, `KindRecovered`, listed by `ze show crashes`).
  `crashlog.appendValue` prints every panic value (16480c0684).
- BGP (f258023eb5, 8fc41a50d9): inner `read` guard in `Session.Run` sends
  Cease / Unspecific through `ceaseAfterPanic` without blocking
  (`tryNotificationWithin`); `connect`, `cancel`, `delivery` and `accept` moved
  onto the guard; the delivery panic sends Unspecific. Lock fixes in
  `reactor_notify.go` and `aigp_readvertise.go`.
- Interop `bgp-panic-cease` (cefef77f15, 27d7e4f0ea, docs 523891fbf2): the
  `ze_le` lab daemon, the read-path panic seam reached by both readers, the
  checker, and interop.md row 60. Evidence under "Interop Tests".
- Siblings, one boundary per task in the audit table: RTR and BMP (24ff93be83),
  DNS (b505e144ab), LDP (d9fa580dce), RSVP-TE (f11f9eb9dc, 1f2ec7260a,
  e4bf3104a6), OSPF (a736ea0800, 3007251b43, b90a703ab5, bce5fe4915), OSPFv3
  (e89eae05b1), IS-IS (4222fb02a9, f02d11be14), BFD (3417534caf), VRRP
  (0b9e085901), L2TP (2de545d809), PPPoE (558c13f782), IKE (91502e4f4a),
  RADIUS (2974d67589), CoA (cef77391f7), DHCP (c5e7232a16), TFTP (1c189da8be),
  TACACS+ (dfe8925c99), plugin IPC (629502dff5, 3294897b53, 895f153c5e,
  07e6474dfa), SSH (56e4e6a9e1), gNMI (5b0daf545c).
- L2TP RADIUS login and accounting boundaries (2c0b7e2e01), the RFC 5176, 8907
  and 1350 discrimination re-records (b8ef092763, eb2c45ea85), and the tacacs
  `nolint` (f49d320bde).
- Closure review (uncommitted, commit A): the commit-confirm worker boundary
  (`config/confirm/panic_guard.go`, `Window.write`, metrics bound in
  `cmd/ze/hub/confirm_window.go`), and the OSPF graceful-restart lock fix
  (`gr_helper.go` `addHelperSession`, `removeHelperSession`, `updateGrace`;
  `gr_restarter.go` `leaveRestart`).

### Bugs Found/Fixed
- Silent swallows: DNS `Authoritative`/`Recursive` and the OSPF registry
  wrappers dropped a panic with no record when no `onPanic` was passed
  (TestQueryPanicNeverSilent, TestOSPFRenderPanicNeverSilent).
- Locks a panic could leave held: BGP, RTR, L2TP, RSVP-TE, OSPF (engine and
  graceful restart), IS-IS ("Lock audit (A-1)", each with a lock test).
- PPPoE `PppoxFD` zero closed fd 0 (558c13f782). RSVP-TE zero role was ingress
  (e4bf3104a6). RSVP-TE admission charge leaked on a contained panic
  (`releaseEverywhere`, 1f2ec7260a). TACACS+ `trySend` leaked a connection on
  any unwind (dfe8925c99). The gNMI Set draft leaked on an unwind (5b0daf545c).

### Documentation Updates
- `docs/guide/monitoring.md` "Contained Panics": one row per task with its
  source anchor; `docs/architecture/diagnostics/crash-capture.md` "Recovered
  panics"; `docs/plugin-development/metrics.md`; `peer-lifecycle.md`, `fsm.md`,
  `rib-storage-design.md` (BGP); each sibling's design page named in its commit;
  `process-protocol.md` and `fleet-config.md` (plugin IPC); at closure
  `docs/guide/config-editor.md` "Commit Confirmed" (worker boundary) and the
  DirectBridge sentence of `process-protocol.md`.

### Deviations from Plan
- The audit grew by rows the 043c682d24 audit missed (engine-op, LDP discovery,
  RADIUS accounting, commit-confirm worker, the OSPF registry wrappers, RSVP-TE
  and OSPF/IS-IS engine workers as separate rows).
- BGP gained a lab scenario (`bgp-panic-cease`) the plan had as an owner
  question; decided by the interop rule.
- L2TP's boundary sits at `reactor.handle`, not at `listener.go`: the readers
  copy and queue only. IKE's sits in the engine, not `transport/udp.go`, for
  the same reason.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/improve-5-panic-boundaries-ed626c22-d6a4-4293-a5f7-287a2deb8cf2.md` (12 files, verdict clean) |
| `./le spec review check` | `review_gate: OK (7 code files, clean, hashes match ...)` |
| Rounds | 4 |
| Reviewer lenses used | AC-8 re-walk of every network-input loop (a goroutine or worker that runs request work for network input), lock reachability from each guarded task, guard placement (deferred directly, task entry only, no per-message defer inside a read loop), goroutine lifecycle, error leakage to remote clients, docs against source |

### Run 1 (closure review, independent of every implementation phase)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | AC-8: every config write (gNMI Set, SSH, web and API commit, accept, abort, managed push) runs on the commit-confirm worker goroutine, outside the gNMI, SSH and web boundaries, so a Ze defect in a commit apply still ended the daemon | `config/confirm/confirm.go` `Window.do` | fixed (uncommitted): `panic_guard.go` `Window.write`, task `config-confirm/write` |
| 2 | ISSUE | A-1: the graceful-restart `m.mu` is held without `defer` across metric calls in `helperEnter`, `updateGrace`, `helperExit` and `exitRestart`, which the OSPF receive worker reaches; the phase 5 premise that origination never runs there is false | `ospf/gr_helper.go`, `ospf/gr_restarter.go` | fixed (uncommitted): `addHelperSession`, `removeHelperSession`, `updateGrace`, `leaveRestart` release by `defer` |
| 3 | NOTE | `process-protocol.md` said no engine boundary contains a DirectBridge call from a goroutine the plugin started; the calling task's own boundary does whenever that goroutine reads network input | `docs/architecture/api/process-protocol.md` | corrected in one edit |
| 4 | NOTE | Every one of the 61 `Contain` sites is `defer X.Contain(...)` (grep over `internal`, `pkg`, `cmd`, no non-deferred call); the remaining raw `recover()` sites in the touched packages are the internal rows of the audit table | - | none needed |

### Fixes applied
- Finding 1. Root cause: `Window.do` (`config/confirm/confirm.go`) hands the
  request closure to `Window.run`, a long-lived worker with no recover, so the
  caller's boundary (gNMI `containUnaryPanic`, the SSH middleware, `net/http`)
  sits on another goroutine. `[workaround]` run the apply on the caller's
  goroutine; `[source]` a boundary on the worker per request. Not the
  workaround: the worker exists to serialise writes against the window
  (owner decision 2026-10-12). Fix: `Window.write` defers
  `writeTask.Contain`, answers the caller a plain error (no panic value) naming
  the crash record and saying the write may be partly applied;
  `confirm.SetPanicMetrics` bound in `startConfirmWindow`; docs
  `config-editor.md`, `monitoring.md`. TestConfirmWritePanicContained: green
  `-race`; red by overlay (`answer <- fn()`): "panic: boom: apply defect" ends
  the test binary.
- Finding 2. Root cause: `grManager.helperEnter`, `helperExit`, `updateGrace`
  and `exitRestart` unlock by hand after metric calls; `handleLSUpdate` ->
  `grInspectV6Update` -> `onGraceReceived` and the content-change observer
  reach them on the receive worker. `[workaround]` none safe (TryLock in the
  end path guesses ownership); `[source]` release by `defer` in extracted
  helpers. TestGRLockReleasedAfterContainedPanic (nil metric under `m.mu`,
  three cases): green `-race`; red by overlay of HEAD's two files: "m.mu is
  still held after the panic under it was recovered" in all three.

### Run 3 (independent review of the two round-1 fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | A `commit confirmed` whose apply panics, contained by `Window.write`, left its window record on disk (saved before the apply) but no window in memory and no timer: the part-applied commit had no revert behind it, and the stale record would revert every later write at the next daemon start. The force path left the record's deadline apart from the armed one | `config/confirm/confirm.go` `Window.open`, `Window.Confirmed` | fixed: `Window.applyArmed`, reviewed clean in Run 4, arms the recorded window when Apply panics; TestConfirmedApplyPanicArmsWindow (open and force cases) |
| 2 | NOTE | OSPF graceful-restart lock: lock order unchanged (`originateSelfLSAs`, `grOriginateGraceLSAs`, `triggerAllSPF` still run after `m.mu` is released), behaviour unchanged; TestGRLockReleasedAfterContainedPanic green `-race`, red 3/3 on HEAD's two files | `ospf/gr_helper.go`, `ospf/gr_restarter.go` | none needed |
| 3 | NOTE | The deadline revert (`Window.run` -> `deadlineRevert`) is not a request and runs outside `Window.write`: a Ze defect in the revert still ends the daemon, and `RecoverOnStart` reverts at the next start. It is a timer task, not network input, so outside AC-8 | `config/confirm/confirm.go` `Window.run` | none in this spec |

- Run 3 finding 1. Root cause: `open` saves the record, then Apply; only the
  error return clears it, and `arm` runs only after a normal return, so a
  panic unwinding through `Window.write` skips both. `[workaround]` clear the
  record on panic, which leaves a part-applied commit with no revert;
  `[source]` arm the recorded window on the way out, which keeps the invariant
  `open` states (no commit applied with no revert behind it) and matches what a
  daemon crash did before the guard (the next start reverted). Docs:
  `config-editor.md` Commit Confirmed paragraph. TestConfirmedApplyPanicArmsWindow:
  green `-race -count=3` over the package; red by overlay of the pre-fix
  `confirm.go` (HEAD plus the `w.write(fn)` route): "no window is open after the
  panicked apply" (open) and "the window's deadline is 59m59.9s away" (force).
  TestConfirmWritePanicContained: green; red by overlay of HEAD's `confirm.go`
  ("panic: boom: apply defect" ends the test binary).

### Run 4 (independent review of the Run 3 fix, `Window.applyArmed`)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Panic: the deferred `arm(p)` runs once and the panic unwinds past the caller's `Clear` / `Save(previous)`, so the record on disk and the armed window are the same `Pending` value (force path: `next`, first rollback kept). Error: `panicked` is false, nothing is armed, the caller clears (open) or restores `previous` (force), and the previous window and timer stay. Success: one `arm`. No double arm or double clear is reachable. A later `accept`, `abort` or deadline acts on the armed window as on any other | `config/confirm/confirm.go` `Window.applyArmed` | none needed |
| 2 | NOTE | Restart: `recoverConfirmWindow` (`cmd/ze/hub/confirm_window.go`) loads the record before the config is read and `RecoverOnStart` reverts it, the same as any window open at shutdown (AC-19); there is no resume path, by design | `cmd/ze/hub/confirm_window.go` | none needed |
| 3 | NOTE | TestConfirmedApplyPanicArmsWindow: green `-race -count=3`, package green `-race`; red under the pre-fix overlay in both cases ("no window is open after the panicked apply", "the window's deadline is 59m59.999s away"). It does not compare the saved record's deadline with the shown one; the code passes the same value to both | `config/confirm/panic_guard_test.go` | none needed |

### Run 2 (over the Run 1 fixes only)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `Window.write` is deferred directly at the per-request entry on the worker; `deadlineRevert` stays unguarded as a timer task with no network input; worker state is touched only on the worker | `config/confirm/panic_guard.go` | none |
| 2 | NOTE | `leaveRestart` snapshots `cfg.RestartInterval` and `reason` under the lock, as the old code did; `exitRestart`'s later actions are unchanged | `ospf/gr_restarter.go` | none |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Commit-confirm worker outside every boundary | `config/confirm/confirm.go` `Window.do` | `config/confirm/panic_guard.go` `Window.write` (uncommitted, commit A) |
| 2 | ISSUE | Graceful-restart `m.mu` held across calls on the receive worker's path | `ospf/gr_helper.go`, `ospf/gr_restarter.go` | deferred helpers (uncommitted, commit A) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| BGP: peer told why (NOTIFICATION), metric, message type, crash record, read-path test | Done | `bgp/reactor/session.go` `Run`, `session_connection.go` `ceaseAfterPanic` | f258023eb5 |
| Every sibling network-input task contained | Done | audit table, every row DONE | 40 commits, plus the closure fixes in commit A |
| One policy, one shared guard | Done | `internal/core/taskguard/guard.go` `Task.Contain` | 61 deferred call sites |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestSessionReadPanicSendsCeaseAndCloses, TestSessionReadPanicCoalesced; interop `bgp-panic-cease` | wire octets asserted |
| AC-2 | Done | TestGuardRecordsPanic; each sibling test asserts count and a `KindRecovered` record | log fields in the taskguard test |
| AC-3 | Done | TestSessionReadPanicSendsCeaseAndCloses (`bufMuxStd.Stats()` balanced) | |
| AC-4 | Done | TestPeerReadPanicIsolation | |
| AC-5 | Done | TestDeliveryPanicSendsCeaseUnspecific; TestOwedKeyPublicationFailureTearsDownSource keeps Out of Resources for no receiver | |
| AC-6 | Done | TestSessionReadPanicWriteLockHeld | after the A-1 fixes the panicking frame cannot hold `writeMu`; the test models another writer |
| AC-7 | Done | every Test cell of the audit table | |
| AC-8 | Done | audit table: no row reads "lacks" | closure added the commit-confirm row |
| AC-9 | Done | TestGuardNoPanicIsSilent; every reader loops over a guarded pass (`for x.receive() {}`), never a defer per message inside a read loop | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestGuardRecordsPanic, TestGuardNoPanicIsSilent | Done | `internal/core/taskguard/guard_test.go` | |
| TestSessionReadPanic* (3), TestPeerReadPanicIsolation | Done | `bgp/reactor/session_panic_test.go` | |
| TestDeliveryPanicSendsCeaseUnspecific | Done | `bgp/reactor/panic_recovery_test.go` | |
| Sibling tests | Done | the Test column of the audit table | 46 named tests, all present (grep at closure) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/taskguard/guard.go`, `guard_test.go` | Done | |
| `bgp/reactor/session_panic_test.go` | Done | |
| `l2tp/listener.go`, `ike/transport/udp.go`, `ike/engine/inbound.go`, `tftpserver/socket_linux.go` | Changed | boundary placed where the work happens: `l2tp/reactor.go` `handle`, `ike/engine/panic_guard.go`, `tftpserver/handler.go` |

### Audit Summary
- **Total items:** 9 ACs, 3 task requirements, every audit row
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** boundary locations, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer cannot panic the daemon through BGP, and is told why | interop | `bgp-panic-cease` green twice (FRR receives Cease / Unspecific, session returns, BIRD session never drops); red with the Cease send removed (assertion 5) |
| Every sibling network-input task ends only its unit | unit at the real entry | the audit table's tests, each red by overlay with its guard removed (binary died) and green with it; e.g. TestOSPFReceivePanicContained keeps the eth1 neighbor while eth0's goes Down |
| A contained panic is visible to the operator | unit + docs | TestGuardRecordsPanic (ERROR line, metric, crash record); `monitoring.md` "Contained Panics" lists every task |
| No lock left held turns containment into a hang | unit | the lock tests in "Lock audit (A-1)", each red on the old code |
| No behaviour change without a panic | functional | `./le test bfd`, `vrrp`, `rsvpte`, `ldp`, `isis`, `ospfv3`, `ospf`, `pppoe`, `dhcp` green in their phases |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every audit row is DONE; the closure fixes land in commit A | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/taskguard/guard.go`, `guard_test.go` | yes | `ls internal/core/taskguard/` at closure |
| `internal/component/bgp/reactor/session_panic_test.go` | yes | grep found TestSessionReadPanicSendsCeaseAndCloses there |
| `internal/component/config/confirm/panic_guard.go`, `panic_guard_test.go`, `internal/plugins/ospf/gr_lock_panic_test.go` | yes | written at closure, tests run |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-7, AC-8 | every audit row has its test | closure grep: all 57 test names present, file per name |
| AC-8 | the commit-confirm worker is contained | `go test -race ./internal/component/config/confirm/`: ok; overlay red "panic: boom: apply defect" |
| AC-9 | guards deferred directly | grep: 61 `defer ... .Contain(`, 0 non-deferred `.Contain(` |
| A-1 | GR lock released | `go test -race -tags ze_ospf -run 'TestGRLock\|TestHelper\|GR\|Grace' ./internal/plugins/ospf/`: ok; overlay red in 3 of 3 cases |
| AC-8 | a panicked `commit confirmed` keeps its revert | `go test -race -count=3 -run TestConfirmedApplyPanicArmsWindow ./internal/component/config/confirm/`: ok; overlay of the pre-fix `confirm.go` red in both cases (open, force) |
| - | scoped lint over the 40 packages the spec's commits touched, plus `config/confirm` | 53 findings, one on this spec's lines (`bfd/engine/loop.go` `serve`, gocritic ptrToRefParam), exempted with its reason in its own commit; the rest are on lines other commits wrote (DelayOpen, IKE, L2TP drain, interoplab goconst) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| UPDATE read by `Session.Run` | none: a panic needs a seam no shipped binary holds; the lab scenario `test/interop/scenarios/bgp-panic-cease` drives the real daemon (`ze_le`) | green twice, red once |
| sibling entries | none (same reason); each Go test drives the real socket, pipe or handler | read at closure |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, then fixed | Lock audit (A-1) table: six components fixed with lock tests; Mistake Log |
| A-2 | confirmed | one defer per task or per dispatched event; AC-9 grep |
| A-3 | confirmed with limits | Known Limitations names each residual |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `monitoring.md` `config-confirm`/`write` row | `config/confirm/panic_guard.go` `writeTask`, `writeEndedByPanic` | yes |
| `config-editor.md` worker paragraph | `Window.do` -> `Window.write` | yes |
| `process-protocol.md` DirectBridge sentence | DirectBridge runs on the caller (`pkg/plugin/rpc/bridge.go`); BMP `handleSession` guard | yes |
| `./le doc check all` | exit 1 (Run 4 rerun, after the `applyArmed` doc sentence): every finding is foreign, none names a file of this spec: the published `dns-resolver` command pages missing from `../gh-pages`, and three anchors in `core-design.md`, `ipsec-9-ikev2-eap-nat.md`, `ci-workflows.md`. The `config-editor.md` anchors (`Window.write`, `writeEndedByPanic`, `Window.applyArmed`) resolve. The earlier own finding (`l2tp.md` `coa.go` anchor) was fixed | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] Every audit-table row has its boundary and its passing test (AC-8)
- [ ] Each phase's scoped package tests pass under `./le test unit` with `-race`; `./le verify changed` green on the closing commit set
- [ ] The full verification (`./le verify worktree`) is Thomas's release check: the closure report gives him the command, no agent runs it

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Race detector clean on new tests

### Integration Checklist
- [ ] The guard is reached from every audit-table entry point, proved by that row's test
- [ ] `ze_task_panics_total` appears in the Prometheus output of a component that recovered a panic
- [ ] A recovered panic is listed by `ze show crashes`

### Documentation Update Checklist
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - inner `Run` guard beside `safeRunOnce`
- [ ] `docs/architecture/behavior/fsm.md` - Cease subcode of a recovered delivery panic
- [ ] `docs/architecture/diagnostics/crash-capture.md` - recovered-panic records
- [ ] `docs/guide/monitoring.md` - `ze_task_panics_total`
- [ ] Each sibling phase's design page, as listed in Files to Modify

### Post-wave corrections (2026-07-10)

The DNS listeners added in July (`bindDoT`, `bindDoH`, `serveHTTP` in
`internal/core/dnsserver/secure.go`; `serve` in `internal/core/dnsserver/manager.go`;
as112 and geodns through `dnsserver.Authoritative`) all dispatch into the per-query
recover in `internal/core/dnsserver/handler.go`. They appear in the audit table's DNS
row with that inheritance, and phase 1 moves that recover onto the shared guard.
