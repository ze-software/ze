# Spec: l2tp-4 -- L2TP Session State Machine

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-l2tp-3-tunnel |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
3. `docs/research/l2tpv2-implementation-guide.md` sections 7.6-7.14, 10, 18-19
4. `internal/component/l2tp/tunnel_fsm.go` -- dispatch point for session messages
5. `internal/component/l2tp/tunnel.go` -- tunnel struct (sessions map added here)
6. `internal/component/l2tp/session.go` -- new file
7. `internal/component/l2tp/session_fsm.go` -- new file

## Task

Implement all four session state machines from RFC 2661 Section 10:

1. **Incoming call, LNS side:** ICRQ/ICRP/ICCN (ze as LNS receives call from LAC)
2. **Outgoing call, LAC side:** receive OCRQ, send OCRP, send OCCN (ze as LAC receives dial-out request)

Deferred to a separate spec within the l2tp set (must complete before set closes):
3. **Incoming call, LAC side:** detect call, send ICRQ, receive ICRP, send ICCN (requires LAC-initiated tunnel)
4. **Outgoing call, LNS side:** send OCRQ, receive OCRP, receive OCCN (requires LAC-initiated tunnel)

Plus: CDN teardown for all session types, WEN (WAN-Error-Notify) and SLI
(Set-Link-Info) handling on established sessions, proxy LCP/auth AVP capture
from ICCN, session management within tunnels (create, lookup, destroy, limits),
and StopCCN cascade (all sessions cleared when tunnel tears down).

**LAC-initiated sessions** (incoming call LAC side, outgoing call LNS side) require
LAC-initiated tunnel creation (sending SCCRQ), which phase 3 does not implement.
These are deferred to a separate spec within the l2tp set (before the set closes),
not dropped. Phase 4 covers all session FSMs reachable from the current reactive
(LNS-side) tunnel infrastructure.

Reference: `docs/research/l2tpv2-implementation-guide.md` sections 10 (session
state machines), 7.6-7.14 (message AVPs), 18 (proxy LCP/auth), 19 (WEN/SLI).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
  -> Constraint: sessions owned by tunnel, mutated only by reactor goroutine
- [ ] `docs/research/l2tpv2-implementation-guide.md` -- protocol spec
  -> Constraint: Section 10 defines 4 session FSMs; Section 7.6-7.14 defines required AVPs per message
- [ ] `docs/research/l2tpv2-ze-integration.md` -- ze integration design
  -> Constraint: kernel data plane for sessions created in phase 5; phase 4 is pure control plane FSM

### Source Files
- [ ] `internal/component/l2tp/tunnel.go` -- L2TPTunnel struct, states, newTunnel
  -> Constraint: tunnel has no session map yet; sessions field must be added
- [ ] `internal/component/l2tp/tunnel_fsm.go` -- handleMessage dispatches SCCRQ/SCCCN/StopCCN/HELLO
  -> Constraint: session messages (ICRQ, ICRP, ICCN, OCRQ, OCRP, OCCN, CDN, WEN, SLI) currently logged and dropped at line 114
  -> Decision: extend handleMessage to dispatch session-scoped messages to session_fsm.go handlers
- [ ] `internal/component/l2tp/reactor.go` -- single reactor goroutine, tunnelsMu
  -> Constraint: all session state mutations inside reactor goroutine; no new goroutines
  -> Decision: ICRQ pre-validation NOT needed at reactor level (unlike SCCRQ); ICRQ arrives on an existing tunnel, so validation happens inside handleMessage
- [ ] `internal/component/l2tp/avp.go` -- AVP constants 0-39 all defined, including session AVPs (14-39)
  -> Constraint: AVP types already exist; no new constants needed
- [ ] `internal/component/l2tp/avp_compound.go` -- ResultCode, Q931Cause, CallErrors, ACCM, ProxyAuthenID readers/writers exist
  -> Constraint: compound AVP helpers already built; session parsers will use them
- [ ] `internal/component/l2tp/config.go` -- Parameters struct, ExtractParameters
  -> Decision: add MaxSessions uint16 field + YANG leaf + env var
