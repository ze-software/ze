# Spec: ipsec-rfc9190

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze implements RFC 9190 (EAP-TLS 1.3) without admitting to it, and no gate
watches the part it does implement.**

`exportEAPTLSMSK` (`internal/component/ike/eap/eap_tls.go`) already selects the
RFC 9190 label, the Type-Code context and the 128-octet export length when the
negotiated version is TLS 1.3. Interop scenario `06-eap-tls13` exercises exactly
that path against strongSwan and passes. But `rfc/enrolled.txt` has no row,
`docs/features/rfc-status.md` has no row, and nothing gates it.

`rfc/short/rfc9190.md` was written on 2026-08-01 (94 rows, 51 MUST-level) and
recorded `backlog` in `rfc/not-enrolled.txt`, because enrolment demands every
gated MUST proven in both polarities or annotated, and annotating is the
conformance judgement `ai/rules/rfc-compliance.md` reserves to the owner.

**Owner ruling, Thomas, 2026-08-01: implement the features, then enrol with
everything proven. Ze claims nothing it has not built.** The alternative,
enrolling now on owner-authorised annotations, was offered and declined.

The goal is that RFC 9190 is enrolled with no `{gap}` and no
`{not-applicable}` covering a feature Ze could have built.

### What is missing, grouped

| Group | Sections | State |
|-------|----------|-------|
| Protected success indication | 2.5 | NOT implemented. strongSwan's `eap_tls.c` `get_msk` checks for it |
| Session resumption and NewSessionTicket | 2.1.2, 2.1.3, 5.7 | absent |
| OCSP stapling and revocation | 5.4 (five MUSTs) | absent |
| Anonymous and privacy-friendly NAIs | 2.1.8, 5.8 | absent |
| Key derivation and the export | 2.3 | IMPLEMENTED, untagged |
| Not mechanically testable as written | 5.10-1 ("MUST mitigate known attacks") | needs an owner reading at enrolment |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9190.md` - the whole obligation set, already extracted
  → Constraint: Section 2.5's protected success indication is wire-visible and
    changes the state machine on BOTH roles.
- [ ] `rfc/short/rfc5216.md` if present - what RFC 9190 supersedes for TLS 1.3
  → Constraint: `exportEAPTLSMSK` must keep selecting by negotiated version. A
    TLS 1.2 peer still gets the RFC 5216 label.

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - Extraction Completeness, and the enrolment gates
  → Constraint: a new enrolment needs a hand-classified `rfc/extraction/rfc9190.json`
    sign-off. A generated skeleton can never pass, by design.

**Key insights:**
- The export path is live and proven by interop today. Everything else is absent.
- Enrolment is all-or-nothing against the 51 gated MUSTs, which is why this is a
  feature spec rather than a bookkeeping one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/eap/eap_tls.go` - `exportEAPTLSMSK` selects the RFC
  9190 label, context and length on TLS 1.3; `tlsMethod.Process` drives the
  authenticator; `tlsFragmenter` handles RFC 5216 Section 2.1.5 fragmentation.
- [ ] `internal/component/ike/eap/peer.go` - the peer side, `startTLSClient` and
  `readAndSendTLS`.
- [ ] `rfc/not-enrolled.txt` - carries the `backlog` row and its evidence.

**Behavior to preserve:**
- A TLS 1.2 peer keeps deriving the RFC 5216 MSK. Scenario `04-eap-tls` proves it
  against a stock strongSwan and must stay green.
- Scenario `06-eap-tls13` must stay green throughout.

**Behavior to change:**
- Add the protected success indication, resumption, OCSP stapling, and privacy
  NAIs. Then enrol.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- An EAP-TLS exchange inside IKE_AUTH, either role.
- Format at entry: EAP packets carrying TLS records, fragmented per RFC 5216
  Section 2.1.5.

