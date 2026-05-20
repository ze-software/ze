# Spec: ipsec-9 -- IKEv2 EAP Authentication and NAT-T

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | ipsec-4, ipsec-7 |
| Phase | 9/9 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions
4. `spec-ipsec-4-data-model-eap.md` -- EAP config types, virtual IP pool, EAP user database
5. `spec-ipsec-7-ikev2-engine.md` -- IKEv2 FSM, IKE_AUTH exchange flow
6. `rfc/short/rfc3748.md` -- EAP framework, method types, exchange model
7. `rfc/short/rfc5216.md` -- EAP-TLS, TLS-in-EAP framing, MSK derivation
8. `rfc/short/rfc2759.md` -- MS-CHAPv2 challenge/response, NT hash, MPPE keys
9. `rfc/short/rfc3948.md` -- NAT-T UDP encapsulation, port 4500

## Task

Extend the IKEv2 engine (ipsec-7) with EAP authentication and NAT traversal.
EAP enables road warrior VPN from Windows, macOS, iOS, and Android clients using
their built-in IKEv2 support. NAT-T allows IPsec tunnels to work when either
peer is behind a NAT device.

### EAP Authentication

EAP modifies the IKE_AUTH exchange. Instead of the initiator sending an AUTH payload
directly, the responder (Ze) authenticates first with its X.509 certificate, then
requests EAP authentication from the initiator. Multiple EAP request/response rounds
follow until the EAP method completes. The initiator then sends a final AUTH payload
derived from the EAP MSK (Master Session Key).

Two EAP methods are supported:

| Method | Type | Use case | Credential |
|--------|------|----------|-----------|
| EAP-TLS | 13 | Certificate-based client auth | Client certificate (strongest) |
| EAP-MSCHAPv2 | 26 | Password-based client auth | Username + password (Windows default) |

### Virtual IP Assignment

Road warrior clients receive a virtual IP address via the IKEv2 Configuration Payload
(CP, RFC 7296 Section 2.19). Ze manages an address pool configured in the YANG tree
(ipsec-4). Configuration attributes pushed to the client include: INTERNAL_IP4_ADDRESS,
INTERNAL_IP6_ADDRESS, INTERNAL_IP4_DNS, INTERNAL_IP6_DNS.

### NAT Traversal

NAT is detected during IKE_SA_INIT using NAT_DETECTION_SOURCE_IP and
NAT_DETECTION_DESTINATION_IP notify payloads. If NAT is detected, IKEv2 switches
to UDP port 4500 and ESP packets are encapsulated in UDP (RFC 3948). NAT keepalives
(periodic 0xFF byte) maintain the NAT binding.

## Required Reading

### Architecture Docs
- [ ] `spec-ipsec-0-umbrella.md` -- umbrella design decisions, EAP in scope
  -> Constraint: EAP-TLS and EAP-MSCHAPv2 are the two supported methods
- [ ] `spec-ipsec-4-data-model-eap.md` -- EAP config types consumed at runtime
  -> Constraint: EAP user lookup uses RemoteAccessConfig from ipsec-4
