# RFC 3748 - Extensible Authentication Protocol (EAP)

Partial in IPsec. Every requirement this repository extracted from RFC 3748, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 84.7% | 50 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 5.1% | 3 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 59 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 82.9% | 131 of 158 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 63 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 10.2% | 6 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial in IPsec |
| Enrolment | Enrolled |
| Requirements | 63 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 158 |
| Tagged units | 158 |
| Recorded audit verdicts | 36 |
| Discrimination records | 131 |
| Summary | `rfc/short/rfc3748.md` |
| Requirement shard | `rfc/requirements/rfc3748.md` |
| RFC text | `rfc/full/rfc3748.txt` |

## Enrolment

Enrolled: IKEv2-carried EAP peer and co-located authenticator are enrolled, with local method termination and no pass-through. Tagged tests cover packet handling and method exchanges, but do not establish full conformance. Current source discards undefined and wrong-role Codes before charging the peer's exchange budget or producing an authenticator Failure. The strengthened IKE carrier proof still requires recorded execution, semantic discrimination and independent rejudgment.

## What the public ledger says

**Status:** Partial in IPsec

**What the ledger says is covered**

- EAP framework inside IKEv2 IKE_AUTH, Success and Failure handling, and the Section 4.2 discards that stop a rogue authenticator bypassing the method: the peer reads an EAP-Success only once the method conversation concluded and drops an EAP-Failure once both ends indicated success. `PeerSession.Process` and `Session.Process` (`internal/core/eap`) now discard undefined and wrong-role Codes at admission
- undefined Codes no longer consume the peer's exchange budget. `handleEAPResponse` ([`internal/component/ike/engine/fsm.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/fsm.go)) returns on a discard before resetting the carrier's deadline or retry count. `TestEngineDiscardedEAPCodesPreserveExchange` exercises encrypted IKE input, both roles' state preservation and UDP silence, and valid continuation through matching MSKs and final initiator AUTH
- its strengthened assertions are pending recorded verification, not a new conformance verdict. The peer answers Type 1 (Identity), Type 2 (Notification) and Type 3 (Nak): a Notification Request draws a five-octet Notification Response and its message reaches the operator log, and a Request for an authentication Type ze does not run draws a six-octet legacy Nak naming the configured method, until the peer has answered a method Request and Section 2.1 closes the Nak (`PeerSession.handleRequest`, [`internal/core/eap/peer.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer.go)). A Type-254 Request draws the same legacy Nak, which is what Section 5.7 prescribes for a peer not equipped to interpret an Expanded Type. Type 4 (MD5-Challenge) runs on both roles and is the `authentication { mode eap-md5 }` an operator selects. It is never a default, and adopting a configuration that names it writes one warning quoting the RFC 7296 Section 2.16 sentence that discourages a method establishing no shared key (`warnKeylessEAPModes`, [`internal/component/ike/engine/eap_auth.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/eap_auth.go)).


**What the ledger says remains**

The previously reported undefined-Code budget and wrong-role Failure defects no longer describe the current admission guards. Their repaired source is not itself observed conformance: the strengthened IKE carrier test still owes recorded execution, semantic discrimination and independent rejudgment, and does not establish independent-implementation interoperability or every EAP method's discard behavior. Both key-deriving methods now export the two keys RFC 3748 Section 7.10 requires.

- **EAP-TLS:** `exportEAPTLSKeys` ([`internal/core/eap/eap_tls.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls.go)) asks the TLS exporter for the whole 128-octet Key_Material both RFC 5216 Section 2.3 and RFC 9190 Section 2.3 define, and cuts it into `MSK = Key_Material(0,63)` and `EMSK = Key_Material(64,127)` on both roles.
- **EAP-MSCHAPv2:** `deriveEMSK` ([`internal/core/eap/mschapv2.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/mschapv2.go)) expands the RFC 3079 Section 3 MPPE master key into 64 octets with HKDF-Expand over SHA-256 under a label of its own, beside the `DeriveMSK` that already took the same root down the MPPE branch. No document defines an EAP-MSCHAPv2 EMSK, and none needs to: Section 7.10 says "The EMSK is not shared with the authenticator or any other third party" and Section 7.2.1 says "Use of the EMSK is reserved", so the key never reaches the wire and no interop rests on its value. The two keys are siblings of the master key rather than parent and child, which is what meets the separation Section 7.10 demands: "an attacker recovering the MSK or EMSK MUST NOT be able to recover the other quantity with a level of effort less than brute force." Reaching either from the other means inverting SHA-1 or HMAC-SHA-256 to get back to the master key. The EMSK then goes no further than the two sessions that derived it, as the same section requires: on both methods it is an unexported field of `Session` and of `PeerSession` with no accessor and no result field, so no other package can read it, and `Close` erases it. Ze reads no Expanded Type (254), which Section 5 states as a SHOULD, and answers a Type-254 Request with the legacy Nak Section 5.7 prescribes rather than composing an Expanded Nak. Ze's authenticator sends no Notification Request, which RFC 3748 Section 5.2 states as an option ("An authenticator MAY send a Notification Request to the peer at any time when there is no outstanding Request, prior to completion of an EAP authentication method") and which the owner declined on 2026-09-01; Ze's peer answers one, which is the mandatory half. Two further features are absent by decision rather than by omission, and neither is a conformance gap. Ze's authenticator terminates every EAP method locally and does not act as a pass-through agent for a backend authentication server; Section 2 says "Support for pass-through is optional". Ze offers neither the One Time Password method (Type 5) nor the Generic Token Card method (Type 6), which Section 5 leaves to the implementation ("Implementations MAY support other Types defined here or in future RFCs"). A later scope decision can revisit any of the three. The Type 4 (MD5-Challenge) deviation authorized on 2026-08-30 was WITHDRAWN by the owner on 2026-09-01, who ordered the method implemented; both roles now run it.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 50 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (50):** [`RFC3748-4-1`](#rfc3748-4-1), [`RFC3748-4-2`](#rfc3748-4-2), [`RFC3748-2-2`](#rfc3748-2-2), [`RFC3748-2.1-1`](#rfc3748-2.1-1), [`RFC3748-2.1-2`](#rfc3748-2.1-2), [`RFC3748-2.1-3`](#rfc3748-2.1-3), [`RFC3748-4.1-1`](#rfc3748-4.1-1), [`RFC3748-4.2-2`](#rfc3748-4.2-2), [`RFC3748-7.10-2`](#rfc3748-7.10-2), [`RFC3748-4-4`](#rfc3748-4-4), [`RFC3748-4.1-3`](#rfc3748-4.1-3), [`RFC3748-4.1-4`](#rfc3748-4.1-4), [`RFC3748-4.1-5`](#rfc3748-4.1-5), [`RFC3748-4.2-5`](#rfc3748-4.2-5), [`RFC3748-4.2-6`](#rfc3748-4.2-6), [`RFC3748-4-5`](#rfc3748-4-5), [`RFC3748-4.2-7`](#rfc3748-4.2-7), [`RFC3748-4.2-8`](#rfc3748-4.2-8), [`RFC3748-4.2-9`](#rfc3748-4.2-9), [`RFC3748-2-3`](#rfc3748-2-3), [`RFC3748-2.2-1`](#rfc3748-2.2-1), [`RFC3748-4.1-6`](#rfc3748-4.1-6), [`RFC3748-4.1-7`](#rfc3748-4.1-7), [`RFC3748-4.1-8`](#rfc3748-4.1-8), [`RFC3748-4.1-9`](#rfc3748-4.1-9), [`RFC3748-4.1-10`](#rfc3748-4.1-10), [`RFC3748-4.1-11`](#rfc3748-4.1-11), [`RFC3748-2.1-4`](#rfc3748-2.1-4), [`RFC3748-4.2-15`](#rfc3748-4.2-15), [`RFC3748-4.2-10`](#rfc3748-4.2-10), [`RFC3748-4.2-11`](#rfc3748-4.2-11), [`RFC3748-4.2-12`](#rfc3748-4.2-12), [`RFC3748-4.2-13`](#rfc3748-4.2-13), [`RFC3748-4.2-14`](#rfc3748-4.2-14), [`RFC3748-7.10-5`](#rfc3748-7.10-5), [`RFC3748-7.10-6`](#rfc3748-7.10-6), [`RFC3748-7.10-7`](#rfc3748-7.10-7), [`RFC3748-5-1`](#rfc3748-5-1), [`RFC3748-5-2`](#rfc3748-5-2), [`RFC3748-5.2-1`](#rfc3748-5.2-1), [`RFC3748-5.2-2`](#rfc3748-5.2-2), [`RFC3748-5.3.1-1`](#rfc3748-5.3.1-1), [`RFC3748-5.3.1-2`](#rfc3748-5.3.1-2), [`RFC3748-5.3.1-3`](#rfc3748-5.3.1-3), [`RFC3748-5.3.1-4`](#rfc3748-5.3.1-4), [`RFC3748-5.4-1`](#rfc3748-5.4-1), [`RFC3748-5.4-2`](#rfc3748-5.4-2), [`RFC3748-5.1-2`](#rfc3748-5.1-2), [`RFC3748-7.5-1`](#rfc3748-7.5-1), [`RFC3748-7.10-4`](#rfc3748-7.10-4)

