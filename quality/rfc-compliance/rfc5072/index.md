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
| Proven by a recorded break | 26.3% | 5 of 19 tagged units, 0 escaped and 2 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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

The 7 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 28 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 7 |
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
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC5072-3-1`](#rfc5072-3-1), [`RFC5072-4.1-1`](#rfc5072-4.1-1), [`RFC5072-4.1-2`](#rfc5072-4.1-2), [`RFC5072-4.1-3`](#rfc5072-4.1-3), [`RFC5072-4.1-7`](#rfc5072-4.1-7), [`RFC5072-4.1-12`](#rfc5072-4.1-12)

**Annotated instead of tested (10):** [`RFC5072-2-1`](#rfc5072-2-1), [`RFC5072-2-2`](#rfc5072-2-2), [`RFC5072-4.1-4`](#rfc5072-4.1-4), [`RFC5072-4.1-5`](#rfc5072-4.1-5), [`RFC5072-4.1-6`](#rfc5072-4.1-6), [`RFC5072-4.1-8`](#rfc5072-4.1-8), [`RFC5072-4.1-9`](#rfc5072-4.1-9), [`RFC5072-4.1-10`](#rfc5072-4.1-10), [`RFC5072-4.1-11`](#rfc5072-4.1-11), [`RFC5072-4.1-19`](#rfc5072-4.1-19)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5072-3-1` | IPV6CP packets MUST NOT be exchanged until PPP has reached the network-layer protocol phase (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L401). **negative:** `unit/verify` [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L673) |
| `RFC5072-2-1` | PPP MUST reach the network-layer protocol phase, and IPv6 Control Protocol MUST reach the Opened state before any IPv6 packet is sent (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L447). **negative:** no negative test. **{single-polarity}:** ze starts its IPv6 service (Router Advertisements) only after IPV6CP reaches Opened, and there is no ze code path that emits an IPv6 packet before Opened to negatively exercise (internal/component/l2tp/ppp/session_run.go:482) |
| `RFC5072-2-2` | PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| `RFC5072-4.1-1` | A Configure-Request MUST contain exactly one instance of the interface-identifier option (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L123). **negative:** `unit/verify` [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1101). **negative:** `unit/verify` [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1033) |
| `RFC5072-4.1-2` | The interface identifier MUST be unique within the PPP link (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L715). **negative:** `unit/verify` [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L764) |
| `RFC5072-4.1-3` | If the two interface identifiers are different and the received identifier is not zero, it MUST be acknowledged with Configure-Ack (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L801). **negative:** `unit/verify` [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L840) |
| `RFC5072-4.1-4` | If the two interface identifiers are equal and non-zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1260). **negative:** no negative test. **{single-polarity}:** the Nak's suggestion is drawn by suggestIPv6CPInterfaceID (ipv6cp.go) and rejects a redraw equal to s.localInterfaceID, so the value ze sends is never the identifier ze holds; the negative shape (an Ack, or a Nak echoing the collision) is what evalIPv6CPRequest's own unacceptable verdict already forecloses, both-polarity proven under RFC5072-4.1-2's negative test (TestIPv6CPNaksCollidingInterfaceID) |
| `RFC5072-4.1-5` | The suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1304). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) rejects and redraws any value equal to s.localInterfaceID, which is the identifier of ze's own last-sent (or about-to-resend) Configure-Request (see the doc comment on pppSession.localInterfaceID, session.go); TestIPv6CPSuggestionDiffersFromLocalIdentifier exhausts the draw loop rather than exercising a negative case there is no code path to produce |
| `RFC5072-4.1-6` | If both interface identifiers are zero, negotiation MUST be terminated by transmitting Configure-Reject with IID=0 (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1173). **negative:** no negative test. **{single-polarity}:** the negative case -- a zero received identifier that does NOT equal ze's own zero-if-forced-so local one -- is the ordinary differing-and-zero fork, already both-polarity proven under RFC5072-4.1-3's negative test (TestIPv6CPNaksZeroInterfaceID), which shows Nak fires rather than Reject; a second test here would re-exercise that same fork rather than a distinct one |
| `RFC5072-4.1-7` | On receiving Configure-Nak, a new Configure-Request MUST be sent with the suggested identifier value (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L874). **negative:** `unit/verify` [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L910) |
| `RFC5072-4.1-8` | If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| `RFC5072-4.1-9` | The "u" bit of the suggested identifier MUST be set to zero unless a globally unique EUI-48/EUI-64 derived identifier is provided for the peer's exclusive use (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L187). **negative:** no negative test. **{single-polarity}:** suggestIPv6CPInterfaceID (ipv6cp.go) clears octet 0's bit 0x02 (canonical bit 6) on every draw; ze never derives a suggestion from a globally unique EUI-48/EUI-64 identifier loaned to the peer, so the exception this MUST carves out is never taken and there is no code path to negatively exercise. Distinct from RFC5072-4.1-11, which is the u bit of ze's own TENTATIVE identifier (requestIPv6CPInterfaceID's call to generateIPv6CPInterfaceID) rather than the SUGGESTED one this row and suggestIPv6CPInterfaceID govern -- that row's gap stands, unchanged by this phase |
| `RFC5072-4.1-10` | When uniqueness source is link-layer addresses or serial numbers, the "u" bit MUST be set to zero (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| `RFC5072-4.1-11` | When a random number is generated, the "u" bit MUST be set to zero (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |
| `RFC5072-4.1-12` | A new Configure-Request MUST NOT contain the interface-identifier option if a valid Configure-Reject is received (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L955). **negative:** `unit/verify` [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L988) |
| `RFC5072-3-2` | Codes other than 1-7 should result in Code-Rejects (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-3-3` | IPV6CP packets received before network-layer phase should be silently discarded (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-13` | The non-zero tentative interface identifier SHOULD be unique to the link and preferably consistently reproducible across initializations (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-14` | A new Configure-Request SHOULD NOT be sent until normal processing would cause it (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-15` | A new Configure-Request SHOULD be sent with the new tentative interface identifier when peer's Nak proposed local value back (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-16` | If negotiation is required and peer did not provide the option, it SHOULD be appended to a Configure-Nak (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-17` | An implementation SHOULD attempt to negotiate the interface identifier for its end of the PPP connection (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-1` | The interface identifier of IPv6 unicast addresses of a PPP interface SHOULD be negotiated in the IPV6CP phase (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-2` | It SHOULD NOT be assumed that the same interface identifier is used for global unicast addresses via SLAAC (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-3` | Default DupAddrDetectTransmits SHOULD be zero when IPV6CP negotiated unique identifiers on an exclusive-prefix PPP link (§5) | RECOMMENDED | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-5-4` | The PPP peer MAY generate interface identifiers using RFC 4941 methods to autoconfigure global unicast addresses (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-18` | If no usable identifier can be produced, it MAY send zero to request the peer to supply one (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5072-4.1-19` | Having Configure-Naked a Configure-Request that omitted the interface-identifier option once, an implementation MUST NOT Configure-Nak a further Configure-Request that also omits it (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1064). **negative:** no negative test. **{single-polarity}:** the violating shape (Naking the option-less request a second time) is exactly the pre-fix defect this MUST NOT closes, and TestIPv6CPMissingOptionIsNakedOnce proves both halves of the single-session sequence -- Nak the first time, Ack rather than Nak the second -- inside one test rather than a separate negative one |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5072-2-2`](#rfc5072-2-2) PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | {gap}, no test | the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| [`RFC5072-4.1-8`](#rfc5072-4.1-8) If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1) | {gap}, no test | absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| [`RFC5072-4.1-10`](#rfc5072-4.1-10) When uniqueness source is link-layer addresses or serial numbers, the "u" bit MUST be set to zero (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| [`RFC5072-4.1-11`](#rfc5072-4.1-11) When a random number is generated, the "u" bit MUST be set to zero (§4.1) | {gap}, no test | ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5072-3-1`](#rfc5072-3-1)

IPV6CP packets MUST NOT be exchanged until PPP has reached the network-layer protocol phase (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L673) | unit/verify | unproven |
| positive | [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L401) | unit/verify | unproven |

### [`RFC5072-2-1`](#rfc5072-2-1)

PPP MUST reach the network-layer protocol phase, and IPv6 Control Protocol MUST reach the Opened state before any IPv6 packet is sent (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L447) | unit/verify | unproven |

### [`RFC5072-2-2`](#rfc5072-2-2)

PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-2-2, so no unit is bound to it.

### [`RFC5072-4.1-1`](#rfc5072-4.1-1)

A Configure-Request MUST contain exactly one instance of the interface-identifier option (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPDuplicateIdentifierOptionIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1101) | unit/verify | revert, verified |
| negative | [`TestIPv6CPRequestWithoutIdentifierIsNotAcked`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1033) | unit/verify | revert, verified |
| positive | [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L123) | unit/verify | unproven |

### [`RFC5072-4.1-2`](#rfc5072-4.1-2)

The interface identifier MUST be unique within the PPP link (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L764) | unit/verify | unproven |
| positive | [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L715) | unit/verify | unproven |

### [`RFC5072-4.1-3`](#rfc5072-4.1-3)

If the two interface identifiers are different and the received identifier is not zero, it MUST be acknowledged with Configure-Ack (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L840) | unit/verify | unproven |
| positive | [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L801) | unit/verify | unproven |

### [`RFC5072-4.1-4`](#rfc5072-4.1-4)

If the two interface identifiers are equal and non-zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPNakOnEqualNonZeroIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1260) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC5072-4.1-5`](#rfc5072-4.1-5)

The suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionDiffersFromLocalIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1304) | unit/verify | revert, verified |

### [`RFC5072-4.1-6`](#rfc5072-4.1-6)

If both interface identifiers are zero, negotiation MUST be terminated by transmitting Configure-Reject with IID=0 (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPBothZeroIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1173) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC5072-4.1-7`](#rfc5072-4.1-7)

On receiving Configure-Nak, a new Configure-Request MUST be sent with the suggested identifier value (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L910) | unit/verify | unproven |
| positive | [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L874) | unit/verify | unproven |

### [`RFC5072-4.1-8`](#rfc5072-4.1-8)

If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-8, so no unit is bound to it.

### [`RFC5072-4.1-9`](#rfc5072-4.1-9)

The "u" bit of the suggested identifier MUST be set to zero unless a globally unique EUI-48/EUI-64 derived identifier is provided for the peer's exclusive use (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPSuggestionHasUniversalBitClear`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L187) | unit/verify | revert, verified |

### [`RFC5072-4.1-10`](#rfc5072-4.1-10)

When uniqueness source is link-layer addresses or serial numbers, the "u" bit MUST be set to zero (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-10, so no unit is bound to it.

### [`RFC5072-4.1-11`](#rfc5072-4.1-11)

When a random number is generated, the "u" bit MUST be set to zero (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-11, so no unit is bound to it.

### [`RFC5072-4.1-12`](#rfc5072-4.1-12)

A new Configure-Request MUST NOT contain the interface-identifier option if a valid Configure-Reject is received (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L988) | unit/verify | unproven |
| positive | [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L955) | unit/verify | unproven |

### [`RFC5072-4.1-19`](#rfc5072-4.1-19)

Having Configure-Naked a Configure-Request that omitted the interface-identifier option once, an implementation MUST NOT Configure-Nak a further Configure-Request that also omits it (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPMissingOptionIsNakedOnce`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L1064) | unit/verify | revert, verified |

## Extraction sign-off

No extraction sign-off exists for RFC 5072, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5072, so its obligations are stated where they were written.
