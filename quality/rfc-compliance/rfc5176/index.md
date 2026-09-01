# RFC 5176 - Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS)

Supported for subscriber access. Every requirement this repository extracted from RFC 5176, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 95.5% | 21 of 22 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.5% | 1 of 22 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 22 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 57.7% | 30 of 52 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 22 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported for subscriber access |
| Enrolment | Enrolled |
| Requirements | 23 |
| Gated MUST-level | 22 |
| Obligations that bind Ze | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 52 |
| Tagged units | 52 |
| Recorded audit verdicts | 0 |
| Discrimination records | 30 |
| Summary | `rfc/short/rfc5176.md` |
| Requirement shard | `rfc/requirements/rfc5176.md` |
| RFC text | `rfc/full/rfc5176.txt` |

## Enrolment

Enrolled: RADIUS Dynamic Authorization Extensions (CoA/Disconnect): five MUST-level requirements, all met by ze's Dynamic Authorization Server (internal/component/l2tp/plugins/authradius/coa.go, wired at register.go). 3.5-1 (verify Request Authenticator before processing), 3.5-2 (silently discard invalid authenticators), 3.3-1 (require at least one session-identification attribute), and 3.5-4 (Request Authenticator = MD5 over the RFC 2865 fields) each carry positive+negative tags on the CoA listener and packet tests. 3.5-3 (Response Authenticator per RFC 2865) is {single-polarity: positive}: ze only emits responses, so there is no inbound Response Authenticator to reject.

## What the public ledger says

**Status:** Supported for subscriber access

**What the ledger says is covered**

