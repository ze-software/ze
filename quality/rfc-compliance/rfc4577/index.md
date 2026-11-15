# RFC 4577 - OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs)

OSPF as the PE/CE protocol for BGP/MPLS IP VPNs. Every requirement listed for RFC 4577, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 46 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 46 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 46 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 46 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 1 | 1 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 45 | of 46 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 46 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 46 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 46 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 46 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 46 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 46 | of 66 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 46 mandatory requirements: they add up to 46. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPF as the PE/CE protocol for BGP/MPLS IP VPNs |
| Automatic checks | Included |
| Requirements | 66 |
| Mandatory requirements (MUST level) | 46 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 63 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 3 |
| Fault test records | 1 |
| Summary | `rfc/short/rfc4577.md` |
| Requirement table | `rfc/requirements/rfc4577.md` |
| RFC text | `rfc/full/rfc4577.txt` |

## Automatic checks

Included in automatic checks: OSPF as the PE/CE Protocol for BGP/MPLS IP VPNs

## Published support status

**Area:** OSPF as the PE/CE protocol for BGP/MPLS IP VPNs

**Proof:** 0 of 46 MUST proven

**Evidence:** -

**Unmet:** RFC4577-4.1.1-1, RFC4577-4.1.1-2, RFC4577-4.1.1-3, RFC4577-4.1.3-1, RFC4577-4.1.4-1, RFC4577-4.2.1-1, RFC4577-4.2.1-2, RFC4577-4.2.4-1, RFC4577-4.2.4-2, RFC4577-4.2.4-3, RFC4577-4.2.4-4, RFC4577-4.2.4-5, RFC4577-4.2.4-6, RFC4577-4.2.4-7, RFC4577-4.2.4-8, RFC4577-4.2.5.1-1, RFC4577-4.2.5.1-2, RFC4577-4.2.5.1-3, RFC4577-4.2.5.1-4, RFC4577-4.2.5.2-1, RFC4577-4.2.5.2-10, RFC4577-4.2.5.2-2, RFC4577-4.2.5.2-3, RFC4577-4.2.5.2-4, RFC4577-4.2.5.2-5, RFC4577-4.2.5.2-6, RFC4577-4.2.5.2-7, RFC4577-4.2.5.2-8, RFC4577-4.2.5.2-9, RFC4577-4.2.6-1, RFC4577-4.2.6-10, RFC4577-4.2.6-11, RFC4577-4.2.6-12, RFC4577-4.2.6-13, RFC4577-4.2.6-14, RFC4577-4.2.6-15, RFC4577-4.2.6-16, RFC4577-4.2.6-2, RFC4577-4.2.6-3, RFC4577-4.2.6-4, RFC4577-4.2.6-6, RFC4577-4.2.6-7, RFC4577-4.2.6-8, RFC4577-4.2.6-9, RFC4577-4.2.7.1-1, RFC4577-4.2.7.1-2, RFC4577-4.2.7.1-3, RFC4577-4.2.7.1-4, RFC4577-4.2.7.1-5, RFC4577-4.2.7.2-1, RFC4577-4.2.7.3-1, RFC4577-4.2.7.3-2, RFC4577-4.2.7.3-3, RFC4577-4.2.7.3-4, RFC4577-4.2.7.4-1, RFC4577-4.2.7.4-2, RFC4577-4.2.7.4-3, RFC4577-4.2.7.4-4, RFC4577-4.2.8.1-1, RFC4577-4.2.8.1-2, RFC4577-4.2.8.1-3, RFC4577-4.2.8.1-4, RFC4577-4.2.8.1-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 45 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **46** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC4577-6-1`](#rfc4577-6-1)

**Annotated (including scoped evidence) (45):** [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1), [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1), [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2), [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2), [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1), [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2), [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3), [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4), [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5), [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1), [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2), [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1), [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2), [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1), [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3), [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4), [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3), [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1), [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2), [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3), [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4), [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5), [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6), [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7), [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2), [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1), [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1), [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2), [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3), [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1), [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1), [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1), [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2), [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3), [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3), [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10), [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13), [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14), [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15), [`RFC4577-4.2.6-16`](#rfc4577-4.2.6-16), [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5), [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3), [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4), [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4), [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5)

## Figures

- **MUST bar**: Proof pending 1, Not built 45
- **Overview bar**: SHOULD proven 1, Proof pending 2, Not built 63

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 46 |
| MUST rows kept (the MUST bar) | 46 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 45 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 20 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 18 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4577.json](../../../data/rfc/rfc4577.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4577-4.1.1-1` | A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of an independent OSPF instance for each OSPF domain a PE attaches to; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.1-1` | The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of one OSPF instance for each OSPF domain a PE attaches to; internal/plugins/ospf/config.go carries RFC 6549 Instance IDs on one ordinary routing domain, not VPN OSPF domains, and the PE is a role Ze owes, so the obligation binds Ze |
| `RFC4577-4.2.1-2` | Each instance of OSPF MUST be associated with a single VRF. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of binding each OSPF instance to a single VRF; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.1.1-2` | If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of tying the interfaces of one OSPF instance to one VRF; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-1` | Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of associating an OSPF instance with Domain Identifiers; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-2` | Each OSPF instance MUST be associated with one or more Domain Identifiers. This MUST be configurable (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of configurable Domain Identifiers; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-3` | If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a configurable primary Domain Identifier; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-4` | If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4) | MUST NOT | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of refusing the NULL Domain Identifier among several; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-5` | If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of carrying the primary Domain Identifier Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-1` | The OSPF Domain Identifier Extended Communities attribute. If the OSPF instance that installed the route has a non-NULL primary Domain Identifier, this MUST be present (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of the OSPF Domain Identifier Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-2` | OSPF Route Type Extended Communities Attribute. This attribute MUST be present. (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of the OSPF Route Type Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.1-1` | When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of setting the DN bit in Type 3 LSAs a PE sends to a CE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.1-2` | When a PE router needs to distribute to a CE router a route that comes from a site outside the latter's OSPF domain, the PE router presents itself as an ASBR (Autonomous System Border Router), and distributes the route in a type 5 LSA. The DN bit [OSPF-DN] MUST be set in these LSAs to ensure that they will be ignored by any other PE routers that receive them. (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of presenting the PE as an ASBR and setting the DN bit in Type 5 LSAs for routes from outside the CE's domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.8.1-1` | The DN bit (Section 4.2.5.1) MUST be set in the LSA. (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of setting the DN bit in LSAs originated from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-3` | When a PE router receives, from a CE router, any LSA with the DN bit [OSPF-DN] set, the information from that LSA MUST NOT be used by the route calculation. (§4.2.6) | MUST NOT | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of leaving LSAs a CE sends with the DN bit set out of the PE route calculation; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-4` | If a Type 5 LSA is received from the CE, and if it has an OSPF route tag value equal to the VPN Route Tag (see Section 4.2.5.2), then the information from that LSA MUST NOT be used by the route calculation. (§4.2.6) | MUST NOT | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of leaving Type 5 LSAs from a CE whose tag equals the VPN Route Tag out of the PE route calculation; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.1-3` | To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of the VPN Route Tag procedures on by default; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-1` | If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a VPN Route Tag for each VRF associated with an OSPF instance; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-2` | By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of putting the VPN Route Tag in the Type 5 LSAs a PE originates from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-3` | The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. Its value MUST therefore be configurable. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a configurable VPN Route Tag; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-4` | If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of requiring a configured VPN Route Tag when the backbone AS number is four bytes long; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-5` | If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured, and it MUST be distinct from any Route Tag used within the VPN itself. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of requiring that configured VPN Route Tag to differ from every tag used in the VPN; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-6` | Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of the VPN Route Tag in each PE-originated Type 5 LSA; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-7` | The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of using the VPN Route Tag to stop a PE-originated Type 5 LSA reaching another PE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.8.1-2` | The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of placing the VPN Route Tag in external LSAs, with a configuration switch that turns it off; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.1.4-1` | If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3) | MUST | 4.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the missing proof is a test of Ze, as a CE, forming an area 0 adjacency to a PE, over a physical link or a virtual link; ze forms ordinary area 0 adjacencies, and internal/plugins/ospf/rfc2328_virtual_route_test.go:70 already tags RFC4577-4.1.4-2 for a CE backbone adjacency over a virtual link, but no test drives an area 0 adjacency against a PE |
| `RFC4577-4.2.7.1-1` | The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.1-2` | The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.1-3` | The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1) | MUST NOT | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.2-1` | The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2) | MUST NOT | 4.2.7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.3-1` | The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.4-1` | Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.4-2` | However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.4-3` | This same rule applies to any packet whose IP destination address is the remote endpoint address of a sham link. Such packets MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-6-1` | OSPF "cryptographic authentication" SHOULD be used between a PE and a CE. It MUST be implemented on each PE. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L12). **negative:** `unit/verify` [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L32) |
| `RFC4577-4.2.4-6` | the default value (if none is configured) SHOULD be NULL (§4.2.4) | SHOULD | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a NULL default Domain Identifier; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-8` | If the Autonomous System number of the VPN backbone is two bytes long, the default value SHOULD be an automatically computed tag based on that Autonomous System number (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of an automatic VPN Route Tag computed from a two-byte backbone AS number; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-9` | If a PE router needs to use OSPF to distribute to a CE router a route that comes from a site outside the CE router's OSPF domain, the PE router SHOULD present itself to the CE router as an Autonomous System Border Router (ASBR) and SHOULD report such routes as AS-external routes. (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of presenting the PE as an ASBR and reporting routes from outside the CE's domain as AS-external; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-6` | To ensure backward compatibility, the type 8000 SHOULD be accepted as well and treated as if it were type 0306. (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of accepting the legacy Route Type type 8000 as 0306; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-7` | The type 8001 SHOULD be accepted as well, to ensure backward compatibility, and should be treated as if it were 0107. (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of accepting the legacy Router ID type 8001 as 0107; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-8` | MED (Multi_EXIT_DISC attribute). By default, this SHOULD be set to the value of the OSPF distance associated with the route, plus 1. (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a default MED of the OSPF distance plus 1 on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.7.3-2` | Sham links SHOULD be treated by OSPF as OSPF Demand Circuits. (§4.2.7.3) | SHOULD | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.4-4` | If a PE determines that the next hop interface for a particular route is a sham link, then the PE SHOULD NOT redistribute that route into BGP as a VPN-IPv4 route. (§4.2.7.4) | SHOULD NOT | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-6-2` | OSPF "cryptographic authentication" SHOULD be used between a PE and a CE. (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC4577ConfiguredLinkSendsAuthenticatedHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_link_auth_test.go#L88). **negative:** no negative test. **{single-polarity}:** the negative -- an interface with no key chain sending unauthenticated OSPF packets -- is the state this SHOULD recommends against, not a behavior the implementation must exhibit, so asserting it would pin the unrecommended path instead of the requirement |
| `RFC4577-4.2.4-7` | If the OSPF instance's Domain Identifier is NULL, the Domain Identifier Extended Communities attribute MAY be omitted when routes from that OSPF instance are distributed by BGP (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of omitting the Domain Identifier attribute for a NULL domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.4-8` | alternatively, a value of the Domain Identifier Extended Communities attribute that represents NULL (see Section 4.2.4) MAY be carried with the route. (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of carrying a NULL Domain Identifier attribute; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-9` | The OSPF Domain Identifier Extended Communities attribute. If the OSPF instance that installed the route has a non-NULL primary Domain Identifier, this MUST be present; if that OSPF instance has only a NULL Domain Identifier, it MAY be omitted. (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of choosing whether the Domain Identifier attribute is present from the instance's domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-10` | For backward compatibility, the type 8005 MAY be used as well and is treated as if it were 0005. (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of accepting the legacy Domain Identifier type 8005 as 0005; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.8.1-3` | The default type 1 metric and the default type 2 metric MAY be different. (§4.2.8.1) | MAY | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of separate default type 1 and type 2 metrics for LSAs originated from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.1.4-2` | This adjacency MAY be via an OSPF virtual link. (The ability to use an OSPF virtual link in this way is an OPTIONAL feature.) (§4.1.4) | MAY | 4.1.4 | **positive:** `unit/verify` [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L70). **negative:** no negative test. **{single-polarity}:** this is a MAY, the permission to reach the area 0 adjacency over a virtual link. A negative would have to assert that some configuration does NOT yield a backbone virtual link, which exercises virtual-link configuration handling rather than the permission this requirement grants |
| `RFC4577-4.2.5.1-4` | When it is no longer necessary to use the VPN Route Tag in a particular deployment, its use (both sending and receiving) may be disabled by configuration. (§4.2.5.1) | MAY | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of a configuration switch that disables sending and receiving the VPN Route Tag; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.1.3-1` | Sham links are an OPTIONAL feature of this specification (§4.1.3, §4.2.7) | OPTIONAL | 4.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.6-11` | OSPF Router ID Extended Communities Attribute. This OPTIONAL attribute specifies the OSPF Router ID of the system that is identified in the BGP Next Hop attribute. (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of the OSPF Router ID Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-12` | The Site of Origin attribute, which is usually required by [VPN], is OPTIONAL for routes that a PE learns from a CE via OSPF. (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of Site of Origin handling for routes a PE learns from a CE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.7.1-4` | If the VRF is associated with only a single OSPF instance, and if the PE's router id in that OSPF instance is an IP address, then the Sham Link Endpoint Address MAY default to that Router ID. (§4.2.7.1) | MAY | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.1.1-3` | If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running. (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of keeping the PE's IGP OSPF instance separate from its VPN OSPF instances; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.5.2-10` | The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of keeping the VPN Route Tag distinct from the route tags used in the OSPF domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-13` | For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP. (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of creating a VPN-IPv4 route in BGP for each prefix a VRF's OSPF instance installs; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-14` | If the OSPF instance has a NULL Domain Identifier, and the OSPF Domain Identifier Extended Communities attribute is present, then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, 0205, or 8005. (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of an all-zero value field for a NULL Domain Identifier attribute; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-15` | 5 for external routes (area number must be 0). (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of Route Type 5 with area number 0 on exported external routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.6-16` | Routes that a PE receives in type 4 LSAs MUST NOT be redistributed to BGP. (§4.2.6) | MUST NOT | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of exporting a VRF's OSPF routes to BGP as VPN-IPv4 routes, so there is no PE export for this restriction to apply to; internal/plugins/ospf/redistribute/source.go::emitDelta exports ordinary OSPF routes to BGP as plain ipv4/unicast with no VRF and no VPN-IPv4 route. The PE role is owed work: no owner decision declines it. This obligation was row RFC4577-4.2.6-5, retired 2026-10-02 on a role decision the owner never made (rfc/corrections/rfc4577.md) |
| `RFC4577-4.2.7.1-5` | If two VRFs are to be connected by a sham link, each VRF must be associated with a "Sham Link Endpoint Address", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF. (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.3-3` | An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver. (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.7.3-4` | The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately. (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| `RFC4577-4.2.8.1-4` | In general, when two such attributes are compared, all eight bytes must be compared. (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of comparing all eight bytes of two Domain Identifier attributes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| `RFC4577-4.2.8.1-5` | If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain. (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not implement the PE procedure of deciding whether a VPN-IPv4 route and an OSPF instance belong to the same domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1) A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1) | {gap}, no test | Ze does not implement the PE procedure of an independent OSPF instance for each OSPF domain a PE attaches to; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1) The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1) | {gap}, no test | Ze does not implement the PE procedure of one OSPF instance for each OSPF domain a PE attaches to; internal/plugins/ospf/config.go carries RFC 6549 Instance IDs on one ordinary routing domain, not VPN OSPF domains, and the PE is a role Ze owes, so the obligation binds Ze |
| [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2) Each instance of OSPF MUST be associated with a single VRF. (§4.2.1) | {gap}, no test | Ze does not implement the PE procedure of binding each OSPF instance to a single VRF; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2) If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1) | {gap}, no test | Ze does not implement the PE procedure of tying the interfaces of one OSPF instance to one VRF; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1) Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4) | {gap}, no test | Ze does not implement the PE procedure of associating an OSPF instance with Domain Identifiers; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2) Each OSPF instance MUST be associated with one or more Domain Identifiers. This MUST be configurable (§4.2.4) | {gap}, no test | Ze does not implement the PE procedure of configurable Domain Identifiers; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3) If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4) | {gap}, no test | Ze does not implement the PE procedure of a configurable primary Domain Identifier; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4) If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4) | {gap}, no test | Ze does not implement the PE procedure of refusing the NULL Domain Identifier among several; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5) If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4) | {gap}, no test | Ze does not implement the PE procedure of carrying the primary Domain Identifier Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1) The OSPF Domain Identifier Extended Communities attribute. If the OSPF instance that installed the route has a non-NULL primary Domain Identifier, this MUST be present (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of the OSPF Domain Identifier Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2) OSPF Route Type Extended Communities Attribute. This attribute MUST be present. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of the OSPF Route Type Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1) When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1) | {gap}, no test | Ze does not implement the PE procedure of setting the DN bit in Type 3 LSAs a PE sends to a CE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2) When a PE router needs to distribute to a CE router a route that comes from a site outside the latter's OSPF domain, the PE router presents itself as an ASBR (Autonomous System Border Router), and distributes the route in a type 5 LSA. The DN bit [OSPF-DN] MUST be set in these LSAs to ensure that they will be ignored by any other PE routers that receive them. (§4.2.5.1) | {gap}, no test | Ze does not implement the PE procedure of presenting the PE as an ASBR and setting the DN bit in Type 5 LSAs for routes from outside the CE's domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1) The DN bit (Section 4.2.5.1) MUST be set in the LSA. (§4.2.8.1) | {gap}, no test | Ze does not implement the PE procedure of setting the DN bit in LSAs originated from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3) When a PE router receives, from a CE router, any LSA with the DN bit [OSPF-DN] set, the information from that LSA MUST NOT be used by the route calculation. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of leaving LSAs a CE sends with the DN bit set out of the PE route calculation; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4) If a Type 5 LSA is received from the CE, and if it has an OSPF route tag value equal to the VPN Route Tag (see Section 4.2.5.2), then the information from that LSA MUST NOT be used by the route calculation. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of leaving Type 5 LSAs from a CE whose tag equals the VPN Route Tag out of the PE route calculation; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3) To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1) | {gap}, no test | Ze does not implement the PE procedure of the VPN Route Tag procedures on by default; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1) If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of a VPN Route Tag for each VRF associated with an OSPF instance; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2) By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of putting the VPN Route Tag in the Type 5 LSAs a PE originates from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3) The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. Its value MUST therefore be configurable. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of a configurable VPN Route Tag; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4) If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of requiring a configured VPN Route Tag when the backbone AS number is four bytes long; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5) If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured, and it MUST be distinct from any Route Tag used within the VPN itself. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of requiring that configured VPN Route Tag to differ from every tag used in the VPN; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6) Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of the VPN Route Tag in each PE-originated Type 5 LSA; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7) The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of using the VPN Route Tag to stop a PE-originated Type 5 LSA reaching another PE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2) The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1) | {gap}, no test | Ze does not implement the PE procedure of placing the VPN Route Tag in external LSAs, with a configuration switch that turns it off; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1) If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3) | {gap}, no test | the missing proof is a test of Ze, as a CE, forming an area 0 adjacency to a PE, over a physical link or a virtual link; ze forms ordinary area 0 adjacencies, and internal/plugins/ospf/rfc2328_virtual_route_test.go:70 already tags RFC4577-4.1.4-2 for a CE backbone adjacency over a virtual link, but no test drives an area 0 adjacency against a PE |
| [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1) The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2) The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3) The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1) The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1) The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1) Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2) However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3) This same rule applies to any packet whose IP destination address is the remote endpoint address of a sham link. Such packets MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.4-6`](#rfc4577-4.2.4-6) the default value (if none is configured) SHOULD be NULL (§4.2.4) | {gap} | Ze does not implement the PE procedure of a NULL default Domain Identifier; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-8`](#rfc4577-4.2.5.2-8) If the Autonomous System number of the VPN backbone is two bytes long, the default value SHOULD be an automatically computed tag based on that Autonomous System number (§4.2.5.2) | {gap} | Ze does not implement the PE procedure of an automatic VPN Route Tag computed from a two-byte backbone AS number; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-9`](#rfc4577-4.2.5.2-9) If a PE router needs to use OSPF to distribute to a CE router a route that comes from a site outside the CE router's OSPF domain, the PE router SHOULD present itself to the CE router as an Autonomous System Border Router (ASBR) and SHOULD report such routes as AS-external routes. (§4.2.5.2) | {gap} | Ze does not implement the PE procedure of presenting the PE as an ASBR and reporting routes from outside the CE's domain as AS-external; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-6`](#rfc4577-4.2.6-6) To ensure backward compatibility, the type 8000 SHOULD be accepted as well and treated as if it were type 0306. (§4.2.6) | {gap} | Ze does not implement the PE procedure of accepting the legacy Route Type type 8000 as 0306; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-7`](#rfc4577-4.2.6-7) The type 8001 SHOULD be accepted as well, to ensure backward compatibility, and should be treated as if it were 0107. (§4.2.6) | {gap} | Ze does not implement the PE procedure of accepting the legacy Router ID type 8001 as 0107; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-8`](#rfc4577-4.2.6-8) MED (Multi_EXIT_DISC attribute). By default, this SHOULD be set to the value of the OSPF distance associated with the route, plus 1. (§4.2.6) | {gap} | Ze does not implement the PE procedure of a default MED of the OSPF distance plus 1 on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.7.3-2`](#rfc4577-4.2.7.3-2) Sham links SHOULD be treated by OSPF as OSPF Demand Circuits. (§4.2.7.3) | {gap} | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.4-4`](#rfc4577-4.2.7.4-4) If a PE determines that the next hop interface for a particular route is a sham link, then the PE SHOULD NOT redistribute that route into BGP as a VPN-IPv4 route. (§4.2.7.4) | {gap} | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.4-7`](#rfc4577-4.2.4-7) If the OSPF instance's Domain Identifier is NULL, the Domain Identifier Extended Communities attribute MAY be omitted when routes from that OSPF instance are distributed by BGP (§4.2.4) | {gap} | Ze does not implement the PE procedure of omitting the Domain Identifier attribute for a NULL domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.4-8`](#rfc4577-4.2.4-8) alternatively, a value of the Domain Identifier Extended Communities attribute that represents NULL (see Section 4.2.4) MAY be carried with the route. (§4.2.4) | {gap} | Ze does not implement the PE procedure of carrying a NULL Domain Identifier attribute; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-9`](#rfc4577-4.2.6-9) The OSPF Domain Identifier Extended Communities attribute. If the OSPF instance that installed the route has a non-NULL primary Domain Identifier, this MUST be present; if that OSPF instance has only a NULL Domain Identifier, it MAY be omitted. (§4.2.6) | {gap} | Ze does not implement the PE procedure of choosing whether the Domain Identifier attribute is present from the instance's domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-10`](#rfc4577-4.2.6-10) For backward compatibility, the type 8005 MAY be used as well and is treated as if it were 0005. (§4.2.6) | {gap} | Ze does not implement the PE procedure of accepting the legacy Domain Identifier type 8005 as 0005; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.8.1-3`](#rfc4577-4.2.8.1-3) The default type 1 metric and the default type 2 metric MAY be different. (§4.2.8.1) | {gap} | Ze does not implement the PE procedure of separate default type 1 and type 2 metrics for LSAs originated from VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.1-4`](#rfc4577-4.2.5.1-4) When it is no longer necessary to use the VPN Route Tag in a particular deployment, its use (both sending and receiving) may be disabled by configuration. (§4.2.5.1) | {gap} | Ze does not implement the PE procedure of a configuration switch that disables sending and receiving the VPN Route Tag; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.1.3-1`](#rfc4577-4.1.3-1) Sham links are an OPTIONAL feature of this specification (§4.1.3, §4.2.7) | {gap} | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.6-11`](#rfc4577-4.2.6-11) OSPF Router ID Extended Communities Attribute. This OPTIONAL attribute specifies the OSPF Router ID of the system that is identified in the BGP Next Hop attribute. (§4.2.6) | {gap} | Ze does not implement the PE procedure of the OSPF Router ID Extended Community on exported VPN-IPv4 routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-12`](#rfc4577-4.2.6-12) The Site of Origin attribute, which is usually required by [VPN], is OPTIONAL for routes that a PE learns from a CE via OSPF. (§4.2.6) | {gap} | Ze does not implement the PE procedure of Site of Origin handling for routes a PE learns from a CE; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.7.1-4`](#rfc4577-4.2.7.1-4) If the VRF is associated with only a single OSPF instance, and if the PE's router id in that OSPF instance is an IP address, then the Sham Link Endpoint Address MAY default to that Router ID. (§4.2.7.1) | {gap} | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3) If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running. (§4.1.1) | {gap}, no test | Ze does not implement the PE procedure of keeping the PE's IGP OSPF instance separate from its VPN OSPF instances; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10) The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. (§4.2.5.2) | {gap}, no test | Ze does not implement the PE procedure of keeping the VPN Route Tag distinct from the route tags used in the OSPF domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13) For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of creating a VPN-IPv4 route in BGP for each prefix a VRF's OSPF instance installs; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14) If the OSPF instance has a NULL Domain Identifier, and the OSPF Domain Identifier Extended Communities attribute is present, then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, 0205, or 8005. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of an all-zero value field for a NULL Domain Identifier attribute; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15) 5 for external routes (area number must be 0). (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of Route Type 5 with area number 0 on exported external routes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.6-16`](#rfc4577-4.2.6-16) Routes that a PE receives in type 4 LSAs MUST NOT be redistributed to BGP. (§4.2.6) | {gap}, no test | Ze does not implement the PE procedure of exporting a VRF's OSPF routes to BGP as VPN-IPv4 routes, so there is no PE export for this restriction to apply to; internal/plugins/ospf/redistribute/source.go::emitDelta exports ordinary OSPF routes to BGP as plain ipv4/unicast with no VRF and no VPN-IPv4 route. The PE role is owed work: no owner decision declines it. This obligation was row RFC4577-4.2.6-5, retired 2026-10-02 on a role decision the owner never made (rfc/corrections/rfc4577.md) |
| [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5) If two VRFs are to be connected by a sham link, each VRF must be associated with a "Sham Link Endpoint Address", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF. (§4.2.7.1) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3) An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver. (§4.2.7.3) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4) The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately. (§4.2.7.3) | {gap}, no test | Ze does not implement sham links; §4.1.3 says "Sham links are an OPTIONAL feature of this specification and are used only when it is necessary to have the service provider's network treated as an intra-area link."; sham links are not built, and no owner decision declines them, so the feature is owed work: internal/plugins/ospf/config.go::parseVirtualLinks configures only RFC 2328 backbone virtual links through a transit area, keyed by the remote router ID, never a VRF-to-VRF sham link |
| [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4) In general, when two such attributes are compared, all eight bytes must be compared. (§4.2.8.1) | {gap}, no test | Ze does not implement the PE procedure of comparing all eight bytes of two Domain Identifier attributes; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |
| [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5) If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain. (§4.2.8.1) | {gap}, no test | Ze does not implement the PE procedure of deciding whether a VPN-IPv4 route and an OSPF instance belong to the same domain; internal/plugins/ospf/config.go::parseRedistribute configures only a plain redistribution source with its metric, metric type and route tag for one ordinary OSPF routing domain, with no VRF, no OSPF domain, no Domain Identifier and no VPN-IPv4 route, and no code under internal/plugins/ospf handles the DN bit or a VPN-IPv4 route. The PE role is owed work: no owner decision declines it |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1)

A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.1.1-1, so there is no linked test.

### [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1)

The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.1-1, so there is no linked test.

### [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2)

Each instance of OSPF MUST be associated with a single VRF. (§4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.1-2, so there is no linked test.

### [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2)

If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.1.1-2, so there is no linked test.

### [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1)

Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.4-1, so there is no linked test.

### [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2)

Each OSPF instance MUST be associated with one or more Domain Identifiers. This MUST be configurable (§4.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.4-2, so there is no linked test.

### [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3)

If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.4-3, so there is no linked test.

### [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4)

If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.4-4, so there is no linked test.

### [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5)

If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.4-5, so there is no linked test.

### [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1)

The OSPF Domain Identifier Extended Communities attribute. If the OSPF instance that installed the route has a non-NULL primary Domain Identifier, this MUST be present (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-1, so there is no linked test.

### [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2)

OSPF Route Type Extended Communities Attribute. This attribute MUST be present. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-2, so there is no linked test.

### [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1)

When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.1-1, so there is no linked test.

### [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2)

When a PE router needs to distribute to a CE router a route that comes from a site outside the latter's OSPF domain, the PE router presents itself as an ASBR (Autonomous System Border Router), and distributes the route in a type 5 LSA. The DN bit [OSPF-DN] MUST be set in these LSAs to ensure that they will be ignored by any other PE routers that receive them. (§4.2.5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.1-2, so there is no linked test.

### [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1)

The DN bit (Section 4.2.5.1) MUST be set in the LSA. (§4.2.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.8.1-1, so there is no linked test.

### [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3)

When a PE router receives, from a CE router, any LSA with the DN bit [OSPF-DN] set, the information from that LSA MUST NOT be used by the route calculation. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-3, so there is no linked test.

### [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4)

If a Type 5 LSA is received from the CE, and if it has an OSPF route tag value equal to the VPN Route Tag (see Section 4.2.5.2), then the information from that LSA MUST NOT be used by the route calculation. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-4, so there is no linked test.

### [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3)

To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.1-3, so there is no linked test.

### [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1)

If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-1, so there is no linked test.

### [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2)

By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-2, so there is no linked test.

### [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3)

The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. Its value MUST therefore be configurable. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-3, so there is no linked test.

### [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4)

If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-4, so there is no linked test.

### [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5)

If the Autonomous System number is four bytes long, then a Route Tag value MUST be configured, and it MUST be distinct from any Route Tag used within the VPN itself. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-5, so there is no linked test.

### [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6)

Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-6, so there is no linked test.

### [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7)

The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-7, so there is no linked test.

### [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2)

The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.8.1-2, so there is no linked test.

### [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1)

If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.1.4-1, so there is no linked test.

### [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1)

The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.1-1, so there is no linked test.

### [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2)

The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.1-2, so there is no linked test.

### [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3)

The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.1-3, so there is no linked test.

### [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1)

The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.2-1, so there is no linked test.

### [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1)

The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.3-1, so there is no linked test.

### [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1)

Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.4-1, so there is no linked test.

### [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2)

However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.4-2, so there is no linked test.

### [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3)

This same rule applies to any packet whose IP destination address is the remote endpoint address of a sham link. Such packets MUST be forwarded according to the corresponding BGP route. (§4.2.7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.4-3, so there is no linked test.

### [`RFC4577-6-1`](#rfc4577-6-1)

OSPF "cryptographic authentication" SHOULD be used between a PE and a CE. It MUST be implemented on each PE. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge): stale only because the 6-2 tag comment was removed from TestRFC4577CryptographicAuthImplemented; its body and the 6-1 tags are unchanged. Forbidden: a router without working OSPF cryptographic authentication. TestRFC4577CryptographicAuthImplemented signs a Hello with the configured key (md5 and hmac-sha-256) and asserts authStore.verify accepts it; TestRFC4577CryptographicAuthRejectsForgery asserts a tampered digest and a packet signed under another secret are refused, red on an accept-all verifier. The operator SHOULD is row 6-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L32) | unit/verify | unproven |
| positive | [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L12) | unit/verify | unproven |

### [`RFC4577-6-2`](#rfc4577-6-2)

OSPF "cryptographic authentication" SHOULD be used between a PE and a CE. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c11). The prior weakness (the tag sat on a unit that signs the Hello itself) is gone: the tag moved to TestRFC4577ConfiguredLinkSendsAuthenticatedHello +, which parses config text naming key chain ce-link (hmac-sha-256), opens the interfaces over a recording transport and takes the first Hello the engine's own Hello loop sends: AuType 2, Key ID 1, and a separate authStore configured from the same text verifies it. Revert record on auth_wiring.go::signPacket observed red. Row carries {single-polarity: positive} (the negative is the unrecommended state). This proves the link behaviour the SHOULD recommends, which is role-independent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4577ConfiguredLinkSendsAuthenticatedHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_link_auth_test.go#L88) | unit/verify | revert, verified |

### [`RFC4577-4.1.4-2`](#rfc4577-4.1.4-2)

This adjacency MAY be via an OSPF virtual link. (The ability to use an OSPF virtual link in this way is an OPTIONAL feature.) (§4.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. A MAY: the permission is proven by the capability working. TestRFC2328VirtualCostChangeReoriginatesBackbone brings a virtual link to Full through real Hello/DD ingress (establishVirtualRoute) and asserts the backbone Router-LSA advertises the virtual neighbor at metric 10, red if the area 0 adjacency cannot run over a virtual link. Single-polarity marker on the MAY.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L70) | unit/verify | unproven |

### [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3)

If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.1.1-3, so there is no linked test.

### [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10)

The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain. (§4.2.5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.5.2-10, so there is no linked test.

### [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13)

For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-13, so there is no linked test.

### [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14)

If the OSPF instance has a NULL Domain Identifier, and the OSPF Domain Identifier Extended Communities attribute is present, then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, 0205, or 8005. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-14, so there is no linked test.

### [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15)

5 for external routes (area number must be 0). (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-15, so there is no linked test.

### [`RFC4577-4.2.6-16`](#rfc4577-4.2.6-16)

Routes that a PE receives in type 4 LSAs MUST NOT be redistributed to BGP. (§4.2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.6-16, so there is no linked test.

### [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5)

If two VRFs are to be connected by a sham link, each VRF must be associated with a "Sham Link Endpoint Address", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF. (§4.2.7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.1-5, so there is no linked test.

### [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3)

An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver. (§4.2.7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.3-3, so there is no linked test.

### [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4)

The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately. (§4.2.7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.7.3-4, so there is no linked test.

### [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4)

In general, when two such attributes are compared, all eight bytes must be compared. (§4.2.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.8.1-4, so there is no linked test.

### [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5)

If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain. (§4.2.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4577-4.2.8.1-5, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-02 |
| Register | prose |
| Source | rfc/full/rfc4577.txt |
| Source fingerprint | 3d9527c9cc48bead |
| Record | rfc/extraction/rfc4577.json |
| Mapped sentences | 46 |
| Declined as scope | 13 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 4 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.1.1` | not stated | 3 | reviewed | not stated |
| `4.1.2` | not stated | 0 | reviewed | not stated |
| `4.1.3` | not stated | 0 | reviewed | not stated |
| `4.1.4` | not stated | 1 | reviewed | not stated |
| `4.1.5` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.2.1` | not stated | 2 | reviewed | not stated |
| `4.2.2` | not stated | 0 | reviewed | not stated |
| `4.2.3` | not stated | 1 | reviewed | not stated |
| `4.2.4` | not stated | 5 | reviewed | not stated |
| `4.2.5` | not stated | 0 | reviewed | not stated |
| `4.2.5.1` | not stated | 3 | reviewed | not stated |
| `4.2.5.2` | not stated | 8 | reviewed | not stated |
| `4.2.5.3` | not stated | 1 | reviewed | not stated |
| `4.2.6` | not stated | 11 | reviewed | not stated |
| `4.2.7` | not stated | 0 | reviewed | not stated |
| `4.2.7.1` | not stated | 5 | reviewed | not stated |
| `4.2.7.2` | not stated | 1 | reviewed | not stated |
| `4.2.7.3` | not stated | 3 | reviewed | not stated |
| `4.2.7.4` | not stated | 3 | reviewed | not stated |
| `4.2.8` | not stated | 0 | reviewed | not stated |
| `4.2.8.1` | not stated | 4 | reviewed | not stated |
| `4.2.8.2` | not stated | 0 | reviewed | not stated |
| `4.2.8.3` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introductory framing of what the document will do, in the Introduction: it announces that the sections to follow specify the PE procedures. It states no procedure itself, and the obligation it points at is carried by the sections it introduces (4.1 and 4.2), whose sites this walk maps. | Thus, we need to specify the procedures that must be implemented by a PE router in order to make this possible. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 is titled 'Requirements' and states the requirements the SOLUTION must satisfy, not a procedure a PE performs. This sentence describes the model the design adopts (the VPN backbone presented to the CEs as a link between PE routers) as a consequence of wanting the backbone route preferred to the backdoor route. The procedures that realise it are sections 4.2.7.1 to 4.2.7.4, whose sites this walk maps. | Assuming that it is desired to have the route via the VPN backbone be preferred to the backdoor route, the VPN backbone itself must be presented to the CE routers at each site as a link between the two PE routers to which the CE routers are respectively attached. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A requirement on the SERVICE, stated in section 3 'Requirements': the transition from a legacy private OSPF backbone to the VPN service is to be simple. It binds the design of the specification, names no message, field, timer or decision a PE implements, and no implementation can be tested against it. | - The transition from the legacy private OSPF backbone to the VPN service must be simple and straightforward. |
| `3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A requirement on the SERVICE, stated in section 3 'Requirements': the VPN service must maintain complete connectivity among the sites the legacy backbone connected. It binds the design, and the procedures that deliver it are in section 4. | Complete connectivity among all such sites must be maintained. |
| `3:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A requirement on the SERVICE, stated in section 3 'Requirements': it must be possible for an operator to make OSPF prefer backbone routes by adjusting metrics. It binds the design of the solution and names no PE procedure; the sham link procedures of 4.2.7 are what realise it. | Since the VPN service is to replace the legacy backbone, it must be possible, by suitable adjustment of the OSPF metrics, to make OSPF prefer routes that traverse the SP's VPN backbone to alternative routes that do not. |
| `4.2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the area 0 requirement section 4.1.1 states and this walk maps at 4.1.4:1: 'If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router.' Section 4.2.3 adds only the parenthesis '(possibly a virtual link)', which the row already carries. | It follows that if the OSPF domain has any area 0 routers other than the PE routers, at least one of those MUST be a CE router, and it MUST have an area 0 link (possibly a virtual link) to at least one PE router. |
| `4.2.5.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | States the REASON for the two obligations above it rather than an obligation of its own: the VPN Route Tag is configured and included 'for backward compatibility with deployed implementations that do not set the DN bit in type 5 LSAs'. What a PE must do is carried by RFC4577-4.2.5.2-1 (configure the tag by default) and RFC4577-4.2.5.2-2 (include it in originated Type 5 LSAs), both mapped by this walk. | The configuration and inclusion of the VPN Route Tag is required for backward compatibility with deployed implementations that do not set the DN bit in type 5 LSAs. |
| `4.2.5.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A caution to the network designer, in a section that discusses what happens when a third routing domain is present: 'extreme care must be taken if there is any mutual redistribution of routes'. Care is not a behaviour an implementation performs, and the sentence names no LSA, attribute or decision procedure. The redistribution rules a PE must follow are in 4.2.6 and 4.2.8, whose sites this walk maps. | Therefore, extreme care must be taken if there is any mutual redistribution of routes between the OSPF domain and any third routing domain (i.e., not the VPN backbone). |
| `4.2.6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in clause of the procedure the next sentence states, which this walk maps to RFC4577-4.2.6-13: the PE examines the VRF so that 'For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP.' Examining the VRF is how that route set is found; it carries no separate obligation. | Otherwise, the PE must examine the corresponding VRF. |
| `4.2.6:10` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to [VPN], which is RFC 4364, and the sentence cites it: 'The attributes specified above are in addition to any other attributes that routes must carry in accordance with [VPN].' It adds nothing of its own, and what a VPN-IPv4 route must carry is RFC 4364's to state. | The attributes specified above are in addition to any other attributes that routes must carry in accordance with [VPN]. |
| `4.2.7.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | States the CONDITION under which sham links are wanted, as the motivation of section 4.2.7.1: if backbone routes are to be preferred to a backdoor link, they must appear intra-area. It is the premise of the sham link feature, which section 4.1.3 makes OPTIONAL, and the obligations that follow it in 4.2.7.1 to 4.2.7.4 are the procedures a PE that offers sham links performs. This walk maps each of those. | If it is desired to have OSPF prefer the routes through the backbone over the routes through the backdoor link, then the routes through the backbone must be appear to be intra-area routes. |
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates in the Security Considerations the separation section 4.2.1 requires and this walk maps at 4.2.1:2: 'Each instance of OSPF MUST be associated with a single VRF.' A VPN is carried by its VRF, so one instance per VRF is what keeps the instances of different VPNs independent; the sentence adds the reason ('to prevent inadvertent leaking of routes between VPNs') and no new behaviour. | The OSPF instances for different VPNs must also be independent OSPF instances, to prevent inadvertent leaking of routes between VPNs. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate from the Intellectual Property Statement: it invites parties to disclose IPR to the IETF. It binds no implementation and describes an IETF process. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4577, so its obligations are stated where they were written.
