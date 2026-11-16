# RFC 5072 - IP Version 6 over PPP

IPv6CP. Every requirement listed for RFC 5072, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 16 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 6 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 11 | 11 of 24 tests linked to requirements, 0 without a fault test and 5 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 28 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IPv6CP |
| Automatic checks | Included |
| Requirements | 28 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tests linked to requirements | 24 |
| Recorded audit verdicts | 11 |
| Fault test records | 16 |
| Summary | `rfc/short/rfc5072.md` |
| Requirement table | `rfc/requirements/rfc5072.md` |
| RFC text | `rfc/full/rfc5072.txt` |

## Automatic checks

Included in automatic checks: IPv6 over PPP / IPV6CP (RFC 5072): 7 MET (no IPV6CP before network phase, unique interface ID, different-non-zero->Ack, Nak->resend CR, valid Reject->teardown, exactly-one IID option enforced on receive, both-zero->Reject) + 5 single-polarity positive (no IPv6 before Opened, no double-Nak on a missing IID option, collision Nak suggests a fresh different identifier, the suggestion differs from ze's last-sent identifier, the suggestion's u/l bit is zero) + 4 gap (tentative identifier's u/l bit not zeroed under 4.1-10 and 4.1-11, 1280 MTU floor lacks tagged discrimination proof, oscillation break)

## Published support status

**Area:** IPv6CP

**Test verification:** 9 of 16 MUST proven

**Evidence:** unit 9

**Unmet:** RFC5072-2-2, RFC5072-4.1-10, RFC5072-4.1-11, RFC5072-4.1-8


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 10 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`RFC5072-3-1`](#rfc5072-3-1), [`RFC5072-4.1-1`](#rfc5072-4.1-1), [`RFC5072-4.1-2`](#rfc5072-4.1-2), [`RFC5072-4.1-3`](#rfc5072-4.1-3), [`RFC5072-4.1-7`](#rfc5072-4.1-7), [`RFC5072-4.1-12`](#rfc5072-4.1-12)

**Annotated (including scoped evidence) (10):** [`RFC5072-2-1`](#rfc5072-2-1), [`RFC5072-2-2`](#rfc5072-2-2), [`RFC5072-4.1-4`](#rfc5072-4.1-4), [`RFC5072-4.1-5`](#rfc5072-4.1-5), [`RFC5072-4.1-6`](#rfc5072-4.1-6), [`RFC5072-4.1-8`](#rfc5072-4.1-8), [`RFC5072-4.1-9`](#rfc5072-4.1-9), [`RFC5072-4.1-10`](#rfc5072-4.1-10), [`RFC5072-4.1-11`](#rfc5072-4.1-11), [`RFC5072-4.1-19`](#rfc5072-4.1-19)

## Figures

- **MUST bar**: Ze verified 9, Tests awaiting verification 3, Not built 4
- **Overview bar**: Ze MUST verified 9, Tests awaiting verification 3, Not assessed 12, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 12 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 12 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5072.json](../../../data/rfc/rfc5072.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5072-3-1` | IPV6CP packets may not be exchanged until PPP has reached the network-layer protocol phase. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L407). **positive:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L170). **negative:** `unit/verify` [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L660). **negative:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L169) |
| `RFC5072-2-1` | Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L453). **positive:** `unit/verify` [`TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go#L75). **negative:** no negative test. **{single-polarity}:** ze starts its IPv6 service (Router Advertisements) only after IPV6CP reaches Opened, and there is no ze code path that emits an IPv6 packet before Opened to negatively exercise: internal/component/l2tp/ppp/ncp.go::ncpsComplete holds the session short of up until IPV6CP is Opened, and only then does internal/component/l2tp/ppp/session_run.go::afterLCPOpen call afterLCPOpenIPv6Service under its IPV6CP-Opened guard |
| `RFC5072-2-2` | PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** testing; ValidateNetworkMRU refuses an IPv6-enabled admission cap below 1280, and ipv6MRUSupported guards IPv6CP, IPv6 protocol support and IPv6 service against smaller negotiated directional limits. These implemented guards have no RFC5072-2-2 tagged/discriminated proof yet; full subscriber IPv6 remains the owner-approved post-release feature in plan/post-release/features/spec-l2tp-ipv6-subscriber.md |
| `RFC5072-4.1-1` | A Configure- Request MUST contain exactly one instance of the interface-identifier option [1]. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L123). **positive:** `unit/verify` [`TestRFC5072ClientConfigureRequestOneIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go#L78). **negative:** `unit/verify` [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1067). **negative:** `unit/verify` [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L999) |
| `RFC5072-4.1-2` | The interface identifier MUST be unique within the PPP link (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L692). **negative:** `unit/verify` [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L741) |
| `RFC5072-4.1-3` | If the two interface identifiers are different and the received interface identifier is not zero, the interface identifier MUST be acknowledged, i.e., a Configure-Ack is sent with the requested interface identifier, meaning that the responding peer agrees with the interface identifier requested. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L778). **negative:** `unit/verify` [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L817) |
| `RFC5072-4.1-4` | If the two interface identifiers are equal and are not zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier value suggested for use by the remote peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1226). **negative:** no negative test. **{single-polarity}:** the Nak's suggestion is drawn by suggestIPv6CPInterfaceID (ipv6cp.go) and rejects a redraw equal to s.localInterfaceID, so the value ze sends is never the identifier ze holds; the negative shape (an Ack, or a Nak echoing the collision) is what evalIPv6CPRequest's own unacceptable verdict already forecloses, both-polarity proven under RFC5072-4.1-2's negative test (TestIPv6CPNaksCollidingInterfaceID) |
| `RFC5072-4.1-5` | Such a suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1272). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) rejects and redraws any value equal to s.localInterfaceID, which is the identifier of ze's own last-sent (or about-to-resend) Configure-Request (see the doc comment on pppSession.localInterfaceID, session.go); TestIPv6CPSuggestionDiffersFromLocalIdentifier feeds a first draw equal to that identifier and asserts the second draw is suggested; ze produces no suggestion outside this function, so no refusing counterpart exists to assert |
| `RFC5072-4.1-6` | If the two interface identifiers are equal to zero, the interface identifier's negotiation MUST be terminated by transmitting the Configure-Reject with the interface-identifier value set to zero. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1139). **negative:** no negative test. **{single-polarity}:** the negative case -- a zero received identifier that does NOT equal ze's own zero-if-forced-so local one -- is the ordinary differing-and-zero fork, already both-polarity proven under RFC5072-4.1-3's negative test (TestIPv6CPNaksZeroInterfaceID), which shows Nak fires rather than Reject; a second test here would re-exercise that same fork rather than a distinct one |
| `RFC5072-4.1-7` | In this case, a new Configure-Request MUST be sent with the identifier value suggested in the last Configure-Nak from the peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L851). **positive:** `unit/verify` [`TestRFC5072ClientAdoptsNakSuggestion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go#L123). **negative:** `unit/verify` [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L887) |
| `RFC5072-4.1-8` | But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| `RFC5072-4.1-9` | The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L187). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) clears octet 0's bit 0x02 (canonical bit 6) on every draw; ze never derives a suggestion from a globally unique EUI-48/EUI-64 identifier loaned to the peer, so the exception this MUST carves out is never taken and there is no code path to negatively exercise. Distinct from RFC5072-4.1-11, which is the u bit of ze's own TENTATIVE identifier (requestIPv6CPInterfaceID's call to generateIPv6CPInterfaceID) rather than the SUGGESTED one this row and suggestIPv6CPInterfaceID govern -- that row's gap stands, unchanged by this phase |
| `RFC5072-4.1-10` | If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze uses no case-2 source of uniqueness (no link-layer address, no machine serial number) for its own Interface-Identifier; it draws a random identifier from crypto/rand, which is the case-3 random identifier RFC5072-4.1-11 governs. internal/component/l2tp/ppp/ipv6cp.go::generateIPv6CPInterfaceID (ipv6cp.go:141-152) never clears that identifier's u bit, and requestIPv6CPInterfaceID (ncp.go:200-207) uses it raw, so clearing the bit in generateIPv6CPInterfaceID meets this row and RFC5072-4.1-11 together |
| `RFC5072-4.1-11` | In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:141-152, generateIPv6CPInterfaceID; used raw by requestIPv6CPInterfaceID, ncp.go:200-207) |
| `RFC5072-4.1-12` | A new Configure-Request MUST NOT contain the interface-identifier option if a valid Interface-Identifier Configure-Reject is received. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L932). **negative:** `unit/verify` [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L956) |
| `RFC5072-3-2` | Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate- Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-3-3` | IPV6CP packets that are received before this phase is reached should be silently discarded. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-13` | The non-zero value of the tentative interface identifier SHOULD be chosen such that the value is unique to the link and, preferably, consistently reproducible across initializations of the IPV6CP finite state machine (administrative Close and reOpen, reboots, etc.). (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-14` | A new Configure-Request SHOULD NOT be sent to the peer until normal processing would cause it to be sent (that is, until a Configure-Nak is received or the Restart timer runs out [1]). (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-15` | But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. In this case, a new Configure- Request SHOULD be sent with the new tentative interface identifier. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-16` | If negotiation of the interface identifier is required, and the peer did not provide the option in its Configure-Request, the option SHOULD be appended to a Configure-Nak. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-17` | By default, an implementation SHOULD attempt to negotiate the interface identifier for its end of the PPP connection. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-1` | The interface identifier of IPv6 unicast addresses [5] of a PPP interface SHOULD be negotiated in the IPV6CP phase of the PPP connection setup (see Section 4.1). (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-2` | However, it SHOULD NOT be assumed that the same interface identifier is used in configuring global unicast addresses for the PPP interface using IPv6 stateless address autoconfiguration [3]. (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-3` | Therefore, it is RECOMMENDED that for PPP links with the IPV6CP interface-identifier option enabled and satisfying the aforementioned two conditions, the default value of the DupAddrDetectTransmits autoconfiguration variable [3] is set to zero by the system management. (§5) | RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-4` | The PPP peer MAY generate one or more interface identifiers, for instance, using a method described in [8], to autoconfigure one or more global unicast addresses. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-18` | If neither a unique number nor a random number can be generated, it is recommended that a zero value be used for the interface identifier transmitted in the Configure-Request. In this case, the PPP peer may provide a valid non-zero interface identifier in its response as described below. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-19` | If the next Configure-Request does not include this option, the peer MUST NOT send another Configure-Nak with this option included. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1030). **negative:** no negative test. **{single-polarity}:** the violating shape (Naking the option-less request a second time) is exactly the pre-fix defect this MUST NOT closes, and TestIPv6CPMissingOptionIsNakedOnce proves both halves of the single-session sequence -- Nak the first time, Ack rather than Nak the second -- inside one test rather than a separate negative one |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5072-2-2`](#rfc5072-2-2) PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | {gap}, no test | testing; ValidateNetworkMRU refuses an IPv6-enabled admission cap below 1280, and ipv6MRUSupported guards IPv6CP, IPv6 protocol support and IPv6 service against smaller negotiated directional limits. These implemented guards have no RFC5072-2-2 tagged/discriminated proof yet; full subscriber IPv6 remains the owner-approved post-release feature in plan/post-release/features/spec-l2tp-ipv6-subscriber.md |
| [`RFC5072-4.1-8`](#rfc5072-4.1-8) But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1) | {gap}, no test | absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| [`RFC5072-4.1-10`](#rfc5072-4.1-10) If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | {gap}, no test | ze uses no case-2 source of uniqueness (no link-layer address, no machine serial number) for its own Interface-Identifier; it draws a random identifier from crypto/rand, which is the case-3 random identifier RFC5072-4.1-11 governs. internal/component/l2tp/ppp/ipv6cp.go::generateIPv6CPInterfaceID (ipv6cp.go:141-152) never clears that identifier's u bit, and requestIPv6CPInterfaceID (ncp.go:200-207) uses it raw, so clearing the bit in generateIPv6CPInterfaceID meets this row and RFC5072-4.1-11 together |
| [`RFC5072-4.1-11`](#rfc5072-4.1-11) In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | {gap}, no test | ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:141-152, generateIPv6CPInterfaceID; used raw by requestIPv6CPInterfaceID, ncp.go:200-207) |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5072-3-1`](#rfc5072-3-1)

IPV6CP packets may not be exchanged until PPP has reached the network-layer protocol phase. (§3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go::TestNCPHeldUntilNetworkPhase, internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go::TestNCPHeldUntilNetworkPhase#2 moved. Rejudged 2026-11-08 after 14d3df65a7 against RFC 5072 Section 3 "IPV6CP packets may not be exchanged until PPP has reached the network-layer protocol phase." Send half: TestNCPHeldUntilNetworkPhase/ipv6 holds CHAP pending 3 s and onlyChallengesUntil fails on any frame but a CHAP Challenge (Code 1) with a changed Identifier (RFC 1994 4.1 retransmission), so an IPV6CP 0x8057 frame fails it; no EventIPRequest and no P2P install (-); after the Response is accepted pendingLifecycleNCP reads the 0x8057 Configure-Request (+). The stricter window isolates the rule better than before. Receive half: TestIPv6CPNoResponseBeforeNetworkPhase (-), TestIPv6CPOpenedEmitsAssigned (+), unchanged. Records: runAuthPhase revert (-), runNCPPhase mutant (+).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L660) | unit/verify | mutant, verified |
| negative | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L169) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L407) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L170) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC5072-2-1`](#rfc5072-2-1)

Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6ServiceStartsOnlyAfterIPv6CPOpened moved. Judge ppp5 2026-09-30. TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened: both NCPs enabled, IPCP Opened, IPV6CP held in Ack-Rcvd; fails on EventSessionUp and now on either attempt log, 'IPv6 service start failed' or 'refusing to start the IPv6 service', the second being what afterLCPOpenIPv6Service logs in this scenario because the peer interface id is never negotiated. So an up gate keyed on any non-Initial IPV6CP state and an early service start both turn it red (recorded ncpsComplete revert). Single-polarity positive per the row marker; ncp_test.go TestIPv6ServiceStartsOnlyAfterIPv6CPOpened proves the service does start once IPV6CP Opens.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L453) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go#L75) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC5072-2-2`](#rfc5072-2-2)

PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5072-2-2, so there is no linked test.

### [`RFC5072-4.1-1`](#rfc5072-4.1-1)

A Configure- Request MUST contain exactly one instance of the interface-identifier option [1]. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPDuplicateIdentifierOptionIsNotAcked, internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPRequestWithoutIdentifierIsNotAcked moved. Rejudge 2026-11-08: new client positive TestRFC5072ClientConfigureRequestOneIdentifier asserts the PPPoE client's first IPv6CP Configure-Request has an options field of exactly 10 octets (one Interface-Identifier, so zero or two instances are red) holding the modified EUI-64. Server side unchanged: TestIPv6CPProposesInterfaceID counts type-1 options (iidCount != 1 red); negatives TestIPv6CPRequestWithoutIdentifierIsNotAcked and TestIPv6CPDuplicateIdentifierOptionIsNotAcked refuse zero- and two-option requests.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1067) | unit/verify | revert, verified |
| negative | [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L999) | unit/verify | revert, verified |
| positive | [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L123) | unit/verify | not verified |
| positive | [`TestRFC5072ClientConfigureRequestOneIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go#L78) | unit/verify | mutant, verified |

### [`RFC5072-4.1-2`](#rfc5072-4.1-2)

The interface identifier MUST be unique within the PPP link (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L741) | unit/verify | not verified |
| positive | [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L692) | unit/verify | not verified |

### [`RFC5072-4.1-3`](#rfc5072-4.1-3)

If the two interface identifiers are different and the received interface identifier is not zero, the interface identifier MUST be acknowledged, i.e., a Configure-Ack is sent with the requested interface identifier, meaning that the responding peer agrees with the interface identifier requested. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPAcksDifferentNonZeroInterfaceID, internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPNaksZeroInterfaceID moved. Forbidden: Nak/Reject of a different non-zero identifier, or an Ack carrying a value other than the requested one. TestIPv6CPAcksDifferentNonZeroInterfaceID fails on response code != Configure-Ack and on ackOpts.InterfaceID != the requested ipv6cpTestPeerID. Negative TestIPv6CPNaksZeroInterfaceID fails when a zero identifier is Acked, bounding the non-zero condition.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L817) | unit/verify | not verified |
| positive | [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L778) | unit/verify | not verified |

### [`RFC5072-4.1-4`](#rfc5072-4.1-4)

If the two interface identifiers are equal and are not zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier value suggested for use by the remote peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPNakOnEqualNonZeroIdentifiers moved. Forbidden: answering equal non-zero identifiers with Ack or Reject, or a Nak whose suggestion equals the colliding value or is zero. TestIPv6CPNakOnEqualNonZeroIdentifiers fails on verdict != unacceptable, code != Configure-Nak, opts.InterfaceID == local (not different) and opts.InterfaceID == zero (not non-zero). Every clause asserted; single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1226) | unit/verify | revert, verified |

### [`RFC5072-4.1-5`](#rfc5072-4.1-5)

Such a suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPSuggestionDiffersFromLocalIdentifier moved. Judge 2026-09-29. TestIPv6CPSuggestionDiffersFromLocalIdentifier now feeds crypto/rand.Reader a first draw equal to the local identifier (u bit clear) and a distinct second: the suggestion must be the second draw. Deleting the id == local rejection in suggestIPv6CPInterfaceID returns the local identifier and fails both assertions. Valid single-polarity: the forced colliding draw is the violating input; ze produces no suggestion outside this function. Marker text corrected.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1272) | unit/verify | revert, verified |

### [`RFC5072-4.1-6`](#rfc5072-4.1-6)

If the two interface identifiers are equal to zero, the interface identifier's negotiation MUST be terminated by transmitting the Configure-Reject with the interface-identifier value set to zero. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPBothZeroIsRejected moved. Forbidden: answering equal-zero identifiers with a Nak/Ack, or a Reject not carrying the zero identifier. TestIPv6CPBothZeroIsRejected fails on code != Configure-Reject, a Reject missing the Interface-Identifier option, and opts.InterfaceID != zero. The sentence defines termination as transmitting that Reject, which is asserted. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1139) | unit/verify | revert, verified |

### [`RFC5072-4.1-7`](#rfc5072-4.1-7)

In this case, a new Configure-Request MUST be sent with the identifier value suggested in the last Configure-Nak from the peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPNakInvalidSuggestionNotAdopted, internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPResendsCRWithNakSuggestedID moved. Rejudge 2026-11-08: new client positive TestRFC5072ClientAdoptsNakSuggestion: a Nak suggesting 02:00:00:00:00:00:00:77 (different from any Nak the client sent, the condition of the sentence) makes the next Configure-Request carry exactly that identifier under a new Identifier. Server side unchanged: TestIPv6CPResendsCRWithNakSuggestedID requires the suggested value; negative TestIPv6CPNakInvalidSuggestionNotAdopted refuses an all-zero suggestion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L887) | unit/verify | not verified |
| positive | [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L851) | unit/verify | not verified |
| positive | [`TestRFC5072ClientAdoptsNakSuggestion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go#L123) | unit/verify | mutant, verified |

### [`RFC5072-4.1-8`](#rfc5072-4.1-8)

But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5072-4.1-8, so there is no linked test.

### [`RFC5072-4.1-9`](#rfc5072-4.1-9)

The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a suggested identifier with the u bit (canonical bit 6, octet 0 mask 0x02) set. TestIPv6CPSuggestionHasUniversalBitClear fails when suggestion[0]&ipv6cpUniversalLocalBitMask != 0 over 64 random draws; deleting the `&^=` clear in suggestIPv6CPInterfaceID turns it red with probability 1-2^-64. suggestIPv6CPInterfaceID is the only source of buildNakOrReject's IPv6 Nak value (ncp.go). The EUI exception is never taken by ze; single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L187) | unit/verify | revert, verified |

### [`RFC5072-4.1-10`](#rfc5072-4.1-10)

If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5072-4.1-10, so there is no linked test.

### [`RFC5072-4.1-11`](#rfc5072-4.1-11)

In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5072-4.1-11, so there is no linked test.

### [`RFC5072-4.1-12`](#rfc5072-4.1-12)

A new Configure-Request MUST NOT contain the interface-identifier option if a valid Interface-Identifier Configure-Reject is received. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPInterfaceIDRejectIsFatal, internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPUnknownOptionRejectNotFatal moved. Forbidden: a further Configure-Request carrying the interface-identifier option after a valid Interface-Identifier Configure-Reject. TestIPv6CPInterfaceIDRejectIsFatal fails when no EventSessionDown follows the Reject (absorbIPv6CPReject returns ncpReplyFatal, so no new CR is ever sent); a regression that absorbs and resends keeps the session up and goes red. Negative TestIPv6CPUnknownOptionRejectNotFatal fails if a Reject of an unrequested option stops negotiation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L956) | unit/verify | revert, verified |
| positive | [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L932) | unit/verify | not verified |

### [`RFC5072-4.1-19`](#rfc5072-4.1-19)

If the next Configure-Request does not include this option, the peer MUST NOT send another Configure-Nak with this option included. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/ncp_test.go::TestIPv6CPMissingOptionIsNakedOnce moved. Forbidden: Naking a second option-less Configure-Request after having Naked the first. TestIPv6CPMissingOptionIsNakedOnce fails when the second evalIPv6CPRequest verdict is not ncpRequestAcceptable (i.e. a second Nak). The first-call assertion bounds it. Single-polarity marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1030) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | rfc2119 |
| Source | rfc/full/rfc5072.txt |
| Source fingerprint | b75abcda38d89696 |
| Record | rfc/extraction/rfc5072.json |
| Mapped sentences | 15 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 15 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | Appendix A | 0 | reviewed | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | Appendix B | 0 | reviewed | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence is the lead-in to the three enumerated responses that follow it; each response carries its own MUST and its own site (4.1:8 Ack, 4.1:9 Nak, 4.1:11 Reject), so the lead-in adds no obligation of its own. | Depending on the result of the comparison, an implementation MUST respond in one of the following ways: |
| `4.1:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Word-for-word repeat of site 4.1:7, restated inside the Configure-Nak bullet; 4.1:7 already maps the u-bit obligation on a suggested identifier. | The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. |

## Superseded

No document obsoletes RFC 5072, so its obligations are stated where they were written.