- [ ] `internal/component/l2tp/header.go` -- MessageHeader.SessionID already parsed
  -> Constraint: RecvEntry.SessionID already carried through reliable engine

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2661 -- L2TP (`rfc/short/rfc2661.md`)
  -> Constraint: Session ID 0 reserved, never assigned; used in ICRQ/OCRQ before peer assigns
  -> Constraint: CDN Assigned Session ID AVP = sender's own ID (for identification)
  -> Constraint: Unknown M=1 AVP in session message -> CDN (tear session, not tunnel) per S24.12

**Key insights:**
- Header SessionID = recipient's assigned ID (like TunnelID)
- SessionID 0 in header means "no session yet" (ICRQ, OCRQ) or "tunnel-scoped" (HELLO, StopCCN)
- Sessions share the tunnel's ReliableEngine; session messages are sequenced at tunnel level
- ICCN carries proxy LCP and proxy auth AVPs (captured for phase 6 PPP engine)
- WEN is LAC->LNS only; SLI is LNS->LAC only (direction enforced by role, not by protocol)
- CDN Result Codes are distinct from StopCCN Result Codes (RFC S4.4.2 vs S5.4.2)

## Current Behavior (MANDATORY)

**Source files read:**
- `tunnel.go` (139L): L2TPTunnel struct with localTID, remoteTID, peerAddr, state, engine, peer capabilities. No session storage.
- `tunnel_fsm.go` (648L): handleMessage dispatches SCCRQ/SCCRP/SCCCN/StopCCN/HELLO. Line 114: all other message types logged as "unsupported" and dropped.
- `reactor.go` (~600L): Single goroutine. locateTunnelLocked creates tunnels from SCCRQ. handle() dispatches via tunnel.Process(). handleTick() manages HELLO/reaper.
- `avp.go` (~300L): All 40 AVP type constants (0-39) defined. AVPIterator for zero-copy parsing. Write helpers for all value types.
- `avp_compound.go` (~200L): ResultCodeValue, Q931CauseValue, CallErrorsValue, ACCMValue, ProxyAuthenIDValue with Read/Write functions.
- `config.go` (165L): Parameters{Enabled, ListenAddrs, MaxTunnels, HelloInterval, SharedSecret}. No MaxSessions.
- `header.go` (~200L): MessageHeader with SessionID field. Already parsed from wire.
- `reliable.go` (~500L): RecvEntry carries SessionID uint16 from header.

**Behavior to preserve:**
- Tunnel FSM unchanged: SCCRQ/SCCRP/SCCCN/StopCCN/HELLO handlers remain as-is
- Reactor single-goroutine model: no new goroutines, no new channels
- Buffer-first encoding: all session message builders use GetBuf/PutBuf + offset writes
- Pre-validation before lock: SCCRQ path unchanged; ICRQ validated inside handleMessage (tunnel already located)
- Engine per-tunnel: sessions share the tunnel's ReliableEngine for sequencing
- Timer goroutine: no session-level timers in phase 4 (session timeout deferred to phase 5)

**Behavior to change:**
- `tunnel.go`: add `sessions map[uint16]*L2TPSession` and `nextSessionID` to L2TPTunnel
- `tunnel_fsm.go`: extend handleMessage to dispatch ICRQ/ICRP/ICCN/OCRQ/OCRP/OCCN/CDN/WEN/SLI to session handlers
- `tunnel_fsm.go`: handleStopCCN sends CDN for each active session before closing tunnel
- `config.go`: add MaxSessions to Parameters, add YANG leaf, add env var
- New: `session.go` -- L2TPSession struct, state enum, session map helpers
- New: `session_fsm.go` -- all session message handlers, parsers, builders

## Data Flow (MANDATORY)

### Entry Point
- UDP datagram arrives at listener -> reactor.handle() -> locateTunnelLocked finds existing tunnel -> tunnel.Process() -> engine.OnReceive() delivers in-order -> handleMessage dispatches to session handler

