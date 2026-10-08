# RFC 5176 - Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS)

Supported for subscriber access. Every requirement this repository extracted from RFC 5176, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 96.7% | 29 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.3% | 1 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 30 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 80.5% | 70 of 87 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 31 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 29 | of 30 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported for subscriber access |
| Enrolment | Enrolled |
| Requirements | 31 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 87 |
| Tagged units | 87 |
| Recorded audit verdicts | 29 |
| Discrimination records | 70 |
| Summary | `rfc/short/rfc5176.md` |
| Requirement shard | `rfc/requirements/rfc5176.md` |
| RFC text | `rfc/full/rfc5176.txt` |

## Enrolment

Enrolled: RADIUS Dynamic Authorization Extensions (CoA/Disconnect): five MUST-level requirements, all met by ze's Dynamic Authorization Server (internal/component/l2tp/plugins/authradius/coa.go, wired at register.go). 3.5-1 (verify Request Authenticator before processing), 3.5-2 (silently discard invalid authenticators), 3.3-1 (require at least one session-identification attribute), and 3.5-4 (Request Authenticator = MD5 over the RFC 2865 fields) each carry positive+negative tags on the CoA listener and packet tests. 3.5-3 (Response Authenticator per RFC 2865) is {single-polarity: positive}: ze only emits responses, so there is no inbound Response Authenticator to reject.

## What the public ledger says

**Status:** Supported for subscriber access

**What the ledger says is covered**

CoA/DM listener for RADIUS-initiated changes and disconnects: Request Authenticator and optional Message-Authenticator verification, source-address allow list, duplicate detection and cached replay, Event-Timestamp window, mandatory-attribute handling with Error-Cause 401, Service-Type refusal with 405, multiple-match refusal with 508, and Proxy-State and State echoed unread. Tests bound per requirement in [`rfc/requirements/rfc5176.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5176.md), and the checklist is bounded by [`rfc/extraction/rfc5176.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc5176.json).

**What the ledger says remains**

