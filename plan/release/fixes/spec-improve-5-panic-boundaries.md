# Spec: improve-5 -- Panic Containment at Network-Input Task Boundaries

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P0 |
| Scope | protocol |
| Release reason | Untrusted network input must not let one session panic terminate the whole control plane. |
| Depends | - |
| Phase | - |
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
| A-1 | No lock is held across a call that can panic in a guarded task, so the teardown after a recover does not block | Ze's lock discipline uses defer unlock on most paths | The teardown deadlocks the unit, or a shared reactor lock stalls every peer | Phase 2 audit of every lock taken in the read path (`s.mu`, `s.writeMu`, `p.mu`, `reactor.mu`); each is released by defer or never held across a callback; AC-4 proves the second peer still progresses | unvalidated |
| A-2 | One deferred call per task (per session, per goroutine, per dispatched event) costs nothing measurable | a defer at function entry is open-coded by the Go compiler | Hot path slows | The guard is never per message inside `readAndProcess*`; `Run` defers once per session; AC-9 | validated by placement |
| A-3 | Shared state touched mid-panic (RIB, pools, LSDB) stays consistent enough for the other units | pools use get/return discipline; a link-state LSDB resyncs when the adjacency restarts | Containment gives false safety | Each phase names the shared state its task mutates and the unit teardown that resyncs it; a site that cannot be contained is listed in Known Limitations | unvalidated |

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

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

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
| BGP `Session.Run` read path (`reactor/session.go`) | session | outer only: `safeRunOnce` (`peer_run.go`), no NOTIFICATION, no metric | 2 | TestSessionReadPanicSendsCeaseAndCloses |
| BGP session cancel goroutine (`session.go`) | session | has it | 2 (moves to the guard) | existing |
| BGP delivery worker (`peer_run.go`) | session | has it, Out of Resources | 2 | TestDeliveryPanicSendsCeaseUnspecific |
| BGP listener accept and handler (`reactor/listener.go`) | connection | has it | 2 (moves to the guard) | TestListenerHandlerRecoversPanic |
| BGP reactor monitor, notify, signal, forward pool and build (`reactor.go`, `reactor_notify.go`, `signal.go`, `forward_pool.go`, `forward_build.go`) | internal | has it | none: not network input | existing |
| Plugin engine event dispatch (`plugin/server/engine_event.go`) | internal | has it | none | existing |
| Internal plugin runner goroutine (`plugin/process/process.go`) | connection | has it, its child goroutines do not | 9 | existing |
| DNS per query, incl. DoT and DoH, as112, geodns (`core/dnsserver/handler.go` `Authoritative`) | request | has it (DoH also through `net/http`) | 1 (moves to the guard) | existing handler tests |
| Web UI, looking glass (`net/http` servers) | request | has it: `net/http` recovers per request | none | - |
| RTR `RTRSession.readLoop` (`bgp/plugins/rpki/rtr_session.go`) | session | lacks | 3 | TestRTRReadPanicContained |
| BMP receiver and session readers (`bgp/plugins/bmp/receiver.go`, `bmp.go`) | session | lacks | 3 | TestBMPReceiverPanicContained |
| LDP `Session.ReadLoop` (`plugins/ldp/session.go`) | session | lacks | 4 | TestLDPReadPanicContained |
| RSVP-TE `rawTransport.readLoop` (`plugins/rsvpte/transport_linux.go`) | neighbor | lacks | 4 | TestRSVPReadPanicContained |
| OSPFv2 `linuxInterface.readLoop` and its `recvCh` worker | neighbor | lacks (only plugin-registered opaque decoders recover) | 5 | TestOSPFReadPanicContained |
| OSPFv3 `linuxInterface.readLoop` and its worker | neighbor | lacks | 5 | TestOSPFv3ReadPanicContained |
| IS-IS `linuxCircuit.readLoop` and its worker | neighbor | lacks | 5 | TestISISReadPanicContained |
| BFD `UDP.readLoop` (`bfd/transport/udp.go`) and engine | neighbor | lacks (engine recovers only a send on a closed channel) | 6 | TestBFDReadPanicContained |
| VRRP `linuxInstance.readLoopV4`, `readLoopV6` | neighbor | lacks | 6 | TestVRRPReadPanicContained |
| L2TP `UDPListener.readLoop`, `tunnelReadLoop` (`l2tp/listener.go`) | session (tunnel) | lacks (drain recovers only pool and auth handler calls) | 7 | TestL2TPReadPanicContained |
| PPPoE discovery reader (`l2tp/pppoe/kernel_linux.go`) | session | lacks | 7 | TestPPPoEReadPanicContained |
| IKE UDP reader and inbound processing (`ike/transport/udp.go`, `ike/engine/inbound.go`) | session (IKE SA) | lacks | 7 | TestIKEInboundPanicContained |
| RADIUS client `Client.readLoop` (`radius/client.go`) | request | lacks | 8 | TestRADIUSClientReadPanicContained |
| TACACS+ client reads (`tacacs/client.go`) | request | lacks | 8 | TestTACACSClientPanicContained |
| DHCP server (`plugins/dhcpserver/register.go`) | request | lacks | 8 | TestDHCPRequestPanicContained |
| TFTP server (`plugins/tftpserver`) | request | lacks | 8 | TestTFTPRequestPanicContained |
| RADIUS CoA listener (`l2tp/plugins/authradius/coa.go`) | request | lacks | 8 | TestCoAPanicContained |
| Plugin RPC `Conn.readLoop`, `MuxConn.readLoop` (`pkg/plugin/rpc`) | connection | lacks (per-call recovers in `sdk_dispatch.go`, `bridge.go` only) | 9 | TestRPCReadLoopPanicContained, TestMuxReadLoopPanicContained |
| Plugin TLS acceptor `PluginAcceptor.handleConn` (`plugin/ipc/tls.go`) | connection | lacks | 9 | TestPluginAcceptorPanicContained |
| Managed server `ManagedServer.handleConn` (`plugin/server/managed_serve.go`) | connection | lacks | 9 | TestManagedServerPanicContained |
| SSH server handlers (`ssh/ssh.go`, `x/crypto/ssh`) | connection | lacks | 9 | TestSSHHandlerPanicContained |
| gNMI server (`gnmi/server.go`, only auth interceptors) | request | lacks: grpc-go does not recover handler panics | 9 | TestGNMIHandlerPanicContained |
| NTP (`plugins/ntp`) | none | no Go receive loop in the package | none | - |

