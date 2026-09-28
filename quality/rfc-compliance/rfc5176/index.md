# RFC 5176 - Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS)

Supported for subscriber access. Every requirement this repository extracted from RFC 5176, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 95.5% | 21 of 22 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.5% | 1 of 22 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 22 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 57.7% | 30 of 52 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 22 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 22 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 22 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 21 | of 22 gated MUSTs judged | 9 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 22 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported for subscriber access |
| Enrolment | Enrolled |
| Requirements | 23 |
| Gated MUST-level | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 52 |
| Tagged units | 52 |
| Recorded audit verdicts | 21 |
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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (21):** [`RFC5176-3.5-1`](#rfc5176-3.5-1), [`RFC5176-3.5-2`](#rfc5176-3.5-2), [`RFC5176-3.3-1`](#rfc5176-3.3-1), [`RFC5176-3.5-4`](#rfc5176-3.5-4), [`RFC5176-2.3-1`](#rfc5176-2.3-1), [`RFC5176-2.3-2`](#rfc5176-2.3-2), [`RFC5176-2.3-3`](#rfc5176-2.3-3), [`RFC5176-2.3-4`](#rfc5176-2.3-4), [`RFC5176-2.3-5`](#rfc5176-2.3-5), [`RFC5176-2.3-6`](#rfc5176-2.3-6), [`RFC5176-2.3-7`](#rfc5176-2.3-7), [`RFC5176-3.1-1`](#rfc5176-3.1-1), [`RFC5176-3.2-1`](#rfc5176-3.2-1), [`RFC5176-3.3-2`](#rfc5176-3.3-2), [`RFC5176-3.4-1`](#rfc5176-3.4-1), [`RFC5176-3.4-2`](#rfc5176-3.4-2), [`RFC5176-3.4-3`](#rfc5176-3.4-3), [`RFC5176-3.5-5`](#rfc5176-3.5-5), [`RFC5176-3.6-1`](#rfc5176-3.6-1), [`RFC5176-6.1-1`](#rfc5176-6.1-1), [`RFC5176-6.3-1`](#rfc5176-6.3-1)

**Annotated instead of tested (1):** [`RFC5176-3.5-3`](#rfc5176-3.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5176-3.5-1` | This value is used to authenticate packets between the Dynamic Authorization Client and the Dynamic Authorization Server. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L337). **negative:** `unit/verify` [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L250) |
| `RFC5176-3.5-2` | A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L252). **negative:** `unit/verify` [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L339) |
| `RFC5176-3.3-1` | The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L402). **negative:** `unit/verify` [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) |
| `RFC5176-3.5-3` | The Authenticator field in a Response packet (e.g., Disconnect-ACK, Disconnect-NAK, CoA-ACK, or CoA-NAK) is called the Response Authenticator, and contains a one-way MD5 hash calculated over a stream of octets consisting of the Code, Identifier, Length, the Request Authenticator field from the packet being replied to, and the response attributes if any, followed by the shared secret. The resulting 16-octet MD5 hash value is stored in the Authenticator field of the Response packet. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66). **negative:** no negative test. **{single-polarity}:** the NAS only emits CoA/Disconnect responses and never receives one, so there is no inbound Response Authenticator to reject; correctness is proven by verifying the emitted authenticator against radius.ResponseAuthenticator (internal/component/radius/packet.go:145) |
| `RFC5176-3.5-4` | The Authenticator field MUST be calculated in the same way as is specified for an Accounting-Request in [RFC2866]. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L183). **positive:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L352). **negative:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L218). **negative:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L358) |
| `RFC5176-2.3-1` | Packets received with an invalid Code field MUST be silently discarded. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L133). **negative:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L136) |
| `RFC5176-2.3-2` | A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L155). **negative:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L158) |
| `RFC5176-2.3-3` | Octets outside the range of the Length field MUST be treated as padding and ignored on reception. If the packet is shorter than the Length field indicates, it MUST be silently discarded. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L192). **negative:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L194) |
| `RFC5176-2.3-4` | The Dynamic Authorization Server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L216). **negative:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L219) |
| `RFC5176-2.3-5` | In CoA-Request and Disconnect-Request packets, all attributes MUST be treated as mandatory. If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK. A NAS MUST respond to a CoA-Request containing one or more unsupported attributes or Attribute values with a CoA-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. A NAS MUST respond to a Disconnect-Request containing one or more unsupported attributes or Attribute values with a Disconnect-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255). **negative:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) |
| `RFC5176-2.3-6` | State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made. If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions. Similarly, a state change MUST NOT occur as a result of a Disconnect-Request that is unsuccessful with respect to any of the matching sessions; a NAS MUST send a Disconnect-NAK in reply if any of the matching sessions cannot be successfully terminated. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293). **negative:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295) |
| `RFC5176-2.3-7` | A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325). **negative:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) |
| `RFC5176-3.1-1` | If there are any Proxy-State attributes in a Disconnect-Request or CoA-Request received from the Dynamic Authorization Client, the Dynamic Authorization Server MUST include those Proxy-State attributes in its response to the Dynamic Authorization Client. A forwarding proxy or NAS MUST NOT modify existing Proxy-State, State, or Class attributes present in the packet. The forwarding proxy or NAS MUST treat any Proxy-State attributes already in the packet as opaque data. Its operation MUST NOT depend on the content of Proxy-State attributes added by previous proxies. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L363). **negative:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) |
| `RFC5176-3.2-1` | A NAS MUST respond to a CoA-Request including a Service-Type Attribute with value "Authorize Only" with a CoA-NAK; a CoA-ACK MUST NOT be sent. If the NAS does not support a Service-Type value of "Authorize Only", then it MUST respond with a CoA-NAK; an Error-Cause Attribute with a value of 405 (Unsupported Service) SHOULD be included. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L397). **negative:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L400) |
| `RFC5176-3.3-2` | The State Attribute is available to be sent by the Dynamic Authorization Client to the NAS in a CoA-Request packet and MUST be sent unmodified from the NAS to the Dynamic Authorization Client in a subsequent ACK or NAK packet. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L427). **negative:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) |
| `RFC5176-3.4-1` | When the HMAC-MD5 message integrity check is calculated the Request Authenticator field and Message-Authenticator Attribute MUST each be considered to be sixteen octets of zero. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L456). **positive:** `unit/verify` [`radius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L126). **negative:** `unit/verify` [`TestRFC5176MessageAuthenticatorRefusesEveryOtherStream`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L148). **negative:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L460) |
| `RFC5176-3.4-2` | The Message-Authenticator Attribute is calculated and inserted in the packet before the Request Authenticator is calculated. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L218). **negative:** `unit/verify` [`TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L239) |
| `RFC5176-3.4-3` | A Dynamic Authorization Server receiving a CoA-Request or Disconnect-Request with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176ListenerAcceptsConformantMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L26). **negative:** `unit/verify` [`TestRFC5176ListenerDiscardsWrongMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L52). **negative:** `unit/verify` [`TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L71) |
| `RFC5176-3.4-4` | The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing. (§3.4) | MAY | 3.4 | **positive:** `unit/verify` [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26). **negative:** `unit/verify` [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L307) |
| `RFC5176-3.5-5` | Values 200-299 represent successful completion, so that these values may only be sent within CoA-ACK or Disconnect-ACK packets and MUST NOT be sent within a CoA-NAK or Disconnect-NAK packet. Values 400-499 represent fatal errors committed by the Dynamic Authorization Client, so that they MAY be sent within CoA-NAK or Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. Values 500-599 represent fatal errors occurring on a Dynamic Authorization Server, so that they MAY be sent within CoA-NAK and Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496). **negative:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) |
| `RFC5176-3.6-1` | Where NAS or session identification attributes are included in Disconnect-Request or CoA-Request packets, they are used for identification purposes only. These attributes MUST NOT be used for purposes other than identification (e.g., within CoA-Request packets to request authorization changes). (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L604). **negative:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607) |
| `RFC5176-6.1-1` | A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119). **positive:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L652). **negative:** `unit/verify` [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173). **negative:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655) |
| `RFC5176-6.3-1` | When the Event-Timestamp Attribute is present, both the Dynamic Authorization Server and the Dynamic Authorization Client MUST check that the Event-Timestamp Attribute is current within an acceptable time window. If the Event-Timestamp Attribute is not current, then the packet MUST be silently discarded. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L710). **negative:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L714) |

