# RFC 5072 - IP Version 6 over PPP

Partial. Every requirement this repository extracted from RFC 5072, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 38.5% | 5 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 15.4% | 2 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 15 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 15 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 46.2% | 6 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 15 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 6 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5072.md` |
| Requirement shard | `rfc/requirements/rfc5072.md` |
| RFC text | `rfc/full/rfc5072.txt` |

## Enrolment

Enrolled: IPv6 over PPP / IPV6CP (RFC 5072): 5 MET (no IPV6CP before network phase, unique interface ID, different-non-zero->Ack, Nak->resend CR, valid Reject->teardown) + 2 single-polarity positive (no IPv6 before Opened, exactly-one IID option) + 6 gap (u/l bit not zeroed, no 1280 MTU floor, collision Nak reuse, oscillation break) + 2 not-applicable (mutual-zero deadlock, EUI-derived source)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Interface-Identifier NCP: independent FSM, generation, Configure-Req/Ack/Nak/Reject, RA/DHCPv6-PD after Opened.

**What the ledger says remains**

Gaps in [`rfc/short/rfc5072.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5072.md): the random interface identifier does not zero the u/l bit (4.1-9/4.1-11); no 1280 MTU floor for IPv6 sessions (2-2, minIPMTU=68); the collision Configure-Nak reuses s.peerInterfaceID rather than a fresh different non-zero identifier (4.1-4/4.1-5); no last-Nak-suggestion oscillation break (4.1-8). IPv6 address/prefix assignment is outside IPv6CP (DHCPv6-PD/SLAAC).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **15** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5072-3-1`](#rfc5072-3-1), [`RFC5072-4.1-2`](#rfc5072-4.1-2), [`RFC5072-4.1-3`](#rfc5072-4.1-3), [`RFC5072-4.1-7`](#rfc5072-4.1-7), [`RFC5072-4.1-12`](#rfc5072-4.1-12)

**Annotated instead of tested (10):** [`RFC5072-2-1`](#rfc5072-2-1), [`RFC5072-2-2`](#rfc5072-2-2), [`RFC5072-4.1-1`](#rfc5072-4.1-1), [`RFC5072-4.1-4`](#rfc5072-4.1-4), [`RFC5072-4.1-5`](#rfc5072-4.1-5), [`RFC5072-4.1-6`](#rfc5072-4.1-6), [`RFC5072-4.1-8`](#rfc5072-4.1-8), [`RFC5072-4.1-9`](#rfc5072-4.1-9), [`RFC5072-4.1-10`](#rfc5072-4.1-10), [`RFC5072-4.1-11`](#rfc5072-4.1-11)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5072-3-1` | IPV6CP packets MUST NOT be exchanged until PPP has reached the network-layer protocol phase (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L397). **negative:** `unit/verify` [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L667) |
| `RFC5072-2-1` | PPP MUST reach the network-layer protocol phase, and IPv6 Control Protocol MUST reach the Opened state before any IPv6 packet is sent (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L442). **negative:** no negative test. **{single-polarity}:** ze starts its IPv6 service (Router Advertisements) only after IPV6CP reaches Opened, and there is no ze code path that emits an IPv6 packet before Opened to negatively exercise (internal/component/l2tp/ppp/session_run.go:482) |
| `RFC5072-2-2` | PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| `RFC5072-4.1-1` | A Configure-Request MUST contain exactly one instance of the interface-identifier option (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L120). **negative:** no negative test. **{single-polarity}:** ze's IPV6CP Configure-Request writer unconditionally serializes exactly one Interface-Identifier option (HasInterfaceID always true), so ze's Configure-Requests structurally satisfy the rule (internal/component/l2tp/ppp/ncp.go:551, ipv6cp.go:74) |
| `RFC5072-4.1-2` | The interface identifier MUST be unique within the PPP link (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L709). **negative:** `unit/verify` [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L758) |
| `RFC5072-4.1-3` | If the two interface identifiers are different and the received identifier is not zero, it MUST be acknowledged with Configure-Ack (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L795). **negative:** `unit/verify` [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L834) |
| `RFC5072-4.1-4` | If the two interface identifiers are equal and non-zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends a Configure-Nak on collision but the suggested value is s.peerInterfaceID, which the collision branch never freshly regenerates, so the different-non-zero guarantee is not produced (internal/component/l2tp/ppp/ncp.go:470, :607) |
| `RFC5072-4.1-5` | The suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Nak suggestion is s.peerInterfaceID with no comparison against ze's last-sent Configure-Request identifier and no last-Configure-Request tracking, so nothing enforces the difference (internal/component/l2tp/ppp/ncp.go:607-611) |
| `RFC5072-4.1-6` | If both interface identifiers are zero, negotiation MUST be terminated by transmitting Configure-Reject with IID=0 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's local interface identifier is always non-zero (generation guarantees non-zero and ze never sends a zero request), so the mutual-zero deadlock this rule resolves cannot arise on ze's end (internal/component/l2tp/ppp/ipv6cp.go:133, ncp.go:184) |
| `RFC5072-4.1-7` | On receiving Configure-Nak, a new Configure-Request MUST be sent with the suggested identifier value (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L868). **negative:** `unit/verify` [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L904) |
| `RFC5072-4.1-8` | If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| `RFC5072-4.1-9` | The "u" bit of the suggested identifier MUST be set to zero unless a globally unique EUI-48/EUI-64 derived identifier is provided for the peer's exclusive use (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's interface identifier is a raw crypto/rand draw (never EUI-derived) and the generator never clears the u/l bit (byte0 & 0x02), so about half of ze's proposed identifiers carry u=1 in violation (internal/component/l2tp/ppp/ipv6cp.go:133-144) |
| `RFC5072-4.1-10` | When uniqueness source is link-layer addresses or serial numbers, the "u" bit MUST be set to zero (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| `RFC5072-4.1-11` | When a random number is generated, the "u" bit MUST be set to zero (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |
| `RFC5072-4.1-12` | A new Configure-Request MUST NOT contain the interface-identifier option if a valid Configure-Reject is received (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L949). **negative:** `unit/verify` [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L982) |
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

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5072-2-2`](#rfc5072-2-2) PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2) | {gap}, no test | the MTU floor applied to an IPv6-enabled session is minIPMTU=68, not 1280, so a session whose negotiated MRU is below 1284 gets a sub-1280 MTU; no 1280 clamp exists (internal/component/l2tp/ppp/session_run.go:42, :472) |
| [`RFC5072-4.1-4`](#rfc5072-4.1-4) If the two interface identifiers are equal and non-zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier (§4.1) | {gap}, no test | ze sends a Configure-Nak on collision but the suggested value is s.peerInterfaceID, which the collision branch never freshly regenerates, so the different-non-zero guarantee is not produced (internal/component/l2tp/ppp/ncp.go:470, :607) |
| [`RFC5072-4.1-5`](#rfc5072-4.1-5) The suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer (§4.1) | {gap}, no test | the Nak suggestion is s.peerInterfaceID with no comparison against ze's last-sent Configure-Request identifier and no last-Configure-Request tracking, so nothing enforces the difference (internal/component/l2tp/ppp/ncp.go:607-611) |
| [`RFC5072-4.1-6`](#rfc5072-4.1-6) If both interface identifiers are zero, negotiation MUST be terminated by transmitting Configure-Reject with IID=0 (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's local interface identifier is always non-zero (generation guarantees non-zero and ze never sends a zero request), so the mutual-zero deadlock this rule resolves cannot arise on ze's end (internal/component/l2tp/ppp/ipv6cp.go:133, ncp.go:184) |
| [`RFC5072-4.1-8`](#rfc5072-4.1-8) If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1) | {gap}, no test | absorbIPv6CPNak adopts the peer's suggestion unconditionally; ze keeps no record of the IID it suggested in its own last Nak and never regenerates on oscillation (internal/component/l2tp/ppp/ncp.go:479-487) |
| [`RFC5072-4.1-9`](#rfc5072-4.1-9) The "u" bit of the suggested identifier MUST be set to zero unless a globally unique EUI-48/EUI-64 derived identifier is provided for the peer's exclusive use (§4.1) | {gap}, no test | ze's interface identifier is a raw crypto/rand draw (never EUI-derived) and the generator never clears the u/l bit (byte0 & 0x02), so about half of ze's proposed identifiers carry u=1 in violation (internal/component/l2tp/ppp/ipv6cp.go:133-144) |
| [`RFC5072-4.1-10`](#rfc5072-4.1-10) When uniqueness source is link-layer addresses or serial numbers, the "u" bit MUST be set to zero (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's sole interface-identifier source is crypto/rand; it never derives an identifier from a link-layer address or serial number, so this source-specific clause binds a code path ze does not have (internal/component/l2tp/ppp/ipv6cp.go:133) |
| [`RFC5072-4.1-11`](#rfc5072-4.1-11) When a random number is generated, the "u" bit MUST be set to zero (§4.1) | {gap}, no test | ze's identifier is randomly generated and the generator does not force the u bit to zero, the exact case this MUST governs (internal/component/l2tp/ppp/ipv6cp.go:133-144) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5072-3-1`](#rfc5072-3-1)

IPV6CP packets MUST NOT be exchanged until PPP has reached the network-layer protocol phase (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNoResponseBeforeNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L667) | unit/verify | unproven |
| positive | [`TestIPv6CPOpenedEmitsAssigned`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L397) | unit/verify | unproven |

### [`RFC5072-2-1`](#rfc5072-2-1)

PPP MUST reach the network-layer protocol phase, and IPv6 Control Protocol MUST reach the Opened state before any IPv6 packet is sent (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6ServiceStartsOnlyAfterIPv6CPOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L442) | unit/verify | unproven |

### [`RFC5072-2-2`](#rfc5072-2-2)

PPP links supporting IPv6 MUST allow the information field to be at least as large as the minimum link MTU size required for IPv6 (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-2-2, so no unit is bound to it.

### [`RFC5072-4.1-1`](#rfc5072-4.1-1)

A Configure-Request MUST contain exactly one instance of the interface-identifier option (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6CPProposesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ipv6cp_test.go#L120) | unit/verify | unproven |

### [`RFC5072-4.1-2`](#rfc5072-4.1-2)

The interface identifier MUST be unique within the PPP link (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksCollidingInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L758) | unit/verify | unproven |
| positive | [`TestIPv6CPInterfaceIDsDiffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L709) | unit/verify | unproven |

### [`RFC5072-4.1-3`](#rfc5072-4.1-3)

If the two interface identifiers are different and the received identifier is not zero, it MUST be acknowledged with Configure-Ack (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNaksZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L834) | unit/verify | unproven |
| positive | [`TestIPv6CPAcksDifferentNonZeroInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L795) | unit/verify | unproven |

### [`RFC5072-4.1-4`](#rfc5072-4.1-4)

If the two interface identifiers are equal and non-zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-4, so no unit is bound to it.

### [`RFC5072-4.1-5`](#rfc5072-4.1-5)

The suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-5, so no unit is bound to it.

### [`RFC5072-4.1-6`](#rfc5072-4.1-6)

If both interface identifiers are zero, negotiation MUST be terminated by transmitting Configure-Reject with IID=0 (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-6, so no unit is bound to it.

### [`RFC5072-4.1-7`](#rfc5072-4.1-7)

On receiving Configure-Nak, a new Configure-Request MUST be sent with the suggested identifier value (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6CPNakInvalidSuggestionNotAdopted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L904) | unit/verify | unproven |
| positive | [`TestIPv6CPResendsCRWithNakSuggestedID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L868) | unit/verify | unproven |

### [`RFC5072-4.1-8`](#rfc5072-4.1-8)

If the received interface identifier equals the one sent in the last Configure-Nak, a new interface identifier MUST be chosen (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-8, so no unit is bound to it.

### [`RFC5072-4.1-9`](#rfc5072-4.1-9)

The "u" bit of the suggested identifier MUST be set to zero unless a globally unique EUI-48/EUI-64 derived identifier is provided for the peer's exclusive use (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5072-4.1-9, so no unit is bound to it.

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
| negative | [`TestIPv6CPUnknownOptionRejectNotFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L982) | unit/verify | unproven |
| positive | [`TestIPv6CPInterfaceIDRejectIsFatal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L949) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5072, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5072, so its obligations are stated where they were written.
