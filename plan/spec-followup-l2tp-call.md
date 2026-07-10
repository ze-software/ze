# Spec: followup-l2tp-call

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-07-10 |

## Session Progress (2026-07-10 closure)

Last-mile session (AC-4 functional .ci + AC-7 interop + closure), on top of the
seven committed phases (b68e7e9c9 → c44fe82d5):

| Item | Deliverable | State |
|------|-------------|-------|
| AC-4 functional .ci | `test/l2tp/tunnel-initiate-sccrq.ci`, `test/l2tp/lns-outgoing-call.ci` | DONE — both PASS. ze dials via the `request l2tp outgoing-call` RPC over the token-guarded REST API (`/api/v1/execute`); a fixed-port ($PORT2) inline Python peer answers SCCRP/OCRP/OCCN. First full-wire OCRP/OCCN proof (Go tests stop at OCRQ-sent). |
| AC-7 interop (real peer) | `test/l2tp-interop/scenarios/03-ze-lac-xl2tpd-lns/` (self-contained runner + `run.py` delegation) | DONE — PASS. ze (initiator) establishes an L2TP control connection with a real **xl2tpd LNS** (SCCRQ→SCCRP→SCCCN), confirmed on both sides. Reproducible: `python3 test/l2tp-interop/run.py 03-ze-lac-xl2tpd-lns`. |
| AC-7 LNS-outgoing exemption (A-6) | documented | xl2tpd cannot answer an OCRQ (source-verified: no `case OCRQ`, logs "Unimplemented message 7"); accel-ppp (`mode lac`) can but only via undocumented source-only CLI. Wire proof = `lns-outgoing-call.ci` (Python LAC) + exemption per `ai/rules/interop-and-goal-validation.md`. |
| AC-9 regression | verified | Only `session-stopccn-cascade` fails in the l2tp suite; PROVEN pre-existing (fails 3/3 at parent `fe6aa242f` via `git archive` build). Logged in `plan/known-failures.md`. No new regressions. |
| Env-blocked (recorded) | runbooks | A-4 LAC kernel channel bridge (`PPPIOCBRIDGECHAN`) + full ze-as-LAC PPP-up + CAP_NET_ADMIN suites → `make ze-qemu-l2tp-ppp-test` (QEMU/CAP_NET_ADMIN). Documented in the scenario README. |

### Prior session progress (continuation)

Committed this session (on top of prior b68e7e9c9 / f79c6bbb2 / 760a60bc6):

| Phase | Commit | ACs | State |
|-------|--------|-----|-------|
| Config: remote dial-target list + PPPoE relay binding | `7542792d2` | AC-6 | DONE (unit + reload tests, parse .ci) |
| outgoing-call RPC + async outcome surfacing | `222aae426` | AC-4 (code), AC-8 (surfacing) | DONE (unit incl. tie-breaker-loss edge case, grammar check); functional .ci PENDING |
| PPPoE->L2TP relay via neutral callsink + LAC bridge | `794315507` | AC-3 | DONE control plane (unit tests, R-1 verified); A-4 kernel bridge AUTHORED+compiles, execution ENV-BLOCKED (no CAP_NET_ADMIN) |
| Initiator FSM transitions recorded for observability | `c44fe82d5` | AC-5 | DONE (snapshot/metrics/fsm-history) |

