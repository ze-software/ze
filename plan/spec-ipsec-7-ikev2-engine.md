# Spec: ipsec-7 -- IKEv2 Engine

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | ipsec-1, ipsec-5, ipsec-6 |
| Phase | 1/10 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions (native IKEv2 architecture)
4. `rfc/short/rfc7296.md` -- IKEv2 core protocol (exchanges, FSM, retransmission)
5. `rfc/short/rfc7427.md` -- signature authentication in IKEv2 (AUTH method 14)
6. `internal/component/iface/pppoe_client.go` -- per-peer goroutine lifecycle model
7. `docs/architecture/core-design.md` -- component lifecycle, registration pattern
8. `spec-ipsec-5-ikev2-wire.md` -- wire codec this spec depends on
9. `spec-ipsec-6-ikev2-crypto.md` -- crypto primitives this spec depends on

## Task

Implement the IKEv2 state machine and engine: the core component that negotiates
IKE Security Associations with remote peers. This is the heart of Ze's native IKEv2
implementation, sitting between the wire codec (ipsec-5) and crypto layer (ipsec-6)
below, and the child SA / dataplane layer (ipsec-8) above.

The engine manages:
- A per-IKE-SA finite state machine (goroutine per SA) covering IKE_SA_INIT and
  IKE_AUTH exchanges
- X.509 certificate authentication (RSA-PSS, ECDSA via RFC 7427 method 14) and PSK
  authentication, with certificate chain validation using the PKI store (ipsec-1)
- Retransmission with exponential backoff (RFC 7296 Section 2.4)
- UDP transport on port 500 (port 4500 for NAT-T is wired by ipsec-9)
- Message ID tracking and windowing
- IKE SA table management (create, lookup by SPI pair, remove)
- Config reconciliation (start new peers, stop removed, restart changed)
- Bus events for IKE SA lifecycle (established, deleted)

The PPPoE client lifecycle in `internal/component/iface/pppoe_client.go` is the model
for per-peer goroutine management: config-driven start/stop, reconnect with backoff,
reconciliation on config reload.

strongSwan reference: `src/libcharon/sa/ike_sa.c` (FSM), `src/libcharon/sa/ikev2/tasks/`
(task-based exchange handling), `src/libcharon/encoding/` (message dispatch).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern, event bus
  -> Constraint: IKE engine registers via init() in register.go, follows OnConfigure/OnShutdown lifecycle
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle: Start/Stop, reconnect backoff, reconcilePPPoEClients
  -> Decision: per-peer goroutine with reconnect follows PPPoEClient.run() pattern
- [ ] `spec-ipsec-0-umbrella.md` -- native IKEv2 design, no external daemon
  -> Constraint: Ze owns the full IKEv2 protocol; no charon, no VICI, no swanctl.conf
- [ ] `spec-ipsec-5-ikev2-wire.md` -- wire codec API (Encode/Decode message, payload types)
  -> Constraint: engine uses wire codec for all message serialization; never constructs raw bytes
- [ ] `spec-ipsec-6-ikev2-crypto.md` -- crypto API (DH, PRF, encryption, proposal negotiation)
  -> Constraint: engine uses crypto layer for all key derivation and proposal selection
- [ ] `spec-ipsec-1-pki-store.md` -- PKI store API (certificate lookup, chain validation)
  -> Constraint: X.509 AUTH uses PKI store for local cert/key and remote cert chain validation

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2: exchanges, FSM, retransmission, message IDs, AUTH methods
  -> Constraint: IKE_SA_INIT is exchange type 34; IKE_AUTH is type 35; message IDs start at 0 for initiator
- [ ] `rfc/short/rfc7427.md` -- signature authentication: AUTH method 14, ASN.1 AlgorithmIdentifier, SIGNATURE_HASH_ALGORITHMS notify
  -> Constraint: prefer AUTH method 14 over legacy types 1 (RSA) and 9 (ECDSA); announce SIGNATURE_HASH_ALGORITHMS in IKE_SA_INIT