Scoped to subscriber access. Two OPTIONAL features of the RFC are out of scope, so the obligations conditional on them are excluded rather than gated: the Section 3.2 "Authorize Only" Service-Type exchange, which ze answers with a CoA-NAK and Error-Cause 405, and the RFC 2865 Section 5.29 Termination-Action re-authorization, for which ze sends no Access-Request.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 29 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (29):** [`RFC5176-3.5-1`](#rfc5176-3.5-1), [`RFC5176-3.3-1`](#rfc5176-3.3-1), [`RFC5176-3.5-4`](#rfc5176-3.5-4), [`RFC5176-2.3-1`](#rfc5176-2.3-1), [`RFC5176-2.3-2`](#rfc5176-2.3-2), [`RFC5176-2.3-3`](#rfc5176-2.3-3), [`RFC5176-2.3-4`](#rfc5176-2.3-4), [`RFC5176-2.3-5`](#rfc5176-2.3-5), [`RFC5176-2.3-6`](#rfc5176-2.3-6), [`RFC5176-2.3-7`](#rfc5176-2.3-7), [`RFC5176-3.1-1`](#rfc5176-3.1-1), [`RFC5176-3.2-1`](#rfc5176-3.2-1), [`RFC5176-3.3-2`](#rfc5176-3.3-2), [`RFC5176-3.4-1`](#rfc5176-3.4-1), [`RFC5176-3.4-2`](#rfc5176-3.4-2), [`RFC5176-3.4-3`](#rfc5176-3.4-3), [`RFC5176-3.5-5`](#rfc5176-3.5-5), [`RFC5176-3.6-1`](#rfc5176-3.6-1), [`RFC5176-6.1-1`](#rfc5176-6.1-1), [`RFC5176-6.3-1`](#rfc5176-6.3-1), [`RFC5176-2.2-1`](#rfc5176-2.2-1), [`RFC5176-3-1`](#rfc5176-3-1), [`RFC5176-3.3-3`](#rfc5176-3.3-3), [`RFC5176-3.5-6`](#rfc5176-3.5-6), [`RFC5176-3.5-7`](#rfc5176-3.5-7), [`RFC5176-3.5-8`](#rfc5176-3.5-8), [`RFC5176-3.5-9`](#rfc5176-3.5-9), [`RFC5176-3.6-2`](#rfc5176-3.6-2), [`RFC5176-6.3-2`](#rfc5176-6.3-2)

**Annotated (including scoped evidence) (1):** [`RFC5176-3.5-3`](#rfc5176-3.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5176-3.5-1` | This value is used to authenticate packets between the Dynamic Authorization Client and the Dynamic Authorization Server. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L335). **negative:** `unit/verify` [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L250) |
| `RFC5176-3.3-1` | The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L398). **positive:** `unit/verify` [`TestRFC5176UnmatchedIdentificationIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L120). **negative:** `unit/verify` [`TestRFC5176MatchedIdentificationSucceeds`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L143). **negative:** `unit/verify` [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) |
| `RFC5176-3.5-3` | The Authenticator field in a Response packet (e.g., Disconnect-ACK, Disconnect-NAK, CoA-ACK, or CoA-NAK) is called the Response Authenticator, and contains a one-way MD5 hash calculated over a stream of octets consisting of the Code, Identifier, Length, the Request Authenticator field from the packet being replied to, and the response attributes if any, followed by the shared secret. The resulting 16-octet MD5 hash value is stored in the Authenticator field of the Response packet. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66). **positive:** `unit/verify` [`TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L206). **negative:** no negative test. **{single-polarity}:** the NAS only emits CoA/Disconnect responses and never receives one, so there is no inbound Response Authenticator to reject; correctness is proven by verifying the emitted authenticator against radius.ResponseAuthenticator (internal/component/radius/packet.go:145) |
| `RFC5176-3.5-4` | The Authenticator field MUST be calculated in the same way as is specified for an Accounting-Request in [RFC2866]. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorMatchesTheFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L183). **positive:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L352). **negative:** `unit/verify` [`TestRFC5176CoARequestAuthenticatorCoversEveryNamedField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2865_response_auth_test.go#L218). **negative:** `unit/verify` [`TestVerifyCoARequestAuth`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L358) |
| `RFC5176-2.3-1` | Packets received with an invalid Code field MUST be silently discarded. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L133). **negative:** `unit/verify` [`TestRFC5176InvalidCodeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L136) |
| `RFC5176-2.3-2` | A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L155). **negative:** `unit/verify` [`TestRFC5176DuplicateRequestAnsweredFromCache`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L158) |
| `RFC5176-2.3-3` | Octets outside the range of the Length field MUST be treated as padding and ignored on reception. If the packet is shorter than the Length field indicates, it MUST be silently discarded. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L192). **negative:** `unit/verify` [`TestRFC5176LengthFieldGovernsTheOctetsRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L194) |
| `RFC5176-2.3-4` | The Dynamic Authorization Server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L216). **negative:** `unit/verify` [`TestRFC5176SharedSecretChosenBySourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L219) |
| `RFC5176-2.3-5` | In CoA-Request and Disconnect-Request packets, all attributes MUST be treated as mandatory. If one or more authorization changes specified in a CoA-Request cannot be carried out, the NAS MUST send a CoA-NAK. A NAS MUST respond to a CoA-Request containing one or more unsupported attributes or Attribute values with a CoA-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. A NAS MUST respond to a Disconnect-Request containing one or more unsupported attributes or Attribute values with a Disconnect-NAK; an Error-Cause Attribute with value 401 (Unsupported Attribute) or 407 (Invalid Attribute Value) MAY be included. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L306). **positive:** `unit/verify` [`TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L51). **positive:** `unit/verify` [`TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L85). **positive:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255). **negative:** `unit/verify` [`TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L112). **negative:** `unit/verify` [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) |
| `RFC5176-2.3-6` | State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made. If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions. Similarly, a state change MUST NOT occur as a result of a Disconnect-Request that is unsuccessful with respect to any of the matching sessions; a NAS MUST send a Disconnect-NAK in reply if any of the matching sessions cannot be successfully terminated. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293). **positive:** `unit/verify` [`TestRFC5176CoANAKMakesNoPartialChange`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L246). **positive:** `unit/verify` [`TestRFC5176DisconnectThatCannotTerminateIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L266). **positive:** `unit/verify` [`TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L152). **negative:** `unit/verify` [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295). **negative:** `unit/verify` [`TestRFC5176DisconnectThatTerminatesIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L287). **negative:** `unit/verify` [`TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L116) |
| `RFC5176-2.3-7` | A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC5176CoAMatchingSeveralSessionsIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L84). **positive:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325). **negative:** `unit/verify` [`TestRFC5176CoAMatchingOneSessionIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L100). **negative:** `unit/verify` [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) |
| `RFC5176-3.1-1` | If there are any Proxy-State attributes in a Disconnect-Request or CoA-Request received from the Dynamic Authorization Client, the Dynamic Authorization Server MUST include those Proxy-State attributes in its response to the Dynamic Authorization Client. A forwarding proxy or NAS MUST NOT modify existing Proxy-State, State, or Class attributes present in the packet. The forwarding proxy or NAS MUST treat any Proxy-State attributes already in the packet as opaque data. Its operation MUST NOT depend on the content of Proxy-State attributes added by previous proxies. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L363). **positive:** `unit/verify` [`TestRFC5176StateAndClassUnmodifiedProxyStateOpaque`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_proxy_state_opaque_test.go#L37). **negative:** `unit/verify` [`TestRFC5176ProxyStateContentIsNeverRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_proxy_state_opaque_test.go#L144). **negative:** `unit/verify` [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) |
| `RFC5176-3.2-1` | A NAS MUST respond to a CoA-Request including a Service-Type Attribute with value "Authorize Only" with a CoA-NAK; a CoA-ACK MUST NOT be sent. If the NAS does not support a Service-Type value of "Authorize Only", then it MUST respond with a CoA-NAK; an Error-Cause Attribute with a value of 405 (Unsupported Service) SHOULD be included. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L397). **negative:** `unit/verify` [`TestRFC5176ServiceTypeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L400) |
| `RFC5176-3.3-2` | The State Attribute is available to be sent by the Dynamic Authorization Client to the NAS in a CoA-Request packet and MUST be sent unmodified from the NAS to the Dynamic Authorization Client in a subsequent ACK or NAK packet. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L427). **positive:** `unit/verify` [`TestRFC5176StateReturnedUnmodifiedInANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L167). **negative:** `unit/verify` [`TestRFC5176NAKForARequestWithoutStateCarriesNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L190). **negative:** `unit/verify` [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) |
| `RFC5176-3.4-1` | When the HMAC-MD5 message integrity check is calculated the Request Authenticator field and Message-Authenticator Attribute MUST each be considered to be sixteen octets of zero. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L456). **positive:** `unit/verify` [`radius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L126). **negative:** `unit/verify` [`TestRFC5176MessageAuthenticatorRefusesEveryOtherStream`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L148). **negative:** `unit/verify` [`authradius/TestRFC5176MessageAuthenticatorZeroesBothFields`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L460) |
| `RFC5176-3.4-2` | The Message-Authenticator Attribute is calculated and inserted in the packet before the Request Authenticator is calculated. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176RequestAuthenticatorCoversTheSignedMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L218). **negative:** `unit/verify` [`TestRFC5176RequestAuthenticatorRefusesTheInvertedOrder`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc5176_message_authenticator_test.go#L239) |
| `RFC5176-3.4-3` | A Dynamic Authorization Server receiving a CoA-Request or Disconnect-Request with a Message-Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC5176ListenerAcceptsConformantMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L26). **negative:** `unit/verify` [`TestRFC5176ListenerDiscardsWrongMessageAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_test.go#L52). **negative:** `unit/verify` [`TestRFC5176WrongMessageAuthenticatorDiscardedWhenNotRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L71) |
| `RFC5176-3.4-4` | The Message-Authenticator Attribute MAY be used to authenticate and integrity-protect CoA-Request, CoA-ACK, CoA-NAK, Disconnect-Request, Disconnect-ACK, and Disconnect-NAK packets in order to prevent spoofing. (§3.4) | MAY | 3.4 | **positive:** `unit/verify` [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26). **negative:** `unit/verify` [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L305) |
| `RFC5176-3.5-5` | Values 200-299 represent successful completion, so that these values may only be sent within CoA-ACK or Disconnect-ACK packets and MUST NOT be sent within a CoA-NAK or Disconnect-NAK packet. Values 400-499 represent fatal errors committed by the Dynamic Authorization Client, so that they MAY be sent within CoA-NAK or Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. Values 500-599 represent fatal errors occurring on a Dynamic Authorization Server, so that they MAY be sent within CoA-NAK and Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496). **negative:** `unit/verify` [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) |
| `RFC5176-3.6-1` | Where NAS or session identification attributes are included in Disconnect-Request or CoA-Request packets, they are used for identification purposes only. These attributes MUST NOT be used for purposes other than identification (e.g., within CoA-Request packets to request authorization changes). (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607). **negative:** `unit/verify` [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L610) |
| `RFC5176-6.1-1` | A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119). **positive:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655). **negative:** `unit/verify` [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173). **negative:** `unit/verify` [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L658) |
| `RFC5176-6.3-1` | When the Event-Timestamp Attribute is present, both the Dynamic Authorization Server and the Dynamic Authorization Client MUST check that the Event-Timestamp Attribute is current within an acceptable time window. If the Event-Timestamp Attribute is not current, then the packet MUST be silently discarded. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L713). **negative:** `unit/verify` [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L717) |
| `RFC5176-2.2-1` | A NAS MUST respond to a CoA-Request including a Service-Type Attribute with an unsupported value with a CoA-NAK; an Error-Cause Attribute with value "Unsupported Service" SHOULD be included. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC5176UnsupportedServiceTypeValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L188). **negative:** `unit/verify` [`TestRFC5176CoAWithoutServiceTypeIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L215) |
| `RFC5176-3-1` | If other attributes are included in a Disconnect- Request, implementations MUST send a Disconnect-NAK; an Error-Cause Attribute with value "Unsupported Attribute" MAY be included. (§3, "other attributes" are attributes other than NAS and session identification attributes; the §3.6 Disconnect table also admits Reply-Message, Class, Acct-Terminate-Cause and, per its Note 2, EAP-Message in a Disconnect-Request, so they are not "other attributes", and §3.6 Note 7 lets a Vendor-Specific Attribute identify a session) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L238). **negative:** `unit/verify` [`TestRFC5176DisconnectAdmitsTheSection36TableAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L310). **negative:** `unit/verify` [`TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L283) |
| `RFC5176-3.3-3` | In either usage, the Dynamic Authorization Server MUST NOT interpret the Attribute locally. (§3.3, "the Attribute" is the State Attribute, in a CoA-Request and in a Termination-Action re-authorization) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestRFC5176StateNeverChangesTheOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L354). **negative:** `unit/verify` [`TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L406) |
| `RFC5176-3.5-6` | This value is only sent within a Disconnect-ACK and MUST NOT be sent within a CoA-ACK, Disconnect-NAK, or CoA-NAK. (§3.5, "This value" is Error-Cause 201, Residual Session Context Removed) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5176ResidualSessionCauseOnlyInADisconnectACK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L441). **negative:** `unit/verify` [`TestRFC5176ResidualSessionCauseInARequestNeverReachesANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L449) |
| `RFC5176-3.5-7` | "Invalid EAP Packet (Ignored)" is a non-fatal error that MUST NOT be sent by implementations of this specification. (§3.5, Error-Cause 202) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5176InvalidEAPPacketCauseNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L457). **negative:** `unit/verify` [`TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L464) |
| `RFC5176-3.5-8` | "Request Not Routable" is a fatal error that MAY be sent by a proxy and MUST NOT be sent by a NAS. (§3.5, Error-Cause 502) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5176RequestNotRoutableCauseNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L483). **negative:** `unit/verify` [`TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L490) |
| `RFC5176-3.5-9` | It MUST NOT be sent within a CoA-ACK, CoA-NAK, or Disconnect-ACK, only within a Disconnect-NAK. (§3.5, "It" is Error-Cause 504, Session Context Not Removable) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5176SessionNotRemovableOnlyInADisconnectNAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L508). **negative:** `unit/verify` [`TestRFC5176SessionNotRemovableNeverInACoANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L526) |
| `RFC5176-3.6-2` | However, the same Attribute MUST NOT be used for both purposes simultaneously. (§3.6 Note 7, "the same Attribute" is a Vendor-Specific Attribute, the purposes session identification and authorization change) | MUST NOT | 3.6 | **positive:** `unit/verify` [`TestRFC5176VendorSpecificIsAChangeOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L542). **negative:** `unit/verify` [`TestRFC5176VendorSpecificIsNeverAnIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L574) |
| `RFC5176-6.3-2` | The time window used for duplicate detection MUST be the same as the window used to detect a stale Event-Timestamp Attribute. (§6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC5176DuplicateAndTimestampWindowsBothHoldInside`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L641). **negative:** `unit/verify` [`TestRFC5176DuplicateAndTimestampWindowsBothEndOutside`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L673) |

## Gaps and untested MUSTs

RFC 5176 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5176-3.5-1`](#rfc5176-3.5-1)

This value is used to authenticate packets between the Dynamic Authorization Client and the Dynamic Authorization Server. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29, re-judged after the RFC5176-3.5-2 tags were dropped from the doc comments; the test bodies are unchanged. (a) forbidden: acting on a request whose Authenticator does not authenticate it. (b) TestCoAListenerInvalidAuth fails if a zero-authenticator CoA-Request gets any response; TestCoAListenerUnknownSession asserts a correctly signed request is processed (CoA-NAK 503).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoAListenerInvalidAuth`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L250) | unit/verify | unproven |
| positive | [`TestCoAListenerUnknownSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L335) | unit/verify | unproven |