### Transformation Path
1. UDP packet -> ParseMessageHeader -> header.SessionID extracted
2. Engine OnReceive -> RecvEntry{SessionID, Payload} delivered in order
3. handleMessage: if session-scoped message type, call dispatchToSession
4. dispatchToSession: for SID=0 (ICRQ/OCRQ) create new session; for SID>0 lookup by local SID
5. Session handler: parse AVP body into info struct -> validate -> produce response -> enqueue through tunnel's engine -> return sendRequest

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Session FSM | AVP iterator over RecvEntry.Payload; zero-copy read | [ ] |
| Session FSM -> Wire | writeBody into pooled buffer -> engine.Enqueue -> sendRequest | [ ] |
| Tunnel -> Session | dispatchToSession called from handleMessage; session struct owned by tunnel | [ ] |

### Integration Points
- `tunnel.sessions` map: session lifecycle managed by tunnel
- `tunnel.engine`: session messages enqueued through tunnel's ReliableEngine (session messages are tunnel-level control messages with session scope)
- `handleStopCCN`: iterates sessions, clears all
- Phase 5 (kernel): session-established is the trigger for kernel tunnel/session creation
- Phase 6 (PPP): proxy LCP/auth AVPs from ICCN are the starting point for PPP negotiation

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| UDP ICRQ packet on established tunnel | -> | handleICRQ + handleICCN (LNS incoming) | `test/l2tp/session-incoming-lns.ci` |
| UDP CDN packet on established session | -> | handleCDN (session teardown) | `test/l2tp/session-cdn-teardown.ci` |
| UDP StopCCN on tunnel with sessions | -> | handleStopCCN cascade | `test/l2tp/session-stopccn-cascade.ci` |
| Config with max-sessions | -> | session limit enforcement | `test/parse/l2tp-max-sessions.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LAC sends ICRQ with valid Assigned Session ID and Call Serial Number on established tunnel | Ze sends ICRP with its own Assigned Session ID; session enters wait-connect state |
| AC-2 | LAC sends ICCN after receiving ICRP | Ze accepts ICCN; session enters established state; proxy LCP/auth AVPs captured if present |
| AC-3 | LAC sends ICCN with invalid/missing required AVPs | Ze sends CDN with Result Code 2 (general error); session destroyed |
| AC-4 | LAC sends ICRQ on non-established tunnel | Ze drops the message (no CDN, no session created) |
| AC-5 | LAC sends ICRQ when max-sessions reached | Ze sends CDN with Result Code 4 (no resources available) |
| AC-6 | LAC sends ICRQ with Assigned Session ID = 0 | Ze sends CDN with Result Code 2 (invalid session ID) |
| AC-7 | Either side sends CDN on established session | Session destroyed, resources cleaned up |
| AC-8 | CDN received on session in any non-idle state | Session destroyed (CDN is valid in any state) |
| AC-9 | StopCCN received on tunnel with active sessions | All sessions torn down (CDN sent for each if possible), then tunnel closes |
| AC-10 | LAC sends WEN on established session | CallErrors counters captured on session; logged |
| AC-11 | LNS sends SLI on established session | ACCM values captured on session; logged |
| AC-12 | OCRQ received on established tunnel (LAC side) | Ze sends OCRP with Assigned Session ID; session enters wait-cs-answer state |
| AC-13 | OCCN received after OCRP (LAC side) | Ze accepts OCCN; session enters established state |
| AC-14 | OCRQ/ICRQ with unknown mandatory AVP (vendor!=0, M=1) | Ze sends CDN for that session (not StopCCN); tunnel unaffected per RFC S24.12 |
| AC-15 | Session ID collision (random allocation hits existing) | Retry allocation; succeed with different ID |
| AC-16 | Header SessionID unknown (non-zero, not in session map) | Message dropped with debug log; no CDN (we don't know the peer's SID) |
| AC-17 | ICCN carries proxy LCP AVPs (types 26, 27, 28) | AVP values captured on session struct for phase 6 PPP engine |
| AC-18 | ICCN carries proxy auth AVPs (types 29-33) | AVP values captured on session struct for phase 6 PPP engine |
| AC-19 | ICCN carries Sequencing Required AVP (type 39) | Flag captured on session; data sequencing noted for kernel setup |
| ~~AC-20~~ | ~~LAC-side incoming call: wait-tunnel~~ | Deferred: requires LAC-initiated tunnel (separate spec within l2tp set) |
| ~~AC-21~~ | ~~LNS-side outgoing call: wait-tunnel~~ | Deferred: requires LAC-initiated tunnel (separate spec within l2tp set) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSession_IncomingLNS_FullHandshake | session_fsm_test.go | AC-1, AC-2: ICRQ->ICRP->ICCN full sequence | [ ] |
| TestSession_IncomingLNS_ICCNMissingAVP | session_fsm_test.go | AC-3: ICCN with missing Tx Connect Speed -> CDN | [ ] |
| TestSession_IncomingLNS_NonEstablishedTunnel | session_fsm_test.go | AC-4: ICRQ on wait-ctl-conn tunnel dropped | [ ] |
| TestSession_MaxSessionsEnforced | session_fsm_test.go | AC-5: max-sessions limit -> CDN RC=4 | [ ] |
| TestSession_ICRQAssignedSIDZero | session_fsm_test.go | AC-6: Assigned Session ID = 0 -> CDN RC=2 | [ ] |
| TestSession_CDN_EstablishedSession | session_fsm_test.go | AC-7: CDN destroys established session | [ ] |
| TestSession_CDN_AnyState | session_fsm_test.go | AC-8: CDN valid in wait-connect and established | [ ] |
| TestSession_StopCCN_CascadeSessions | reactor_test.go | AC-9: StopCCN clears all sessions via CDN | [ ] |
| TestSession_WEN_CallErrors | session_fsm_test.go | AC-10: WEN captured on session | [ ] |
| TestSession_SLI_ACCM | session_fsm_test.go | AC-11: SLI captured on session | [ ] |
| TestSession_OutgoingLAC_OCRQ | session_fsm_test.go | AC-12: OCRQ -> OCRP with Assigned SID | [ ] |
| TestSession_OutgoingLAC_OCCN | session_fsm_test.go | AC-13: OCCN -> established | [ ] |
| TestSession_UnknownMandatoryAVP | session_fsm_test.go | AC-14: unknown M=1 AVP -> CDN (not StopCCN) | [ ] |
| TestSession_IDCollisionRetry | session_fsm_test.go | AC-15: collision retry succeeds | [ ] |
| TestSession_UnknownHeaderSID | session_fsm_test.go | AC-16: unknown SID in header -> drop | [ ] |
| TestSession_ProxyLCP | session_fsm_test.go | AC-17: proxy LCP AVPs captured | [ ] |
| TestSession_ProxyAuth | session_fsm_test.go | AC-18: proxy auth AVPs captured | [ ] |
| TestSession_SequencingRequired | session_fsm_test.go | AC-19: Sequencing Required flag captured | [ ] |
| ~~TestSession_LACWaitTunnel_Stub~~ | ~~session_fsm_test.go~~ | ~~AC-20~~ | Deferred to LAC-initiated tunnel spec |
| ~~TestSession_LNSOutgoingWaitTunnel_Stub~~ | ~~session_fsm_test.go~~ | ~~AC-21~~ | Deferred to LAC-initiated tunnel spec |
| TestParseICRQ_Valid | session_fsm_test.go | ICRQ parser: valid body with all fields | [ ] |
| TestParseICRQ_MissingSID | session_fsm_test.go | ICRQ parser: missing Assigned Session ID -> error | [ ] |
| TestParseICCN_Valid | session_fsm_test.go | ICCN parser: valid body with proxy AVPs | [ ] |
| TestParseICCN_MissingTxSpeed | session_fsm_test.go | ICCN parser: missing Tx Connect Speed -> error | [ ] |
| TestParseOCRQ_Valid | session_fsm_test.go | OCRQ parser: valid body with all required fields | [ ] |
| TestParseCDN_Valid | session_fsm_test.go | CDN parser: Result Code + optional Q.931 | [ ] |
| TestWriteICRPBody | session_fsm_test.go | ICRP builder round-trip | [ ] |
| TestWriteICCNBody | session_fsm_test.go | ICCN builder round-trip | [ ] |
| TestWriteOCRPBody | session_fsm_test.go | OCRP builder round-trip | [ ] |
| TestWriteOCCNBody | session_fsm_test.go | OCCN builder round-trip | [ ] |
| TestWriteCDNBody | session_fsm_test.go | CDN builder round-trip | [ ] |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Assigned Session ID | 1-65535 | 65535 | 0 (reserved) | N/A (uint16) |
| Call Serial Number | 0-4294967295 | 4294967295 | N/A (any uint32 valid) | N/A (uint32) |
| Max Sessions config | 0-65535 | 65535 | N/A (0 = unbounded) | 65536 |
| Tx Connect Speed | 0-4294967295 | 4294967295 | N/A (any uint32 valid) | N/A (uint32) |
| Result Code (CDN) | 1-7 | 7 | 0 (invalid) | 8+ (unknown, treated as general) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| session-incoming-lns.ci | test/l2tp/ | Config + UDP ICRQ/ICCN -> session established | [ ] |
| session-cdn-teardown.ci | test/l2tp/ | Established session + CDN -> session destroyed | [ ] |
| session-stopccn-cascade.ci | test/l2tp/ | Tunnel with sessions + StopCCN -> all sessions cleared | [ ] |
| l2tp-max-sessions.ci | test/parse/ | Config with max-sessions parses correctly | [ ] |

### Future (if deferring any tests)
- Session timeout timer (phase 5: no session-level timers in phase 4)
- Kernel session creation on established (phase 5)
- PPP negotiation triggered by session-established (phase 6)

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/l2tp/tunnel.go` | Add `sessions map[uint16]*L2TPSession`, `nextSessionID uint16`, session map helpers (allocateSID, addSession, removeSession, clearSessions) |
| `internal/component/l2tp/tunnel_fsm.go` | Extend handleMessage to dispatch session-scoped messages via dispatchToSession; extend handleStopCCN to cascade CDN to all sessions |
| `internal/component/l2tp/config.go` | Add `MaxSessions uint16` to Parameters; extract from YANG; add env var |
| `internal/component/l2tp/config_test.go` | Test MaxSessions extraction |
| `internal/component/l2tp/schema/ze-l2tp-conf.yang` | Add `max-sessions` leaf under `l2tp {}` |
| `internal/component/l2tp/reactor.go` | Pass MaxSessions through to TunnelDefaults or tunnel constructor |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `schema/ze-l2tp-conf.yang` |
| Env var | Yes | `config.go` (ze.l2tp.max-sessions) |
| Tunnel struct | Yes | `tunnel.go` (sessions map) |
| handleMessage dispatch | Yes | `tunnel_fsm.go` |
| StopCCN cascade | Yes | `tunnel_fsm.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (max-sessions leaf) |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2661.md` (session state machines section) |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