### Transformation Path
1. `tlsFragmenter.reassemble` rebuilds the peer's flight.
2. `crypto/tls` drives the handshake over `eapTLSTransport`.
3. On completion, `exportEAPTLSMSK` derives the MSK by negotiated version.
4. The MSK feeds the IKEv2 AUTH payload.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EAP ↔ TLS engine | `eapTLSTransport`, a `net.Conn` over EAP payloads | Yes, scenarios 04 and 06 |
| EAP ↔ IKEv2 AUTH | the 64-octet MSK | Yes, both scenarios |
| Ze ↔ strongSwan | EAP-TLS on the wire | Yes, scenario 06 on TLS 1.3 |

### Integration Points
- `exportEAPTLSMSK` already branches on version and is the seam Section 2.3 owns.
- Section 2.5 needs a new step AFTER the handshake and BEFORE EAP-Success, on
  both roles.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Registration over hardcoding | N-A | no new command or family |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | strongSwan 5.9.14 implements the Section 2.5 protected success indication, so it can validate ours | its `eap_tls.c` `get_msk` checks for it | the interop proof needs a different peer, or a raw-socket harness | read `eap_tls.c`, then run scenario 06 with 2.5 on | unvalidated |
| A-2 | Adding 2.5 does not break the TLS 1.2 path | 2.5 is TLS 1.3 only | scenario 04 reddens | scenario 04 stays green at every step | unvalidated |
| A-3 | Resumption, OCSP and privacy NAIs are each independently landable | they touch different sections | the spec cannot be phased and must land at once | map each to its files during design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Section 2.5 is wire-visible, so a wrong implementation breaks a currently-green interop scenario | scenario 06 reddens | land 2.5 first and alone, with 04 and 06 run at every step |
| R-2 | Enrolment stays blocked because one MUST is untestable as written (5.10-1) | the extraction sign-off cannot classify it | that single row goes to Thomas with the RFC text, per `ai/rules/rfc-compliance.md` |
| R-3 | OCSP stapling pulls in a revocation-checking surface Ze has nowhere else | the change reaches outside `ike/eap` | scope it to EAP-TLS, and say so if it cannot be |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | EAP-TLS authentication, on both roles. Scenarios 04 and 06 are the canaries. |
| How is it reverted? | Per phase. Section 2.5 is separable from resumption and from OCSP. |
| Who else touches this path? | `plan/spec-rfcgate-1b-rfc7296-pilot.md` landed the transport and MSK fixes this builds on. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An EAP-TLS exchange completing on TLS 1.3 | → | the Section 2.5 indication sender | `TestEAPTLS13SendsProtectedSuccessIndication` |
| A peer's protected success indication | → | the receive path | `TestEAPTLS13RequiresProtectedSuccessIndication` |
| A resumption attempt | → | the NewSessionTicket path | `TestEAPTLS13ResumptionUsesATicket` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EAP-TLS completes on TLS 1.3, ze as authenticator | ze sends the protected success indication before EAP-Success |
| AC-2 | EAP-TLS completes on TLS 1.3, ze as peer | ze requires the indication and fails the exchange without it |
| AC-3 | A TLS 1.2 EAP-TLS exchange | no indication is sent, and the RFC 5216 MSK is derived exactly as today |
| AC-4 | A peer offers a session ticket | ze resumes, and the resumed session derives a correct MSK |
| AC-5 | A stapled OCSP response is present, and absent | ze honours Section 5.4 in both cases |
| AC-6 | An anonymous NAI | ze accepts it per Section 2.1.8 |
| AC-7 | `make ze-rfc-check` | RFC 9190 is enrolled, and no gated MUST carries `{gap}` or `{not-applicable}` for a feature this spec built |
| AC-8 | Scenarios 04 and 06 | both green at every phase boundary |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Authenticates a road-warrior with EAP-TLS over TLS 1.3 | IKE_AUTH → EAP-TLS → RFC 9190 MSK → IKEv2 AUTH | scenario `06-eap-tls13` |
| 2 | Reconnects and resumes rather than re-running the full handshake | ticket → resumed TLS → MSK | a new interop scenario |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLS13SendsProtectedSuccessIndication` | `internal/component/ike/eap/rfc9190_test.go` | AC-1 | |
| `TestEAPTLS13RequiresProtectedSuccessIndication` | same | AC-2 | |
| `TestEAPTLS12SendsNoIndication` | same | AC-3 | |
| `TestEAPTLS13ResumptionUsesATicket` | same | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| exported key material | 128 octets (RFC 9190 Section 2.3) | 128 | 127 | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls13-resumption` | `test/ipsec/ipsec-eap-tls13-resumption.ci` | a peer reconnects and resumes rather than re-handshaking. `option=env:var=ze_test_ike_dataplane:value=noop`, following `test/ipsec/ipsec-child-rekey.ci` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `06-eap-tls13` | `test/ipsec-interop/scenarios/` | strongSwan | the indication is accepted by a real peer | exists, must stay green |
| a resumption scenario | same | strongSwan | AC-4 against a real peer | |