**Key insights:**
- Per-IKE-SA goroutine: each SA has its own FSM goroutine, similar to PPPoEClient.run()
- IKE_SA_INIT (4 messages total with IKE_AUTH) establishes crypto and authenticates
- Retransmission: initiator retransmits unanswered requests; responder retransmits last response
- Message IDs: sequential uint32, separate counters for initiator and responder
- SPI: 8-byte random value chosen by each side; SPI pair identifies the IKE SA
- AUTH method 14 (RFC 7427): modern digital signature with ASN.1 algorithm ID
- PSK AUTH: prf(prf(shared_secret, "Key Pad for IKEv2"), signed_octets)
- Config reconciliation: diff peer names, stop removed, start new, restart changed (same as PPPoE)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient: Start/Stop, run() goroutine, reconnect backoff, reconcilePPPoEClients diffs config vs running
  -> Decision: IKE SA manager mirrors this pattern; per-peer goroutine, reconcileIKEPeers on config reload
- [ ] `internal/core/events/events.go` -- EventBus, Publish, Subscribe. JSON payloads
  -> Constraint: IKE SA events use EventBus; topic prefix `vpn/ipsec/`
- [ ] `docs/architecture/core-design.md` -- component registration, OnConfigure, OnShutdown
  -> Constraint: IKE engine is a component at internal/component/ike/; registers via init()
- [ ] `internal/component/ipsec/types.go` -- IKEGroup, ESPGroup, SiteToSitePeer structs (from ipsec-3)
  -> Constraint: engine consumes these config types directly
- [ ] `internal/component/ipsec/config.go` -- IPsec config parser (from ipsec-3)
  -> Constraint: engine receives parsed config, does not re-parse YANG tree

**Behavior to preserve:**
- PPPoE client lifecycle unchanged
- EventBus API unchanged
- Config transaction and rollback mechanics unchanged
- IPsec config types (ipsec-3) unchanged

**Behavior to change:**
- New `internal/component/ike/` component with registration, engine, and transport
- UDP listeners on port 500 for IKEv2 protocol messages
- Per-peer IKE SA goroutines managing FSM state transitions
- IKE SA table tracking all active SAs by SPI pair
- Bus events published for IKE SA established/deleted
- Config reconciliation starts/stops/restarts peer connections on reload

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG `vpn ipsec {}` tree parsed (ipsec-3), engine receives SiteToSitePeer list
- Network: UDP packet on port 500 received by transport layer
- Timer: retransmission timer fires for unanswered request

### Transformation Path
1. OnConfigure: engine receives parsed IPsec config (IKEGroups, ESPGroups, SiteToSitePeers)
2. Reconciler diffs new config against running SA table; starts/stops/restarts peers
3. For initiator peers: engine spawns goroutine, sends IKE_SA_INIT (SAi1, KEi, Ni)
4. Transport sends UDP datagram on port 500 via wire codec (ipsec-5) encode
5. Response received: transport dispatches to SA goroutine by SPI pair
6. SA goroutine decodes response via wire codec, processes in FSM
7. IKE_SA_INIT response: crypto layer (ipsec-6) computes SKEYSEED and SK_* keys
8. IKE_AUTH request: engine builds AUTH payload (X.509 signature or PSK), encrypted with SK_e
9. IKE_AUTH response: engine verifies remote AUTH, validates certificate chain via PKI store
10. On success: IKE SA established, bus event published, ready for CREATE_CHILD_SA (ipsec-8)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to engine | Parsed SiteToSitePeer structs from ipsec-3 | [ ] |
| Engine to wire codec | ike/wire.Encode/Decode for message serialization | [ ] |
| Engine to crypto | ike/crypto for DH, SKEYSEED, encryption/decryption, proposal negotiation | [ ] |
| Engine to PKI store | pki.Store interface for certificate lookup and chain validation | [ ] |
| Engine to network | UDP socket send/receive via transport layer | [ ] |
| Engine to bus | EventBus.Publish for SA lifecycle events | [ ] |