- [ ] `spec-ipsec-7-ikev2-engine.md` -- IKE_AUTH exchange flow to extend with EAP
  -> Constraint: EAP extends the responder's IKE_AUTH handling, not a parallel path
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle pattern
  -> Decision: NAT keepalive goroutine follows similar periodic-send pattern

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3748.md` -- EAP framework: packet format, exchange model, method types
  -> Constraint: EAP is lock-step request/response, authenticator retransmits, peer does not
- [ ] `rfc/short/rfc5216.md` -- EAP-TLS: TLS handshake in EAP, fragmentation (L/M/S flags), MSK from TLS-PRF
  -> Constraint: MSK = TLS-PRF(master_secret, "client EAP encryption", ...), first 64 octets
- [ ] `rfc/short/rfc2759.md` -- MS-CHAPv2: NtPasswordHash (MD4 of UTF-16LE), ChallengeResponse, AuthenticatorResponse
  -> Constraint: password must be UTF-16LE encoded before MD4; username stripped of DOMAIN\ prefix in ChallengeHash
- [ ] `rfc/short/rfc3948.md` -- NAT-T: UDP encapsulation of ESP, non-ESP marker, port 4500
  -> Constraint: 4 bytes of zero prepended to IKEv2 on port 4500; kernel handles ESP-in-UDP via XFRM SA encap attribute
- [ ] `rfc/short/rfc7296.md` -- IKEv2 Sections 2.16 (EAP), 2.19 (Configuration Payload), 2.23 (NAT-T)
  -> Constraint: responder MUST authenticate with certificate before requesting EAP

**Key insights:**
- EAP-MSCHAPv2 is the default auth method for Windows built-in IKEv2 VPN client
- EAP-TLS wraps a full TLS handshake inside EAP packets with fragmentation support
- MSK from EAP feeds into IKEv2 AUTH: prf(prf(MSK, "Key Pad for IKEv2"), signed_octets)
- NAT detection uses SHA-1 hash of SPI pair + IP + port; mismatch means NAT present
- On port 4500, both IKEv2 and ESP share the socket; non-ESP marker distinguishes them
- Virtual IP pool is simple allocate/release; no DHCP relay or external server

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/` (from ipsec-7) -- IKE_AUTH exchange, FSM states
  -> Constraint: EAP extends IKE_AUTH responder path; initiator path unchanged for site-to-site
- [ ] `internal/component/ike/transport/udp.go` (from ipsec-7) -- UDP socket on port 500
  -> Constraint: NAT-T adds port 4500 listener and port switching logic
- [ ] `internal/component/ike/wire/payload_notify.go` (from ipsec-5) -- Notify payload type
  -> Constraint: NAT_DETECTION_SOURCE_IP (16388) and NAT_DETECTION_DESTINATION_IP (16389) already encoded
- [ ] `internal/component/ike/wire/payload_eap.go` (from ipsec-5) -- EAP payload type
  -> Constraint: EAP payload carries opaque EAP packet bytes; method dispatch happens in this spec
- [ ] `internal/component/ike/wire/payload_cp.go` (from ipsec-5) -- Configuration Payload
  -> Constraint: CP payload already encoded; this spec fills attribute values from pool

**Behavior to preserve:**
- IKE_SA_INIT exchange unchanged (NAT detection adds notify payloads but does not change flow)
- IKE_AUTH with X.509/PSK for site-to-site peers unchanged
- Wire codec for all payload types unchanged
- Crypto primitives unchanged

**Behavior to change:**
- IKE_AUTH responder path extended: when peer requires EAP, send EAP-Request instead of expecting AUTH
- EAP method dispatch: route EAP packets to EAP-TLS or EAP-MSCHAPv2 handler based on type
- EAP-TLS handler: TLS handshake state machine, fragmentation, MSK extraction
- EAP-MSCHAPv2 handler: challenge generation, response validation, authenticator response, MSK derivation
- Virtual IP pool: allocate addresses from configured range, release on SA deletion
- NAT detection in IKE_SA_INIT: compute and verify NAT_DETECTION_*_IP hashes
- UDP transport: add port 4500 listener, non-ESP marker handling, port switching
- NAT keepalive: periodic 0xFF byte sender goroutine
- XFRM SA creation: set UDP encapsulation attribute when NAT detected

## Data Flow (MANDATORY)

### Entry Point
- IKE_AUTH exchange: responder detects EAP-capable peer (no AUTH payload from initiator, or EAP-only IDi)
- IKE_SA_INIT exchange: NAT detection notify payloads included by both sides

### Transformation Path

**EAP flow:**
1. Responder authenticates to initiator with X.509 certificate (ipsec-7, unchanged)
2. Responder sends EAP-Request/Identity
3. Initiator responds with EAP-Response/Identity (username)
4. Responder dispatches to EAP method based on config (EAP-TLS or EAP-MSCHAPv2)
5. Multiple EAP request/response rounds until method completes
6. Responder sends EAP-Success
7. Initiator sends IKE_AUTH with AUTH payload derived from MSK
8. Responder verifies AUTH, sends final IKE_AUTH response with AUTH + SA + TS + CP (virtual IP)

