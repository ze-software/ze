# RFC 3748 - Extensible Authentication Protocol (EAP)

Supported in IPsec. Every requirement this repository extracted from RFC 3748, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 62.3% | 33 of 53 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.4% | 5 of 53 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 53 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 44.2% | 34 of 77 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 61 | of 67 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 61 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 28.3% | 15 of 53 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 53 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported in IPsec |
| Enrolment | Enrolled |
| Requirements | 67 |
| Gated MUST-level | 61 |
| Obligations that bind Ze | 53 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
| Gated with no test | 15 |
| Nightly-only evidence | 0 |
| Test tags | 77 |
| Tagged units | 77 |
| Recorded audit verdicts | 0 |
| Discrimination records | 34 |
| Summary | `rfc/short/rfc3748.md` |
| Requirement shard | `rfc/requirements/rfc3748.md` |
| RFC text | `rfc/full/rfc3748.txt` |

## Enrolment

Enrolled: Extensible Authentication Protocol / EAP (RFC 3748): IKEv2-carried EAP peer + co-located authenticator (EAP-TLS, EAP-MSCHAPv2, local termination, no pass-through). Both roles are gated: the packet format and its length handling, the Identifier and Type rules that bind a Request to its Response, one method per conversation, method completion -> Success/Failure, the 4-octet terminal packet, the peer's end of an unsuccessful method, and four silent discards -- a Code outside 1-4 by either role, and by the peer a canned Success, a Success the method does not permit yet, and a Failure after both ends indicated success. A non-Response carrying a DEFINED Code is still answered with EAP-Failure; a Code outside 1-4 is discarded, which corrects a row that claimed the first behavior for every non-Response (2026-09-01). Lower-layer ordering, error detection, MTU and duplicate detection are the IKEv2 carrier's, pass-through and Expanded Types are out of scope, and no EMSK is derived.

## What the public ledger says

**Status:** Supported in IPsec

**What the ledger says is covered**

EAP framework inside IKEv2 IKE_AUTH, Success and Failure handling, and the Section 4.2 discards that stop a rogue authenticator bypassing the method: the peer reads an EAP-Success only once the method conversation concluded, drops an EAP-Failure once both ends indicated success, and both roles drop a Code outside 1-4 (`PeerSession.Process` and `Session.Process`, `internal/component/ike/eap`). The peer answers Type 1 (Identity), Type 2 (Notification) and Type 3 (Nak): a Notification Request draws a five-octet Notification Response and its message reaches the operator log, and a Request for an authentication Type ze does not run draws a six-octet legacy Nak naming the configured method, until the peer has answered a method Request and Section 2.1 closes the Nak (`PeerSession.handleRequest`, [`internal/component/ike/eap/peer.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/peer.go)). A Type-254 Request draws the same legacy Nak, which is what Section 5.7 prescribes for a peer not equipped to interpret an Expanded Type. Type 4 (MD5-Challenge) runs on both roles and is the `authentication { mode eap-md5 }` an operator selects. It is never a default, and adopting a configuration that names it writes one warning quoting the RFC 7296 Section 2.16 sentence that discourages a method establishing no shared key (`warnKeylessEAPModes`, [`internal/component/ike/engine/eap_auth.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/eap_auth.go)).

**What the ledger says remains**

Ze reads no Expanded Type (254), which Section 5 states as a SHOULD, and answers a Type-254 Request with the legacy Nak Section 5.7 prescribes rather than composing an Expanded Nak. Ze's authenticator sends no Notification Request, which RFC 3748 Section 5.2 states as an option ("An authenticator MAY send a Notification Request to the peer at any time when there is no outstanding Request, prior to completion of an EAP authentication method") and which the owner declined on 2026-09-01; Ze's peer answers one, which is the mandatory half. Two further features are absent by decision rather than by omission, and neither is a conformance gap. Ze's authenticator terminates every EAP method locally and does not act as a pass-through agent for a backend authentication server; Section 2 says "Support for pass-through is optional". Ze offers neither the One Time Password method (Type 5) nor the Generic Token Card method (Type 6), which Section 5 leaves to the implementation ("Implementations MAY support other Types defined here or in future RFCs"). A later scope decision can revisit any of the three. The Type 4 (MD5-Challenge) deviation authorized on 2026-08-30 was WITHDRAWN by the owner on 2026-09-01, who ordered the method implemented; both roles now run it.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 33 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 15 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **61** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (33):** [`RFC3748-4-1`](#rfc3748-4-1), [`RFC3748-4-2`](#rfc3748-4-2), [`RFC3748-2-2`](#rfc3748-2-2), [`RFC3748-2.1-1`](#rfc3748-2.1-1), [`RFC3748-2.1-2`](#rfc3748-2.1-2), [`RFC3748-2.1-3`](#rfc3748-2.1-3), [`RFC3748-4.2-2`](#rfc3748-4.2-2), [`RFC3748-4-4`](#rfc3748-4-4), [`RFC3748-4.1-3`](#rfc3748-4.1-3), [`RFC3748-4.1-4`](#rfc3748-4.1-4), [`RFC3748-4.1-5`](#rfc3748-4.1-5), [`RFC3748-4.2-5`](#rfc3748-4.2-5), [`RFC3748-4.2-6`](#rfc3748-4.2-6), [`RFC3748-4-5`](#rfc3748-4-5), [`RFC3748-4.2-7`](#rfc3748-4.2-7), [`RFC3748-4.2-8`](#rfc3748-4.2-8), [`RFC3748-4.2-9`](#rfc3748-4.2-9), [`RFC3748-4.1-10`](#rfc3748-4.1-10), [`RFC3748-4.1-11`](#rfc3748-4.1-11), [`RFC3748-2.1-4`](#rfc3748-2.1-4), [`RFC3748-5-1`](#rfc3748-5-1), [`RFC3748-5-2`](#rfc3748-5-2), [`RFC3748-5.2-1`](#rfc3748-5.2-1), [`RFC3748-5.2-2`](#rfc3748-5.2-2), [`RFC3748-5.3.1-1`](#rfc3748-5.3.1-1), [`RFC3748-5.3.1-2`](#rfc3748-5.3.1-2), [`RFC3748-5.3.1-3`](#rfc3748-5.3.1-3), [`RFC3748-5.3.1-4`](#rfc3748-5.3.1-4), [`RFC3748-5.4-1`](#rfc3748-5.4-1), [`RFC3748-5.4-2`](#rfc3748-5.4-2), [`RFC3748-5.1-2`](#rfc3748-5.1-2), [`RFC3748-7.5-1`](#rfc3748-7.5-1), [`RFC3748-7.10-4`](#rfc3748-7.10-4)