### [`RFC5176-3.3-1`](#rfc5176-3.3-1)

The combination of NAS and session identification attributes included in a CoA-Request or Disconnect-Request packet MUST match at least one session in order for a Request to be successful; otherwise a Disconnect-NAK or CoA-NAK MUST be sent. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. A CoA-Request and a Disconnect-Request whose Acct-Session-Id matches no session get CoA-NAK and Disconnect-NAK 503 with no bus event and no teardown; the same two requests with a matching Acct-Session-Id succeed (CoA-ACK with one event, Disconnect-ACK with one teardown). Both request kinds, both outcomes; the older no-identification negative remains.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176MatchedIdentificationSucceeds`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L143) | unit/verify | revert, verified |
| negative | [`TestRFC5176NoSessionIdNotActedOn`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC5176UnmatchedIdentificationIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestDisconnectReplayReturnsCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L398) | unit/verify | revert, verified |

### [`RFC5176-3.5-3`](#rfc5176-3.5-3)

The Authenticator field in a Response packet (e.g., Disconnect-ACK, Disconnect-NAK, CoA-ACK, or CoA-NAK) is called the Response Authenticator, and contains a one-way MD5 hash calculated over a stream of octets consisting of the Code, Identifier, Length, the Request Authenticator field from the packet being replied to, and the response attributes if any, followed by the shared secret. The resulting 16-octet MD5 hash value is stored in the Authenticator field of the Response packet. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The oracle is no longer the producer: TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5 reads CoA-ACK, CoA-NAK, Disconnect-ACK and Disconnect-NAK raw off the socket and compares the Authenticator field with an MD5 the test computes over Code, Identifier, Length, the sent Request Authenticator, the response attributes and the secret. The {single-polarity: positive} annotation holds: the NAS only emits responses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5176ResponseAuthenticatorMatchesAnIndependentMD5`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC5176ResponseAuthenticator`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L66) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Every clause now has a unit on the real listener socket. Unsupported ATTRIBUTE: TestRFC5176UnsupportedAttributeNAKed (CoA-NAK 401 for Session-Timeout, Disconnect-NAK 401 for Filter-Id with zero teardowns; identification-only Disconnect is ACKed). Unsupported VALUE in a CoA: TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed (sole Filter-Id 'not-a-rate' or 'cos:') and TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed (a supported rate beside an unparseable Filter-Id, a second rate, or an unknown vendor VSA: CoA-NAK 407 and zero bus events), which is the case the old first-match producer ACKed. Unsupported VALUE in a Disconnect: TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed (2-octet NAS-Port gives Disconnect-NAK 407, a Vendor-Specific gives 401, zero teardowns). The ACK counterpart TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed isolates the value (same attributes, valid values: CoA-ACK, Disconnect-ACK with a 4-octet NAS-Port). Producer read: readCoAChange walks every Filter-Id and VSA and refuses the first it cannot apply; unsupportedAttrValue enforces the fixed lengths for both codes in handlePacket. The cannot-be-carried-out sentence is met in its value form here and in its NAS-side-failure form by the units tagged RFC5176-2.3-6 (506 with no event). Records observed for readCoAChange, unsupportedAttrValue, unsupportedAttr and handleCoA (panic-body breaks; by inspection a targeted break of setRate's duplicate check or of the NAS-Port length also turns the NAK units red).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestRFC5176CoAWithAnUnsupportedAttributeValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L306) | unit/verify | revert, verified |
| positive | [`TestRFC5176CoAWithOneUnsupportedValueBesideSupportedOnesIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestRFC5176DisconnectWithAnUnsupportedValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC5176UnsupportedAttributeNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L255) | unit/verify | revert, verified |

