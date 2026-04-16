# Spec: l2tp-6a -- PPP Scaffold + LCP Base

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-0-umbrella |
| Phase | 10/13 |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
3. `plan/learned/599-l2tp-5-kernel.md` -- Phase 5 hand-off (fds source)
4. `docs/research/l2tpv2-implementation-guide.md` sections 22, 26.7, Appendix C
5. `rfc/short/rfc1661.md` -- LCP FSM
6. `rfc/short/rfc2661.md` -- proxy LCP (Section 18)

## Task

Build the PPP package scaffold and a working LCP base layer that drives a
PPP session from kernel session creation to LCP-Opened state. Ship the
plumbing, the LCP FSM, and the auth-phase hook (stubbed); leave the auth
methods themselves to spec-l2tp-6b-auth and the NCPs to
spec-l2tp-6c-ncp.

| Capability | In Scope |
|------------|----------|
| `ppp` package layout | yes -- new `internal/component/ppp/` peer of `l2tp` |
| `ppp.Manager` lifecycle | yes -- Start, Stop, sessions-in / events-out channels |
| Per-session goroutine | yes -- one goroutine per active PPP session |
| PPP frame I/O on chan fd | yes -- `os.NewFile` wrap, blocking reads via Go runtime poller |
| LCP packet codec | yes -- Configure-Request/Ack/Nak/Reject, Terminate-Request/Ack, Code-Reject, Echo-Request/Reply, Discard-Request |
| LCP FSM | yes -- RFC 1661 ten states, full transition table |
| LCP option negotiation | yes -- MRU (type 1), Authentication-Protocol (type 3), Magic-Number (type 5), Async-Control-Character-Map (type 2), Protocol-Field-Compression (type 7), Address-and-Control-Field-Compression (type 8) |
| LCP Echo keepalive | yes -- periodic Echo-Request, drop session on N consecutive no-replies |
| Proxy LCP | yes -- RFC 2661 Section 18, skip negotiation when `proxyLastSentLCPConfReq` etc. populated |
| pppN MTU set | yes -- `iface.Backend.SetMTU` after LCP-Opened, MTU = MRU - 4 |
| Auth-phase hook | yes -- stub interface that always succeeds; 6b replaces with real auth |
| `kernelSetupSucceeded` event | yes -- new event from L2TP kernel worker -> reactor -> ppp.Manager |
| Authentication wire formats | NO -- spec-l2tp-6b-auth |
| IPCP / IPv6CP | NO -- spec-l2tp-6c-ncp |
| pppN address / route configuration | NO -- spec-l2tp-6c-ncp |

## Required Reading

### Architecture Docs

- [ ] `docs/research/l2tpv2-ze-integration.md` Section 11 -- concurrency model
  -> Decision: per-session goroutines for PPP (overrides Section 11.3 "PPP worker pool" wording, which is C/epoll thinking; in Go the runtime poller makes per-session blocking reads idiomatic and cheap)
  -> Constraint: PPP per-session goroutine reads chan fd via blocking I/O; cleanup on close
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 22 -- LNS PPP termination
  -> Constraint: order is kernel-session-create -> PPPoX socket -> chan fd -> unit fd -> LCP -> auth -> NCPs -> netlink configure
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 26.7 -- /dev/ppp ioctls
  -> Constraint: `PPPIOCSMRU` is the only ioctl needed in 6a (set MRU on unit fd after LCP negotiation)
- [ ] `.claude/rules/goroutine-lifecycle.md` -- "channel + worker" pattern, per-session OK
  -> Constraint: stop protocol = close fd, goroutine sees EBADF, exits, manager `WaitGroup` reaps
- [ ] `.claude/rules/buffer-first.md` -- offset writes for wire encoding
  -> Constraint: PPP frame and LCP packet encoding go into pooled buffers via `WriteTo(buf, off) int`, no `append`, no `make` in helpers
- [ ] `.claude/rules/api-contracts.md` -- document caller obligations
  -> Constraint: `ppp.Manager.Start`/`Stop` lifecycle documented; `StartSession` payload must populate fds before send

### RFC Summaries (MUST for protocol work)

- [ ] `rfc/short/rfc1661.md` -- LCP states, options, automaton
  -> Constraint: ten states (Initial, Starting, Closed, Stopped, Closing, Stopping, Req-Sent, Ack-Rcvd, Ack-Sent, Opened); twelve events; the actions listed in Section 4.3