## Gaps and untested MUSTs

RFC 5176 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5176-3.5-1`](#rfc5176-3.5-1)

This value is used to authenticate packets between the Dynamic Authorization Client and the Dynamic Authorization Server. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: acting on a request whose Authenticator does not authenticate it. (b) TestCoAListenerInvalidAuth fails if a zero-authenticator CoA-Request gets any response; TestCoAListenerUnknownSession asserts a correctly signed request is processed (CoA-NAK 503).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L250) | unit/verify | unproven |
| positive | [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L337) | unit/verify | unproven |

### [`RFC5176-3.5-2`](#rfc5176-3.5-2)

A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: answering a request from an untrusted source. (b) TestCoAListenerInvalidAuth asserts no response to a request from an allowed address that carries a bad authenticator; TestCoAListenerUnknownSession asserts a valid one is answered. The units prove the discard of an unauthenticated packet, which is the row's reading of "untrusted"; the source-address clause of the same sentence is proven by the units tagged RFC5176-6.1-1, not here. This row now quotes the same sentence as RFC5176-6.1-1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L339) | unit/verify | unproven |
| positive | [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L252) | unit/verify | unproven |

### [`RFC5176-3.3-1`](#rfc5176-3.3-1)

The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: succeeding when no session matches, or answering it with anything other than the matching NAK. (b) TestRFC5176NoSessionIdNotActedOn asserts Disconnect-NAK and zero teardowns for a Disconnect-Request with no identification attribute; TestDisconnectReplayReturnsCachedResponse asserts ACK and one teardown for a match. The CoA-Request branch (CoA-NAK when nothing matches) has no assertion in a unit tagged to this row, and no unit sends identification attributes that match nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) | unit/verify | unproven |
| positive | [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L402) | unit/verify | unproven |

### [`RFC5176-3.5-3`](#rfc5176-3.5-3)

The Authenticator field in a Response packet (e.g., Disconnect-ACK, Disconnect-NAK, CoA-ACK, or CoA-NAK) is called the Response Authenticator, and contains a one-way MD5 hash calculated over a stream of octets consisting of the Code, Identifier, Length, the Request Authenticator field from the packet being replied to, and the response attributes if any, followed by the shared secret. The resulting 16-octet MD5 hash value is stored in the Authenticator field of the Response packet. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: a response whose Authenticator is not MD5 over Code, Identifier, Length, the request's Request Authenticator, the response attributes and the secret. (b) TestRFC5176ResponseAuthenticator checks the emitted NAK with radius.VerifyResponseAuth, which calls radius.ResponseAuthenticator, the same function coa.go sendResponse uses. The oracle is the producer, so a formula error in ResponseAuthenticator passes; only the inputs the listener passes are proven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66) | unit/verify | unproven |

### [`RFC5176-3.5-4`](#rfc5176-3.5-4)

The Authenticator field MUST be calculated in the same way as is specified for an Accounting-Request in [RFC2866]. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a Request Authenticator not computed as RFC 2866 specifies (MD5 over Code+Identifier+Length+16 zero octets+attributes+secret). (b) TestRFC5176CoARequestAuthenticatorMatchesTheFormula compares AccountingRequestAuth with an independent MD5 in the test over non-zero wire octets; TestRFC5176CoARequestAuthenticatorCoversEveryNamedField refuses changed Code, Identifier, attribute and secret; packet_test.go refuses a corrupted byte.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L358) | unit/verify | unproven |
| negative | [`TestRFC5176CoARequestAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L218) | unit/verify | unproven |
| positive | [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L352) | unit/verify | unproven |
| positive | [`TestRFC5176CoARequestAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L183) | unit/verify | unproven |

### [`RFC5176-2.3-1`](#rfc5176-2.3-1)

Packets received with an invalid Code field MUST be silently discarded. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: answering or acting on a packet whose Code is not CoA-Request or Disconnect-Request. (b) TestRFC5176InvalidCodeDiscarded: sendRawCoAPacketExpectNoResponse for a correctly signed Code 44 packet; the CoA-Request with the same attributes is answered. One invalid value exercises the whitelist in handlePacket.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L133) | unit/verify | revert, verified |

### [`RFC5176-2.3-2`](#rfc5176-2.3-2)

A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: failing to detect a duplicate with the same source IP address, source UDP port and Identifier. (b) TestRFC5176DuplicateRequestAnsweredFromCache asserts teardowns == 1 after two byte-identical copies, but sendRawCoAPacket dials a new socket per copy, so the copies differ in source port and share the Request Authenticator. No assertion covers a same-IP/port/Identifier retransmission whose Authenticator differs. The test pins replayKey (coa.go), which keys on source IP, Code, Identifier and Authenticator and omits the source port the sentence names.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L155) | unit/verify | revert, verified |

### [`RFC5176-2.3-3`](#rfc5176-2.3-3)

Octets outside the range of the Length field MUST be treated as padding and ignored on reception. If the packet is shorter than the Length field indicates, it MUST be silently discarded. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: reading octets past Length as attributes, or processing a datagram shorter than its Length. (b) TestRFC5176LengthFieldGovernsTheOctetsRead asserts a request with eight trailing octets is ACKed, and a datagram four octets short of its Length gets no response.

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

In CoA-Request and Disconnect-Request packets, all attributes MUST be treated as mandatory. If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK. A NAS MUST respond to a CoA-Request containing one or more unsupported attributes or Attribute values with a CoA-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. A NAS MUST respond to a Disconnect-Request containing one or more unsupported attributes or Attribute values with a Disconnect-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: accepting a request with an unsupported attribute or an unsupported attribute value, or ACKing a CoA whose change cannot be carried out. (b) TestRFC5176UnsupportedAttributeNAKed asserts CoA-NAK 401 for Session-Timeout, Disconnect-NAK 401 for Filter-Id in a Disconnect-Request, no teardown, and ACK for an identification-only request. No assertion covers an unsupported attribute VALUE (the 407 case), and the cannot-be-carried-out sentence has no assertion in this unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255) | unit/verify | revert, verified |

### [`RFC5176-2.3-6`](#rfc5176-2.3-6)

State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made. If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions. Similarly, a state change MUST NOT occur as a result of a Disconnect-Request that is unsuccessful with respect to any of the matching sessions; a NAS MUST send a Disconnect-NAK in reply if any of the matching sessions cannot be successfully terminated. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: a non-atomic CoA (ACK or partial change on failure) or a state change or ACK for a Disconnect-Request that fails for a matching session. (b) TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed asserts CoA-NAK 506 with no bus and CoA-ACK plus one event with a bus. It does not assert that no change was made on the NAK path (no observer exists there), and no assertion covers the Disconnect-Request clause: a Disconnect-Request whose session cannot be terminated, with Disconnect-NAK and no state change.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295) | unit/verify | revert, verified |
| positive | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293) | unit/verify | revert, verified |

### [`RFC5176-2.3-7`](#rfc5176-2.3-7)

A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: a NAS without multi-session support ACKing a CoA-Request or Disconnect-Request that matches several sessions. (b) TestRFC5176MultipleMatchingSessionsNAKed asserts Disconnect-NAK 508 and zero teardowns for the Disconnect-Request; the CoA-Request half (CoA-NAK) has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) | unit/verify | revert, verified |
| positive | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325) | unit/verify | revert, verified |

### [`RFC5176-3.1-1`](#rfc5176-3.1-1)

If there are any Proxy-State attributes in a Disconnect-Request or CoA-Request received from the Dynamic Authorization Client, the Dynamic Authorization Server MUST include those Proxy-State attributes in its response to the Dynamic Authorization Client. A forwarding proxy or NAS MUST NOT modify existing Proxy-State, State, or Class attributes present in the packet. The forwarding proxy or NAS MUST treat any Proxy-State attributes already in the packet as opaque data. Its operation MUST NOT depend on the content of Proxy-State attributes added by previous proxies. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: omitting or altering the request's Proxy-State attributes in the response, modifying State or Class, or depending on Proxy-State content. (b) TestRFC5176ProxyStateReturnedUnmodified asserts both Proxy-State values come back byte-equal and in order, and none is invented. The State and Class non-modification clause and the opaque/no-dependence clauses have no assertion in this unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) | unit/verify | revert, verified |
| positive | [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L363) | unit/verify | revert, verified |

### [`RFC5176-3.2-1`](#rfc5176-3.2-1)

A NAS MUST respond to a CoA-Request including a Service-Type Attribute with value "Authorize Only" with a CoA-NAK; a CoA-ACK MUST NOT be sent. If the NAS does not support a Service-Type value of "Authorize Only", then it MUST respond with a CoA-NAK; an Error-Cause Attribute with a value of 405 (Unsupported Service) SHOULD be included. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: answering an Authorize Only CoA-Request with a CoA-ACK, or not NAKing an unsupported Authorize Only. (b) TestRFC5176ServiceTypeNAKed wantNAK(CoA-NAK, 405) and zero events for Service-Type 17; the same request without Service-Type is ACKed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L400) | unit/verify | revert, verified |
| positive | [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L397) | unit/verify | revert, verified |

### [`RFC5176-3.3-2`](#rfc5176-3.3-2)

The State Attribute is available to be sent by the Dynamic Authorization Client to the NAS in a CoA-Request packet and MUST be sent unmodified from the NAS to the Dynamic Authorization Client in a subsequent ACK or NAK packet. (§3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: returning the State Attribute modified, or not returning it, in the ACK or NAK. (b) TestRFC5176StateReturnedUnmodified asserts byte-equal State in the response to an accepted CoA-Request (an ACK) and no State when none was sent. The NAK half of "ACK or NAK" has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) | unit/verify | revert, verified |
| positive | [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L427) | unit/verify | revert, verified |

### [`RFC5176-3.4-1`](#rfc5176-3.4-1)

When the HMAC-MD5 message integrity check is calculated the Request Authenticator field and Message-Authenticator Attribute MUST each be considered to be sixteen octets of zero. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: computing the HMAC-MD5 with the Request Authenticator or the Message-Authenticator not zeroed. (b) TestRFC5176MessageAuthenticatorRefusesEveryOtherStream refuses the Request Authenticator hashed as on the wire and the attribute not zeroed; TestRFC5176MessageAuthenticatorZeroesBothFields accepts the conformant stream with a non-zero wire Request Authenticator; the listener-level walk test discards the pre-fix computation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L460) | unit/verify | revert, verified |
| negative | [`TestRFC5176MessageAuthenticatorRefusesEveryOtherStream`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L148) | unit/verify | unproven |
| positive | [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L456) | unit/verify | revert, verified |
| positive | [`radius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L126) | unit/verify | unproven |