CoA/DM listener for RADIUS-initiated changes and disconnects: Request Authenticator and optional Message-Authenticator verification, source-address allow list, duplicate detection and cached replay, Event-Timestamp window, mandatory-attribute handling with Error-Cause 401, Service-Type refusal with 405, multiple-match refusal with 508, and Proxy-State and State echoed unread. Tests bound per requirement in [`rfc/requirements/rfc5176.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5176.md), and the checklist is bounded by [`rfc/extraction/rfc5176.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc5176.json). <!-- source: [`internal/component/l2tp/plugins/authradius/coa.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa.go) -- handlePacket/handleCoA/handleDisconnect/sendResponse -->

**What the ledger says remains**

Scoped to subscriber access. Two OPTIONAL features of the RFC are out of scope, so the obligations conditional on them are excluded rather than gated: the Section 3.2 "Authorize Only" Service-Type exchange, which ze answers with a CoA-NAK and Error-Cause 405, and the RFC 2865 Section 5.29 Termination-Action re-authorization, for which ze sends no Access-Request.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 21 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (21):** [`RFC5176-3.5-1`](#rfc5176-3.5-1), [`RFC5176-3.5-2`](#rfc5176-3.5-2), [`RFC5176-3.3-1`](#rfc5176-3.3-1), [`RFC5176-3.5-4`](#rfc5176-3.5-4), [`RFC5176-2.3-1`](#rfc5176-2.3-1), [`RFC5176-2.3-2`](#rfc5176-2.3-2), [`RFC5176-2.3-3`](#rfc5176-2.3-3), [`RFC5176-2.3-4`](#rfc5176-2.3-4), [`RFC5176-2.3-5`](#rfc5176-2.3-5), [`RFC5176-2.3-6`](#rfc5176-2.3-6), [`RFC5176-2.3-7`](#rfc5176-2.3-7), [`RFC5176-3.1-1`](#rfc5176-3.1-1), [`RFC5176-3.2-1`](#rfc5176-3.2-1), [`RFC5176-3.3-2`](#rfc5176-3.3-2), [`RFC5176-3.4-1`](#rfc5176-3.4-1), [`RFC5176-3.4-2`](#rfc5176-3.4-2), [`RFC5176-3.4-3`](#rfc5176-3.4-3), [`RFC5176-3.5-5`](#rfc5176-3.5-5), [`RFC5176-3.6-1`](#rfc5176-3.6-1), [`RFC5176-6.1-1`](#rfc5176-6.1-1), [`RFC5176-6.3-1`](#rfc5176-6.3-1)

**Annotated instead of tested (1):** [`RFC5176-3.5-3`](#rfc5176-3.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5176-3.5-1` | A CoA-Request or Disconnect-Request MUST have its Request Authenticator verified before any attribute of it is acted on (anchored §3.5; the sentence it enforces is at §2.3, "The Authenticator field MUST be calculated in the same way as is specified for an Accounting-Request in [RFC2866]", and a value nobody checks authenticates nothing) | MUST | 3.5 | **positive:** `unit/verify` [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L337). **negative:** `unit/verify` [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L250) |
| `RFC5176-3.5-2` | A CoA-Request or Disconnect-Request whose Request Authenticator does not match MUST be discarded with no response emitted (anchored §3.5; a sender that cannot compute the Authenticator holds no shared secret, so §6.1, "A Dynamic Authorization Server MUST silently discard Disconnect-Request or CoA-Request packets from untrusted sources", covers it) | MUST | 3.5 | **positive:** `unit/verify` [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L252). **negative:** `unit/verify` [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L339) |
| `RFC5176-3.3-1` | The combination of NAS and session identification attributes in a CoA-Request or Disconnect-Request MUST match at least one session for the request to succeed, and a request matching none MUST be answered with a CoA-NAK or a Disconnect-NAK (anchored §3.3; stated at §3) | MUST | 3.3 | **positive:** `unit/verify` [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L402). **negative:** `unit/verify` [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) |
| `RFC5176-3.5-3` | Response Authenticator MUST be computed per RFC 2865 Section 3 (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66). **negative:** no negative test. **{single-polarity}:** the NAS only emits CoA/Disconnect responses and never receives one, so there is no inbound Response Authenticator to reject; correctness is proven by verifying the emitted authenticator against radius.ResponseAuthenticator (internal/component/radius/packet.go:145) |
| `RFC5176-3.5-4` | Request Authenticator MUST be computed as MD5(Code + Identifier + Length + 16-zero-octets + Attributes + Secret) (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L183). **positive:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L352). **negative:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L218). **negative:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L358) |
| `RFC5176-2.3-1` | A packet received with an invalid Code field MUST be silently discarded (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L133). **negative:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L136) |
| `RFC5176-2.3-2` | A Dynamic Authorization Server MUST detect a duplicate request carrying the same source address, Identifier and Request Authenticator within a short span of time, and MUST answer it with the response the first copy earned (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L155). **negative:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L158) |
| `RFC5176-2.3-3` | Octets outside the range of the Length field MUST be treated as padding and ignored on reception, and a packet shorter than its Length field indicates MUST be silently discarded (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L192). **negative:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L194) |
| `RFC5176-2.3-4` | The Dynamic Authorization Server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L216). **negative:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L219) |
| `RFC5176-2.3-5` | Every attribute of a CoA-Request or Disconnect-Request MUST be treated as mandatory, so a request carrying an attribute the NAS does not support MUST be answered with a CoA-NAK or a Disconnect-NAK; a Disconnect-Request MUST carry only NAS and session identification attributes (§2.3, restated at §3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255). **negative:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) |
| `RFC5176-2.3-6` | A CoA-Request whose authorization changes cannot all be carried out MUST be answered with a CoA-NAK and MUST leave the matching session unchanged, and a Disconnect-Request that cannot terminate the matching session MUST be answered with a Disconnect-NAK (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293). **negative:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295) |
| `RFC5176-2.3-7` | When the identification attributes match more than one session, a NAS that supports multi-session requests MUST apply the request to all of them, and a NAS that does not MUST answer with a CoA-NAK or a Disconnect-NAK (§2.3, with the apply-to-all branch stated at §3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325). **negative:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) |
| `RFC5176-3.1-1` | The Dynamic Authorization Server MUST include the request's Proxy-State attributes in its response, unmodified, in the order they arrived, and treated as opaque data (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L363). **negative:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) |
| `RFC5176-3.2-1` | A NAS MUST answer a CoA-Request carrying a Service-Type Attribute whose value it does not support, "Authorize Only" included, with a CoA-NAK, and MUST NOT answer it with a CoA-ACK (§3.2, with the unsupported-value branch stated at §2.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L397). **negative:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L400) |
| `RFC5176-3.3-2` | The Dynamic Authorization Server MUST NOT interpret the State Attribute locally, and MUST send it unmodified in the ACK or NAK it returns (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L427). **negative:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) |
| `RFC5176-3.4-1` | When the HMAC-MD5 message integrity check of a CoA-Request or Disconnect-Request is calculated, the Request Authenticator field and the Message-Authenticator Attribute MUST each be considered to be sixteen octets of zero (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L456). **positive:** `unit/verify` [`radius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L126). **negative:** `unit/verify` [`TestRFC5176MessageAuthenticatorRefusesEveryOtherStream`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L148). **negative:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L460) |
| `RFC5176-3.4-2` | The Message-Authenticator Attribute is calculated and inserted in the packet before the Request Authenticator is calculated, so the Request Authenticator MUST cover the Message-Authenticator value as sent (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L218). **negative:** `unit/verify` [`TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L239) |
| `RFC5176-3.4-3` | A Dynamic Authorization Server receiving a CoA-Request or Disconnect-Request with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176ListenerAcceptsConformantMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L26). **negative:** `unit/verify` [`TestRFC5176ListenerDiscardsWrongMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L52). **negative:** `unit/verify` [`TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L71) |
| `RFC5176-3.4-4` | The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK and Disconnect-NAK packets in order to prevent spoofing, so a request that carries none is answered rather than discarded; the `require-message-authenticator` leaf turns its absence into a discard for an operator who wants the Blast-RADIUS mitigation (§3.4) | MAY | 3.4 | **positive:** `unit/verify` [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26). **negative:** `unit/verify` [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L307) |
| `RFC5176-3.5-5` | An Error-Cause value in the 200-299 range MUST NOT be sent within a CoA-NAK or Disconnect-NAK, a value in the 400-599 range MUST NOT be sent within a CoA-ACK or Disconnect-ACK, 202 MUST NOT be sent by an implementation of this specification, 502 MUST NOT be sent by a NAS, 201 MUST NOT leave a packet other than a Disconnect-ACK and 504 MUST NOT leave a packet other than a Disconnect-NAK (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496). **negative:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) |
| `RFC5176-3.6-1` | NAS and session identification attributes MUST NOT be used for a purpose other than identification, and the same Vendor-Specific Attribute MUST NOT serve identification and authorization change at the same time (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L604). **negative:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607) |
| `RFC5176-6.1-1` | A Dynamic Authorization Server MUST silently discard Disconnect-Request or CoA-Request packets from untrusted sources, so a source that is not a configured Dynamic Authorization Client is refused; an EMPTY allow list means no configured server resolved and refuses every source rather than accepting all of them (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119). **positive:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L652). **negative:** `unit/verify` [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173). **negative:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655) |
| `RFC5176-6.3-1` | When an Event-Timestamp Attribute is present the Dynamic Authorization Server MUST check that it is current within an acceptable time window, and MUST silently discard the packet when it is not; that window MUST be the one used for duplicate detection (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L710). **negative:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L714) |