**Annotated (including scoped evidence) (9):** [`RFC3748-2-1`](#rfc3748-2-1), [`RFC3748-4.1-2`](#rfc3748-4.1-2), [`RFC3748-4.2-1`](#rfc3748-4.2-1), [`RFC3748-4.2-4`](#rfc3748-4.2-4), [`RFC3748-2.3-1`](#rfc3748-2.3-1), [`RFC3748-3.1-1`](#rfc3748-3.1-1), [`RFC3748-3.1-2`](#rfc3748-3.1-2), [`RFC3748-3.1-3`](#rfc3748-3.1-3), [`RFC3748-5.7-1`](#rfc3748-5.7-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3748-4-1` | A message with the Length field set to a value larger than the number of received octets MUST be silently discarded. (S4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L113). **positive:** `unit/verify` [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L73). **positive:** `unit/verify` [`TestRFC3748PeerDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L160). **negative:** `unit/verify` [`TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L112). **negative:** `unit/verify` [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L80). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L159) |
| `RFC3748-4-2` | The Length field is two octets and indicates the length, in octets, of the EAP packet including the Code, Identifier, Length, and Data fields. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748LengthCountsTheWholePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L357). **positive:** `unit/verify` [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L89). **negative:** `unit/verify` [`TestRFC3748LengthCountsTheWholePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L375). **negative:** `unit/verify` [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L99) |
| `RFC3748-2-1` | EAP is a 'lock step' protocol, so that other than the initial Request, a new Request cannot be sent prior to receiving a valid Response. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC3748AuthenticatorLockStep`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L168). **positive:** `unit/verify` [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L123). **negative:** no negative test. **{single-polarity}:** the authenticator Session emits one packet per call -- Begin returns a single Request (internal/core/eap/eap.go:164) and each Process returns a single *Packet (eap.go:176), so a new Request cannot precede its Response and no path leaves two outstanding, giving no negative case |
| `RFC3748-2-2` | other than the initial Request, a new Request cannot be sent prior to receiving a valid Response. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L209). **positive:** `unit/verify` [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L128). **negative:** `unit/verify` [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L109) |
| `RFC3748-2.1-1` | the peer and authenticator MUST utilize only one authentication method (Type 4 or greater) within an EAP conversation (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L223). **positive:** `unit/verify` [`TestRFC3748ThePeerKeepsToOneMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L238). **negative:** `unit/verify` [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L230). **negative:** `unit/verify` [`TestRFC3748ThePeerKeepsToOneMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L229) |
| `RFC3748-2.1-2` | However, the peer and authenticator MUST utilize only one authentication method (Type 4 or greater) within an EAP conversation, after which the authenticator MUST send a Success or Failure packet. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L255). **positive:** `unit/verify` [`TestRFC3748OneMethodThenTheResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L195). **negative:** `unit/verify` [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L265). **negative:** `unit/verify` [`TestRFC3748OneMethodThenTheResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L210) |
| `RFC3748-2.1-3` | A peer MUST NOT send a Nak (legacy or expanded) in reply to a Request after an initial non-Nak Response has been sent. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L372). **negative:** `unit/verify` [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L385) |
| `RFC3748-4.1-1` | Responses MUST only be sent in reply to a valid Request and never be retransmitted on a timer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748PeerHasNoRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L277). **positive:** `unit/verify` [`TestRFC3748PeerNeverRetransmitsOnATimer`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_peer_timer_test.go#L29). **negative:** `unit/verify` [`TestRFC3748PeerAnswersNoInvalidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L201) |
| `RFC3748-4.1-2` | If a peer receives a valid duplicate Request for which it has already sent a Response, it MUST resend its original Response without reprocessing the Request. (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** duplicate-Request handling is delegated to the IKEv2 carrier -- on timeout the initiator (EAP peer) re-sends its last IKE_AUTH message, which carries its last EAP Response (sa.LastSentMsg, internal/component/ike/engine/fsm.go:138); the EAP peer state machine holds no per-Request retransmission state |
| `RFC3748-4.2-1` | Because the Success and Failure packets are not acknowledged, they are not retransmitted by the authenticator (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748SuccessFailureNotRetransmitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L141). **negative:** no negative test. **{single-polarity}:** the authenticator Session emits Success or Failure once and then enters a terminal state where Process returns nil (internal/core/eap/eap.go:186), so the EAP layer never retransmits them; there is no negative case |
| `RFC3748-4.2-2` | Success and Failure packets MUST NOT contain additional data. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748SuccessAndFailureCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L332). **positive:** `unit/verify` [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L110). **negative:** `unit/verify` [`TestRFC3748SuccessAndFailureCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L341) |
| `RFC3748-4.2-4` | The Identifier field MUST match the Identifier field of the Response packet that it is sent in response to. (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L24). **positive:** `unit/verify` [`TestFailureIdentifierMatchesResponseOnNAK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L59). **positive:** `unit/verify` [`TestIdentityFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L94). **positive:** `unit/verify` [`TestSuccessIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L163). **negative:** no negative test. **{single-polarity}:** this obligation is on the SENDER. The authenticator's terminal packets are produced by Session.failure and the result.Done arm of Session.handleMethod (internal/core/eap/eap.go), and both now stamp the answered Response's Identifier. A negative case would need a RECEIVER that discards a mismatched Success or Failure, which Section 4.2 does not require of a sender and which ze's PeerSession.Process (internal/core/eap/peer.go) does not do: it switches on request.Code alone |
| `RFC3748-2.3-1` | Compliant pass- through authenticator implementations MUST by default forward EAP packets of any Type. (S2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authenticator always terminates the EAP method locally (eap.Session, internal/core/eap/eap.go:130); there is no AAA/RADIUS back end in the IKE engine, so it never operates as a pass-through |
| `RFC3748-3.1-1` | Lower layer transports for EAP MUST preserve ordering between a source and destination at a given priority level (the ordering guarantee provided by [IEEE-802]). (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** in-order delivery is a lower-layer obligation; ze carries EAP only inside IKEv2, whose message-ID sequencing delivers each IKE_AUTH request/response in order (RFC 7296 Section 2.3; internal/component/ike/engine/msgid.go:76), so the EAP framework code neither provides nor can violate it |
| `RFC3748-3.1-2` | While EAP does not assume that the lower layer is reliable, it does rely on lower layer error detection (e.g., CRC, Checksum, MIC, etc.). (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** bit-error detection is a lower-layer obligation; ze's EAP packets travel inside the IKEv2 SK payload whose AEAD/ICV check rejects any corrupted frame on decrypt (internal/component/ike/engine/fsm.go:658), so the EAP framework code adds no CRC of its own |
| `RFC3748-3.1-3` | EAP is capable of functioning on lower layers that provide an EAP MTU size of 1020 octets or greater. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the 1020-octet minimum MTU is a lower-layer obligation; ze carries EAP inside IKEv2 SK payloads over UDP, which admit EAP packets far larger than 1020 octets, and EAP-TLS method data is itself fragmented in 1024-octet chunks (internal/core/eap/eap_tls.go:28) |
| `RFC3748-5.7-1` | An implementation that supports the Expanded attribute MUST treat EAP Types that are less than 256 equivalently, whether they appear as a single octet or as the 32-bit Vendor-Type within an Expanded Type where Vendor-Id is 0. (§5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze offers MD5-Challenge (4), EAP-TLS (13) and EAP-MSCHAPv2 (26); NewSession rejects every other type (NewSession, internal/core/eap/eap.go) and the codec never encodes or parses an Expanded Type (254) packet or its Vendor-Id field, so no Vendor-Id namespace rule can bind. The peer reads TypeExpandedEAP only to route a Type-254 Request to the legacy Nak that Section 5.7 prescribes for a peer not equipped to interpret it (PeerSession.naks, internal/core/eap/peer.go), and that Nak carries no Vendor-Id |
| `RFC3748-7.10-2` | an EAP method supporting key derivation MUST export a Master Session Key (MSK) of at least 64 octets, and an Extended Master Session Key (EMSK) of at least 64 octets. (§7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748EAPTLSExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L109). **positive:** `unit/verify` [`TestRFC3748MSCHAPv2ExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L92). **positive:** `unit/verify` [`TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L91). **positive:** `unit/verify` [`TestRFC3748MSKSize`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L312). **negative:** `unit/verify` [`TestRFC3748NoEMSKWithoutACompletedExport`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L193). **negative:** `unit/verify` [`TestRFC3748NoMSCHAPv2EMSKWithoutASuccessfulExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L154) |
| `RFC3748-4-4` | Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored upon reception. (S4, S4.1) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L106). **positive:** `unit/verify` [`TestRFC3748PaddingPastLengthIsNotData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L393). **negative:** `unit/verify` [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L118). **negative:** `unit/verify` [`TestRFC3748PaddingPastLengthIsNotData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L414) |
| `RFC3748-4.1-3` | The Identifier field MUST be the same if a Request packet is retransmitted due to a timeout while waiting for a Response. Any new (non-retransmission) Requests MUST modify the Identifier field. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L98). **positive:** `unit/verify` [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L136). **negative:** `unit/verify` [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L147) |
| `RFC3748-4.1-4` | The Identifier field of the Response MUST match that of the currently outstanding Request. (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L168). **negative:** `unit/verify` [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L179) |
| `RFC3748-4.1-5` | The Type field of a Response MUST either match that of the Request, or correspond to a legacy or Expanded Nak (see Section 5.3) indicating that a Request Type is unacceptable to the peer. (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L205). **negative:** `unit/verify` [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L216) |
| `RFC3748-4.2-5` | On the peer, once the method completes unsuccessfully (that is, either the authenticator sends a failure result indication, or the peer decides that it does not want to continue the conversation, possibly after sending a failure result indication), the peer MUST terminate the conversation and indicate failure to the lower layer. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerEndsAConversationItRefuses`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L286). **positive:** `unit/verify` [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L250). **negative:** `unit/verify` [`TestRFC3748PeerEndsAConversationItRefuses`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L315). **negative:** `unit/verify` [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L265) |
| `RFC3748-4.2-6` | If the authenticator has not sent a result indication, and the peer is willing to continue the conversation, the peer waits for a Success or Failure packet once the method completes, and MUST NOT silently discard either of them. (S4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L284). **negative:** `unit/verify` [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L296) |
| `RFC3748-4-5` | Since EAP only defines Codes 1-4, EAP packets with other codes MUST be silently discarded by both authenticators and peers. (S4) | MUST | 4 | **positive:** `unit/verify` [`TestEngineDiscardedEAPCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_discard_admission_test.go#L22). **positive:** `unit/verify` [`TestRFC3748DiscardedCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_role_budget_test.go#L12). **positive:** `unit/verify` [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L223). **negative:** `unit/verify` [`TestEngineDiscardedEAPCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_discard_admission_test.go#L25). **negative:** `unit/verify` [`TestRFC3748DiscardedCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_role_budget_test.go#L13). **negative:** `unit/verify` [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L263) |
| `RFC3748-4.2-7` | By default, an EAP peer MUST silently discard a "canned" Success packet (a Success packet sent immediately upon connection). (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L82). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L103) |
| `RFC3748-4.2-8` | A peer EAP implementation receiving a Success or Failure packet where sending one is not explicitly permitted MUST silently discard it. (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L258). **positive:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L132). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L270). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L145) |
| `RFC3748-4.2-9` | On the peer, after success result indications have been exchanged by both sides, a Failure packet MUST be silently discarded (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L178). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L199) |
| `RFC3748-2-3` | The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestRFC3748NoTerminalPacketWhileTheRequestStands`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_retransmission_test.go#L33). **negative:** `unit/verify` [`TestRFC3748NoTerminalPacketWhileTheRequestStands`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_retransmission_test.go#L86) |
| `RFC3748-2.2-1` | Given these considerations, the Success, Failure, Nak Response(s), and Notification Request/Response messages MUST NOT be used to carry data destined for delivery to other EAP methods. (S2.2) | MUST NOT | 2.2 | **positive:** `unit/verify` [`TestRFC3748FrameworkMessagesReachNoMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L292). **positive:** `unit/verify` [`TestRFC3748NoFrameworkMessageReachesAnEAPMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L181). **positive:** `unit/verify` [`TestRFC3748TheAuthenticatorSendsNoNotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L281). **negative:** `unit/verify` [`TestRFC3748ANotificationRequestCarriesNothingToTheMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L303). **negative:** `unit/verify` [`TestRFC3748FrameworkMessagesReachNoMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L330). **negative:** `unit/verify` [`TestRFC3748NoFrameworkMessageReachesAnEAPMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L214) |
| `RFC3748-4.1-6` | Additional Request packets MUST be sent until a valid Response packet is received, an optional retry counter expires, or a lower layer failure indication is received (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748OutstandingRequestStandsUntilAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L126). **negative:** `unit/verify` [`TestRFC3748OutstandingRequestStandsUntilAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L141) |
| `RFC3748-4.1-7` | The peer MUST send a Response packet in reply to a valid Request packet (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L100). **positive:** `unit/verify` [`TestRFC3748PeerAnswersOnlyAValidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L163). **negative:** `unit/verify` [`TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L117). **negative:** `unit/verify` [`TestRFC3748PeerAnswersOnlyAValidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L179) |
| `RFC3748-4.1-8` | Requests MUST be processed in the order that they are received, and MUST be processed to their completion before inspecting the next Request (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748PeerReadsEachRequestAgainstItsPredecessors`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L140). **positive:** `unit/verify` [`TestRFC3748RequestsAreProcessedInOrderToCompletion`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L208). **negative:** `unit/verify` [`TestRFC3748PeerReadsEachRequestAgainstItsPredecessors`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L157). **negative:** `unit/verify` [`TestRFC3748RequestsAreProcessedInOrderToCompletion`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L225) |
| `RFC3748-4.1-9` | A single Type MUST be specified for each EAP Request or Response (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748OnePacketCarriesOneType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L248). **negative:** `unit/verify` [`TestRFC3748OnePacketCarriesOneType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L274) |
| `RFC3748-4.1-10` | An authenticator receiving a Response whose Identifier value does not match that of the currently outstanding Request MUST silently discard the Response (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAuthenticatorDiscardsAResponseAnsweringNoOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L192). **negative:** `unit/verify` [`TestAuthenticatorProcessesAResponseAnsweringTheOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L213) |
| `RFC3748-4.1-11` | An EAP server receiving a Response not meeting these requirements MUST silently discard it. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAuthenticatorDiscardsAResponseOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L278). **positive:** `unit/verify` [`TestRFC3748AuthenticatorDiscardsANakAfterTheInitialNonNakResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L169). **negative:** `unit/verify` [`TestAuthenticatorProcessesAResponseOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L305) |
| `RFC3748-2.1-4` | Once a peer has sent a Response of the same Type as the initial Request, an authenticator MUST NOT send a Request of a different Type prior to completion of the final round of a given method (with the exception of a Notification-Request) and MUST NOT send a Request for an additional method of any Type after completion of the initial authentication method; a peer receiving such Requests MUST treat them as invalid, and silently discard them. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPeerDiscardsARequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L328). **positive:** `unit/verify` [`TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L147). **negative:** `unit/verify` [`TestPeerProcessesARequestOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L368). **negative:** `unit/verify` [`TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L165) |
| `RFC3748-4.2-15` | the peer MUST terminate the conversation and indicate failure to the lower layer. The peer MUST silently discard Success packets and MAY silently discard Failure packets. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L316). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L341) |
| `RFC3748-4.2-10` | Success and Failure packets MUST NOT be sent by an EAP authenticator if the specification of the given method does not explicitly permit the method to finish at that point (S4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC3748NoTerminalPacketLeavesMidMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L94). **negative:** `unit/verify` [`TestRFC3748NoTerminalPacketLeavesMidMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L110) |
| `RFC3748-4.2-11` | Because the Success and Failure packets are not acknowledged, they are not retransmitted by the authenticator, and may be potentially lost. A peer MUST allow for this circumstance as described in this note. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerConcludesFailureWithoutTheFailurePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L238). **positive:** `unit/verify` [`TestRFC3748PeerOutlivesALostTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L284). **negative:** `unit/verify` [`TestRFC3748PeerOutlivesALostTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L255) |
| `RFC3748-4.2-12` | After the authenticator sends a failure result indication to the peer, regardless of the response from the peer, it MUST subsequently send a Failure packet (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748FailureFollowsTheFailureIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L129). **negative:** `unit/verify` [`TestRFC3748FailureFollowsTheFailureIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L155) |
| `RFC3748-4.2-13` | After the authenticator sends a success result indication to the peer and receives a success result indication from the peer, it MUST subsequently send a Success packet (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748SuccessFollowsBothSuccessIndications`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L177). **negative:** `unit/verify` [`TestRFC3748SuccessFollowsBothSuccessIndications`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L196) |
| `RFC3748-4.2-14` | If the peer attempts to authenticate to the authenticator and fails to do so, the authenticator MUST send a Failure packet and MUST NOT grant access by sending a Success packet (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748FailedAuthenticationIsRefusedNotGranted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L215). **negative:** `unit/verify` [`TestRFC3748FailedAuthenticationIsRefusedNotGranted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L231) |
| `RFC3748-7.10-5` | The MSK and EMSK MUST NOT be used directly to protect data (S7.10) | MUST NOT | 7.10 | **positive:** `unit/verify` [`TestRFC3748TheEAPMSKNeverKeysTheDataItProtects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_msk_test.go#L114). **negative:** `unit/verify` [`TestRFC3748TheEAPMSKNeverKeysTheDataItProtects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_msk_test.go#L148) |
| `RFC3748-7.10-6` | The EMSK is reserved for future use and MUST remain on the EAP peer and EAP server where it is derived; it MUST NOT be transported to, or shared with, additional parties, or used to derive any other keys. (S7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748EAPTLSExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L159). **positive:** `unit/verify` [`TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L96). **positive:** `unit/verify` [`TestRFC3748TheEMSKStaysOnBothEndsThatDerivedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L246). **positive:** `unit/verify` [`TestRFC3748TheMSCHAPv2EMSKStaysWhereItWasDerived`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L207). **negative:** `unit/verify` [`TestRFC3748TheEMSKIsNeverHandedOutward`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L284). **negative:** `unit/verify` [`TestRFC3748TheMSCHAPv2EMSKIsNeverHandedOutward`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L245) |
| `RFC3748-7.10-7` | Since EAP does not provide for explicit key lifetime negotiation, EAP peers, authenticators, and authentication servers MUST be prepared for situations in which one of the parties discards the key state, which remains valid on another party. (§7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748AnExchangeOutlivesDiscardedKeyState`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_key_state_test.go#L54). **negative:** `unit/verify` [`TestRFC3748AnExchangeOutlivesDiscardedKeyState`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_key_state_test.go#L40) |
| `RFC3748-5-1` | Nak (Type 3) or Expanded Nak (Type 254) are valid only for Response packets, they MUST NOT be sent in a Request. (S5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L313). **negative:** `unit/verify` [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L323) |
| `RFC3748-5-2` | All EAP implementations MUST support Types 1-4, which are defined in this document (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3748PeerSupportsTypesOneToFour`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L320). **negative:** `unit/verify` [`TestRFC3748PeerRefusesATypeOutsideOneToFour`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L391) |
| `RFC3748-5.2-1` | The peer MUST respond to a Notification Request with a Notification Response unless the EAP authentication method specification prohibits the use of Notification messages. (S5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestPeerAnswersANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L89). **negative:** `unit/verify` [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L147) |
| `RFC3748-5.2-2` | In any case, a Nak Response MUST NOT be sent in response to a Notification Request. (S5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L153). **negative:** `unit/verify` [`TestPeerNaksAnAuthenticationTypeButNotANotification`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L179) |
| `RFC3748-5.3.1-1` | Where a peer receives a Request for an unacceptable authentication Type (4-253,255), or a peer lacking support for Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST be sent (S5.3.1, S5.7) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestPeerNaksAnExpandedTypeRequestWithALegacyNak`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L259). **positive:** `unit/verify` [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L140). **negative:** `unit/verify` [`TestPeerDoesNotNakATypeItHandles`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L179) |
| `RFC3748-5.3.1-2` | The Type-Data field of the Nak Response (Type 3) MUST contain one or more octets indicating the desired authentication Type(s), one octet per Type, or the value zero (0) to indicate no proposed alternative (S5.3.1) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L147). **negative:** `unit/verify` [`TestNakNamesTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L230) |
| `RFC3748-5.3.1-3` | The Identifier field of a legacy Nak Response MUST match the Identifier field of the Request packet that it is sent in response to (S5.3.1) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L289). **negative:** `unit/verify` [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L304) |
| `RFC3748-5.3.1-4` | Since the legacy Nak Type is valid only in Responses and has very limited functionality, it MUST NOT be used as a general purpose error indication, such as for communication of error messages, or negotiation of parameters specific to a particular EAP method. (S5.3.1) | MUST NOT | 5.3.1 | **positive:** `unit/verify` [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L324). **negative:** `unit/verify` [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L347) |
| `RFC3748-5.4-1` | The Request contains a "challenge" message to the peer. A Response MUST be sent in reply to the Request. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L431). **positive:** `unit/verify` [`TestRFC3748MD5ChallengeRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L117). **negative:** `unit/verify` [`TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L454) |
| `RFC3748-5.4-2` | EAP peer and EAP server implementations MUST support the MD5- Challenge mechanism. (S5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC3748MD5ChallengeServerRefusesAWrongValue`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L472). **positive:** `unit/verify` [`TestRFC3748MD5ChallengeSupportedByBothRoles`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L239). **negative:** `unit/verify` [`TestRFC3748MD5ChallengeServerRefusesAWrongValue`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L490) |
| `RFC3748-5.1-2` | The Identity Response field MUST NOT be null terminated (S5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L366). **negative:** `unit/verify` [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L374) |
| `RFC3748-7.5-1` | If a per- packet MIC is employed within an EAP method, then peers, authentication servers, and authenticators not operating in pass- through mode MUST validate the MIC. (S7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L473). **positive:** `unit/verify` [`TestRFC3748TheServerValidatesThePeersMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L571). **negative:** `unit/verify` [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L486). **negative:** `unit/verify` [`TestRFC3748TheServerValidatesThePeersMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L581) |
| `RFC3748-7.10-4` | EAP Methods deriving keys MUST provide for mutual authentication between the EAP peer and the EAP Server. (S7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L505). **positive:** `unit/verify` [`TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L595). **negative:** `unit/verify` [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L520). **negative:** `unit/verify` [`TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L608) |
| `RFC3748-5.1-1` | EAP Methods SHOULD include a method-specific mechanism for obtaining the identity, so that they do not have to rely on the Identity Response. Identity Requests and Responses are sent in cleartext, so an attacker may snoop on the identity, or even modify or spoof identity exchanges. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-7.4-1` | This attack may be mitigated by the following measures: [a] Requiring mutual authentication within EAP tunneling mechanisms. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-3` | To improve reliability, if a peer receives a lower layer success indication as defined in Section 7.2, it MAY conclude that a Success packet has been lost, and behave as if it had actually received a Success packet. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-2.3-2` | Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3748-4.1-2`](#rfc3748-4.1-2) If a peer receives a valid duplicate Request for which it has already sent a Response, it MUST resend its original Response without reprocessing the Request. (S4.1) | no test | no test carries this requirement id; annotated {not-applicable}: duplicate-Request handling is delegated to the IKEv2 carrier -- on timeout the initiator (EAP peer) re-sends its last IKE_AUTH message, which carries its last EAP Response (sa.LastSentMsg, internal/component/ike/engine/fsm.go:138); the EAP peer state machine holds no per-Request retransmission state |
| [`RFC3748-2.3-1`](#rfc3748-2.3-1) Compliant pass- through authenticator implementations MUST by default forward EAP packets of any Type. (S2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authenticator always terminates the EAP method locally (eap.Session, internal/core/eap/eap.go:130); there is no AAA/RADIUS back end in the IKE engine, so it never operates as a pass-through |
| [`RFC3748-3.1-1`](#rfc3748-3.1-1) Lower layer transports for EAP MUST preserve ordering between a source and destination at a given priority level (the ordering guarantee provided by [IEEE-802]). (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: in-order delivery is a lower-layer obligation; ze carries EAP only inside IKEv2, whose message-ID sequencing delivers each IKE_AUTH request/response in order (RFC 7296 Section 2.3; internal/component/ike/engine/msgid.go:76), so the EAP framework code neither provides nor can violate it |
| [`RFC3748-3.1-2`](#rfc3748-3.1-2) While EAP does not assume that the lower layer is reliable, it does rely on lower layer error detection (e.g., CRC, Checksum, MIC, etc.). (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: bit-error detection is a lower-layer obligation; ze's EAP packets travel inside the IKEv2 SK payload whose AEAD/ICV check rejects any corrupted frame on decrypt (internal/component/ike/engine/fsm.go:658), so the EAP framework code adds no CRC of its own |
| [`RFC3748-3.1-3`](#rfc3748-3.1-3) EAP is capable of functioning on lower layers that provide an EAP MTU size of 1020 octets or greater. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the 1020-octet minimum MTU is a lower-layer obligation; ze carries EAP inside IKEv2 SK payloads over UDP, which admit EAP packets far larger than 1020 octets, and EAP-TLS method data is itself fragmented in 1024-octet chunks (internal/core/eap/eap_tls.go:28) |
| [`RFC3748-5.7-1`](#rfc3748-5.7-1) An implementation that supports the Expanded attribute MUST treat EAP Types that are less than 256 equivalently, whether they appear as a single octet or as the 32-bit Vendor-Type within an Expanded Type where Vendor-Id is 0. (§5.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze offers MD5-Challenge (4), EAP-TLS (13) and EAP-MSCHAPv2 (26); NewSession rejects every other type (NewSession, internal/core/eap/eap.go) and the codec never encodes or parses an Expanded Type (254) packet or its Vendor-Id field, so no Vendor-Id namespace rule can bind. The peer reads TypeExpandedEAP only to route a Type-254 Request to the legacy Nak that Section 5.7 prescribes for a peer not equipped to interpret it (PeerSession.naks, internal/core/eap/peer.go), and that Nak carries no Vendor-Id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3748-4-1`](#rfc3748-4-1)

A message with the Length field set to a value larger than the number of received octets MUST be silently discarded. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: a Length larger than the received octets; the message MUST be silently discarded. Both roles are driven through the engine's real entry points with an IKE layer that is valid (checksum verifies, generic header states the true length), so the one fault is the EAP Length. Authenticator (TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength, handleResponderInbound): the overlong EAP Response leaves StateEAPInProgress and writes no datagram (rtxExpectSilence), then the real round-2 request at the same Message ID is processed and answered. Peer (TestRFC3748PeerDiscardsAnOverlongEAPLength, handleInbound): an overlong EAP Request on the first IKE_AUTH response and on round 2 leaves state, NextMsgID and LastSentMsg unchanged, then the real message is processed. Producer: wire/payload_eap.go returns ErrEAPLengthExceedsData, engine/eap_auth.go::eapMessageDiscarded, the three callers return before any state change. Both tests went red on the pre-fix code (StateDead). The four records are panic breaks of eapMessageDiscarded (reach only); the targeted discrimination is the pre-fix red. core/eap TestRFC3748PacketLengthDiscard still covers only the decode error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestRFC3748PeerDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L159) | unit/verify | revert, verified |
| negative | [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsAnOverlongEAPLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_length_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L73) | unit/verify | revert, verified |

### [`RFC3748-4-2`](#rfc3748-4-2)

The Length field is two octets and indicates the length, in octets, of the EAP packet including the Code, Identifier, Length, and Data fields. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a Request with Type and 3 data octets encodes to 8 octets with Length 8, and decodes back. Negative: the same packet with Length 4 (Data only) or 3 (Type-Data only) is refused by DecodePacket; a decoder reading Length as data length would accept both, so the pair discriminates the header-inclusive reading.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748LengthCountsTheWholePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L375) | unit/verify | revert, verified |
| negative | [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC3748LengthCountsTheWholePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L357) | unit/verify | revert, verified |
| positive | [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L89) | unit/verify | revert, verified |