**NAT-T flow:**
1. During IKE_SA_INIT, both sides include NAT_DETECTION_*_IP notify payloads
2. Receiver computes expected hashes and compares
3. If mismatch: NAT detected, flag on IKE SA
4. Subsequent IKEv2 messages sent on port 4500 with non-ESP marker
5. Child SA creation includes UDP encapsulation attribute in XFRM SA
6. NAT keepalive goroutine sends 0xFF every 20s on the 4500 socket

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine to EAP framework | Method dispatch by EAP type from config | [ ] |
| EAP-TLS to Go crypto/tls | TLS handshake via tls.Conn over EAP transport | [ ] |
| EAP-MSCHAPv2 to password hash | NtPasswordHash from config (ipsec-4) | [ ] |
| EAP MSK to IKEv2 AUTH | prf(prf(MSK, "Key Pad for IKEv2"), signed_octets) | [ ] |
| Virtual IP pool to CP payload | Pool allocator returns IP, packed into CP attributes | [ ] |
| NAT detection to transport | NAT flag on IKE SA triggers port switch | [ ] |
| NAT flag to XFRM SA | ipsec-8 sets UDP encap attribute if NAT detected | [ ] |

### Integration Points
- `internal/component/ike/engine/` (ipsec-7) -- extended IKE_AUTH responder path
- `internal/component/ike/crypto/` (ipsec-6) -- PRF for AUTH from MSK
- `internal/component/ike/wire/` (ipsec-5) -- EAP, CP, Notify payload encode/decode
- `internal/component/ike/dataplane/` (ipsec-8) -- UDP encap attribute on XFRM SA
- `internal/component/ipsec/` (ipsec-4) -- RemoteAccessConfig, EAP user database, VirtualIPPool