### Integration Points
- `internal/component/ike/wire/` (ipsec-5) -- message encoding/decoding
- `internal/component/ike/crypto/` (ipsec-6) -- key derivation, encryption, proposals
- `internal/component/pki/` (ipsec-1) -- certificate store for X.509 authentication
- `internal/component/ipsec/` (ipsec-3) -- config types (IKEGroup, ESPGroup, SiteToSitePeer)
- `internal/core/events/` -- EventBus for SA state change events
- `cmd/ze/hub/main.go` -- component wiring at startup

### Architectural Verification
- [ ] No bypassed layers (engine uses wire codec and crypto layer, never constructs raw bytes)
- [ ] No unintended coupling (engine depends on PKI store interface, not internals)
- [ ] No duplicated functionality (per-peer goroutine follows PPPoE pattern, does not reinvent)
- [ ] Zero-copy preserved where applicable (wire codec uses buffer-first pattern from ipsec-5)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `vpn ipsec { site-to-site { peer ... } }` | -> | IKE engine starts peer goroutine, sends IKE_SA_INIT | `test/ipsec/ipsec-site-to-site-initiate.ci` |
| IKE_SA_INIT + IKE_AUTH exchange completes | -> | IKE SA established, bus event published | `test/ipsec/ipsec-sa-established.ci` |
| Config reload removes a peer | -> | Reconciler stops peer goroutine, deletes IKE SA | `test/ipsec/ipsec-reload-peer-remove.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has site-to-site peer with connection-type initiate | IKE engine sends IKE_SA_INIT on port 500; proposal, DH key exchange, and nonce payloads correct per RFC 7296 |
| AC-2 | Remote peer responds with IKE_SA_INIT response | Engine selects matching proposal, computes SKEYSEED and SK_* key hierarchy |
| AC-3 | IKE_AUTH with X.509 certificate (RFC 7427 method 14) | Engine sends AUTH payload with digital signature; validates remote certificate chain via PKI store |
| AC-4 | IKE_AUTH with PSK | Engine computes AUTH as prf(prf(shared_secret, "Key Pad for IKEv2"), signed_octets) |
| AC-5 | IKE_SA_INIT request unanswered for retransmit interval | Engine retransmits with exponential backoff per RFC 7296 Section 2.4 |
| AC-6 | IKE_AUTH completed successfully | IKE SA in ESTABLISHED state; bus event vpn/ipsec/sa-up published with peer name and SA details |
| AC-7 | Ze acts as responder (connection-type respond) | Engine accepts incoming IKE_SA_INIT on port 500, completes IKE_AUTH, authenticates initiator |
| AC-8 | Config reload changes peer remote-address | Reconciler stops old SA goroutine, starts new one to changed address; other SAs untouched |
| AC-9 | Config reload adds new peer | New SA goroutine started; existing SAs untouched |
| AC-10 | Config reload removes peer | SA goroutine stopped, IKE SA deleted; bus event vpn/ipsec/sa-down published |
| AC-11 | Invalid remote AUTH payload (bad signature or wrong PSK) | IKE_AUTH rejected with AUTHENTICATION_FAILED notify; SA not created |
| AC-12 | No matching proposal in remote IKE_SA_INIT | Responded with NO_PROPOSAL_CHOSEN notify; SA not created |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFSMInitiatorInit` | `internal/component/ike/engine/fsm_test.go` | FSM starts in INIT state, transitions to SA_INIT_SENT on initiate | |
| `TestFSMInitiatorSAInitResponse` | `internal/component/ike/engine/fsm_test.go` | FSM processes IKE_SA_INIT response, transitions to AUTH_SENT | |
| `TestFSMInitiatorAuthResponse` | `internal/component/ike/engine/fsm_test.go` | FSM processes IKE_AUTH response, transitions to ESTABLISHED | |
| `TestFSMResponderSAInit` | `internal/component/ike/engine/fsm_test.go` | FSM receives IKE_SA_INIT, responds, transitions to SA_INIT_RESPONDED | |
| `TestFSMResponderAuth` | `internal/component/ike/engine/fsm_test.go` | FSM receives IKE_AUTH, authenticates, transitions to ESTABLISHED | |
| `TestAuthX509Sign` | `internal/component/ike/engine/auth_test.go` | X.509 AUTH payload built with RFC 7427 digital signature | |
| `TestAuthX509Verify` | `internal/component/ike/engine/auth_test.go` | Remote X.509 AUTH payload verified with certificate chain | |
| `TestAuthPSKCompute` | `internal/component/ike/engine/auth_test.go` | PSK AUTH computed as prf(prf(secret, "Key Pad for IKEv2"), signed_octets) | |
| `TestAuthPSKVerify` | `internal/component/ike/engine/auth_test.go` | Remote PSK AUTH verified against local shared secret | |
| `TestRetransmitBackoff` | `internal/component/ike/engine/fsm_test.go` | Unanswered request retransmitted with exponential backoff | |
| `TestMessageIDTracking` | `internal/component/ike/engine/fsm_test.go` | Message IDs increment correctly for initiator and responder | |
| `TestSATableCreate` | `internal/component/ike/engine/table_test.go` | SA created and retrievable by SPI pair | |
| `TestSATableRemove` | `internal/component/ike/engine/table_test.go` | SA removed from table, lookup returns nil | |
| `TestSATableLookupBySPI` | `internal/component/ike/engine/table_test.go` | Lookup by initiator+responder SPI pair returns correct SA | |
| `TestReconcilePeersAdded` | `internal/component/ike/engine/reconcile_test.go` | New peer in config triggers SA goroutine start | |
| `TestReconcilePeersRemoved` | `internal/component/ike/engine/reconcile_test.go` | Removed peer triggers SA goroutine stop | |
| `TestReconcilePeersChanged` | `internal/component/ike/engine/reconcile_test.go` | Changed peer triggers stop + start | |
| `TestReconcilePeersUnchanged` | `internal/component/ike/engine/reconcile_test.go` | Unchanged peer not touched | |
| `TestUDPTransportSendReceive` | `internal/component/ike/transport/udp_test.go` | UDP send/receive on port 500, dispatch by SPI | |
| `TestAuthFailedNotify` | `internal/component/ike/engine/fsm_test.go` | Bad remote AUTH triggers AUTHENTICATION_FAILED notify | |
| `TestNoProposalChosenNotify` | `internal/component/ike/engine/fsm_test.go` | No matching proposal triggers NO_PROPOSAL_CHOSEN notify | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Message ID | 0-4294967295 (uint32) | 4294967295 | N/A (wraps) | N/A (wraps) |
| SPI | 1-2^64-1 (uint64, non-zero) | 2^64-1 | 0 (invalid, zero SPI) | N/A |
| Retransmit count | 0-max_retransmit (configurable) | max value | N/A | max+1 triggers timeout |
| Retransmit interval | 500ms base, 60s max | 60s | N/A | N/A (capped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-site-to-site-initiate` | `test/ipsec/ipsec-site-to-site-initiate.ci` | Config with site-to-site peer triggers IKE negotiation | |
| `ipsec-sa-established` | `test/ipsec/ipsec-sa-established.ci` | IKE SA established after IKE_SA_INIT + IKE_AUTH | |
| `ipsec-reload-peer-remove` | `test/ipsec/ipsec-reload-peer-remove.ci` | Config reload removes peer, SA deleted | |
| `ipsec-auth-x509` | `test/ipsec/ipsec-auth-x509.ci` | X.509 certificate authentication succeeds | |
| `ipsec-auth-psk` | `test/ipsec/ipsec-auth-psk.ci` | PSK authentication succeeds | |

## Files to Modify
- `cmd/ze/hub/main.go` -- import `internal/component/ike/engine` for init() registration

## Files to Create
- `internal/component/ike/engine/fsm.go` -- per-IKE-SA finite state machine (states, transitions, timers)
- `internal/component/ike/engine/fsm_test.go` -- FSM unit tests
- `internal/component/ike/engine/sa.go` -- IKE SA struct (SPI pair, keys, state, peer config)
- `internal/component/ike/engine/auth.go` -- AUTH payload computation and verification (X.509, PSK)
- `internal/component/ike/engine/auth_test.go` -- AUTH unit tests
- `internal/component/ike/engine/initiator.go` -- initiator exchange logic (build IKE_SA_INIT, IKE_AUTH requests)
- `internal/component/ike/engine/responder.go` -- responder exchange logic (process incoming, build responses)
- `internal/component/ike/engine/table.go` -- SA table (concurrent-safe map by SPI pair)
- `internal/component/ike/engine/table_test.go` -- SA table unit tests
- `internal/component/ike/engine/reconcile.go` -- config reconciliation (diff peers, start/stop goroutines)
- `internal/component/ike/engine/reconcile_test.go` -- reconciler unit tests
- `internal/component/ike/engine/register.go` -- component registration via init()
- `internal/component/ike/engine/events.go` -- bus event types for vpn/ipsec/sa-up, sa-down
- `internal/component/ike/transport/udp.go` -- UDP socket listener/sender, dispatch by SPI pair
- `internal/component/ike/transport/udp_test.go` -- transport unit tests
- `test/ipsec/ipsec-site-to-site-initiate.ci` -- functional test: IKE negotiation starts
- `test/ipsec/ipsec-sa-established.ci` -- functional test: IKE SA established
- `test/ipsec/ipsec-reload-peer-remove.ci` -- functional test: peer removal on reload
- `test/ipsec/ipsec-auth-x509.ci` -- functional test: X.509 authentication
- `test/ipsec/ipsec-auth-psk.ci` -- functional test: PSK authentication

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A (engine consumes existing ipsec-3 config) |
| CLI commands/flags | [ ] | Deferred to ipsec-10 (CLI/Diag spec) |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [ ] | `test/ipsec/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (IPsec VPN: have) |
| 2 | Config syntax changed? | [ ] | N/A (config from ipsec-3) |
| 3 | CLI command added/changed? | [ ] | Deferred to ipsec-10 |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/ipsec.md` (new, deferred to ipsec-10) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfc7296.md` (IKEv2 engine), `rfc/short/rfc7427.md` (signature auth) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` (native IKEv2 support) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` (new ike component) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register IKE engine component, create entry points
   - Tests: `test/ipsec/ipsec-site-to-site-initiate.ci` (failing stub)
   - Files: `register.go`, `cmd/ze/hub/main.go` (import)
   - Verify: component registered, OnConfigure called, stub logs "not implemented"

2. **Phase: SA Table and Transport** -- SA table management, UDP socket listener
   - Tests: `TestSATableCreate`, `TestSATableRemove`, `TestSATableLookupBySPI`, `TestUDPTransportSendReceive`
   - Files: `table.go`, `table_test.go`, `sa.go`, `transport/udp.go`, `transport/udp_test.go`
   - Verify: SA table is concurrent-safe; UDP transport sends and receives on port 500

3. **Phase: FSM Core** -- state machine, IKE_SA_INIT exchange for initiator and responder
   - Tests: `TestFSMInitiatorInit`, `TestFSMInitiatorSAInitResponse`, `TestFSMResponderSAInit`
   - Files: `fsm.go`, `fsm_test.go`, `initiator.go`, `responder.go`
   - Verify: IKE_SA_INIT completes between two local instances; SKEYSEED derived

4. **Phase: Authentication** -- X.509 (RFC 7427) and PSK AUTH payloads
   - Tests: `TestAuthX509Sign`, `TestAuthX509Verify`, `TestAuthPSKCompute`, `TestAuthPSKVerify`
   - Files: `auth.go`, `auth_test.go`
   - Verify: AUTH payloads computed and verified correctly for both methods

5. **Phase: IKE_AUTH Exchange** -- full IKE_AUTH for initiator and responder
   - Tests: `TestFSMInitiatorAuthResponse`, `TestFSMResponderAuth`, `TestAuthFailedNotify`, `TestNoProposalChosenNotify`
   - Files: `fsm.go` (extend), `initiator.go` (extend), `responder.go` (extend), `events.go`
   - Verify: IKE_AUTH completes; SA transitions to ESTABLISHED; bus event published

6. **Phase: Retransmission** -- exponential backoff for unanswered requests
   - Tests: `TestRetransmitBackoff`, `TestMessageIDTracking`
   - Files: `fsm.go` (extend)
   - Verify: unanswered requests retransmitted with increasing intervals; capped at max

7. **Phase: Reconciliation** -- config reload diffs peers
   - Tests: `TestReconcilePeersAdded`, `TestReconcilePeersRemoved`, `TestReconcilePeersChanged`, `TestReconcilePeersUnchanged`
   - Files: `reconcile.go`, `reconcile_test.go`
   - Verify: peer diffs correct; goroutines started/stopped as needed

8. **Functional tests** -- create after feature works
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has implementation with file:line |
| Correctness | AUTH payload matches RFC 7296/7427; retransmission intervals correct; SPI never zero |
| Naming | Bus event topics use `vpn/ipsec/` prefix; FSM state names match RFC terminology |
| Data flow | Engine uses wire codec for all serialization; crypto layer for all key ops; never raw bytes |
| Rule: no-layering | Engine does not install XFRM SAs (that is ipsec-8); engine does not handle EAP (that is ipsec-9) |
| Rule: buffer-first | Wire codec interactions follow buffer-first pattern |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| FSM handles IKE_SA_INIT + IKE_AUTH | `go test -run TestFSMInitiator` passes |
| X.509 AUTH works | `go test -run TestAuthX509` passes |
| PSK AUTH works | `go test -run TestAuthPSK` passes |
| SA table is concurrent-safe | `go test -race -run TestSATable` passes |
| Reconciler diffs peers | `go test -run TestReconcilePeers` passes |
| Component registered | `grep -rn 'RegisterComponent.*ike' internal/component/ike/` |
| Bus events published | `grep -rn 'Publish.*vpn/ipsec' internal/component/ike/` |
| Functional tests exist | `ls test/ipsec/ipsec-site-to-site-initiate.ci test/ipsec/ipsec-sa-established.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All incoming IKEv2 messages validated: header length, payload chain integrity, SPI non-zero, message ID in window |
| Crypto safety | SPI generated from crypto/rand; nonces from crypto/rand; DH private keys not logged; SK_* keys zeroed after use |
| DoS resistance | Rate limiting on incoming IKE_SA_INIT; cookie mechanism (RFC 7296 Section 2.6) for half-open SA flood |
| Certificate validation | Full chain validation; expiry check; key usage check; no self-signed certs accepted unless explicitly configured |
| Error handling | Error notifies (AUTHENTICATION_FAILED, NO_PROPOSAL_CHOSEN, INVALID_SYNTAX) sent without leaking internal state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: IKE_SA_INIT exchange flow, IKE_AUTH exchange flow, AUTH payload computation,
retransmission timers, message ID windowing, error notify generation.

Add `// RFC 7427 Section X.Y: "<quoted requirement>"` for digital signature authentication.

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-7-ikev2-engine.md`
- [ ] Summary included in commit