### [`RFC3748-2-1`](#rfc3748-2-1)

EAP is a 'lock step' protocol, so that other than the initial Request, a new Request cannot be sent prior to receiving a valid Response. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (row marker). TestRFC3748NoNewRequestBeforeAValidResponse: after Responses with another Identifier and another Type drew nil, only the peer's valid Response draws the next packet, under a new Identifier, and its replay draws nil (eap.go Session.Process Identifier check). The legacy TestRFC3748AuthenticatorLockStep tag still cannot fail on its own; the verdict rests on the new unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestRFC3748AuthenticatorLockStep`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L168) | unit/verify | revert, verified |

### [`RFC3748-2-2`](#rfc3748-2-2)

other than the initial Request, a new Request cannot be sent prior to receiving a valid Response. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 3748 in full, especially Section 2 [3] (no new Request before a valid Response), Sections 2.2-2.3 role routing and 4.1 Identifier/Type validity, and every tagged unit. TestRFC3748AuthenticatorRequiresValidResponse now supplies a genuine matching Identity Response and requires a non-nil CodeRequest of TypeMSCHAPv2; it does not expect a wrong-role Request to draw Failure. TestRFC3748NoNewRequestBeforeAValidResponse separately supplies a wrong-Identifier Response and a wrong-Type Response while the Challenge is outstanding and requires nil for each; the real peer then answers the retained Challenge, requires a next packet with a changed Identifier if it is a Request, and requires nil on replay. Session.Process checks CodeResponse and the outstanding Identifier before state dispatch; handleMethod checks the configured Type before calling the method. The malformed method payload in the wrong-Identifier probe cannot independently satisfy its nil assertion: mschapv2Method.handleResponse returns Err on that short payload and Session.handleMethod would emit Failure, which the assertion rejects. The wrong-Type probe likewise reaches a non-nil Failure if the framework Type discard is removed. handleIdentity, handleMethod and finalRequest emit their next Requests only while processing the admitted Response; handleResponderEAP consumes nil as no output and forwards non-nil packets through sendResponderEAP. Retain enforced for this lock-step obligation; this source rejudgment is not an executed test or named-peer result.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC3748NoNewRequestBeforeAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L209) | unit/verify | revert, verified |