## Gaps and untested MUSTs

RFC 5176 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5176-3.5-1`](#rfc5176-3.5-1)

A CoA-Request or Disconnect-Request MUST have its Request Authenticator verified before any attribute of it is acted on (anchored §3.5; the sentence it enforces is at §2.3, "The Authenticator field MUST be calculated in the same way as is specified for an Accounting-Request in [RFC2866]", and a value nobody checks authenticates nothing)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L250) | unit/verify | unproven |
| positive | [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L337) | unit/verify | unproven |

### [`RFC5176-3.5-2`](#rfc5176-3.5-2)

A CoA-Request or Disconnect-Request whose Request Authenticator does not match MUST be discarded with no response emitted (anchored §3.5; a sender that cannot compute the Authenticator holds no shared secret, so §6.1, "A Dynamic Authorization Server MUST silently discard Disconnect-Request or CoA-Request packets from untrusted sources", covers it)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L339) | unit/verify | unproven |
| positive | [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L252) | unit/verify | unproven |

### [`RFC5176-3.3-1`](#rfc5176-3.3-1)

The combination of NAS and session identification attributes in a CoA-Request or Disconnect-Request MUST match at least one session for the request to succeed, and a request matching none MUST be answered with a CoA-NAK or a Disconnect-NAK (anchored §3.3; stated at §3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) | unit/verify | unproven |
| positive | [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L402) | unit/verify | unproven |

### [`RFC5176-3.5-3`](#rfc5176-3.5-3)

Response Authenticator MUST be computed per RFC 2865 Section 3 (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66) | unit/verify | unproven |

### [`RFC5176-3.5-4`](#rfc5176-3.5-4)

Request Authenticator MUST be computed as MD5(Code + Identifier + Length + 16-zero-octets + Attributes + Secret) (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L358) | unit/verify | unproven |
| negative | [`TestRFC5176CoARequestAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L218) | unit/verify | unproven |
| positive | [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L352) | unit/verify | unproven |
| positive | [`TestRFC5176CoARequestAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L183) | unit/verify | unproven |

### [`RFC5176-2.3-1`](#rfc5176-2.3-1)

A packet received with an invalid Code field MUST be silently discarded (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L133) | unit/verify | revert, verified |

### [`RFC5176-2.3-2`](#rfc5176-2.3-2)

A Dynamic Authorization Server MUST detect a duplicate request carrying the same source address, Identifier and Request Authenticator within a short span of time, and MUST answer it with the response the first copy earned (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L155) | unit/verify | revert, verified |

### [`RFC5176-2.3-3`](#rfc5176-2.3-3)

Octets outside the range of the Length field MUST be treated as padding and ignored on reception, and a packet shorter than its Length field indicates MUST be silently discarded (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L192) | unit/verify | revert, verified |

### [`RFC5176-2.3-4`](#rfc5176-2.3-4)

The Dynamic Authorization Server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L219) | unit/verify | revert, verified |
| positive | [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L216) | unit/verify | revert, verified |

### [`RFC5176-2.3-5`](#rfc5176-2.3-5)

Every attribute of a CoA-Request or Disconnect-Request MUST be treated as mandatory, so a request carrying an attribute the NAS does not support MUST be answered with a CoA-NAK or a Disconnect-NAK; a Disconnect-Request MUST carry only NAS and session identification attributes (§2.3, restated at §3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255) | unit/verify | revert, verified |

### [`RFC5176-2.3-6`](#rfc5176-2.3-6)

A CoA-Request whose authorization changes cannot all be carried out MUST be answered with a CoA-NAK and MUST leave the matching session unchanged, and a Disconnect-Request that cannot terminate the matching session MUST be answered with a Disconnect-NAK (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295) | unit/verify | revert, verified |
| positive | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293) | unit/verify | revert, verified |

### [`RFC5176-2.3-7`](#rfc5176-2.3-7)

When the identification attributes match more than one session, a NAS that supports multi-session requests MUST apply the request to all of them, and a NAS that does not MUST answer with a CoA-NAK or a Disconnect-NAK (§2.3, with the apply-to-all branch stated at §3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) | unit/verify | revert, verified |
| positive | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325) | unit/verify | revert, verified |

### [`RFC5176-3.1-1`](#rfc5176-3.1-1)

The Dynamic Authorization Server MUST include the request's Proxy-State attributes in its response, unmodified, in the order they arrived, and treated as opaque data (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) | unit/verify | revert, verified |
| positive | [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L363) | unit/verify | revert, verified |

### [`RFC5176-3.2-1`](#rfc5176-3.2-1)

A NAS MUST answer a CoA-Request carrying a Service-Type Attribute whose value it does not support, "Authorize Only" included, with a CoA-NAK, and MUST NOT answer it with a CoA-ACK (§3.2, with the unsupported-value branch stated at §2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L400) | unit/verify | revert, verified |
| positive | [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L397) | unit/verify | revert, verified |

### [`RFC5176-3.3-2`](#rfc5176-3.3-2)

The Dynamic Authorization Server MUST NOT interpret the State Attribute locally, and MUST send it unmodified in the ACK or NAK it returns (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) | unit/verify | revert, verified |
| positive | [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L427) | unit/verify | revert, verified |

### [`RFC5176-3.4-1`](#rfc5176-3.4-1)

When the HMAC-MD5 message integrity check of a CoA-Request or Disconnect-Request is calculated, the Request Authenticator field and the Message-Authenticator Attribute MUST each be considered to be sixteen octets of zero (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L460) | unit/verify | revert, verified |
| negative | [`TestRFC5176MessageAuthenticatorRefusesEveryOtherStream`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L148) | unit/verify | unproven |
| positive | [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L456) | unit/verify | revert, verified |
| positive | [`radius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L126) | unit/verify | unproven |