- [ ] `rfc/short/rfc2661.md` Section 18 -- proxy LCP
  -> Constraint: when `Initial-Received-LCP-CONFREQ` (AVP 26), `Last-Sent-LCP-CONFREQ` (AVP 27), `Last-Received-LCP-CONFREQ` (AVP 28) are all present, LNS may skip LCP and use the proxied options as if negotiated

**Key insights:**
- LCP FSM is shared with NCPs (IPCP, IPv6CP) via RFC 1661 -- 6a builds the generic FSM machinery; 6c instantiates it for the NCPs
- PPP frame parser strips PPP HDLC framing the kernel does NOT remove on /dev/ppp (kernel hands raw frames; PFC/ACFC affect framing)
- Magic number is required for Echo loopback detection -- generate cryptographically random uint32 per session

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/l2tp/kernel_linux.go` -- `kernelWorker.run()` returns fds in `w.sessions[key]` after `setupSession`; today there is no success-path event back to the reactor (only `kernelSetupFailed`)
  -> Constraint: a new `kernelSetupSucceeded` event carrying `pppSessionFDs` must be added; mirrors the failure path
- [ ] `internal/component/l2tp/reactor.go` -- `select` loop reads `kernelErrCh`; no PPP integration today
  -> Constraint: add `kernelSuccessCh` to the select; dispatch successes to `ppp.Manager`
- [ ] `internal/component/l2tp/kernel_event.go` -- event type definitions
  -> Constraint: add `kernelSetupSucceeded` struct type
- [ ] `internal/component/l2tp/session.go` -- `L2TPSession` carries `proxyInitialRecvLCPConfReq`, `proxyLastSentLCPConfReq`, `proxyLastRecvLCPConfReq` from ICCN
  -> Constraint: pass these bytes verbatim to PPP via the `StartSession` payload; PPP decides whether to use them
- [ ] `internal/component/l2tp/pppox_linux.go` -- `pppSessionFDs` struct shape
  -> Constraint: PPP package re-defines an equivalent transport-agnostic struct (PPPoE will produce the same shape later)
- [ ] `internal/component/iface/backend.go` -- `Backend.SetMTU(name, mtu)` already exists
  -> Constraint: PPP calls `iface.GetBackend().SetMTU("ppp" + unitNum, mtu)` after LCP-Opened

**Behavior to preserve:**
- All existing L2TP behavior (Phases 1-5) unchanged
- `kernelSetupFailed` path unchanged
- `pppSessionFDs` struct shape unchanged in l2tp package (PPP defines its own copy)
- `iface.Backend` interface unchanged in 6a (extension comes in 6c if P2P address method is needed)

**Behavior to change:**
- L2TP kernel worker emits a new `kernelSetupSucceeded` event after fds are ready
- L2TP reactor dispatches that event to `ppp.Manager.SessionsIn()`
- L2TP reactor reads `ppp.Manager.EventsOut()` and acts on `EventSessionDown` (PPP-initiated teardown signals L2TP to send CDN)

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point

- L2TP session reaches `L2TPSessionEstablished` (handleICCN/handleOCCN)
- L2TP reactor enqueues `kernelSetupEvent` to kernel worker (existing Phase 5 flow)
- Kernel worker creates PPPoX socket, chan fd, unit fd
- Kernel worker emits new `kernelSetupSucceeded` event with fds + tunnel/session IDs + lnsMode + proxy LCP bytes (sourced from L2TPSession via reactor when forming the event)

### Transformation Path

1. L2TP reactor receives `kernelSetupSucceeded` from `kernelSuccessCh`
2. Reactor builds `ppp.StartSession` payload (chan fd, unit fd, unit num, MRU defaults, peer addr for logging, proxy LCP bytes if any) and writes to `ppp.Manager.SessionsIn()`
3. Manager spawns per-session goroutine, registers in `sessions` map under key `(tunnelID, sessionID)`
4. Goroutine wraps chan fd with `os.NewFile`, creates `bufio.Reader` over it (per-frame reads)
5. If proxy LCP bytes present: parse them, jump LCP FSM straight to Opened state with proxied options; send no LCP packets
6. Otherwise: drive LCP FSM through Up/Open events, Configure-Request/Ack/Nak/Reject negotiation, until Opened
7. On LCP-Opened: emit `EventLCPUp` on events channel; call `iface.Backend.SetMTU(pppN, negotiatedMRU - 4)`; call stubbed auth hook
8. Stub auth hook returns success immediately (6b replaces with real flow)
9. On stub success: emit `EventSessionUp` on events channel
10. Echo timer fires every `echo-interval` (default 10s); on N consecutive no-replies (default 3) emit `EventSessionDown` and exit goroutine
11. On manager `StopSession(id)`: close chan/unit fds, goroutine sees EBADF, cleans up, exits

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| L2TP reactor -> kernel worker | existing `kernelWorker.Enqueue(kernelSetupEvent)` | [ ] |
| Kernel worker -> L2TP reactor | new `kernelSuccessCh chan kernelSetupSucceeded` | [ ] |
| L2TP reactor -> PPP manager | `ppp.Manager.SessionsIn() chan<- StartSession` | [ ] |
| PPP per-session goroutine -> chan fd | `os.NewFile` + blocking `Read`/`Write` via Go runtime poller | [ ] |
| PPP -> L2TP reactor (events) | `ppp.Manager.EventsOut() <-chan Event` read in reactor select | [ ] |
| PPP -> netlink | `iface.GetBackend().SetMTU(name, mtu)` | [ ] |

### Integration Points
- `iface.GetBackend()` for MTU set on pppN
- `slogutil.Logger("ppp")` for logging
- `internal/component/l2tp/reactor.go` adds one `case kerr := <-r.kernelSuccessCh:` arm in `run()`
- `internal/component/l2tp/subsystem.go` constructs `ppp.NewManager(...)`, calls `Start`/`Stop` alongside reactor

### Architectural Verification
- [ ] No bypassed layers (PPP never imports l2tp; l2tp imports ppp only at the manager wiring point)
- [ ] No unintended coupling (PPP knows nothing about L2TP tunnel/session IDs structurally; treats them as opaque uint16 keys)
- [ ] No duplicated functionality (uses `iface.Backend` rather than rolling netlink; mirrors `kernelOps` pattern for ioctls)
- [ ] Zero-copy preserved where applicable (frame buffer pooled; LCP encode uses offset writes)

## Wiring Test (MANDATORY -- NOT deferrable)

Phase 5 precedent (learned/599): full `.ci` coverage with kernel modules + root + real peer is deferred to spec-l2tp-7-subsystem. Here we wire to a Go-level test that uses `net.Pipe` as the chan/unit fd substitute and a "test peer" goroutine that drives LCP packets from the other end. The PPP code path is identical (it sees an `io.ReadWriteCloser`).

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| L2TP reactor receives `kernelSetupSucceeded` | -> | `ppp.Manager.SessionsIn() <- StartSession` dispatch | `TestL2TPReactorDispatchesToPPPManager` (l2tp/reactor_ppp_test.go) |
| `ppp.Manager` receives StartSession | -> | spawns per-session goroutine, opens chan fd, runs LCP | `TestManagerStartSessionRunsLCP` (ppp/manager_test.go) |
| LCP CONFREQ from peer | -> | LCP FSM advances Req-Sent -> Ack-Sent -> Opened | `TestLCPFSMHappyPath` (ppp/lcp_fsm_test.go) |
| LCP-Opened reached | -> | `iface.Backend.SetMTU` called with negotiated MRU | `TestLCPOpenedSetsMTU` (ppp/manager_test.go, fake backend) |
| Proxy LCP AVPs present | -> | LCP jumps to Opened without sending packets | `TestProxyLCPSkipsNegotiation` (ppp/proxy_test.go) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | L2TP kernel worker completes `setupSession` successfully | Worker emits `kernelSetupSucceeded` event with fds + IDs |
| AC-2 | Reactor receives `kernelSetupSucceeded` | Reactor writes `ppp.StartSession` to `Manager.SessionsIn()` |
| AC-3 | `ppp.Manager` receives `StartSession` | Manager registers session in map; per-session goroutine started |
| AC-4 | Peer sends LCP CONFREQ with MRU=1500, Auth-Proto=PAP, Magic=X | PPP responds with CONFACK, FSM transitions Req-Sent -> Ack-Sent -> Opened |
| AC-5 | LCP reaches Opened | `EventLCPUp` emitted on events channel; `iface.Backend.SetMTU(name, mru-4)` called |
| AC-6 | Stub auth hook called after LCP-Opened | Hook returns success; `EventSessionUp` emitted |
| AC-7 | Peer sends LCP Echo-Request | PPP replies with Echo-Reply carrying same Magic-Number and Identifier |
| AC-8 | PPP sends Echo-Request, no reply for `echo-failures` consecutive intervals | `EventSessionDown` emitted; goroutine exits |
| AC-9 | Peer sends LCP Terminate-Request | PPP replies with Terminate-Ack; FSM transitions Opened -> Stopped; `EventSessionDown` emitted |
| AC-10 | Peer sends LCP CONFREQ with unknown option (mandatory) | PPP responds with CONFREJ listing the unknown option |
| AC-11 | Peer sends LCP CONFREQ with MRU=2000 (above local max) | PPP responds with CONFNAK suggesting local max |
| AC-12 | Manager `StopSession(id)` called | chan/unit fds closed; per-session goroutine exits within 100ms; entry removed from sessions map |
| AC-13 | StartSession payload contains proxy LCP AVPs | LCP FSM jumps to Opened state with proxied options; no packets sent on chan fd |
| AC-14 | LCP-Opened with negotiated MRU=1460 | `SetMTU` called with 1456 (MRU - 4 for PPP overhead) |
| AC-15 | Manager `Start()` and `Stop()` called | All session goroutines reaped on Stop; second Start after Stop is rejected |
| AC-16 | Two concurrent sessions on different tunnels | Each runs independent LCP; events tagged with correct (tunnelID, sessionID) |
| AC-17 | Goroutine reads from chan fd, fd closed externally | Read returns EBADF (or `os.ErrClosed`); goroutine logs and exits cleanly |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPFrameParse` | `internal/component/ppp/frame_test.go` | PPP frame protocol field decode (PFC and non-PFC) | |
| `TestPPPFrameWriteTo` | `internal/component/ppp/frame_test.go` | Frame serialization, offset writes, no allocs in helper | |
| `TestLCPPacketParse` | `internal/component/ppp/lcp_test.go` | LCP code/identifier/length/data parse with bounds checks | |
| `TestLCPPacketWriteTo` | `internal/component/ppp/lcp_test.go` | LCP serialization round-trip | |
| `TestLCPOptionsParse` | `internal/component/ppp/lcp_options_test.go` | All six supported options parse correctly | |
| `TestLCPOptionsNegotiate` | `internal/component/ppp/lcp_options_test.go` | Local MRU clamp, Auth-Proto Nak, Magic propose | |
| `TestLCPFSMHappyPath` | `internal/component/ppp/lcp_fsm_test.go` | Up -> Open -> CONFREQ/CONFACK -> Opened transitions per RFC 1661 §4.3 |
| `TestLCPFSMRetransmit` | `internal/component/ppp/lcp_fsm_test.go` | CONFREQ retransmit on no reply within timeout |
| `TestLCPFSMTerminate` | `internal/component/ppp/lcp_fsm_test.go` | Opened -> Closing on Terminate-Request; Stopped on ack |
| `TestLCPFSMCodeReject` | `internal/component/ppp/lcp_fsm_test.go` | Unknown LCP code triggers Code-Reject |
| `TestLCPEchoReply` | `internal/component/ppp/echo_test.go` | Echo-Request triggers Echo-Reply with matched Identifier+Magic |
| `TestLCPEchoTimeout` | `internal/component/ppp/echo_test.go` | N consecutive no-replies emits SessionDown |
| `TestProxyLCPParse` | `internal/component/ppp/proxy_test.go` | Decode proxied AVP bytes into LCP options |
| `TestProxyLCPSkipsNegotiation` | `internal/component/ppp/proxy_test.go` | Manager spawns goroutine that emits LCPUp without sending CONFREQ |
| `TestManagerStartStop` | `internal/component/ppp/manager_test.go` | Start/Stop lifecycle, second Start rejected, all goroutines reaped |
| `TestManagerStartSessionRunsLCP` | `internal/component/ppp/manager_test.go` | StartSession with net.Pipe peer drives LCP to Opened |
| `TestManagerStopSession` | `internal/component/ppp/manager_test.go` | StopSession closes fds, goroutine exits within 100ms |
| `TestManagerSessionByID` | `internal/component/ppp/manager_test.go` | Locked read of session state during goroutine activity |
| `TestLCPOpenedSetsMTU` | `internal/component/ppp/manager_test.go` | Fake `iface.Backend` records `SetMTU` call with mru-4 |
| `TestPPPOpsSetMRU` | `internal/component/ppp/ops_test.go` | Stub `pppOps.setMRU` records the unit fd + mru it was called with |
| `TestL2TPReactorDispatchesToPPPManager` | `internal/component/l2tp/reactor_ppp_test.go` | `kernelSetupSucceeded` -> Manager.SessionsIn() write |
| `TestKernelWorkerEmitsSucceeded` | `internal/component/l2tp/kernel_linux_test.go` | After fake `pppSetup` returns fds, worker writes `kernelSetupSucceeded` to errCh's success twin |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MRU | 64-65535 | 65535 | 63 | N/A (uint16 caps) |
| LCP packet length | 4-1500 | 1500 | 3 | 1501 (treated as malformed) |
| LCP option length | 2-255 | 255 | 1 | N/A (uint8 caps) |
| Echo failures threshold | 1-255 | 255 | 0 | N/A (uint8 caps) |
| Echo interval | 1s-3600s | 3600s | 0s | N/A (uint16 seconds caps) |
| LCP retransmit attempts | 1-10 | 10 | 0 | 11 |
| Magic number | non-zero uint32 | 0xFFFFFFFF | 0 | N/A |