### [`RFC3748-2.1-1`](#rfc3748-2.1-1)

the peer and authenticator MUST utilize only one authentication method (Type 4 or greater) within an EAP conversation (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Authenticator clause: TestRFC3748OneMethodPerConversation (a Type-13 Response once MS-CHAPv2 is active draws nil, no success; unproven, no record). Peer clause: TestRFC3748ThePeerKeepsToOneMethod: an EAP-TLS Request to a peer mid-MS-CHAPv2 is discarded (no Response, no Err), and the same peer then acks the MS-CHAPv2 Success with Type 26 and concludes. Records are whole-body panics of handleRequest, so Q2 for the discard is answered by reading, not by the record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748ThePeerKeepsToOneMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L229) | unit/verify | revert, verified |
| negative | [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC3748ThePeerKeepsToOneMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L223) | unit/verify | revert, verified |

### [`RFC3748-2.1-2`](#rfc3748-2.1-2)

However, the peer and authenticator MUST utilize only one authentication method (Type 4 or greater) within an EAP conversation, after which the authenticator MUST send a Success or Failure packet. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both clauses: every method Request of a full MS-CHAPv2 flight carries Type 26 (wantOneMethodRequests) and the flight ends in EAP-Success; the wrong-password flight also stays on Type 26 and ends in EAP-Failure with Succeeded false; after either result a Type-4 Response draws nil. Positive and negative are distinct inputs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748OneMethodThenTheResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L210) | unit/verify | revert, verified |
| negative | [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L265) | unit/verify | revert, verified |
| positive | [`TestRFC3748OneMethodThenTheResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L255) | unit/verify | revert, verified |

### [`RFC3748-2.1-3`](#rfc3748-2.1-3)

A peer MUST NOT send a Nak (legacy or expanded) in reply to a Request after an initial non-Nak Response has been sent. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a peer answering a Request with a legacy or expanded Nak after its first non-Nak Response. TestRFC3748PeerNaksBeforeItCommitsToAMethod negative: after commitment a Type-40 Request must be Discarded and fails on res.Response != nil (any Response, so both Nak forms) or res.Err. Positive: before commitment the same Type-40 Request draws a legacy Nak naming MS-CHAPv2 (wantLegacyNak), so the discard is tied to commitment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L372) | unit/verify | revert, verified |

### [`RFC3748-4.1-1`](#rfc3748-4.1-1)

Responses MUST only be sent in reply to a valid Request and never be retransmitted on a timer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. 'Only in reply to a valid Request': TestRFC3748PeerAnswersNoInvalidRequest (a Request of another method mid-method and a method Request after Success each discarded, a valid Request answered). 'Never retransmitted on a timer': TestRFC3748PeerNeverRetransmitsOnATimer walks PeerSession for a time.Timer, time.Ticker or chan time.Time (none), checks Process is the only method returning *Packet or PeerResult, and shows an answered peer handed an early EAP-Success returns no Response and marks it Discarded. PeerSession has no transport, so Process is its only send path. Limit: the reflection does not look for a func-typed send callback; adding one would also add a send path the method-set check misses. All units recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerAnswersNoInvalidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerNeverRetransmitsOnATimer`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_peer_timer_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerHasNoRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L277) | unit/verify | revert, verified |

### [`RFC3748-4.1-2`](#rfc3748-4.1-2)

If a peer receives a valid duplicate Request for which it has already sent a Response, it MUST resend its original Response without reprocessing the Request. (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-2, so no unit is bound to it.

### [`RFC3748-4.2-1`](#rfc3748-4.2-1)

Because the Success and Failure packets are not acknowledged, they are not retransmitted by the authenticator (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} carried on the row. Forbidden: the authenticator re-sending Success or Failure. TestRFC3748SuccessFailureNotRetransmitted fails on a non-nil packet from Process after a Failure (Nak path) and after a Success (full MS-CHAPv2 flight).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748SuccessFailureNotRetransmitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L141) | unit/verify | unproven |

### [`RFC3748-4.2-2`](#rfc3748-4.2-2)

