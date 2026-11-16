# RFC 4638 - Accommodating a Maximum Transit Unit/Maximum Receive Unit (MTU/MRU) Greater Than 1492 in the Point-to-Point Protocol over Ethernet (PPPoE)

PPPoE MTU/MRU above 1492. Every requirement listed for RFC 4638, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 6 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 13 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | PPPoE MTU/MRU above 1492 |
| Automatic checks | Included |
| Requirements | 13 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc4638.md` |
| Requirement table | `rfc/requirements/rfc4638.md` |
| RFC text | `rfc/full/rfc4638.txt` |

## Automatic checks

Included in automatic checks: PPPoE MRU/MTU above 1492: the PPP-Max-Payload discovery tag and the LCP MRU ceiling it lifts. Every MUST-level row is classified: the 1492 ceiling rows roll up to RFC 2516 and RFC 1661, the server-side larger-MRU rows and the client tag row (RFC4638-4-1) are gaps owned by plan/release/fixes/spec-pppoe-rfc4638-max-payload.md, because no owner ruling names an MTU above 1492 unwanted on either side (ai/META-DECISIONS.md, Goal and scope).

## Published support status

**Area:** PPPoE MTU/MRU above 1492

**Test verification:** 0 of 6 MUST proven

**Evidence:** -

**Unmet:** RFC4638-3-1, RFC4638-4-1, RFC4638-4-2, RFC4638-5.1-2, RFC4638-5.1-3, RFC4638-5.2-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 6 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 4 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (6):** [`RFC4638-3-1`](#rfc4638-3-1), [`RFC4638-4-1`](#rfc4638-4-1), [`RFC4638-4-2`](#rfc4638-4-2), [`RFC4638-5.1-2`](#rfc4638-5.1-2), [`RFC4638-5.1-3`](#rfc4638-5.1-3), [`RFC4638-5.2-2`](#rfc4638-5.2-2)

**Derived from other rows (4):** [`RFC4638-3-2`](#rfc4638-3-2), [`RFC4638-3-3`](#rfc4638-3-3), [`RFC4638-5.1-1`](#rfc4638-5.1-1), [`RFC4638-5.1-4`](#rfc4638-5.1-4)

## Figures

- **MUST bar**: Not built 6
- **Overview bar**: Not assessed 3, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4638.json](../../../data/rfc/rfc4638.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4638-3-1`](#rfc4638-3-1)

Since next-generation broadband networks are built around Ethernet systems supporting baby-giants and jumbo frames with payload sizes larger than the normal Ethernet MTU of 1500 octets, a BRAS acting as a PPPoE server MUST support PPPoE MRU negotiations larger than 1492 octets in order to limit the amount of fragmented packets in networks similar to those described in Section 1. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-3-1, so there is no linked test.

### [`RFC4638-3-2`](#rfc4638-3-2)

By default, the Maximum-Receive-Unit (MRU) option MUST follow the rules set forward in RFC 1661 [2] but MUST NOT be negotiated to a size larger than 1492 to guarantee compatibility with Ethernet network segments limited to 1500-octet frames. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-3-2, so there is no linked test.

### [`RFC4638-3-3`](#rfc4638-3-3)

In such a case, as the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MRU MUST NOT be greater than 1492. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-3-3, so there is no linked test.

### [`RFC4638-4-1`](#rfc4638-4-1)

If a PPPoE client wants to use an MTU/MRU higher than 1492 octets, then it MUST include an optional PPP-Max-Payload Tag in the PADI and PADR packets. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-4-1, so there is no linked test.

### [`RFC4638-4-2`](#rfc4638-4-2)

If the PPPoE server can support an MTU/MRU higher than 1492 octets, it MUST respond with an echo of the clients tag in the PADO and PADS packets when the PPP-Max-Payload tag is received from the client. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-4-2, so there is no linked test.

### [`RFC4638-5.1-1`](#rfc4638-5.1-1)

Since Ethernet (without jumbo frames) has a maximum payload size of 1500 octets, the PPPoE header is 6 octets, and the PPP Protocol ID is 2 octets, the Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a size larger than 1492, unless both the PPPoE client and server have indicated the ability to support a larger MRU in the PPPoE Discovery Stage. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-5.1-1, so there is no linked test.

### [`RFC4638-5.1-2`](#rfc4638-5.1-2)

The initial MRU negotiation for the PPP/PPPoE server MUST follow a flow as shown below: (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-5.1-2, so there is no linked test.

### [`RFC4638-5.1-3`](#rfc4638-5.1-3)

If the PPP-Max-Payload tag is present and greater than 1492, it MUST be considered along with the server's interface MTU settings when the maximum value is selected for the normal RFC 1661 [2] MRU negotiation which decides the actual MRU to use. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-5.1-3, so there is no linked test.

### [`RFC4638-5.1-4`](#rfc4638-5.1-4)

If the PPP-Max-Payload tag isn't present or is present but below 1492, then the existing MRU constraint of 1492 octets MUST stay applicable, thus preserving backward compatibility. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-5.1-4, so there is no linked test.

### [`RFC4638-5.2-2`](#rfc4638-5.2-2)

If these packets receive replies, the sending side MUST not send packets bigger than 1492 octets for this session. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4638-5.2-2, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 3 | reviewed | not stated |
| `4` | not stated | 2 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 4 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 4638 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4638, so its obligations are stated where they were written.
