# Spec: ipsec-11 -- IPsec Interop Testing: EAP Authentication

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-05-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-ipsec-0-umbrella.md` -- umbrella spec, child spec table
4. `plan/learned/744-ipsec-9-ikev2-eap-nat.md` -- EAP implementation decisions
5. `plan/learned/740-ipsec-7-ikev2-engine.md` -- engine architecture
6. `test/ipsec-interop/lab.py` -- Docker interop lab framework
7. `test/ipsec-interop/scenarios/01-psk-site-to-site/` -- reference scenario structure

## Task

Ze's IKEv2 EAP implementation (EAP-MSCHAPv2 and EAP-TLS) has unit test
coverage for the crypto primitives but has never been exercised in a real
IKE exchange against a third-party implementation. The existing Docker
interop lab validates PSK site-to-site only (scenarios 01 and 02). Without
end-to-end EAP interop tests, wire format bugs, EAP identifier sequencing
errors, MSK-derived AUTH computation mismatches, and Configuration Payload
encoding issues could all pass unit tests but fail against a real peer.

This is the only validation path available without a Windows machine. Windows
clients use EAP-MSCHAPv2 by default; strongSwan configured as a responder
requiring EAP-MSCHAPv2 is the closest available proxy.

**Scope:** Two new interop scenarios in the existing Docker lab (Ze as
initiator, strongSwan as responder), plus the engine changes needed to
support the EAP exchange flow in IKE_AUTH. This spec does NOT implement
the responder FSM (that is separate future work). It validates what
exists today: Ze's initiator-side EAP inside real IKE_AUTH exchanges.

**Why now:** The EAP code (ipsec-9) was implemented and unit-tested but
the interop lab (built during ipsec-7/8) was not extended to cover EAP.
The gap was discovered while investigating Windows compatibility.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/744-ipsec-9-ikev2-eap-nat.md` -- EAP implementation
  -> Decision: MD4 is custom (Go removed it), RFC 2759 magic constants exclude trailing null
  -> Constraint: EAP session stored on SA as `any` to avoid import cycle
- [ ] `plan/learned/740-ipsec-7-ikev2-engine.md` -- IKE engine architecture
  -> Decision: plugin registration via SDK, per-peer goroutine lifecycle
  -> Constraint: responder FSM is a stub (waits on stopCh), initiator-only exchanges
- [ ] `test/ipsec-interop/lab.py` -- Docker interop lab framework
  -> Constraint: scenarios follow ze.conf + swanctl.conf + check.py structure
  -> Constraint: lab builds ze-linux binary (CGO_ENABLED=0 GOOS=linux), builds Docker images, runs scenarios

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2 IKE_AUTH exchange with EAP (Section 2.16)
  -> Constraint: EAP in IKE_AUTH extends the exchange: responder sends EAP-Request in place of AUTH, initiator responds with EAP-Response, loop until EAP-Success, then both sides send AUTH using MSK-derived key
- [ ] `rfc/short/rfc3748.md` -- EAP framework: Identity, Request/Response, Success/Failure
  -> Constraint: EAP identifier must be echoed in responses; MSK must be exported for AUTH
- [ ] `rfc/short/rfc2759.md` -- MS-CHAPv2: Challenge/Response/Success flow
  -> Constraint: magic constants exclude trailing null byte (gotcha from ipsec-9)