Remaining (spec stays open):
- **AC-4 functional .ci**: `test/l2tp/tunnel-initiate-sccrq.ci` (Python LNS) and `test/l2tp/lns-outgoing-call.ci` (Python LAC answering OCRQ). The RPC + reactor path is unit-covered (reactor sync tests drive dial->SCCRQ->SCCRP->OCRQ over the loopback client; FSM tests cover OCRP->OCCN). The .ci needs a concurrent fixed-port Python peer + management-API dispatch (ze_api) issuing `request l2tp outgoing-call`.
- **AC-7 interop (MANDATORY, BLOCKING)**: Docker scenario `test/l2tp-interop/scenarios/03-ze-lac-xl2tpd-lns` (ze-as-LAC vs xl2tpd-as-LNS, PPP up end-to-end); LNS-outgoing vs a real peer or documented A-6 exemption.
- **A-4 kernel bridge execution**: `make ze-qemu-l2tp-ppp-test` on a CAP_NET_ADMIN host (`bridge_integration_linux_test.go`, `//go:build integration && linux`, authored+compiles).
- **Closure**: /ze-review gate, learned summary, two-commit closure -- only after AC-4 .ci + AC-7 land.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/l2tp/session_fsm.go`, `tunnel_fsm.go`, `tunnel.go`, `reactor.go`, `reactor_kernel.go`
4. `internal/component/l2tp/pppoe/` (server.go, doc.go boundary) - the LAC call trigger substrate
5. `docs/research/l2tpv2-implementation-guide.md` (S9 tunnel FSM, S10 session FSMs), `rfc/short/rfc2661.md`
6. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

> **Decision resolved (2026-07-09, user):** LAC/LNS call-initiation IS on the roadmap - **both flows** (LAC-side incoming call and LNS-side outgoing call), including the SCCRQ-initiation path. The former "BLOCKED / decision needed" banner is lifted.

## Task

LAC-side incoming-call and LNS-side outgoing-call flows. Both are genuinely unimplemented on the initiation side (FSM stubs) and both need the LAC-initiated tunnel path (sending SCCRQ), which does not exist yet.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence re-verified at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **LAC-side incoming call (L159)** - detect call, send ICRQ, receive ICRP, send ICCN. `handleICRP` (`session_fsm.go:154`) is a stub; no SCCRQ send.
- **LNS-side outgoing call (L160)** - send OCRQ, receive OCRP, receive OCCN. `handleOCRP` (`session_fsm.go:290`) is a stub.

### Design-time corrections (2026-07-09, verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| "Both genuinely unimplemented (FSM stubs)" | Half-true: the ANSWERING sides are implemented - `handleOCRQ` (:227, ze answers an LNS-sent OCRQ) and `handleOCCN` (:303) work with tests; what's missing is every INITIATION send (SCCRQ/SCCCN/ICRQ/ICCN/OCRQ have no encoders) plus three receive stubs: SCCRP (`tunnel_fsm.go:106-108`), ICRP (:154, verified firsthand), OCRP (:290, verified firsthand) |
| (implicit) call trigger exists | No LAC call trigger exists: the PPPoE server (`pppoe/server.go:181-196`) always terminates PPP locally; there is no PPPoE→L2TP relay, and pppoe is architecturally forbidden from importing l2tp (`pppoe/doc.go:12-14`) |
| (implicit) config can express a dial target | No role/peer/remote concept exists in `ze-l2tp-conf.yang` - a LAC/outgoing design needs a new config surface (remote endpoint, secret, dial policy) |
| (implicit) data plane needs rework | It does not: genl tunnel/session create take local/remote IDs symmetrically and `lnsMode` is already a parameter (`kernel_linux.go:71`, `pppox_linux.go:129`); the kernel plumbing is initiator-agnostic |

## Required Reading

### Source files / docs

- [ ] `internal/component/l2tp/tunnel.go` (:20-26 states), `tunnel_fsm.go`
  → Constraint: `L2TPTunnelWaitCtlReply` (:22, "LAC: sent SCCRQ") exists but is never entered; `tunnel.go:107` and `TunnelDefaults` (`tunnel_fsm.go:21` - HostName/framing/bearer/window/secret) already anticipate SCCRQ initiation
  → Constraint: SCCRP receive is a stub at `tunnel_fsm.go:106-108`; the initiator path is Idle → (send SCCRQ) → WaitCtlReply → (recv SCCRP, verify challenge) → (send SCCCN) → Established (RFC 2661 Section 6)
- [ ] `internal/component/l2tp/session_fsm.go`
  → Constraint: LAC incoming-call session path: (send ICRQ) → WaitReply → (recv ICRP, fill stub :154) → (send ICCN) → Established with `lnsMode=false`; LNS outgoing: (send OCRQ) → WaitReply → (recv OCRP, fill stub :290) → WaitConnect → (recv OCCN, handler :303 exists) → Established
  → Constraint: session states WaitTunnel/WaitReply (`session.go:21-22`) are declared and never entered - this spec puts them into service
- [ ] `internal/component/l2tp/avp.go` (:20-81 constants, :199-286 writers), `avp_compound.go`, `hidden.go`, `auth.go`
  → Constraint: all message-type + AVP constants and generic writers exist; parse-side complete for every message; NEW encoders needed: SCCRQ, SCCCN, ICRQ, ICCN, OCRQ (write-side builders in the style of `writeSCCRPBody` `tunnel_fsm.go:451` / `writeICRPBody` `session_fsm.go:1092`)
- [ ] `internal/component/l2tp/reactor.go` (:303 handle, :642 locateTunnelLocked, :704 tunnel create), `reliable.go`
  → Constraint: single reactor goroutine owns the tunnel map; an initiated tunnel must be created and driven from the same goroutine (new "dial" event into the reactor, not a second owner)
  → Constraint: the reliable engine (Ns/Nr, retransmit, ZLB) is transport-agnostic - initiated tunnels reuse it unchanged
- [ ] `internal/component/l2tp/listener.go`, `reactor_kernel.go` (:18 collectKernelEventsLocked, :38 SocketFD use), `genl_linux.go` (:103, :130, :208), `kernel_linux.go` (:64, :71), `pppox_linux.go` (:96, :129)
  → Decision: initiated tunnels REUSE the listener UDP socket (source port 1701) - one socket feeds both directions and the kernel data plane already dups+connects per tunnel (`connectedUDPSocket` :130); alternative (ephemeral client socket) recorded in Key Design Decisions
- [ ] `internal/component/l2tp/pppoe/server.go` (:181-196 local termination), `pppoe/doc.go` (:12-14 import boundary), `internal/component/l2tp/subscriber/`
  → Decision: the LAC call trigger is a PADS-completed PPPoE session relayed into L2TP instead of local `ppp.StartSession`; the relay crosses the pppoe↛l2tp boundary via a neutral registration interface (l2tp registers a call-sink; pppoe consults it when the service config says relay) - registration over hardcoding
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `ai/rules/config-surface.md`, `ai/rules/config-naming.md`, `ai/patterns/config-option.md`
  → Constraint: new YANG surface required: a `remote` (dial-target) list [name, address, port default 1701, shared-secret, outgoing-calls permission] + per-PPPoE-service relay binding; every leaf max-native-validated
- [ ] `rfc/short/rfc2661.md` + `docs/research/l2tpv2-implementation-guide.md` (S9/S10)
  → Constraint: RFC 2661 Sections 5.1 (SCCRQ), 6.1-6.4 (control connection), 10.1-10.2 (call establishment states); tie-breaker AVP handling for simultaneous SCCRQ (reactor tie-breaker exists for answering; initiator side must participate)
- [ ] `ai/rules/interop-and-goal-validation.md`, `test/l2tp-interop/` (Dockerfile.lac, scenarios), `test/l2tp/session-incoming-lns.ci` (inline Python peer pattern)
  → Constraint: interop is BLOCKING for wire-protocol behavior: ze-as-LAC must establish against a non-ze LNS (xl2tpd); the inline-Python-peer .ci pattern is invertible (Python LNS answering ze's SCCRQ) for functional tests

**Key insights:**
- The wire codecs, reliable transport, kernel data plane, and answering-side FSMs are done; this spec adds the initiator half: 5 encoders, 3 receive-stub implementations, 2 never-entered state paths, a dial API on the reactor, a config surface, and the PPPoE→L2TP relay trigger.
- The PPPoE relay is the architecturally sensitive piece (import boundary); the L2TP FSM work itself is well-trodden (mirror of existing answering handlers + existing test patterns).
- Data-plane bridging for LAC (PPPoE frames ↔ L2TP session) is the main kernel-level unknown (A-4).

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `internal/component/l2tp/session_fsm.go` - handleICRP :154 and handleOCRP :290 log-only stubs (verified firsthand); answering handlers implemented
- [ ] `internal/component/l2tp/tunnel.go` + `tunnel_fsm.go` - WaitCtlReply never entered (verified firsthand :22); SCCRP receive stub :106-108; no SCCRQ builder anywhere
- [ ] `internal/component/l2tp/reactor.go`, `reactor_kernel.go`, `genl_linux.go`, `pppox_linux.go` - single-goroutine reactor; initiator-agnostic kernel plumbing
- [ ] `internal/component/l2tp/pppoe/server.go` - PADS → local PPP termination, no relay
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - no dial-target/role surface

**Behavior to preserve:**
- All LNS-answering behavior (SCCRQ/SCCCN receive, ICRQ→ICRP, ICCN, teardown, HELLO, reliable transport semantics, tie-breaker).
- PPPoE local termination remains the default for services not configured to relay.
- Existing `.ci`/interop/scale suites and their assertions.
- Kernel setup semantics (`lnsMode` correctness per direction).

**Behavior to change:**
- ze can initiate tunnels (SCCRQ) and calls (ICRQ as LAC, OCRQ as LNS); the three receive stubs become real transitions; WaitCtlReply/WaitTunnel/WaitReply become live states.
- New config surface for dial targets + PPPoE relay binding; new RPC to trigger an outgoing call.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LAC: PPPoE PADS completion (relay-configured service) → dial event into l2tp reactor
- LNS outgoing: `request l2tp outgoing-call` RPC (cmd registry) referencing a configured remote + called number

### Transformation Path
1. Dial event → reactor creates initiator tunnel (Idle → send SCCRQ via listener socket → WaitCtlReply)
2. SCCRP received → challenge verified → SCCCN sent → tunnel Established
3. LAC: session ICRQ → WaitReply → ICRP → ICCN → Established (`lnsMode=false`); LNS: OCRQ → WaitReply → OCRP → WaitConnect → OCCN → Established
4. Kernel events: genl tunnel/session create (existing, initiator-agnostic); LAC bridges the PPPoE channel to the L2TP channel (A-4); LNS-outgoing runs PPP as today
5. Observability: existing snapshots/metrics/FSM-history extended with initiator states

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| call event → L2TP FSM | reactor dial event (new); PPPoE relay via neutral call-sink registration (new) | [ ] |
| ze → peer | L2TP control messages over the wire (existing reliable engine; 5 new encoders) | [ ] |
| control → kernel | existing genl/pppox path, `lnsMode` per direction | [ ] |
| pppoe ↛ l2tp import boundary | registration interface in a neutral package (doc.go:12-14 preserved) | [ ] |

### Integration Points
- `internal/component/l2tp/reactor.go` - dial-event entry
- `internal/component/l2tp/tunnel_fsm.go` / `session_fsm.go` - initiator transitions
- `internal/component/l2tp/pppoe/` - relay consultation point (via neutral interface)
- `internal/component/l2tp/cmd/` - outgoing-call RPC
- `internal/component/l2tp/yang/` - remote list + relay binding

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated - pppoe/l2tp boundary intact)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Corrected evidence holds at implement time | re-verified 2026-07-09 (firsthand: :154, :290, tunnel.go:22 + stub bodies) | Re-scope item | grep/LSP at implement-audit | confirmed |
| A-2 | Both flows in scope | user decision 2026-07-09 ("Yes, both flows") | N/A - recorded | this spec | confirmed |
| A-3 | Reusing the listener socket (src 1701) for initiated tunnels is valid against real LNS peers | kernel data plane dups+connects per tunnel (genl_linux.go:130); RFC 2661 does not mandate ephemeral source | Ephemeral client socket per initiated tunnel (reactor-owned) | xl2tpd interop scenario | **confirmed** — scenario 03: ze dials from its listener socket and xl2tpd (`Connection established ... Remote: 1721`) accepts the source; no ephemeral socket needed |
| A-4 | LAC data-plane = kernel channel bridge between the PPPoE channel and the pppol2tp channel (PPPIOCBRIDGECHAN), no PPP termination at the LAC | kernel plumbing is initiator-agnostic; bridge ioctl is the standard LAC construct in modern kernels (unverified against ze's target kernels) | Userspace frame relay between the two sockets (slower but functional); record perf implication | QEMU integration test on target kernel | **env-blocked** — bridge authored+compiles (`bridge_linux.go`, `//go:build integration && linux`); execution needs CAP_NET_ADMIN + PPPoL2TP kernel modules: `make ze-qemu-l2tp-ppp-test`. Runbook in scenario-03 README |
| A-5 | xl2tpd (existing interop image) can serve as the LNS answering ze's SCCRQ/ICRQ | test/l2tp-interop Dockerfile.lac already runs xl2tpd as LAC; xl2tpd supports LNS mode | Use FRR/accel-ppp or keep Python-LNS functional test as the wire proof for the LAC flow | new interop scenario bring-up | **confirmed** — scenario 03: xl2tpd `[lns default]` answers ze's SCCRQ (SCCRP→SCCCN→established). (ICRQ path is the PPPoE-relay + A-4 data plane, env-blocked.) |
| A-6 | LNS-outgoing (OCRQ) has a peer implementation available for interop | xl2tpd/accel-ppp outgoing-call support unknown | Python LAC simulator answers OCRQ (functional tier); interop exemption documented per interop-and-goal-validation.md with justification | scenario research at implement time | **resolved (exemption)** — source-verified: xl2tpd has NO OCRQ answerer (logs "Unimplemented message 7", tears down); accel-ppp `mode lac` answers but only via undocumented source-only CLI. Wire proof = `lns-outgoing-call.ci` (Python LAC) + documented exemption |
| A-7 | Simultaneous-open (both ends send SCCRQ) is resolvable with the existing tie-breaker logic once ze initiates | reactor tie-breaker exists for the answering path | Implement initiator-side tie-breaker per RFC 2661 5.1 | unit test with crossed SCCRQs | **confirmed** — `TestReactor_InitiatorTieBreaker` + `TestReactor_PlaceOutgoingCallSync_TieBreakerLoss` (reactor_dial_test.go); `tunnel-initiate-sccrq.ci` asserts the Tie Breaker AVP is on the wire |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PPPoE relay design leaks l2tp types into pppoe (boundary break) | pppoe imports l2tp (lint/tier check) | Neutral call-sink interface in a shared package (subscriber/ or new small package); `make ze-tier-check` gates |
| R-2 | Reactor dial path introduces a second tunnel-map writer (race) | race detector hits in reactor tests | Dial is an event consumed by the single reactor goroutine, same as packets |
| R-3 | Challenge/hidden-AVP handling differs subtly on the initiator side | auth failures against xl2tpd only | Reuse auth.go primitives; interop scenario with shared-secret exercises it |
| R-4 | LAC session brings RADIUS/pool/shaper plugin assumptions that only held for LNS mode | plugin hooks fire with lnsMode=false unexpectedly | Audit observer/subscriber-bridge/RADIUS paths for lnsMode assumptions during phase 3; LAC sessions skip local IP assignment (no IPCP at LAC) |
| R-5 | Scope creep into a full BNG steering policy language | relay config grows beyond service→tunnel binding | Minimal binding: per-PPPoE-service `relay remote <name>`; a steering-policy engine is a separate concern this feature does not cover |
| R-6 | Outgoing-call RPC without an operator use case rots | RPC unused after merge | Tie to the interop/functional test as its consumer; document the operator workflow in guide |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Dial event for configured remote | → | SCCRQ sent, tunnel enters WaitCtlReply, SCCRP verified, SCCCN sent, Established | `TestTunnelInitiatorHandshake` (unit) + `.ci` `test/l2tp/tunnel-initiate-sccrq.ci` (Python LNS) |
| PADS-completed PPPoE session on a relay-configured service | → | call-sink → ICRQ → ICRP (stub :154 filled) → ICCN → Established, lnsMode=false | `TestLACIncomingCallFlow` + `.ci` `test/l2tp/lac-incoming-call.ci` |
| `request l2tp outgoing-call remote <r> called <n>` | → | OCRQ → OCRP (stub :290 filled) → WaitConnect → OCCN → Established | `TestLNSOutgoingCallFlow` + `.ci` `test/l2tp/lns-outgoing-call.ci` (Python LAC answers) |
| Crossed SCCRQs (simultaneous open) | → | tie-breaker resolves one tunnel | `TestInitiatorTieBreaker` |
| ze-as-LAC against real LNS | → | full stack over Docker | interop scenario `test/l2tp-interop/03-ze-lac-xl2tpd-lns` |
| LAC data plane after ICCN | → | PPPoE↔L2TP channel bridge carries PPP frames | QEMU integration `TestLACChannelBridge` (A-4) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Dial event toward a configured remote | ze sends a well-formed SCCRQ (HostName, framing/bearer caps, window, challenge when secret set) from the listener socket; enters WaitCtlReply; retransmit/ZLB semantics via the existing reliable engine |
| AC-2 | SCCRP received (valid / invalid challenge response) | Valid: SCCCN sent, tunnel Established, HELLO schedule active; invalid: StopCCN with correct result code, tunnel Closed |
| AC-3 | PPPoE service configured with `relay remote <name>`; client completes PADS | No local PPP termination; ICRQ sent with serial + bearer/framing AVPs; ICRP transitions WaitReply→(ICCN sent)→Established; kernel session created with lnsMode=false; PPP frames bridged PPPoE↔L2TP (A-4 mechanism); teardown propagates both ways (CDN ↔ PADT) |
| AC-4 | `request l2tp outgoing-call` RPC with remote + called number | OCRQ sent on an Established tunnel (initiating one if needed); OCRP handled (WaitConnect); OCCN completes the call; RPC returns session identifiers; failure paths surface result codes |
| AC-5 | Tunnels/sessions initiated by ze | Visible in existing snapshots/CLI (`show l2tp tunnels/sessions`) with correct state names (wait-ctl-reply, wait-reply); FSM history + metrics record the new transitions |
| AC-6 | Config surface | `remote` list (name, address, port [1..65535, default 1701], shared-secret, outgoing-calls flag) + per-PPPoE-service relay binding; max native YANG validation; config-naming.md compliant; reload-safe (subsystem_reload.go path) |
| AC-7 | Interop | ze-as-LAC establishes tunnel + incoming call against xl2tpd-as-LNS (Docker scenario) with PPP session up end-to-end; LNS-outgoing proven against a real peer if A-6 finds one, else Python-peer functional + documented exemption |
| AC-8 | Simultaneous open | Crossed SCCRQs resolve to a single tunnel per RFC 2661 5.1 tie-breaker |
| AC-9 | Regression | All existing l2tp unit/functional/interop/scale suites pass unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | ISP wholesales broadband: subscriber PPPoE lands on ze (LAC) and is tunneled to the retail LNS | PADS → relay → SCCRQ/ICRQ/ICCN → bridged PPP | interop `03-ze-lac-xl2tpd-lns` |
| 2 | Operator triggers a dial-out session from the LNS | RPC → OCRQ/OCRP/OCCN → PPP up | `lns-outgoing-call.ci` |
| 3 | NOC inspects an initiated tunnel | CLI `show l2tp tunnels` | AC-5 assertions in `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteSCCRQ/SCCCN/ICRQ/ICCN/OCRQ` (encode↔parse round-trip with existing parsers) | `internal/component/l2tp/avp_encode_test.go` (new) | encoders | |
| `TestTunnelInitiatorHandshake`, `TestInitiatorChallengeReject`, `TestInitiatorTieBreaker` | `tunnel_fsm_test.go` | AC-1, AC-2, AC-8 | |
| `TestLACIncomingCallFlow`, `TestICRPWrongState` | `session_fsm_test.go` | AC-3 | |
| `TestLNSOutgoingCallFlow`, `TestOCRPWrongState` | `session_fsm_test.go` | AC-4 | |
| `TestReactorDialEvent` (single-goroutine ownership) | `reactor_test.go` | R-2 | |
| `TestCallSinkRegistration` (pppoe boundary) | pppoe + neutral pkg tests | R-1 | |
| `TestRemoteConfigParse`, YANG validation tests | config tests | AC-6 | |
| `TestLACChannelBridge` (integration && linux, QEMU) | `kernel_integration_linux_test.go` | A-4/AC-3 data plane | |
| Fuzz: new encoders through existing header/AVP fuzzers | existing fuzz files | codec robustness | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| remote port | 1-65535 (default 1701) | 65535 | 0 | N/A uint16 |
| call serial number | 0-4294967295 | max | N/A | wraps per RFC |
| window size (initiator SCCRQ) | 1-65535 (existing default) | 65535 | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tunnel-initiate-sccrq.ci` (Python LNS answers) | test/l2tp | initiator handshake on the wire | **PASS** |
| `lac-incoming-call.ci` | test/l2tp | full LAC call over inline peers | env-blocked — the LAC incoming call is triggered only by the PPPoE relay (no RPC) and its data plane is the A-4 kernel bridge (CAP_NET_ADMIN); control plane covered by `session_initiator_*_test.go` + `relay_sink` unit tests. Runbook: `make ze-qemu-l2tp-ppp-test` |
| `lns-outgoing-call.ci` (Python LAC answers OCRQ) | test/l2tp | outgoing-call flow | **PASS** |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `03-ze-lac-xl2tpd-lns` | test/l2tp-interop/scenarios/ | xl2tpd (LNS mode) | **PASS** — ze-initiated L2TP control connection interoperates with real xl2tpd LNS (SCCRQ→SCCRP→SCCCN, established both sides). LAC incoming-call PPP data plane (A-4) is env-blocked (see README runbook) |
| LNS-outgoing vs real peer (A-6 permitting) | test/l2tp-interop/scenarios/ | xl2tpd or accel-ppp | **exemption** — xl2tpd cannot answer OCRQ (source-verified); accel-ppp `mode lac` can but via undocumented source-only CLI. Wire proof = `lns-outgoing-call.ci` + documented exemption (A-6) |

## Files to Modify

- `internal/component/l2tp/tunnel_fsm.go` - SCCRQ/SCCCN builders, SCCRP handler (fill :106-108), initiator transitions, tie-breaker
- `internal/component/l2tp/session_fsm.go` - ICRQ/ICCN/OCRQ builders, fill handleICRP (:154) + handleOCRP (:290), WaitReply/WaitTunnel paths
- `internal/component/l2tp/reactor.go` + `reactor_kernel.go` - dial event, initiated-tunnel lifecycle, LAC kernel-event variant
- `internal/component/l2tp/kernel_linux.go` / `pppox_linux.go` - LAC channel-bridge setup (A-4)
- `internal/component/l2tp/pppoe/server.go` - relay consultation (via neutral interface)
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` + `config.go` + `subsystem_reload.go` - remote list + relay binding
- `internal/component/l2tp/cmd/l2tp.go` (+ cmd YANG) - outgoing-call RPC + snapshots for initiated state names
- `internal/component/l2tp/snapshot.go`, `metrics.go`, `fsm_history.go` - initiator-state observability
- `docs/guide/l2tp.md`, `docs/architecture/wire/l2tp.md`, `docs/features.md` - per Documentation Update Checklist

