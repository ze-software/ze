# RFC 5072 - IP Version 6 over PPP

Partial. Every requirement this repository extracted from RFC 5072, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 37.5% | 6 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 37.5% | 6 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 16 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 63.6% | 14 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 28 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.2% | 1 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.8% | 3 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 28 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 11 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc5072.md` |
| Requirement shard | `rfc/requirements/rfc5072.md` |
| RFC text | `rfc/full/rfc5072.txt` |

## Enrolment

Enrolled: IPv6 over PPP / IPV6CP (RFC 5072): 7 MET (no IPV6CP before network phase, unique interface ID, different-non-zero->Ack, Nak->resend CR, valid Reject->teardown, exactly-one IID option enforced on receive, both-zero->Reject) + 5 single-polarity positive (no IPv6 before Opened, no double-Nak on a missing IID option, collision Nak suggests a fresh different identifier, the suggestion differs from ze's last-sent identifier, the suggestion's u/l bit is zero) + 3 gap (tentative identifier's u/l bit not zeroed, no 1280 MTU floor, oscillation break) + 1 not-applicable (EUI-derived source)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Interface-Identifier NCP: independent FSM, generation, Configure-Req/Ack/Nak/Reject, RA/DHCPv6-PD after Opened.

**What the ledger says remains**

Gaps in [`rfc/short/rfc5072.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5072.md): ze's own TENTATIVE interface identifier (requestIPv6CPInterfaceID/generateIPv6CPInterfaceID) does not zero the u/l bit (4.1-11; the Nak-suggested identifier's u/l bit is fixed, 4.1-9); no 1280 MTU floor for IPv6 sessions (2-2, minIPMTU=68); no last-Nak-suggestion oscillation break (4.1-8). IPv6 address/prefix assignment is outside IPv6CP (DHCPv6-PD/SLAAC).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC5072-3-1`](#rfc5072-3-1), [`RFC5072-4.1-1`](#rfc5072-4.1-1), [`RFC5072-4.1-2`](#rfc5072-4.1-2), [`RFC5072-4.1-3`](#rfc5072-4.1-3), [`RFC5072-4.1-7`](#rfc5072-4.1-7), [`RFC5072-4.1-12`](#rfc5072-4.1-12)

**Annotated (including scoped evidence) (10):** [`RFC5072-2-1`](#rfc5072-2-1), [`RFC5072-2-2`](#rfc5072-2-2), [`RFC5072-4.1-4`](#rfc5072-4.1-4), [`RFC5072-4.1-5`](#rfc5072-4.1-5), [`RFC5072-4.1-6`](#rfc5072-4.1-6), [`RFC5072-4.1-8`](#rfc5072-4.1-8), [`RFC5072-4.1-9`](#rfc5072-4.1-9), [`RFC5072-4.1-10`](#rfc5072-4.1-10), [`RFC5072-4.1-11`](#rfc5072-4.1-11), [`RFC5072-4.1-19`](#rfc5072-4.1-19)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5072-3-1` | IPV6CP packets may not be exchanged until PPP has reached the network-layer protocol phase. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L411). **positive:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L167). **negative:** `unit/verify` [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L666). **negative:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L166) |
| `RFC5072-2-1` | Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L457). **positive:** `unit/verify` [`TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go#L75). **negative:** no negative test. **{single-polarity}:** ze starts its IPv6 service (Router Advertisements) only after IPV6CP reaches Opened, and there is no ze code path that emits an IPv6 packet before Opened to negatively exercise: internal/component/l2tp/ppp/ncp.go::ncpsComplete holds the session short of up until IPV6CP is Opened, and only then does internal/component/l2tp/ppp/session_run.go::afterLCPOpen call afterLCPOpenIPv6Service under its IPV6CP-Opened guard |
| `RFC5072-2-2` | PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| `RFC5072-4.1-1` | A Configure- Request MUST contain exactly one instance of the interface-identifier option [1]. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L123). **negative:** `unit/verify` [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1073). **negative:** `unit/verify` [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1005) |
| `RFC5072-4.1-2` | The interface identifier MUST be unique within the PPP link (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L698). **negative:** `unit/verify` [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L747) |
| `RFC5072-4.1-3` | If the two interface identifiers are different and the received interface identifier is not zero, the interface identifier MUST be acknowledged, i.e., a Configure-Ack is sent with the requested interface identifier, meaning that the responding peer agrees with the interface identifier requested. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L784). **negative:** `unit/verify` [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L823) |
| `RFC5072-4.1-4` | If the two interface identifiers are equal and are not zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier value suggested for use by the remote peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1232). **negative:** no negative test. **{single-polarity}:** the Nak's suggestion is drawn by suggestIPv6CPInterfaceID (ipv6cp.go) and rejects a redraw equal to s.localInterfaceID, so the value ze sends is never the identifier ze holds; the negative shape (an Ack, or a Nak echoing the collision) is what evalIPv6CPRequest's own unacceptable verdict already forecloses, both-polarity proven under RFC5072-4.1-2's negative test (TestIPv6CPNaksCollidingInterfaceID) |
| `RFC5072-4.1-5` | Such a suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1278). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) rejects and redraws any value equal to s.localInterfaceID, which is the identifier of ze's own last-sent (or about-to-resend) Configure-Request (see the doc comment on pppSession.localInterfaceID, session.go); TestIPv6CPSuggestionDiffersFromLocalIdentifier feeds a first draw equal to that identifier and asserts the second draw is suggested; ze produces no suggestion outside this function, so no refusing counterpart exists to assert |
| `RFC5072-4.1-6` | If the two interface identifiers are equal to zero, the interface identifier's negotiation MUST be terminated by transmitting the Configure-Reject with the interface-identifier value set to zero. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1145). **negative:** no negative test. **{single-polarity}:** the negative case -- a zero received identifier that does NOT equal ze's own zero-if-forced-so local one -- is the ordinary differing-and-zero fork, already both-polarity proven under RFC5072-4.1-3's negative test (TestIPv6CPNaksZeroInterfaceID), which shows Nak fires rather than Reject; a second test here would re-exercise that same fork rather than a distinct one |
| `RFC5072-4.1-7` | In this case, a new Configure-Request MUST be sent with the identifier value suggested in the last Configure-Nak from the peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L857). **negative:** `unit/verify` [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L893) |
| `RFC5072-4.1-8` | But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| `RFC5072-4.1-9` | The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L187). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) clears octet 0's bit 0x02 (canonical bit 6) on every draw; ze never derives a suggestion from a globally unique EUI-48/EUI-64 identifier loaned to the peer, so the exception this MUST carves out is never taken and there is no code path to negatively exercise. Distinct from RFC5072-4.1-11, which is the u bit of ze's own TENTATIVE identifier (requestIPv6CPInterfaceID's call to generateIPv6CPInterfaceID) rather than the SUGGESTED one this row and suggestIPv6CPInterfaceID govern -- that row's gap stands, unchanged by this phase |
| `RFC5072-4.1-10` | If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| `RFC5072-4.1-11` | In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |
| `RFC5072-4.1-12` | A new Configure-Request MUST NOT contain the interface-identifier option if a valid Interface-Identifier Configure-Reject is received. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L938). **negative:** `unit/verify` [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L962) |
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
| `RFC5072-4.1-19` | If the next Configure-Request does not include this option, the peer MUST NOT send another Configure-Nak with this option included. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1036). **negative:** no negative test. **{single-polarity}:** the violating shape (Naking the option-less request a second time) is exactly the pre-fix defect this MUST NOT closes, and TestIPv6CPMissingOptionIsNakedOnce proves both halves of the single-session sequence -- Nak the first time, Ack rather than Nak the second -- inside one test rather than a separate negative one |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5072-2-2`](#rfc5072-2-2) PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | {gap}, no test | the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| [`RFC5072-4.1-8`](#rfc5072-4.1-8) But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1) | {gap}, no test | absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| [`RFC5072-4.1-10`](#rfc5072-4.1-10) If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| [`RFC5072-4.1-11`](#rfc5072-4.1-11) In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1) | {gap}, no test | ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5072-3-1`](#rfc5072-3-1)

IPV6CP packets may not be exchanged until PPP has reached the network-layer protocol phase. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Send half now driven: TestNCPHeldUntilNetworkPhase/ipv6, CHAP pending 3 s -> no IPV6CP frame (no frame at all) and no address request (-); accepted -> IPV6CP (0x8057) Configure-Request (+). Receive half: TestIPv6CPNoResponseBeforeNetworkPhase. Records on runAuthPhase (-) and runNCPPhase (+).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L666) | unit/verify | revert, verified |
| negative | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L411) | unit/verify | revert, verified |
| positive | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L167) | unit/verify | revert, verified |