**Annotated instead of tested (13):** [`RFC3748-2-1`](#rfc3748-2-1), [`RFC3748-4.1-1`](#rfc3748-4.1-1), [`RFC3748-4.1-2`](#rfc3748-4.1-2), [`RFC3748-4.2-1`](#rfc3748-4.2-1), [`RFC3748-4.2-4`](#rfc3748-4.2-4), [`RFC3748-2.3-1`](#rfc3748-2.3-1), [`RFC3748-3.1-1`](#rfc3748-3.1-1), [`RFC3748-3.1-2`](#rfc3748-3.1-2), [`RFC3748-3.1-3`](#rfc3748-3.1-3), [`RFC3748-3.1-4`](#rfc3748-3.1-4), [`RFC3748-5.7-1`](#rfc3748-5.7-1), [`RFC3748-7.10-1`](#rfc3748-7.10-1), [`RFC3748-7.10-2`](#rfc3748-7.10-2)

**No test and no annotation (15):** [`RFC3748-2-3`](#rfc3748-2-3), [`RFC3748-2.2-1`](#rfc3748-2.2-1), [`RFC3748-4.1-6`](#rfc3748-4.1-6), [`RFC3748-4.1-7`](#rfc3748-4.1-7), [`RFC3748-4.1-8`](#rfc3748-4.1-8), [`RFC3748-4.1-9`](#rfc3748-4.1-9), [`RFC3748-4.2-15`](#rfc3748-4.2-15), [`RFC3748-4.2-10`](#rfc3748-4.2-10), [`RFC3748-4.2-11`](#rfc3748-4.2-11), [`RFC3748-4.2-12`](#rfc3748-4.2-12), [`RFC3748-4.2-13`](#rfc3748-4.2-13), [`RFC3748-4.2-14`](#rfc3748-4.2-14), [`RFC3748-7.10-5`](#rfc3748-7.10-5), [`RFC3748-7.10-6`](#rfc3748-7.10-6), [`RFC3748-7.10-7`](#rfc3748-7.10-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3748-4-1` | Packets shorter than the Length field indicates MUST be silently discarded (S4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L73). **negative:** `unit/verify` [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L80) |
| `RFC3748-4-2` | Minimum EAP packet length is 4 octets (S4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L89). **negative:** `unit/verify` [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L99) |
| `RFC3748-2-1` | The authenticator MUST operate in lock-step: only one outstanding Request at a time (S2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC3748AuthenticatorLockStep`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L167). **negative:** no negative test. **{single-polarity}:** the authenticator Session emits one packet per call -- Begin returns a single Request (internal/component/ike/eap/eap.go:164) and each Process returns a single *Packet (eap.go:176), so a new Request cannot precede its Response and no path leaves two outstanding, giving no negative case |
| `RFC3748-2-2` | The authenticator MUST receive a valid Response before sending a new Request (S2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L208). **negative:** `unit/verify` [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L220) |
| `RFC3748-2.1-1` | Only one authentication method (Type >= 4) per EAP conversation (S2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L235). **negative:** `unit/verify` [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L242) |
| `RFC3748-2.1-2` | After the method completes, the authenticator MUST send Success or Failure (S2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L267). **negative:** `unit/verify` [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L277) |
| `RFC3748-2.1-3` | The peer MUST NOT send a NAK (Type 3) after the first non-NAK Response in a conversation (S2.1, S5.3) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L372). **negative:** `unit/verify` [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L385) |
| `RFC3748-4.1-1` | Only the authenticator retransmits on timer; the peer MUST NOT retransmit on its own timer (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748PeerHasNoRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L289). **negative:** no negative test. **{single-polarity}:** the EAP peer (PeerSession.Process, internal/component/ike/eap/peer.go:97) is a synchronous request-driven transform with no timer -- it emits a Response only when handed a Request and never self-retransmits; lower-layer retransmission is IKEv2's (internal/component/ike/engine/fsm.go:138), so there is no forbidden self-timer to test negatively |
| `RFC3748-4.1-2` | The peer MUST re-send its last Response when a duplicate Request arrives (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** duplicate-Request handling is delegated to the IKEv2 carrier -- on timeout the initiator (EAP peer) re-sends its last IKE_AUTH message, which carries its last EAP Response (sa.LastSentMsg, internal/component/ike/engine/fsm.go:138); the EAP peer state machine holds no per-Request retransmission state |
| `RFC3748-4.2-1` | Success and Failure packets MUST NOT be retransmitted (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748SuccessFailureNotRetransmitted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L140). **negative:** no negative test. **{single-polarity}:** the authenticator Session emits Success or Failure once and then enters a terminal state where Process returns nil (internal/component/ike/eap/eap.go:186), so the EAP layer never retransmits them; there is no negative case |
| `RFC3748-4.2-2` | Success and Failure contain only Code, Identifier, and Length (4 octets, no Type field) (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L110). **negative:** `unit/verify` [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L130) |
| `RFC3748-4.2-4` | The Identifier of a Success or Failure MUST match the Identifier of the Response it answers (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L24). **positive:** `unit/verify` [`TestFailureIdentifierMatchesResponseOnNAK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L49). **positive:** `unit/verify` [`TestIdentityFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L84). **positive:** `unit/verify` [`TestSuccessIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L153). **negative:** no negative test. **{single-polarity}:** this obligation is on the SENDER. The authenticator's terminal packets are produced by Session.failure and the result.Done arm of Session.handleMethod (internal/component/ike/eap/eap.go), and both now stamp the answered Response's Identifier. A negative case would need a RECEIVER that discards a mismatched Success or Failure, which Section 4.2 does not require of a sender and which ze's PeerSession.Process (internal/component/ike/eap/peer.go) does not do: it switches on request.Code alone |
| `RFC3748-2.3-1` | A pass-through authenticator MUST forward packets of any Type without interpreting method-specific data (S2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authenticator always terminates the EAP method locally (eap.Session, internal/component/ike/eap/eap.go:130); there is no AAA/RADIUS back end in the IKE engine, so it never operates as a pass-through |
| `RFC3748-3.1-1` | Lower layer MUST provide in-order delivery for EAP (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** in-order delivery is a lower-layer obligation; ze carries EAP only inside IKEv2, whose message-ID sequencing delivers each IKE_AUTH request/response in order (RFC 7296 Section 2.3; internal/component/ike/engine/msgid.go:76), so the EAP framework code neither provides nor can violate it |
| `RFC3748-3.1-2` | Lower layer MUST provide error detection (CRC, checksum, or MIC) (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** bit-error detection is a lower-layer obligation; ze's EAP packets travel inside the IKEv2 SK payload whose AEAD/ICV check rejects any corrupted frame on decrypt (internal/component/ike/engine/fsm.go:658), so the EAP framework code adds no CRC of its own |
| `RFC3748-3.1-3` | Lower layer MUST support a minimum MTU of 1020 octets for EAP packets (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the 1020-octet minimum MTU is a lower-layer obligation; ze carries EAP inside IKEv2 SK payloads over UDP, which admit EAP packets far larger than 1020 octets, and EAP-TLS method data is itself fragmented in 1024-octet chunks (internal/component/ike/eap/eap_tls.go:28) |
| `RFC3748-3.1-4` | Lower layer MUST provide duplicate detection (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** duplicate detection is a lower-layer obligation; IKEv2 detects a duplicated message by message ID and replays the cached response (internal/component/ike/engine/msgid.go:79; responder.go:81), so the EAP framework code neither provides nor can violate it |
| `RFC3748-5.7-1` | When Type = 254 (Expanded Types), Vendor-Id 0 = IETF namespace (S5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze offers MD5-Challenge (4), EAP-TLS (13) and EAP-MSCHAPv2 (26); NewSession rejects every other type (NewSession, internal/component/ike/eap/eap.go) and the codec never encodes or parses an Expanded Type (254) packet or its Vendor-Id field, so no Vendor-Id namespace rule can bind. The peer reads TypeExpandedEAP only to route a Type-254 Request to the legacy Nak that Section 5.7 prescribes for a peer not equipped to interpret it (PeerSession.naks, internal/component/ike/eap/peer.go), and that Nak carries no Vendor-Id |
| `RFC3748-7.10-1` | MSK MUST be at least 64 octets (S7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748MSKSize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L324). **negative:** no negative test. **{single-polarity}:** the MSK is the compile-time fixed [64]byte returned by DeriveMSK (internal/component/ike/eap/mschapv2.go:206) and by the EAP-TLS ExportKeyingMaterial(...,64) (eap_tls.go:262), so the 64-octet floor cannot be undershot and has no negative case |
| `RFC3748-7.10-2` | EMSK MUST be at least 64 octets (S7.10) | MUST | 7.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives only the MSK (exported for the IKEv2 AUTH payload) and never derives or consumes the EMSK -- no EMSK is produced anywhere in internal/component/ike/eap, so there is no EMSK to size-check |
| `RFC3748-7.10-3` | An EAP method that establishes no shared key, such as MD5-Challenge, SHOULD NOT be used with IKEv2 (RFC 7296 S2.16 states the rule; S7.10 anchors this id) | SHOULD NOT | 7.10 | **positive:** `unit/verify` [`TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L118). **negative:** `unit/verify` [`TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L169) |
| `RFC3748-4-4` | Octets outside the range of the Length field MUST be ignored upon reception (S4, S4.1) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L105). **negative:** `unit/verify` [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L117) |
| `RFC3748-4.1-3` | A retransmitted Request MUST carry the same Identifier value, and a new (non-retransmission) Request MUST carry one different from the previous Request (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L135). **negative:** `unit/verify` [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L146) |
| `RFC3748-4.1-4` | The Identifier field of a Response MUST match that of the currently outstanding Request (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L167). **negative:** `unit/verify` [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L178) |
| `RFC3748-4.1-5` | The Type field of a Response MUST match that of the Request, or be a legacy or Expanded NAK (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L204). **negative:** `unit/verify` [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L215) |
| `RFC3748-4.2-5` | Once the method completes unsuccessfully, the peer MUST terminate the conversation and indicate failure to the lower layer (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L249). **negative:** `unit/verify` [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L264) |
| `RFC3748-4.2-6` | Where the authenticator has sent no result indication, the peer MUST NOT silently discard the Success or Failure it waits for (S4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L283). **negative:** `unit/verify` [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L295) |
| `RFC3748-4-5` | EAP packets carrying a Code outside 1-4 MUST be silently discarded by both authenticators and peers (S4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L223). **negative:** `unit/verify` [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L263) |
| `RFC3748-4.2-7` | By default, an EAP peer MUST silently discard a "canned" Success packet, one sent immediately upon connection (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L82). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L103) |
| `RFC3748-4.2-8` | A peer receiving a Success or Failure packet where sending one is not explicitly permitted MUST silently discard it (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L132). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L145) |
| `RFC3748-4.2-9` | On the peer, after success result indications have been exchanged by both sides, a Failure packet MUST be silently discarded (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L178). **negative:** `unit/verify` [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L199) |
| `RFC3748-2-3` | The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-2.2-1` | The Success, Failure, Nak Response and Notification Request/Response messages MUST NOT be used to carry data destined for delivery to other EAP methods (S2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.1-6` | Additional Request packets MUST be sent until a valid Response packet is received, an optional retry counter expires, or a lower layer failure indication is received (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.1-7` | The peer MUST send a Response packet in reply to a valid Request packet (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.1-8` | Requests MUST be processed in the order that they are received, and MUST be processed to their completion before inspecting the next Request (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.1-9` | A single Type MUST be specified for each EAP Request or Response (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.1-10` | An authenticator receiving a Response whose Identifier value does not match that of the currently outstanding Request MUST silently discard the Response (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAuthenticatorDiscardsAResponseAnsweringNoOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L182). **negative:** `unit/verify` [`TestAuthenticatorProcessesAResponseAnsweringTheOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L203) |
| `RFC3748-4.1-11` | An EAP server receiving a Response whose Type is neither the outstanding Request's nor a legacy Nak MUST silently discard it (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAuthenticatorDiscardsAResponseOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L297). **negative:** `unit/verify` [`TestAuthenticatorProcessesAResponseOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L324) |
| `RFC3748-2.1-4` | A peer receiving a Request of a Type other than the one under way MUST silently discard it, because an authenticator MUST NOT send a Request of a different Type before the method's final round completes (S2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPeerDiscardsARequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L347). **negative:** `unit/verify` [`TestPeerProcessesARequestOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L387) |
| `RFC3748-4.2-15` | The peer MUST silently discard a Success packet that arrives after the peer has ended the session (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-10` | Success and Failure packets MUST NOT be sent by an EAP authenticator if the specification of the given method does not explicitly permit the method to finish at that point (S4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-11` | A peer MUST allow for the circumstance that a Success or Failure packet, being unacknowledged, can be lost (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-12` | After the authenticator sends a failure result indication to the peer, regardless of the response from the peer, it MUST subsequently send a Failure packet (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-13` | After the authenticator sends a success result indication to the peer and receives a success result indication from the peer, it MUST subsequently send a Success packet (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-14` | If the peer attempts to authenticate to the authenticator and fails to do so, the authenticator MUST send a Failure packet and MUST NOT grant access by sending a Success packet (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-7.10-5` | The MSK and EMSK MUST NOT be used directly to protect data (S7.10) | MUST NOT | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-7.10-6` | The EMSK MUST remain on the EAP peer and EAP server where it is derived, and MUST NOT be transported to, shared with, or used to derive keys for additional parties (S7.10) | MUST | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-7.10-7` | EAP peers, authenticators and authentication servers MUST be prepared for situations in which one of the parties discards the key state, which remains valid on another party (S7.10) | MUST | 7.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-5-1` | NAK (Type 3) and Expanded NAK (Type 254) MUST NOT be sent in a Request (S5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L312). **negative:** `unit/verify` [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L322) |
| `RFC3748-5-2` | All EAP implementations MUST support Types 1-4, which are defined in this document (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3748PeerSupportsTypesOneToFour`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L316). **negative:** `unit/verify` [`TestRFC3748PeerRefusesATypeOutsideOneToFour`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L387) |
| `RFC3748-5.2-1` | The peer MUST respond to a Notification Request with a Notification Response, unless the EAP authentication method specification prohibits the use of Notification messages (S5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestPeerAnswersANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L89). **negative:** `unit/verify` [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L147) |
| `RFC3748-5.2-2` | A Nak Response MUST NOT be sent in response to a Notification Request (S5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L153). **negative:** `unit/verify` [`TestPeerNaksAnAuthenticationTypeButNotANotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L179) |
| `RFC3748-5.3.1-1` | Where a peer receives a Request for an unacceptable authentication Type (4-253,255), or a peer lacking support for Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST be sent (S5.3.1, S5.7) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestPeerNaksAnExpandedTypeRequestWithALegacyNak`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L259). **positive:** `unit/verify` [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L140). **negative:** `unit/verify` [`TestPeerDoesNotNakATypeItHandles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L179) |
| `RFC3748-5.3.1-2` | The Type-Data field of the Nak Response (Type 3) MUST contain one or more octets indicating the desired authentication Type(s), one octet per Type, or the value zero (0) to indicate no proposed alternative (S5.3.1) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L147). **negative:** `unit/verify` [`TestNakNamesTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L230) |
| `RFC3748-5.3.1-3` | The Identifier field of a legacy Nak Response MUST match the Identifier field of the Request packet that it is sent in response to (S5.3.1) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L289). **negative:** `unit/verify` [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L304) |
| `RFC3748-5.3.1-4` | The legacy Nak MUST NOT be used as a general purpose error indication, such as for communication of error messages or negotiation of method-specific parameters (S5.3.1) | MUST NOT | 5.3.1 | **positive:** `unit/verify` [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L324). **negative:** `unit/verify` [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L347) |
| `RFC3748-5.4-1` | A Response MUST be sent in reply to an MD5-Challenge Request (S5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC3748MD5ChallengeRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L117). **negative:** `unit/verify` [`TestRFC3748MD5ChallengeRequeryDrawsNoResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L206) |
| `RFC3748-5.4-2` | EAP peer and EAP server implementations MUST support the MD5-Challenge mechanism (S5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC3748MD5ChallengeSupportedByBothRoles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L237). **negative:** `unit/verify` [`TestRFC3748MD5ChallengeIsTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L278) |
| `RFC3748-5.1-2` | The Identity Response field MUST NOT be null terminated (S5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L365). **negative:** `unit/verify` [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L373) |
| `RFC3748-7.5-1` | Where an EAP method employs a per-packet MIC, the peer and an authenticator not in pass-through mode MUST validate it (S7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L471). **negative:** `unit/verify` [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L484) |
| `RFC3748-7.10-4` | EAP methods deriving keys MUST provide for mutual authentication between the EAP peer and the EAP server (S7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L502). **negative:** `unit/verify` [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L517) |
| `RFC3748-5.1-1` | Identity (Type 1) SHOULD NOT be relied upon for authentication; it is sent in cleartext (S5.1, S7.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-7.4-1` | EAP methods used in environments subject to MITM attacks SHOULD provide mutual authentication (S7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4.2-3` | Peer SHOULD treat lower-layer success as implicit EAP Success if EAP Success is lost (S4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-2.3-2` | The authenticator MAY act as a pass-through, forwarding EAP to a back-end server via AAA (S2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3748-4-3` | Octets beyond the Length field MAY be ignored (S4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3748-4.1-2`](#rfc3748-4.1-2) The peer MUST re-send its last Response when a duplicate Request arrives (S4.1) | no test | no test carries this requirement id; annotated {not-applicable}: duplicate-Request handling is delegated to the IKEv2 carrier -- on timeout the initiator (EAP peer) re-sends its last IKE_AUTH message, which carries its last EAP Response (sa.LastSentMsg, internal/component/ike/engine/fsm.go:138); the EAP peer state machine holds no per-Request retransmission state |
| [`RFC3748-2.3-1`](#rfc3748-2.3-1) A pass-through authenticator MUST forward packets of any Type without interpreting method-specific data (S2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authenticator always terminates the EAP method locally (eap.Session, internal/component/ike/eap/eap.go:130); there is no AAA/RADIUS back end in the IKE engine, so it never operates as a pass-through |
| [`RFC3748-3.1-1`](#rfc3748-3.1-1) Lower layer MUST provide in-order delivery for EAP (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: in-order delivery is a lower-layer obligation; ze carries EAP only inside IKEv2, whose message-ID sequencing delivers each IKE_AUTH request/response in order (RFC 7296 Section 2.3; internal/component/ike/engine/msgid.go:76), so the EAP framework code neither provides nor can violate it |
| [`RFC3748-3.1-2`](#rfc3748-3.1-2) Lower layer MUST provide error detection (CRC, checksum, or MIC) (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: bit-error detection is a lower-layer obligation; ze's EAP packets travel inside the IKEv2 SK payload whose AEAD/ICV check rejects any corrupted frame on decrypt (internal/component/ike/engine/fsm.go:658), so the EAP framework code adds no CRC of its own |
| [`RFC3748-3.1-3`](#rfc3748-3.1-3) Lower layer MUST support a minimum MTU of 1020 octets for EAP packets (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the 1020-octet minimum MTU is a lower-layer obligation; ze carries EAP inside IKEv2 SK payloads over UDP, which admit EAP packets far larger than 1020 octets, and EAP-TLS method data is itself fragmented in 1024-octet chunks (internal/component/ike/eap/eap_tls.go:28) |
| [`RFC3748-3.1-4`](#rfc3748-3.1-4) Lower layer MUST provide duplicate detection (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: duplicate detection is a lower-layer obligation; IKEv2 detects a duplicated message by message ID and replays the cached response (internal/component/ike/engine/msgid.go:79; responder.go:81), so the EAP framework code neither provides nor can violate it |
| [`RFC3748-5.7-1`](#rfc3748-5.7-1) When Type = 254 (Expanded Types), Vendor-Id 0 = IETF namespace (S5.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze offers MD5-Challenge (4), EAP-TLS (13) and EAP-MSCHAPv2 (26); NewSession rejects every other type (NewSession, internal/component/ike/eap/eap.go) and the codec never encodes or parses an Expanded Type (254) packet or its Vendor-Id field, so no Vendor-Id namespace rule can bind. The peer reads TypeExpandedEAP only to route a Type-254 Request to the legacy Nak that Section 5.7 prescribes for a peer not equipped to interpret it (PeerSession.naks, internal/component/ike/eap/peer.go), and that Nak carries no Vendor-Id |
| [`RFC3748-7.10-2`](#rfc3748-7.10-2) EMSK MUST be at least 64 octets (S7.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives only the MSK (exported for the IKEv2 AUTH payload) and never derives or consumes the EMSK -- no EMSK is produced anywhere in internal/component/ike/eap, so there is no EMSK to size-check |
| [`RFC3748-2-3`](#rfc3748-2-3) The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2) | no test | no test carries this requirement id |
| [`RFC3748-2.2-1`](#rfc3748-2.2-1) The Success, Failure, Nak Response and Notification Request/Response messages MUST NOT be used to carry data destined for delivery to other EAP methods (S2.2) | no test | no test carries this requirement id |
| [`RFC3748-4.1-6`](#rfc3748-4.1-6) Additional Request packets MUST be sent until a valid Response packet is received, an optional retry counter expires, or a lower layer failure indication is received (S4.1) | no test | no test carries this requirement id |
| [`RFC3748-4.1-7`](#rfc3748-4.1-7) The peer MUST send a Response packet in reply to a valid Request packet (S4.1) | no test | no test carries this requirement id |
| [`RFC3748-4.1-8`](#rfc3748-4.1-8) Requests MUST be processed in the order that they are received, and MUST be processed to their completion before inspecting the next Request (S4.1) | no test | no test carries this requirement id |
| [`RFC3748-4.1-9`](#rfc3748-4.1-9) A single Type MUST be specified for each EAP Request or Response (S4.1) | no test | no test carries this requirement id |
| [`RFC3748-4.2-15`](#rfc3748-4.2-15) The peer MUST silently discard a Success packet that arrives after the peer has ended the session (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-4.2-10`](#rfc3748-4.2-10) Success and Failure packets MUST NOT be sent by an EAP authenticator if the specification of the given method does not explicitly permit the method to finish at that point (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-4.2-11`](#rfc3748-4.2-11) A peer MUST allow for the circumstance that a Success or Failure packet, being unacknowledged, can be lost (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-4.2-12`](#rfc3748-4.2-12) After the authenticator sends a failure result indication to the peer, regardless of the response from the peer, it MUST subsequently send a Failure packet (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-4.2-13`](#rfc3748-4.2-13) After the authenticator sends a success result indication to the peer and receives a success result indication from the peer, it MUST subsequently send a Success packet (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-4.2-14`](#rfc3748-4.2-14) If the peer attempts to authenticate to the authenticator and fails to do so, the authenticator MUST send a Failure packet and MUST NOT grant access by sending a Success packet (S4.2) | no test | no test carries this requirement id |
| [`RFC3748-7.10-5`](#rfc3748-7.10-5) The MSK and EMSK MUST NOT be used directly to protect data (S7.10) | no test | no test carries this requirement id |
| [`RFC3748-7.10-6`](#rfc3748-7.10-6) The EMSK MUST remain on the EAP peer and EAP server where it is derived, and MUST NOT be transported to, shared with, or used to derive keys for additional parties (S7.10) | no test | no test carries this requirement id |
| [`RFC3748-7.10-7`](#rfc3748-7.10-7) EAP peers, authenticators and authentication servers MUST be prepared for situations in which one of the parties discards the key state, which remains valid on another party (S7.10) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3748-4-1`](#rfc3748-4-1)

Packets shorter than the Length field indicates MUST be silently discarded (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC3748PacketLengthDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L73) | unit/verify | unproven |

### [`RFC3748-4-2`](#rfc3748-4-2)

Minimum EAP packet length is 4 octets (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC3748MinimumPacketLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L89) | unit/verify | unproven |

### [`RFC3748-2-1`](#rfc3748-2-1)

The authenticator MUST operate in lock-step: only one outstanding Request at a time (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748AuthenticatorLockStep`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L167) | unit/verify | unproven |

### [`RFC3748-2-2`](#rfc3748-2-2)

The authenticator MUST receive a valid Response before sending a new Request (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC3748AuthenticatorRequiresValidResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L208) | unit/verify | unproven |

### [`RFC3748-2.1-1`](#rfc3748-2.1-1)

Only one authentication method (Type >= 4) per EAP conversation (S2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L242) | unit/verify | unproven |
| positive | [`TestRFC3748OneMethodPerConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L235) | unit/verify | unproven |

### [`RFC3748-2.1-2`](#rfc3748-2.1-2)

After the method completes, the authenticator MUST send Success or Failure (S2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L277) | unit/verify | unproven |
| positive | [`TestRFC3748MethodCompletionSendsResult`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L267) | unit/verify | unproven |

### [`RFC3748-2.1-3`](#rfc3748-2.1-3)

The peer MUST NOT send a NAK (Type 3) after the first non-NAK Response in a conversation (S2.1, S5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerNaksBeforeItCommitsToAMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L372) | unit/verify | revert, verified |

### [`RFC3748-4.1-1`](#rfc3748-4.1-1)

Only the authenticator retransmits on timer; the peer MUST NOT retransmit on its own timer (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748PeerHasNoRetransmitTimer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L289) | unit/verify | unproven |

### [`RFC3748-4.1-2`](#rfc3748-4.1-2)

The peer MUST re-send its last Response when a duplicate Request arrives (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-2, so no unit is bound to it.

### [`RFC3748-4.2-1`](#rfc3748-4.2-1)

Success and Failure packets MUST NOT be retransmitted (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748SuccessFailureNotRetransmitted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L140) | unit/verify | unproven |

### [`RFC3748-4.2-2`](#rfc3748-4.2-2)

Success and Failure contain only Code, Identifier, and Length (4 octets, no Type field) (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L130) | unit/verify | unproven |
| positive | [`TestRFC3748SuccessFailureFormat`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L110) | unit/verify | unproven |

### [`RFC3748-4.2-4`](#rfc3748-4.2-4)

The Identifier of a Success or Failure MUST match the Identifier of the Response it answers (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L24) | unit/verify | unproven |
| positive | [`TestFailureIdentifierMatchesResponseOnNAK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L49) | unit/verify | unproven |
| positive | [`TestIdentityFailureIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L84) | unit/verify | unproven |
| positive | [`TestSuccessIdentifierMatchesResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L153) | unit/verify | unproven |

### [`RFC3748-2.3-1`](#rfc3748-2.3-1)

A pass-through authenticator MUST forward packets of any Type without interpreting method-specific data (S2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-2.3-1, so no unit is bound to it.

### [`RFC3748-3.1-1`](#rfc3748-3.1-1)

Lower layer MUST provide in-order delivery for EAP (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-1, so no unit is bound to it.

### [`RFC3748-3.1-2`](#rfc3748-3.1-2)

Lower layer MUST provide error detection (CRC, checksum, or MIC) (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-2, so no unit is bound to it.

### [`RFC3748-3.1-3`](#rfc3748-3.1-3)

Lower layer MUST support a minimum MTU of 1020 octets for EAP packets (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-3, so no unit is bound to it.

### [`RFC3748-3.1-4`](#rfc3748-3.1-4)

Lower layer MUST provide duplicate detection (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-3.1-4, so no unit is bound to it.

### [`RFC3748-5.7-1`](#rfc3748-5.7-1)

When Type = 254 (Expanded Types), Vendor-Id 0 = IETF namespace (S5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-5.7-1, so no unit is bound to it.

### [`RFC3748-7.10-1`](#rfc3748-7.10-1)

MSK MUST be at least 64 octets (S7.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3748MSKSize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_test.go#L324) | unit/verify | unproven |

### [`RFC3748-7.10-2`](#rfc3748-7.10-2)

EMSK MUST be at least 64 octets (S7.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-7.10-2, so no unit is bound to it.

### [`RFC3748-7.10-3`](#rfc3748-7.10-3)

An EAP method that establishes no shared key, such as MD5-Challenge, SHOULD NOT be used with IKEv2 (RFC 7296 S2.16 states the rule; S7.10 anchors this id)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L118) | unit/verify | revert, verified |

### [`RFC3748-4-4`](#rfc3748-4-4)

Octets outside the range of the Length field MUST be ignored upon reception (S4, S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L117) | unit/verify | unproven |
| positive | [`TestRFC3748LengthBoundsTypeData`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L105) | unit/verify | unproven |

### [`RFC3748-4.1-3`](#rfc3748-4.1-3)

A retransmitted Request MUST carry the same Identifier value, and a new (non-retransmission) Request MUST carry one different from the previous Request (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L146) | unit/verify | unproven |
| positive | [`TestRFC3748NewRequestChangesIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L135) | unit/verify | unproven |

### [`RFC3748-4.1-4`](#rfc3748-4.1-4)

The Identifier field of a Response MUST match that of the currently outstanding Request (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC3748ResponseEchoesRequestIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L167) | unit/verify | unproven |

### [`RFC3748-4.1-5`](#rfc3748-4.1-5)

The Type field of a Response MUST match that of the Request, or be a legacy or Expanded NAK (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L215) | unit/verify | revert, verified |
| positive | [`TestRFC3748ResponseTypeMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L204) | unit/verify | revert, verified |

### [`RFC3748-4.2-5`](#rfc3748-4.2-5)

Once the method completes unsuccessfully, the peer MUST terminate the conversation and indicate failure to the lower layer (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L264) | unit/verify | unproven |
| positive | [`TestRFC3748PeerEndsAnUnsuccessfulConversation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L249) | unit/verify | unproven |

### [`RFC3748-4.2-6`](#rfc3748-4.2-6)

Where the authenticator has sent no result indication, the peer MUST NOT silently discard the Success or Failure it waits for (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L295) | unit/verify | unproven |
| positive | [`TestRFC3748PeerActsOnTheTerminalPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L283) | unit/verify | unproven |

### [`RFC3748-4-5`](#rfc3748-4-5)

EAP packets carrying a Code outside 1-4 MUST be silently discarded by both authenticators and peers (S4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L263) | unit/verify | revert, verified |
| positive | [`TestRFC3748UndefinedCodesAreSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L223) | unit/verify | revert, verified |

### [`RFC3748-4.2-7`](#rfc3748-4.2-7)

By default, an EAP peer MUST silently discard a "canned" Success packet, one sent immediately upon connection (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsACannedSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L82) | unit/verify | revert, verified |

### [`RFC3748-4.2-8`](#rfc3748-4.2-8)

A peer receiving a Success or Failure packet where sending one is not explicitly permitted MUST silently discard it (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsASuccessTheMethodDoesNotPermitYet`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L132) | unit/verify | revert, verified |

### [`RFC3748-4.2-9`](#rfc3748-4.2-9)

On the peer, after success result indications have been exchanged by both sides, a Failure packet MUST be silently discarded (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerDiscardsAFailureAfterMutualSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L178) | unit/verify | revert, verified |

### [`RFC3748-2-3`](#rfc3748-2-3)

The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-2-3, so no unit is bound to it.

### [`RFC3748-2.2-1`](#rfc3748-2.2-1)

The Success, Failure, Nak Response and Notification Request/Response messages MUST NOT be used to carry data destined for delivery to other EAP methods (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-2.2-1, so no unit is bound to it.

### [`RFC3748-4.1-6`](#rfc3748-4.1-6)

Additional Request packets MUST be sent until a valid Response packet is received, an optional retry counter expires, or a lower layer failure indication is received (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-6, so no unit is bound to it.

### [`RFC3748-4.1-7`](#rfc3748-4.1-7)

The peer MUST send a Response packet in reply to a valid Request packet (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-7, so no unit is bound to it.

### [`RFC3748-4.1-8`](#rfc3748-4.1-8)

Requests MUST be processed in the order that they are received, and MUST be processed to their completion before inspecting the next Request (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-8, so no unit is bound to it.

### [`RFC3748-4.1-9`](#rfc3748-4.1-9)

A single Type MUST be specified for each EAP Request or Response (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.1-9, so no unit is bound to it.

### [`RFC3748-4.1-10`](#rfc3748-4.1-10)

An authenticator receiving a Response whose Identifier value does not match that of the currently outstanding Request MUST silently discard the Response (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthenticatorProcessesAResponseAnsweringTheOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L203) | unit/verify | unproven |
| positive | [`TestAuthenticatorDiscardsAResponseAnsweringNoOutstandingRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_identifier_test.go#L182) | unit/verify | unproven |

### [`RFC3748-4.1-11`](#rfc3748-4.1-11)

An EAP server receiving a Response whose Type is neither the outstanding Request's nor a legacy Nak MUST silently discard it (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthenticatorProcessesAResponseOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L324) | unit/verify | unproven |
| positive | [`TestAuthenticatorDiscardsAResponseOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L297) | unit/verify | unproven |

### [`RFC3748-2.1-4`](#rfc3748-2.1-4)

A peer receiving a Request of a Type other than the one under way MUST silently discard it, because an authenticator MUST NOT send a Request of a different Type before the method's final round completes (S2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerProcessesARequestOfTheMethodType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L387) | unit/verify | unproven |
| positive | [`TestPeerDiscardsARequestOfAnotherType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_discard_test.go#L347) | unit/verify | revert, verified |

### [`RFC3748-4.2-15`](#rfc3748-4.2-15)

The peer MUST silently discard a Success packet that arrives after the peer has ended the session (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-15, so no unit is bound to it.

### [`RFC3748-4.2-10`](#rfc3748-4.2-10)

Success and Failure packets MUST NOT be sent by an EAP authenticator if the specification of the given method does not explicitly permit the method to finish at that point (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-10, so no unit is bound to it.

### [`RFC3748-4.2-11`](#rfc3748-4.2-11)

A peer MUST allow for the circumstance that a Success or Failure packet, being unacknowledged, can be lost (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-11, so no unit is bound to it.

### [`RFC3748-4.2-12`](#rfc3748-4.2-12)

After the authenticator sends a failure result indication to the peer, regardless of the response from the peer, it MUST subsequently send a Failure packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-12, so no unit is bound to it.

### [`RFC3748-4.2-13`](#rfc3748-4.2-13)

After the authenticator sends a success result indication to the peer and receives a success result indication from the peer, it MUST subsequently send a Success packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-13, so no unit is bound to it.

### [`RFC3748-4.2-14`](#rfc3748-4.2-14)

If the peer attempts to authenticate to the authenticator and fails to do so, the authenticator MUST send a Failure packet and MUST NOT grant access by sending a Success packet (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-4.2-14, so no unit is bound to it.

### [`RFC3748-7.10-5`](#rfc3748-7.10-5)

The MSK and EMSK MUST NOT be used directly to protect data (S7.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-7.10-5, so no unit is bound to it.

### [`RFC3748-7.10-6`](#rfc3748-7.10-6)

The EMSK MUST remain on the EAP peer and EAP server where it is derived, and MUST NOT be transported to, shared with, or used to derive keys for additional parties (S7.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-7.10-6, so no unit is bound to it.

### [`RFC3748-7.10-7`](#rfc3748-7.10-7)

EAP peers, authenticators and authentication servers MUST be prepared for situations in which one of the parties discards the key state, which remains valid on another party (S7.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3748-7.10-7, so no unit is bound to it.

### [`RFC3748-5-1`](#rfc3748-5-1)

NAK (Type 3) and Expanded NAK (Type 254) MUST NOT be sent in a Request (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L322) | unit/verify | unproven |
| positive | [`TestRFC3748NoNAKInARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L312) | unit/verify | unproven |

### [`RFC3748-5-2`](#rfc3748-5-2)

All EAP implementations MUST support Types 1-4, which are defined in this document (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748PeerRefusesATypeOutsideOneToFour`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L387) | unit/verify | revert, verified |
| positive | [`TestRFC3748PeerSupportsTypesOneToFour`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L316) | unit/verify | revert, verified |

### [`RFC3748-5.2-1`](#rfc3748-5.2-1)

The peer MUST respond to a Notification Request with a Notification Response, unless the EAP authentication method specification prohibits the use of Notification messages (S5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestPeerAnswersANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L89) | unit/verify | revert, verified |

### [`RFC3748-5.2-2`](#rfc3748-5.2-2)

A Nak Response MUST NOT be sent in response to a Notification Request (S5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerNaksAnAuthenticationTypeButNotANotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestPeerNeverNaksANotificationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_notification_test.go#L153) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-1`](#rfc3748-5.3.1-1)

Where a peer receives a Request for an unacceptable authentication Type (4-253,255), or a peer lacking support for Expanded Types receives a Request for Type 254, a Nak Response (Type 3) MUST be sent (S5.3.1, S5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerDoesNotNakATypeItHandles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnExpandedTypeRequestWithALegacyNak`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L140) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-2`](#rfc3748-5.3.1-2)

The Type-Data field of the Nak Response (Type 3) MUST contain one or more octets indicating the desired authentication Type(s), one octet per Type, or the value zero (0) to indicate no proposed alternative (S5.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNakNamesTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestPeerNaksAnUnacceptableAuthenticationType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L147) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-3`](#rfc3748-5.3.1-3)

The Identifier field of a legacy Nak Response MUST match the Identifier field of the Request packet that it is sent in response to (S5.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L304) | unit/verify | revert, verified |
| positive | [`TestNakIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L289) | unit/verify | revert, verified |

### [`RFC3748-5.3.1-4`](#rfc3748-5.3.1-4)

The legacy Nak MUST NOT be used as a general purpose error indication, such as for communication of error messages or negotiation of method-specific parameters (S5.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L347) | unit/verify | revert, verified |
| positive | [`TestPeerDoesNotNakAMethodError`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_nak_test.go#L324) | unit/verify | revert, verified |

### [`RFC3748-5.4-1`](#rfc3748-5.4-1)

A Response MUST be sent in reply to an MD5-Challenge Request (S5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MD5ChallengeRequeryDrawsNoResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L117) | unit/verify | revert, verified |

### [`RFC3748-5.4-2`](#rfc3748-5.4-2)

EAP peer and EAP server implementations MUST support the MD5-Challenge mechanism (S5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748MD5ChallengeIsTheConfiguredMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestRFC3748MD5ChallengeSupportedByBothRoles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_md5challenge_test.go#L237) | unit/verify | revert, verified |

### [`RFC3748-5.1-2`](#rfc3748-5.1-2)

The Identity Response field MUST NOT be null terminated (S5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L373) | unit/verify | unproven |
| positive | [`TestRFC3748IdentityResponseIsNotNullTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L365) | unit/verify | unproven |

### [`RFC3748-7.5-1`](#rfc3748-7.5-1)

Where an EAP method employs a per-packet MIC, the peer and an authenticator not in pass-through mode MUST validate it (S7.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L484) | unit/verify | unproven |
| positive | [`TestRFC3748EAPTLSValidatesItsPerPacketMIC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L471) | unit/verify | unproven |

### [`RFC3748-7.10-4`](#rfc3748-7.10-4)

EAP methods deriving keys MUST provide for mutual authentication between the EAP peer and the EAP server (S7.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L517) | unit/verify | unproven |
| positive | [`TestRFC3748KeyDerivingMethodAuthenticatesBothEnds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc3748_walk_test.go#L502) | unit/verify | unproven |

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
| `2.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of a 'tunneled' EAP method specification, a document role rather than a wire role, and the sentence states what such a specification must say about running a second method inside the tunnel. Ze publishes no EAP method specification, and neither method it runs tunnels a second one: tlsMethod carries TLS records and no EAP packet (tlsMethod.Process, internal/component/ike/eap/eap_tls.go), and mschapv2Method carries the MS-CHAPv2 opcodes alone (mschapv2Method.Process, internal/component/ike/eap/eap_mschapv2.go). The producer that would act as the role if Ze did is the specification of such a method, and Ze holds only the implementations of RFC 5216 and draft-kamath-pppext-eap-mschapv2-02. | To address security vulnerabilities, "tunneled" methods MUST support protection against man-in-the-middle attacks. |
| `2.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a pass-through authenticator must be capable of forwarding a Code=2 Response to the backend authentication server. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A pass-through authenticator implementation MUST be capable of forwarding EAP packets received from the peer with Code=2 (Response) to the backend authentication server. |
| `2.3:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a pass-through authenticator must be capable of forwarding Code=1, Code=3 and Code=4 packets received from the backend authentication server to the peer. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | It also MUST be capable of receiving EAP packets from the backend authentication server and forwarding EAP packets of Code=1 (Request), Code=3 (Success), and Code=4 (Failure) to the peer. |
| `2.3:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that a compliant pass-through authenticator must by default forward EAP packets of any Type. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | For sessions in which the authenticator acts as a pass-through, it MUST determine the outcome of the authentication solely based on the Accept/Reject indication sent by the backend authentication server; the outcome MUST NOT be determined by the contents of an EAP packet sent along with the Accept/Reject indication, or the absence of such an encapsulated EAP packet. |
| `3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is a PPP implementation that has negotiated EAP as its LCP Authentication-Protocol (0xC227), which Section 3.2 titles 'EAP Usage Within PPP'. Ze never acts as it, and the producer that would is authMethodFromAuthProto (internal/component/l2tp/ppp/auth.go): it recognises PAP and CHAP alone and answers AuthMethodNone for every other Auth-Protocol value, 0xC227 included, so Ze's PPP falls back to the no-wire-auth phase rather than starting an EAP conversation. Ze carries EAP only inside the IKEv2 SK payload (startEAPExchange, internal/component/ike/engine/fsm.go). RFC 3748 obliges no implementation to support PPP as a lower layer: Section 2.2 lists PPP among the layers EAP 'has been run over', which is a description rather than a requirement. | If authentication of the link is desired, an implementation MUST specify the Authentication Protocol Configuration Option during the Link Establishment phase. |
| `4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the same-Identifier rule for a retransmitted Request. Site 4.1:8 maps the id. | Retransmitted Requests MUST be sent with the same Identifier value in order to distinguish them from new Requests. |
| `4.1:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the same-Identifier rule for a Request retransmitted on a timeout. Site 4.1:8 maps the id. | The Identifier field MUST be the same if a Request packet is retransmitted due to a timeout while waiting for a Response. |
| `4.1:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:2 maps the id. | Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored upon reception. |
| `4.1:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Repeats the Section 4 Length paragraph verbatim inside Section 4.1. Site 4:3 maps the id. | A message with the Length field set to a value larger than the number of received octets MUST be silently discarded. |
| `4.1:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates Section 2.1's ban on a Nak after an initial non-Nak Response. Site 2.1:3 maps the id. | A peer MUST NOT send a Nak (legacy or expanded) in response to a Request, after an initial non-Nak Response has been sent. |
| `4.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Failure arm of the same obligation, one sentence after the Success arm. Site 4.2:1 maps the id. | If the authenticator cannot authenticate the peer (unacceptable Responses to one or more Requests), then after unsuccessful completion of the EAP method in progress, the implementation MUST transmit an EAP packet with the Code field set to 4 (Failure). |
| `4.2:15` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of an unexercised MAY: 'However, an authenticator MAY omit having the peer authenticate to it in situations where limited access is offered (e.g., guest access).  In this case, the authenticator MUST send a Success packet.' Ze's authenticator offers no guest access: every path to a Success runs through a completed method (Session.handleMethod, internal/component/ike/eap/eap.go). | In this case, the authenticator MUST send a Success packet. |
| `4.3:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of a MAY inside a RECOMMENDED algorithm: '...the retransmission timer is calculated with a jitter by using the RTO value and randomly adding a value drawn between -RTOmin/2 and RTOmin/2.  Alternative calculations to create jitter MAY be used.  These MUST be pseudo-random.' Ze runs no EAP-layer retransmission timer at all, which Section 4.3 itself directs for a reliable lower layer. | These MUST be pseudo-random. |
| `5.2:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The MUST is the consequent of a MAY, and the enclosing construction is: 'An EAP method MAY indicate within its specification that Notification messages must not be sent during that method.  In this case, the peer MUST silently discard Notification Requests from the point where an initial Request for that Type is answered with a Response of the same Type.' The antecedent is false for every method Ze runs. Neither specification exercises that MAY: the string 'Notification' appears nowhere in rfc/full/rfc5216.txt and nowhere in rfc/full/rfc2759.txt, so no method Ze offers prohibits Notification messages and the peer is never in the state this sentence describes. What Ze owes a Notification Request OUTSIDE that state is Section 5.2's opening obligation, which sites 5.2:1 and 5.2:5 relocate to plan/spec-eap-notification-and-nak.md under RFC3748-5.2-1. | In this case, the peer MUST silently discard Notification Requests from the point where an initial Request for that Type is answered with a Response of the same Type. |
| `5.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on SENDING a Notification Request, which RFC 3748 Section 5.2 states as an option in its own words: "An authenticator MAY send a Notification Request to the peer at any time when there is no outstanding Request, prior to completion of an EAP authentication method." The owner decided on 2026-09-01 (D-2 of plan/spec-eap-notification-and-nak.md) that ze's authenticator sends none, and Section 5.2 itself notes "In most circumstances, Notification should not be required." No producer composes a Type-2 Request: Session.Begin issues Identity and Session.handleMethod issues only the method's own packets (internal/component/ike/eap/eap.go). The absent FEATURE is disclosed in rfc/short/rfc3748.md and a later scope decision can revisit it. Ze's peer ANSWERS a Notification Request, which is the mandatory half and is RFC3748-5.2-1. | The message MUST NOT be null terminated. |
| `5.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates Section 5.2's opening obligation to answer a Notification Request with a Notification Response, which site 5.2:1 maps to RFC3748-5.2-1. | A Response MUST be sent in reply to the Request with a Type field of 2 (Notification). |
| `5.3.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states when an Expanded Nak may be sent. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/component/ike/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | It MUST be sent only in reply to a Request of Type 254 (Expanded Type) where the authentication Type is unacceptable. |
| `5.3.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the ban on an Expanded Nak as a general error indication. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/component/ike/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | Since the Expanded Nak Type is valid only in Responses and has very limited functionality, it MUST NOT be used as a general purpose error indication, such as for communication of error messages, or negotiation of parameters specific to a particular EAP method. |
| `5.3.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the Expanded Nak Identifier rule. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/component/ike/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Identifier field of an Expanded Nak Response MUST match the Identifier field of the Request packet that it is sent in response to. |
| `5.3.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Expanded Type (254) namespace, which Ze does not offer. The sentence states the Expanded Nak Vendor-Data contents. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' TypeExpandedEAP is a bare constant with no producer and NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered (internal/component/ike/eap/eap.go). Section 5.7 routes a peer that cannot interpret an Expanded Type to the LEGACY Nak of Section 5.3.1 instead, which is site 5.7:2, so declining Type 254 leaves Ze a conformant answer rather than none. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Vendor-Data field of the Nak Response MUST contain one or more authentication Types (4 or greater), all in expanded format, 8 octets per Type, or the value zero (0), also in Expanded Type format, to indicate no proposed alternative. |
| `5.4:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states what an authenticator that supports only pass-through must do with an MD5-Challenge Response. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | An authenticator that supports only pass- through MUST allow communication with a backend authentication server that is capable of supporting MD5-Challenge, although the EAP authenticator implementation need not support MD5-Challenge itself. |
| `5.4:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation is [RFC1994]'s, cited by this sentence: 'while [RFC1994] states that both the Identifier and Challenge fields MUST change each time a Challenge ... is sent'. | EAP allows for retransmission of MD5-Challenge Request packets, while [RFC1994] states that both the Identifier and Challenge fields MUST change each time a Challenge (the CHAP equivalent of the MD5-Challenge Request packet) is sent. |
| `5.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states answering an OTP Request. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request. |
| `5.5:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the Type of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Response MUST be of Type 5 (OTP), Nak (Type 3), or Expanded Nak (Type 254). |
| `5.5:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the ban on cleartext passwords. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The EAP OTP method is intended for use with the One-Time Password system only, and MUST NOT be used to provide support for cleartext passwords. |
| `5.5:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the One Time Password method (Type 5), which Ze does not offer. The sentence states the message not being null terminated. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 5 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The messages MUST NOT be null terminated. |
| `5.6:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states answering a GTC Request. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request. |
| `5.6:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the Type of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The Response MUST be of Type 6 (GTC), Nak (Type 3), or Expanded Nak (Type 254). |
| `5.6:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the ban on cleartext passwords outside a protected tunnel. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The EAP GTC method is intended for use with the Token Cards supporting challenge/response authentication and MUST NOT be used to provide support for cleartext passwords in the absence of a protected tunnel with server authentication. |
| `5.6:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the message not being null terminated. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | The message MUST NOT be null terminated. |
| `5.6:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on the Generic Token Card method (Type 6), which Ze does not offer. The sentence states the Type field of the answering Response. RFC 3748 Section 5 states the option in its own words: 'All EAP implementations MUST support Types 1-4, which are defined in this document, and SHOULD support Type 254.  Implementations MAY support other Types defined here or in future RFCs.' NewSession (internal/component/ike/eap/eap.go) builds a method for Type 4, Type 13 and Type 26 and refuses every other type, so no other Type is offered, and Type 6 derives no MSK, so RFC 7296 Section 2.16 would key its AUTH payloads from SK_pi and SK_pr; RFC3748-7.10-3 states that as a SHOULD NOT rather than a prohibition. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | A Response MUST be sent in reply to the Request with a Type field of 6 (Generic Token Card). |
| `5.7:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Nak this sentence demands IS the legacy Nak of Section 5.3.1, which it cites by number, so the obligation is the one site 5.3.1:3 maps to RFC3748-5.3.1-1. PeerSession.naks routes Type 254 to that same builder rather than composing an Expanded Nak (internal/component/ike/eap/peer.go). | Peers not equipped to interpret the Expanded Type MUST send a Nak as described in Section 5.3.1, and negotiate a more suitable authentication method. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the Security Claims section a method specification must include. | In order to clearly articulate the security provided by an EAP method, EAP method specifications MUST include a Security Claims section, including the following declarations: |
| `7.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the effective key strength estimate. | If the method derives keys, then the effective key strength MUST be estimated. |
| `7.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the key hierarchy reference or MSK/EMSK derivation description. | EAP methods deriving keys MUST either provide a reference to a key hierarchy specification, or describe how Master Session Keys (MSKs) and Extended Master Session Keys (EMSKs) are to be derived. |
| `7.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the claims that are NOT being made. | In addition to the security claims that are made, the specification MUST indicate which of the security claims detailed in Section 7.2.1 are NOT being made. |
| `7.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. Section 7.2.1 is the claims VOCABULARY a specification writes its Security Claims section in, and this sentence states describing the EAP packets and fields protected. | When making this claim, a method specification MUST describe the EAP packets and fields within the EAP packet that are protected. |
| `7.2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. Section 7.2.1 is the claims VOCABULARY a specification writes its Security Claims section in, and this sentence states the identity protection a claiming method must support. | A method making this claim MUST support identity protection (see Section 7.3). |
| `7.10:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states that keying material a method exports must be independent of the ciphersuite negotiated to protect data, which is a property of the DERIVATION a specification defines. The two Ze runs are defined elsewhere: RFC 5216 Section 2.3 for EAP-TLS (exportEAPTLSMSK, internal/component/ike/eap/eap_tls.go) and draft-kamath-pppext-eap-mschapv2-02 for EAP-MSCHAPv2 (DeriveMSK, internal/component/ike/eap/mschapv2.go). | Keying material exported by EAP methods MUST be independent of the ciphersuite negotiated to protect data. |
| `7.10:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states cryptographic separation between the MSK and EMSK branches, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Methods supporting key derivation MUST demonstrate cryptographic separation between the MSK and EMSK branches of the EAP key hierarchy. |
| `7.10:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the non-recoverability of one key from the other, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Without violating a fundamental cryptographic assumption (such as the non-invertibility of a one-way function), an attacker recovering the MSK or EMSK MUST NOT be able to recover the other quantity with a level of effort less than brute force. |
| `7.10:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the separation of non-overlapping MSK substrings, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Non-overlapping substrings of the MSK MUST be cryptographically separate from each other, as defined in Section 7.2.1. |
| `7.10:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the same property stated as a knowledge bound, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | That is, knowledge of one substring MUST NOT help in recovering some other substring without breaking some hard cryptographic assumption. |
| `7.10:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the separation of non-overlapping EMSK substrings, which is a property a method's key hierarchy must be SHOWN to have. The showing is the specification's, and Ze holds the implementations of the two it runs. | Likewise, non-overlapping substrings of the EMSK MUST be cryptographically separate from each other, and from substrings of the MSK. |
| `7.13:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on pass-through operation, which Ze does not offer. The sentence states that the AAA protocol spoken between an authenticator and a backend authentication server must support per-packet authentication. Section 7.13 states its own antecedent, 'in the case where the authenticator and authentication server reside on different machines', which is pass-through. RFC 3748 Section 2 states the option in its own words: 'Network Access Server (NAS) devices (e.g., a switch or access point) do not have to understand each authentication method and MAY act as a pass-through agent for a backend authentication server.  Support for pass-through is optional.' NewSession (internal/component/ike/eap/eap.go) builds the method in process and the IKE engine has no AAA back end for EAP: nothing under internal/component/radius/ produces an EAP-Message attribute, and handleResponderEAP (internal/component/ike/engine/responder_eap.go) answers from the local eap.Session alone. The absent feature is disclosed in docs/features/rfc-status.md as an implementation gap a later scope decision can revisit, never as a conformance gap. | In practice, this implies that the AAA protocol spoken between the authenticator and authentication server MUST support per-packet authentication, integrity, and replay protection. |
| `7.16:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states which result indications are protected, which a specification claiming protected result indications must document. | A method supporting protected result indications MUST indicate which result indications are protected, and which are not. |
| `7.16:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an EAP METHOD SPECIFICATION, which is a document role rather than a wire role: Section 7.2 addresses it as 'EAP method specifications MUST include a Security Claims section'. Ze publishes no EAP method specification. The producer that would act as the role if Ze did is the specification itself, and for the two methods Ze runs those documents are RFC 5216 (EAP-TLS) and draft-kamath-pppext-eap-mschapv2-02 (EAP-MSCHAPv2); newTLSMethod and newMSCHAPv2Method (internal/component/ike/eap/eap.go) implement what those two state, and state nothing themselves. This sentence states the four claims a protected-result-indication method must also support, which a specification claiming protected result indications must document. | Since protected result indications require use of a key for per-packet authentication and integrity protection, methods supporting protected result indications MUST also support the "key derivation", "mutual authentication", "integrity protection", and "replay protection" claims. |

## Superseded

No document obsoletes RFC 3748, so its obligations are stated where they were written.