**Key insights:**
- Ze is initiator-only; strongSwan must be configured as EAP authenticator (responder)
- strongSwan supports `eap-mschapv2` and `eap-tls` plugins natively on Alpine
- The Docker lab framework handles container lifecycle, network setup, and assertions
- EAP-MSCHAPv2 requires strongSwan to have `eap-mschapv2` plugin loaded and a secrets block with the EAP identity/password
- EAP-TLS requires X.509 certificates on both sides: Ze presents a client cert, strongSwan validates against its CA
- Both EAP methods require strongSwan to authenticate with a server certificate first (pubkey auth local, eap-* auth remote)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/ipsec-interop/run.py` -- lab runner: builds images, iterates `scenarios/`, runs check.py per scenario
  -> Constraint: scenario selection by name, NO_BUILD env var for skipping rebuilds
- [ ] `test/ipsec-interop/lab.py` -- Docker helpers: container lifecycle, StrongSwan/FRR classes, XFRM helpers
  -> Constraint: StrongSwan class has wait_sa_established(), has_child_sa(), xfrm_state() methods
  -> Constraint: Scenario class handles setup()/teardown()/run_check()/dump_logs()
- [ ] `test/ipsec-interop/scenarios/01-psk-site-to-site/` -- reference scenario
  -> Constraint: ze.conf uses vpn ipsec site-to-site with mode pre-shared-secret
  -> Constraint: swanctl.conf uses `connections {}` with `local { auth = psk }`
  -> Constraint: check.py validates SA established, Child SA installed, XFRM state, ESP traffic flow
- [ ] `test/ipsec-interop/Dockerfile.strongswan` -- Alpine 3.21, strongswan package, charon entrypoint
  -> Constraint: stock Alpine strongswan package includes eap-mschapv2 and eap-tls plugins
- [ ] `test/ipsec-interop/Dockerfile.ze` -- Alpine 3.21, copies ze-linux binary, tini entrypoint
- [ ] `internal/component/ike/engine/fsm.go` -- initiator FSM drives IKE_SA_INIT + IKE_AUTH
  -> Constraint: `handleAuthResponse` (line 341) expects AUTH in the response and transitions to StateEstablished. Does not handle EAP payload in response
  -> Constraint: `handleInbound` switch (line 181) handles StateAuthSent for IKE_AUTH responses only
- [ ] `internal/component/ike/engine/auth.go` -- `computeLocalAuth` (line 142) routes EAP modes to `computeEAPAuth`
  -> Constraint: `buildAuthRequest` (line 77) always includes AUTH payload immediately; EAP initiator should send IDi without AUTH first
- [ ] `internal/component/ike/engine/eap_auth.go` -- `ComputeAuthFromMSK` (line 19), `NewEAPSession` (line 74)
  -> Constraint: MSK-derived AUTH uses prf(prf(MSK, "Key Pad for IKEv2"), signed_octets)
- [ ] `internal/component/ike/engine/sa.go` -- SAState enum (line 18): StateIdle through StateDead. No EAP-in-progress state
- [ ] `internal/component/ike/eap/eap.go` -- EAP session dispatcher, Identity exchange
- [ ] `internal/component/ike/eap/eap_mschapv2.go` -- MS-CHAPv2 method handler
- [ ] `internal/component/ike/eap/eap_tls.go` -- EAP-TLS method handler with custom TLS transport
- [ ] `internal/component/ipsec/types.go` -- AuthEAPTLS, AuthEAPMSCHAPv2 enum values; RemoteAccessConfig with EAP users and virtual IP pool

**Behavior to preserve:**
- Existing PSK scenarios (01, 02) must continue to pass
- Lab framework API (Scenario, StrongSwan, FRR classes) must remain backward-compatible
- IKE engine initiator PSK and X.509 paths unaffected
- Docker image build process unchanged

**Behavior to change:**
- IKE engine initiator IKE_AUTH must support EAP exchange flow (multi-round-trip) when auth mode is EAP
- New scenario `03-eap-mschapv2` added to interop lab
- New scenario `04-eap-tls` added to interop lab
- Dockerfile.strongswan may need additional packages for EAP-TLS certificate mounting
- A test PKI (CA cert, server cert, client cert) must be generated for EAP-TLS scenario

## Data Flow (MANDATORY)

### Entry Point
- Config load: Ze reads ze.conf with EAP authentication mode
- IKE engine: initiates IKE_SA_INIT, then IKE_AUTH with EAP
- strongSwan: configured as EAP authenticator, challenges Ze with EAP-Request

### Transformation Path (EAP-MSCHAPv2)
1. Ze sends IKE_SA_INIT request (same as PSK)
2. strongSwan responds with IKE_SA_INIT response (same as PSK)
3. Ze sends IKE_AUTH request: HDR, SK {IDi, SAi2, TSi, TSr} -- NO AUTH payload
4. strongSwan responds: HDR, SK {IDr, CERT, AUTH, EAP-Request/Identity}
5. Ze verifies server AUTH, creates EAP session
6. Ze sends: HDR, SK {EAP-Response/Identity(username)}
7. strongSwan sends: HDR, SK {EAP-Request/MSCHAPv2-Challenge}
8. Ze sends: HDR, SK {EAP-Response/MSCHAPv2-Response}
9. strongSwan sends: HDR, SK {EAP-Request/MSCHAPv2-Success}
10. Ze sends: HDR, SK {EAP-Response/MSCHAPv2-Ack}
11. strongSwan sends: HDR, SK {EAP-Success}
12. Ze sends: HDR, SK {AUTH} -- AUTH derived from MSK
13. strongSwan sends: HDR, SK {AUTH, SAr2, TSi, TSr}
14. Child SA established, XFRM SAs installed

### Transformation Path (EAP-TLS)
1. Steps 1-2 same as above
2. Ze sends IKE_AUTH request: HDR, SK {IDi, SAi2, TSi, TSr} -- NO AUTH
3. strongSwan responds: HDR, SK {IDr, CERT, AUTH, EAP-Request/Identity}
4. Ze verifies server AUTH, creates EAP-TLS session
5. Ze sends: HDR, SK {EAP-Response/Identity}
6. strongSwan sends: HDR, SK {EAP-Request/EAP-TLS(ServerHello)}
7. Ze sends: HDR, SK {EAP-Response/EAP-TLS(ClientHello, client cert)}
8. Multiple EAP-Request/EAP-Response rounds for TLS handshake (fragmented)
9. strongSwan sends: HDR, SK {EAP-Success}
10. Ze sends: HDR, SK {AUTH} -- AUTH derived from TLS MSK via ExportKeyingMaterial
11. strongSwan sends: HDR, SK {AUTH, SAr2, TSi, TSr}
12. Child SA established

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze IKE engine to strongSwan | UDP port 500 over Docker bridge network | [ ] |
| Ze EAP session to IKE engine | EAP payloads extracted from decrypted SK, dispatched to eap.Session | [ ] |
| strongSwan VICI to test harness | `swanctl --list-sas` queries charon state | [ ] |
| Ze kernel to XFRM | `ip xfrm state` shows installed SAs (when XFRM available) | [ ] |

### Integration Points
- `internal/component/ike/engine/fsm.go` -- `handleAuthResponse` (line 341) must recognize EAP payload and enter multi-round EAP loop
- `internal/component/ike/engine/auth.go` -- `buildAuthRequest` (line 77) must omit AUTH for EAP initiator (first request)
- `internal/component/ike/engine/eap_auth.go` -- `NewEAPSession` (line 74) creates method handler; `ComputeAuthFromMSK` (line 19) computes final AUTH
- `internal/component/ike/eap/eap.go` -- Session.Process handles EAP Request/Response dispatch
- `test/ipsec-interop/lab.py` -- StrongSwan class used for SA verification

### Architectural Verification
- [ ] No bypassed layers (EAP payloads flow through standard SK decrypt path)
- [ ] No unintended coupling (test scenarios use only public lab.py API)
- [ ] No duplicated functionality (extends existing interop lab, does not create parallel infrastructure)
- [ ] Zero-copy preserved where applicable (EAP payloads parsed from existing wire codec)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Ze config with `authentication { mode eap-mschapv2 }`, strongSwan as EAP responder | -> | IKE engine EAP exchange in IKE_AUTH, MSK-derived AUTH accepted by strongSwan | `test/ipsec-interop/scenarios/03-eap-mschapv2/check.py` |
| Ze config with `authentication { mode eap-tls }`, strongSwan as EAP responder with server cert | -> | IKE engine EAP-TLS exchange, TLS handshake fragmented in EAP, MSK exported, AUTH accepted | `test/ipsec-interop/scenarios/04-eap-tls/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze configured with EAP-MSCHAPv2, strongSwan as EAP authenticator | IKE SA established between Ze and strongSwan. `swanctl --list-sas` shows ESTABLISHED |
| AC-2 | AC-1 conditions | Child SA installed. `swanctl --list-sas` shows INSTALLED child |
| AC-3 | AC-1 conditions, XFRM available | ESP SAs present in both containers (`ip xfrm state` shows `proto esp`) |
| AC-4 | AC-1 conditions, XFRM available | Traffic flows through ESP tunnel (XFRM byte counters increase after ping) |
| AC-5 | Ze configured with EAP-TLS, test PKI (CA, server cert, client cert) | IKE SA established. Certificate chain validated by both sides |
| AC-6 | AC-5 conditions | Child SA installed with XFRM SAs |
| AC-7 | Wrong EAP-MSCHAPv2 password in ze.conf | IKE_AUTH fails. No SA established. Ze logs authentication failure |
| AC-8 | Existing PSK scenarios (01, 02) after code changes | Both scenarios still pass with no modifications |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildAuthRequestEAP` | `internal/component/ike/engine/auth_test.go` | `buildAuthRequest` omits AUTH payload when auth mode is EAP (first IKE_AUTH) | |
| `TestHandleAuthResponseEAP` | `internal/component/ike/engine/fsm_test.go` | `handleAuthResponse` recognizes EAP payload in decrypted inner payloads and transitions to StateEAPInProgress | |
| `TestEAPExchangeLoop` | `internal/component/ike/engine/fsm_test.go` | Multi-round EAP exchange: Identity, Method, Success transitions produce MSK and transition to AUTH | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `03-eap-mschapv2` | `test/ipsec-interop/scenarios/03-eap-mschapv2/check.py` | Ze initiates IKE with EAP-MSCHAPv2 to strongSwan, SA established, ESP traffic verified | |
| `04-eap-tls` | `test/ipsec-interop/scenarios/04-eap-tls/check.py` | Ze initiates IKE with EAP-TLS to strongSwan (test PKI), SA established | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EAP identifier | 0-255 | 255 | N/A (wraps) | N/A (wraps) |
| EAP round count | 1-20 | 20 | N/A | 21 (abort exchange) |

### Future (if deferring any tests)
- Packet capture validation (EAP payload encoding, identifier sequencing): useful for debugging but not blocking for interop validation

## Files to Modify

- `internal/component/ike/engine/sa.go` -- add `StateEAPInProgress` to SAState enum
- `internal/component/ike/engine/fsm.go` -- `handleAuthResponse`: detect EAP payload, enter EAP loop; `handleInbound`: add StateEAPInProgress case; new `handleEAPResponse` function
- `internal/component/ike/engine/auth.go` -- `buildAuthRequest`: omit AUTH when auth mode is EAP; new `buildEAPOnlyAuthRequest` for post-EAP AUTH message

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7296.md` -- note EAP exchange in IKE_AUTH (Section 2.16) is now implemented for initiator |
| 10 | Test infrastructure changed? | Yes | Add scenario descriptions to interop lab docs (if README exists) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `test/ipsec-interop/scenarios/03-eap-mschapv2/ze.conf` -- Ze config with EAP-MSCHAPv2 auth
- `test/ipsec-interop/scenarios/03-eap-mschapv2/swanctl.conf` -- strongSwan as EAP-MSCHAPv2 authenticator
- `test/ipsec-interop/scenarios/03-eap-mschapv2/check.py` -- validation: SA established, Child SA, XFRM, traffic
- `test/ipsec-interop/scenarios/04-eap-tls/ze.conf` -- Ze config with EAP-TLS auth and client cert reference
- `test/ipsec-interop/scenarios/04-eap-tls/swanctl.conf` -- strongSwan as EAP-TLS authenticator with server cert
- `test/ipsec-interop/scenarios/04-eap-tls/check.py` -- validation: SA established, Child SA, cert chain
- `test/ipsec-interop/scenarios/04-eap-tls/gen-pki.sh` -- script to generate test PKI (self-signed CA, server cert, client cert)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- implement EAP exchange in IKE_AUTH, write failing interop test |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` + interop lab run |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- IKE_AUTH EAP exchange in engine
   - Tests: `TestBuildAuthRequestEAP`, `TestHandleAuthResponseEAP`, `TestEAPExchangeLoop`
   - Files: `internal/component/ike/engine/sa.go`, `fsm.go`, `auth.go`
   - Changes:
     - Add `StateEAPInProgress` to SAState enum (sa.go)
     - `buildAuthRequest` (auth.go line 77): when auth mode is EAP, build IDi + SAi2 + TSi + TSr WITHOUT AUTH payload. RFC 7296 Section 2.16: initiator omits AUTH to signal EAP willingness
     - `handleAuthResponse` (fsm.go line 341): detect EAP payload in decrypted inner payloads. If EAP present: verify responder AUTH (server auth), extract EAP-Request, create EAP session via `NewEAPSession`, process first EAP request, build EAP-Response, send encrypted, transition to StateEAPInProgress
     - New `handleEAPResponse` function (fsm.go): for StateEAPInProgress, decrypt SK payload, extract EAP payload, dispatch to `eap.Session.Process`. On EAP-Success: extract MSK, compute AUTH via `ComputeAuthFromMSK`, send IKE_AUTH with AUTH only, transition to StateAuthSent. On EAP-Request: send EAP-Response, stay in StateEAPInProgress
     - `handleInbound` switch (fsm.go line 181): add `case StateEAPInProgress` for IKE_AUTH responses
   - Verify: unit tests pass; PSK scenarios 01/02 unaffected

2. **Phase: EAP-MSCHAPv2 Interop Scenario** -- scenario 03
   - Tests: `test/ipsec-interop/scenarios/03-eap-mschapv2/check.py`
   - Files: ze.conf, swanctl.conf, check.py for scenario 03
   - Config details:
     - Ze ze.conf: `authentication { mode eap-mschapv2; }` with eap-identity and eap-password. Server certificate validation requires Ze to trust strongSwan's server cert (CA in PKI config)
     - strongSwan swanctl.conf: `local { auth = pubkey; certs = server.pem }`, `remote { auth = eap-mschapv2; eap_id = %any }`, `secrets { eap-user { id = testuser; secret = testpass } }`
     - Both EAP scenarios require a server certificate for strongSwan. Share the same test PKI generation between scenarios 03 and 04, or generate per-scenario
     - IKE proposal: AES-256-GCM + SHA-256 + DH-14
   - Check: SA established, Child SA installed, XFRM state (if available), ESP byte counters
   - Verify: `python3 test/ipsec-interop/run.py 03-eap-mschapv2` passes

3. **Phase: Test PKI Generation** -- certificates for EAP scenarios
   - Tests: gen-pki.sh produces valid certificate chain
   - Files: `test/ipsec-interop/pki/gen-pki.sh` (shared across scenarios)
   - Details:
     - Self-signed CA (ECDSA P-256, CN=ze-interop-ca)
     - Server certificate for strongSwan (CN=172.28.0.3 or SAN=IP:172.28.0.3)
     - Client certificate for Ze EAP-TLS (CN=ze-test-client)
     - All with 10-year validity (test-only, never deployed)
     - Output: `pki/ca.pem`, `pki/server.pem`, `pki/server-key.pem`, `pki/client.pem`, `pki/client-key.pem`
     - Generated at build time by run.py or by individual check.py setup
   - Verify: `openssl verify -CAfile pki/ca.pem pki/server.pem` succeeds

4. **Phase: EAP-TLS Interop Scenario** -- scenario 04
   - Tests: `test/ipsec-interop/scenarios/04-eap-tls/check.py`
   - Files: ze.conf, swanctl.conf, check.py for scenario 04
   - Config details:
     - Ze ze.conf: `authentication { mode eap-tls; certificate ze-client; ca-certificate test-ca; }` with PKI section loading test client cert and CA
     - strongSwan swanctl.conf: `local { auth = pubkey; certs = server.pem }`, `remote { auth = eap-tls }` with CA trust store containing test CA
     - Certificates mounted into containers via Docker volumes
   - Check: SA established, Child SA installed, certificate chain validated
   - Verify: `python3 test/ipsec-interop/run.py 04-eap-tls` passes

5. **Phase: Regression Verification** -- confirm existing scenarios unbroken
   - Tests: scenarios 01 and 02
   - Verify: `python3 test/ipsec-interop/run.py` (all scenarios) passes

6. **Functional tests** -> Run full interop lab
7. **RFC refs** -> Add `// RFC 7296 Section 2.16` comments to EAP exchange code
8. **Full verification** -> `make ze-verify` (lint + tests), interop lab
9. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-8 all have implementation evidence |
| Correctness | EAP exchange follows RFC 7296 Section 2.16 exactly: no AUTH in first IKE_AUTH from initiator, AUTH after EAP-Success derived from MSK |
| Correctness | strongSwan swanctl.conf uses correct syntax for EAP methods and secrets |
| Correctness | Server AUTH verified before processing EAP requests (prevents rogue server credential harvesting) |
| Naming | Scenario directories follow `NN-description` pattern consistent with 01/02 |
| Data flow | EAP payloads extracted from decrypted SK, dispatched to eap.Session, responses re-encrypted in SK |
| Rule: buffer-first | EAP response payloads encoded via existing wire codec WriteTo pattern |
| Regression | PSK scenarios 01 and 02 pass after engine changes |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| StateEAPInProgress in SA state machine | `grep -n StateEAPInProgress internal/component/ike/engine/sa.go` |
| buildAuthRequest omits AUTH for EAP | Unit test `TestBuildAuthRequestEAP` passes |
| handleAuthResponse processes EAP payloads | Unit test `TestHandleAuthResponseEAP` passes |
| EAP exchange loop works | Unit test `TestEAPExchangeLoop` passes |
| Scenario 03 directory exists | `ls test/ipsec-interop/scenarios/03-eap-mschapv2/` |
| Scenario 03 passes | `python3 test/ipsec-interop/run.py 03-eap-mschapv2` exit 0 |
| Scenario 04 directory exists | `ls test/ipsec-interop/scenarios/04-eap-tls/` |
| Scenario 04 passes | `python3 test/ipsec-interop/run.py 04-eap-tls` exit 0 |
| Scenarios 01-02 still pass | `python3 test/ipsec-interop/run.py` exit 0 |
| Test PKI generation script | `ls test/ipsec-interop/pki/gen-pki.sh` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | EAP payloads from strongSwan must be length-validated before processing |
| Credential handling | Test EAP passwords in ze.conf are test-only strings, not real credentials |
| Certificate handling | EAP-TLS test certs are self-signed with 10-year validity; never used outside test lab. Private keys committed only for test PKI |
| Timing attacks | MSK-derived AUTH comparison uses `subtle.ConstantTimeCompare` (existing pattern in auth.go) |
| Resource exhaustion | EAP exchange loop has a maximum round limit (prevent infinite loops from malicious responder) |
| Server auth first | Initiator verifies responder AUTH before processing any EAP requests (RFC 7296 Section 2.16 requirement) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| strongSwan rejects EAP | Check swanctl.conf syntax, verify EAP plugin loaded via `docker exec ... ls /usr/lib/strongswan/plugins/` |
| Ze AUTH rejected by strongSwan | Verify MSK derivation, check AUTH computation matches RFC 7296 Section 2.16 |
| EAP-TLS handshake fails | Check certificate chain, verify TLS version compatibility, check fragment reassembly |
| Docker networking issue | Check container IPs, verify network creation in lab.py |
| XFRM not available (Docker for Mac) | Skip XFRM assertions gracefully (existing pattern in scenario 02 check.py) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Insights