### Architectural Verification
- [ ] No bypassed layers (EAP runs inside IKE_AUTH, not a parallel exchange)
- [ ] No unintended coupling (EAP methods are independent; pool is config-driven)
- [ ] No duplicated functionality (reuses crypto/tls for EAP-TLS, existing PRF for AUTH)
- [ ] Zero-copy preserved where applicable (EAP packets carried as byte slices)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH with EAP-MSCHAPv2 peer config | -> | EAP exchange completes, MSK derived, AUTH verified | `test/ipsec/ipsec-eap-mschapv2.ci` |
| IKE_AUTH with EAP-TLS peer config | -> | TLS handshake in EAP completes, MSK derived, AUTH verified | `test/ipsec/ipsec-eap-tls.ci` |
| IKE_SA_INIT with NAT present | -> | NAT detected, port switched to 4500, keepalive started | `test/ipsec/ipsec-nat-detection.ci` |
| Road warrior client requests virtual IP | -> | IP allocated from pool, CP response sent | `test/ipsec/ipsec-virtual-ip.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer configured with `authentication { mode eap-mschapv2 }` and matching user/password | EAP-MSCHAPv2 exchange completes in IKE_AUTH, client authenticated, IKE SA established |
| AC-2 | Peer configured with `authentication { mode eap-tls }` and client certificate | EAP-TLS exchange completes, TLS handshake succeeds, IKE SA established |
| AC-3 | EAP-MSCHAPv2 with wrong password | EAP-Failure sent, IKE_AUTH fails, no SA created |
| AC-4 | EAP-TLS with untrusted client certificate | TLS handshake fails, EAP-Failure sent, no SA created |
| AC-5 | Road warrior client requests virtual IP | IP allocated from configured pool, returned in CP response |
| AC-6 | Road warrior disconnects | Virtual IP released back to pool, available for next client |
| AC-7 | Pool exhausted (all IPs assigned) | New client receives INTERNAL_ADDRESS_FAILURE notify, no SA |
| AC-8 | Initiator behind NAT | NAT_DETECTION hash mismatch detected, subsequent messages on port 4500 |
| AC-9 | NAT detected | XFRM SA created with UDP encapsulation attribute (encap type, sport, dport) |
| AC-10 | NAT keepalive | 0xFF byte sent every 20s on port 4500 to maintain NAT binding |
| AC-11 | No NAT present | NAT_DETECTION hashes match, stays on port 500, no keepalive, no encap attribute |
| AC-12 | EAP MSK feeds AUTH | AUTH payload computed as prf(prf(MSK, "Key Pad for IKEv2"), signed_octets) and verified by peer |
| AC-13 | CP response includes DNS | INTERNAL_IP4_DNS and INTERNAL_IP6_DNS attributes set from pool config |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPMSCHAPv2Challenge` | `internal/component/ike/eap/eap_mschapv2_test.go` | Challenge generation, NtPasswordHash, GenerateNTResponse | |
| `TestEAPMSCHAPv2AuthResponse` | `internal/component/ike/eap/eap_mschapv2_test.go` | Mutual authentication: authenticator response verified | |
| `TestEAPMSCHAPv2MSK` | `internal/component/ike/eap/eap_mschapv2_test.go` | MSK derivation from MPPE keys (GetMasterKey, GetAsymmetricStartKey) | |
| `TestEAPMSCHAPv2WrongPassword` | `internal/component/ike/eap/eap_mschapv2_test.go` | Wrong password produces different NT-Response, rejected | |
| `TestEAPTLSHandshake` | `internal/component/ike/eap/eap_tls_test.go` | TLS handshake completes via EAP fragmentation | |
| `TestEAPTLSFragmentation` | `internal/component/ike/eap/eap_tls_test.go` | Large TLS records fragmented with L/M/S flags | |
| `TestEAPTLSMSK` | `internal/component/ike/eap/eap_tls_test.go` | MSK derived from TLS-PRF("client EAP encryption") | |
| `TestEAPTLSUntrustedCert` | `internal/component/ike/eap/eap_tls_test.go` | Untrusted client certificate causes TLS failure | |
| `TestEAPDispatch` | `internal/component/ike/eap/eap_test.go` | EAP method dispatched by configured type (13 or 26) | |
| `TestEAPIdentityExchange` | `internal/component/ike/eap/eap_test.go` | Identity request/response extracts username | |
| `TestVirtualIPPoolAllocate` | `internal/component/ike/eap/pool_test.go` | IP allocated from range, tracked as in-use | |
| `TestVirtualIPPoolRelease` | `internal/component/ike/eap/pool_test.go` | Released IP available for reallocation | |
| `TestVirtualIPPoolExhausted` | `internal/component/ike/eap/pool_test.go` | Allocation fails when pool full | |
| `TestVirtualIPPoolDualStack` | `internal/component/ike/eap/pool_test.go` | IPv4 + IPv6 allocated together from dual-stack pool | |
| `TestNATDetectionHash` | `internal/component/ike/transport/nat_test.go` | SHA-1(SPI_i, SPI_r, IP, Port) computed correctly | |
| `TestNATDetectionPresent` | `internal/component/ike/transport/nat_test.go` | Hash mismatch correctly identifies NAT | |
| `TestNATDetectionAbsent` | `internal/component/ike/transport/nat_test.go` | Hash match correctly identifies no NAT | |
| `TestNonESPMarker` | `internal/component/ike/transport/nat_test.go` | 4-byte zero prefix added/stripped on port 4500 | |
| `TestNATKeepalive` | `internal/component/ike/transport/keepalive_test.go` | 0xFF byte sent periodically | |
| `TestAuthFromMSK` | `internal/component/ike/engine/eap_test.go` | AUTH = prf(prf(MSK, "Key Pad for IKEv2"), signed_octets) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EAP type | 1-255 | 255 | 0 | 256 |
| NAT keepalive interval | 5-120 seconds | 120 | 4 | 121 |
| Virtual IP pool size | 1 - 65534 addresses | /16 (65534) | 0 (empty range) | /15 (too large) |
| EAP-TLS fragment size | 512-65535 bytes | 65535 | 511 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-mschapv2` | `test/ipsec/ipsec-eap-mschapv2.ci` | Windows client authenticates with username/password via EAP-MSCHAPv2 | |
| `ipsec-eap-tls` | `test/ipsec/ipsec-eap-tls.ci` | Client authenticates with certificate via EAP-TLS | |
| `ipsec-nat-detection` | `test/ipsec/ipsec-nat-detection.ci` | NAT detected, port switched, keepalive running | |
| `ipsec-virtual-ip` | `test/ipsec/ipsec-virtual-ip.ci` | Road warrior receives virtual IP from pool | |

## Files to Modify
- `internal/component/ike/engine/` -- extend IKE_AUTH responder for EAP flow; add EAP state to IKE SA
- `internal/component/ike/transport/udp.go` -- add port 4500 listener, non-ESP marker, port switching
- `internal/component/ike/dataplane/` (ipsec-8) -- set UDP encap attribute on XFRM SA when NAT detected

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A (EAP config handled by ipsec-4) |
| CLI commands/flags | [ ] | Deferred to ipsec-10 (`show vpn ipsec sa` shows EAP auth method) |
| Editor autocomplete | [ ] | YANG-driven (automatic from ipsec-4) |
| Functional test for new RPC/API | [ ] | `test/ipsec/ipsec-eap-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (EAP authentication: have, NAT-T: have) |
| 2 | Config syntax changed? | [ ] | Handled by ipsec-4 |
| 3 | CLI command added/changed? | [ ] | Deferred to ipsec-10 |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/ipsec.md` (extend with EAP and NAT-T sections) |
| 7 | Wire format changed? | [ ] | N/A (IKEv2 is standard) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfc3748.md`, `rfc/short/rfc5216.md`, `rfc/short/rfc2759.md`, `rfc/short/rfc3948.md` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` (EAP and NAT-T support) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` (IKE EAP subsystem) |

