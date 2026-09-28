# RFC 4577 - OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs)

Not supported. Every requirement this repository extracted from RFC 4577, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4.3% | 2 of 46 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 46 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 46 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 46 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 46 | of 66 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 44 | of 46 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 95.7% | 44 of 46 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 46 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 46 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 46 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Not supported |
| Enrolment | Enrolled |
| Requirements | 66 |
| Gated MUST-level | 46 |
| Not applicable, so out of scope | 44 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4577.md` |
| Requirement shard | `rfc/requirements/rfc4577.md` |
| RFC text | `rfc/full/rfc4577.txt` |

## Enrolment

Enrolled: OSPF as the PE/CE Protocol for BGP/MPLS IP VPNs

## What the public ledger says

**Status:** Not supported

**What the ledger says is covered:**

Ordinary OSPF: area and external routing, RFC 6549 instances, cryptographic authentication, backbone virtual links and plain unicast redistribution. These are not RFC 4577 VPN-PE import/export.

**What the ledger says remains**

Native VPN-PE behavior is not selected: no OSPF-domain/VRF association, VPN metadata import/export, VPN Route Tag loop prevention or optional sham links. Ordinary routing and CE obligations remain mandatory and tested under their governing RFCs.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 44 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **46** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4577-4.2.6-5`](#rfc4577-4.2.6-5), [`RFC4577-6-1`](#rfc4577-6-1)

**Annotated instead of tested (44):** [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1), [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1), [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2), [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2), [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1), [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2), [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3), [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4), [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5), [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1), [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2), [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1), [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2), [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1), [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3), [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4), [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3), [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1), [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2), [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3), [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4), [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5), [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6), [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7), [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2), [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1), [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1), [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2), [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3), [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1), [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1), [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1), [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2), [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3), [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3), [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10), [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13), [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14), [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15), [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5), [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3), [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4), [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4), [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4577-4.1.1-1` | A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.1-1` | The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.1-2` | Each instance of OSPF MUST be associated with a single VRF. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.1.1-2` | If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-1` | Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-2` | Domain Identifier association must be configurable (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-3` | If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-4` | If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4) | MUST NOT | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-5` | If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-1` | The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-2` | The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.1-1` | When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.1-2` | When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.8.1-1` | The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-3` | When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-4` | If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.1-3` | To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1) | MUST | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-1` | If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-2` | By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-3` | The VPN Route Tag value must be configurable (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-4` | If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-5` | A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-6` | Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-7` | The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.8.1-2` | The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-5` | Routes that a PE receives in type 4 LSAs MUST NOT be redistributed to BGP. (§4.2.6) | MUST NOT | 4.2.6 | **positive:** `unit/verify` [`TestRFC4577Type3SummaryBecomesRedistributableRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L18). **negative:** `unit/verify` [`TestRFC4577Type4SummaryNotRedistributed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L41) |
| `RFC4577-4.1.4-1` | If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3) | MUST | 4.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** runtime conformance cannot establish the operator's whole-domain topology; Section 4.1.4 says "at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router". The 2026-09-21 ordinary-role decision does not waive that deployment obligation. interfaceConfig and virtual_link.go provide physical/backbone virtual attachment; docs/guide/ospf.md requires it for a CE deployment |
| `RFC4577-4.2.7.1-1` | The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.1-2` | The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.1-3` | The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1) | MUST NOT | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.2-1` | The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2) | MUST NOT | 4.2.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.3-1` | The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.4-1` | Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.4-2` | However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.4-3` | A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4) | MUST | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-6-1` | OSPF cryptographic authentication must be implemented on each PE (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L12). **negative:** `unit/verify` [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L34) |
| `RFC4577-4.2.4-6` | The default Domain Identifier value (if none is configured) should be NULL (§4.2.4) | SHOULD | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-8` | If the VPN backbone AS number is two bytes long, the default VPN Route Tag should be the automatically computed tag based on that AS number (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-9` | A PE distributing to a CE a route from outside the CE's OSPF domain should present itself as an ASBR and should report such routes as AS-external routes (§4.2.5.2) | SHOULD | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-6` | For backward compatibility, OSPF Route Type type 8000 should be accepted and treated as 0306 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-7` | For backward compatibility, OSPF Router ID type 8001 should be accepted and treated as 0107 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-8` | The MED of a PE-originated VPN-IPv4 OSPF route should by default be set to the OSPF distance of the route plus 1 (§4.2.6) | SHOULD | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.7.3-2` | Sham links should be treated by OSPF as OSPF Demand Circuits (§4.2.7.3) | SHOULD | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.4-4` | If a PE determines the next hop interface for a route is a sham link, it should not redistribute that route into BGP as a VPN-IPv4 route (§4.2.7.4) | SHOULD NOT | 4.2.7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-6-2` | OSPF cryptographic authentication should be used between a PE and a CE (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L17). **negative:** no negative test. **{single-polarity}:** the negative -- an interface with no key chain sending unauthenticated OSPF packets -- is the state this SHOULD recommends against, not a behavior the implementation must exhibit, so asserting it would pin the unrecommended path instead of the requirement |
| `RFC4577-4.2.4-7` | If the OSPF instance's Domain Identifier is NULL, the Domain Identifier Extended Communities attribute may be omitted from BGP-distributed routes (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.4-8` | Alternatively, a Domain Identifier Extended Communities attribute value representing NULL may be carried with the route (§4.2.4) | MAY | 4.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-9` | If the OSPF instance has only a NULL Domain Identifier, the OSPF Domain Identifier Extended Communities attribute may be omitted from a PE-originated route (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-10` | For backward compatibility, the OSPF Domain Identifier type 8005 may be used and is treated as if it were 0005 (§4.2.6) | MAY | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.8.1-3` | The default type 1 metric and the default type 2 metric may be different (§4.2.8.1) | MAY | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.1.4-2` | The CE-to-PE area 0 adjacency may be via an OSPF virtual link (OPTIONAL feature) (§4.1.4, §4.2.3) | MAY | 4.1.4 | **positive:** `unit/verify` [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L70). **negative:** no negative test. **{single-polarity}:** this is a MAY, the permission to reach the area 0 adjacency over a virtual link. A negative would have to assert that some configuration does NOT yield a backbone virtual link, which exercises virtual-link configuration handling rather than the permission this requirement grants |
| `RFC4577-4.2.5.1-4` | When the VPN Route Tag is no longer needed for backward compatibility, its use (sending and receiving) may be disabled by configuration (§4.2.5.1, §4.2.5.2) | MAY | 4.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.1.3-1` | Sham links are an OPTIONAL feature of this specification (§4.1.3, §4.2.7) | OPTIONAL | 4.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.6-11` | The OSPF Router ID Extended Communities attribute is an OPTIONAL attribute (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-12` | The Site of Origin attribute, which is usually required by [VPN], is OPTIONAL for routes that a PE learns from a CE via OSPF. (§4.2.6) | OPTIONAL | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.7.1-4` | If a VRF is associated with a single OSPF instance and the PE's router id there is an IP address, the Sham Link Endpoint Address may default to that Router ID (§4.2.7.1) | MAY | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.1.1-3` | If the PE runs OSPF as its IGP, that IGP instance must be separate and independent from any other OSPF instance the PE runs: "If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running." (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.5.2-10` | The VPN Route Tag value must be distinct from any OSPF Route Tag used within the OSPF domain: "The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain." (§4.2.5.2) | MUST | 4.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-13` | For every address prefix installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP: "For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP." (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-14` | Where the OSPF instance has a NULL Domain Identifier and the OSPF Domain Identifier Extended Communities attribute is present, the attribute value field must be all zeroes: "then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, or 0205" (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.6-15` | In the OSPF Route Type Extended Community, the Area Number field must be 0 when the Route Type is 5: "5 for external routes (area number must be 0)" (§4.2.6) | MUST | 4.2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.7.1-5` | Each VRF connected by a sham link must be associated with a Sham Link Endpoint Address: "If two VRFs are to be connected by a sham link, each VRF must be associated with a \\"Sham Link Endpoint Address\\", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF." (§4.2.7.1) | MUST | 4.2.7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.3-3` | An OSPF packet sent on a sham link must carry the sender's Sham Link Endpoint Address as its IP source address and the receiver's as its IP destination address: "An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver." (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.7.3-4` | The TTL of an OSPF packet sent on a sham link must be set appropriately, because the packet traverses multiple hops of the VPN backbone: "The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately." (§4.2.7.3) | MUST | 4.2.7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| `RFC4577-4.2.8.1-4` | All eight bytes must be compared when two Domain Identifier Extended Communities attributes are compared: "In general, when two such attributes are compared, all eight bytes must be compared." (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| `RFC4577-4.2.8.1-5` | Before a VPN-IPv4 route is redistributed to an OSPF instance, it must be determined whether the route and that instance belong to the same domain: "If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain." (§4.2.8.1) | MUST | 4.2.8.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1) A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1) The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2) Each instance of OSPF MUST be associated with a single VRF. (§4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2) If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1) Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2) Domain Identifier association must be configurable (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3) If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4) If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5) If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1) The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2) The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1) When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2) When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1) The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3) When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4) If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3) To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1) If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2) By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3) The VPN Route Tag value must be configurable (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4) If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5) A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6) Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7) The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2) The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1) If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: runtime conformance cannot establish the operator's whole-domain topology; Section 4.1.4 says "at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router". The 2026-09-21 ordinary-role decision does not waive that deployment obligation. interfaceConfig and virtual_link.go provide physical/backbone virtual attachment; docs/guide/ospf.md requires it for a CE deployment |
| [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1) The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2) The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3) The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1) The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1) The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1) Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2) However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3) A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3) If the PE runs OSPF as its IGP, that IGP instance must be separate and independent from any other OSPF instance the PE runs: "If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running." (§4.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10) The VPN Route Tag value must be distinct from any OSPF Route Tag used within the OSPF domain: "The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain." (§4.2.5.2) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13) For every address prefix installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP: "For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP." (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14) Where the OSPF instance has a NULL Domain Identifier and the OSPF Domain Identifier Extended Communities attribute is present, the attribute value field must be all zeroes: "then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, or 0205" (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15) In the OSPF Route Type Extended Community, the Area Number field must be 0 when the Route Type is 5: "5 for external routes (area number must be 0)" (§4.2.6) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5) Each VRF connected by a sham link must be associated with a Sham Link Endpoint Address: "If two VRFs are to be connected by a sham link, each VRF must be associated with a \\"Sham Link Endpoint Address\\", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF." (§4.2.7.1) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3) An OSPF packet sent on a sham link must carry the sender's Sham Link Endpoint Address as its IP source address and the receiver's as its IP destination address: "An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver." (§4.2.7.3) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4) The TTL of an OSPF packet sent on a sham link must be set appropriately, because the packet traverses multiple hops of the VPN backbone: "The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately." (§4.2.7.3) | no test | no test carries this requirement id; annotated {not-applicable}: optional sham links not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 4.1.3 says "Sham links are an OPTIONAL feature of this specification"; virtual_link.go implements ordinary RFC 2328 virtual links, not VRF-to-VRF sham links |
| [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4) All eight bytes must be compared when two Domain Identifier Extended Communities attributes are compared: "In general, when two such attributes are compared, all eight bytes must be compared." (§4.2.8.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |
| [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5) Before a VPN-IPv4 route is redistributed to an OSPF instance, it must be determined whether the route and that instance belong to the same domain: "If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain." (§4.2.8.1) | no test | no test carries this requirement id; annotated {not-applicable}: native VPN-PE role not selected by the 2026-09-21 owner decision below the scope statement; RFC 4577 Section 1 says "No special procedures are needed in the CE router though; CE routers just run whatever OSPF implementations they may have." Ordinary InjectExternal and parseRedistribute are not VPN import/export producers |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4577-4.1.1-1`](#rfc4577-4.1.1-1)

A PE router that attaches to more than one OSPF domain MUST run an independent instance of OSPF for each domain. (§4.1.1, §4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.1-1, so no unit is bound to it.

### [`RFC4577-4.2.1-1`](#rfc4577-4.2.1-1)

The PE MUST support one OSPF instance for each OSPF domain to which it attaches. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.1-1, so no unit is bound to it.

### [`RFC4577-4.2.1-2`](#rfc4577-4.2.1-2)

Each instance of OSPF MUST be associated with a single VRF. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.1-2, so no unit is bound to it.

### [`RFC4577-4.1.1-2`](#rfc4577-4.1.1-2)

If two interfaces belong to the same OSPF instance, then both interfaces must be associated with the same VRF. (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.1-2, so no unit is bound to it.

### [`RFC4577-4.2.4-1`](#rfc4577-4.2.4-1)

Each OSPF instance MUST be associated with one or more Domain Identifiers. (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-1, so no unit is bound to it.

### [`RFC4577-4.2.4-2`](#rfc4577-4.2.4-2)

Domain Identifier association must be configurable (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-2, so no unit is bound to it.

### [`RFC4577-4.2.4-3`](#rfc4577-4.2.4-3)

If an OSPF instance has multiple Domain Identifiers, one of these is considered its "primary" Domain Identifier; this MUST be determinable by configuration. (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-3, so no unit is bound to it.

### [`RFC4577-4.2.4-4`](#rfc4577-4.2.4-4)

If an OSPF instance has more than one Domain Identifier, the NULL Domain Identifier MUST NOT be one of them. (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-4, so no unit is bound to it.

### [`RFC4577-4.2.4-5`](#rfc4577-4.2.4-5)

If a particular OSPF instance has a non-NULL Domain Identifier, when routes from that OSPF instance are distributed by BGP as VPN-IPv4 routes, the routes MUST carry the Domain Identifier Extended Communities attribute that corresponds to the OSPF instance's Primary Domain Identifier. (§4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.4-5, so no unit is bound to it.

### [`RFC4577-4.2.6-1`](#rfc4577-4.2.6-1)

The OSPF Domain Identifier Extended Communities attribute must be present on a PE-originated VPN-IPv4 route if the originating OSPF instance has a non-NULL primary Domain Identifier (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-1, so no unit is bound to it.

### [`RFC4577-4.2.6-2`](#rfc4577-4.2.6-2)

The OSPF Route Type Extended Communities attribute must be present on every PE-originated VPN-IPv4 OSPF route (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-2, so no unit is bound to it.

### [`RFC4577-4.2.5.1-1`](#rfc4577-4.2.5.1-1)

When a type 3 LSA is sent from a PE router to a CE router, the DN bit [OSPF-DN] in the LSA Options field MUST be set. (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-1, so no unit is bound to it.

### [`RFC4577-4.2.5.1-2`](#rfc4577-4.2.5.1-2)

When a PE distributes to a CE a route from outside the CE's OSPF domain (type 5 LSA), the DN bit must be set (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-2, so no unit is bound to it.

### [`RFC4577-4.2.8.1-1`](#rfc4577-4.2.8.1-1)

The DN bit must be set in the (external) LSA reporting a route from a different domain (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-1, so no unit is bound to it.

### [`RFC4577-4.2.6-3`](#rfc4577-4.2.6-3)

When a PE receives from a CE any LSA with the DN bit set, the information from that LSA must not be used by the route calculation (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-3, so no unit is bound to it.

### [`RFC4577-4.2.6-4`](#rfc4577-4.2.6-4)

If a Type 5 LSA received from the CE has an OSPF route tag equal to the VPN Route Tag, its information must not be used by the route calculation (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-4, so no unit is bound to it.

### [`RFC4577-4.2.5.1-3`](#rfc4577-4.2.5.1-3)

To ensure backward compatibility, all implementations adhering to this specification MUST by default support the VPN Route Tag procedures specified in Sections 4.2.5.2, 4.2.8.1, and 4.2.8.2. (§4.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.1-3, so no unit is bound to it.

### [`RFC4577-4.2.5.2-1`](#rfc4577-4.2.5.2-1)

If a particular VRF in a PE is associated with an instance of OSPF, then by default it MUST be configured with a special OSPF route tag value, which we call the VPN Route Tag. (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-1, so no unit is bound to it.

### [`RFC4577-4.2.5.2-2`](#rfc4577-4.2.5.2-2)

By default, this route tag MUST be included in the Type 5 LSAs that the PE originates (as the result of receiving a BGP-distributed VPN-IPv4 route, see Section 4.2.8) and sends to any of the attached CEs. (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-2, so no unit is bound to it.

### [`RFC4577-4.2.5.2-3`](#rfc4577-4.2.5.2-3)

The VPN Route Tag value must be configurable (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-3, so no unit is bound to it.

### [`RFC4577-4.2.5.2-4`](#rfc4577-4.2.5.2-4)

If the VPN backbone AS number is four bytes long, a Route Tag value must be configured (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-4, so no unit is bound to it.

### [`RFC4577-4.2.5.2-5`](#rfc4577-4.2.5.2-5)

A configured four-byte-AS Route Tag must be distinct from any Route Tag used within the VPN itself (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-5, so no unit is bound to it.

### [`RFC4577-4.2.5.2-6`](#rfc4577-4.2.5.2-6)

Each such Type 5 LSA MUST contain an OSPF route tag whose value is that of the VPN Route Tag. (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-6, so no unit is bound to it.

### [`RFC4577-4.2.5.2-7`](#rfc4577-4.2.5.2-7)

The VPN Route Tag MUST be used to ensure that a Type 5 LSA originated by a PE router is not redistributed through the OSPF area to another PE router. (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-7, so no unit is bound to it.

### [`RFC4577-4.2.8.1-2`](#rfc4577-4.2.8.1-2)

The VPN Route Tag (see Section 4.2.5.2) MUST be placed in the LSA, unless the use of the VPN Route Tag has been turned off by configuration. (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-2, so no unit is bound to it.

### [`RFC4577-4.2.6-5`](#rfc4577-4.2.6-5)

Routes that a PE receives in type 4 LSAs MUST NOT be redistributed to BGP. (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4577Type4SummaryNotRedistributed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L41) | unit/verify | unproven |
| positive | [`TestRFC4577Type3SummaryBecomesRedistributableRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc4577_test.go#L18) | unit/verify | unproven |

### [`RFC4577-4.1.4-1`](#rfc4577-4.1.4-1)

If the OSPF domain has any area 0 routers other than the PE routers, then at least one of those MUST be a CE router and MUST have an area 0 link to at least one PE router. (§4.1.4, §4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.4-1, so no unit is bound to it.

### [`RFC4577-4.2.7.1-1`](#rfc4577-4.2.7.1-1)

The Sham Link Endpoint Address associated with a VRF MUST be configurable. (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-1, so no unit is bound to it.

### [`RFC4577-4.2.7.1-2`](#rfc4577-4.2.7.1-2)

The Sham Link Endpoint Address MUST be distributed by BGP as a VPN-IPv4 address whose IPv4 address prefix part is 32 bits long. (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-2, so no unit is bound to it.

### [`RFC4577-4.2.7.1-3`](#rfc4577-4.2.7.1-3)

The Sham Link Endpoint Address MUST NOT be advertised by OSPF; if there is no BGP route to the Sham Link Endpoint Address, that address is to appear unreachable, so that the sham link appears to be down. (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-3, so no unit is bound to it.

### [`RFC4577-4.2.7.2-1`](#rfc4577-4.2.7.2-1)

The sham link endpoint address MUST NOT be used as the endpoint address of an OSPF Virtual Link. (§4.2.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.2-1, so no unit is bound to it.

### [`RFC4577-4.2.7.3-1`](#rfc4577-4.2.7.3-1)

The OSPF metric associated with a sham link MUST be configurable (and there MUST be a configurable default). (§4.2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.3-1, so no unit is bound to it.

### [`RFC4577-4.2.7.4-1`](#rfc4577-4.2.7.4-1)

Any other route advertised in an LSA that is transmitted over a sham link MUST also be redistributed (by the PE flooding the LSA over the sham link) into BGP. (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-1, so no unit is bound to it.

### [`RFC4577-4.2.7.4-2`](#rfc4577-4.2.7.4-2)

However, when forwarding a packet, if the preferred route for that packet has the sham link as its next hop interface, then the packet MUST be forwarded according to the corresponding BGP route. (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-2, so no unit is bound to it.

### [`RFC4577-4.2.7.4-3`](#rfc4577-4.2.7.4-3)

A packet whose IP destination is the remote endpoint address of a sham link must be forwarded according to the corresponding BGP route (§4.2.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.4-3, so no unit is bound to it.

### [`RFC4577-6-1`](#rfc4577-6-1)

OSPF cryptographic authentication must be implemented on each PE (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4577CryptographicAuthRejectsForgery`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L34) | unit/verify | unproven |
| positive | [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L12) | unit/verify | unproven |

### [`RFC4577-6-2`](#rfc4577-6-2)

OSPF cryptographic authentication should be used between a PE and a CE (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4577CryptographicAuthImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc4577_test.go#L17) | unit/verify | unproven |

### [`RFC4577-4.1.4-2`](#rfc4577-4.1.4-2)

The CE-to-PE area 0 adjacency may be via an OSPF virtual link (OPTIONAL feature) (§4.1.4, §4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2328VirtualCostChangeReoriginatesBackbone`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc2328_virtual_route_test.go#L70) | unit/verify | unproven |

### [`RFC4577-4.1.1-3`](#rfc4577-4.1.1-3)

If the PE runs OSPF as its IGP, that IGP instance must be separate and independent from any other OSPF instance the PE runs: "If the PE is running OSPF as its IGP (Interior Gateway Protocol), the instance of OSPF running as the IGP must be separate and independent from any other instance of OSPF that the PE is running." (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.1.1-3, so no unit is bound to it.

### [`RFC4577-4.2.5.2-10`](#rfc4577-4.2.5.2-10)

The VPN Route Tag value must be distinct from any OSPF Route Tag used within the OSPF domain: "The value of the VPN Route Tag is arbitrary but must be distinct from any OSPF Route Tag being used within the OSPF domain." (§4.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.5.2-10, so no unit is bound to it.

### [`RFC4577-4.2.6-13`](#rfc4577-4.2.6-13)

For every address prefix installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP: "For every address prefix that was installed in the VRF by one of its associated OSPF instances, the PE must create a VPN-IPv4 route in BGP." (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-13, so no unit is bound to it.

### [`RFC4577-4.2.6-14`](#rfc4577-4.2.6-14)

Where the OSPF instance has a NULL Domain Identifier and the OSPF Domain Identifier Extended Communities attribute is present, the attribute value field must be all zeroes: "then the attribute's value field must be all zeroes, and its type field may be any of 0005, 0105, or 0205" (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-14, so no unit is bound to it.

### [`RFC4577-4.2.6-15`](#rfc4577-4.2.6-15)

In the OSPF Route Type Extended Community, the Area Number field must be 0 when the Route Type is 5: "5 for external routes (area number must be 0)" (§4.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.6-15, so no unit is bound to it.

### [`RFC4577-4.2.7.1-5`](#rfc4577-4.2.7.1-5)

Each VRF connected by a sham link must be associated with a Sham Link Endpoint Address: "If two VRFs are to be connected by a sham link, each VRF must be associated with a \"Sham Link Endpoint Address\", a 32-bit IPv4 address that is treated as an address of the PE router containing that VRF." (§4.2.7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.1-5, so no unit is bound to it.

### [`RFC4577-4.2.7.3-3`](#rfc4577-4.2.7.3-3)

An OSPF packet sent on a sham link must carry the sender's Sham Link Endpoint Address as its IP source address and the receiver's as its IP destination address: "An OSPF protocol packet sent on a Sham Link from one PE to another must have as its IP source address the Sham Link Endpoint Address of the sender, and as its IP destination address the Sham Link Endpoint Address of the receiver." (§4.2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.3-3, so no unit is bound to it.

### [`RFC4577-4.2.7.3-4`](#rfc4577-4.2.7.3-4)

The TTL of an OSPF packet sent on a sham link must be set appropriately, because the packet traverses multiple hops of the VPN backbone: "The packet will travel from one PE router to the other over the VPN backbone, which means that it can be expected to traverse multiple hops. As such, its TTL (Time to Live) field must be set appropriately." (§4.2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.7.3-4, so no unit is bound to it.

### [`RFC4577-4.2.8.1-4`](#rfc4577-4.2.8.1-4)

All eight bytes must be compared when two Domain Identifier Extended Communities attributes are compared: "In general, when two such attributes are compared, all eight bytes must be compared." (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-4, so no unit is bound to it.

### [`RFC4577-4.2.8.1-5`](#rfc4577-4.2.8.1-5)

Before a VPN-IPv4 route is redistributed to an OSPF instance, it must be determined whether the route and that instance belong to the same domain: "If a VPN-IPv4 route is to be redistributed to a particular instance, it must be determined whether that route and that OSPF instance belong to the same domain." (§4.2.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4577-4.2.8.1-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 3 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.1.3` | not stated | 0 | walked | not stated |
| `4.1.4` | not stated | 1 | walked | not stated |
| `4.1.5` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 2 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.3` | not stated | 1 | walked | not stated |
| `4.2.4` | not stated | 5 | walked | not stated |
| `4.2.5` | not stated | 0 | walked | not stated |
| `4.2.5.1` | not stated | 3 | walked | not stated |
| `4.2.5.2` | not stated | 8 | walked | not stated |
| `4.2.5.3` | not stated | 1 | walked | not stated |
| `4.2.6` | not stated | 11 | walked | not stated |
| `4.2.7` | not stated | 0 | walked | not stated |
| `4.2.7.1` | not stated | 5 | walked | not stated |
| `4.2.7.2` | not stated | 1 | walked | not stated |
| `4.2.7.3` | not stated | 3 | walked | not stated |
| `4.2.7.4` | not stated | 3 | walked | not stated |
| `4.2.8` | not stated | 0 | walked | not stated |
| `4.2.8.1` | not stated | 4 | walked | not stated |
| `4.2.8.2` | not stated | 0 | walked | not stated |
| `4.2.8.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |

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