### [`RFC5072-2-1`](#rfc5072-2-1)

Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp5 2026-09-30. TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened: both NCPs enabled, IPCP Opened, IPV6CP held in Ack-Rcvd; fails on EventSessionUp and now on either attempt log, 'IPv6 service start failed' or 'refusing to start the IPv6 service', the second being what afterLCPOpenIPv6Service logs in this scenario because the peer interface id is never negotiated. So an up gate keyed on any non-Initial IPV6CP state and an early service start both turn it red (recorded ncpsComplete revert). Single-polarity positive per the row marker; ncp_test.go TestIPv6ServiceStartsOnlyAfterIPv6CPOpened proves the service does start once IPV6CP Opens.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L457) | unit/verify | revert, verified |
| positive | [`TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_opened_test.go#L75) | unit/verify | revert, verified |

### [`RFC5072-2-2`](#rfc5072-2-2)

PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-2-2, so no unit is bound to it.

### [`RFC5072-4.1-1`](#rfc5072-4.1-1)

A Configure- Request MUST contain exactly one instance of the interface-identifier option [1]. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Configure-Request with zero or more than one interface-identifier option. Sent side: TestIPv6CPProposesInterfaceID counts type-1 options in ze's raw initial CR and fails on iidCount != 1 (red on zero or two). Received side: TestIPv6CPRequestWithoutIdentifierIsNotAcked fails when a zero-option CR is ncpRequestAcceptable; TestIPv6CPDuplicateIdentifierOptionIsNotAcked fails when a two-option CR is Acked. Both polarities, both counts covered.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1073) | unit/verify | revert, verified |
| negative | [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1005) | unit/verify | revert, verified |
| positive | [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L123) | unit/verify | unproven |

### [`RFC5072-4.1-2`](#rfc5072-4.1-2)

The interface identifier MUST be unique within the PPP link (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L747) | unit/verify | unproven |
| positive | [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L698) | unit/verify | unproven |

### [`RFC5072-4.1-3`](#rfc5072-4.1-3)

If the two interface identifiers are different and the received interface identifier is not zero, the interface identifier MUST be acknowledged, i.e., a Configure-Ack is sent with the requested interface identifier, meaning that the responding peer agrees with the interface identifier requested. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Nak/Reject of a different non-zero identifier, or an Ack carrying a value other than the requested one. TestIPv6CPAcksDifferentNonZeroInterfaceID fails on response code != Configure-Ack and on ackOpts.InterfaceID != the requested ipv6cpTestPeerID. Negative TestIPv6CPNaksZeroInterfaceID fails when a zero identifier is Acked, bounding the non-zero condition.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L823) | unit/verify | unproven |
| positive | [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L784) | unit/verify | unproven |

### [`RFC5072-4.1-4`](#rfc5072-4.1-4)

If the two interface identifiers are equal and are not zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier value suggested for use by the remote peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering equal non-zero identifiers with Ack or Reject, or a Nak whose suggestion equals the colliding value or is zero. TestIPv6CPNakOnEqualNonZeroIdentifiers fails on verdict != unacceptable, code != Configure-Nak, opts.InterfaceID == local (not different) and opts.InterfaceID == zero (not non-zero). Every clause asserted; single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1232) | unit/verify | revert, verified |

### [`RFC5072-4.1-5`](#rfc5072-4.1-5)

Such a suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. TestIPv6CPSuggestionDiffersFromLocalIdentifier now feeds crypto/rand.Reader a first draw equal to the local identifier (u bit clear) and a distinct second: the suggestion must be the second draw. Deleting the id == local rejection in suggestIPv6CPInterfaceID returns the local identifier and fails both assertions. Valid single-polarity: the forced colliding draw is the violating input; ze produces no suggestion outside this function. Marker text corrected.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1278) | unit/verify | revert, verified |

### [`RFC5072-4.1-6`](#rfc5072-4.1-6)

If the two interface identifiers are equal to zero, the interface identifier's negotiation MUST be terminated by transmitting the Configure-Reject with the interface-identifier value set to zero. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering equal-zero identifiers with a Nak/Ack, or a Reject not carrying the zero identifier. TestIPv6CPBothZeroIsRejected fails on code != Configure-Reject, a Reject missing the Interface-Identifier option, and opts.InterfaceID != zero. The sentence defines termination as transmitting that Reject, which is asserted. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1145) | unit/verify | revert, verified |

### [`RFC5072-4.1-7`](#rfc5072-4.1-7)

In this case, a new Configure-Request MUST be sent with the identifier value suggested in the last Configure-Nak from the peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after a peer Nak with a suggestion, resending the old identifier or none. TestIPv6CPResendsCRWithNakSuggestedID fails when the resent CR lacks the option or opts.InterfaceID != suggested. Negative TestIPv6CPNakInvalidSuggestionNotAdopted fails if an all-zero suggestion is adopted, bounding the rule to usable suggestions.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L893) | unit/verify | unproven |
| positive | [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L857) | unit/verify | unproven |

### [`RFC5072-4.1-8`](#rfc5072-4.1-8)

But if the received interface identifier is equal to the one sent in the last Configure-Nak, a new interface identifier MUST be chosen. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-8, so no unit is bound to it.

### [`RFC5072-4.1-9`](#rfc5072-4.1-9)

The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a suggested identifier with the u bit (canonical bit 6, octet 0 mask 0x02) set. TestIPv6CPSuggestionHasUniversalBitClear fails when suggestion[0]&ipv6cpUniversalLocalBitMask != 0 over 64 random draws; deleting the `&^=` clear in suggestIPv6CPInterfaceID turns it red with probability 1-2^-64. suggestIPv6CPInterfaceID is the only source of buildNakOrReject's IPv6 Nak value (ncp.go). The EUI exception is never taken by ze; single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc5072_ipv6cp_test.go#L187) | unit/verify | revert, verified |

### [`RFC5072-4.1-10`](#rfc5072-4.1-10)

If an IEEE global identifier is not available, a different source of uniqueness should be used. Suggested sources of uniqueness include link-layer addresses, machine serial numbers, et cetera. In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-10, so no unit is bound to it.

### [`RFC5072-4.1-11`](#rfc5072-4.1-11)

In this case, the "u" bit of the interface identifier MUST be set to zero (0). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-11, so no unit is bound to it.

### [`RFC5072-4.1-12`](#rfc5072-4.1-12)

A new Configure-Request MUST NOT contain the interface-identifier option if a valid Interface-Identifier Configure-Reject is received. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a further Configure-Request carrying the interface-identifier option after a valid Interface-Identifier Configure-Reject. TestIPv6CPInterfaceIDRejectIsFatal fails when no EventSessionDown follows the Reject (absorbIPv6CPReject returns ncpReplyFatal, so no new CR is ever sent); a regression that absorbs and resends keeps the session up and goes red. Negative TestIPv6CPUnknownOptionRejectNotFatal fails if a Reject of an unrequested option stops negotiation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L962) | unit/verify | revert, verified |
| positive | [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L938) | unit/verify | unproven |

### [`RFC5072-4.1-19`](#rfc5072-4.1-19)

If the next Configure-Request does not include this option, the peer MUST NOT send another Configure-Nak with this option included. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Naking a second option-less Configure-Request after having Naked the first. TestIPv6CPMissingOptionIsNakedOnce fails when the second evalIPv6CPRequest verdict is not ncpRequestAcceptable (i.e. a second Nak). The first-call assertion bounds it. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1036) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 15 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | Appendix B | 0 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence is the lead-in to the three enumerated responses that follow it; each response carries its own MUST and its own site (4.1:8 Ack, 4.1:9 Nak, 4.1:11 Reject), so the lead-in adds no obligation of its own. | Depending on the result of the comparison, an implementation MUST respond in one of the following ways: |
| `4.1:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Word-for-word repeat of site 4.1:7, restated inside the Configure-Nak bullet; 4.1:7 already maps the u-bit obligation on a suggested identifier. | The "u" (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer. |

## Superseded

No document obsoletes RFC 5072, so its obligations are stated where they were written.