| File | Purpose | Est. lines |
|------|---------|-----------|
| `internal/component/l2tp/session.go` | L2TPSession struct, L2TPSessionState enum, session map helpers on tunnel, session ID allocation | ~180 |
| `internal/component/l2tp/session_fsm.go` | All session message handlers (handleICRQ, handleICRP, handleICCN, handleOCRQ, handleOCRP, handleOCCN, handleCDN, handleWEN, handleSLI), parsers (parseICRQ, parseICCN, parseOCRQ, parseOCRP, parseOCCN, parseCDN), builders (writeICRPBody, writeICCNBody, writeOCRPBody, writeOCCNBody, writeCDNBody) | ~600 |
| `internal/component/l2tp/session_fsm_test.go` | All session unit tests: FSM transitions, parsers, builders, boundary tests | ~800 |
| `test/l2tp/session-incoming-lns.ci` | Functional test: incoming call LNS-side handshake | ~40 |
| `test/l2tp/session-cdn-teardown.ci` | Functional test: CDN session teardown | ~30 |
| `test/l2tp/session-stopccn-cascade.ci` | Functional test: StopCCN cascades to sessions | ~35 |
| `test/parse/l2tp-max-sessions.ci` | Config parse test: max-sessions leaf | ~15 |

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

1. **Session struct and state enum** (`session.go`): Define L2TPSessionState (Idle, WaitTunnel, WaitReply, WaitConnect, WaitCSAnswer, Established), L2TPSession struct with all fields, session map helpers on L2TPTunnel (allocateSID, addSession, removeSession, clearSessions). Add sessions map to tunnel.go.