### [`RFC5176-3.4-2`](#rfc5176-3.4-2)

The Message-Authenticator Attribute is calculated and inserted in the packet before the Request Authenticator is calculated. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a Request Authenticator computed over a zeroed Message-Authenticator, i.e. the attribute not inserted first. (b) TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder refuses the inverted order and a Message-Authenticator changed after signing; TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator accepts the conformant order with a non-zero attribute.

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

The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: treating the MAY as a MUST by discarding a request without Message-Authenticator by default. (b) TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault asserts Disconnect-ACK and one teardown with no Message-Authenticator; TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired asserts discard only when the operator leaf requires it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/coa_test.go#L307) | unit/verify | unproven |
| positive | [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26) | unit/verify | unproven |

### [`RFC5176-3.5-5`](#rfc5176-3.5-5)

Values 200-299 represent successful completion, so that these values may only be sent within CoA-ACK or Disconnect-ACK packets and MUST NOT be sent within a CoA-NAK or Disconnect-NAK packet. Values 400-499 represent fatal errors committed by the Dynamic Authorization Client, so that they MAY be sent within CoA-NAK or Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. Values 500-599 represent fatal errors occurring on a Dynamic Authorization Server, so that they MAY be sent within CoA-NAK and Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a 200-299 Error-Cause in a NAK, or a 400-599 value in an ACK. (b) TestRFC5176ErrorCausePlacement fails on a NAK cause outside 400-599 and on any Error-Cause in an ACK, over five NAK paths and one ACK path, and requires both kinds seen. The NAK paths 508, 506 and Disconnect unknown-session are not in the table.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) | unit/verify | revert, verified |
| positive | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496) | unit/verify | revert, verified |