### [`RFC5176-2.3-6`](#rfc5176-2.3-6)

State changes resulting from a CoA-Request MUST be atomic: if the CoA-Request is successful for all matching sessions, the NAS MUST send a CoA-ACK in reply, and all requested authorization changes MUST be made. If the CoA-Request is unsuccessful for any matching sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization changes MUST NOT be made for any of the matching sessions. Similarly, a state change MUST NOT occur as a result of a Disconnect-Request that is unsuccessful with respect to any of the matching sessions; a NAS MUST send a Disconnect-NAK in reply if any of the matching sessions cannot be successfully terminated. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. CoA atomicity on both lookup paths. Subscriber path: TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange (rate + CoS on a session with no access interface: CoA-NAK 506 and zero events, so the rate part is not made), TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed (with an interface: CoA-ACK and all three events). L2TP-only path: TestRFC5176CoANAKMakesNoPartialChange (506, zero events) and TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed pair. Disconnect: TestRFC5176DisconnectThatCannotTerminateIsNAKed (NAK after one attempt) vs TestRFC5176DisconnectThatTerminatesIsACKed. Producer read: applySubscriberCoA decides every refusal (no bus, CoS without interface) before the first Emit. The residual NAK after a successful Emit is unreachable, and that is judged sufficient: Server.Emit -> deliverEvent (plugin/server/dispatch.go) fails only for an unregistered (namespace, event-type) or a JSON marshal failure; the three events are package-level events.Register handles, and their payloads are flat structs of integers and strings that json.Marshal cannot fail on. (The coa.go comment names only the first cause; harmless.) Records observed for applySubscriberCoA, handleDisconnect and handleCoA.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176DisconnectThatTerminatesIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L287) | unit/verify | revert, verified |
| negative | [`TestRFC5176RequestWhoseEveryValueIsSupportedIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L116) | unit/verify | revert, verified |
| negative | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L295) | unit/verify | revert, verified |
| positive | [`TestRFC5176CoANAKMakesNoPartialChange`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L246) | unit/verify | revert, verified |
| positive | [`TestRFC5176DisconnectThatCannotTerminateIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRFC5176SubscriberCoAThatCannotBeCarriedOutWholeMakesNoChange`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_values_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestRFC5176ChangeThatCannotBeCarriedOutIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L293) | unit/verify | revert, verified |

### [`RFC5176-2.3-7`](#rfc5176-2.3-7)

A NAS that does not support dynamic authorization changes applying to multiple sessions MUST send a CoA-NAK or Disconnect-NAK in reply; an Error-Cause Attribute with value 508 (Multiple Session Selection Unsupported) SHOULD be included. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. CoA half added: a CoA-Request whose User-Name matches two sessions gets CoA-NAK 508 and zero bus events; the same request narrowed by NAS-Port to one session is ACKed with one event, so the NAK is specific to the multiple match. The Disconnect half (Disconnect-NAK 508, zero teardowns) was already held. Producer oneSession serves both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176CoAMatchingOneSessionIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L328) | unit/verify | revert, verified |
| positive | [`TestRFC5176CoAMatchingSeveralSessionsIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC5176MultipleMatchingSessionsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L325) | unit/verify | revert, verified |