2. **ICRQ/ICRP handlers** (`session_fsm.go`): parseICRQ, handleICRQ (create session, send ICRP), writeICRPBody. Extend handleMessage dispatch. Test with TestSession_IncomingLNS (ICRQ -> ICRP). Boundary: SID=0 rejected.

3. **ICCN handler** (`session_fsm.go`): parseICCN (with proxy LCP/auth AVP capture), handleICCN (validate, transition to established). Test full handshake ICRQ->ICRP->ICCN. Boundary: missing Tx Connect Speed.

4. **CDN handler** (`session_fsm.go`): parseCDN, handleCDN (destroy session), writeCDNBody. Test CDN in all states. Extend handleStopCCN to cascade CDN to all sessions.

5. **OCRQ/OCRP/OCCN handlers** (`session_fsm.go`): parseOCRQ, handleOCRQ (create session, send OCRP), parseOCRP, handleOCRP, parseOCCN, handleOCCN, writeOCRPBody, writeOCCNBody. Test outgoing call LAC/LNS sequences.

6. **WEN/SLI handlers** (`session_fsm.go`): handleWEN (capture CallErrors), handleSLI (capture ACCM). Test counters captured.

7. **Config and limits**: Add MaxSessions to Parameters + YANG + env var. Enforce in handleICRQ/handleOCRQ. Test limit enforcement.

