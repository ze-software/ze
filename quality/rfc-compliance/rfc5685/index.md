# RFC 5685 - Redirect Mechanism for the Internet Key Exchange Protocol Version 2 (IKEv2)

Future. Every requirement this repository extracted from RFC 5685, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 26 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 26 | of 35 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 26 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 26 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 35 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 26 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5685.md` |
| Requirement shard | `rfc/requirements/rfc5685.md` |
| RFC text | `rfc/full/rfc5685.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-concentrator profile. The extraction is COMPLETE: the source text is at rfc/full/rfc5685.txt and the extraction sign-off is rfc/extraction/rfc5685.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: all 26 MUST-level requirements are classified, and all 26 are `{gap}`, because Ze's IKE engine implements neither the gateway nor the client side of redirect. `internal/component/ike/wire/payload_notify.go` declares none of REDIRECT_SUPPORTED (16406), REDIRECT (16407) or REDIRECTED_FROM (16408). Three sites are excluded in the extraction sign-off: two bind a Mobile IPv6 mobile node, which Ze does not implement, and one binds the author of a future specification.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None: Ze neither sends nor recognizes REDIRECT_SUPPORTED, REDIRECT or REDIRECTED_FROM.

**What the ledger says remains**

Gateway redirect in IKE_SA_INIT, IKE_AUTH and INFORMATIONAL, anycast redirect, client nonce and AUTH verification of a REDIRECT, REDIRECTED_FROM on the new gateway, and redirect loop limits are absent. Several prohibitions (sending REDIRECT to a client that did not offer support, altering PAD or SPD state on a redirect) hold today only because no redirect path exists, and no test proves them. All 26 MUST-level requirements are annotated `{gap}` in [`rfc/short/rfc5685.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5685.md). Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 26 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (26):** [`RFC5685-3-1`](#rfc5685-3-1), [`RFC5685-3-2`](#rfc5685-3-2), [`RFC5685-3-3`](#rfc5685-3-3), [`RFC5685-3-4`](#rfc5685-3-4), [`RFC5685-3-5`](#rfc5685-3-5), [`RFC5685-3-6`](#rfc5685-3-6), [`RFC5685-3-7`](#rfc5685-3-7), [`RFC5685-4-1`](#rfc5685-4-1), [`RFC5685-5-1`](#rfc5685-5-1), [`RFC5685-5-2`](#rfc5685-5-2), [`RFC5685-6-1`](#rfc5685-6-1), [`RFC5685-6-2`](#rfc5685-6-2), [`RFC5685-6-3`](#rfc5685-6-3), [`RFC5685-6-4`](#rfc5685-6-4), [`RFC5685-6-5`](#rfc5685-6-5), [`RFC5685-6-6`](#rfc5685-6-6), [`RFC5685-9.1-1`](#rfc5685-9.1-1), [`RFC5685-9.1-2`](#rfc5685-9.1-2), [`RFC5685-9.2-1`](#rfc5685-9.2-1), [`RFC5685-9.2-2`](#rfc5685-9.2-2), [`RFC5685-9.2-3`](#rfc5685-9.2-3), [`RFC5685-9.2-4`](#rfc5685-9.2-4), [`RFC5685-9.2-5`](#rfc5685-9.2-5), [`RFC5685-9.3-1`](#rfc5685-9.3-1), [`RFC5685-9.3-2`](#rfc5685-9.3-2), [`RFC5685-11-1`](#rfc5685-11-1)

## Attainable ceiling

Mandatory ceiling: 26 of 26 (100.0%). Optional ceiling: 9 of 9 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc5685 26 of 26 (100.0%). Lowest optional ceiling: rfc5685 9 of 9 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5685-3-1` | The gateway MUST keep track of those clients that indicated support for the redirect mechanism and those that didn't. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not read REDIRECT_SUPPORTED, so it records no client's redirect support; internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16406; internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-2` | If the IKE_SA_INIT request did not indicate support for the redirect mechanism, the responder MUST NOT send the REDIRECT payload to the VPN client. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze has no REDIRECT encoder (internal/component/ike/wire/payload_notify.go declares no 16407), so it sends REDIRECT to no client, but no test proves the prohibition; internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-3` | When the client receives the IKE_SA_INIT response, it MUST verify that the nonce data matches the value sent in the IKE_SA_INIT request. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze initiator does not recognize REDIRECT and verifies no nonce data in one; internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-4` | If the values do not match, the client MUST silently discard the response (and keep waiting for another response). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze initiator has no REDIRECT nonce check whose failure could discard a response; internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-5` | In the IKE_SA_INIT exchange with the new VPN gateway, the client MUST include the REDIRECTED_FROM payload (see Section 9.3). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never follows a redirect and declares no REDIRECTED_FROM (16408); internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-6` | When the client gets redirected, it MUST use the same Peer Authorization Database (PAD) and Security Policy Database (SPD) entries as it would have used with the original gateway. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze initiator never connects to a redirected gateway, so no PAD or SPD reuse across a redirect exists; internal/component/ike/wire/payload_notify.go |
| `RFC5685-3-7` | Receiving a redirect notification MUST NOT result in the modification of any PAD or SPD entries. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze does not recognize REDIRECT (no 16407 in internal/component/ike/wire/payload_notify.go), so receiving one changes no PAD or SPD entry, but no test proves it; internal/component/ike/wire/payload_notify.go |
| `RFC5685-4-1` | If the destination address on the IKE_SA_INIT request is an anycast address, the VPN gateway that received the IKE_SA_INIT request MUST include the REDIRECT payload to redirect the VPN client to a unicast address of one of the VPN gateways. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no REDIRECT, so an IKE_SA_INIT reaching a Ze responder on an anycast address is answered without one; internal/component/ike/wire/payload_notify.go |
| `RFC5685-5-1` | The REDIRECT payload MUST carry information about the new VPN gateway. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no REDIRECT encoder; internal/component/ike/wire/payload_notify.go |
| `RFC5685-5-2` | The gateway MUST NOT include any nonce data in the REDIRECT payload, since it is a gateway- initiated redirect and is protected by the IKEv2 security association. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no gateway-initiated REDIRECT in INFORMATIONAL; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-1` | The gateway MUST verify the client's AUTH payload before sending the REDIRECT payload, and the client MUST verify the gateway's AUTH payload before acting on the REDIRECT payload. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze neither sends nor acts on a REDIRECT in IKE_AUTH; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-2` | In this case, the gateway MUST send the REDIRECT Notify payload in either the first or the last IKE_AUTH response. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no REDIRECT in any IKE_AUTH response; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-3` | The client and the gateway MUST verify the AUTH payloads as described above. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the AUTH verification this sentence orders is the one that gates a REDIRECT in IKE_AUTH, and Ze has no such REDIRECT path; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-4` | Since EAP is used as the authentication mechanism, the client does not include AUTH payload to authenticate its identity, but the server MUST still include its own AUTH payload, and the client MUST verify it. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence governs a REDIRECT sent during EAP IKE_AUTH, and Ze has no such REDIRECT path; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-5` | In all of the cases above, the client MUST accept the REDIRECT notification only in the first IKE_AUTH response or the last IKE_AUTH response. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze initiator accepts REDIRECT in no IKE_AUTH response, because it does not recognize the notification; internal/component/ike/wire/payload_notify.go |
| `RFC5685-6-6` | It MUST NOT accept the REDIRECT notification in an intermediate IKE_AUTH response. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. The Ze initiator does not recognize REDIRECT, so it acts on none in an intermediate IKE_AUTH response, but no test proves it; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.1-1` | The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECT_SUPPORTED notification; internal/component/ike/wire/payload_notify.go declares no 16406; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.1-2` | The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECT_SUPPORTED notification; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.2-1` | The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECT notification; internal/component/ike/wire/payload_notify.go declares no 16407; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.2-2` | The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECT notification; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.2-3` | The IPv4 address, the IPv6 address, or the FQDN of the new VPN gateway MUST be encoded as described in Section 3.5 of [2]. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes no New Responder GW Identity; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.2-4` | The size of the nonce MUST be between 16 and 256 bytes, as described in Section 3.9 of [2]. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes no REDIRECT Nonce Data; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.2-5` | It MUST NOT be included in the REDIRECT payload if sent in an IKE_AUTH response or in a gateway-initiated redirect message. (§9.2) | MUST NOT | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no REDIRECT in IKE_AUTH or INFORMATIONAL; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.3-1` | The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECTED_FROM notification; internal/component/ike/wire/payload_notify.go declares no 16408; internal/component/ike/wire/payload_notify.go |
| `RFC5685-9.3-2` | The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no REDIRECTED_FROM notification; internal/component/ike/wire/payload_notify.go |
| `RFC5685-11-1` | The redirect mechanism MUST NOT update any state on the client apart from the VPN gateway information. (§11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze has no redirect mechanism, so no client state is updated by one, but no test proves it; internal/component/ike/wire/payload_notify.go |
| `RFC5685-5-3` | Once the client sends an acknowledgement to the gateway, it SHOULD delete the existing security associations with the old gateway by sending an INFORMATIONAL message with a DELETE payload. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-6-7` | When the client receives the IKE_AUTH response with the REDIRECT payload, it SHOULD delete the IKEv2 security association with the gateway by sending an INFORMATIONAL message with a DELETE payload. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-7-1` | To prevent this, the client SHOULD be configured to not accept more than a certain number of redirects (MAX_REDIRECTS) within a short time period (REDIRECT_LOOP_DETECT_PERIOD) for a particular IKEv2 SA setup. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-3-8` | The client MAY get redirected again by the new VPN gateway if the new VPN gateway also cannot serve the client. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-4-2` | The VPN gateway that received the IKE_SA_INIT request MAY redirect the client to its own unicast address if it is not overloaded. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-5-4` | The gateway MAY also decide to delete the security associations without any signaling from the client, again by sending an INFORMATIONAL message with a DELETE payload (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-6-8` | When EAP is used, the gateway MAY also redirect the client based on the unauthenticated identity presented by the client in the first IKE_AUTH exchange, itself. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-8-1` | The MN MAY treat the information received in the IKE_SA_INIT response in a similar way as it would treat HA discovery information received from other unauthenticated (and potentially untrustworthy) sources (such as DNS lookups not protected with DNS Security (DNSSEC)). (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5685-8-2` | If the REDIRECT notification is received during the IKE_AUTH exchange (after the HA has been authenticated; see Section 6), the MN MAY pass the new address to Mobile IPv6 and treat it in a similar fashion as information from the Home Agent Switch message [5]. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5685-3-1`](#rfc5685-3-1) The gateway MUST keep track of those clients that indicated support for the redirect mechanism and those that didn't. (§3) | {gap}, no test | Ze does not read REDIRECT_SUPPORTED, so it records no client's redirect support; internal/component/ike/wire/payload_notify.go declares no Notify Message Type 16406; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-2`](#rfc5685-3-2) If the IKE_SA_INIT request did not indicate support for the redirect mechanism, the responder MUST NOT send the REDIRECT payload to the VPN client. (§3) | {gap}, no test | unproven rather than known divergent. Ze has no REDIRECT encoder (internal/component/ike/wire/payload_notify.go declares no 16407), so it sends REDIRECT to no client, but no test proves the prohibition; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-3`](#rfc5685-3-3) When the client receives the IKE_SA_INIT response, it MUST verify that the nonce data matches the value sent in the IKE_SA_INIT request. (§3) | {gap}, no test | the Ze initiator does not recognize REDIRECT and verifies no nonce data in one; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-4`](#rfc5685-3-4) If the values do not match, the client MUST silently discard the response (and keep waiting for another response). (§3) | {gap}, no test | the Ze initiator has no REDIRECT nonce check whose failure could discard a response; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-5`](#rfc5685-3-5) In the IKE_SA_INIT exchange with the new VPN gateway, the client MUST include the REDIRECTED_FROM payload (see Section 9.3). (§3) | {gap}, no test | Ze never follows a redirect and declares no REDIRECTED_FROM (16408); internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-6`](#rfc5685-3-6) When the client gets redirected, it MUST use the same Peer Authorization Database (PAD) and Security Policy Database (SPD) entries as it would have used with the original gateway. (§3) | {gap}, no test | the Ze initiator never connects to a redirected gateway, so no PAD or SPD reuse across a redirect exists; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-3-7`](#rfc5685-3-7) Receiving a redirect notification MUST NOT result in the modification of any PAD or SPD entries. (§3) | {gap}, no test | unproven rather than known divergent. Ze does not recognize REDIRECT (no 16407 in internal/component/ike/wire/payload_notify.go), so receiving one changes no PAD or SPD entry, but no test proves it; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-4-1`](#rfc5685-4-1) If the destination address on the IKE_SA_INIT request is an anycast address, the VPN gateway that received the IKE_SA_INIT request MUST include the REDIRECT payload to redirect the VPN client to a unicast address of one of the VPN gateways. (§4) | {gap}, no test | Ze sends no REDIRECT, so an IKE_SA_INIT reaching a Ze responder on an anycast address is answered without one; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-5-1`](#rfc5685-5-1) The REDIRECT payload MUST carry information about the new VPN gateway. (§5) | {gap}, no test | Ze has no REDIRECT encoder; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-5-2`](#rfc5685-5-2) The gateway MUST NOT include any nonce data in the REDIRECT payload, since it is a gateway- initiated redirect and is protected by the IKEv2 security association. (§5) | {gap}, no test | Ze has no gateway-initiated REDIRECT in INFORMATIONAL; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-1`](#rfc5685-6-1) The gateway MUST verify the client's AUTH payload before sending the REDIRECT payload, and the client MUST verify the gateway's AUTH payload before acting on the REDIRECT payload. (§6) | {gap}, no test | Ze neither sends nor acts on a REDIRECT in IKE_AUTH; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-2`](#rfc5685-6-2) In this case, the gateway MUST send the REDIRECT Notify payload in either the first or the last IKE_AUTH response. (§6) | {gap}, no test | Ze sends no REDIRECT in any IKE_AUTH response; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-3`](#rfc5685-6-3) The client and the gateway MUST verify the AUTH payloads as described above. (§6) | {gap}, no test | the AUTH verification this sentence orders is the one that gates a REDIRECT in IKE_AUTH, and Ze has no such REDIRECT path; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-4`](#rfc5685-6-4) Since EAP is used as the authentication mechanism, the client does not include AUTH payload to authenticate its identity, but the server MUST still include its own AUTH payload, and the client MUST verify it. (§6) | {gap}, no test | the sentence governs a REDIRECT sent during EAP IKE_AUTH, and Ze has no such REDIRECT path; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-5`](#rfc5685-6-5) In all of the cases above, the client MUST accept the REDIRECT notification only in the first IKE_AUTH response or the last IKE_AUTH response. (§6) | {gap}, no test | the Ze initiator accepts REDIRECT in no IKE_AUTH response, because it does not recognize the notification; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-6-6`](#rfc5685-6-6) It MUST NOT accept the REDIRECT notification in an intermediate IKE_AUTH response. (§6) | {gap}, no test | unproven rather than known divergent. The Ze initiator does not recognize REDIRECT, so it acts on none in an intermediate IKE_AUTH response, but no test proves it; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.1-1`](#rfc5685-9.1-1) The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.1) | {gap}, no test | Ze builds no REDIRECT_SUPPORTED notification; internal/component/ike/wire/payload_notify.go declares no 16406; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.1-2`](#rfc5685-9.1-2) The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.1) | {gap}, no test | Ze builds no REDIRECT_SUPPORTED notification; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.2-1`](#rfc5685-9.2-1) The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.2) | {gap}, no test | Ze builds no REDIRECT notification; internal/component/ike/wire/payload_notify.go declares no 16407; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.2-2`](#rfc5685-9.2-2) The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.2) | {gap}, no test | Ze builds no REDIRECT notification; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.2-3`](#rfc5685-9.2-3) The IPv4 address, the IPv6 address, or the FQDN of the new VPN gateway MUST be encoded as described in Section 3.5 of [2]. (§9.2) | {gap}, no test | Ze encodes no New Responder GW Identity; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.2-4`](#rfc5685-9.2-4) The size of the nonce MUST be between 16 and 256 bytes, as described in Section 3.9 of [2]. (§9.2) | {gap}, no test | Ze encodes no REDIRECT Nonce Data; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.2-5`](#rfc5685-9.2-5) It MUST NOT be included in the REDIRECT payload if sent in an IKE_AUTH response or in a gateway-initiated redirect message. (§9.2) | {gap}, no test | Ze sends no REDIRECT in IKE_AUTH or INFORMATIONAL; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.3-1`](#rfc5685-9.3-1) The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.3) | {gap}, no test | Ze builds no REDIRECTED_FROM notification; internal/component/ike/wire/payload_notify.go declares no 16408; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-9.3-2`](#rfc5685-9.3-2) The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.3) | {gap}, no test | Ze builds no REDIRECTED_FROM notification; internal/component/ike/wire/payload_notify.go |
| [`RFC5685-11-1`](#rfc5685-11-1) The redirect mechanism MUST NOT update any state on the client apart from the VPN gateway information. (§11) | {gap}, no test | unproven rather than known divergent. Ze has no redirect mechanism, so no client state is updated by one, but no test proves it; internal/component/ike/wire/payload_notify.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5685-3-1`](#rfc5685-3-1)

The gateway MUST keep track of those clients that indicated support for the redirect mechanism and those that didn't. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-1, so no unit is bound to it.

### [`RFC5685-3-2`](#rfc5685-3-2)

If the IKE_SA_INIT request did not indicate support for the redirect mechanism, the responder MUST NOT send the REDIRECT payload to the VPN client. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-2, so no unit is bound to it.

### [`RFC5685-3-3`](#rfc5685-3-3)

When the client receives the IKE_SA_INIT response, it MUST verify that the nonce data matches the value sent in the IKE_SA_INIT request. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-3, so no unit is bound to it.

### [`RFC5685-3-4`](#rfc5685-3-4)

If the values do not match, the client MUST silently discard the response (and keep waiting for another response). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-4, so no unit is bound to it.

### [`RFC5685-3-5`](#rfc5685-3-5)

In the IKE_SA_INIT exchange with the new VPN gateway, the client MUST include the REDIRECTED_FROM payload (see Section 9.3). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-5, so no unit is bound to it.

### [`RFC5685-3-6`](#rfc5685-3-6)

When the client gets redirected, it MUST use the same Peer Authorization Database (PAD) and Security Policy Database (SPD) entries as it would have used with the original gateway. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-6, so no unit is bound to it.

### [`RFC5685-3-7`](#rfc5685-3-7)

Receiving a redirect notification MUST NOT result in the modification of any PAD or SPD entries. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-3-7, so no unit is bound to it.

### [`RFC5685-4-1`](#rfc5685-4-1)

If the destination address on the IKE_SA_INIT request is an anycast address, the VPN gateway that received the IKE_SA_INIT request MUST include the REDIRECT payload to redirect the VPN client to a unicast address of one of the VPN gateways. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-4-1, so no unit is bound to it.

### [`RFC5685-5-1`](#rfc5685-5-1)

The REDIRECT payload MUST carry information about the new VPN gateway. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-5-1, so no unit is bound to it.

### [`RFC5685-5-2`](#rfc5685-5-2)

The gateway MUST NOT include any nonce data in the REDIRECT payload, since it is a gateway- initiated redirect and is protected by the IKEv2 security association. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-5-2, so no unit is bound to it.

### [`RFC5685-6-1`](#rfc5685-6-1)

The gateway MUST verify the client's AUTH payload before sending the REDIRECT payload, and the client MUST verify the gateway's AUTH payload before acting on the REDIRECT payload. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-1, so no unit is bound to it.

### [`RFC5685-6-2`](#rfc5685-6-2)

In this case, the gateway MUST send the REDIRECT Notify payload in either the first or the last IKE_AUTH response. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-2, so no unit is bound to it.

### [`RFC5685-6-3`](#rfc5685-6-3)

The client and the gateway MUST verify the AUTH payloads as described above. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-3, so no unit is bound to it.

### [`RFC5685-6-4`](#rfc5685-6-4)

Since EAP is used as the authentication mechanism, the client does not include AUTH payload to authenticate its identity, but the server MUST still include its own AUTH payload, and the client MUST verify it. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-4, so no unit is bound to it.

### [`RFC5685-6-5`](#rfc5685-6-5)

In all of the cases above, the client MUST accept the REDIRECT notification only in the first IKE_AUTH response or the last IKE_AUTH response. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-5, so no unit is bound to it.

### [`RFC5685-6-6`](#rfc5685-6-6)

It MUST NOT accept the REDIRECT notification in an intermediate IKE_AUTH response. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-6-6, so no unit is bound to it.

### [`RFC5685-9.1-1`](#rfc5685-9.1-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.1-1, so no unit is bound to it.

### [`RFC5685-9.1-2`](#rfc5685-9.1-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.1-2, so no unit is bound to it.

### [`RFC5685-9.2-1`](#rfc5685-9.2-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.2-1, so no unit is bound to it.

### [`RFC5685-9.2-2`](#rfc5685-9.2-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.2-2, so no unit is bound to it.

### [`RFC5685-9.2-3`](#rfc5685-9.2-3)

The IPv4 address, the IPv6 address, or the FQDN of the new VPN gateway MUST be encoded as described in Section 3.5 of [2]. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.2-3, so no unit is bound to it.

### [`RFC5685-9.2-4`](#rfc5685-9.2-4)

The size of the nonce MUST be between 16 and 256 bytes, as described in Section 3.9 of [2]. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.2-4, so no unit is bound to it.

### [`RFC5685-9.2-5`](#rfc5685-9.2-5)

It MUST NOT be included in the REDIRECT payload if sent in an IKE_AUTH response or in a gateway-initiated redirect message. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.2-5, so no unit is bound to it.

### [`RFC5685-9.3-1`](#rfc5685-9.3-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.3-1, so no unit is bound to it.

### [`RFC5685-9.3-2`](#rfc5685-9.3-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-9.3-2, so no unit is bound to it.

### [`RFC5685-11-1`](#rfc5685-11-1)

The redirect mechanism MUST NOT update any state on the client apart from the VPN gateway information. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5685-11-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5685.txt |
| Source fingerprint | ff73460a83920d9b |
| Record | rfc/extraction/rfc5685.json |
| Mapped sentences | 26 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 7 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 6 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 2 | walked | not stated |
| `9.2` | not stated | 5 | walked | not stated |
| `9.3` | not stated | 2 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 1 | walked | not stated |
| `12` | not stated | 1 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 0 | walked | not stated |
| `14.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Mobile IPv6 mobile node (MN) using IKEv2 with its home agent (HA), the Section 8 role. Ze implements no Mobile IPv6: no Go file under internal/ names Mobile IPv6, MIPv6 or a home agent, and the IKE engine in internal/component/ike/ keeps no Mobile IPv6 binding state, so neither Ze nor a layer acting for it fills the MN role. The generic redirect obligations Ze does owe are the Section 3, 6 and 11 rows. | However, if the MN has authenticated information about its home agent, it MUST NOT be updated based on the IKE_SA_INIT response. |
| `8:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Mobile IPv6 mobile node (MN) using IKEv2 with its home agent (HA), the Section 8 role. Ze implements no Mobile IPv6: no Go file under internal/ names Mobile IPv6, MIPv6 or a home agent, and the IKE engine in internal/component/ike/ keeps no Mobile IPv6 binding state, so neither Ze nor a layer acting for it fills the MN role. The generic redirect obligations Ze does owe are the Section 3, 6 and 11 rows. | Gateway-initiated REDIRECT notifications exchanged in INFORMATIONAL exchanges (see Section 5) MUST NOT result in updating any Mobile IPv6 state. |
| `12:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of a future specification that extends the GW Identity Type registry, not an IKEv2 implementation: it says what such a document must state. Ze writes no IETF specification, so no implementation role carries it. | A specification that extends this registry MUST also mention which of the new values are valid in which Notify payload. |

## Superseded

No document obsoletes RFC 5685, so its obligations are stated where they were written.