## Files to Create

- neutral call-sink package (e.g. `internal/component/l2tp/callsink/` or extension of `subscriber/`) - R-1 boundary preservation (final location decided at implement audit against `ai/rules/module-tiers.md`)
- `internal/component/l2tp/avp_encode_test.go` (or per-message encoder files + tests)
- `.ci` + interop scenario files listed above

## Implementation Steps

1. **Phase: Wiring (encoders + initiator handshake)** - encode/parse round-trip tests fail → SCCRQ/SCCCN builders + SCCRP handler → `tunnel-initiate-sccrq.ci` (AC-1, AC-2, AC-8).
2. **Phase: config + dial API** - remote list YANG, reactor dial event, RPC skeleton (AC-6).
3. **Phase: LAC incoming call** - ICRQ/ICCN builders, ICRP handler, PPPoE relay interface, kernel bridge (AC-3; A-4 QEMU proof).
4. **Phase: LNS outgoing call** - OCRQ builder, OCRP handler, RPC completion (AC-4).
5. **Phase: observability** - snapshots/metrics/history (AC-5).
6. **Phase: interop** - xl2tpd-LNS scenario (AC-7); A-6 verdict for outgoing.
7. **Full verification** - `make ze-verify` + l2tp suites + QEMU integration.
8. **Complete spec** - audit tables, `plan/learned/NNN-followup-l2tp-call.md`, two-commit closure.