8. **Functional tests**: Write .ci tests for session-incoming-lns, session-cdn-teardown, session-stopccn-cascade, l2tp-max-sessions.

9. **Verification**: `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation and test |
| Correctness | Session FSM transitions match RFC 2661 Section 10 exactly |
| Session cleanup | StopCCN cascades CDN to all sessions; no session leaks |
| ID semantics | Header SessionID = recipient's ID; CDN Assigned SID AVP = sender's ID |
| Mandatory AVP handling | Unknown M=1 in session message -> CDN (not StopCCN) per S24.12 |
| Buffer-first | No append/make in session message builders |
| State guards | Every handler checks tunnel state (must be established for session ops) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| session.go | `go vet`, struct fields match spec |
| session_fsm.go | All handlers + parsers + builders present |
| session_fsm_test.go | All unit tests pass with -race |
| .ci tests | `ze-test l2tp` passes |
| YANG leaf | `test/parse/l2tp-max-sessions.ci` passes |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Assigned Session ID = 0 rejected; all required AVPs validated before acting |
| Resource limits | Max sessions enforced; no unbounded session creation |
| Mandatory AVP | Unknown M=1 tears down session not tunnel (S24.12) |
| ID collision | Session ID allocation retries on collision; no infinite loop |

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

- Session messages are tunnel-level control messages with session scope. They are sequenced by the tunnel's ReliableEngine, not by a session-level engine. This means session message ordering is per-tunnel, not per-session.
- ICRQ uses SessionID=0 in the header (like SCCRQ uses TunnelID=0), so session-creating messages dispatch at the tunnel level, not the session level.
- CDN's Assigned Session ID AVP carries the SENDER's own ID (for identification), not the recipient's. This is the opposite of the header SessionID convention.
- Proxy LCP/auth AVPs in ICCN are opaque byte blobs captured for phase 6. Phase 4 does not interpret them beyond storing them.
- LAC-initiated sessions (incoming LAC side, outgoing LNS side) require LAC-initiated tunnel creation (sending SCCRQ), which phase 3 does not implement. Deferred to a separate spec within the l2tp set; must complete before the set closes.

## RFC Documentation

Add `// RFC 2661 Section X.Y` above enforcing code:
- Section 10.1-10.4: session state machine transitions
- Section 7.6-7.14: required AVPs per session message
- Section 4.1: Message Type AVP must be first
- Section 24.12: unknown mandatory AVP in session message -> CDN (not StopCCN)
- Section 18: proxy LCP and proxy authentication AVP handling
- Section 19: WEN and SLI message handling

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