### Functional Tests

Per `rules/integration-completeness.md` deferral precedent (Phase 5 / learned/599): kernel-dependent `.ci` is deferred to spec-l2tp-7-subsystem. PPP unit testing uses `net.Pipe` as the fd substitute, which exercises the entire PPP code path.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-lcp-net-pipe` | `internal/component/ppp/manager_test.go::TestManagerStartSessionRunsLCP` | LNS-side LCP completes against scripted peer over net.Pipe | |

### Future (if deferring any tests)

- `.ci` test against accel-ppp as peer -- deferred to spec-l2tp-7-subsystem (recorded in plan/deferrals.md)

## Files to Modify

- `internal/component/l2tp/kernel_event.go` -- add `kernelSetupSucceeded` event type carrying `pppSessionFDs` + IDs + lnsMode
- `internal/component/l2tp/kernel_linux.go` -- `setupSession` emits `kernelSetupSucceeded` after fds obtained; new field `successCh chan<- kernelSetupSucceeded` on `kernelWorker`
- `internal/component/l2tp/kernel_other.go` -- non-Linux signature parity
- `internal/component/l2tp/reactor.go` -- new `kernelSuccessCh <-chan kernelSetupSucceeded`; `select` arm; new `dispatchToPPP(kerr kernelSetupSucceeded)` helper; new `pppManager` field; `EventsOut()` reader handling `EventSessionDown` (sends CDN)
- `internal/component/l2tp/subsystem.go` -- construct `ppp.NewManager(logger)`, `Start`/`Stop` ordering: subsystem Start -> ppp.Start -> reactor.Start; subsystem Stop -> reactor.Stop -> ppp.Stop -> kernel worker stop
- `internal/component/l2tp/subsystem_test.go` -- adapt for `ppp.Manager` lifecycle assertions

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A in 6a; LCP knobs (echo-interval, echo-failures, max-mru) come via env vars |
| CLI commands/flags | [ ] | N/A in 6a; `show l2tp session` extension comes in 6c when session is "user-up" |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `internal/component/ppp/manager_test.go` (Go-level, `.ci` deferred to Phase 7) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A in 6a (PPP not yet user-reachable end-to-end; that's 6c) |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A (PPP wire format only emitted; not new to ze docs scope) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc1661.md` -- mark LCP FSM enforcement points |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A in 6a |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- add `ppp` component as peer of `l2tp`; note transport-agnostic boundary for PPPoE later |