## Goal Validation

Per `ai/rules/interop-and-goal-validation.md` — goal → concrete evidence:

| Goal (from Task) | Evidence |
|------------------|----------|
| ze can INITIATE a tunnel (send SCCRQ) | `tunnel-initiate-sccrq.ci` (PASS: SCCRQ well-formed with HostName/framing/bearer/window + Challenge + Tie Breaker AVP); `TestReactor_DialLoopbackHandshake`; **real-peer interop** scenario 03 (xl2tpd `control_finish: message type is Start-Control-Connection-Request(1) ... sending SCCRP`) |
| ze verifies SCCRP + sends SCCCN + establishes | `tunnel-initiate-sccrq.ci` (ze log `tunnel now established (initiator)`); interop scenario 03 (xl2tpd `Connection established to 127.0.0.1, <port>. Local: 10880, Remote: 1721 ... LNS session is 'default'`) |
| LNS-side outgoing call (OCRQ→OCRP→OCCN) | `lns-outgoing-call.ci` (PASS: OCRQ msgtype=7 called=5551234; ze logs `OCRP received; session wait-connect`, `session established (outgoing)`; RPC returns local/remote SID). Real-peer answerer exempted (A-6) |
| `request l2tp outgoing-call` RPC returns identifiers / surfaces failures | `lns-outgoing-call.ci` (RPC status=done, local-sid+remote-sid); `outgoing_call_test.go`; interop 03 shows the failure surface when the peer (xl2tpd) can't answer the OCRQ |
| LAC-side incoming call (ICRQ→ICRP→ICCN) | `session_initiator_fsm_test.go` (`TestPlaceIncomingCall_Handshake`), `TestReactor_PlaceIncomingCall_AutoICRQ`, `relay_sink` unit tests. Functional/interop + PPP data plane = A-4 env-blocked (runbook) |
| Simultaneous open resolved (tie-breaker) | `TestReactor_InitiatorTieBreaker`, `TestReactor_PlaceOutgoingCallSync_TieBreakerLoss`; `tunnel-initiate-sccrq.ci` asserts the Tie Breaker AVP on the wire |
| Interoperates with an independent RFC 2661 implementation | scenario 03: full SCCRQ/SCCRP/SCCCN control connection with real **xl2tpd 1.3.18** LNS, confirmed both sides |
| No regression to existing L2TP behavior | l2tp suite unchanged except `session-stopccn-cascade` (PROVEN pre-existing at parent `fe6aa242f`, `plan/known-failures.md`) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