Success and Failure packets MUST NOT contain additional data. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: Encode of a Success and of a Failure carrying Type 26 and Type-Data yields exactly code,id,00,04. Negative: a received Success/Failure whose Length covers 3 data octets is delivered with Type 0 and nil Type-Data (DecodePacket). The sender obligation is fully proven by the exact 4-octet encode; the neighbouring Request-needs-Type arm is untagged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748SuccessAndFailureCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L341) | unit/verify | revert, verified |
| positive | [`TestRFC3748SuccessAndFailureCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L332) | unit/verify | revert, verified |
| positive | [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L110) | unit/verify | revert, verified |

### [`RFC3748-4.2-4`](#rfc3748-4.2-4)

The Identifier field MUST match the Identifier field of the Response packet that it is sent in response to. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read the full RFC 3748 text and Section 4.2 Identifier rule, all four tagged test units, Session.Process/handleIdentity/handleMethod/finalRequest/failure, and the real MS-CHAPv2 producer and peer routing. The sender-only single-polarity annotation remains appropriate: the forbidden outcome is a terminal packet whose Identifier differs from the Response it answers, not a receiver acceptance rule. TestFailureIdentifierMatchesResponse now drives real wrong credentials through the method Failure Request, the peer acknowledgement and stateLastWord, requires CodeFailure within eight rounds, and compares its Identifier exactly with that actual acknowledgement Response (the previous refused-code 0x42 fixture no longer describes this unit). TestFailureIdentifierMatchesResponseOnNAK pins CodeFailure and Identifier 0x7E; TestIdentityFailureIdentifierMatchesResponse pins CodeFailure and 0x11 on the existing identity refusal branch; TestSuccessIdentifierMatchesResponse pins CodeSuccess and 0x5A on the Done branch. These assertions reject the former terminal Identifier increment. failure copies answered.Identifier, and the Done arm stamps response.Identifier; the IKE consumer forwards both terminal packet kinds without changing the EAP Identifier. The malformed identity fixture is evidence only for the Identifier on that existing terminal branch, not proof that emitting Failure for a wrong Type complies with the separate Section 4.1 discard obligation. Retain enforced for the scoped sender Identifier rule, with no live interop or test-execution claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L24) | unit/verify | unproven |
| positive | [`TestFailureIdentifierMatchesResponseOnNAK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L59) | unit/verify | unproven |
| positive | [`TestIdentityFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L94) | unit/verify | unproven |
| positive | [`TestSuccessIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L163) | unit/verify | unproven |

### [`RFC3748-2.3-1`](#rfc3748-2.3-1)

Compliant pass- through authenticator implementations MUST by default forward EAP packets of any Type. (S2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-2.3-1, so no unit is bound to it.

### [`RFC3748-3.1-1`](#rfc3748-3.1-1)

Lower layer transports for EAP MUST preserve ordering between a source and destination at a given priority level (the ordering guarantee provided by [IEEE-802]). (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-1, so no unit is bound to it.

### [`RFC3748-3.1-2`](#rfc3748-3.1-2)

While EAP does not assume that the lower layer is reliable, it does rely on lower layer error detection (e.g., CRC, Checksum, MIC, etc.). (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-2, so no unit is bound to it.

### [`RFC3748-3.1-3`](#rfc3748-3.1-3)

EAP is capable of functioning on lower layers that provide an EAP MTU size of 1020 octets or greater. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-3, so no unit is bound to it.

### [`RFC3748-5.7-1`](#rfc3748-5.7-1)

An implementation that supports the Expanded attribute MUST treat EAP Types that are less than 256 equivalently, whether they appear as a single octet or as the 32-bit Vendor-Type within an Expanded Type where Vendor-Id is 0. (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-5.7-1, so no unit is bound to it.

### [`RFC3748-7.10-2`](#rfc3748-7.10-2)

an EAP method supporting key derivation MUST export a Master Session Key (MSK) of at least 64 octets, and an Extended Master Session Key (EMSK) of at least 64 octets. (§7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Unchanged judgement after a comment-only edit to TestRFC3748EAPTLSExportsASixtyFourOctetEMSK (new 7.10-6 tag): MSK and EMSK values, not sizes, on both methods; EMSK is export octets 64-127 (EAP-TLS) or the HKDF expansion (MS-CHAPv2), equal on both ends; negatives show no EMSK without a completed export or successful exchange. All units recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoEMSKWithoutACompletedExport`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L193) | unit/verify | revert, verified |
| negative | [`TestRFC3748NoMSCHAPv2EMSKWithoutASuccessfulExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC3748EAPTLSExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC3748MSCHAPv2ExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC3748MSKSize`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_test.go#L312) | unit/verify | revert, verified |

### [`RFC3748-4-4`](#rfc3748-4-4)

Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored upon reception. (S4, S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: an Identity Response with 3 octets past Length decodes with Type-Data 'user', and Session.Process records identity 'user' and draws the method Request. Negative: the same octets with Length raised to cover them are delivered as 'userXYZ', so the positive reads the Length field, not the buffer. Distinct inputs, unlike the legacy TestRFC3748LengthBoundsTypeData pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PaddingPastLengthIsNotData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L414) | unit/verify | revert, verified |
| negative | [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L118) | unit/verify | revert, verified |
| positive | [`TestRFC3748PaddingPastLengthIsNotData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L393) | unit/verify | revert, verified |
| positive | [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L106) | unit/verify | revert, verified |

### [`RFC3748-4.1-3`](#rfc3748-4.1-3)

The Identifier field MUST be the same if a Request packet is retransmitted due to a timeout while waiting for a Response. Any new (non-retransmission) Requests MUST modify the Identifier field. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 2 (new Requests modify the Identifier): TestRFC3748NewRequestChangesIdentifier, each Request differs from the one before and no Identifier repeats in the exchange. Clause 1 (a retransmitted Request keeps the Identifier): Ze runs no EAP-layer retransmission timer (IKEv2 carries EAP and the IKE initiator retransmits), so the retransmission path is the replay of the cached IKE_AUTH response; TestEapRtxResponderReplaysCachedResponseMidEAP asserts the EAP Request re-sent on a retransmitted IKE_AUTH is byte-identical to the first, so its Identifier is unchanged, while a rebuild cannot match (fresh CBC IV). Record: revert responder.go::replayCachedResponse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L136) | unit/verify | revert, verified |

### [`RFC3748-4.1-4`](#rfc3748-4.1-4)

The Identifier field of the Response MUST match that of the currently outstanding Request. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a peer Response whose Identifier differs from the outstanding Request's. TestRFC3748ResponseEchoesRequestIdentifier fails on resp.Identifier != requests[i].Identifier for every Response of a full MS-CHAPv2 flight, and on an unusual Identifier 0xC3 not echoed (catches a peer that counts or uses a constant).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L179) | unit/verify | unproven |
| positive | [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L168) | unit/verify | unproven |

### [`RFC3748-4.1-5`](#rfc3748-4.1-5)

The Type field of a Response MUST either match that of the Request, or correspond to a legacy or Expanded Nak (see Section 5.3) indicating that a Request Type is unacceptable to the peer. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: the Response Type matches the Request; or it is a Nak for an unacceptable Type. Clause 1: resp.Type != requests[i].Type fails over the flight. Clause 2 and the forbidden case (answering an unacceptable Type with a non-Nak): a Type-99 Request fails on res.Err, no Response, Response.Type != TypeNAK, or Type-Data != [MS-CHAPv2].

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L205) | unit/verify | revert, verified |

### [`RFC3748-4.2-5`](#rfc3748-4.2-5)

On the peer, once the method completes unsuccessfully (that is, either the authenticator sends a failure result indication, or the peer decides that it does not want to continue the conversation, possibly after sending a failure result indication), the peer MUST terminate the conversation and indicate failure to the lower layer. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both triggers: authenticator failure indication on TestRFC3748PeerEndsAnUnsuccessfulConversation (unproven legacy); peer-decides trigger on TestRFC3748PeerEndsAConversationItRefuses: a wrong Authenticator Response gives Err, no ack, a following EAP-Success concludes nothing with zero MSK and Succeeded false (peer.go handleMSCHAPv2Success sets peerStateFailed, Process discards). Negative: the matching value is acked and concludes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerEndsAConversationItRefuses`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L315) | unit/verify | revert, verified |
| negative | [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L265) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerEndsAConversationItRefuses`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L286) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L250) | unit/verify | revert, verified |

### [`RFC3748-4.2-6`](#rfc3748-4.2-6)

If the authenticator has not sent a result indication, and the peer is willing to continue the conversation, the peer waits for a Success or Failure packet once the method completes, and MUST NOT silently discard either of them. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the waiting peer silently discarding the Success or the Failure. Success: peerFinal.Done false or an all-zero MSK fails. Failure: peerFinal.Err nil or Done true fails. Both packets of the quoted 'either of them' are covered.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L296) | unit/verify | unproven |
| positive | [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L284) | unit/verify | unproven |

### [`RFC3748-4-5`](#rfc3748-4-5)

Since EAP only defines Codes 1-4, EAP packets with other codes MUST be silently discarded by both authenticators and peers. (S4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC3748 Section 4: "Since EAP only defines Codes 1-4, EAP packets with other codes MUST be silently discarded by both authenticators and peers." The actual two-role core units check state, identifier, method, keys, round budget and legitimate controls; the engine unit injects encrypted undefined Codes before each real EAP round and finishes matching nonzero MSKs and final AUTH. Removing the initiator Discarded early return now produces the named Code-0 initiator output/retransmission failure while the legitimate retry control passes. Charging undefined Codes to peer.rounds now fails budget/exchange assertions while valid round-budget controls pass. Current Process admission rejects undefined Codes before state/method/budget access on both roles, and responder routing consumes Discarded before output/cache/message-ID advancement. The two previously requested semantic witnesses are observed; no fresh foreign EAP interoperability run is claimed. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineDiscardedEAPCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_discard_admission_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L263) | unit/verify | revert, verified |
| negative | [`TestRFC3748DiscardedCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_role_budget_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestEngineDiscardedEAPCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_eap_discard_admission_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L223) | unit/verify | revert, verified |
| positive | [`TestRFC3748DiscardedCodesPreserveExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_role_budget_test.go#L12) | unit/verify | revert, verified |

### [`RFC3748-4.2-7`](#rfc3748-4.2-7)

By default, an EAP peer MUST silently discard a "canned" Success packet (a Success packet sent immediately upon connection). (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on a Success sent at connection. A Success on a fresh peer fails on eapdWantDiscarded, on Succeeded(), and if the following Identity Request is no longer answered. Positive: the same Success at method end must conclude Done with a non-zero MSK.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L82) | unit/verify | revert, verified |

### [`RFC3748-4.2-8`](#rfc3748-4.2-8)

A peer EAP implementation receiving a Success or Failure packet where sending one is not explicitly permitted MUST silently discard it. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves. Success: TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet. Failure: TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted: after both success indications an EAP-Failure is discarded (no Err, no Response) and the following EAP-Success still concludes; mid-method, where Failure is permitted, the same packet yields ErrEAPFailure and is not discarded (peer.go Process CodeFailure arm).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L270) | unit/verify | revert, verified |
| negative | [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L132) | unit/verify | revert, verified |

### [`RFC3748-4.2-9`](#rfc3748-4.2-9)

On the peer, after success result indications have been exchanged by both sides, a Failure packet MUST be silently discarded (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L178) | unit/verify | revert, verified |

### [`RFC3748-2-3`](#rfc3748-2-3)

The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoTerminalPacketWhileTheRequestStands`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_retransmission_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestRFC3748NoTerminalPacketWhileTheRequestStands`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_retransmission_test.go#L33) | unit/verify | revert, verified |

