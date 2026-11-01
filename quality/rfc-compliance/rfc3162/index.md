# RFC 3162 - RADIUS and IPv6

RADIUS IPv6 attributes. Every requirement this repository extracted from RFC 3162, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3 | of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 3 | 3 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 6 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3 | of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 6 gated MUSTs: they add up to 6. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | RADIUS IPv6 attributes |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 0 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc3162.md` |
| Requirement shard | `rfc/requirements/rfc3162.md` |
| RFC text | `rfc/full/rfc3162.txt` |

## Enrolment

Enrolled: Ze is a RADIUS NAS for L2TP subscribers (internal/component/l2tp/plugins/authradius) and so the client this document addresses. Walked site by site on 2026-10-26 (rfc/extraction/rfc3162.json). Every MUST is classified: RFC3162-2.1-1, RFC3162-2.5-1 and RFC3162-2.6-1 are met by the producers they name and owe tests; RFC3162-3.2-1 (no Framed-Interface-Id in a re-authentication Access-Request), RFC3162-2.3-1 (no Framed-IPv6-Prefix encoding) and RFC3162-5-1 (no zero-length secret over IPsec ESP) are gaps owned by plan/release/fixes/spec-radius-rfc3162-ipv6-attributes.md, because no owner ruling names those features unwanted (ai/META-DECISIONS.md, Goal and scope).

## What the public ledger says

**Area:** RADIUS IPv6 attributes

**Proof:** 3 of 6 MUST proven

**Evidence:** unit 3

**Unmet:** RFC3162-2.3-1, RFC3162-3.2-1, RFC3162-5-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (6):** [`RFC3162-2.1-1`](#rfc3162-2.1-1), [`RFC3162-3.2-1`](#rfc3162-3.2-1), [`RFC3162-2.3-1`](#rfc3162-2.3-1), [`RFC3162-2.5-1`](#rfc3162-2.5-1), [`RFC3162-2.6-1`](#rfc3162-2.6-1), [`RFC3162-5-1`](#rfc3162-5-1)

## Figures

- **MUST bar**: Ze proven 3, Ze correctness gap 3
- **Overview bar**: Ze MUST proven 3, SHOULD not decided 7, Ze correctness gap 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze proven | 3 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 3 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 7 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 7 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc3162.json](../../../data/rfc/rfc3162.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3162-2.1-1` | NAS-IPv6- Address and/or NAS-IP-Address MAY be present in an Access-Request packet; however, if neither attribute is present then NAS- Identifier MUST be present. (§2.1) | MUST | 2.1 - NAS-IPv6-Address | **positive:** `unit/verify` [`TestRFC3162AccessRequestFromAnIPv6SourceNamesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L29). **negative:** no negative test. **{single-polarity}:** the fallback is an addition to the packet, so its other direction, a configured NAS-IP-Address making the NAS-Identifier optional, is RFC 2865 Section 4.1 behaviour proven by TestRFC2866AcctNASIdentityFallbackIsNarrow under RFC2866-4.1-3, not a separate obligation of this sentence |
| `RFC3162-3.2-1` | If the Interface-Identifier IPv6CP option [11] has been successfully negotiated, this Attribute MUST be included in an Access-Request packet as a hint by the NAS to the server that it would prefer that value. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never sends Framed-Interface-Id: internal/component/l2tp/plugins/authradius/handler.go::buildAccessRequestAttrs writes no such attribute, and with authentication reauth-interval set, internal/component/l2tp/ppp/session_run.go runs runAuthPhase again on an established CHAP session, so a re-authentication Access-Request leaves after IPv6CP negotiated an Interface-Identifier and omits it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC3162-2.3-1` | This field, which is reserved and MUST be present, is always set to zero. (§2.3) | MUST | 2.3 - Framed-IPv6-Prefix | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements no Framed-IPv6-Prefix encoding: internal/component/l2tp/plugins/authradius/handler.go::buildAccessRequestAttrs and internal/component/l2tp/plugins/authradius/acct.go::buildAcctPacket write none, so no Reserved field is ever written; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC3162-2.5-1` | It is intended to be human readable and MUST NOT affect operation of the protocol. (§2.5) | MUST NOT | 2.5 - Framed-IPv6-Route | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC3162FramedIPv6RouteTextDoesNotStopTheAccept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L57). **{single-polarity}:** the obligation is that the text affects nothing outside its own route, and no well-formed or malformed text exists for which stopping the Access-Accept would be correct |
| `RFC3162-2.6-1` | If a NAS does not support multiple prefix pools, the NAS MUST ignore this Attribute. (§2.6) | MUST | 2.6 - Framed-IPv6-Pool | **positive:** `unit/verify` [`TestRFC3162FramedIPv6PoolIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L82). **negative:** no negative test. **{single-polarity}:** ze supports no prefix pools in any configuration, so the attribute is ignored unconditionally and there is no case in which acting on it would be correct |
| `RFC3162-5-1` | In this case, a shared secret of zero length MUST be assumed. (§5) | MUST | 5 - Security Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/radius/client.go::Exchange refuses an empty shared secret (RFC 2865 Section 3), so RADIUS run over IPsec ESP with no configured secret is refused rather than run with a zero-length secret; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC3162-2.1-2` | This Attribute indicates the identifying IPv6 Address of the NAS which is requesting authentication of the user, and SHOULD be unique to the NAS within the scope of the RADIUS server. (§2.1) | SHOULD | 2.1 - NAS-IPv6-Address | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-2.5-2` | Whenever the gateway address is the IPv6 unspecified address the IP address of the user SHOULD be used as the gateway address. (§2.5) | SHOULD | 2.5 - Framed-IPv6-Route | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-2.6-2` | This Attribute contains the name of an assigned pool that SHOULD be used to assign an IPv6 prefix for the user. (§2.6) | SHOULD | 2.6 - Framed-IPv6-Pool | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-3.2-2` | It MAY be used in Access-Accept packets. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-2.3-2` | It MAY be used in Access-Accept packets, and can appear multiple times. (§2.3) | MAY | 2.3 - Framed-IPv6-Prefix | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-2.3-3` | It MAY be used in an Access-Request packet as a hint by the NAS to the server that it would prefer these prefix(es), but the server is not required to honor the hint. (§2.3) | MAY | 2.3 - Framed-IPv6-Prefix | **positive:** no positive test. **negative:** no negative test |
| `RFC3162-2.4-1` | It MAY be used in an Access- Request packet as a hint to the server that the NAS would prefer to use that host, but the server is not required to honor the hint. (§2.4) | MAY | 2.4 - Login-IPv6-Host | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3162-3.2-1`](#rfc3162-3.2-1) If the Interface-Identifier IPv6CP option [11] has been successfully negotiated, this Attribute MUST be included in an Access-Request packet as a hint by the NAS to the server that it would prefer that value. (§3.2) | {gap}, no test | Ze never sends Framed-Interface-Id: internal/component/l2tp/plugins/authradius/handler.go::buildAccessRequestAttrs writes no such attribute, and with authentication reauth-interval set, internal/component/l2tp/ppp/session_run.go runs runAuthPhase again on an established CHAP session, so a re-authentication Access-Request leaves after IPv6CP negotiated an Interface-Identifier and omits it; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC3162-2.3-1`](#rfc3162-2.3-1) This field, which is reserved and MUST be present, is always set to zero. (§2.3) | {gap}, no test | Ze implements no Framed-IPv6-Prefix encoding: internal/component/l2tp/plugins/authradius/handler.go::buildAccessRequestAttrs and internal/component/l2tp/plugins/authradius/acct.go::buildAcctPacket write none, so no Reserved field is ever written; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC3162-5-1`](#rfc3162-5-1) In this case, a shared secret of zero length MUST be assumed. (§5) | {gap}, no test | internal/component/radius/client.go::Exchange refuses an empty shared secret (RFC 2865 Section 3), so RADIUS run over IPsec ESP with no configured secret is refused rather than run with a zero-length secret; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3162-2.1-1`](#rfc3162-2.1-1)

NAS-IPv6- Address and/or NAS-IP-Address MAY be present in an Access-Request packet; however, if neither attribute is present then NAS- Identifier MUST be present. (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3162AccessRequestFromAnIPv6SourceNamesTheNAS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L29) | unit/verify | revert, verified |

### [`RFC3162-3.2-1`](#rfc3162-3.2-1)

If the Interface-Identifier IPv6CP option [11] has been successfully negotiated, this Attribute MUST be included in an Access-Request packet as a hint by the NAS to the server that it would prefer that value. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3162-3.2-1, so no unit is bound to it.

### [`RFC3162-2.3-1`](#rfc3162-2.3-1)

This field, which is reserved and MUST be present, is always set to zero. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3162-2.3-1, so no unit is bound to it.

### [`RFC3162-2.5-1`](#rfc3162-2.5-1)

It is intended to be human readable and MUST NOT affect operation of the protocol. (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3162FramedIPv6RouteTextDoesNotStopTheAccept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L57) | unit/verify | revert, verified |

### [`RFC3162-2.6-1`](#rfc3162-2.6-1)

If a NAS does not support multiple prefix pools, the NAS MUST ignore this Attribute. (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3162FramedIPv6PoolIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc3162_nas_attributes_test.go#L82) | unit/verify | revert, verified |

### [`RFC3162-5-1`](#rfc3162-5-1)

In this case, a shared secret of zero length MUST be assumed. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3162-5-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc3162.txt |
| Source fingerprint | 56fa0fda2bd77411 |
| Record | rfc/extraction/rfc3162.json |
| Mapped sentences | 6 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract and table of contents. |
| `1` | Introduction | 0 | walked | Introduction. States the purpose of the six attributes; no normative keyword. |
| `1.1` | Requirements language | 0 | walked | Requirements language. RFC 2119 boilerplate only. |
| `2` | Attributes | 0 | walked | Attributes. Introduces the six attribute sections; no keyword of its own. |
| `2.1` | NAS-IPv6-Address | 1 | walked | NAS-IPv6-Address. One MUST (NAS-Identifier when neither address is present), one SHOULD on uniqueness and one MAY on presence, all captured. |
| `3.2` | not stated | 1 | walked | Framed-Interface-Id, printed as 3.2 in the RFC where 2.2 was meant. One conditional MUST captured as RFC3162-3.2-1 and one MAY captured as RFC3162-3.2-2. |
| `2.3` | Framed-IPv6-Prefix | 1 | walked | Framed-IPv6-Prefix. The Reserved-field MUST and two MAYs are captured. |
| `2.4` | Login-IPv6-Host | 0 | walked | Login-IPv6-Host. Two MAYs; the request hint is captured as RFC3162-2.4-1. No MUST. |
| `2.5` | Framed-IPv6-Route | 1 | walked | Framed-IPv6-Route. The text MUST NOT and the SHOULD on an unspecified gateway are captured; the text format SHOULD is described in the summary. |
| `2.6` | Framed-IPv6-Pool | 1 | walked | Framed-IPv6-Pool. The MUST to ignore and the SHOULD to use the pool are captured. |
| `3` | Table of Attributes | 0 | walked | Table of Attributes. The quantity table is reproduced in the summary; it carries no legend with keywords. |
| `4` | References | 0 | skipped (references) | References. |
| `5` | Security Considerations | 1 | walked | Security Considerations. The zero-length secret MUST is captured as RFC3162-5-1. |
| `6` | IANA Considerations: types 95 to 100 | 0 | skipped (iana) | IANA Considerations: types 95 to 100. |
| `7` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `8` | Authors' Addresses and the full copyright statement | 0 | skipped (front-matter) | Authors' Addresses and the full copyright statement. |

### Excluded sentences

The walk over RFC 3162 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 3162, so its obligations are stated where they were written.