### [`RFC5176-3.4-2`](#rfc5176-3.4-2)

The Message-Authenticator Attribute is calculated and inserted in the packet before the Request Authenticator is calculated, so the Request Authenticator MUST cover the Message-Authenticator value as sent (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L239) | unit/verify | unproven |
| positive | [`TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L218) | unit/verify | unproven |

### [`RFC5176-3.4-3`](#rfc5176-3.4-3)

A Dynamic Authorization Server receiving a CoA-Request or Disconnect-Request with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L71) | unit/verify | unproven |
| negative | [`TestRFC5176ListenerDiscardsWrongMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC5176ListenerAcceptsConformantMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L26) | unit/verify | unproven |

### [`RFC5176-3.4-4`](#rfc5176-3.4-4)

The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK and Disconnect-NAK packets in order to prevent spoofing, so a request that carries none is answered rather than discarded; the `require-message-authenticator` leaf turns its absence into a discard for an operator who wants the Blast-RADIUS mitigation (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L307) | unit/verify | unproven |
| positive | [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26) | unit/verify | unproven |

### [`RFC5176-3.5-5`](#rfc5176-3.5-5)

An Error-Cause value in the 200-299 range MUST NOT be sent within a CoA-NAK or Disconnect-NAK, a value in the 400-599 range MUST NOT be sent within a CoA-ACK or Disconnect-ACK, 202 MUST NOT be sent by an implementation of this specification, 502 MUST NOT be sent by a NAS, 201 MUST NOT leave a packet other than a Disconnect-ACK and 504 MUST NOT leave a packet other than a Disconnect-NAK (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) | unit/verify | revert, verified |
| positive | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496) | unit/verify | revert, verified |