### IKE_AUTH EAP Exchange (RFC 7296 Section 2.16)

The key difference from PSK/X.509 IKE_AUTH:

**Standard (PSK/X.509) -- 2 messages:**
- Initiator: HDR, SK {IDi, [CERT], AUTH, SAi2, TSi, TSr}
- Responder: HDR, SK {IDr, [CERT], AUTH, SAr2, TSi, TSr}

**EAP -- 6+ messages:**
- Initiator: HDR, SK {IDi, [CERTREQ], SAi2, TSi, TSr} -- NO AUTH
- Responder: HDR, SK {IDr, [CERT], AUTH, EAP(Request)} -- server authenticates first
- Initiator: HDR, SK {EAP(Response)}
- ... (EAP method-specific rounds) ...
- Responder: HDR, SK {EAP(Success)}
- Initiator: HDR, SK {AUTH} -- AUTH derived from MSK
- Responder: HDR, SK {AUTH, SAr2, TSi, TSr} -- final, with Child SA

The initiator MUST verify the responder's AUTH before processing EAP requests
(prevents rogue server from harvesting EAP credentials). The initiator's AUTH
after EAP-Success uses `prf(prf(MSK, "Key Pad for IKEv2"), signed_octets)`
instead of the PSK or certificate-based computation.

### Current Engine Gap

