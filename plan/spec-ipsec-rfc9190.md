# Spec: ipsec-rfc9190

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## OWNER RULING 2026-09-05: §2.1.2 binds Ze, so AC-4 stands

Put to him as the one genuine ambiguity an RFC audit found, and answered "yes
binding". RFC 9190 §2.1.2: *"To enable resumption when using EAP-TLS with TLS
1.3, the EAP-TLS server MUST send one or more post-handshake NewSessionTicket
messages (each associated with a PSK, a PSK identity, a ticket lifetime, and
other parameters) in the initial authentication."* The purpose clause is NOT an
antecedent that a server escapes by declining resumption: §2.1.3 makes only
ACCEPTING resumption optional (*"It is up to the EAP-TLS peer to use
resumption"*, and *"the EAP-TLS server MAY choose to require a full
handshake"*), and no sentence anywhere makes ISSUANCE optional.

Two consequences the implementation must carry:

- `SessionTicketsDisabled: true` in `newTLSMethod` (`internal/core/eap/eap_tls.go`,
  and the peer side in `peer.go`) is measured against a MUST. The comment calling
  that decision deliberate is VOID as authority (`ai/rules/rfc-compliance.md`:
  every earlier answer pointing away from full compliance is void and must be
  re-raised, not cited).
- `TestEAPTLSIssuesNoUnredeemableSessionTicket` pins that flag, and its stated
  purpose is to keep AC-4's six §5.6/§5.7 MUSTs "provably dead until resumption
  is built". Under this ruling it pins a non-conformance, so it is rewritten when
  resumption lands rather than defended.

RFC 9190 §5.4 is not affected by this ruling and was never ambiguous: three
unconditional MUSTs on revocation and OCSP stapling, which no layer performed
when this ruling was written. They were gaps, never exclusions, and phases 4 and
6 built all five.

## Task

**Ze implements RFC 9190 (EAP-TLS 1.3) without admitting to it, and no gate
watches the part it does implement.**

`exportEAPTLSMSK` (`internal/core/eap/eap_tls.go`) already selects the
RFC 9190 label, the Type-Code context and the 128-octet export length when the
negotiated version is TLS 1.3. Interop scenario `eap-tls13` exercises exactly
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
| Protected success indication | 2.5 | SERVER side implemented 2026-08-12 (phase 1). The peer side ANSWERS it but does not consume it, which the published RFC does not require |
| Session resumption and NewSessionTicket | 2.1.2, 2.1.3, 5.7 | IMPLEMENTED 2026-09-08 (phase 5), both roles, with the §5.4 revocation check carried across a resumed handshake. No interop counterpart exists; see the Interop Tests table |
| OCSP stapling and revocation | 5.4 (five MUSTs) | IMPLEMENTED. 5.4-1 landed 2026-09-05 (phase 4), and 5.4-2 through 5.4-5 landed 2026-09-08 (phase 6): the authenticator staples the operator's `ocsp-response`, the peer enforces the stapled status under `certificate-status-request`, and it re-checks the chain over https once the Child SA is up. That last one also closed RFC5216-5.4-2, which was a published gap on an enrolled RFC |
| Anonymous and privacy-friendly NAIs | 2.1.8, 5.8 | IMPLEMENTED 2026-09-08 (phase 5b), peer and server. The peer derives an anonymous NAI in `NewPeerSessionTLS` rather than at the send site, so no caller reaches the Identity Response with the configured `local-id` in it. Section 5.8's three MUSTs open "If anonymous NAIs are not used", and ze now uses one on every EAP-TLS exchange, so their antecedent is false: they are exclusions for the extraction, never gaps |
| Key derivation and the export | 2.3 | IMPLEMENTED, untagged |
| Known attack mitigation | 5.10-1 ("MUST mitigate known attacks") | IMPLEMENTED and PROVEN 2026-09-08. The requirement points at RFC 7457 and BCP 195, so the list is the test: `internal/core/eap/rfc9190_attack_mitigation_test.go` carries sixteen tagged units over the nine RFC 7457 Section 2 attacks an EAP-TLS implementation can hold a property against, both polarities, each with a discrimination record. Owner ruling, Thomas, 2026-09-08: "in that case we need to ensure we run these nine test in our suite (positive and negative)" |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9190.md` - the whole obligation set, already extracted
  → Constraint: Section 2.5's protected success indication is wire-visible and
    changes the state machine on BOTH roles.
- [ ] `rfc/short/rfc5216.md` if present - what RFC 9190 supersedes for TLS 1.3
  → Constraint: `exportEAPTLSMSK` must keep selecting by negotiated version. A
    TLS 1.2 peer still gets the RFC 5216 label.

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-11-interop-eap.md` - EAP-MSCHAPv2 and EAP-TLS driven from the initiator seat
  → Constraint: it is the design page of `internal/core/eap/revocation.go`, so the
    Section 5.4 chain walk and its two-callback shape are stated there.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - EAP authentication and NAT traversal, for site-to-site and road-warrior peers
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the IKE responder EAP authenticator, whose `eapTLSServerConfig` hands the CRLs to the EAP method
- [ ] `docs/architecture/pki/pki-store.md` - the CA store the `crl` leaf-list lives in
  → Constraint: a CA with no list and a list revoking nothing are different
    answers, and `CACertEntry.CRLPEM` keeps them apart by answering nil for the
    first.
- [ ] `docs/architecture/testing/interop.md` - how a scenario is discovered, named and checked
- [ ] `ai/rules/rfc-compliance.md` - Extraction Completeness, and the enrolment gates
  → Constraint: a new enrolment needs a hand-classified `rfc/extraction/rfc9190.json`
    sign-off. A generated skeleton can never pass, by design.

**Key insights:**
- The export path is live and proven by interop today. Everything else is absent.
- Enrolment is all-or-nothing against the 51 gated MUSTs, which is why this is a
  feature spec rather than a bookkeeping one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/eap/eap_tls.go` - `exportEAPTLSMSK` selects the RFC
  9190 label, context and length on TLS 1.3; `tlsMethod.Process` drives the
  authenticator; `tlsFragmenter` handles RFC 5216 Section 2.1.5 fragmentation.
- [ ] `internal/core/eap/peer.go` - the peer side, `startTLSClient` and
  `readAndSendTLS`.
- [ ] `rfc/not-enrolled.txt` - carries the `backlog` row and its evidence.

**Behavior to preserve:**
- A TLS 1.2 peer keeps deriving the RFC 5216 MSK. Scenario `eap-tls` proves it
  against a stock strongSwan and must stay green.
- Scenario `eap-tls13` must stay green throughout.

**Behavior to change:**
- Add the protected success indication, resumption, OCSP stapling, and privacy
  NAIs. Then enrol.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

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
| EAP ↔ TLS engine | `eapTLSTransport`, a `net.Conn` over EAP payloads | Yes, scenarios eap-tls and eap-tls13 |
| EAP ↔ IKEv2 AUTH | the 64-octet MSK | Yes, both scenarios |
| Ze ↔ strongSwan | EAP-TLS on the wire | Yes, scenario eap-tls13 on TLS 1.3 |

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
| A-1 | strongSwan 5.9.14 implements the Section 2.5 protected success indication, so it can validate ours | its `eap_tls.c` `get_msk` checks for it | the interop proof needs a different peer, or a raw-socket harness | read `eap_tls.c`, then run scenario eap-tls13 with 2.5 on | confirmed 2026-08-12. `get_msk` returns FAILED and logs `missing protected success indication for EAP-TLS with TLS 1.3` when `get_version_max() >= TLS_1_3 && !indication_sent_received`; `client_process` requires exactly one octet equal to 0. MEASURED both ways in scenario responder-eap-tls13 |
| A-2 | Adding 2.5 does not break the TLS 1.2 path | 2.5 is TLS 1.3 only | scenario eap-tls reddens | scenario eap-tls stays green at every step | confirmed 2026-08-12. Scenarios eap-tls and eap-tls13 both green after the change, and `TestEAPTLS12SendsNoProtectedSuccessIndication` pins it in unit form |
| A-3 | Resumption, OCSP and privacy NAIs are each independently landable | they touch different sections | the spec cannot be phased and must land at once | map each to its files during design | confirmed for revocation 2026-09-05. RFC9190-5.4-1 landed on its own, with resumption untouched: `checkChainRevocation` (`internal/core/eap/revocation.go`) is reached from each role's `tls.Config.VerifyConnection` and shares no code with the ticket path. Not yet shown for resumption or for privacy NAIs |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Section 2.5 is wire-visible, so a wrong implementation breaks a currently-green interop scenario | scenario eap-tls13 reddens | land 2.5 first and alone, with eap-tls and eap-tls13 run at every step |
| R-2 | RETIRED 2026-09-08. 5.10-1 is not untestable: RFC 9190 Section 5.10 cites RFC 7457 and BCP 195, and those documents supply the attack list a test can be written against | -- | sixteen tagged units in `internal/core/eap/rfc9190_attack_mitigation_test.go`, nine attacks, both polarities, every one with a discrimination record |
| R-3 | OCSP stapling pulls in a revocation-checking surface Ze has nowhere else | the change reaches outside `ike/eap` | scope it to EAP-TLS, and say so if it cannot be |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | EAP-TLS authentication, on both roles. Scenarios eap-tls and eap-tls13 are the canaries. |
| How is it reverted? | Per phase. Section 2.5 is separable from resumption and from OCSP. |
| Who else touches this path? | the rfcgate-1b RFC 7296 pilot spec landed the transport and MSK fixes this builds on. |

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
| AC-5 | A stapled OCSP response is present, and absent | ze honours Section 5.4 in both cases. Done phase 6: the authenticator staples what the operator configured, the peer refuses an entry with no valid status once certificate-status-request is set, and it re-checks the chain over https once the Child SA is up |
| AC-6 | An anonymous NAI | ze accepts it per Section 2.1.8 |
| AC-7 | `./le rfc check` | RFC 9190 is enrolled, and no gated MUST carries `{gap}` or `{not-applicable}` for a feature this spec built |
| AC-8 | Scenarios eap-tls and eap-tls13 | both green at every phase boundary. **GREEN again on 2026-09-08**, with responder-eap-tls13 and responder-eap-tls13-revoked-client green in the same tree. The red between phase 5b and here was charon, not ze: `process_cert_verify` (strongSwan 5.9.14 `src/libtls/tls_server.c`) looks the peer certificate up by the EAP identity, which `load_method` (`src/libcharon/sa/ikev2/authenticators/eap_authenticator.c`) takes from the wire whenever `eap_id` is `%any`, so the anonymous NAI RFC 9190 Section 2.1.8 requires can never name a certificate. Each scenario's `swanctl.conf` now names `eap_id = ze-test-client`, which keys that lookup on the configuration. `anonymousNAI` is unchanged. Phase 5, 2026-09-08: eap-tls, eap-tls13 AND responder-eap-tls13 recorded green after resumption landed, which is the evidence that issuing a ticket is invisible to a peer that never offers `psk_dhe_ke` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Authenticates a road-warrior with EAP-TLS over TLS 1.3 | IKE_AUTH → EAP-TLS → RFC 9190 MSK → IKEv2 AUTH | scenario `eap-tls13` |
| 2 | Reconnects and resumes rather than re-running the full handshake | ticket → resumed TLS → MSK | `test/ipsec/ipsec-eap-tls13-resumption.ci`, two ze daemons across a `clear vpn ipsec sa` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLS13SendsProtectedSuccessIndication` | `internal/core/eap/rfc9190_test.go` | AC-1 | done, phase 1 |
| `TestEAPTLS13RefusedClientGetsNoSuccessIndication` | same | AC-1 negative (RFC9190-2.5-2) | done, phase 1 |
| `TestEAPTLS13RequiresProtectedSuccessIndication` | `internal/core/eap/rfc9190_peer_indication_test.go` | AC-2: the peer refuses an EAP-TLS 1.3 exchange whose authenticator sent no indication, and the MSK is denied | done, phase 7. UNTAGGED on purpose: the published RFC puts no obligation on the peer, errata 7577 proposes one and is Reported rather than Verified, and none of the three `RFC9190-2.5-*` ids covers a peer |
| `TestEAPTLS13PeerCompletesWithTheIndication`, `TestEAPTLS12PeerCompletesWithNoIndication` | same | AC-2 negative polarity: a peer that refused every exchange, and a requirement reading the configured version rather than the negotiated one, would each pass the row above | done, phase 7 |
| `TestEAPTLS13PeerRefusesTheWrongIndicationPayload` | same | AC-2: the VALUE is checked, over four payloads. strongSwan's `client_process` requires exactly one octet equal to 0 | done, phase 7 |
| `TestEAPTLSPeerAccumulatesTheApplicationDataItReads` | same | AC-2: the judgement is over the WHOLE of the application data, across reads, and `eapTLSIndicationKept` bounds what one authenticator can make the session hold | done, phase 7 |
| `TestEAPTLS13PeerTellsAnUnreadableIndicationFromAnAbsentOne` | same | AC-2: a failed post-handshake read and an authenticator that sent nothing are different answers with different messages | done, phase 7 |
| `TestEAPTLS13ResumedExchangeStillRequiresTheIndication` | same | AC-2 x AC-4: RFC 9190 Figure 3 carries the indication on a resumed exchange too, so a peer cannot be made to skip the check by offering a ticket | done, phase 7 |
| `TestEAPTLS12SendsNoProtectedSuccessIndication` | same | AC-3 | done, phase 1 |
| ~~`TestEAPTLSIssuesNoUnredeemableSessionTicket`~~ | same | REWRITTEN phase 5 as `TestEAPTLS13TicketAndIndicationShareOneEAPRequest`, per the OWNER RULING above: it pinned `SessionTicketsDisabled`, which under that ruling pins a non-conformance. The replacement asserts RFC 9190 Figure 2, that the ticket and the 0x00 indication leave in ONE EAP-Request. It carried no `RFC requirement:` tag; the commit gate reads removed test functions, so name it in the commit body | rewritten, phase 5 |
| `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems` | `internal/core/eap/rfc9190_resumption_test.go` | AC-4, RFC9190-2.1.2-1 and 2.1.3-1: a ticket is issued and the next exchange resumes on it | done, phase 5 |
| `TestEAPTLS13ResumedSessionDerivesTheRFC9190MSK` | same | AC-4, both ends derive the SAME 64-octet MSK from a resumed session | done, phase 5 |
| `TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication` | same | AC-4 x AC-1: RFC 9190 Figure 3 carries the 0x00 indication on a resumed exchange too | done, phase 5 |
| `TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`, `TestEAPTLS13ResumptionRefusesARevokedClientChain` | same | AC-4 x AC-5, the fail-open regression: §5.4 still refuses a revoked chain when the handshake RESUMED | done, phase 5 |
| `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` | same | AC-4 x AC-5 negative: a gate that refused every resumption would pass both rows above | done, phase 5 |
| `TestEAPTLS13PeerRefusesAResumptionItCannotRebuildAChainFor` | `internal/core/eap/rfc9190_resumption_refusal_test.go` | AC-4, the rebuild fails closed: no chain is never a clean answer | done, phase 5 |
| `TestVerifyConnectionRefusesAnEmptyChainSetOnAFullHandshake` | same | AC-4, the gate was EXTENDED and not loosened: a non-resumed empty chain set still refuses | done, phase 5 |
| `TestEAPTLS13ResumptionStillNeedsARevocationSource` | same | AC-4 x AC-5, `errNoRevocationSource` is still reached on a resumed TLS 1.3 session | done, phase 5 |
| `TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey`, `TestResumptionCacheIsPerPeering` | same | AC-4, per-peering isolation. `clientSessionCacheKey` collapses every EAP-TLS connection onto the constant key `"eap"`, so one shared store would offer peer A's ticket to peer B | done, phase 5 |
| `TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode` | same | AC-8 in unit form: a client that never offers `psk_dhe_ke` (charon's shape) gets no ticket, so issuance is invisible to it | done, phase 5 |
| `TestEAPTLS13RefusesATicketPastTheSection57Lifetime`, `TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling`, `TestResumptionCacheDropsAnExpiredEntryOnPut` | same | RFC9190-5.7-5: the 604800s bound is on STORING, which crypto/tls does not answer, so ze's cache does | done, phase 5 |
| `TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired`, `TestEAPTLS13RefusesAResumptionAgainstAReplacedTrustAnchor` | same | RFC9190-5.7-2 and 5.7-6: the cached chain is re-judged against the CURRENT material | done, phase 5 |
| `TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket`, `TestResumptionOffOffersNoTicketOnThePeer` | same | the `session-resumption` leaf gates USING a ticket and never ISSUING one, which §2.1.2 makes unconditional | done, phase 5 |
| `TestEAPTLSResumptionRotatesItsTicketKey` | same | `SetSessionTicketKeys` turns Go's own rotation off, so ze reproduces the 24h/7d schedule it displaces | done, phase 5 |
| `TestEAPTLS12ExchangeIsUnchangedByResumptionState` | same | AC-3 under resumption: TLS 1.2 issues no ticket and derives the RFC 5216 MSK | done, phase 5 |
| `TestResumptionStoreIsOnePerPeerAndOutlivesTheSA`, `TestResumptionStoreIsReplacedWhenTheAuthenticationChanges`, `TestResumptionStoreIsDroppedForAPeerTheConfigNoLongerNames`, `TestEAPTLSConfigsCarryTheResumptionStore`, `TestEAPTLSConfigsRefuseAnSAWithNoResumptionStore`, `TestEAPTLSAuthenticationIsCountedByOutcome` | `internal/component/ike/engine/rfc9190_resumption_wiring_test.go` | the wiring test: the store outlives `clear vpn ipsec sa`, which destroys the peer session (`TerminateAllSAs`, `register.go`), and is invalidated when the operator edits the authentication | done, phase 5 |
| `TestSessionResumptionDefaultsToTheYANGDefault`, `TestSessionResumptionReadsBothPolarities`, `TestSessionResumptionRefusesANonBoolean`, `TestSessionResumptionIsPartOfPeerEquality` | `internal/component/ike/ipsec/config_session_resumption_test.go` | the `session-resumption` leaf an operator writes | done, phase 5 |
| `TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm` | `internal/core/eap/rfc9190_nai_test.go` | AC-6, RFC9190-2.1.8-2 positive: `alice@example.com` reaches the wire as `@example.com`, and no packet the peer sent carries the username | done, phase 5b |
| `TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm` | same | AC-6, RFC9190-2.1.8-5: an identity with no realm has no username to omit, so it takes the fixed `anonymous` Section 2.1.8 allows. This is the shape every EAP-TLS scenario in the repo configures | done, phase 5b |
| `TestEAPTLSPeerAnonymizesEveryConfiguredIdentity` | same | AC-6, RFC9190-2.1.8-3 positive: every NAI the peer can emit matches the RFC 7542 Section 2.2 grammar, over 12 identity shapes including the ones no grammar accepts. It is also what keeps §5.8-1/2/3's antecedent false | done, phase 5b |
| `TestNAIGrammarMatchesRFC7542Section22` | same | AC-6, RFC9190-2.1.8-3 negative: the grammar refuses 19 strings, one for each rule, so the positive claim is not a checker that accepts everything | done, phase 5b |
| `TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity` | same | AC-6, RFC9190-2.1.8-2 negative: RFC 9190 governs EAP-TLS, so a password method's Identity Response is unchanged. A peer that anonymized every method would pass every row above | done, phase 5b |
| `TestEAPTLS13PeerDerivesItsRealmFromTheCertificateNAI` | `internal/core/eap/rfc9190_cert_nai_test.go` | AC-6, RFC9190-2.1.7-1 positive on the wire: the local-id carries no realm, the client certificate names `alice@example.com` as an rfc822Name, and the Identity Response is `@example.com` with `alice` in no packet | done, phase 7b |
| `TestEAPTLSPeerDerivesTheRealmFromEveryNAIBearingCertificateField` | same | AC-6, RFC9190-2.1.7-1 positive over each field that can hold an NAI: the rfc822Name Section 2.1.7 names, the userPrincipalName otherName an enterprise CA writes, and the subject common name, with the extension preferred over the subject | done, phase 7b |
| `TestEAPTLSPeerKeepsTheAnonymousFallbackWhenNoCertificateNAIIsUsable` | same | AC-6, RFC9190-2.1.7-1 negative: a dNSName is no NAI, and a certificate that names no realm, one whose realm the grammar refuses, and material that does not parse each leave the fixed username. A peer that derived a realm from anything would pass the rows above and fail every row here | done, phase 7b |
| `TestEAPTLSPeerPrefersTheConfiguredRealmOverTheCertificate` | same | AC-6, RFC9190-2.1.7-1 negative: the certificate is the source only where the identity gives no realm, because Section 2.1.3 wants the realm the deployment routes on again on a resumption | done, phase 7b |
| `TestEAPTLSPeerDropsTheUsernameTheCertificateCarries` | same | AC-6, RFC9190-2.1.8-2 positive: a certificate NAI carries a username exactly as a configured identity does, and none of the three fields lets one survive the derivation | done, phase 7b |
| `TestEAPTLS13AuthenticatorAcceptsAnAnonymousNAI` | same | AC-6, RFC9190-2.1.8-1 positive on the SERVER role: `@realm` and `anonymous@realm` each complete an exchange with a shared MSK | done, phase 5b |
| `TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal` | same | AC-6, RFC9190-2.1.8-4 with 2.1.8-6: the authenticator's own `tls.Config` ends the handshake on an empty `certificate_list` and completes on a real one | done, phase 5b |
| `TestEAPTLS13RefusesARevokedClientCertificate` | `internal/core/eap/rfc9190_revocation_test.go` | AC-5, RFC9190-5.4-1 positive on the authenticator | done, phase 4 |
| `TestEAPTLS13RefusesARevokedServerCertificate` | same | AC-5, RFC9190-5.4-1 positive on the peer | done, phase 4 |
| `TestEAPTLS13RefusesARevokedIntermediate` | same | AC-5, "all the certificates in the certificate chains" rather than the leaf alone | done, phase 4 |
| `TestEAPTLS13RefusesAnUncheckableChain` | same | AC-5, the "MUST be checked" half: no source means no check | done, phase 4 |
| `TestEAPTLS13RefusesAStaleRevocationList` | same | AC-5, a list past its nextUpdate answers nothing about the present | done, phase 4 |
| `TestEAPTLS13ExceptsTheTrustAnchorFromRevocation` | same | AC-5, the "(except the trust anchor)" clause is applied rather than merely unreachable | done, phase 4 |
| `TestEAPTLS13CompletesWithAnUnrevokedChain` | same | AC-5, RFC9190-5.4-1 negative: a gate that refused everything would pass every row above | done, phase 4 |
| `TestEAPTLS12CompletesWithNoRevocationList` | same | AC-5, RFC9190-5.4-1 negative: Section 5.4 opens "When EAP-TLS is used with TLS 1.3", so RFC 5216 Section 5.4 governs TLS 1.2 | done, phase 4 |
| `TestParseCACRLAcceptsPEMAndBase64`, `TestCACRLPEMRoundTripsToTheConsumer`, `TestCACRLPEMIsNilWhenNoListIsConfigured`, `TestParseCACRLRefusesAnotherCAsList`, `TestParseCACRLRefusesACertificatePastedIntoTheCRLLeaf` | `internal/component/pki/config_crl_test.go` | the `crl` leaf-list an operator writes, and what the consumer receives | done, phase 4 |
| `TestEAPTLSConfigsCarryTheCARevocationLists`, `TestEAPTLSConfigsCarryNoListWhenTheCAHasNone` | `internal/component/ike/engine/rfc9190_crl_wiring_test.go` | the wiring test: what the operator wrote reaches BOTH EAP-TLS roles | done, phase 4 |
| `TestEAPTLS13StaplesTheConfiguredOCSPResponse` | `internal/core/eap/rfc9190_ocsp_test.go` | AC-5, RFC9190-5.4-2 positive: the peer's ConnectionState carries back the exact DER the operator configured | done, phase 6 |
| `TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse` | same | AC-5, RFC9190-5.4-2 negative: the staple is the operator's response and not a fixed answer | done, phase 6 |
| `TestEAPTLS13PeerRefusesAnAuthenticatorThatStaplesNothing`, `TestEAPTLS13PeerRefusesARevokedStapledStatus`, `TestEAPTLS13PeerRefusesAnIntermediateItCannotReadTheStatusOf` | same | AC-5, RFC9190-5.4-3 positive: absent, revoked, and unreadable-because-Go-drops-it are each an invalid CertificateEntry | done, phase 6 |
| `TestEAPTLS13PeerCompletesWithAValidStapledStatus`, `TestEAPTLS13PeerWithoutTheStatusLeafAcceptsAnAuthenticatorThatStaplesNothing` | same | AC-5, RFC9190-5.4-3 negative: a peer that refused every chain, and a leaf that gated nothing, would each pass the rows above | done, phase 6 |
| `TestCertificateStatusRefusesAnExpiredResponse`, `TestCertificateStatusRefusesAResponseDatedAhead`, `TestCertificateStatusRefusesAResponseAboutAnotherCertificate`, `TestCertificateStatusRefusesAnUnknownStatus`, `TestCertificateStatusRefusesAnUndelegatedResponder` | same | AC-5, RFC9190-5.4-3 positive: the four properties `CheckCertificateStatus` requires together | done, phase 6 |
| `TestCertificateStatusAcceptsADelegatedResponder`, `TestStapledChainStatusExceptsTheTrustAnchor` | same | AC-5, RFC9190-5.4-3 negative: a delegated responder RFC 6960 Section 4.2.2.2 defines is accepted, and the anchor is asked for no status | done, phase 6 |
| `TestEAPTLS13PeerKeepsTheChainItAcceptedForTheLaterCheck`, `TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`, `TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt` | same | AC-5, RFC9190-5.4-4 and RFC5216-5.4-2: the post-authentication check reads the chain the handshake accepted, on both TLS versions, and reads none from a handshake that failed | done, phase 6 |
| `TestParseCertificateOCSPResponseAcceptsPEMAndBase64`, `TestCertificateOCSPResponseIsNilWhenNoneIsConfigured`, `TestParseCertificateOCSPResponseRefusesAnotherCertificatesResponse`, `TestParseCertificateOCSPResponseRefusesACertificatePastedIntoTheLeaf`, `TestParseCertificateOCSPResponseRefusesAWrongPEMLabel` | `internal/component/pki/config_ocsp_test.go` | the `ocsp-response` leaf an operator writes, and what the consumer receives | done, phase 6 |
| `TestEAPTLSConfigsCarryTheStapledOCSPResponse`, `TestEAPTLSConfigsCarryNoStapleWhenTheCertificateHasNone`, `TestEAPTLSPeerConfigCarriesTheCertificateStatusRequestLeaf` | `internal/component/ike/engine/rfc9190_ocsp_wiring_test.go` | the wiring test: the response reaches the authenticator and the status leaf reaches the peer | done, phase 6 |
| `TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`, `TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood` | `internal/component/ike/engine/rfc9190_postauth_test.go` | AC-5, RFC9190-5.4-4 and RFC5216-5.4-2 in both polarities, over a REAL EAP-TLS exchange driven through both config builders | done, phase 6 |
| `TestPostAuthenticationCheckRefusesAnInsecureResponderURL`, `TestPostAuthenticationCheckReadsAnHTTPSResponder` | same | AC-5, RFC9190-5.4-5 in both polarities: an http responder is refused before any connection opens, and an https one is asked | done, phase 6 |
| `TestPostAuthenticationCheckIsNotStartedWithoutAnEAPTLSPeerSession`, `TestPostAuthenticationCheckReportsUncheckedWithNoResponder`, `TestServerCertRecheckStopReleasesAFetchInFlight` | same | the check's three other states: nothing to check, no answer obtained, and the stop path that releases a fetch in flight | done, phase 6 |
| `TestEAPTLSSetsNoLengthBitOnAnUnfragmentedMessage` | `internal/core/eap/rfc9190_fragmentation_test.go` | RFC9190-2.1.9-1 negative: a message that fits in one fragment leaves `nextFragment` with the L bit clear and its TLS data at offset 1, and no whole message or continuation fragment of a live exchange carries the bit from either seat | done, phase 8 |
| `TestEAPTLSKeepsTheLengthBitOnAFragmentedMessage` | same | RFC9190-2.1.9-1 positive: the prohibition covers unfragmented messages alone, so a genuinely fragmented message still declares its length on its first fragment. A fix that dropped the bit everywhere would pass the row above | done, phase 8 |
| `TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit` | same | RFC9190-2.1.9-2 positive: `reassemble` yields the same octets from both shapes the sentence permits | done, phase 8 |
| `TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself` | same | RFC9190-2.1.9-2 negative: an L bit with no length behind it, and a payload longer than the length declared, are each refused, so acceptance is not blanket | done, phase 8 |
| `TestEAPTLSCapsBothRolesAtTLS13` | `internal/core/eap/rfc9190_version_cap_test.go` | RFC9190-1-1 positive: `newTLSMethod` and `PeerSession.tlsClientConfig` each set `MaxVersion` to `tls.VersionTLS13`, and the peer's ClientHello offers nothing above it | done, phase 8 |
| `TestEAPTLSVersionCapLeavesTLS12Reachable` | same | RFC9190-1-1 negative: the ceiling is a range end and not a pin, so a TLS 1.2 exchange still completes and both ends derive the same MSK. A role that met the MUST by pinning `MaxVersion` to `MinVersion` would pass the row above | done, phase 8 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| exported key material | 128 octets (RFC 9190 Section 2.3) | 128 | 127 | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls13-resumption` | `test/ipsec/ipsec-eap-tls13-resumption.ci` | a peer reconnects and resumes rather than re-handshaking, across the `clear vpn ipsec sa` that destroys and rebuilds the peer session | done, phase 5. Discrimination measured 2026-09-08: with `resumptionFor` building a fresh store per lookup it goes RED at 8.1s (`resumed=true` never logged); restored, GREEN at 6.1s |
| `ipsec-eap-tls13-ocsp-stapling` | `test/ipsec/ipsec-eap-tls13-ocsp-stapling.ci` | an operator configures an ocsp-response on the authenticator and certificate-status-request on the peer, and the tunnel comes up | done, phase 6. Discrimination measured 2026-09-08: with `cert.OCSPStaple = nil` in `newTLSMethod` it goes RED, and the red is its pair's own message, `reject=stderr pattern found: stapled no OCSP response` |
| `ipsec-eap-tls13-ocsp-required` | `test/ipsec/ipsec-eap-tls13-ocsp-required.ci` | the same configuration with the ocsp-response leaf removed and nothing else changed: no SA is established and the peer's log names the missing CertificateStatus | done, phase 6. Discrimination measured 2026-09-08: with `checkStapledChainStatus` returning nil it goes RED at 90.1s, because the await for the refusal never matches and the SA establishes instead |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `eap-tls13` | `test/interop-ipsec/scenarios/` | strongSwan | Ze as EAP-TLS CLIENT on TLS 1.3 | exists, green after phase 1 |
| `responder-eap-tls13` | same | strongSwan | Ze as EAP-TLS SERVER sends the indication and a real client accepts it. Reverting the write makes charon log `missing protected success indication` and the SA never establishes | done, phase 1 |
| ~~a resumption scenario~~ | - | - | **No third-party implementation can drive ze's RFC 9190 resumption, so AC-4 has no interop counterpart in existence. Owner ruling 2026-09-08: ze-to-ze evidence stands.** strongSwan 5.9.14's TLS client never sends `psk_key_exchange_modes` (`src/libtls/tls_peer.c` `send_client_hello` writes eight extensions, none of them that) and its `tls_cache_create` (`src/libtls/tls_cache.c`) has no caller in its tree, so ze issues it no ticket: `shouldSendSessionTickets` (`crypto/tls/handshake_server_tls13.go`) requires the client to have offered `psk_dhe_ke`. Libreswan states in its own comment `programs/pluto/ikev2_eap.c` "EAP responder transitions, there is no initiator code", and exports the MSK with the RFC 5216 label `"client EAP encryption"` at 64 octets unconditionally, so it is not RFC 9190 conformant and would not agree with ze on a TLS 1.3 MSK. OpenIKED speaks EAP-MSCHAPv2 only. This is a fact about the world, NOT a `{gap}` and NOT a `{not-applicable}`: the behavior exists and is proven, only a second implementation is missing | recorded, phase 5 |
| `responder-eap-tls13-revoked-client` | same | strongSwan | RFC9190-5.4-1 against a real peer, with Ze in the EAP-TLS SERVER role. It is `responder-eap-tls13` with ONE file changed, the CRL naming the strongSwan client certificate's serial number | done, phase 4. Red phase measured 2026-09-05: with `checkChainRevocation` made to return nil, charon logs `received protected success indication via TLS` and `CHILD_SA ze-child{1} established`; with it restored, charon logs `received fatal TLS alert 'bad certificate'` and `EAP_TLS method failed`, and neither end installs an XFRM state |

## Files to Modify
- `internal/core/eap/eap_tls.go` - the indication on the authenticator, resumption, and the authenticator's revocation gate.
- `internal/core/eap/peer.go` - the indication on the peer, and the peer's revocation gate.
- `internal/core/eap/eap.go` - `MethodConfig.CRLPEM`.
- `internal/component/pki/config.go`, `types.go`, `store.go`, `yang/ze-pki-conf.yang` - the `crl` leaf-list a CA carries, and `CACertEntry.CRLPEM`.
- `internal/component/ike/engine/responder_eap.go`, `fsm.go` - the lists reaching each EAP-TLS role.
- `internal/le/interoplab/ipsec/checkers.go` - the revoked-client checker.
- `rfc/enrolled.txt`, `rfc/not-enrolled.txt` - move the row at the end.
- `docs/features/rfc-status.md` - the public row.

## Files to Create
- `rfc/extraction/rfc9190.json` - the hand-classified sign-off enrolment requires.
- `internal/core/eap/rfc9190_test.go` - created, phase 1.
- `internal/core/eap/revocation.go` - created, phase 4. The RFC 9190 Section 5.4
  chain walk both roles run.
- `internal/core/eap/rfc9190_revocation_test.go` - created, phase 4.
- `internal/core/eap/nai.go` - created, phase 5b. The RFC 9190 Section 2.1.8
  anonymous NAI, and the RFC 7542 Section 2.2 grammar it must match.
- `internal/core/eap/rfc9190_nai_test.go` - created, phase 5b.
- `internal/core/eap/rfc9190_fragmentation_test.go` - created, phase 8. The RFC 9190
  Section 2.1.9 L-bit pair on send and the accept-both-shapes pair on receive.
- `internal/core/eap/rfc9190_version_cap_test.go` - created, phase 8. The RFC 9190
  Section 1 TLS version ceiling on both roles.
- `rfc/full/rfc7542.txt` - fetched, phase 5b. RFC 9190 Section 2.1.8 makes the
  NAI grammar normative, and a summary is never the authority for it.
- `internal/component/pki/config_crl_test.go` - created, phase 4.
- `internal/component/ike/engine/rfc9190_crl_wiring_test.go` - created, phase 4.
- `test/interop-ipsec/scenarios/responder-eap-tls13/` - created, phase 1. The
  first scenario in the lab with Ze in the EAP-TLS SERVER role. eap-tls and eap-tls13 both
  put strongSwan there, which is why a wire-visible violation on Ze's
  authenticator survived until 2026-08-12.
- `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client/` - created, phase 4.
- `test/ipsec/ipsec-eap-tls13-resumption.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | resumption and OCSP need operator control; read `ai/rules/config.md` |
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

1. Land Section 2.5 ALONE, both roles, with scenarios eap-tls and eap-tls13 run before and after.
   DONE 2026-08-12 for the SERVER role, plus scenario responder-eap-tls13 which reads that role
   against strongSwan for the first time.
   The PEER role is DONE 2026-09-08 (phase 7): `requireSuccessIndication`
   (`internal/core/eap/peer_indication.go`) refuses the EAP-Success on a negotiated TLS 1.3
   session unless the accumulated application data is exactly one octet equal to 0x00, and
   `consumePostHandshakeRecords` beside it keeps an absent indication, a failed read and a
   wrong payload apart. It is STRICTER than the published RFC, which addresses Section 2.5
   only to the server, so nothing it added carries an `RFC requirement:` tag: see the AC-2
   rows in the TDD Test Plan.
2. Validate A-1 by reading strongSwan's `eap_tls.c` before assuming it can check ours.
   DONE 2026-08-12, and the reading is quoted in the A-1 row.
3. Resumption and NewSessionTicket.
   DONE 2026-09-08 (phase 5), both roles. The ticket key and the client cache belong to
   the PEERING and not to the SA or the peer session: `resumptionFor`
   (`internal/component/ike/engine/resumption.go`) keys them by peer name and invalidates
   on an authentication edit, because `TerminateAllSAs` (`register.go`) deletes the
   `PeerSession` and `clear vpn ipsec sa` is the operator command most likely to precede a
   second authentication. `Resumption` (`internal/core/eap/resumption.go`) owns the
   §5.7-5 604800s STORAGE bound, which `crypto/tls` does not answer, and reproduces the
   24h/7d key rotation that `SetSessionTicketKeys` turns off.

   THE PEER-SIDE CHAIN REBUILD IS THE ONE SECURITY-RELEVANT ADDITION.
   `rebuildResumedChains` (`internal/core/eap/peer_chain.go`) exists because Go skips
   `verifyPeerCertificate` entirely on a resumed handshake, which is where this role BUILDS
   the chain the §5.4 check reads. It reads `cs.PeerCertificates` and NOT
   `cs.VerifiedChains`: `Conn.verifyServerCertificate` assigns `c.verifiedChains` only
   under `else if !c.config.InsecureSkipVerify`, and this peer sets `InsecureSkipVerify`
   because EAP-TLS carries no server hostname, so `cs.VerifiedChains` is empty on a FULL ze
   handshake too and reading it would leave every resumed session refused. It is gated on
   `cs.DidResume`, so a non-resumed empty chain set still refuses exactly as before.
4. OCSP stapling and revocation.
   DONE. RFC9190-5.4-1 landed 2026-09-05 (the chain revocation check on both roles, with
   CRL as the source Section 5.4 permits). The remaining four landed 2026-09-08.

   5.4-2 is the `ocsp-response` leaf on a `pki certificate` reaching
   `tls.Certificate.OCSPStaple` (`newTLSMethod`, `internal/core/eap/eap_tls.go`), which is
   what crypto/tls answers a client's status_request with. No fetch-and-refresh path was
   built and none is owed: RFC 6066 and RFC 8446 Section 4.4.2.1 fix how the response is
   CARRIED and say nothing about how a server obtains one, so an operator-supplied
   response implements the requirement. The response's validity is the relying party's
   judgement (RFC 6960 Section 3.2), and ze makes it as a peer.

   5.4-3 is `checkStapledChainStatus` (`internal/core/eap/ocsp.go`), gated on the
   `certificate-status-request` leaf, which is what makes ze a peer that USES Certificate
   Status Requests and so the antecedent of that sentence's MUST. The Go limitation is
   REAL and was re-read on 2026-09-08 against `$(go env GOROOT)/src/crypto/tls/handshake_messages.go`:
   `unmarshalCertificate` still skips the extensions of every entry after the leaf, and
   `marshalCertificate` writes none for them either. It is handled by FAILING CLOSED
   rather than by recording a gap: an intermediate below the trust anchor is a
   CertificateEntry whose status ze cannot see, Section 5.4 makes an entry without a
   valid status invalid, and "not sent" and "not visible" are the same bytes to this
   role. A chain carrying one is therefore refused while the leaf is on. What would lift
   that restriction is a crypto/tls exposing per-entry extensions, and nothing in ze can.

   5.4-4 and 5.4-5 are `startServerCertRecheck`
   (`internal/component/ike/engine/postauth_revocation.go`), started by `runEstablished`
   after the Child SA is installed, because the tunnel the EAP exchange was run to obtain
   IS the connectivity Section 5.4 waits for. It reads the chain the peer accepted
   (`eap.PeerSession.ServerChains`), asks each certificate's own OCSP responder over
   https, refusing an http url before any connection opens, and sends a verdict the owner
   loop turns into a closed SA. A responder that answers nothing leaves the SA up with a
   warning: 5.4-6 and 5.4-7, the SHOULD NOTs that would govern distrusting the network,
   are not implemented and are not these requirements.

   IT ALSO CLOSED RFC5216-5.4-2, which is the same obligation stated without a version
   condition: proven on TLS 1.2 by `TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`
   and on the engine path by the two `TestPostAuthenticationCheck*` tests, each with a
   discrimination record in `rfc/discrimination/rfc5216.json`.

   IT ALSO CLOSED A GAP ON ANOTHER RFC, 2026-09-07. `rfc/short/rfc5216.md` recorded
   RFC5216-5.4-1, "CRL checking MUST be supported", as `{gap: no CRL logic exists
   anywhere in the EAP-TLS path}`, and RFC 5216 is ENROLLED, so that annotation was a
   published claim about a behaviour the same producer now performs. The obligation
   carries no version condition, unlike RFC 9190 Section 5.4, so it is proven on
   TLS 1.2: `TestEAPTLS12RefusesARevokedClientCertificate` and
   `TestEAPTLS12CompletesWithAnUnrevokedChain`, each with a discrimination record in
   `rfc/discrimination/rfc5216.json`. RFC5216-5.4-2, post-authentication revocation
   checking, is still a gap and is the same obligation as RFC9190-5.4-4 and 5.4-5, so
   one piece of work closes all three.
5. Anonymous and privacy-friendly NAIs.
   DONE 2026-09-08 (phase 5b), both roles. `anonymousNAI` and `validNAI`
   (`internal/core/eap/nai.go`) derive and check the NAI, and `NewPeerSessionTLS`
   (`peer.go`) is the only site that calls the derivation: it is the one constructor
   of an EAP-TLS peer, so the MUST NOT cannot be reached around. The obligation binds
   a client that SUPPORTS TLS 1.3 rather than one that negotiated it, which is also
   the only reading the code could act on, because the Identity Response leaves before
   any ClientHello. `NewPeerSessionTLS` no longer sets `userName` either: that is the
   MS-CHAPv2 name field, which EAP-TLS never reaches, so it was the operator's
   username stored on a session that must not emit one.

   THE SERVER ROLE NEEDED NO CODE. `Session.handleIdentity` (`eap.go`) records the
   NAI and asks it for nothing, and no caller reads `Session.Identity()`, so ze
   already authenticates a peer that reveals no username. That is RFC9190-2.1.8-1 and
   it is also what RFC9190-2.2-1 requires ("Unauthenticated information MUST NOT be
   used ... to give authorization"). The test is the gate on it, not the feature.

   THE CERTIFICATE IS THE SECOND SOURCE OF THE REALM, added 2026-09-08 (phase 7b).
   RFC 9190 Section 2.1.7: "When the client certificate contains an NAI as subject
   name or alternative subject name, an anonymous NAI SHOULD be derived from the NAI
   in the certificate". `naiSource` (`nai.go`) chooses the source and `anonymousNAI`
   still decides what is sent, so a realm read out of a certificate arrives with its
   username already dropped. Three fields are read, in order: the subjectAltName
   rfc822Name, the subjectAltName userPrincipalName otherName, and the subject
   common name. A dNSName is not one of them, because it carries no "@" and the
   RFC 7542 Section 2.2 grammar reads it as a utf8-username rather than a realm.
   The configured identity still wins wherever it carries a realm, and certificate
   material that does not parse yields no candidate rather than a failed session.

      TWO SECTION 5.8 CONSEQUENCES FOR STEP 6. RFC9190-5.8-1, 5.8-2 and 5.8-3 open "If
   anonymous NAIs are not used", and ze now uses one on every EAP-TLS exchange, so
   they are exclusions rather than gaps and `TestEAPTLSPeerAnonymizesEveryConfiguredIdentity`
   is what holds their antecedent false. RFC9190-5.8-4 (record padding) is a SHOULD
   that `crypto/tls` exposes no control for; it is untouched by this step.
6. Write `rfc/extraction/rfc9190.json` by hand and run `./le rfc check`.
   DONE 2026-09-08. 52 sites in 36 sections, register `prose`, 48 mapped and 4 excluded
   (0.08 of every site). `./le rfc check` reports no finding against
   `rfc/extraction/rfc9190.json`. The four exclusions are the IETF Trust boilerplate at
   `front:1`, the indicative consequence of the Section 2.1.3 MAY at `2.1.3:2`, step 3 of the
   Section 2.5 procedure at `2.5:2` (`duplicate-of` RFC9190-2.5-1), and the accounting
   rationale at `5.7:5`. No site carries `binds-another-role` and none carries
   `feature-out-of-scope`.

   THE WALK FOUND AN OBLIGATION THE SUMMARY HAD MISSED, which is the forward arithmetic
   doing the job it exists for. RFC 9190 Section 1: *"Therefore, implementations MUST limit
   the maximum TLS version they use to 1.3, unless later versions are explicitly enabled by
   the administrator."* It is now declared as RFC9190-1-1 and site `1:1` maps it. Ze does not
   meet it: `newTLSMethod` (`internal/core/eap/eap_tls.go`) and `startTLSClient`
   (`internal/core/eap/peer.go`) are the two `tls.Config` builders, both set `MinVersion`, and
   neither sets `MaxVersion`.

   THE SECTION 5.8 CLASSIFICATION IS A MAPPING, NOT AN EXCLUSION, and the reason is
   mechanical rather than a change of mind. The antecedent is false, verified at the producer:
   `anonymousNAI` (`internal/core/eap/nai.go`) has exactly two return values, `"@"+realm` and
   the constant `anonymousUser`, and `NewPeerSessionTLS` (`peer.go`) is the one constructor of
   an EAP-TLS peer session and applies it unconditionally, so ze never emits a
   privacy-friendly username. But `rfc/short/rfc9190.md` DECLARES RFC9190-5.8-1, 5.8-2 and
   5.8-3, and `evaluateExtraction` (`internal/le/rfc/signoff.go`) refuses a sign-off in which
   a declared gated requirement is the target of no site and appears in no `unsourced-ids`.
   Excluding the three sites would therefore fail the gate. The precedent is RFC 8671 site
   `5.2:2`, which is MAPPED for the same reason while its sibling `5.2:1`, whose id is not
   declared, is excluded `feature-out-of-scope`. The scope decision is recorded in the reason
   at site `5.8:1`, and the annotation that would retire the three rows is the owner's to
   authorise.
7. Move the row from `rfc/not-enrolled.txt` to `rfc/enrolled.txt`, add the status row.
   NOT DONE, and it must not be done yet. Measured 2026-09-08 by counting
   `RFC requirement: RFC9190-<id> <polarity>` tags under `internal/`, `test/`, `cmd/` and
   `pkg/` against this summary's gated rows: of 52 gated MUST-level requirements, 16 carry
   both polarities, 7 carry a positive only, and 29 carry no tagged test at all. Enrolling
   over 36 unproven MUSTs would publish a `Supported` row `ai/rules/rfc-compliance.md` names
   as the exact failure it forbids, and the remedy that rule allows is to write the tests, not
   to lower the row.

   TWO OF THE 36 ARE UNMET IN CODE rather than merely unproven, so a test written today would
   be RED. RFC9190-2.1.9-1: *"Implementations MUST NOT set the L bit in unfragmented
   messages"* (Section 2.1.9). `tlsFragmenter.nextFragment` (`internal/core/eap/eap_tls.go`)
   is the one producer of outbound EAP-TLS TypeData on BOTH roles, and it sets `eapTLSFlagL`
   and the four-octet length whenever `isFirst` holds, with no test of `isLast`, so every
   unfragmented message ze sends carries the L bit. The fix is to gate both writes on
   `isFirst && !isLast`; it is wire-visible on every EAP-TLS message, so it owes
   `./le functional ipsec` and the `eap-tls`, `eap-tls13` and `responder-eap-tls13` scenarios.
   RFC9190-1-1 is the second, described in step 6.
8. 5.10-1 needs no classification: it is proven in both polarities by
   `internal/core/eap/rfc9190_attack_mitigation_test.go` and needs no annotation.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The Section 2.5 protected success indication is sent by ze's EAP-TLS server and required by its peer | interop | `test/interop-ipsec/scenarios/responder-eap-tls13` against strongSwan 5.9.14, green. Reverting `tlsMethod.indicateSuccess` makes charon log `missing protected success indication for EAP-TLS with TLS 1.3` and no SA establishes; restored, `CHILD_SA ze-child{1} established` |
| Resumption and NewSessionTicket work on both roles across an SA teardown | functional | `test/ipsec/ipsec-eap-tls13-resumption.ci`, two ze daemons across `clear vpn ipsec sa`. Red phase measured 2026-09-08: with `resumptionFor` building a fresh store per lookup it fails at 8.1s (`resumed=true` never logged); restored, green at 6.1s |
| OCSP stapling is honoured in both directions, and a chain with no valid status is refused | functional | `test/ipsec/ipsec-eap-tls13-ocsp-stapling.ci` and `test/ipsec/ipsec-eap-tls13-ocsp-required.ci`. Red phases measured 2026-09-08: `cert.OCSPStaple = nil` reddens the first with `stapled no OCSP response`; `checkStapledChainStatus` returning nil reddens the second at 90.1s because the SA establishes instead |
| Section 5.4 revocation is enforced against a real third-party peer | interop | `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client`, green. Red phase measured 2026-09-05: with `checkChainRevocation` returning nil charon reaches `CHILD_SA ze-child{1} established`; restored, charon logs `received fatal TLS alert 'bad certificate'` and neither end installs an XFRM state |
| Ze emits an anonymous NAI on every EAP-TLS exchange, so no permanent identifier reaches the wire | functional | `TestEAPTLSPeerAnonymizesEveryConfiguredIdentity` over 12 identity shapes, with `TestNAIGrammarMatchesRFC7542Section22` refusing 19 strings as its negative, and `TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity` showing a password method is unchanged. Producer verified: `anonymousNAI` (`internal/core/eap/nai.go`) has two return values and `NewPeerSessionTLS` (`peer.go`) is its only caller |
| Section 5.10-1, "MUST mitigate known attacks", is proven rather than declared untestable | functional | 16 tagged units in `internal/core/eap/rfc9190_attack_mitigation_test.go` over the nine RFC 7457 Section 2 attacks an EAP-TLS implementation can hold a property against, both polarities, each with a record in `rfc/discrimination/rfc9190.json` |
| Every normative sentence of RFC 9190 is accounted for, so nothing the summary missed can hide | extraction sign-off | `rfc/extraction/rfc9190.json`, signed 2026-09-08: 52 sites in 36 sections, 48 mapped and 4 excluded, `./le rfc check` reporting no finding against it. It FOUND a miss: RFC9190-1-1, the Section 1 MUST on the maximum TLS version, was undeclared until this walk |
| RFC 9190 is enrolled with no `{gap}` and no `{not-applicable}` covering a feature ze could have built | `./le rfc check` | NOT ACHIEVED. Of 52 gated MUST-level requirements, 16 carry both polarities, 7 a positive only, and 29 no tagged test; two of the 36 (RFC9190-2.1.9-1 and RFC9190-1-1) are unmet in code. Measured 2026-09-08 by counting `RFC requirement:` tags under `internal/`, `test/`, `cmd/` and `pkg/`. `rfc/not-enrolled.txt` still carries the row, and its reason states the same three numbers |

## Critical Review Checklist

Added 2026-08-12, when phase 1 started: the spec was written as a skeleton and
carried no such table, which `/ze-implement` needs before it may run.

| Check | What to verify |
|-------|----------------|
| The indication is sent only after the client Finished is processed | The write happens on a round where `tlsMethod.handshaked` is already set. RFC9190-2.5-2 is a MUST NOT, so a write on any earlier round is a violation, not an optimisation |
| The indication is sent exactly once | A second EAP-Request carrying application data 0x00 breaks step 3 of the procedure ("send no more EAP-Requests"). A one-shot flag, checked before the write |
| TLS 1.2 sends nothing | The write is gated on the NEGOTIATED version read from the completed connection, never on `MinVersion` or on config. Scenario eap-tls is the proof |
| The record is encrypted application data, not a handshake message | It goes through `tls.Conn.Write`, so the record layer applies the traffic keys. A raw transport write would emit plaintext |
| Ze in the SERVER role is exercised by an interop test | Scenarios eap-tls and eap-tls13 both put strongSwan in the server role, which is why this defect survived. A scenario with Ze as the EAP-TLS server is the only thing that reads this code against another implementation |
| The interop scenario discriminates | Revert the indication, run the scenario, and record what strongSwan did. A scenario that passes either way proves nothing (`ai/rules/interop-and-goal-validation.md`) |
| Session tickets are not issued unredeemably | `newTLSMethod` builds a fresh `tls.Config` per EAP session and Go mints ticket keys per Config instance, so a ticket issued in one session cannot be read in any other. Six §5.6/§5.7 MUSTs are conditional on resumption and are dead while that holds |

## Goal Gates

- `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- `./le rfc check` shows RFC 9190 enrolled, with no annotation covering a
  feature this spec built.
- Scenarios eap-tls, eap-tls13 and responder-eap-tls13 green. They are the evidence that
  issuing a ticket is INVISIBLE to a peer that does not resume; there is no
  resumption scenario and the Interop Tests table records why.
- `test/ipsec/ipsec-eap-tls13-resumption.ci` green, with its red phase measured.

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
- [ ] Scenarios eap-tls and eap-tls13 green at every phase boundary
- [ ] `rfc/extraction/rfc9190.json` hand-classified -- DONE 2026-09-08, 52 sites in 36
  sections, 48 mapped and 4 excluded, and `./le rfc check` reports no finding against it
- [ ] `./le rfc check` green with RFC 9190 enrolled -- NOT DONE. 36 of 52 gated MUSTs
  lack a second polarity and two of them are unmet in code, so enrolment is refused