### [`RFC5176-3.6-1`](#rfc5176-3.6-1)

Where NAS or session identification attributes are included in Disconnect-Request or CoA-Request packets, they are used for identification purposes only. These attributes MUST NOT be used for purposes other than identification (e.g., within CoA-Request packets to request authorization changes). (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: using an identification attribute as an authorization change. (b) TestRFC5176IdentificationAttributesIdentifyOnly asserts a CoA carrying User-Name, NAS-Port, Called-Station-Id and NAS-Identifier plus Filter-Id emits exactly one rate event for session 20 at the Filter-Id rate, and an identification-only CoA is NAKed 401 with no new event.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607) | unit/verify | revert, verified |
| positive | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L604) | unit/verify | revert, verified |

### [`RFC5176-6.1-1`](#rfc5176-6.1-1)

A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: answering or acting on a request from a source that is not a configured client. (b) TestRFC5176UntrustedSourceDiscarded asserts no response and zero teardowns from a non-listed source and from an empty allow list, and ACK from a trusted one; TestCoASourceFilterDiscardsWhenNoServerResolved and TestCoASourceFilterAnswersAConfiguredClient repeat the pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173) | unit/verify | unproven |
| negative | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655) | unit/verify | revert, verified |
| positive | [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119) | unit/verify | unproven |
| positive | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L652) | unit/verify | revert, verified |

### [`RFC5176-6.3-1`](#rfc5176-6.3-1)

When the Event-Timestamp Attribute is present, both the Dynamic Authorization Server and the Dynamic Authorization Client MUST check that the Event-Timestamp Attribute is current within an acceptable time window. If the Event-Timestamp Attribute is not current, then the packet MUST be silently discarded. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: acting on or answering a request whose Event-Timestamp is outside the window. (b) TestRFC5176StaleEventTimestampDiscarded asserts no response and zero teardowns for a timestamp two windows old, and ACK for a current one. A future-dated timestamp is not exercised.

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
