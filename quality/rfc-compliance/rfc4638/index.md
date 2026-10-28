# RFC 4638 - Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE)

Partial. Every requirement this repository extracted from RFC 4638, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 6 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 6 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4638.md` |
| Requirement shard | `rfc/requirements/rfc4638.md` |
| RFC text | `rfc/full/rfc4638.txt` |

## Enrolment

Enrolled: PPPoE MRU/MTU above 1492: the PPP-Max-Payload discovery tag and the LCP MRU ceiling it lifts. Every MUST-level row is classified: the 1492 ceiling rows roll up to RFC 2516 and RFC 1661, the server-side larger-MRU rows and the client tag row (RFC4638-4-1) are gaps owned by plan/release/fixes/spec-pppoe-rfc4638-max-payload.md, because no owner ruling names an MTU above 1492 unwanted on either side (ai/META-DECISIONS.md, Goal and scope).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The 1492-octet default: both the access concentrator (`MaxMRU: PPPoEMaxMTU`, [`internal/component/l2tp/pppoe/server.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server.go)) and the client (`pppoeSessionMTU`, 68..1492) never negotiate an MRU above 1492. A PADI's PPP-Max-Payload tag is echoed in the PADO (`BuildPADO`).

**What the ledger says remains**

The larger MRU itself is absent: the access concentrator caps the LCP MRU at 1492 whatever the client's PPP-Max-Payload tag says ([`RFC4638-3-1`](#rfc4638-3-1), [`RFC4638-5.1-2`](#rfc4638-5.1-2), [`RFC4638-5.1-3`](#rfc4638-5.1-3)), `BuildPADS` does not echo the tag while `BuildPADO` does ([`RFC4638-4-2`](#rfc4638-4-2)), and no MRU-sized Echo-Request test runs ([`RFC4638-5.2-2`](#rfc4638-5.2-2)). The client cannot use an MTU above 1492 and never sends the tag ([`RFC4638-4-1`](#rfc4638-4-1)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 4 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (6):** [`RFC4638-3-1`](#rfc4638-3-1), [`RFC4638-4-1`](#rfc4638-4-1), [`RFC4638-4-2`](#rfc4638-4-2), [`RFC4638-5.1-2`](#rfc4638-5.1-2), [`RFC4638-5.1-3`](#rfc4638-5.1-3), [`RFC4638-5.2-2`](#rfc4638-5.2-2)