`buildAuthRequest` (auth.go line 77) always includes AUTH payload. For EAP, the
first IKE_AUTH MUST NOT include AUTH (omitting it signals EAP willingness).

`handleAuthResponse` (fsm.go line 341) expects AUTH in the response and transitions
directly to StateEstablished. For EAP, the first response includes AUTH (server auth)
plus an EAP-Request payload. The initiator must verify the server AUTH, then enter
a multi-round EAP exchange loop before sending its own AUTH.

The `handleInbound` switch (fsm.go line 181) only handles StateAuthSent for IKE_AUTH
responses. A new StateEAPInProgress state is needed for the multi-round EAP exchange.

### strongSwan EAP Configuration

Both EAP methods require strongSwan to authenticate with a server certificate to the
initiator (the responder does pubkey auth locally). This means even the MSCHAPv2
scenario needs a test PKI:
- `local { auth = pubkey }` with server cert in `/etc/swanctl/x509/`
- `remote { auth = eap-mschapv2 }` or `remote { auth = eap-tls }`
- The CA cert goes in `/etc/swanctl/x509ca/` on both sides

The test PKI can be shared between scenarios 03 and 04 to avoid duplication.

### Message ID Handling in EAP Exchange

RFC 7296 Section 2.2: each IKE_AUTH request/response pair shares a message ID.
The first IKE_AUTH uses message ID 1. Each subsequent EAP round-trip increments
the message ID. The initiator must track the message ID counter correctly across
the EAP exchange to avoid retransmission confusion.

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

## RFC Documentation

Add `// RFC 7296 Section 2.16` above EAP exchange handling code.
MUST document: omit AUTH in first EAP IKE_AUTH request, verify server AUTH before EAP processing, MSK-derived AUTH after EAP-Success, EAP round limit.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

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
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-11-interop-eap.md`
- [ ] Summary included in commit