Pre-checks: `make ze-validate` clean; `audit-test-relaxation.py` clean (no tests
deleted/weakened — this session only ADDS tests). This session's diff is
test/interop/doc only (no Go production code); the initiator production code was
committed in the seven prior phases, so wiring/functional-test/RFC-compliance
apply to those (already covered). New symbols introduced this session: none.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Doc drift: the `remote` dial-target config and `request l2tp outgoing-call` RPC (committed in prior phases) were undocumented; RFC 2661 rfc-status row omitted the initiator/outgoing-call coverage; interop lab doc lacked scenario 03. | docs/guide/l2tp.md, docs/features/rfc-status.md, docs/labs/l2tp-interop.md | fixed (see Fixes) |
| 2 | ISSUE | Discovery indexes stale: `callsink` package (committed 794315507) absent from `ai/PACKAGE-MAP.md`/`ai/DOCS-TO-CODE.md`; doc-test failed. | ai/PACKAGE-MAP.md, ai/DOCS-TO-CODE.md | fixed (see Fixes) |
| 3 | NOTE | Scenario-03 runner uses fixed ports (17010-17012) and leaves the ze log file handle open until process exit. | test/l2tp-interop/scenarios/03-ze-lac-xl2tpd-lns/run.py | acknowledged — acceptable for a single-scenario harness |
| 4 | NOTE | Two new `.ci` hardcode REST ports (18092/18093), matching the existing `test/plugin/rest-*.ci` pattern; small parallel-collision risk. | test/l2tp/*.ci | acknowledged — established pattern; suite runs green |

### Fixes applied
- Added an "Initiator: dial targets and outgoing calls" section to `docs/guide/l2tp.md` (remote list, relay binding, `request l2tp outgoing-call` grammar) with source anchors to `yang/ze-l2tp-conf.yang` + `cmd/outgoing_call.go`.
- Updated the RFC 2661 row in `docs/features/rfc-status.md` (initiator dial + OCRQ + config/RPC) with source anchors to `tunnel_initiator.go`/`session_initiator.go`.
- Added scenario 03 to `docs/labs/l2tp-interop.md` (topology inversion, self-contained runner, OCRQ/A-6 note, A-4 env-block).
- Ran `make ze-discovery-index`: `ai/PACKAGE-MAP.md`/`ai/DOCS-TO-CODE.md` now carry the `callsink` + initiator/outgoing-call/relay/bridge entries. `make ze-doc-test` passes.

### Run 2 (re-run after fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (none) | Clean — remaining changes are docs + generated indexes (no new logic); `make ze-doc-test` exit 0; both `.ci` stable 3/3; scenario 03 PASS. | — | — |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

Evidence (this session, AC-4 functional + AC-7 interop):
- `bin/ze-test l2tp --pattern tunnel-initiate` → `PASS 17 tunnel-initiate-sccrq` (3/3 stable). Asserts: SCCRQ well-formed (Challenge + Tie Breaker AVP), `tunnel now established (initiator)`, RPC status=done.
- `bin/ze-test l2tp --pattern lns-outgoing` → `PASS 7 lns-outgoing-call` (3/3 stable). Asserts: OCRQ msgtype=7 called=5551234, `session established (outgoing)`, RPC returns local-sid+remote-sid.
- `python3 test/l2tp-interop/run.py 03-ze-lac-xl2tpd-lns` → `PASS 1 scenario(s)`. Real xl2tpd 1.3.18 LNS: `Connection established to 127.0.0.1, <port>. Local: 10880, Remote: 1721 ... LNS session is 'default'`; then `message type 7 (Outgoing-Call-Request)` + `closing down tunnel` (A-6).
- Full l2tp suite (17 files): 16 PASS + `session-stopccn-cascade` (PROVEN pre-existing at parent `fe6aa242f`). The seven prior phases' unit tests (initiator FSMs, encoders, reactor dial, tie-breaker, outcome surfacing) remain green under `-race`.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Initiated tunnels reuse the listener socket (src 1701) | ephemeral client socket per tunnel | One socket keeps reactor/kernel handoff unchanged (SocketFD path); RFC-compatible; A-3 interop-validates; fallback recorded |
| PPPoE→L2TP relay via neutral call-sink registration | pppoe imports l2tp directly / l2tp polls pppoe | Preserves the deliberate import boundary (pppoe/doc.go:12-14); registration is the repo-wide pattern |
| LAC bridges kernel channels (no PPP termination at LAC) | terminate + re-originate PPP at the LAC | RFC 2661 LAC semantics: PPP endpoints are subscriber and LNS; termination would break end-to-end LCP/auth (A-4 governs the mechanism) |
| Outgoing-call trigger = RPC referencing configured remotes | config-declared static dial-out | Calls are operational events; config declares capability (remote list), RPC exercises it |
| Dial as a reactor event | separate dialer goroutine owning tunnels | Single-owner invariant of the tunnel map (R-2) |

## Known Limitations

- L2TPv3 (RFC 3931) is not implemented; L2TPv2 only.
- LAC relay binds PPPoE services to one remote each (no failover/load-balancing policy - R-5); multi-LNS steering policy is a separate concern this feature does not cover.
- No RADIUS-driven LNS selection at the LAC in this pass (RADIUS-supplied tunnel attributes are a separate concern; record if requested).
- WEN/SLI initiation not added (receive already handled); only call-establishment messages gain encoders.

## Notes
- Designed 2026-07-09 from skeleton; user decisions 2026-07-09: both flows approved, batch conversion to ready authorized.
- Adjacent open spec `plan/spec-finish-l2tp.md` (tests/docs residuals) is non-overlapping; coordinate at implement time.