### [`RFC3748-2.2-1`](#rfc3748-2.2-1)

Given these considerations, the Success, Failure, Nak Response(s), and Notification Request/Response messages MUST NOT be used to carry data destined for delivery to other EAP methods. (S2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. All four message kinds asserted at the receiver and the composer: a Nak Response carrying a desired-Type list ends the exchange with EAP-Failure and reaches no method (recording/tap method sees nothing), Success and Failure encode to their four octets, the peer's Notification Response carries zero octets of Type-Data, ze's authenticator sends no Notification Request, and an injected Notification carrying MS-CHAPv2 result bytes does not advance the method. A live Response of the method Type reaches the method with its Type-Data (control arm), so the silence is not a dead method layer. Every unit now carries an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748FrameworkMessagesReachNoMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L330) | unit/verify | revert, verified |
| negative | [`TestRFC3748NoFrameworkMessageReachesAnEAPMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L214) | unit/verify | revert, verified |
| negative | [`TestRFC3748ANotificationRequestCarriesNothingToTheMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L303) | unit/verify | revert, verified |
| positive | [`TestRFC3748FrameworkMessagesReachNoMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L292) | unit/verify | revert, verified |
| positive | [`TestRFC3748NoFrameworkMessageReachesAnEAPMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheAuthenticatorSendsNoNotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L281) | unit/verify | revert, verified |

### [`RFC3748-4.1-6`](#rfc3748-4.1-6)

Additional Request packets MUST be sent until a valid Response packet is received, an optional retry counter expires, or a lower layer failure indication is received (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748OutstandingRequestStandsUntilAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L141) | unit/verify | unproven |
| positive | [`TestRFC3748OutstandingRequestStandsUntilAValidResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L126) | unit/verify | unproven |

### [`RFC3748-4.1-7`](#rfc3748-4.1-7)

The peer MUST send a Response packet in reply to a valid Request packet (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerAnswersOnlyAValidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L179) | unit/verify | unproven |
| negative | [`TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L117) | unit/verify | unproven |
| positive | [`TestRFC3748PeerAnswersOnlyAValidRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L163) | unit/verify | unproven |
| positive | [`TestRFC3748PeerAnswersAValidRequestAndDiscardsAnInvalidOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L100) | unit/verify | unproven |

### [`RFC3748-4.1-8`](#rfc3748-4.1-8)

Requests MUST be processed in the order that they are received, and MUST be processed to their completion before inspecting the next Request (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748RequestsAreProcessedInOrderToCompletion`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L225) | unit/verify | unproven |
| negative | [`TestRFC3748PeerReadsEachRequestAgainstItsPredecessors`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L157) | unit/verify | unproven |
| positive | [`TestRFC3748RequestsAreProcessedInOrderToCompletion`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC3748PeerReadsEachRequestAgainstItsPredecessors`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_ordering_test.go#L140) | unit/verify | unproven |

### [`RFC3748-4.1-9`](#rfc3748-4.1-9)

A single Type MUST be specified for each EAP Request or Response (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748OnePacketCarriesOneType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L274) | unit/verify | unproven |
| positive | [`TestRFC3748OnePacketCarriesOneType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_lockstep_test.go#L248) | unit/verify | unproven |

### [`RFC3748-4.1-10`](#rfc3748-4.1-10)

An authenticator receiving a Response whose Identifier value does not match that of the currently outstanding Request MUST silently discard the Response (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthenticatorProcessesAResponseAnsweringTheOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L213) | unit/verify | unproven |
| positive | [`TestAuthenticatorDiscardsAResponseAnsweringNoOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_identifier_test.go#L192) | unit/verify | unproven |

### [`RFC3748-4.1-11`](#rfc3748-4.1-11)

An EAP server receiving a Response not meeting these requirements MUST silently discard it. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both 'these requirements' asserted at the authenticator: a Response of another Type draws nil and does not complete the exchange (exact nil, Succeeded false), a Nak after the initial non-Nak Response is discarded and the method still concludes; the method-Type control Response is processed to EAP-Success, so the discard is the Type check, not a refusal of every Response. All three units carry observed-red records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthenticatorProcessesAResponseOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestAuthenticatorDiscardsAResponseOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestRFC3748AuthenticatorDiscardsANakAfterTheInitialNonNakResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L169) | unit/verify | revert, verified |

### [`RFC3748-2.1-4`](#rfc3748-2.1-4)

Once a peer has sent a Response of the same Type as the initial Request, an authenticator MUST NOT send a Request of a different Type prior to completion of the final round of a given method (with the exception of a Notification-Request) and MUST NOT send a Request for an additional method of any Type after completion of the initial authentication method; a peer receiving such Requests MUST treat them as invalid, and silently discard them. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Authenticator MUST NOTs: every Request of a completed MS-CHAPv2 flight is Type 26 and a Type-13 Response after EAP-Success draws nil (no additional method); mid-method a Type-13 Response draws nil and the MS-CHAPv2 method continues with a Type-26 Request. Peer clause: TestPeerDiscardsARequestOfAnotherType (record); TestAuthenticatorProcessesAResponseOfTheMethodType is an unproven control. Ze's authenticator holds one configured method, so the mid-method provocation is the reachable violation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L165) | unit/verify | revert, verified |
| negative | [`TestPeerProcessesARequestOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L368) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestPeerDiscardsARequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_discard_test.go#L328) | unit/verify | revert, verified |

### [`RFC3748-4.2-15`](#rfc3748-4.2-15)

the peer MUST terminate the conversation and indicate failure to the lower layer. The peer MUST silently discard Success packets and MAY silently discard Failure packets. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: the peer terminates and indicates failure; MUST silently discard Success (the Failure discard is a MAY). Termination: the EAP-Failure fails unless res.Err is ErrEAPFailure. Success discard: a later Success fails on !Discarded, Done, a non-zero MSK or Succeeded(). Positive: a completed peer concludes on the Success (Done false or Discarded fails).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L341) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsASuccessAfterItEndedTheSession`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L316) | unit/verify | revert, verified |

### [`RFC3748-4.2-10`](#rfc3748-4.2-10)

Success and Failure packets MUST NOT be sent by an EAP authenticator if the specification of the given method does not explicitly permit the method to finish at that point (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoTerminalPacketLeavesMidMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC3748NoTerminalPacketLeavesMidMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L94) | unit/verify | revert, verified |

### [`RFC3748-4.2-11`](#rfc3748-4.2-11)

Because the Success and Failure packets are not acknowledged, they are not retransmitted by the authenticator, and may be potentially lost. A peer MUST allow for this circumstance as described in this note. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves of the note. Lost Success: the peer does not conclude before it, discards strays, concludes on the late Success. Lost Failure (TestRFC3748PeerConcludesFailureWithoutTheFailurePacket): after the MS-CHAPv2 failure indication is acknowledged and the EAP-Failure is dropped, a forged EAP-Success or a stray Request ends with ErrEAPFailure, not Done, no MSK, no Response; the producer parks pendingErr in handleMSCHAPv2Failure, and without it the forged Success would be silently discarded and the test fails. Every tagged unit carries a record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerOutlivesALostTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L255) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerConcludesFailureWithoutTheFailurePacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerOutlivesALostTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L284) | unit/verify | revert, verified |

### [`RFC3748-4.2-12`](#rfc3748-4.2-12)

After the authenticator sends a failure result indication to the peer, regardless of the response from the peer, it MUST subsequently send a Failure packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748FailureFollowsTheFailureIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestRFC3748FailureFollowsTheFailureIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L129) | unit/verify | revert, verified |

### [`RFC3748-4.2-13`](#rfc3748-4.2-13)

After the authenticator sends a success result indication to the peer and receives a success result indication from the peer, it MUST subsequently send a Success packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748SuccessFollowsBothSuccessIndications`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L196) | unit/verify | revert, verified |
| positive | [`TestRFC3748SuccessFollowsBothSuccessIndications`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L177) | unit/verify | revert, verified |

### [`RFC3748-4.2-14`](#rfc3748-4.2-14)

If the peer attempts to authenticate to the authenticator and fails to do so, the authenticator MUST send a Failure packet and MUST NOT grant access by sending a Success packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748FailedAuthenticationIsRefusedNotGranted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L231) | unit/verify | revert, verified |
| positive | [`TestRFC3748FailedAuthenticationIsRefusedNotGranted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_result_indication_test.go#L215) | unit/verify | revert, verified |

### [`RFC3748-7.10-5`](#rfc3748-7.10-5)