### [`RFC5176-3.6-1`](#rfc5176-3.6-1)

NAS and session identification attributes MUST NOT be used for a purpose other than identification, and the same Vendor-Specific Attribute MUST NOT serve identification and authorization change at the same time (§3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607) | unit/verify | revert, verified |
| positive | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L604) | unit/verify | revert, verified |

### [`RFC5176-6.1-1`](#rfc5176-6.1-1)

A Dynamic Authorization Server MUST silently discard Disconnect-Request or CoA-Request packets from untrusted sources, so a source that is not a configured Dynamic Authorization Client is refused; an EMPTY allow list means no configured server resolved and refuses every source rather than accepting all of them (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173) | unit/verify | unproven |
| negative | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655) | unit/verify | revert, verified |
| positive | [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119) | unit/verify | unproven |
| positive | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L652) | unit/verify | revert, verified |

### [`RFC5176-6.3-1`](#rfc5176-6.3-1)

When an Event-Timestamp Attribute is present the Dynamic Authorization Server MUST check that it is current within an acceptable time window, and MUST silently discard the packet when it is not; that window MUST be the one used for duplicate detection (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L714) | unit/verify | revert, verified |
| positive | [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L710) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-6-supported-extraction-signoff, RFC 5176 conformance package |
| Signed off | 2026-09-01 |
| Register | rfc2119 |
| Source | rfc/full/rfc5176.txt |
| Source fingerprint | 4852faf09c5bbdd6 |
| Record | rfc/extraction/rfc5176.json |
| Mapped sentences | 44 |
| Declined as scope | 28 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title, status, copyright and table of contents | 0 | skipped (front-matter) | Title, status, copyright and table of contents. |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 19 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `3.1` | not stated | 11 | walked | not stated |
| `3.2` | not stated | 7 | walked | not stated |
| `3.3` | not stated | 9 | walked | not stated |
| `3.4` | not stated | 4 | walked | not stated |
| `3.5` | not stated | 7 | walked | not stated |
| `3.6` | not stated | 3 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | skipped (iana) | IANA Considerations: it allocates Error-Cause values 407 and 508 and binds IANA, not an implementation. |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 2 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 4 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | Reference list | 0 | skipped (references) | Reference list. |
| `8.1` | Normative references | 0 | skipped (references) | Normative references. |
| `8.2` | Informative references | 0 | skipped (references) | Informative references. |
| `9` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `A` | not stated | 0 | skipped (appendix-non-normative) | Appendix A, Changes from RFC 3576: a change log against the obsoleted document. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Identifier management by the sender. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | The Identifier field MUST be changed whenever the content of the Attributes field changes, or whenever a valid reply has been received for a previous request. |
| `2.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Identifier reuse on retransmission by the sender. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | For retransmissions where the contents are identical, the Identifier MUST remain unchanged. |
| `2.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Retransmission by the sender. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | If the Dynamic Authorization Client is retransmitting a Disconnect-Request or CoA-Request to the same Dynamic Authorization Server as before, and the attributes haven't changed, the same Request Authenticator, Identifier, and source port MUST be used. |
| `2.3:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Authenticator and Identifier choice by the sender. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | If any attributes have changed, a new Authenticator and Identifier MUST be used. |
| `2.3:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Failover to a secondary DAS by the sender. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | Since this represents a new request, a new Request Authenticator and Identifier MUST be used. |
| `3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | What a Disconnect-Request MUST contain, an obligation on the composer. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them. The receive-side counterpart is site 3:4, which binds ze and maps to RFC5176-2.3-5 | A Disconnect-Request MUST contain only NAS and session identification attributes. |
| `3.1:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | The forwarding proxy MUST NOT modify any other Proxy-State attributes that were in the packet; it may choose not to forward them, but it MUST NOT change their contents. |
| `3.1:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | If the forwarding proxy omits the Proxy-State attributes in the request, it MUST attach them to the response before sending it. |
| `3.1:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | When the proxy forwards a Disconnect-Request or CoA-Request, it MAY add a Proxy-State Attribute, but it MUST NOT add more than one. |
| `3.1:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | If a Proxy-State Attribute is added to a packet when forwarding the packet, the Proxy-State Attribute MUST be added after any existing Proxy-State attributes. |
| `3.1:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | The forwarding proxy MUST NOT change the order of any attributes of the same type, including Proxy-State. |
| `3.1:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | When the proxy receives a response to a CoA-Request or Disconnect- Request, it MUST remove its own Proxy-State Attribute (the last Proxy-State in the packet) before forwarding the response. |
| `3.1:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS forwarding proxy. ze forwards no CoA or Disconnect packet: coaListener.handlePacket (internal/component/l2tp/plugins/authradius/coa.go) dispatches to handleCoA or handleDisconnect and both answer locally, and no other file in the tree emits a code in the 40-45 range | Since Disconnect and CoA responses are authenticated on the entire packet contents, the stripping of the Proxy-State Attribute invalidates the integrity check, so the proxy MUST recompute it. |
| `3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | What a Disconnect-Request MUST NOT contain, an obligation on the composer. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them. The receive side is disconnectSupportedAttrs (internal/component/l2tp/plugins/authradius/coa.go), which omits Service-Type so such a request is answered with a Disconnect-NAK under RFC5176-2.3-5 | A Service-Type Attribute MUST NOT be included within a Disconnect-Request. |
| `3.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | What an Authorize Only CoA-Request MUST contain, an obligation on the composer. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them. The receive-side counterpart is site 3.2:5, which maps to RFC5176-3.2-1 | A CoA-Request containing a Service-Type Attribute with value "Authorize Only" MUST in addition contain only NAS or session identification attributes, as well as a State Attribute. |
| `3.2:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 5176 Section 3.2: "Support for a CoA-Request including a Service-Type Attribute with value \\"Authorize Only\\" is OPTIONAL on the NAS and Dynamic Authorization Client." The owner declined the feature on 2026-08-31, and handlePacket (internal/component/l2tp/plugins/authradius/coa.go) shows it: every CoA-Request carrying a Service-Type is answered with a CoA-NAK and Error-Cause 405 before any authorization change is read, so no Authorize Only exchange ever starts. The absent feature is disclosed in the RFC 5176 row of docs/features/rfc-status.md | If a CoA-Request packet including a Service-Type value of "Authorize Only" is successfully processed, the NAS MUST respond with a CoA-NAK containing a Service-Type Attribute with value "Authorize Only", and an Error-Cause Attribute with value 507 (Request Initiated). |
| `3.2:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 5176 Section 3.2: "Support for a CoA-Request including a Service-Type Attribute with value \\"Authorize Only\\" is OPTIONAL on the NAS and Dynamic Authorization Client." The owner declined the feature on 2026-08-31, and handlePacket (internal/component/l2tp/plugins/authradius/coa.go) shows it: every CoA-Request carrying a Service-Type is answered with a CoA-NAK and Error-Cause 405 before any authorization change is read, so no Authorize Only exchange ever starts. The absent feature is disclosed in the RFC 5176 row of docs/features/rfc-status.md | The NAS then MUST send an Access-Request to the RADIUS server including a Service-Type Attribute with value "Authorize Only", along with a State Attribute. |
| `3.3:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Block-quoted RFC 2865 Section 5.44, introduced by "[RFC2865], Section 5.44 states:". The obligation is RFC 2865's and is carried by rfc/short/rfc2865.md | An Access-Request MUST contain either a User-Password or a CHAP-Password or State. |
| `3.3:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The second sentence of the same block quote of RFC 2865 Section 5.44. The obligation is RFC 2865's | An Access-Request MUST NOT contain both a User-Password and a CHAP-Password. |
| `3.3:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 5176 Section 3.2: "Support for a CoA-Request including a Service-Type Attribute with value \\"Authorize Only\\" is OPTIONAL on the NAS and Dynamic Authorization Client." The owner declined the feature on 2026-08-31, and handlePacket (internal/component/l2tp/plugins/authradius/coa.go) shows it: every CoA-Request carrying a Service-Type is answered with a CoA-NAK and Error-Cause 405 before any authorization change is read, so no Authorize Only exchange ever starts. The absent feature is disclosed in the RFC 5176 row of docs/features/rfc-status.md. ze sends no Access-Request carrying Service-Type Authorize Only | In order to satisfy the requirements of [RFC2865], Section 5.44, an Access-Request with Service-Type Attribute with value "Authorize Only" MUST contain a State Attribute. |
| `3.3:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 5176 Section 3.2: "Support for a CoA-Request including a Service-Type Attribute with value \\"Authorize Only\\" is OPTIONAL on the NAS and Dynamic Authorization Client." The owner declined the feature on 2026-08-31, and handlePacket (internal/component/l2tp/plugins/authradius/coa.go) shows it: every CoA-Request carrying a Service-Type is answered with a CoA-NAK and Error-Cause 405 before any authorization change is read, so no Authorize Only exchange ever starts. The absent feature is disclosed in the RFC 5176 row of docs/features/rfc-status.md. Its first clause binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them, and its second is conditioned on "the resulting Access-Request, if any", which ze never sends | In order to provide a State Attribute to the NAS, a Dynamic Authorization Client sending a CoA-Request with a Service-Type Attribute with a value of "Authorize Only" MUST include a State Attribute, and the NAS MUST send the State Attribute unmodified to the RADIUS server in the resulting Access-Request, if any. |
| `3.3:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 2865 Section 5.29 makes the feature optional: "If the Value is set to RADIUS-Request, upon termination of the specified service the NAS MAY send a new Access-Request to the RADIUS server, including the State attribute if any." ze performs no Termination-Action: the three non-test Access-Request producers in the tree are buildAuthAttrs (internal/component/l2tp/plugins/authradius/handler.go), (*radiusAuthenticator).Authenticate (internal/component/radius/authenticator.go) and the two doctor probes, and each builds an Access-Request at authentication time only. No code names attribute 29. The absent feature is disclosed in the RFC 5176 row of docs/features/rfc-status.md | If the NAS performs the Termination-Action by sending a new Access- Request upon termination of the current session, it MUST include the State Attribute unchanged in that Access-Request. |
| `3.3:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | A packet-shape rule on the CoA-Request the sender builds; RFC 5176 Section 3.6 states the same bound as the 0-1 column for State. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | A CoA-Request packet MUST have only zero or one State Attribute. |
| `3.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Verification of a CoA/Disconnect-ACK or -NAK, a packet ze emits and never receives. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | A Dynamic Authorization Client receiving a CoA/Disconnect-ACK or CoA/Disconnect-NAK with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `3.4:4` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The response-direction Message-Authenticator, whose enclosing construction is RFC 5176 Section 3.4: "The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing." coaListener.sendResponse (internal/component/l2tp/plugins/authradius/coa.go) includes no Message-Authenticator in a CoA-ACK, CoA-NAK, Disconnect-ACK or Disconnect-NAK, which the MAY permits, so the computation rule has no packet to govern | When the HMAC-MD5 message integrity check is calculated, the Message-Authenticator Attribute MUST be considered to be sixteen octets of zero. |
| `3.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The legend of the Section 3.6 table of attributes. The keywords define what the 0, 0+, 0-1 and 1 columns MEAN; they state no obligation on an implementation. The obligations the table expresses are its rows | 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. 1 Exactly one instance of this attribute MUST be present in packet. |
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The Diameter-considerations restatement of Section 3.2's rule on what a Disconnect-Request may carry. Binds the Dynamic Authorization Client, the entity originating CoA-Request and Disconnect-Request packets (RFC 5176 Section 1.3). ze originates neither: coaListener (internal/component/l2tp/plugins/authradius/coa.go) is the only site in the tree that names radius.CodeCoARequest or radius.CodeDisconnectRequest, and it only receives them | As a result, as noted in Section 3.2, the Service-Type Attribute MUST NOT be used within a Disconnect-Request. |
| `6.1:2` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The else-branch of an optional check. Its enclosing construction is RFC 5176 Section 6.1: "In situations where the Dynamic Authorization Client is co-resident with a RADIUS authentication or accounting server, a proxy MAY perform a \\"reverse path forwarding\\" (RPF) check to verify that a Disconnect-Request or CoA-Request originates from an authorized Dynamic Authorization Client." ze performs no RPF check and maintains no realm routing table, which the same section says makes an RPF check impossible for a NAS | If the source address of the Disconnect-Request or CoA-Request is within this set, then the CoA-Request or Disconnect-Request is forwarded; otherwise it MUST be silently discarded. |

## Superseded

No document obsoletes RFC 5176, so its obligations are stated where they were written.