**Derived from other rows (4):** [`RFC4638-3-2`](#rfc4638-3-2), [`RFC4638-3-3`](#rfc4638-3-3), [`RFC4638-5.1-1`](#rfc4638-5.1-1), [`RFC4638-5.1-4`](#rfc4638-5.1-4)

## Attainable ceiling

Mandatory ceiling: 6 of 6 (100.0%). Optional ceiling: 3 of 3 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc4638 6 of 6 (100.0%). Lowest optional ceiling: rfc4638 3 of 3 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4638-3-1` | Since next-generation broadband networks are built around Ethernet systems supporting baby-giants and jumbo frames with payload sizes larger than the normal Ethernet MTU of 1500 octets, a BRAS acting as a PPPoE server MUST support PPPoE MRU negotiations larger than 1492 octets in order to limit the amount of fragmented packets in networks similar to those described in Section 1. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the access concentrator hands every session MaxMRU PPPoEMaxMTU (1492) and never raises it; internal/component/l2tp/pppoe/server.go |
| `RFC4638-3-2` | By default, the Maximum-Receive-Unit (MRU) option MUST follow the rules set forward in RFC 1661 [2] but MUST NOT be negotiated to a size larger than 1492 to guarantee compatibility with Ethernet network segments limited to 1500-octet frames. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC1661-2-2, RFC2516-x-9; the sentence restates the RFC 1661 MRU rule and the RFC 2516 1492 ceiling those rows carry. **derived:** gap: RFC1661-2-2 is annotated {gap} |
| `RFC4638-3-3` | In such a case, as the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MRU MUST NOT be greater than 1492. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC2516-x-9; the 1492 ceiling is the RFC 2516 Section 7 obligation. **derived:** met |
| `RFC4638-4-1` | If a PPPoE client wants to use an MTU/MRU higher than 1492 octets, then it MUST include an optional PPP-Max-Payload Tag in the PADI and PADR packets. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the PPPoE client cannot use an MTU/MRU above 1492: internal/component/l2tp/pppoeclient/dialer.go::pppoeSessionMTU refuses a configured session MTU above 1492, and the client puts no PPP-Max-Payload tag in its PADI or PADR; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC4638-4-2` | If the PPPoE server can support an MTU/MRU higher than 1492 octets, it MUST respond with an echo of the clients tag in the PADO and PADS packets when the PPP-Max-Payload tag is received from the client. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildPADO echoes the tag but BuildPADS does not, and the server cannot honour a larger MRU (RFC4638-3-1); internal/component/l2tp/pppoe/discovery.go |
| `RFC4638-5.1-1` | Since Ethernet (without jumbo frames) has a maximum payload size of 1500 octets, the PPPoE header is 6 octets, and the PPP Protocol ID is 2 octets, the Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a size larger than 1492, unless both the PPPoE client and server have indicated the ability to support a larger MRU in the PPPoE Discovery Stage. (§5.1) | MUST NOT | 5.1 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC2516-x-9; ze never negotiates above 1492, which is the RFC 2516 Section 7 obligation. **derived:** met |
| `RFC4638-5.1-2` | The initial MRU negotiation for the PPP/PPPoE server MUST follow a flow as shown below: (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the server's PPP_MRU_Max is always 1492; the PPP-Max-Payload tag and the interface MTU never enter it; internal/component/l2tp/pppoe/server.go |
| `RFC4638-5.1-3` | If the PPP-Max-Payload tag is present and greater than 1492, it MUST be considered along with the server's interface MTU settings when the maximum value is selected for the normal RFC 1661 [2] MRU negotiation which decides the actual MRU to use. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the session start carries MaxMRU PPPoEMaxMTU regardless of the tag; internal/component/l2tp/pppoe/server.go |
| `RFC4638-5.1-4` | If the PPP-Max-Payload tag isn't present or is present but below 1492, then the existing MRU constraint of 1492 octets MUST stay applicable, thus preserving backward compatibility. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC2516-x-9; the constraint that stays applicable is the RFC 2516 Section 7 ceiling. **derived:** met |
| `RFC4638-5.2-2` | If these packets receive replies, the sending side MUST not send packets bigger than 1492 octets for this session. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze negotiates no MRU above 1492 (RFC4638-3-1) and runs no MRU-sized Echo-Request test, so the fallback this row governs does not exist |
| `RFC4638-5.2-1` | If the MRU is negotiated to a value larger than 1492 octets, the sending side SHOULD have the option of sending one or more MRU-sized Echo-Request packets once the session is opened. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4638-3-4` | When such a tag is received by the PPPoE server, the server MAY allow the negotiation of an MRU larger than 1492 and the use of an MTU larger than 1492, subject to limitations of its local configuration and according to the rules set forward in RFC 1661 [2], within the limits of the maximum payload size indicated by the PPPoE client. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4638-5.2-3` | If no Echo-Replies are received, the sending side MAY choose to repeat the test with 1492 octets Echo-Request packets. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4638-3-1`](#rfc4638-3-1) Since next-generation broadband networks are built around Ethernet systems supporting baby-giants and jumbo frames with payload sizes larger than the normal Ethernet MTU of 1500 octets, a BRAS acting as a PPPoE server MUST support PPPoE MRU negotiations larger than 1492 octets in order to limit the amount of fragmented packets in networks similar to those described in Section 1. (§3) | {gap}, no test | the access concentrator hands every session MaxMRU PPPoEMaxMTU (1492) and never raises it; internal/component/l2tp/pppoe/server.go |
| [`RFC4638-3-2`](#rfc4638-3-2) By default, the Maximum-Receive-Unit (MRU) option MUST follow the rules set forward in RFC 1661 [2] but MUST NOT be negotiated to a size larger than 1492 to guarantee compatibility with Ethernet network segments limited to 1500-octet frames. (§3) | no test | no test carries this requirement id; annotated {rollup}: RFC1661-2-2, RFC2516-x-9; the sentence restates the RFC 1661 MRU rule and the RFC 2516 1492 ceiling those rows carry |
| [`RFC4638-3-3`](#rfc4638-3-3) In such a case, as the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MRU MUST NOT be greater than 1492. (§3) | no test | no test carries this requirement id; annotated {rollup}: RFC2516-x-9; the 1492 ceiling is the RFC 2516 Section 7 obligation |
| [`RFC4638-4-1`](#rfc4638-4-1) If a PPPoE client wants to use an MTU/MRU higher than 1492 octets, then it MUST include an optional PPP-Max-Payload Tag in the PADI and PADR packets. (§4) | {gap}, no test | the PPPoE client cannot use an MTU/MRU above 1492: internal/component/l2tp/pppoeclient/dialer.go::pppoeSessionMTU refuses a configured session MTU above 1492, and the client puts no PPP-Max-Payload tag in its PADI or PADR; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC4638-4-2`](#rfc4638-4-2) If the PPPoE server can support an MTU/MRU higher than 1492 octets, it MUST respond with an echo of the clients tag in the PADO and PADS packets when the PPP-Max-Payload tag is received from the client. (§4) | {gap}, no test | BuildPADO echoes the tag but BuildPADS does not, and the server cannot honour a larger MRU (RFC4638-3-1); internal/component/l2tp/pppoe/discovery.go |
| [`RFC4638-5.1-1`](#rfc4638-5.1-1) Since Ethernet (without jumbo frames) has a maximum payload size of 1500 octets, the PPPoE header is 6 octets, and the PPP Protocol ID is 2 octets, the Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a size larger than 1492, unless both the PPPoE client and server have indicated the ability to support a larger MRU in the PPPoE Discovery Stage. (§5.1) | no test | no test carries this requirement id; annotated {rollup}: RFC2516-x-9; ze never negotiates above 1492, which is the RFC 2516 Section 7 obligation |
| [`RFC4638-5.1-2`](#rfc4638-5.1-2) The initial MRU negotiation for the PPP/PPPoE server MUST follow a flow as shown below: (§5.1) | {gap}, no test | the server's PPP_MRU_Max is always 1492; the PPP-Max-Payload tag and the interface MTU never enter it; internal/component/l2tp/pppoe/server.go |
| [`RFC4638-5.1-3`](#rfc4638-5.1-3) If the PPP-Max-Payload tag is present and greater than 1492, it MUST be considered along with the server's interface MTU settings when the maximum value is selected for the normal RFC 1661 [2] MRU negotiation which decides the actual MRU to use. (§5.1) | {gap}, no test | the session start carries MaxMRU PPPoEMaxMTU regardless of the tag; internal/component/l2tp/pppoe/server.go |
| [`RFC4638-5.1-4`](#rfc4638-5.1-4) If the PPP-Max-Payload tag isn't present or is present but below 1492, then the existing MRU constraint of 1492 octets MUST stay applicable, thus preserving backward compatibility. (§5.1) | no test | no test carries this requirement id; annotated {rollup}: RFC2516-x-9; the constraint that stays applicable is the RFC 2516 Section 7 ceiling |
| [`RFC4638-5.2-2`](#rfc4638-5.2-2) If these packets receive replies, the sending side MUST not send packets bigger than 1492 octets for this session. (§5.2) | {gap}, no test | ze negotiates no MRU above 1492 (RFC4638-3-1) and runs no MRU-sized Echo-Request test, so the fallback this row governs does not exist |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4638-3-1`](#rfc4638-3-1)

Since next-generation broadband networks are built around Ethernet systems supporting baby-giants and jumbo frames with payload sizes larger than the normal Ethernet MTU of 1500 octets, a BRAS acting as a PPPoE server MUST support PPPoE MRU negotiations larger than 1492 octets in order to limit the amount of fragmented packets in networks similar to those described in Section 1. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-3-1, so no unit is bound to it.

### [`RFC4638-3-2`](#rfc4638-3-2)

By default, the Maximum-Receive-Unit (MRU) option MUST follow the rules set forward in RFC 1661 [2] but MUST NOT be negotiated to a size larger than 1492 to guarantee compatibility with Ethernet network segments limited to 1500-octet frames. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-3-2, so no unit is bound to it.

### [`RFC4638-3-3`](#rfc4638-3-3)

In such a case, as the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MRU MUST NOT be greater than 1492. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-3-3, so no unit is bound to it.

### [`RFC4638-4-1`](#rfc4638-4-1)

If a PPPoE client wants to use an MTU/MRU higher than 1492 octets, then it MUST include an optional PPP-Max-Payload Tag in the PADI and PADR packets. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-4-1, so no unit is bound to it.

### [`RFC4638-4-2`](#rfc4638-4-2)

If the PPPoE server can support an MTU/MRU higher than 1492 octets, it MUST respond with an echo of the clients tag in the PADO and PADS packets when the PPP-Max-Payload tag is received from the client. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-4-2, so no unit is bound to it.

### [`RFC4638-5.1-1`](#rfc4638-5.1-1)

Since Ethernet (without jumbo frames) has a maximum payload size of 1500 octets, the PPPoE header is 6 octets, and the PPP Protocol ID is 2 octets, the Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a size larger than 1492, unless both the PPPoE client and server have indicated the ability to support a larger MRU in the PPPoE Discovery Stage. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-5.1-1, so no unit is bound to it.

### [`RFC4638-5.1-2`](#rfc4638-5.1-2)

The initial MRU negotiation for the PPP/PPPoE server MUST follow a flow as shown below: (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-5.1-2, so no unit is bound to it.

### [`RFC4638-5.1-3`](#rfc4638-5.1-3)

If the PPP-Max-Payload tag is present and greater than 1492, it MUST be considered along with the server's interface MTU settings when the maximum value is selected for the normal RFC 1661 [2] MRU negotiation which decides the actual MRU to use. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-5.1-3, so no unit is bound to it.

### [`RFC4638-5.1-4`](#rfc4638-5.1-4)

If the PPP-Max-Payload tag isn't present or is present but below 1492, then the existing MRU constraint of 1492 octets MUST stay applicable, thus preserving backward compatibility. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-5.1-4, so no unit is bound to it.

### [`RFC4638-5.2-2`](#rfc4638-5.2-2)

If these packets receive replies, the sending side MUST not send packets bigger than 1492 octets for this session. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4638-5.2-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4638.txt |
| Source fingerprint | 6f81020d0eba25ab |
| Record | rfc/extraction/rfc4638.json |
| Mapped sentences | 10 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 3 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 4 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 4638 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4638, so its obligations are stated where they were written.