### Lock audit (A-1)
Phase 2 records here, for every lock the BGP read path takes (`s.mu`, `s.writeMu`,
`s.sendHoldMu`, `s.openMu`, `p.mu`, `reactor.mu`), whether it is released by
`defer` or never held across a call that can panic. A lock that fails both is fixed
in phase 2, because a held shared lock would stall every peer (AC-4).

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

## RFC Documentation

The BGP guard's Cease carries `// RFC 4271 Section 6.7` and `// RFC 4271 Section 4.5`
quotes above the statement that sends it. Each sibling phase quotes the RFC section
of the neighbor-down or session-end event it raises (OSPF RFC 2328 Section 10.2,
IS-IS ISO 10589, BFD RFC 5880 Section 6.8, VRRP RFC 5798, LDP RFC 5036, IKEv2 RFC 7296,
L2TP RFC 2661).

## Owner Questions

| Question | Options | Reading this spec proceeds under |
|----------|---------|----------------------------------|
| Is a lab interop scenario that injects a panic owed, given it needs an injection compiled only under a non-shipped build tag (`ze_le`) in the binary the lab runs? | (a) the Go wire test plus the existing Cease interop coverage is the evidence; (b) add a `ze_le`-only injection and a `bgp-panic-cease` scenario | (a): no injection code outside `_test.go` files |

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
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