### [`RFC5176-3.1-1`](#rfc5176-3.1-1)

If there are any Proxy-State attributes in a Disconnect-Request or CoA-Request received from the Dynamic Authorization Client, the Dynamic Authorization Server MUST include those Proxy-State attributes in its response to the Dynamic Authorization Client. A forwarding proxy or NAS MUST NOT modify existing Proxy-State, State, or Class attributes present in the packet. The forwarding proxy or NAS MUST treat any Proxy-State attributes already in the packet as opaque data. Its operation MUST NOT depend on the content of Proxy-State attributes added by previous proxies. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 (Proxy-State returned): TestRFC5176ProxyStateReturnedUnmodified both polarities at sendResponse. Clause 2 (no modification of Proxy-State/State/Class) and clause 3 (opaque): + TestRFC5176StateAndClassUnmodifiedProxyStateOpaque, State byte-equal in the CoA-ACK, a 253-octet all-values Proxy-State and an attribute-shaped one returned byte-equal; Class is never echoed by sendResponse, so its assertion holds by construction (no Class can come back altered). Clause 4 (operation independent of Proxy-State content): the same test's two Disconnects differing only in Proxy-State both ACK with 2 teardowns, and - TestRFC5176ProxyStateContentIsNeverRead: an unknown-session Disconnect whose Proxy-State encodes the live session's Acct-Session-Id gets the same NAK code and Error-Cause as without it, 0 teardowns (recorded at handleDisconnect). All units observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ProxyStateContentIsNeverRead`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_proxy_state_opaque_test.go#L144) | unit/verify | revert, verified |
| negative | [`TestRFC5176ProxyStateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L365) | unit/verify | revert, verified |
| positive | [`TestRFC5176StateAndClassUnmodifiedProxyStateOpaque`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_proxy_state_opaque_test.go#L37) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. NAK half added: a CoA-Request carrying State (0xdead00beef, embedded zero) answered by CoA-NAK 506 and by CoA-NAK 503 gets the State back byte-equal; a NAK to a request with no State carries none. The ACK half and its no-State counterpart were already held. Producer sendResponse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176NAKForARequestWithoutStateCarriesNone`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestRFC5176StateReturnedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L429) | unit/verify | revert, verified |
| positive | [`TestRFC5176StateReturnedUnmodifiedInANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_branches_test.go#L167) | unit/verify | revert, verified |
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
| negative | [`TestCoAListenerMissingMessageAuthenticatorDroppedWhenRequired`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_listener_test.go#L305) | unit/verify | unproven |
| positive | [`TestRFC5176MessageAuthenticatorAbsentIsAcceptedByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_message_authenticator_optional_test.go#L26) | unit/verify | unproven |

### [`RFC5176-3.5-5`](#rfc5176-3.5-5)

Values 200-299 represent successful completion, so that these values may only be sent within CoA-ACK or Disconnect-ACK packets and MUST NOT be sent within a CoA-NAK or Disconnect-NAK packet. Values 400-499 represent fatal errors committed by the Dynamic Authorization Client, so that they MAY be sent within CoA-NAK or Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. Values 500-599 represent fatal errors occurring on a Dynamic Authorization Server, so that they MAY be sent within CoA-NAK and Disconnect-NAK packets, and MUST NOT be sent within CoA-ACK or Disconnect-ACK packets. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). Re-judged after the D-15 tag narrowing (unit changed only in comment). (a) forbidden: a 200-299 Error-Cause in a NAK, or a 400-599 value in an ACK. + TestRFC5176ErrorCausePlacement fails on a NAK cause outside 400-599; - (#2) fails on any Error-Cause in a CoA-ACK or Disconnect-ACK; both kinds required seen. The 201/202/502/504 clauses moved to RFC5176-3.5-6..9, which drive all 15 response paths (incl. 504, 506, 508). Negative record re-observed red (sendResponse).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L499) | unit/verify | revert, verified |
| positive | [`TestRFC5176ErrorCausePlacement`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L496) | unit/verify | revert, verified |

### [`RFC5176-3.6-1`](#rfc5176-3.6-1)

Where NAS or session identification attributes are included in Disconnect-Request or CoA-Request packets, they are used for identification purposes only. These attributes MUST NOT be used for purposes other than identification (e.g., within CoA-Request packets to request authorization changes). (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: using an identification attribute as an authorization change. (b) TestRFC5176IdentificationAttributesIdentifyOnly asserts a CoA carrying User-Name, NAS-Port, Called-Station-Id and NAS-Identifier plus Filter-Id emits exactly one rate event for session 20 at the Filter-Id rate, and an identification-only CoA is NAKed 401 with no new event.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L610) | unit/verify | revert, verified |
| positive | [`TestRFC5176IdentificationAttributesIdentifyOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L607) | unit/verify | revert, verified |

### [`RFC5176-6.1-1`](#rfc5176-6.1-1)

A Dynamic Authorization Server MUST silently discard Disconnect- Request or CoA-Request packets from untrusted sources. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: answering or acting on a request from a source that is not a configured client. (b) TestRFC5176UntrustedSourceDiscarded asserts no response and zero teardowns from a non-listed source and from an empty allow list, and ACK from a trusted one; TestCoASourceFilterDiscardsWhenNoServerResolved and TestCoASourceFilterAnswersAConfiguredClient repeat the pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCoASourceFilterAnswersAConfiguredClient`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L173) | unit/verify | unproven |
| negative | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L658) | unit/verify | revert, verified |
| positive | [`TestCoASourceFilterDiscardsWhenNoServerResolved`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_coa_test.go#L119) | unit/verify | unproven |
| positive | [`TestRFC5176UntrustedSourceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L655) | unit/verify | revert, verified |

### [`RFC5176-6.3-1`](#rfc5176-6.3-1)

When the Event-Timestamp Attribute is present, both the Dynamic Authorization Server and the Dynamic Authorization Client MUST check that the Event-Timestamp Attribute is current within an acceptable time window. If the Event-Timestamp Attribute is not current, then the packet MUST be silently discarded. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: acting on or answering a request whose Event-Timestamp is outside the window. (b) TestRFC5176StaleEventTimestampDiscarded asserts no response and zero teardowns for a timestamp two windows old, and ACK for a current one. A future-dated timestamp is not exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L717) | unit/verify | revert, verified |
| positive | [`TestRFC5176StaleEventTimestampDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go#L713) | unit/verify | revert, verified |

### [`RFC5176-2.2-1`](#rfc5176-2.2-1)

A NAS MUST respond to a CoA-Request including a Service-Type Attribute with an unsupported value with a CoA-NAK; an Error-Cause Attribute with value "Unsupported Service" SHOULD be included. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176UnsupportedServiceTypeValueIsNAKed: Service-Type 1/2/6/8 in a CoA-Request -> CoA-NAK with Error-Cause exactly [405], no change (MUST and the SHOULD both asserted). - TestRFC5176CoAWithoutServiceTypeIsACKed: the same request without Service-Type -> CoA-ACK, no Error-Cause, one change, so the NAK is owed to the value. Both over a real UDP socket; records observed red (handlePacket, handleCoA).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176CoAWithoutServiceTypeIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L215) | unit/verify | revert, verified |
| positive | [`TestRFC5176UnsupportedServiceTypeValueIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L188) | unit/verify | revert, verified |

### [`RFC5176-3-1`](#rfc5176-3-1)

If other attributes are included in a Disconnect- Request, implementations MUST send a Disconnect-NAK; an Error-Cause Attribute with value "Unsupported Attribute" MAY be included. (§3, "other attributes" are attributes other than NAS and session identification attributes; the §3.6 Disconnect table also admits Reply-Message, Class, Acct-Terminate-Cause and, per its Note 2, EAP-Message in a Disconnect-Request, so they are not "other attributes", and §3.6 Note 7 lets a Vendor-Specific Attribute identify a session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judge 2026-10-01 (access cont 12, independent judge). Read RFC 5176 Section 3 ('A Disconnect-Request MUST contain only NAS and session identification attributes. If other attributes are included in a Disconnect-Request, implementations MUST send a Disconnect-NAK'), the Section 3.6 Disconnect table and Note 2, and coa.go unsupportedAttr/disconnectSupportedAttrs/handleDisconnect. The cont-11 findings are closed: the EAP-Message case is gone from the positive (table-admitted 0+, Note 2; its 401 is Section 2.3/3.5 'Unsupported Attribute ... such as a Vendor-Specific or EAP-Message', stated in the unit comment), the gloss names EAP-Message beside Reply-Message, Class and Acct-Terminate-Cause, and the Admits unit now carries negative polarity. + TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed: Filter-Id, Framed-MTU, Session-Timeout, Idle-Timeout, Service-Type and State (all '0' in the Disconnect Request column) each beside Acct-Session-Id+User-Name -> Disconnect-NAK with Error-Cause exactly [401], zero teardowns. - TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed (all seven identification attributes -> ACK, one teardown) and - TestRFC5176DisconnectAdmitsTheSection36TableAttributes (Reply-Message, Class, Acct-Terminate-Cause singly and together -> ACK, no Error-Cause, one teardown each), so the NAK is owed to the other attribute alone and the rule does not over-trigger. Real UDP listener, isolated inputs. Judge also removed a nested parenthesis from the gloss (it cut the trailing section cite, failing the verbatim-quote gate). Records observed red: + unsupportedAttr, - handleDisconnect (both negatives); the orphan positive record on the Admits unit was removed (its tag is gone).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176DisconnectAdmitsTheSection36TableAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L310) | unit/verify | revert, verified |
| negative | [`TestRFC5176DisconnectCarryingOnlyIdentificationIsACKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L283) | unit/verify | revert, verified |
| positive | [`TestRFC5176DisconnectWithANonIdentificationAttributeIsNAKed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L238) | unit/verify | revert, verified |

### [`RFC5176-3.3-3`](#rfc5176-3.3-3)

In either usage, the Dynamic Authorization Server MUST NOT interpret the Attribute locally. (§3.3, "the Attribute" is the State Attribute, in a CoA-Request and in a Termination-Action re-authorization)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176StateNeverChangesTheOutcome: no State, State shaped as a rate, as another session's Acct-Session-Id and as a Filter-Id TLV -> identical CoA-ACK and one 10 Mbit/s change on session 20, State returned octet-equal. - TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity: State '10mbit' with no change attribute -> NAK 401; State naming the live session beside an Acct-Session-Id naming none -> NAK 503; no change, State echoed. The Termination-Action usage is unreachable (Ze NAKs Termination-Action in a CoA). Records observed red (sendResponse, readCoAChange).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176StateIsNeverReadAsAChangeOrAnIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L406) | unit/verify | revert, verified |
| positive | [`TestRFC5176StateNeverChangesTheOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L354) | unit/verify | revert, verified |

### [`RFC5176-3.5-6`](#rfc5176-3.5-6)

This value is only sent within a Disconnect-ACK and MUST NOT be sent within a CoA-ACK, Disconnect-NAK, or CoA-NAK. (§3.5, "This value" is Error-Cause 201, Residual Session Context Removed)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176ResidualSessionCauseOnlyInADisconnectACK: coaResponsePaths drives 15 response paths (both ACKs, NAK 401/404/405/407/503/504/506/508 for CoA and Disconnect), asserts each exact code+cause, requires all four response codes seen, and fails on 201 outside a Disconnect-ACK. - TestRFC5176ResidualSessionCauseInARequestNeverReachesANAK (R1(b)): requests carrying Error-Cause 201 and a Proxy-State holding a 201 TLV -> NAK with only [401], Proxy-State whole, no teardown. Records observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176ResidualSessionCauseInARequestNeverReachesANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L449) | unit/verify | revert, verified |
| positive | [`TestRFC5176ResidualSessionCauseOnlyInADisconnectACK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L441) | unit/verify | revert, verified |

### [`RFC5176-3.5-7`](#rfc5176-3.5-7)

"Invalid EAP Packet (Ignored)" is a non-fatal error that MUST NOT be sent by implementations of this specification. (§3.5, Error-Cause 202)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176InvalidEAPPacketCauseNeverSent: no response on any of the 15 paths carries 202. - TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest (R1(b)): an injected 202 is never echoed (401 only), and a Disconnect with a malformed EAP-Message gets [401], never 202. Records observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176InvalidEAPPacketCauseNeverSentForAnEAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L464) | unit/verify | revert, verified |
| positive | [`TestRFC5176InvalidEAPPacketCauseNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L457) | unit/verify | revert, verified |

### [`RFC5176-3.5-8`](#rfc5176-3.5-8)

"Request Not Routable" is a fatal error that MAY be sent by a proxy and MUST NOT be sent by a NAS. (§3.5, Error-Cause 502)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). Ze is the NAS. + TestRFC5176RequestNotRoutableCauseNeverSent: no response on any path carries 502. - TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession (R1(b)): injected 502 never echoed (401 only); a Disconnect naming another realm's user through another NAS-Identifier -> [503], never 502. Records observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176RequestNotRoutableCauseNeverSentForAForeignSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L490) | unit/verify | revert, verified |
| positive | [`TestRFC5176RequestNotRoutableCauseNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L483) | unit/verify | revert, verified |

### [`RFC5176-3.5-9`](#rfc5176-3.5-9)

It MUST NOT be sent within a CoA-ACK, CoA-NAK, or Disconnect-ACK, only within a Disconnect-NAK. (§3.5, "It" is Error-Cause 504, Session Context Not Removable)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). D-8 fixed here: handleDisconnect sent 503 (Session Context Not Found) for a located session whose teardown failed; it now sends 504 (radius.ErrorCauseSessionNotRemovable), also on the PPPoE not-wired branch (producer read). + TestRFC5176SessionNotRemovableOnlyInADisconnectNAK: the teardown-refused Disconnect path answers Disconnect-NAK [504] and no CoA-ACK, CoA-NAK or Disconnect-ACK on any path carries 504. - TestRFC5176SessionNotRemovableNeverInACoANAK: injected 504 never echoed; an undeliverable CoA gets CoA-NAK 506, never 504. Records observed red (handleDisconnect, handleCoA). Residual not proven here: the svc==nil branch after a found session still answers 503.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176SessionNotRemovableNeverInACoANAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L526) | unit/verify | revert, verified |
| positive | [`TestRFC5176SessionNotRemovableOnlyInADisconnectNAK`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L508) | unit/verify | revert, verified |

### [`RFC5176-3.6-2`](#rfc5176-3.6-2)

However, the same Attribute MUST NOT be used for both purposes simultaneously. (§3.6 Note 7, "the same Attribute" is a Vendor-Specific Attribute, the purposes session identification and authorization change)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176VendorSpecificIsAChangeOnly: Acct-Session-Id + MikroTik rate VSA -> CoA-ACK and one 10 Mbit/s change on session 20 (VSA used for change only). - TestRFC5176VendorSpecificIsNeverAnIdentity: VSA alone -> NAK 503, VSA beside an unknown Acct-Session-Id -> 503 (the VSA never selects the one live session), Disconnect + VSA -> 401; no change, no teardown. Records observed red (readCoAChange, oneSession).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176VendorSpecificIsNeverAnIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L574) | unit/verify | revert, verified |
| positive | [`TestRFC5176VendorSpecificIsAChangeOnly`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L542) | unit/verify | revert, verified |

### [`RFC5176-6.3-2`](#rfc5176-6.3-2)

The time window used for duplicate detection MUST be the same as the window used to detect a stale Event-Timestamp Attribute. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 10, independent). + TestRFC5176DuplicateAndTimestampWindowsBothHoldInside: at coaReplayWindow-10s a duplicate is answered from the cache (same octets, one teardown) and an Event-Timestamp that old is answered. - TestRFC5176DuplicateAndTimestampWindowsBothEndOutside: at window+10s the duplicate is processed again (second teardown) and the Event-Timestamp is silently discarded. The pair pins both mechanisms to one window within +/-10s: a shorter dedup window fails the positive, a longer one the negative. Records observed red (cachedReplay, eventTimestampState).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5176DuplicateAndTimestampWindowsBothEndOutside`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L673) | unit/verify | revert, verified |
| positive | [`TestRFC5176DuplicateAndTimestampWindowsBothHoldInside`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc5176_section_rows_test.go#L641) | unit/verify | revert, verified |

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
