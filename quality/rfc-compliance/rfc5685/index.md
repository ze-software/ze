# RFC 5685 - Redirect Mechanism for the Internet Key Exchange Protocol Version 2 (IKEv2)

IKEv2 redirect. Every requirement listed for RFC 5685, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 26 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 26 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 26 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 26 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 26 | of 26 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 26 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 26 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 26 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 26 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 26 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 26 | of 35 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 26 mandatory requirements: they add up to 26. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IKEv2 redirect |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 35 |
| Mandatory requirements (MUST level), not checked | 26 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 26 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5685.md` |
| Requirement table | `rfc/requirements/rfc5685.md` |
| RFC text | `rfc/full/rfc5685.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-concentrator profile. The extraction is COMPLETE: the source text is at rfc/full/rfc5685.txt and the extraction sign-off is rfc/extraction/rfc5685.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: all 26 MUST-level requirements are classified, and all 26 are `{gap}`, because Ze's IKE engine implements neither the gateway nor the client side of redirect. `internal/component/ike/wire/payload_notify.go` declares none of REDIRECT_SUPPORTED (16406), REDIRECT (16407) or REDIRECTED_FROM (16408). Three sites are excluded in the extraction sign-off: two bind a Mobile IPv6 mobile node, which Ze does not implement, and one binds the author of a future specification.

## Published support status

**Area:** IKEv2 redirect

**Proof:** 0 of 26 MUST proven

**Evidence:** -

