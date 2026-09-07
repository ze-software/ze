# RFC 3579 - RADIUS (Remote Authentication Dial In User Service) Support For Extensible Authentication Protocol (EAP)

Partial. Every requirement this repository extracted from RFC 3579, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 54.8% | 17 of 31 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 31 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 31 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 94.3% | 33 of 35 tagged units, 2 escaped and 0 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 31 | of 50 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 31 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 31 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 31 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 31 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 45.2% | 14 of 31 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 31 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 50 |
| Gated MUST-level | 31 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 14 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 35 |
| Tagged units | 35 |
| Recorded audit verdicts | 0 |
| Discrimination records | 35 |
| Summary | `rfc/short/rfc3579.md` |
| Requirement shard | `rfc/requirements/rfc3579.md` |
| RFC text | `rfc/full/rfc3579.txt` |

## Enrolment

Enrolled: RADIUS/EAP, ze in the NAS (RADIUS client) role: 31 MUST-level requirements. Ze runs the conversation this document describes, for operator login. `internal/component/radius/dict.go` declares EAP-Message at 79, `SignMessageAuthenticator` (`internal/component/radius/packet.go`) signs every EAP-bearing Access-Request beside the two verifiers that were there first, and `authenticateEAP` (`internal/component/radius/authenticator_eap.go`) answers each Access-Challenge instead of rejecting it. The obligations addressed to the RADIUS authentication server, to a RADIUS proxy and to a security server are excluded in `rfc/extraction/rfc3579.json` as `binds-another-role`: ze binds an ephemeral UDP socket in `NewClient` and `(*Client).readLoop` dispatches a datagram only against a waiter `(*Client).Exchange` registered for its own outstanding request, so no Access-Request receive path exists anywhere in the tree.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze is the NAS and runs the RADIUS/EAP conversation for operator login, with `auth-method eap-md5` or `eap-mschapv2`. It encapsulates one EAP packet per RADIUS packet in consecutive EAP-Message attributes split at 253 octets (`appendEAPMessage`, `eapPacketFrom`, [`internal/component/radius/eap.go`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap.go)), signs every EAP-bearing Access-Request (`SignMessageAuthenticator`, [`internal/component/radius/packet.go`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet.go)), silently discards a reply whose Message-Authenticator does not verify (`(*Client).dispatchResponse`, [`internal/component/radius/client.go`](https://github.com/ze-software/ze/blob/main/internal/component/radius/client.go)), and answers each Access-Challenge with the peer's EAP-Response and the challenge's State unmodified (`authenticateEAP`, [`internal/component/radius/authenticator_eap.go`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap.go)). It copies the peer's own EAP-Response/Identity Type-Data into User-Name on every request of the conversation, validates each server EAP header before the peer sees it, processes the EAP-Message attributes before it reads the reply code, and never offers a password credential in place of the configured EAP method. Ze answers EAP itself, as the peer, rather than passing frames through.

**What the ledger says remains**

Fourteen MUST-level requirements are unproven or unreached, and none of them changes what an operator can configure. The pass-through architecture accounts for most: ze answers EAP for the operator and forwards no EAP frame, so Section 2.1's EAP-Start and the Nak relay have no path, and the Section 2.2 Error-Cause 202 branch cannot fire because ze holds no queue of EAP-Responses for its antecedent to match. Four prohibitions hold by construction and are tagged by nothing, because a test would have to observe a packet no code path can build: Sections 2.1, 2.6.3 and 2.6.5. EAP-TLS is implemented in the peer (`internal/core/eap`) and not offered by `auth-method`, which needs an operator certificate and key. Ze's PPP NAS never negotiates EAP, so the Section 4.3.6 obligations that bind a NAS whose peers require EAP are unreached. The IPsec-for-RADIUS profile of Section 4.2 is absent: ze offers no way to run RADIUS over an IPsec SA and no zero-length shared secret. Section 4.3.4's ban on sharing a secret between an EAP NAS and a User-Password NAS is not checked at config load.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 17 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **31** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (17):** [`RFC3579-1-1`](#rfc3579-1-1), [`RFC3579-1.2-1`](#rfc3579-1.2-1), [`RFC3579-2.1-1`](#rfc3579-2.1-1), [`RFC3579-2.1-3`](#rfc3579-2.1-3), [`RFC3579-2.2-1`](#rfc3579-2.2-1), [`RFC3579-2.6.3-1`](#rfc3579-2.6.3-1), [`RFC3579-2.6.3-2`](#rfc3579-2.6.3-2), [`RFC3579-2.6.4-1`](#rfc3579-2.6.4-1), [`RFC3579-3-1`](#rfc3579-3-1), [`RFC3579-3.1-1`](#rfc3579-3.1-1), [`RFC3579-3.1-2`](#rfc3579-3.1-2), [`RFC3579-3.1-3`](#rfc3579-3.1-3), [`RFC3579-3.1-4`](#rfc3579-3.1-4), [`RFC3579-3.2-1`](#rfc3579-3.2-1), [`RFC3579-3.3-1`](#rfc3579-3.3-1), [`RFC3579-3.3-2`](#rfc3579-3.3-2), [`RFC3579-4.3.6-2`](#rfc3579-4.3.6-2)

**Annotated instead of tested (14):** [`RFC3579-1-2`](#rfc3579-1-2), [`RFC3579-1.2-2`](#rfc3579-1.2-2), [`RFC3579-2.1-2`](#rfc3579-2.1-2), [`RFC3579-2.2-2`](#rfc3579-2.2-2), [`RFC3579-2.6.3-3`](#rfc3579-2.6.3-3), [`RFC3579-2.6.5-1`](#rfc3579-2.6.5-1), [`RFC3579-4.2-1`](#rfc3579-4.2-1), [`RFC3579-4.2-2`](#rfc3579-4.2-2), [`RFC3579-4.2-3`](#rfc3579-4.2-3), [`RFC3579-4.3.4-1`](#rfc3579-4.3.4-1), [`RFC3579-4.3.6-1`](#rfc3579-4.3.6-1), [`RFC3579-4.3.6-3`](#rfc3579-4.3.6-3), [`RFC3579-4.3.6-4`](#rfc3579-4.3.6-4), [`RFC3579-4.3.6-5`](#rfc3579-4.3.6-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3579-1-1` | "a NAS that is unable to offer EAP service MUST NOT implement the RADIUS attributes for EAP" (§1) | MUST NOT | 1 - Introduction | **positive:** `unit/verify` [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L110). **negative:** `unit/verify` [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L150) |
| `RFC3579-1-2` | "A NAS MUST treat a RADIUS Access-Accept requesting an unavailable service as an Access-Reject instead" (§1) | MUST | 1 - Introduction | **positive:** no positive test. **negative:** no negative test. **{gap}:** `(*radiusAuthenticator).result` rejects an Access-Accept whose Service-Type is not Login-User (`AcceptedServiceType`, internal/component/radius/attr.go), which is the RFC 2865 form of this rule and covers the EAP path too, because authenticateEAP returns through that same function. What stays untested is this document's own form of the rule, an Accept concluding an EAP conversation ze could not run: `parseAuthMethod` refuses an unknown auth-method at config load, so ze only ever runs a method it offers and the branch has no input |
| `RFC3579-1.2-1` | A displayable message "MUST NOT affect operation of the protocol" (§1.2) | MUST NOT | 1.2 - Terminology | **positive:** `unit/verify` [`TestRadiusAdminEapNotificationIsAnsweredAndLogged`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L359). **negative:** `unit/verify` [`TestRadiusAdminEapNotificationDoesNotSteerTheLogin`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L396) |
| `RFC3579-1.2-2` | "The message encoding MUST follow the UTF-8 transformation format [RFC2279]" (§1.2) | MUST | 1.2 - Terminology | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze now DECODES a displayable message -- `(*PeerSession).notificationResponse` (internal/core/eap/peer.go) carries the Type-2 Request's text out and `processEAPMessage` (internal/component/radius/authenticator_eap.go) logs it -- and encodes none. This sentence binds whoever encodes the message, and ze writes no Reply-Message and sends no Notification Request, so nothing here chooses an encoding to assert |
| `RFC3579-2.1-1` | "Reception of a RADIUS Access-Reject packet MUST result in the NAS denying access to the authenticating peer" (§2.1, restated later in §2.1) | MUST | 2.1 - Protocol Overview | **positive:** `unit/verify` [`TestRadiusAdminEapAccessRejectDeniesAccess`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L430). **negative:** `unit/verify` [`TestRadiusAdminEapAccessRejectDeniesEvenCarryingEAPSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L455) |
| `RFC3579-2.1-2` | "The NAS MUST NOT \\"manufacture\\" a Success or Failure packet as the result of a timeout" (§2.1) | MUST NOT | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test. **{gap}:** every EAP packet ze puts on the RADIUS wire came out of `(*eap.PeerSession).Process` (internal/core/eap/peer.go), and a timeout returns an error from `(*Client).SendToServers` that `authenticateEAP` passes up, so no branch manufactures a Success or Failure. Nothing tags the prohibition, because a test would have to observe a packet no code path can build |
| `RFC3579-2.1-3` | "if the NAS initially sends an EAP-Request/Identity message to the peer, the NAS MUST copy the contents of the Type-Data field of the EAP-Response/Identity received from the peer into the User-Name attribute and MUST include the Type-Data field of the EAP-Response/Identity in the User-Name attribute in every subsequent Access-Request" (§2.1) | MUST | 2.1 - Protocol Overview | **positive:** `unit/verify` [`TestRadiusAdminEapUserNameIsThePeerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L74). **negative:** `unit/verify` [`TestRadiusAdminEapUserNameRidesEverySubsequentRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L109) |
| `RFC3579-2.2-1` | "the NAS MUST validate the EAP header fields (Code, Identifier, Length) prior to forwarding an EAP packet to or from the RADIUS server" (§2.2) | MUST | 2.2 - Invalid Packets | **positive:** `unit/verify` [`TestRadiusAdminEapReadsTheServerEAPHeaderFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L152). **negative:** `unit/verify` [`TestRadiusAdminEapRefusesAMalformedServerEAPHeader`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L190) |
| `RFC3579-2.2-2` | On an Access-Challenge carrying Error-Cause 202, "a new EAP-Response packet, if available, MUST be sent to the RADIUS server within an Access-Request, and the EAP-Message attribute(s) included within the Access-Challenge are silently discarded" (§2.2) | MUST | 2.2 - Invalid Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Error-Cause attribute, so an Access-Challenge carrying 202 is answered like any other. The MUST is conditional -- "If so" refers to additional EAP-Response packets received matching the current Identifier -- and ze holds no such queue: it is its own peer, and `(*eap.PeerSession).Process` (internal/core/eap/peer.go) returns exactly one Response per Request. So the antecedent never holds and there is no second Response to send |
| `RFC3579-2.6.3-1` | "The NAS MUST make its access control decision based solely on the RADIUS Packet Type (Access-Accept/Access-Reject)" (§2.6.3) | MUST | 2.6.3 - Conflicting Messages | **positive:** `unit/verify` [`TestRadiusAdminEapDecisionFollowsTheRadiusCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L586). **negative:** `unit/verify` [`TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L863) |
| `RFC3579-2.6.3-2` | "The access control decision MUST NOT be based on the contents of the EAP packet encapsulated in one or more EAP-Message attributes, if present" (§2.6.3) | MUST NOT | 2.6.3 - Conflicting Messages | **positive:** `unit/verify` [`TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L867). **negative:** `unit/verify` [`TestRadiusAdminEapDecisionFollowsTheRadiusCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L589) |
| `RFC3579-2.6.3-3` | "the NAS MUST NOT \\"manufacture\\" EAP packets in order to correct contradictory messages that it receives" (§2.6.3) | MUST NOT | 2.6.3 - Conflicting Messages | **positive:** no positive test. **negative:** no negative test. **{gap}:** `authenticateEAP` (internal/component/radius/authenticator_eap.go) sends only what `(*eap.PeerSession).Process` returned, and on a contradictory reply it logs the peer's objection and drops it rather than correcting it. Nothing tags the prohibition, because a test would have to observe a manufactured packet no code path can build |
| `RFC3579-2.6.4-1` | "the NAS MUST first process the attributes, including the EAP-Message attribute(s), prior to processing the Accept/Reject indication" (§2.6.4) | MUST | 2.6.4 - Priority | **positive:** `unit/verify` [`TestRadiusAdminEapProcessesTheEAPMessageBeforeTheCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L230). **negative:** `unit/verify` [`TestRadiusAdminEapProcessesAnUnparseableEAPMessageBeforeTheCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L260) |
| `RFC3579-2.6.5-1` | "Reply-Message attribute(s) MUST NOT be included in any RADIUS message containing an EAP-Message attribute" (§2.6.5) | MUST NOT | 2.6.5 - Displayable Messages | **positive:** no positive test. **negative:** no negative test. **{gap}:** `eapCredential` (internal/component/radius/authenticator_eap.go) builds an EAP-bearing Access-Request from the EAP-Message run, the Message-Authenticator and the server's State, and appends no Reply-Message; `exchange` adds Service-Type, NAS-Identifier, User-Name and NAS-IP-Address, none of them Reply-Message. The exclusion holds by construction and no test tags it |
| `RFC3579-3-1` | "either NAS-Identifier, NAS-IP-Address or NAS-IPv6-Address attributes MUST be included" in an Access-Request (§3) | MUST | 3 - Attributes | **positive:** `unit/verify` [`TestRadiusAdminEapAccessRequestNamesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L292). **negative:** `unit/verify` [`TestRadiusAdminEapNamesTheNASWithoutASourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L327) |
| `RFC3579-3.1-1` | "If multiple EAP-Message attributes are contained within an Access-Request or Access-Challenge packet, they MUST be in order and they MUST be consecutive attributes" (§3.1) | MUST | 3.1 - EAP-Message | **positive:** `unit/verify` [`TestEAPMessageConcatenatesOnTheWayIn`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L135). **positive:** `unit/verify` [`TestEAPMessageSplitsAtTheAttributeLimit`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L51). **negative:** `unit/verify` [`TestEAPMessageKeepsTheRunConsecutive`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L95) |
| `RFC3579-3.1-2` | "Multiple EAP packets MUST NOT be encoded within EAP-Message attributes contained within a single Access-Challenge, Access-Accept, Access-Reject or Access-Request packet" (§3.1) | MUST NOT | 3.1 - EAP-Message | **positive:** `unit/verify` [`TestRadiusAdminEapOneEAPPacketPerRadiusPacket`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L548). **negative:** `unit/verify` [`TestEAPMessageRefusesASecondRun`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L166) |
| `RFC3579-3.1-3` | "the Message-Authenticator attribute MUST be used to protect all Access-Request, Access-Challenge, Access-Accept, and Access-Reject packets containing an EAP-Message attribute" (§3.1, restated §3.2, §3.3 Note 1 and §4.3.2) | MUST | 3.1 - EAP-Message | **positive:** `unit/verify` [`TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L375). **negative:** `unit/verify` [`TestEAPRequestWithoutMessageAuthenticatorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L732) |
| `RFC3579-3.1-4` | "A NAS supporting the EAP-Message attribute MUST calculate the correct value of the Message-Authenticator and MUST silently discard the packet if it does not match the value sent" (§3.1, restated for any RADIUS client receiving an Access-Accept, Access-Reject or Access-Challenge at §3.2) | MUST | 3.1 - EAP-Message | **positive:** `unit/verify` [`TestRadiusAdminEapVerifiedChallengeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L898). **negative:** `unit/verify` [`TestRadiusAdminEapDiscardsUnauthenticatedChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L484) |
| `RFC3579-3.2-1` | "Message-Authenticator = HMAC-MD5 (Type, Identifier, Length, Request Authenticator, Attributes)", keyed with the shared secret, with the signature string "considered to be sixteen octets of zero" and the value inserted before the Response Authenticator is calculated (§3.2) | MUST | 3.2 - Message-Authenticator | **positive:** `unit/verify` [`TestSignMessageAuthenticatorMatchesRFC3579`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L438). **negative:** `unit/verify` [`TestSignMessageAuthenticatorZeroesTheSignatureField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L472) |
| `RFC3579-3.3-1` | "The EAP-Message and Message-Authenticator attributes specified in this document MUST NOT be present in an Accounting-Request" (§3.3) | MUST NOT | 3.3 - Table of Attributes | **positive:** `unit/verify` [`TestAccountingRequestRefusesEAPAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L691). **negative:** `unit/verify` [`TestAccountingRequestRefusesEAPAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L688) |
| `RFC3579-3.3-2` | "An Access-Request that contains either a User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes" (§3.3 Note 1) | MUST NOT | 3.3 - Table of Attributes | **positive:** `unit/verify` [`TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L379). **negative:** `unit/verify` [`TestAccessRequestRefusesTwoCredentialTypes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L938) |
| `RFC3579-4.2-1` | Where RADIUS runs over IPsec ESP with a non-null transform and no shared secret is configured, "a shared secret of zero length MUST be assumed" (§4.2) | MUST | 4.2 - Security Protocol | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze offers no way to run RADIUS over an IPsec SA and its RADIUS server configuration always carries a secret (`internal/component/radius/config.go`), so the zero-length case is unreachable; spec-radius-admin-eap does not add it |
| `RFC3579-4.2-2` | "When IPsec ESP is used with RADIUS, per-packet authentication, integrity and replay protection MUST be used. 3DES-CBC MUST be supported as an encryption transform" (§4.2) | MUST | 4.2 - Security Protocol | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no RADIUS-over-IPsec profile, so nothing selects a transform for RADIUS traffic; spec-radius-admin-eap does not add it |
| `RFC3579-4.2-3` | "HMAC-SHA1-96 MUST be supported as an authentication transform" (§4.2) | MUST | 4.2 - Security Protocol | **positive:** no positive test. **negative:** no negative test. **{gap}:** same absent RADIUS-over-IPsec profile; ze negotiates no IPsec SA for RADIUS traffic; spec-radius-admin-eap does not add it |
| `RFC3579-4.3.4-1` | "the RADIUS shared secret used by a NAS supporting EAP MUST NOT be reused by a NAS utilizing the User-Password attribute" (§4.3.4) | MUST NOT | 4.3.4 - Known Plaintext Attacks | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds both the admin backend's secret and the subscriber backend's secret and compares neither, so a configuration reusing one secret across an EAP NAS and a PAP NAS is accepted in silence; spec-radius-admin-eap |
| `RFC3579-4.3.6-1` | "Should the NAS not be able to negotiate EAP, or should the EAP-Request sent by the NAS be of a different EAP type than what is expected, the authenticating peer MUST disconnect" (§4.3.6) | MUST | 4.3.6 - Negotiation Attacks | **positive:** no positive test. **negative:** no negative test. **{gap}:** `(*eap.PeerSession).Process` (internal/core/eap/peer.go) NAKs toward its one configured method rather than answering a Request of another type, and a server that insists ends the exchange with an error that `authenticateEAP` returns, which ends the login. What is untested is that ending read as this section's disconnect |
| `RFC3579-4.3.6-2` | "An authenticating peer expecting EAP to be negotiated for a session MUST NOT negotiate a weaker method, such as CHAP or PAP" (§4.3.6) | MUST NOT | 4.3.6 - Negotiation Attacks | **positive:** `unit/verify` [`TestRadiusAdminEapNeverSendsAPasswordCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L483). **negative:** `unit/verify` [`TestRadiusAdminEapDoesNotDowngradeAfterARejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L515) |
| `RFC3579-4.3.6-3` | "if any peers of the NAS MUST do EAP, then the NAS MUST attempt to negotiate EAP for every session" (§4.3.6) | MUST | 4.3.6 - Negotiation Attacks | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's PPP NAS offers PAP, CHAP-MD5 and MS-CHAPv2 and never EAP (`internal/component/l2tp/ppp`), so it cannot attempt to negotiate EAP for any session; spec-radius-admin-eap |
| `RFC3579-4.3.6-4` | "The authenticating peer MUST refuse to renegotiate authentication, even if the renegotiation is from CHAP to EAP" (§4.3.6, restated in the same section for an EAP-capable peer) | MUST | 4.3.6 - Negotiation Attacks | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's admin login has no renegotiation path, and its PPP NAS never negotiates EAP, so nothing refuses a renegotiation; spec-radius-admin-eap |
| `RFC3579-4.3.6-5` | Where EAP was negotiated and the RADIUS server or proxy does not support it, "a PPP NAS MUST send an LCP-Terminate and disconnect the peer" (§4.3.6) | MUST | 4.3.6 - Negotiation Attacks | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's PPP NAS never negotiates EAP, so the branch that would send LCP-Terminate on an EAP-incapable server does not exist; spec-radius-admin-eap |
| `RFC3579-1.2-3` | An implementation that silently discards a packet "SHOULD provide the capability of logging the error, including the contents of the silently discarded packet, and SHOULD record the event in a statistics counter" (§1.2) | SHOULD | 1.2 - Terminology | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-4` | "Once EAP has been negotiated, the NAS SHOULD send an initial EAP-Request message to the authenticating peer" (§2.1) | SHOULD | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-7` | "After a suitable number of timeouts have elapsed, the NAS SHOULD instead end the EAP conversation" (§2.1) | SHOULD | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-8` | On an EAP-Response/Nak "the NAS SHOULD send Access-Request encapsulating the received EAP-Response/Nak" (§2.1) | SHOULD | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-9` | Where the peer identity cannot be determined from the EAP-Response, "the User-Name attribute SHOULD be determined by another means" (§2.1) | SHOULD | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-11` | Where EAP-unaware peers are common, the EAP-Start technique "SHOULD NOT be employed by default" (§2.1) | SHOULD NOT | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.2-3` | A NAS receiving an Access-Challenge with Error-Cause 202 "SHOULD discard the EAP-Response packet most recently transmitted to the RADIUS server and check whether additional EAP-Response packets have been received matching the current Identifier value" (§2.2) | SHOULD | 2.2 - Invalid Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.6.5-2` | "a NAS receiving a Reply-Message attribute from the RADIUS server SHOULD silently discard the attribute, rather than attempting to translate it to an EAP Notification Request" (§2.6.5) | SHOULD | 2.6.5 - Displayable Messages | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-3-2` | "The NAS-Port or NAS-Port-Id attributes SHOULD be included by the NAS in Access-Request packets" (§3) | SHOULD | 3 - Attributes | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-3.1-5` | "When RADIUS is used to enable EAP authentication, Access-Request, Access-Challenge, Access-Accept, and Access-Reject packets SHOULD contain one or more EAP-Message attributes" (§3.1) | SHOULD | 3.1 - EAP-Message | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-3.1-6` | "Access-Challenge, Access-Accept, or Access-Reject packets including EAP-Message attribute(s) without a Message-Authenticator attribute SHOULD be silently discarded by the NAS" (§3.1) | SHOULD | 3.1 - EAP-Message | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-4.2-4` | "implementations of this specification SHOULD support IPsec [RFC2401] along with IKE [RFC2409] for key management", and IPsec ESP with a non-null encryption transform and authentication "SHOULD be used" (§4.2) | SHOULD | 4.2 - Security Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-4.2-5` | "AES-CBC SHOULD be supported" and "SHOULD be offered as a preferred encryption transform if supported", and "DES-CBC SHOULD NOT be used as the encryption transform" (§4.2) | SHOULD | 4.2 - Security Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-4.3.2-1` | "the Message-Authenticator attribute SHOULD be used in Access-Request packets that do not have a User-Password attribute, in order to establish the identity of the NAS sending the request" (§4.3.2) | SHOULD | 4.3.2 - Spoofing and Hijacking | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2-1` | "A NAS MAY authenticate local peers while at the same time acting as a pass-through for non-local peers and authentication methods it does not implement locally" (§2) | MAY | 2 - RADIUS Support for EAP | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-5` | "A NAS MAY be configured to initiate with a default authentication method" (§2.1) | MAY | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-6` | "the NAS MAY act as a pass-through, encapsulating the EAP-Response within EAP-Message attribute(s) sent to the RADIUS server within a RADIUS Access-Request packet" (§2.1) | MAY | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-2.1-10` | "on detecting the presence of the peer, the NAS MAY send an Access-Request packet to the RADIUS server containing an EAP-Message attribute signifying EAP-Start" (§2.1) | MAY | 2.1 - Protocol Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC3579-3.2-2` | The Message-Authenticator attribute "MAY be used to authenticate and integrity-protect Access-Requests in order to prevent spoofing" and "MAY be used in any Access-Request" (§3.2) | MAY | 3.2 - Message-Authenticator | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3579-1-2`](#rfc3579-1-2) "A NAS MUST treat a RADIUS Access-Accept requesting an unavailable service as an Access-Reject instead" (§1) | {gap}, no test | `(*radiusAuthenticator).result` rejects an Access-Accept whose Service-Type is not Login-User (`AcceptedServiceType`, internal/component/radius/attr.go), which is the RFC 2865 form of this rule and covers the EAP path too, because authenticateEAP returns through that same function. What stays untested is this document's own form of the rule, an Accept concluding an EAP conversation ze could not run: `parseAuthMethod` refuses an unknown auth-method at config load, so ze only ever runs a method it offers and the branch has no input |
| [`RFC3579-1.2-2`](#rfc3579-1.2-2) "The message encoding MUST follow the UTF-8 transformation format [RFC2279]" (§1.2) | {gap}, no test | ze now DECODES a displayable message -- `(*PeerSession).notificationResponse` (internal/core/eap/peer.go) carries the Type-2 Request's text out and `processEAPMessage` (internal/component/radius/authenticator_eap.go) logs it -- and encodes none. This sentence binds whoever encodes the message, and ze writes no Reply-Message and sends no Notification Request, so nothing here chooses an encoding to assert |
| [`RFC3579-2.1-2`](#rfc3579-2.1-2) "The NAS MUST NOT \\"manufacture\\" a Success or Failure packet as the result of a timeout" (§2.1) | {gap}, no test | every EAP packet ze puts on the RADIUS wire came out of `(*eap.PeerSession).Process` (internal/core/eap/peer.go), and a timeout returns an error from `(*Client).SendToServers` that `authenticateEAP` passes up, so no branch manufactures a Success or Failure. Nothing tags the prohibition, because a test would have to observe a packet no code path can build |
| [`RFC3579-2.2-2`](#rfc3579-2.2-2) On an Access-Challenge carrying Error-Cause 202, "a new EAP-Response packet, if available, MUST be sent to the RADIUS server within an Access-Request, and the EAP-Message attribute(s) included within the Access-Challenge are silently discarded" (§2.2) | {gap}, no test | ze reads no Error-Cause attribute, so an Access-Challenge carrying 202 is answered like any other. The MUST is conditional -- "If so" refers to additional EAP-Response packets received matching the current Identifier -- and ze holds no such queue: it is its own peer, and `(*eap.PeerSession).Process` (internal/core/eap/peer.go) returns exactly one Response per Request. So the antecedent never holds and there is no second Response to send |
| [`RFC3579-2.6.3-3`](#rfc3579-2.6.3-3) "the NAS MUST NOT \\"manufacture\\" EAP packets in order to correct contradictory messages that it receives" (§2.6.3) | {gap}, no test | `authenticateEAP` (internal/component/radius/authenticator_eap.go) sends only what `(*eap.PeerSession).Process` returned, and on a contradictory reply it logs the peer's objection and drops it rather than correcting it. Nothing tags the prohibition, because a test would have to observe a manufactured packet no code path can build |
| [`RFC3579-2.6.5-1`](#rfc3579-2.6.5-1) "Reply-Message attribute(s) MUST NOT be included in any RADIUS message containing an EAP-Message attribute" (§2.6.5) | {gap}, no test | `eapCredential` (internal/component/radius/authenticator_eap.go) builds an EAP-bearing Access-Request from the EAP-Message run, the Message-Authenticator and the server's State, and appends no Reply-Message; `exchange` adds Service-Type, NAS-Identifier, User-Name and NAS-IP-Address, none of them Reply-Message. The exclusion holds by construction and no test tags it |
| [`RFC3579-4.2-1`](#rfc3579-4.2-1) Where RADIUS runs over IPsec ESP with a non-null transform and no shared secret is configured, "a shared secret of zero length MUST be assumed" (§4.2) | {gap}, no test | ze offers no way to run RADIUS over an IPsec SA and its RADIUS server configuration always carries a secret (`internal/component/radius/config.go`), so the zero-length case is unreachable; spec-radius-admin-eap does not add it |
| [`RFC3579-4.2-2`](#rfc3579-4.2-2) "When IPsec ESP is used with RADIUS, per-packet authentication, integrity and replay protection MUST be used. 3DES-CBC MUST be supported as an encryption transform" (§4.2) | {gap}, no test | ze has no RADIUS-over-IPsec profile, so nothing selects a transform for RADIUS traffic; spec-radius-admin-eap does not add it |
| [`RFC3579-4.2-3`](#rfc3579-4.2-3) "HMAC-SHA1-96 MUST be supported as an authentication transform" (§4.2) | {gap}, no test | same absent RADIUS-over-IPsec profile; ze negotiates no IPsec SA for RADIUS traffic; spec-radius-admin-eap does not add it |
| [`RFC3579-4.3.4-1`](#rfc3579-4.3.4-1) "the RADIUS shared secret used by a NAS supporting EAP MUST NOT be reused by a NAS utilizing the User-Password attribute" (§4.3.4) | {gap}, no test | ze holds both the admin backend's secret and the subscriber backend's secret and compares neither, so a configuration reusing one secret across an EAP NAS and a PAP NAS is accepted in silence; spec-radius-admin-eap |
| [`RFC3579-4.3.6-1`](#rfc3579-4.3.6-1) "Should the NAS not be able to negotiate EAP, or should the EAP-Request sent by the NAS be of a different EAP type than what is expected, the authenticating peer MUST disconnect" (§4.3.6) | {gap}, no test | `(*eap.PeerSession).Process` (internal/core/eap/peer.go) NAKs toward its one configured method rather than answering a Request of another type, and a server that insists ends the exchange with an error that `authenticateEAP` returns, which ends the login. What is untested is that ending read as this section's disconnect |
| [`RFC3579-4.3.6-3`](#rfc3579-4.3.6-3) "if any peers of the NAS MUST do EAP, then the NAS MUST attempt to negotiate EAP for every session" (§4.3.6) | {gap}, no test | ze's PPP NAS offers PAP, CHAP-MD5 and MS-CHAPv2 and never EAP (`internal/component/l2tp/ppp`), so it cannot attempt to negotiate EAP for any session; spec-radius-admin-eap |
| [`RFC3579-4.3.6-4`](#rfc3579-4.3.6-4) "The authenticating peer MUST refuse to renegotiate authentication, even if the renegotiation is from CHAP to EAP" (§4.3.6, restated in the same section for an EAP-capable peer) | {gap}, no test | ze's admin login has no renegotiation path, and its PPP NAS never negotiates EAP, so nothing refuses a renegotiation; spec-radius-admin-eap |
| [`RFC3579-4.3.6-5`](#rfc3579-4.3.6-5) Where EAP was negotiated and the RADIUS server or proxy does not support it, "a PPP NAS MUST send an LCP-Terminate and disconnect the peer" (§4.3.6) | {gap}, no test | ze's PPP NAS never negotiates EAP, so the branch that would send LCP-Terminate on an EAP-incapable server does not exist; spec-radius-admin-eap |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3579-1-1`](#rfc3579-1-1)

"a NAS that is unable to offer EAP service MUST NOT implement the RADIUS attributes for EAP" (§1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L150) | unit/verify | no-break escape (declaration-only), which is not a proof, verified |
| positive | [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L110) | unit/verify | no-break escape (declaration-only), which is not a proof, verified |

### [`RFC3579-1-2`](#rfc3579-1-2)

"A NAS MUST treat a RADIUS Access-Accept requesting an unavailable service as an Access-Reject instead" (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-1-2, so no unit is bound to it.

### [`RFC3579-1.2-1`](#rfc3579-1.2-1)

A displayable message "MUST NOT affect operation of the protocol" (§1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapNotificationDoesNotSteerTheLogin`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L396) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapNotificationIsAnsweredAndLogged`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L359) | unit/verify | revert, verified |

### [`RFC3579-1.2-2`](#rfc3579-1.2-2)

"The message encoding MUST follow the UTF-8 transformation format [RFC2279]" (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-1.2-2, so no unit is bound to it.

### [`RFC3579-2.1-1`](#rfc3579-2.1-1)

"Reception of a RADIUS Access-Reject packet MUST result in the NAS denying access to the authenticating peer" (§2.1, restated later in §2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapAccessRejectDeniesEvenCarryingEAPSuccess`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L455) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapAccessRejectDeniesAccess`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L430) | unit/verify | revert, verified |

### [`RFC3579-2.1-2`](#rfc3579-2.1-2)

"The NAS MUST NOT \"manufacture\" a Success or Failure packet as the result of a timeout" (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-2.1-2, so no unit is bound to it.

### [`RFC3579-2.1-3`](#rfc3579-2.1-3)

"if the NAS initially sends an EAP-Request/Identity message to the peer, the NAS MUST copy the contents of the Type-Data field of the EAP-Response/Identity received from the peer into the User-Name attribute and MUST include the Type-Data field of the EAP-Response/Identity in the User-Name attribute in every subsequent Access-Request" (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapUserNameRidesEverySubsequentRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapUserNameIsThePeerIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L74) | unit/verify | revert, verified |

### [`RFC3579-2.2-1`](#rfc3579-2.2-1)

"the NAS MUST validate the EAP header fields (Code, Identifier, Length) prior to forwarding an EAP packet to or from the RADIUS server" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapRefusesAMalformedServerEAPHeader`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L190) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapReadsTheServerEAPHeaderFields`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L152) | unit/verify | revert, verified |

### [`RFC3579-2.2-2`](#rfc3579-2.2-2)

On an Access-Challenge carrying Error-Cause 202, "a new EAP-Response packet, if available, MUST be sent to the RADIUS server within an Access-Request, and the EAP-Message attribute(s) included within the Access-Challenge are silently discarded" (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-2.2-2, so no unit is bound to it.

### [`RFC3579-2.6.3-1`](#rfc3579-2.6.3-1)

"The NAS MUST make its access control decision based solely on the RADIUS Packet Type (Access-Accept/Access-Reject)" (§2.6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L863) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapDecisionFollowsTheRadiusCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L586) | unit/verify | revert, verified |

### [`RFC3579-2.6.3-2`](#rfc3579-2.6.3-2)

"The access control decision MUST NOT be based on the contents of the EAP packet encapsulated in one or more EAP-Message attributes, if present" (§2.6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapDecisionFollowsTheRadiusCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L589) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L867) | unit/verify | revert, verified |

### [`RFC3579-2.6.3-3`](#rfc3579-2.6.3-3)

"the NAS MUST NOT \"manufacture\" EAP packets in order to correct contradictory messages that it receives" (§2.6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-2.6.3-3, so no unit is bound to it.

### [`RFC3579-2.6.4-1`](#rfc3579-2.6.4-1)

"the NAS MUST first process the attributes, including the EAP-Message attribute(s), prior to processing the Accept/Reject indication" (§2.6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapProcessesAnUnparseableEAPMessageBeforeTheCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L260) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapProcessesTheEAPMessageBeforeTheCode`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L230) | unit/verify | revert, verified |

### [`RFC3579-2.6.5-1`](#rfc3579-2.6.5-1)

"Reply-Message attribute(s) MUST NOT be included in any RADIUS message containing an EAP-Message attribute" (§2.6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-2.6.5-1, so no unit is bound to it.

### [`RFC3579-3-1`](#rfc3579-3-1)

"either NAS-Identifier, NAS-IP-Address or NAS-IPv6-Address attributes MUST be included" in an Access-Request (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapNamesTheNASWithoutASourceAddress`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L327) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapAccessRequestNamesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L292) | unit/verify | revert, verified |

### [`RFC3579-3.1-1`](#rfc3579-3.1-1)

"If multiple EAP-Message attributes are contained within an Access-Request or Access-Challenge packet, they MUST be in order and they MUST be consecutive attributes" (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPMessageKeepsTheRunConsecutive`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestEAPMessageConcatenatesOnTheWayIn`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestEAPMessageSplitsAtTheAttributeLimit`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L51) | unit/verify | revert, verified |

### [`RFC3579-3.1-2`](#rfc3579-3.1-2)

"Multiple EAP packets MUST NOT be encoded within EAP-Message attributes contained within a single Access-Challenge, Access-Accept, Access-Reject or Access-Request packet" (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPMessageRefusesASecondRun`](https://github.com/ze-software/ze/blob/main/internal/component/radius/eap_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapOneEAPPacketPerRadiusPacket`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L548) | unit/verify | revert, verified |

### [`RFC3579-3.1-3`](#rfc3579-3.1-3)

"the Message-Authenticator attribute MUST be used to protect all Access-Request, Access-Challenge, Access-Accept, and Access-Reject packets containing an EAP-Message attribute" (§3.1, restated §3.2, §3.3 Note 1 and §4.3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPRequestWithoutMessageAuthenticatorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L732) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L375) | unit/verify | revert, verified |

### [`RFC3579-3.1-4`](#rfc3579-3.1-4)

"A NAS supporting the EAP-Message attribute MUST calculate the correct value of the Message-Authenticator and MUST silently discard the packet if it does not match the value sent" (§3.1, restated for any RADIUS client receiving an Access-Accept, Access-Reject or Access-Challenge at §3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapDiscardsUnauthenticatedChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L484) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapVerifiedChallengeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L898) | unit/verify | revert, verified |

### [`RFC3579-3.2-1`](#rfc3579-3.2-1)

"Message-Authenticator = HMAC-MD5 (Type, Identifier, Length, Request Authenticator, Attributes)", keyed with the shared secret, with the signature string "considered to be sixteen octets of zero" and the value inserted before the Response Authenticator is calculated (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSignMessageAuthenticatorZeroesTheSignatureField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L472) | unit/verify | revert, verified |
| positive | [`TestSignMessageAuthenticatorMatchesRFC3579`](https://github.com/ze-software/ze/blob/main/internal/component/radius/packet_test.go#L438) | unit/verify | revert, verified |

### [`RFC3579-3.3-1`](#rfc3579-3.3-1)

"The EAP-Message and Message-Authenticator attributes specified in this document MUST NOT be present in an Accounting-Request" (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAccountingRequestRefusesEAPAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L688) | unit/verify | revert, verified |
| positive | [`TestAccountingRequestRefusesEAPAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L691) | unit/verify | revert, verified |

### [`RFC3579-3.3-2`](#rfc3579-3.3-2)

"An Access-Request that contains either a User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes" (§3.3 Note 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAccessRequestRefusesTwoCredentialTypes`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L938) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapAccessRequestIsSignedAndCarriesEAPMessage`](https://github.com/ze-software/ze/blob/main/internal/component/radius/authenticator_eap_test.go#L379) | unit/verify | revert, verified |

### [`RFC3579-4.2-1`](#rfc3579-4.2-1)

Where RADIUS runs over IPsec ESP with a non-null transform and no shared secret is configured, "a shared secret of zero length MUST be assumed" (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.2-1, so no unit is bound to it.

### [`RFC3579-4.2-2`](#rfc3579-4.2-2)

"When IPsec ESP is used with RADIUS, per-packet authentication, integrity and replay protection MUST be used. 3DES-CBC MUST be supported as an encryption transform" (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.2-2, so no unit is bound to it.

### [`RFC3579-4.2-3`](#rfc3579-4.2-3)

"HMAC-SHA1-96 MUST be supported as an authentication transform" (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.2-3, so no unit is bound to it.

### [`RFC3579-4.3.4-1`](#rfc3579-4.3.4-1)

"the RADIUS shared secret used by a NAS supporting EAP MUST NOT be reused by a NAS utilizing the User-Password attribute" (§4.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.3.4-1, so no unit is bound to it.

### [`RFC3579-4.3.6-1`](#rfc3579-4.3.6-1)

"Should the NAS not be able to negotiate EAP, or should the EAP-Request sent by the NAS be of a different EAP type than what is expected, the authenticating peer MUST disconnect" (§4.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.3.6-1, so no unit is bound to it.

### [`RFC3579-4.3.6-2`](#rfc3579-4.3.6-2)

"An authenticating peer expecting EAP to be negotiated for a session MUST NOT negotiate a weaker method, such as CHAP or PAP" (§4.3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRadiusAdminEapDoesNotDowngradeAfterARejection`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L515) | unit/verify | revert, verified |
| positive | [`TestRadiusAdminEapNeverSendsAPasswordCredential`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc3579_nas_obligations_test.go#L483) | unit/verify | revert, verified |

### [`RFC3579-4.3.6-3`](#rfc3579-4.3.6-3)

"if any peers of the NAS MUST do EAP, then the NAS MUST attempt to negotiate EAP for every session" (§4.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.3.6-3, so no unit is bound to it.

### [`RFC3579-4.3.6-4`](#rfc3579-4.3.6-4)

"The authenticating peer MUST refuse to renegotiate authentication, even if the renegotiation is from CHAP to EAP" (§4.3.6, restated in the same section for an EAP-capable peer)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.3.6-4, so no unit is bound to it.

### [`RFC3579-4.3.6-5`](#rfc3579-4.3.6-5)

Where EAP was negotiated and the RADIUS server or proxy does not support it, "a PPP NAS MUST send an LCP-Terminate and disconnect the peer" (§4.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3579-4.3.6-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, plan/spec-radius-admin-eap.md phase 2, rfc3579 |
| Signed off | 2026-09-03 |
| Register | rfc2119 |
| Source | rfc/full/rfc3579.txt |
| Source fingerprint | af4e5a944b99ad93 |
| Record | rfc/extraction/rfc3579.json |
| Mapped sentences | 30 |
| Declined as scope | 26 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract and table of contents. The Abstract says what the memo adds to RADIUS and binds no speaker. |
| `1` | Introduction | 3 | walked | Introduction. States what RADIUS and EAP are, and carries the RFC 2865 rule that a NAS must not implement the attributes for a service it cannot offer, in its EAP form. |
| `1.1` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph. No obligation of its own. |
| `1.2` | Terminology | 2 | walked | Terminology. Defines authenticator, peer, authentication server, silently discard, displayable message, NAS, service and session. Two of those definitions carry obligations, sites 1.2:1 and 1.2:2; 'silently discard' carries two SHOULDs captured as RFC3579-1.2-3. |
| `2` | RADIUS Support for EAP | 0 | walked | RADIUS Support for EAP. Says why the pass-through architecture exists and that a NAS may mix local authentication with pass-through, which is the MAY captured as RFC3579-2-1. |
| `2.1` | Protocol Overview | 8 | walked | Protocol Overview. The conversation itself: who sends the first EAP-Request, how the NAS encapsulates a Response, what ends the conversation, EAP-Start, and the User-Name copying rule that lets a non-EAP-aware proxy forward the request. |
| `2.2` | Invalid Packets | 3 | walked | Invalid Packets. The NAS header validation, the Error-Cause 202 branch, and the RADIUS server's fatal and non-fatal error handling. The DoS advice at the end is written as 'it is advisable' and 'is recommended' in lower case, and binds nobody. |
| `2.3` | Retransmission | 0 | walked | Retransmission. Describes how Session-Timeout in an Access-Challenge sets the EAP retransmission timer for that one EAP-Request. Indicative prose, no obligation. |
| `2.4` | Fragmentation | 1 | walked | Fragmentation. One obligation, on the RADIUS server, not to exceed a Framed-MTU the NAS supplied. The NAS half is written as 'may be included'. |
| `2.5` | Alternative Uses | 1 | walked | Alternative Uses. RADIUS-encapsulated EAP between a RADIUS server and a security server. One obligation, on the RADIUS server. |
| `2.6` | Usage Guidelines | 0 | walked | Usage Guidelines. A heading over 2.6.1 to 2.6.5 with no body text of its own. |
| `2.6.1` | Identifier Space | 1 | walked | Identifier Space. One obligation, on RADIUS server implementations, to keep the EAP Identifier spaces of distinct sessions apart. |
| `2.6.2` | Role Reversal | 1 | walked | Role Reversal. Says role reversal is not supported, with the obligation on the RADIUS server to reject an Access-Request encapsulating an EAP-Request, and a SHOULD on the same actor about the Nak it includes. |
| `2.6.3` | Conflicting Messages | 3 | walked | Conflicting Messages. Three NAS obligations, all captured, plus the SHOULD list of combinations a RADIUS server is told not to send. |
| `2.6.4` | Priority | 1 | walked | Priority. One NAS obligation about the order in which attributes and the Accept/Reject indication are processed. |
| `2.6.5` | Displayable Messages | 2 | walked | Displayable Messages. The RADIUS server's encapsulation obligation, the exclusion of Reply-Message from any EAP-bearing message, and the NAS SHOULD to discard a Reply-Message rather than translate it. |
| `3` | Attributes | 2 | walked | Attributes. The NAS identification attributes an Access-Request carries, and the RADIUS server's obligation to echo User-Name. |
| `3.1` | EAP-Message | 6 | walked | EAP-Message. The attribute format, the ordering and single-EAP-packet rules, the Message-Authenticator protection rule, and the calculate-and-discard obligations on each of the two roles. |
| `3.2` | Message-Authenticator | 3 | walked | Message-Authenticator. The attribute format and the HMAC-MD5 construction. The construction itself is written without an RFC 2119 keyword, so the site scan does not see it, and it is declared here as an unsourced id: 'MUST calculate the correct value' at sites 3.1:4 and 3.1:6 has no meaning without the formula this section states. |
| `3.3` | Table of Attributes | 1 | walked | Table of Attributes. One obligation in the body, that neither attribute appears in an Accounting-Request. The table rows, its three notes and its legend are the section '0' the site scan derives. |
| `0` | not stated | 3 | walked | The Section 3.3 Table of Attributes itself: the per-packet-type counts, Notes 1 to 3, and the legend that defines the cell values. Note 1 carries two obligations and the legend carries none. |
| `4` | Security Considerations | 0 | walked | Security Considerations. A heading over 4.1 to 4.3 with no body text of its own. |
| `4.1` | Security Requirements | 0 | walked | Security Requirements. The ten-item threat list and the statement that confidentiality, data origin authentication, integrity and replay protection are needed. Written in indicative prose with no keyword. |
| `4.2` | Security Protocol | 4 | walked | Security Protocol. The IPsec profile for RADIUS: the transforms that must be supported, the zero-length shared secret where ESP with a non-null transform carries the traffic, and the IKE mode and identity payload advice. |
| `4.3` | Security Issues | 0 | walked | Security Issues. A heading and the list of the ten vulnerabilities 4.3.1 to 4.3.10 expand. |
| `4.3.1` | Privacy Issues | 0 | walked | Privacy Issues. One SHOULD, to protect RADIUS with IPsec ESP, restating Section 4.2's advice. |
| `4.3.2` | Spoofing and Hijacking | 1 | walked | Spoofing and Hijacking. The SHOULD to use Message-Authenticator in an Access-Request with no User-Password, captured as RFC3579-4.3.2-1, and the restatement of the Section 3.1 protection rule. |
| `4.3.3` | Dictionary Attacks | 0 | walked | Dictionary Attacks. Quotes RFC 2865's shared-secret advice and recommends IPsec ESP. The quoted SHOULD belongs to RFC 2865. |
| `4.3.4` | Known Plaintext Attacks | 1 | walked | Known Plaintext Attacks. Explains the User-Password keystream reuse problem and states one obligation, that an EAP NAS's shared secret is not reused by a NAS using User-Password. |
| `4.3.5` | Replay Attacks | 0 | walked | Replay Attacks. Describes what RADIUS does and does not give for replay protection. Indicative prose, no obligation. |
| `4.3.6` | Negotiation Attacks | 8 | walked | Negotiation Attacks. The downgrade rules: what the authenticating peer must do when EAP was expected, what the NAS must attempt, and what the RADIUS server or proxy must answer. |
| `4.3.7` | Impersonation | 1 | walked | Impersonation. Quotes RFC 2865 Section 3 on choosing the shared secret by source address, and states SHOULDs on RADIUS proxies. Ze is neither the RADIUS server nor a proxy. |
| `4.3.8` | Man in the Middle Attacks | 0 | walked | Man in the Middle Attacks. One SHOULD, that EAP methods incorporate their own per-packet integrity protection. That obligation is on the EAP method, which RFC 3748 and the method's own RFC govern. |
| `4.3.9` | Separation of Authenticator and Authentication Server | 0 | walked | Separation of Authenticator and Authentication Server. Explains what mutual authentication means when the two are on different hosts, and that the MSK must reach the authenticator. The key transport mechanism is stated to be out of scope for this document. |
| `4.3.10` | Multiple Databases | 0 | walked | Multiple Databases. Recommends consolidating the security server's and the RADIUS server's user stores. No obligation, and the actor is the deployment. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. 'This specification does not create any new registries, or define any new RADIUS attributes or values.' |
| `6` | References | 0 | skipped (references) | References. A heading over 6.1 and 6.2. |
| `6.1` | not stated | 0 | skipped (references) | Normative References: RFC 1321, 2104, 2119, 2279, 2284, 2401, 2406, 2409, 2486, 2865, 2988, 3162, 3280 and 3576. |
| `6.2` | not stated | 0 | skipped (references) | Informative References, and the non-RFC citations IEEE802, IEEE8021X, MD5Attack, Masters and NASREQ. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence opens 'As noted in [RFC2865]' and restates RFC 2865 Section 1.1. The obligation belongs to RFC 2865, whose summary carries it as RFC2865-1.1-1. | As noted in [RFC2865], a Network Access Server (NAS) that does not implement a given service MUST NOT implement the RADIUS attributes for that service. |
| `2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The sentence names 'a RADIUS server compliant with this specification and wishing to authenticate with EAP' as the actor. | On receiving a valid Access-Request packet containing EAP-Message attribute(s), a RADIUS server compliant with this specification and wishing to authenticate with EAP MUST respond with an Access-Challenge packet containing EAP-Message attribute(s). |
| `2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is the RADIUS server that does not support EAP. | If the RADIUS server does not support EAP or does not wish to authenticate with EAP, it MUST respond with an Access-Reject. |
| `2.1:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'the local RADIUS server' deciding a realm from the peer identity. | If the realm is determined based on the peer identity, the local RADIUS server MUST respond with a RADIUS Access-Challenge including an EAP-Message attribute encapsulating an EAP-Request/Identity packet. |
| `2.1:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The Access-Reject is sent by the RADIUS server that does not support the EAP-Message attribute. The NAS half of the same paragraph, denying access on that reject, is site 2.1:8. | If an Access-Request is sent to a RADIUS server which does not support the EAP-Message attribute, then an Access-Reject MUST be sent in response. |
| `2.1:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates site 2.1:3, 'Reception of a RADIUS Access-Reject packet MUST result in the NAS denying access to the authenticating peer', for the particular reject an EAP-incapable server sends. It adds no obligation RFC3579-2.1-1 does not already carry. | On receiving an Access-Reject, the NAS MUST deny access to the authenticating peer. |
| `2.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'A RADIUS server determining that a fatal error has occurred'. | A RADIUS server determining that a fatal error has occurred MUST send an Access-Reject containing an EAP-Message attribute encapsulating EAP-Failure. |
| `2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'A RADIUS server having received a Framed-MTU attribute'. The NAS half of Section 2.4, supplying Framed-MTU, is written as 'may be included' and states no obligation. | A RADIUS server having received a Framed-MTU attribute in an Access-Request packet MUST NOT send any subsequent packet in this EAP conversation containing EAP-Message attributes whose values, when concatenated, exceed the length specified by the Framed-MTU value, taking the link type (specified by the NAS-Port-Type attribute) into account. |
| `2.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. Section 2.5 describes a RADIUS server talking to a security server, and the actor is the RADIUS server adding the missing Access-Accept attributes. | This means that the RADIUS server MUST add these attributes prior to sending an Access-Accept message to the NAS. |
| `2.6.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'RADIUS server implementations' keeping EAP Identifier spaces apart per session. | RADIUS server implementations MUST be able to distinguish between EAP packets with the same Identifier existing within distinct sessions, originating on the same NAS. |
| `2.6.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is the RADIUS server refusing a role-reversed Access-Request that encapsulates an EAP-Request. | A RADIUS server MUST respond to an Access-Request encapsulating an EAP-Request with an Access-Reject. |
| `2.6.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is the RADIUS server sending a displayable message to a NAS. | When sending a displayable message to a NAS during an EAP conversation, the RADIUS server MUST encapsulate displayable messages within EAP-Message/EAP-Request/Notification attribute(s). |
| `3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is the RADIUS server echoing User-Name into its Access-Accept. | In order to permit forwarding of the Access-Reply by EAP-unaware proxies, if a User-Name attribute was included in an Access-Request, the RADIUS server MUST include the User-Name attribute in subsequent Access-Accept packets. |
| `3.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'A RADIUS server supporting the EAP-Message attribute'. The NAS form of the same check is site 3.1:6. | A RADIUS server supporting the EAP-Message attribute MUST calculate the correct value of the Message-Authenticator and MUST silently discard the packet if it does not match the value sent. |
| `3.1:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'A RADIUS server not supporting the EAP-Message attribute'. | A RADIUS server not supporting the EAP-Message attribute MUST return an Access-Reject if it receives an Access-Request containing an EAP-Message attribute. |
| `3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates site 3.1:3 from the Message-Authenticator side: the attribute is mandatory in any of the four packet types that includes an EAP-Message. It adds no obligation RFC3579-3.1-3 does not already carry. | It MUST be used in any Access-Request, Access-Accept, Access-Reject or Access-Challenge that includes an EAP-Message attribute. |
| `3.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'A RADIUS server receiving an Access-Request with a Message-Authenticator attribute present'. | A RADIUS server receiving an Access-Request with a Message-Authenticator attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `3.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates site 3.1:6 for a RADIUS client rather than for a NAS supporting EAP-Message: recompute the Message-Authenticator of a received Access-Accept, Access-Reject or Access-Challenge and silently discard on a mismatch. One behaviour, one code path in ze (verifyResponseMessageAuthenticator), so it maps to the same requirement. | A RADIUS client receiving an Access-Accept, Access-Reject or Access-Challenge with a Message-Authenticator attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `0:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Note 1 restates site 3.1:3, 'If any packet type contains an EAP-Message attribute it MUST also contain a Message-Authenticator'. It adds no obligation RFC3579-3.1-3 does not already carry. | If any packet type contains an EAP-Message attribute it MUST also contain a Message-Authenticator. |
| `0:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The legend of the Section 3.3 table, defining what the cells 0, 0+, 0-1, 1 and 1+ mean. The keywords describe the notation, not a speaker's behaviour; the obligations the table expresses are carried by Note 1 and by the attribute sections. | 0 This attribute MUST NOT be present. 0+ Zero or more instances of this attribute MAY be present. 0-1 Zero or one instance of this attribute MAY be present. 1 Exactly one instance of this attribute MUST be present. 1+ One or more of these attributes MUST be present. |
| `4.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'a RADIUS server that cannot know whether incoming traffic is IPsec-protected'. The client-side half of the same paragraph is site 4.2:1. | However, a RADIUS server that cannot know whether incoming traffic is IPsec-protected MUST be configured with a non-null RADIUS shared secret. |
| `4.3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Security Issues restatement of site 3.1:3, 'the Message-Authenticator attribute MUST be used in all RADIUS packets containing an EAP-Message attribute'. It adds no obligation RFC3579-3.1-3 does not already carry. | To provide stronger security, the Message-Authenticator attribute MUST be used in all RADIUS packets containing an EAP-Message attribute. |
| `4.3.6:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is the RADIUS server answering a CHAP Access-Request where EAP is required. | However, if CHAP has been negotiated but EAP is required, the RADIUS server MUST respond with an Access-Reject, rather than an Access-Challenge/EAP-Message/EAP-Request packet. |
| `4.3.6:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS authentication server. Ze never acts as one: NewClient (internal/component/radius/client.go) binds an ephemeral UDP socket with no service port, and (*Client).readLoop dispatches a datagram only against a waiter that (*Client).Exchange registered for one of ze's own outstanding requests, so no Access-Request receive path exists in the tree. If ze ran a RADIUS server the listener would live beside that producer. The actor is 'the server or proxy' that does not support EAP. Ze is neither a RADIUS server nor a RADIUS proxy: it forwards no other party's RADIUS packet. | If EAP is negotiated but is not supported by the RADIUS proxy or server, then the server or proxy MUST respond with an Access-Reject. |
| `4.3.6:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates site 4.3.6:5 one paragraph later, for an EAP-capable peer rather than for the peer generally. It adds no obligation RFC3579-4.3.6-4 does not already carry. | An EAP-capable authenticating peer MUST refuse to renegotiate the authentication protocol if EAP had initially been negotiated. |
| `4.3.7:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence is the body of a block quotation introduced by '[RFC2865] Section 3 states:'. The obligation belongs to RFC 2865, whose summary is rfc/short/rfc2865.md, and it binds the RADIUS server in any case. | A RADIUS server MUST use the source IP address of the RADIUS UDP packet to decide which shared secret to use, so that RADIUS requests can be proxied. |

## Superseded

No document obsoletes RFC 3579, so its obligations are stated where they were written.