The MSK and EMSK MUST NOT be used directly to protect data (S7.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748TheEAPMSKNeverKeysTheDataItProtects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_msk_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheEAPMSKNeverKeysTheDataItProtects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_msk_test.go#L114) | unit/verify | revert, verified |

### [`RFC3748-7.10-6`](#rfc3748-7.10-6)

The EMSK is reserved for future use and MUST remain on the EAP peer and EAP server where it is derived; it MUST NOT be transported to, or shared with, additional parties, or used to derive any other keys. (S7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. EAP-TLS side now closed: TestRFC3748EAPTLSExportsASixtyFourOctetEMSK asserts the MSK equals export octets 0-63 exactly, TLS 1.2 and 1.3, so an MSK hashed from or mixed with the EMSK fails; MS-CHAPv2 MSK asserted octet by octet from the MPPE MasterKey alone. Remain-on-both-ends: the EMSK is held on both ends and appears in no outward value or wire octet (negatives on both methods). All units recorded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748TheEMSKIsNeverHandedOutward`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L284) | unit/verify | revert, verified |
| negative | [`TestRFC3748TheMSCHAPv2EMSKIsNeverHandedOutward`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L245) | unit/verify | revert, verified |
| positive | [`TestRFC3748EAPTLSExportsASixtyFourOctetEMSK`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheEMSKStaysOnBothEndsThatDerivedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_emsk_test.go#L246) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheMSCHAPv2EMSKStaysWhereItWasDerived`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_mschapv2_emsk_test.go#L207) | unit/verify | revert, verified |
| positive | [`TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_remaining_clause_test.go#L96) | unit/verify | revert, verified |

### [`RFC3748-7.10-7`](#rfc3748-7.10-7)

Since EAP does not provide for explicit key lifetime negotiation, EAP peers, authenticators, and authentication servers MUST be prepared for situations in which one of the parties discards the key state, which remains valid on another party. (§7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an exchange that fails when one party discarded key state the other still holds. With the authenticator's ticket keys discarded and a peer offering the stale ticket, resumption on either side fails the test (lines 67, 70) and the new MSK must differ from the kept-state MSK; control: with the state kept, both sides resume (lines 48, 51).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748AnExchangeOutlivesDiscardedKeyState`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_key_state_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC3748AnExchangeOutlivesDiscardedKeyState`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_key_state_test.go#L54) | unit/verify | revert, verified |

### [`RFC3748-5-1`](#rfc3748-5-1)

Nak (Type 3) or Expanded Nak (Type 254) are valid only for Response packets, they MUST NOT be sent in a Request. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Request of Type 3 or 254. Every Request of a full MS-CHAPv2 flight fails on req.Type == TypeNAK or TypeExpandedEAP; a Nak Response, the input a faulty authenticator would echo, must draw a Failure (out.Code != CodeFailure fails).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L323) | unit/verify | unproven |
| positive | [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L313) | unit/verify | unproven |

### [`RFC3748-5-2`](#rfc3748-5-2)

All EAP implementations MUST support Types 1-4, which are defined in this document (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerRefusesATypeOutsideOneToFour`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerSupportsTypesOneToFour`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L320) | unit/verify | revert, verified |

### [`RFC3748-5.2-1`](#rfc3748-5.2-1)

The peer MUST respond to a Notification Request with a Notification Response unless the EAP authentication method specification prohibits the use of Notification messages. (S5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no Notification Response to a Notification Request (the prohibiting-method exception applies to no method Ze runs). wantNotificationResponse fails in the identity state, on a 4000-octet message, on a zero-length and a non-UTF-8 message, and res.Discarded fails; the identity exchange must continue afterwards.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestPeerAnswersANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L89) | unit/verify | revert, verified |

### [`RFC3748-5.2-2`](#rfc3748-5.2-2)

In any case, a Nak Response MUST NOT be sent in response to a Notification Request. (S5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Nak answering a Notification Request. wantNotificationResponse (Type 2 Response) fails on each malformed Notification Request; the same peer Naks a Type-40 Request (Response.Type != TypeNAK fails) and then answers a Notification with a Notification Response, so the Notification path is distinguished from the Nak path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerNaksAnAuthenticationTypeButNotANotification`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_notification_test.go#L153) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-1`](#rfc3748-5.3.1-1)

Where a peer receives a Request for an unacceptable authentication Type (4-253,255), or a peer lacking support for Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST be sent (S5.3.1, S5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerDoesNotNakATypeItHandles`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnExpandedTypeRequestWithALegacyNak`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L140) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-2`](#rfc3748-5.3.1-2)

The Type-Data field of the Nak Response (Type 3) MUST contain one or more octets indicating the desired authentication Type(s), one octet per Type, or the value zero (0) to indicate no proposed alternative (S5.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNakNamesTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L147) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-3`](#rfc3748-5.3.1-3)

The Identifier field of a legacy Nak Response MUST match the Identifier field of the Request packet that it is sent in response to (S5.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L304) | unit/verify | revert, verified |
| positive | [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L289) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-4`](#rfc3748-5.3.1-4)

Since the legacy Nak Type is valid only in Responses and has very limited functionality, it MUST NOT be used as a general purpose error indication, such as for communication of error messages, or negotiation of parameters specific to a particular EAP method. (S5.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Nak used as an error indication. A truncated MS-CHAPv2 Challenge must return the method error with no Response (res.Response != nil fails, so no Nak carries it); control: the same peer Naks an unacceptable Type 40 (wantLegacyNak).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L347) | unit/verify | revert, verified |
| positive | [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_nak_test.go#L324) | unit/verify | revert, verified |

### [`RFC3748-5.4-1`](#rfc3748-5.4-1)

The Request contains a "challenge" message to the peer. A Response MUST be sent in reply to the Request. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: the peer answers ze's MD5-Challenge Request with a Type-4 Response carrying the Request's Identifier and the MD5 value computed outside the package. Negative: a Type-4 Request carrying no challenge (Value-Size 0, or no Type-Data) violates the row's first sentence and draws no Response and an error. The Identity Requery tag was removed; its orphan record is the stale entry rfc check reports.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L454) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L431) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L117) | unit/verify | revert, verified |

### [`RFC3748-5.4-2`](#rfc3748-5.4-2)

EAP peer and EAP server implementations MUST support the MD5- Challenge mechanism. (S5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: ze's peer and ze's server run Type 4 to EAP-Success and peer Done (two units). Negative: a peer holding another secret draws EAP-Failure and Succeeded false, so a server that accepts any Type-4 Response (no real support of the mechanism) goes red. The configured-method Nak unit is no longer tagged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MD5ChallengeServerRefusesAWrongValue`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L490) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeServerRefusesAWrongValue`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L472) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeSupportedByBothRoles`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_md5challenge_test.go#L239) | unit/verify | revert, verified |

### [`RFC3748-5.1-2`](#rfc3748-5.1-2)

The Identity Response field MUST NOT be null terminated (S5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L374) | unit/verify | unproven |
| positive | [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L366) | unit/verify | unproven |

### [`RFC3748-7.5-1`](#rfc3748-7.5-1)

If a per- packet MIC is employed within an EAP method, then peers, authentication servers, and authenticators not operating in pass- through mode MUST validate the MIC. (S7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Peer clause: TestRFC3748EAPTLSValidatesItsPerPacketMIC. Server clause: TestRFC3748TheServerValidatesThePeersMIC: a clean flight ends in EAP-Success; one octet changed in the peer's second record-bearing Response (the TLS 1.3 encrypted client flight, so the octet sits under the AEAD tag) never draws EAP-Success. Ze's authenticator is the authentication server, no pass-through mode exists.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748TheServerValidatesThePeersMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L581) | unit/verify | revert, verified |
| negative | [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L486) | unit/verify | revert, verified |
| positive | [`TestRFC3748TheServerValidatesThePeersMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L571) | unit/verify | revert, verified |
| positive | [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L473) | unit/verify | revert, verified |

### [`RFC3748-7.10-4`](#rfc3748-7.10-4)

EAP Methods deriving keys MUST provide for mutual authentication between the EAP peer and the EAP Server. (S7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both directions for both key-deriving methods. EAP-TLS: peer refuses an untrusted server (legacy walk unit); the new unit shows the server refuses an untrusted client certificate with no EAP-Success and zero MSK. MS-CHAPv2: mutual success with an MSK; wrong NT-Response gives EAP-Failure and zero authenticator MSK; wrong Authenticator Response gives peer Err and zero peer MSK. The negative record's red lands in the positive half (driveClauseFlight also calls handleMSCHAPv2Success); Q2 for each negative assertion answered by reading.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L608) | unit/verify | revert, verified |
| negative | [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L520) | unit/verify | revert, verified |
| positive | [`TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_clause_test.go#L595) | unit/verify | revert, verified |
| positive | [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc3748_walk_test.go#L505) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | rfc3748walk (extraction sign-off agent, spec rfcgate-6-supported-extraction-signoff) |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc3748.txt |
| Source fingerprint | df5b1ebf6f637eef |
| Record | rfc/extraction/rfc3748.json |
| Mapped sentences | 53 |
| Declined as scope | 50 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, abstract, status of this memo and table of contents. |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 1 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 3 | walked | not stated |
| `2.1` | not stated | 4 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 4 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `4.1` | not stated | 16 | walked | not stated |
| `4.2` | not stated | 16 | walked | not stated |
| `4.3` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 5 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 4 | walked | not stated |
| `5.3.2` | not stated | 4 | walked | not stated |
| `5.4` | not stated | 4 | walked | not stated |
| `5.5` | not stated | 4 | walked | not stated |
| `5.6` | not stated | 5 | walked | not stated |
| `5.7` | not stated | 2 | walked | not stated |
| `5.8` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | skipped (iana) | IANA Considerations: the registry actions bind IANA, not an implementation. |
| `6.1` | not stated | 0 | skipped (iana) | IANA Considerations, Packet Codes: a registry allocation policy. |
| `6.2` | not stated | 0 | skipped (iana) | IANA Considerations, Method Types: a registry allocation policy. |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 4 | walked | not stated |
| `7.2.1` | not stated | 2 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 1 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `7.8` | not stated | 0 | walked | not stated |
| `7.9` | not stated | 0 | walked | not stated |
| `7.10` | not stated | 11 | walked | not stated |
| `7.11` | not stated | 0 | walked | not stated |
| `7.12` | not stated | 0 | walked | not stated |
| `7.13` | not stated | 1 | walked | not stated |
| `7.14` | not stated | 0 | walked | not stated |
| `7.15` | not stated | 0 | walked | not stated |
| `7.16` | not stated | 2 | walked | not stated |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `9` | References | 0 | skipped (references) | References. |
| `9.1` | Normative References | 0 | skipped (references) | Normative References. |
| `9.2` | Informative References | 0 | skipped (references) | Informative References. |
| `A` | not stated | 0 | skipped (appendix-non-normative) | Appendix A, Changes from RFC 2284: a historical diff against an obsoleted document. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.2 is the Terminology list and the sentence sits inside the definition ENTRY for 'Displayable Message', fixing what the term means wherever a later section uses it. The obligations that put a displayable message on the wire are stated at Sections 5.1, 5.2, 5.5 and 5.6, and those sites carry their own dispositions. | The message encoding MUST follow the UTF-8 transformation format [RFC2279]. |
| `2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 2.1 obligation to close the conversation with a Success or a Failure, for the Failure arm. Site 4.2:1 maps the id. | [4] The conversation continues until the authenticator cannot authenticate the peer (unacceptable Responses to one or more Requests), in which case the authenticator implementation MUST transmit an EAP Failure (Code 4). |
| `2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the same Section 2.1 obligation for the Success arm. Site 4.2:1 maps the id. | Alternatively, the authentication conversation can continue until the authenticator determines that successful authentication has occurred, in which case the authenticator MUST transmit an EAP Success (Code 3). |
| `2.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of a 'tunneled' EAP method specification, a document role rather than a wire role, and the sentence states what such a specification must say about running a second method inside the tunnel. Ze publishes no EAP method specification, and neither method it runs tunnels a second one: tlsMethod carries TLS records and no EAP packet (tlsMethod.Process, internal/core/eap/eap_tls.go), and mschapv2Method carries the MS-CHAPv2 opcodes alone (mschapv2Method.Process, internal/core/eap/eap_mschapv2.go). The producer that would act as the role if Ze did is the specification of such a method, and Ze holds only the implementations of RFC 5216 and draft-kamath-pppext-eap-mschapv2-02. | To address security vulnerabilities, "tunneled" methods MUST support protection against man-in-the-middle attacks. |
| `2.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a pass-through authenticator must be capable of forwarding a Code=2 Response to the backend authentication server. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/core/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A pass-through authenticator implementation MUST be capable of forwarding EAP packets received from the peer with Code=2 (Response) to the backend authentication server. |
| `2.3:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a pass-through authenticator must be capable of forwarding Code=1, Code=3 and Code=4 packets received from the backend authentication server to the peer. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/core/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | It also MUST be capable of receiving EAP packets from the backend authentication server and forwarding EAP packets of Code=1 (Request), Code=3 (Success), and Code=4 (Failure) to the peer. |
| `2.3:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a compliant pass-through authenticator must by default forward EAP packets of any Type. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/core/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | For sessions in which the authenticator acts as a pass-through, it MUST determine the outcome of the authentication solely based on the Accept/Reject indication sent by the backend authentication server; the outcome MUST NOT be determined by the contents of an EAP packet sent along with the Accept/Reject indication, or the absence of such an encapsulated EAP packet. |
| `3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is a PPP implementation that has negotiated EAP as its LCP Authentication-Protocol (0xC227), which Section 3.2 titles 'EAP Usage Within PPP'. Ze never acts as it, and the producer that would is authMethodFromAuthProto (internal/component/l2tp/ppp/auth.go): it recognises PAP and CHAP alone and answers AuthMethodNone for every other Auth-Protocol value, 0xC227 included, so Ze's PPP falls back to the no-wire-auth phase rather than starting an EAP conversation. Ze carries EAP only inside the IKEv2 SK payload (startEAPExchange, internal/component/ike/engine/fsm.go). RFC 3748 obliges no implementation to support PPP as a lower layer: Section 2.2 lists PPP among the layers EAP 'has been run over', which is a description rather than a requirement. | If authentication of the link is desired, an implementation MUST specify the Authentication Protocol Configuration Option during the Link Establishment phase. |
| `4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the same-Identifier rule for a retransmitted Request. Site 4.1:8 maps the id. | Retransmitted Requests MUST be sent with the same Identifier value in order to distinguish them from new Requests. |
| `4.1:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the same-Identifier rule for a Request retransmitted on a timeout. Site 4.1:8 maps the id. | The Identifier field MUST be the same if a Request packet is retransmitted due to a timeout while waiting for a Response. |
| `4.1:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:2 maps the id. | Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored upon reception. |
| `4.1:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:3 maps the id. | A message with the Length field set to a value larger than the number of received octets MUST be silently discarded. |
| `4.1:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates Section 2.1's ban on a Nak after an initial non-Nak Response. Site 2.1:3 maps the id. | A peer MUST NOT send a Nak (legacy or expanded) in response to a Request, after an initial non-Nak Response has been sent. |
| `4.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Failure arm of the same obligation, one sentence after the Success arm. Site 4.2:1 maps the id. | If the authenticator cannot authenticate the peer (unacceptable Responses to one or more Requests), then after unsuccessful completion of the EAP method in progress, the implementation MUST transmit an EAP packet with the Code field set to 4 (Failure). |
| `4.2:15` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of an unexercised MAY: 'However, an authenticator MAY omit having the peer authenticate to it in situations where limited access is offered (e.g., guest access).  In this case, the authenticator MUST send a Success packet.' Ze's authenticator offers no guest access: every path to a Success runs through a completed method (Session.handleMethod, internal/core/eap/eap.go). | In this case, the authenticator MUST send a Success packet. |
| `4.3:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of a MAY inside a RECOMMENDED algorithm: '...the retransmission timer is calculated with a jitter by using the RTO value and randomly adding a value drawn between -RTOmin/2 and RTOmin/2.  Alternative calculations to create jitter MAY be used.  These MUST be pseudo-random.' Ze runs no EAP-layer retransmission timer at all, which Section 4.3 itself directs for a reliable lower layer. | These MUST be pseudo-random. |
| `5.2:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of a MAY, and the enclosing construction is: 'An EAP method MAY indicate within its specification that Notification messages must not be sent during that method.  In this case, the peer MUST silently discard Notification Requests from the point where an initial Request for that Type is answered with a Response of the same Type.' The antecedent is false for every method Ze runs. Neither specification exercises that MAY: the string 'Notification' appears nowhere in rfc/full/rfc5216.txt and nowhere in rfc/full/rfc2759.txt, so no method Ze offers prohibits Notification messages and the peer is never in the state this sentence describes. What Ze owes a Notification Request OUTSIDE that state is Section 5.2's opening obligation, which sites 5.2:1 and 5.2:5 relocate to plan/spec-eap-notification-and-nak.md under RFC3748-5.2-1. | In this case, the peer MUST silently discard Notification Requests from the point where an initial Request for that Type is answered with a Response of the same Type. |
| `5.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on SENDING a Notification Request, which RFC 3748 Section 5.2 states as an option in its own words: "An authenticator MAY send a Notification Request to the peer at any time when there is no outstanding Request, prior to completion of an EAP authentication method." The owner decided on 2026-09-01 (D-2 of plan/spec-eap-notification-and-nak.md) that ze's authenticator sends none, and Section 5.2 itself notes "In most circumstances, Notification should not be required." No producer composes a Type-2 Request: Session.Begin issues Identity and Session.handleMethod issues only the method's own packets (internal/core/eap/eap.go). The absent FEATURE is disclosed in rfc/short/rfc3748.md and a later scope decision can revisit it. Ze's peer ANSWERS a Notification Request, which is the mandatory half and is RFC3748-5.2-1. | The message MUST NOT be null terminated. |
| `5.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates Section 5.2's opening obligation to answer a Notification Request with a Notification Response, which site 5.2:1 maps to RFC3748-5.2-1. | A Response MUST be sent in reply to the Request with a Type field of 2 (Notification). |
| `5.3.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states when an Expanded Nak may be sent. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/core/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | It MUST be sent only in reply to a Request of Type 254 (Expanded Type) where the authentication Type is unacceptable. |
| `5.3.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the ban on an Expanded Nak as a general error indication. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/core/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | Since the Expanded Nak Type is valid only in Responses and has very limited functionality, it MUST NOT be used as a general purpose error indication, such as for communication of error messages, or negotiation of parameters specific to a particular EAP method. |
| `5.3.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the Expanded Nak Identifier rule. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/core/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Identifier field of an Expanded Nak Response MUST match the Identifier field of the Request packet that it is sent in response to. |
| `5.3.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the Expanded Nak Vendor-Data contents. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/core/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Vendor-Data field of the Nak Response MUST contain one or more authentication Types (4 or greater), all in expanded format, 8 octets per Type, or the value zero (0), also in Expanded Type format, to indicate no proposed alternative. |
| `5.4:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states what an authenticator that supports only pass-through must do with an MD5-Challenge Response. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/core/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | An authenticator that supports only pass- through MUST allow communication with a backend authentication server that is capable of supporting MD5-Challenge, although the EAP authenticator implementation need not support MD5-Challenge itself. |
| `5.4:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation is [RFC1994]'s, cited by this sentence: 'while [RFC1994] states that both the Identifier and Challenge fields MUST change each time a Challenge ... is sent'. | EAP allows for retransmission of MD5-Challenge Request packets, while [RFC1994] states that both the Identifier and Challenge fields MUST change each time a Challenge (the CHAP equivalent of the MD5-Challenge Request packet) is sent. |
| `5.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states answering an OTP Request. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request. |
| `5.5:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the Type of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Response MUST be of Type 5 (OTP), Nak (Type 3), or Expanded Nak (Type 254). |
| `5.5:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the ban on cleartext passwords. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The EAP OTP method is intended for use with the One-Time Password system only, and MUST NOT be used to provide support for cleartext passwords. |
| `5.5:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the message not being null terminated. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The messages MUST NOT be null terminated. |
| `5.6:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states answering a GTC Request. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request. |
| `5.6:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the Type of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Response MUST be of Type 6 (GTC), Nak (Type 3), or Expanded Nak (Type 254). |
| `5.6:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the ban on cleartext passwords outside a protected tunnel. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The EAP GTC method is intended for use with the Token Cards supporting challenge/response authentication and MUST NOT be used to provide support for cleartext passwords in the absence of a protected tunnel with server authentication. |
| `5.6:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the message not being null terminated. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The message MUST NOT be null terminated. |
| `5.6:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the Type field of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/core/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request with a Type field of 6 (Generic Token Card). |
| `5.7:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Nak this sentence demands IS the legacy Nak of Section 5.3.1, which it cites by number, so the obligation is the one site 5.3.1:3 maps to RFC3748-5.3.1-1. PeerSession.naks routes Type 254 to that same builder rather than composing an Expanded Nak (internal/core/eap/peer.go). | Peers not equipped to interpret the Expanded Type MUST send a Nak as described in Section 5.3.1, and negotiate a more suitable authentication method. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the Security Claims section a method specification must include. | In order to clearly articulate the security provided by an EAP method, EAP method specifications MUST include a Security Claims section, including the following declarations: |
| `7.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the effective key strength estimate. | If the method derives keys, then the effective key strength MUST be estimated. |
| `7.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the key hierarchy reference or MSK/EMSK derivation description. | EAP methods deriving keys MUST either provide a reference to a key hierarchy specification, or describe how Master Session Keys (MSKs) and Extended Master Session Keys (EMSKs) are to be derived. |
| `7.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the claims that are NOT being made. | In addition to the security claims that are made, the specification MUST indicate which of the security claims detailed in Section 7.2.1 are NOT being made. |
| `7.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. Section 7.2.1 is the claims VOCABULARY a specification writes its Security Claims section in, and this sentence states describing the EAP packets and fields protected. | When making this claim, a method specification MUST describe the EAP packets and fields within the EAP packet that are protected. |
| `7.2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. Section 7.2.1 is the claims VOCABULARY a specification writes its Security Claims section in, and this sentence states the identity protection a claiming method must support. | A method making this claim MUST support identity protection (see Section 7.3). |
| `7.10:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states that keying material a method exports must be independent of the ciphersuite negotiated to protect data, which is a property of the DERIVATION a specification defines. The two Ze runs are defined elsewhere: RFC 5216 Section 2.3 for EAP-TLS (exportEAPTLSMSK, internal/core/eap/eap_tls.go) and draft-kamath-pppext-eap-mschapv2-02 for EAP-MSCHAPv2 (DeriveMSK, internal/core/eap/mschapv2.go). | Keying material exported by EAP methods MUST be independent of the ciphersuite negotiated to protect data. |
| `7.10:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states cryptographic separation between the MSK and EMSK branches, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Methods supporting key derivation MUST demonstrate cryptographic separation between the MSK and EMSK branches of the EAP key hierarchy. |
| `7.10:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the non-recoverability of one key from the other, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Without violating a fundamental cryptographic assumption (such as the non-invertibility of a one-way function), an attacker recovering the MSK or EMSK MUST NOT be able to recover the other quantity with a level of effort less than brute force. |
| `7.10:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the separation of non-overlapping MSK substrings, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Non-overlapping substrings of the MSK MUST be cryptographically separate from each other, as defined in Section 7.2.1. |
| `7.10:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the same property stated as a knowledge bound, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | That is, knowledge of one substring MUST NOT help in recovering some other substring without breaking some hard cryptographic assumption. |
| `7.10:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the separation of non-overlapping EMSK substrings, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Likewise, non-overlapping substrings of the EMSK MUST be cryptographically separate from each other, and from substrings of the MSK. |
| `7.13:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that the AAA protocol spoken between an authenticator and a backend authentication server must support per-packet authentication. Section 7.13 states its own antecedent, 'in the case where the authenticator and authentication server reside on different machines', which is pass-through. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/core/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | In practice, this implies that the AAA protocol spoken between the authenticator and authentication server MUST support per-packet authentication, integrity, and replay protection. |
| `7.16:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states which result indications are protected, which a specification claiming protected result indications must document. | A method supporting protected result indications MUST indicate which result indications are protected, and which are not. |
| `7.16:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/core/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the four claims a protected-result-indication method must also support, which a specification claiming protected result indications must document. | Since protected result indications require use of a key for per-packet authentication and integrity protection, methods supporting protected result indications MUST also support the "key derivation", "mutual authentication", "integrity protection", and "replay protection" claims. |

## Superseded

No document obsoletes RFC 3748, so its obligations are stated where they were written.