## Files to Create

- `internal/component/ppp/doc.go` -- package documentation
- `internal/component/ppp/manager.go` -- `Manager` type, `Start`/`Stop`/`SessionsIn`/`EventsOut`/`StopSession`/`SessionByID`
- `internal/component/ppp/session.go` -- `pppSession` struct with FSM state, fds, mutex
- `internal/component/ppp/start_session.go` -- `StartSession` payload type (transport-agnostic)
- `internal/component/ppp/events.go` -- `Event` sealed sum (EventLCPUp, EventLCPDown, EventSessionUp, EventSessionDown)
- `internal/component/ppp/frame.go` -- PPP frame parser/serializer (transport-agnostic)
- `internal/component/ppp/frame_linux.go` -- chan/unit fd `os.NewFile` wrapping
- `internal/component/ppp/frame_other.go` -- non-Linux stubs
- `internal/component/ppp/lcp.go` -- LCP packet types (Code/Identifier/Length/Data)
- `internal/component/ppp/lcp_fsm.go` -- ten-state FSM, transition table
- `internal/component/ppp/lcp_handlers.go` -- per-state handlers if `lcp_fsm.go` exceeds modularity threshold
- `internal/component/ppp/lcp_options.go` -- option types, parse, serialize, negotiation
- `internal/component/ppp/echo.go` -- Echo-Request/Reply handler + timer
- `internal/component/ppp/proxy.go` -- proxy LCP AVP decode + FSM short-circuit
- `internal/component/ppp/ops.go` -- `pppOps` struct (one func field for `PPPIOCSMRU`)
- `internal/component/ppp/mtu_linux.go` -- real `setMRU` ioctl wrapper
- `internal/component/ppp/mtu_other.go` -- non-Linux stub
- `internal/component/ppp/auth_hook.go` -- stub `AuthHook` interface that always returns success; 6b replaces