## Files to Modify
- `internal/component/ike/eap/eap_tls.go` - the indication on the authenticator, and resumption.
- `internal/component/ike/eap/peer.go` - the indication on the peer.
- `rfc/enrolled.txt`, `rfc/not-enrolled.txt` - move the row at the end.
- `docs/features/rfc-status.md` - the public row.

## Files to Create
- `rfc/extraction/rfc9190.json` - the hand-classified sign-off enrolment requires.
- `internal/component/ike/eap/rfc9190_test.go`
- `test/ipsec/ipsec-eap-tls13-resumption.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | resumption and OCSP need operator control; read `ai/rules/config-surface.md` |
| YANG validation constraints | Yes | with the leaves above |
| CLI commands/flags | No | no new command |
| Functional test for new RPC/API | N-A | no new RPC |
| Doctor check | No | no new runtime dependency |
| Prometheus counters | Yes | a resumption hit/miss counter is worth having |

### Documentation Update Checklist (BLOCKING)
| Doc | Update? | File / reason |
|-----|---------|---------------|
| RFC compliance | Yes | `docs/features/rfc-status.md`, a new RFC 9190 row |
| Feature list | Yes | `docs/features.md`, EAP-TLS 1.3 |
| User guide | Yes | the new YANG leaves |

## Implementation Steps

1. Land Section 2.5 ALONE, both roles, with scenarios 04 and 06 run before and after.
2. Validate A-1 by reading strongSwan's `eap_tls.c` before assuming it can check ours.
3. Resumption and NewSessionTicket.
4. OCSP stapling and revocation.
5. Anonymous and privacy-friendly NAIs.
6. Write `rfc/extraction/rfc9190.json` by hand and run `make ze-rfc-check`.
7. Move the row from `rfc/not-enrolled.txt` to `rfc/enrolled.txt`, add the status row.
8. Raise 5.10-1 with Thomas if it still cannot be classified honestly.

## Goal Gates

- `make ze-verify` passes.
- `make ze-rfc-check` shows RFC 9190 enrolled, with no annotation covering a
  feature this spec built.
- Scenarios 04 and 06 green, plus the new resumption scenario.

## Quality Gates

- Every AC has a named test whose assertion states the AC's observable behavior.
- Every test is mutation-verified.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc9190.md` exists (94 rows, 51 MUST-level, protocol-only). This spec
does not rewrite it. It builds what the rows describe, then enrols.

## Known Limitations

Ze takes the RFC 9190 export path today and is not enrolled, so nothing gates it
until this spec closes. That is the state Thomas accepted when he chose to build
before claiming.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] A-1 validated against strongSwan's source before Section 2.5 lands
- [ ] Scenarios 04 and 06 green at every phase boundary
- [ ] `rfc/extraction/rfc9190.json` hand-classified
- [ ] `make ze-rfc-check` green with RFC 9190 enrolled