**Unmet:** RFC5685-11-1, RFC5685-3-1, RFC5685-3-2, RFC5685-3-3, RFC5685-3-4, RFC5685-3-5, RFC5685-3-6, RFC5685-3-7, RFC5685-4-1, RFC5685-5-1, RFC5685-5-2, RFC5685-6-1, RFC5685-6-2, RFC5685-6-3, RFC5685-6-4, RFC5685-6-5, RFC5685-6-6, RFC5685-9.1-1, RFC5685-9.1-2, RFC5685-9.2-1, RFC5685-9.2-2, RFC5685-9.2-3, RFC5685-9.2-4, RFC5685-9.2-5, RFC5685-9.3-1, RFC5685-9.3-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 26 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **26** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (26):** [`RFC5685-3-1`](#rfc5685-3-1), [`RFC5685-3-2`](#rfc5685-3-2), [`RFC5685-3-3`](#rfc5685-3-3), [`RFC5685-3-4`](#rfc5685-3-4), [`RFC5685-3-5`](#rfc5685-3-5), [`RFC5685-3-6`](#rfc5685-3-6), [`RFC5685-3-7`](#rfc5685-3-7), [`RFC5685-4-1`](#rfc5685-4-1), [`RFC5685-5-1`](#rfc5685-5-1), [`RFC5685-5-2`](#rfc5685-5-2), [`RFC5685-6-1`](#rfc5685-6-1), [`RFC5685-6-2`](#rfc5685-6-2), [`RFC5685-6-3`](#rfc5685-6-3), [`RFC5685-6-4`](#rfc5685-6-4), [`RFC5685-6-5`](#rfc5685-6-5), [`RFC5685-6-6`](#rfc5685-6-6), [`RFC5685-9.1-1`](#rfc5685-9.1-1), [`RFC5685-9.1-2`](#rfc5685-9.1-2), [`RFC5685-9.2-1`](#rfc5685-9.2-1), [`RFC5685-9.2-2`](#rfc5685-9.2-2), [`RFC5685-9.2-3`](#rfc5685-9.2-3), [`RFC5685-9.2-4`](#rfc5685-9.2-4), [`RFC5685-9.2-5`](#rfc5685-9.2-5), [`RFC5685-9.3-1`](#rfc5685-9.3-1), [`RFC5685-9.3-2`](#rfc5685-9.3-2), [`RFC5685-11-1`](#rfc5685-11-1)

## Figures

- **MUST bar**: Not built 26
- **Overview bar**: Not assessed 9, Not built 26

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 26 |
| MUST rows kept (the MUST bar) | 26 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 26 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 9 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 9 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5685.json](../../../data/rfc/rfc5685.json).

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

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC5685-3-1`](#rfc5685-3-1)

The gateway MUST keep track of those clients that indicated support for the redirect mechanism and those that didn't. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-1, so there is no linked test.

### [`RFC5685-3-2`](#rfc5685-3-2)

If the IKE_SA_INIT request did not indicate support for the redirect mechanism, the responder MUST NOT send the REDIRECT payload to the VPN client. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-2, so there is no linked test.

### [`RFC5685-3-3`](#rfc5685-3-3)

When the client receives the IKE_SA_INIT response, it MUST verify that the nonce data matches the value sent in the IKE_SA_INIT request. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-3, so there is no linked test.

### [`RFC5685-3-4`](#rfc5685-3-4)

If the values do not match, the client MUST silently discard the response (and keep waiting for another response). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-4, so there is no linked test.

### [`RFC5685-3-5`](#rfc5685-3-5)

In the IKE_SA_INIT exchange with the new VPN gateway, the client MUST include the REDIRECTED_FROM payload (see Section 9.3). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-5, so there is no linked test.

### [`RFC5685-3-6`](#rfc5685-3-6)

When the client gets redirected, it MUST use the same Peer Authorization Database (PAD) and Security Policy Database (SPD) entries as it would have used with the original gateway. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-6, so there is no linked test.

### [`RFC5685-3-7`](#rfc5685-3-7)

Receiving a redirect notification MUST NOT result in the modification of any PAD or SPD entries. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-3-7, so there is no linked test.

### [`RFC5685-4-1`](#rfc5685-4-1)

If the destination address on the IKE_SA_INIT request is an anycast address, the VPN gateway that received the IKE_SA_INIT request MUST include the REDIRECT payload to redirect the VPN client to a unicast address of one of the VPN gateways. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-4-1, so there is no linked test.

### [`RFC5685-5-1`](#rfc5685-5-1)

The REDIRECT payload MUST carry information about the new VPN gateway. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-5-1, so there is no linked test.

### [`RFC5685-5-2`](#rfc5685-5-2)

The gateway MUST NOT include any nonce data in the REDIRECT payload, since it is a gateway- initiated redirect and is protected by the IKEv2 security association. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-5-2, so there is no linked test.

### [`RFC5685-6-1`](#rfc5685-6-1)

The gateway MUST verify the client's AUTH payload before sending the REDIRECT payload, and the client MUST verify the gateway's AUTH payload before acting on the REDIRECT payload. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-1, so there is no linked test.

### [`RFC5685-6-2`](#rfc5685-6-2)

In this case, the gateway MUST send the REDIRECT Notify payload in either the first or the last IKE_AUTH response. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-2, so there is no linked test.

### [`RFC5685-6-3`](#rfc5685-6-3)

The client and the gateway MUST verify the AUTH payloads as described above. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-3, so there is no linked test.

### [`RFC5685-6-4`](#rfc5685-6-4)

Since EAP is used as the authentication mechanism, the client does not include AUTH payload to authenticate its identity, but the server MUST still include its own AUTH payload, and the client MUST verify it. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-4, so there is no linked test.

### [`RFC5685-6-5`](#rfc5685-6-5)

In all of the cases above, the client MUST accept the REDIRECT notification only in the first IKE_AUTH response or the last IKE_AUTH response. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-5, so there is no linked test.

### [`RFC5685-6-6`](#rfc5685-6-6)

It MUST NOT accept the REDIRECT notification in an intermediate IKE_AUTH response. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-6-6, so there is no linked test.

### [`RFC5685-9.1-1`](#rfc5685-9.1-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.1-1, so there is no linked test.

### [`RFC5685-9.1-2`](#rfc5685-9.1-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.1-2, so there is no linked test.

### [`RFC5685-9.2-1`](#rfc5685-9.2-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.2-1, so there is no linked test.

### [`RFC5685-9.2-2`](#rfc5685-9.2-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.2-2, so there is no linked test.

### [`RFC5685-9.2-3`](#rfc5685-9.2-3)

The IPv4 address, the IPv6 address, or the FQDN of the new VPN gateway MUST be encoded as described in Section 3.5 of [2]. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.2-3, so there is no linked test.

### [`RFC5685-9.2-4`](#rfc5685-9.2-4)

The size of the nonce MUST be between 16 and 256 bytes, as described in Section 3.9 of [2]. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.2-4, so there is no linked test.

### [`RFC5685-9.2-5`](#rfc5685-9.2-5)

It MUST NOT be included in the REDIRECT payload if sent in an IKE_AUTH response or in a gateway-initiated redirect message. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.2-5, so there is no linked test.

### [`RFC5685-9.3-1`](#rfc5685-9.3-1)

The 'SPI Size' field MUST be set to 0 to indicate that the SPI is not present in this message. (§9.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.3-1, so there is no linked test.

### [`RFC5685-9.3-2`](#rfc5685-9.3-2)

The 'Protocol ID' MUST be set to 0, since the notification is not specific to a particular security association. (§9.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-9.3-2, so there is no linked test.

### [`RFC5685-11-1`](#rfc5685-11-1)

The redirect mechanism MUST NOT update any state on the client apart from the VPN gateway information. (§11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5685-11-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 7 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `6` | not stated | 6 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 2 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 2 | reviewed | not stated |
| `9.2` | not stated | 5 | reviewed | not stated |
| `9.3` | not stated | 2 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 1 | reviewed | not stated |
| `12` | not stated | 1 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `14.1` | not stated | 0 | reviewed | not stated |
| `14.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Mobile IPv6 mobile node (MN) using IKEv2 with its home agent (HA), the Section 8 role. Ze implements no Mobile IPv6: no Go file under internal/ names Mobile IPv6, MIPv6 or a home agent, and the IKE engine in internal/component/ike/ keeps no Mobile IPv6 binding state, so neither Ze nor a layer acting for it fills the MN role. The generic redirect obligations Ze does owe are the Section 3, 6 and 11 rows. | However, if the MN has authenticated information about its home agent, it MUST NOT be updated based on the IKE_SA_INIT response. |
| `8:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Mobile IPv6 mobile node (MN) using IKEv2 with its home agent (HA), the Section 8 role. Ze implements no Mobile IPv6: no Go file under internal/ names Mobile IPv6, MIPv6 or a home agent, and the IKE engine in internal/component/ike/ keeps no Mobile IPv6 binding state, so neither Ze nor a layer acting for it fills the MN role. The generic redirect obligations Ze does owe are the Section 3, 6 and 11 rows. | Gateway-initiated REDIRECT notifications exchanged in INFORMATIONAL exchanges (see Section 5) MUST NOT result in updating any Mobile IPv6 state. |
| `12:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the author of a future specification that extends the GW Identity Type registry, not an IKEv2 implementation: it says what such a document must state. Ze writes no IETF specification, so no implementation role carries it. | A specification that extends this registry MUST also mention which of the new values are valid in which Notify payload. |

## Superseded

No document obsoletes RFC 5685, so its obligations are stated where they were written.