## Files to Create
- `internal/component/ike/eap/eap.go` -- EAP framework: method interface, dispatch, identity exchange
- `internal/component/ike/eap/eap_test.go` -- EAP dispatch and identity tests
- `internal/component/ike/eap/eap_tls.go` -- EAP-TLS handler: TLS handshake, fragmentation, MSK
- `internal/component/ike/eap/eap_tls_test.go` -- EAP-TLS tests
- `internal/component/ike/eap/eap_mschapv2.go` -- EAP-MSCHAPv2 handler: challenge/response, MSK
- `internal/component/ike/eap/eap_mschapv2_test.go` -- EAP-MSCHAPv2 tests
- `internal/component/ike/eap/mschapv2.go` -- MS-CHAPv2 crypto: NtPasswordHash, ChallengeResponse, AuthenticatorResponse, MPPE keys
- `internal/component/ike/eap/pool.go` -- Virtual IP pool: allocate, release, dual-stack
- `internal/component/ike/eap/pool_test.go` -- Pool tests
- `internal/component/ike/transport/nat.go` -- NAT detection: hash computation, non-ESP marker
- `internal/component/ike/transport/nat_test.go` -- NAT detection tests
- `internal/component/ike/transport/keepalive.go` -- NAT keepalive: periodic 0xFF sender
- `internal/component/ike/transport/keepalive_test.go` -- Keepalive tests
- `internal/component/ike/engine/eap_test.go` -- AUTH-from-MSK integration test
- `test/ipsec/ipsec-eap-mschapv2.ci` -- functional test
- `test/ipsec/ipsec-eap-tls.ci` -- functional test
- `test/ipsec/ipsec-nat-detection.ci` -- functional test
- `test/ipsec/ipsec-virtual-ip.ci` -- functional test

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

1. **Phase: Wiring (MANDATORY FIRST)** -- register EAP dispatch in IKE_AUTH, write failing wiring tests
   - Tests: `test/ipsec/ipsec-eap-mschapv2.ci`
   - Files: `eap/eap.go` (method interface + dispatch stub), engine extension (EAP state on IKE SA)
   - Verify: EAP dispatch reached during IKE_AUTH; wiring test fails because method returns error

2. **Phase: NAT Detection** -- NAT_DETECTION hash computation and port switching
   - Tests: `TestNATDetectionHash`, `TestNATDetectionPresent`, `TestNATDetectionAbsent`, `TestNonESPMarker`
   - Files: `transport/nat.go`, `transport/udp.go` (extend)
   - Verify: NAT correctly detected or not; port 4500 used when NAT present