Test files (one per source file, plus shared `helpers_test.go` for `net.Pipe` peer driver):
- `internal/component/ppp/manager_test.go`
- `internal/component/ppp/session_test.go`
- `internal/component/ppp/frame_test.go`
- `internal/component/ppp/lcp_test.go`
- `internal/component/ppp/lcp_fsm_test.go`
- `internal/component/ppp/lcp_options_test.go`
- `internal/component/ppp/echo_test.go`
- `internal/component/ppp/proxy_test.go`
- `internal/component/ppp/ops_test.go`
- `internal/component/ppp/helpers_test.go`
- `internal/component/ppp/export_test.go`
- `internal/component/l2tp/reactor_ppp_test.go`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + learned/599 |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify-fast` |
| 5-12 | Standard flow |

### Implementation Phases

1. **Package skeleton + types** -- `doc.go`, `start_session.go`, `events.go`, `auth_hook.go` stub. Package compiles; no tests yet.
2. **PPP frame codec** -- `frame.go` parse/serialize. Tests: `TestPPPFrameParse`, `TestPPPFrameWriteTo`. RFC 1661 §2 (encapsulation).
3. **LCP packet codec** -- `lcp.go` parse/serialize. Tests: `TestLCPPacketParse`, `TestLCPPacketWriteTo`.
4. **LCP option codec + negotiation** -- `lcp_options.go`. Tests: `TestLCPOptionsParse`, `TestLCPOptionsNegotiate`.
5. **LCP FSM** -- `lcp_fsm.go`. Tests: `TestLCPFSMHappyPath`, `TestLCPFSMRetransmit`, `TestLCPFSMTerminate`, `TestLCPFSMCodeReject`. Reference: RFC 1661 §4.
6. **Echo handler** -- `echo.go`. Tests: `TestLCPEchoReply`, `TestLCPEchoTimeout`.
7. **Proxy LCP** -- `proxy.go`. Tests: `TestProxyLCPParse`, `TestProxyLCPSkipsNegotiation`.
8. **fd I/O wrappers** -- `frame_linux.go` / `frame_other.go`. No new tests; exercised via manager tests.
9. **MRU ioctl** -- `ops.go`, `mtu_linux.go` / `mtu_other.go`. Test: `TestPPPOpsSetMRU` (with stub function field).
10. **Manager** -- `manager.go`, `session.go`. Tests: `TestManagerStartStop`, `TestManagerStartSessionRunsLCP`, `TestManagerStopSession`, `TestManagerSessionByID`, `TestLCPOpenedSetsMTU`. Uses `helpers_test.go` for `net.Pipe` peer driver and fake `iface.Backend`.
11. **L2TP integration** -- modify `kernel_event.go`, `kernel_linux.go`, `kernel_other.go`, `reactor.go`, `subsystem.go`. Tests: `TestKernelWorkerEmitsSucceeded`, `TestL2TPReactorDispatchesToPPPManager`. Adapt `subsystem_test.go`.
12. **RFC refs** -- add `// RFC 1661 Section X.Y` annotations; create updated `rfc/short/rfc1661.md` if Section 4 not already covered.
13. **Functional verification** -- `make ze-verify-fast`; race detector with `make ze-race-reactor` (l2tp reactor concurrency code touched).

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a Go test naming a file:line and the assertion verifies the AC behavior, not a proxy |
| Correctness | LCP FSM transitions byte-for-byte match RFC 1661 §4.3 actions table; option negotiation respects MRU clamp and Auth-Proto Nak |
| Naming | Package is `ppp` (no hyphens); types `Manager`, `Session`, `LCPState`; events `EventLCPUp` etc. |
| Data flow | PPP imports nothing from `l2tp` package (verify with `go list -deps`); `L2TPSession` proxy fields passed verbatim, not type-coupled |
| Rule: buffer-first | LCP encode helpers use offset writes, no `append`, no `make` |
| Rule: goroutine-lifecycle | One per session; close-fd shutdown; `WaitGroup` reaps |
| Rule: api-contracts | `Manager.Start`, `Manager.Stop`, `Manager.StopSession` document obligations |
| Rule: design-doc-references | Every new `.go` file has `// Design:` annotation |
| Rule: related-refs | Sibling references between `lcp.go` / `lcp_fsm.go` / `lcp_options.go` |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/ppp/` package exists with documented Manager | `ls internal/component/ppp/` and `go doc codeberg.org/thomas-mangin/ze/internal/component/ppp` |
| `kernelSetupSucceeded` event flows from worker -> reactor | `TestKernelWorkerEmitsSucceeded` + `TestL2TPReactorDispatchesToPPPManager` pass |
| LCP reaches Opened against scripted peer | `TestManagerStartSessionRunsLCP` passes |
| Proxy LCP short-circuit works | `TestProxyLCPSkipsNegotiation` passes |
| MTU set on pppN after LCP-Opened | `TestLCPOpenedSetsMTU` passes (fake backend records call) |
| Echo keepalive triggers SessionDown on N misses | `TestLCPEchoTimeout` passes |
| Auth-phase hook exists and is reachable | `grep -rn AuthHook internal/component/ppp/` shows interface + stub impl + invocation site |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | LCP packet length validated against frame length before reading data; option length validated against packet remaining |
| Resource exhaustion | Per-session retransmit count bounded; Echo failure threshold bounded; max sessions bound enforced by L2TP layer |
| Magic number quality | Generated via `crypto/rand`, non-zero |
| fd leak | Every error path before `goroutine.Done()` closes both chan and unit fds |
| Stub auth | `auth_hook.go` stub MUST log `WARN: PPP auth stub returning success -- replace in spec-l2tp-6b-auth` so a partial deploy is visibly insecure |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 1661 §4 and the Section 18 proxy text |
| Race detector failure under `ze-race-reactor` | Lock order audit on `tunnelsMu` interaction with new dispatch path |
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

- The L2TP reactor today only hears about kernel FAILURES; success was implicit (worker stored fds in its map, no notification). Phase 6 forces the symmetric event because PPP needs the fds. Adding `kernelSetupSucceeded` is a small, isolated change; in retrospect Phase 5 could have included it.
- PPP is transport-agnostic by design (PPPoE will produce the same chan/unit fds). Putting PPP under `internal/component/ppp/` rather than `internal/component/l2tp/ppp/` honors that.
- The "PPP worker pool" wording in `docs/research/l2tpv2-ze-integration.md` Section 11.3 is C/epoll thinking. Per-session goroutines via Go's runtime poller are the idiomatic Go answer and what high-perf code in ze already does.

## RFC Documentation

Add `// RFC 1661 Section X.Y: "<quoted requirement>"` above:
- LCP FSM state transitions (Section 4.3)
- Option negotiation rules (Section 5)
- Magic-Number loopback detection (Section 6.4)
- Echo-Request/Reply handling (Section 5.8)
- Code-Reject generation for unknown codes (Section 5.7)

Add `// RFC 2661 Section 18: "..."` above proxy LCP short-circuit logic.

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes (l2tp reactor concurrency touched)
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC 1661 constraint comments added
- [ ] RFC 2661 §18 constraint comment on proxy short-circuit
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (PPP does not import l2tp)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (Go-level via net.Pipe; .ci deferred to Phase 7)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