3. **Phase: EAP-MSCHAPv2** -- challenge/response crypto, MSK derivation
   - Tests: `TestEAPMSCHAPv2Challenge`, `TestEAPMSCHAPv2AuthResponse`, `TestEAPMSCHAPv2MSK`, `TestEAPMSCHAPv2WrongPassword`
   - Files: `eap/eap_mschapv2.go`, `eap/mschapv2.go`
   - Verify: correct NT-Response, mutual auth, MSK matches expected

4. **Phase: EAP-TLS** -- TLS handshake in EAP, fragmentation, MSK
   - Tests: `TestEAPTLSHandshake`, `TestEAPTLSFragmentation`, `TestEAPTLSMSK`, `TestEAPTLSUntrustedCert`
   - Files: `eap/eap_tls.go`
   - Verify: TLS handshake completes over EAP transport, MSK extracted

5. **Phase: Virtual IP Pool** -- allocate/release, dual-stack, exhaustion
   - Tests: `TestVirtualIPPoolAllocate`, `TestVirtualIPPoolRelease`, `TestVirtualIPPoolExhausted`, `TestVirtualIPPoolDualStack`
   - Files: `eap/pool.go`
   - Verify: IPs allocated and released correctly; pool exhaustion handled

6. **Phase: Integration** -- AUTH from MSK, CP response, NAT keepalive, XFRM encap
   - Tests: `TestAuthFromMSK`, `TestNATKeepalive`
   - Files: engine EAP extension, `transport/keepalive.go`, dataplane encap extension
   - Verify: end-to-end EAP auth with MSK-based AUTH, virtual IP in CP, NAT keepalive running

7. **Functional tests** -- create after feature works
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-13 has implementation with file:line |
| Correctness | NtPasswordHash uses MD4 of UTF-16LE (not UTF-8); NAT hash uses SHA-1 (not SHA-256) |
| Naming | EAP method types use RFC numbers; NAT notify types use IANA values |
| Data flow | EAP runs inside IKE_AUTH exchange, not a separate protocol session |
| Rule: buffer-first | EAP and NAT payloads use buffer-first encoding pattern |
| Rule: exact-or-reject | Unknown EAP types rejected with EAP-NAK, not silently ignored |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| EAP framework exists | `grep -rn 'type Method interface' internal/component/ike/eap/eap.go` |
| EAP-MSCHAPv2 passes | `go test -run TestEAPMSCHAPv2 ./internal/component/ike/eap/` |
| EAP-TLS passes | `go test -run TestEAPTLS ./internal/component/ike/eap/` |
| Virtual IP pool works | `go test -run TestVirtualIPPool ./internal/component/ike/eap/` |
| NAT detection works | `go test -run TestNATDetection ./internal/component/ike/transport/` |
| NAT keepalive works | `go test -run TestNATKeepalive ./internal/component/ike/transport/` |
| AUTH from MSK works | `go test -run TestAuthFromMSK ./internal/component/ike/engine/` |
| Functional tests exist | `ls test/ipsec/ipsec-eap-*.ci test/ipsec/ipsec-nat-*.ci test/ipsec/ipsec-virtual-ip.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | EAP packets from untrusted peers: validate type, length, identifier before dispatch |
| Password handling | EAP passwords never logged; $9$ decoded only when needed for hash computation; zeroed after use |
| TLS configuration | EAP-TLS uses minimum TLS 1.2; strong cipher suites only; client certificate chain validated against PKI store CA |
| NAT hash | SHA-1 is used per RFC (not a crypto weakness here; it is a non-security hash for NAT detection) |
| Pool exhaustion | Pool full returns clear error; no DoS via rapid connect/disconnect (rate limiting in ipsec-7) |
| Timing attacks | NtPasswordHash comparison uses constant-time compare |
| Memory cleanup | MSK, NT hash, and TLS session keys zeroed after IKE_AUTH completes |

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

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: EAP exchange order in IKE_AUTH, MSK-to-AUTH derivation, NAT detection hash, NAT keepalive interval, virtual IP assignment via CP.

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
- [ ] AC-1..AC-13 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-ipsec-9-ikev2-eap-nat.md`
- [ ] Summary included in commit
