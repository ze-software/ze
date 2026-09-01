# RFC 7454 - BGP Operations and Security

No row in the public ledger. Every requirement this repository extracted from RFC 7454, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 0 | of 64 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (non-normative) |
| Requirements | 64 |
| Gated MUST-level | 0 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7454.md` |
| Requirement shard | `rfc/requirements/rfc7454.md` |
| RFC text | `rfc/full/rfc7454.txt` |

## Enrolment

Not enrolled (non-normative): BGP Operations and Security, published as BCP 194 with IETF category Best Current Practice. A capitalised MUST / MUST NOT / SHALL / SHALL NOT scan over rfc/full/rfc7454.txt hits four keywords and all four sit inside the RFC 2119 key-words sentence of section 1.1, which tells a reader how to read the other sentences and states no obligation of its own. Outside that sentence the document uses no MUST-level keyword except one NOT REQUIRED in section 5.1, and that phrase negates a requirement instead of stating one. The summary written 2026-08-08 therefore captures 64 requirements and gates none of them: 42 SHOULD, 12 SHOULD NOT, 8 RECOMMENDED, 1 MAY, and the single NOT REQUIRED of section 5.1 recorded at the OPTIONAL level. The document also addresses network administrators rather than protocol implementers, and section 12 states that it "does not aim to describe existing BGP implementations". A zero-MUST BCP can reach the public ledger two ways, as a non-normative disposition or as a manual-walk extraction sign-off with a register-reason, and that choice is a ledger judgement for the owner. Thomas made it on 2026-08-12: non-normative, on the grounds that the scan recorded above finds no MUST-level keyword outside the key-words sentence, so backlog overstated a debt this text does not create. The choice is no longer open.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7454.

## Coverage

RFC 7454 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7454-2-1` | "network administrators SHOULD carefully appraise this impact before implementation", for every configured exception to this document (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-4-1` | Protection of the BGP speaker "SHOULD be achieved by an Access Control List (ACL) that would discard all packets directed to TCP port 179 on the local device and sourced from an address not known or permitted to become a BGP neighbor" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-4-2` | "If supported, an ACL specific to the control plane of the router SHOULD be used (receive-ACL, control-plane policing, etc.)" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-5.1-1` | "TCP-AO SHOULD be preferred when implemented", over TCP MD5 (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-5.1-4` | "network administrators SHOULD block spoofed packets (packets with a source IP address belonging to their IP address space) at all edges of their network" (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-5.2-1` | "Network administrators SHOULD implement TTL security on directly connected BGP peerings" (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.1.1-1` | The IANA IPv4 Special-Purpose Address Registry "SHOULD be used for prefix-filter configuration" (§6.1.1.1) | SHOULD | 6.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.1.1-2` | IPv4 prefixes "with value "False" in column "Global" SHOULD be discarded on Internet BGP peerings" (§6.1.1.1) | SHOULD | 6.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.1.2-1` | The IANA IPv6 Special-Purpose Address Registry "SHOULD be used for prefix-filter configuration" (§6.1.1.2) | SHOULD | 6.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.1.2-2` | "Only prefixes with value "False" in column "Global" SHOULD be discarded on Internet BGP peerings", for IPv6 (§6.1.1.2) | SHOULD | 6.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.1-1` | "network administrators SHOULD ensure that all IPv6 prefix filters are updated within a maximum of one month after any change in the list of IPv6 prefixes allocated by IANA" (§6.1.2.1) | SHOULD | 6.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.1-1` | For IRR-derived prefix filters, "Refreshing the filters on a daily basis SHOULD be considered" (§6.1.2.2.1) | SHOULD | 6.1.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.1-2` | "network administrators SHOULD publish and maintain their resources properly in the IRR database maintained by their RIR, when available" (§6.1.2.2.1) | SHOULD | 6.1.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-1` | "network administrators SHOULD implement any SIDR proposed mechanism (for example, route origin validation) on top of the other existing mechanisms" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-2` | "If route origin validation is implemented, the reader SHOULD refer to the rules described in RFC 7115" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-3` | "each external route received on a router SHOULD be checked against the Resource Public Key Infrastructure (RPKI) data set" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-4` | "If a corresponding ROA (Route Origin Authorization) is found and is valid, then the prefix SHOULD be accepted" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-5` | "If the ROA is found and is INVALID, then the prefix SHOULD be discarded" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-6` | "If a ROA is not found, then the prefix SHOULD be accepted" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-7` | Where no ROA is found, "the corresponding route SHOULD be given a low preference" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2.2.2-8` | "network administrators SHOULD sign their routing objects so their routes can be validated by other networks running origin validation" (§6.1.2.2.2) | SHOULD | 6.1.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.4-1` | "A network SHOULD filter its own prefixes on peerings with all its peers (inbound direction)" (§6.1.4) | SHOULD | 6.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.5.1-2` | "If the IXP LAN prefix is accepted at all, it SHOULD only be accepted from the ASes that the IXP authorizes to announce it" (§6.1.5.1) | SHOULD | 6.1.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.5.2-1` | "any IXP member SHOULD make sure it has a route for the IXP LAN prefix or a less specific prefix on all its routers" (§6.1.5.2) | SHOULD | 6.1.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.5.2-2` | An IXP member SHOULD make sure "that it announces the IXP LAN prefix or the less specific route (up to a default route) to its downstreams" (§6.1.5.2) | SHOULD | 6.1.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.5.2-3` | "The announcements done for this purpose SHOULD pass IRR-generated filters described in Section 6.1.2.2.1 as well as "prefixes that are too specific" filters described in Section 6.1.3" (§6.1.5.2) | SHOULD | 6.1.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.2.1.1.2-1` | "Before applying a strict policy, the reader SHOULD check the impact on the filter and make sure the solution is not worse than the problem" (§6.2.1.1.2) | SHOULD | 6.2.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.2.2.1-1` | With end customers, "only customer prefixes SHOULD be accepted" (§6.2.2.1) | SHOULD | 6.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.2.2.1-2` | With end customers, prefixes other than the customer's "SHOULD be discarded" (§6.2.2.1) | SHOULD | 6.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-1` | "Network administrators SHOULD accept from customers only 2-byte or 4-byte AS paths containing ASNs belonging to (or authorized to transit through) the customer" (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-2` | Where those filter expressions cannot be built, administrators "SHOULD consider accepting only path lengths relevant to the type of customer they have" (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-3` | Where those filter expressions cannot be built, administrators "SHOULD try to discourage excessive prepending in such paths" (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-5` | Where an upstream offers black-hole origination based on a private AS number, "in that case, prefixes SHOULD be accepted" (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-10` | Private AS numbers "SHOULD be stripped when received from BGP peers that are not party to such private arrangements" (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-10-1` | "an inbound route policy SHOULD be applied on IXP peerings in order to set the next hop for accepted prefixes to the BGP peer IP address (belonging to the IXP LAN) that sent the prefix" (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-10-3` | "This policy also SHOULD be adjusted if the best practice of Remote Triggered Black Holing (aka RTBH as described in RFC 6666) is implemented" (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-11-1` | "Network administrators SHOULD scrub inbound communities with their number in the high-order bits, and allow only those communities that customers/peers can use as a signaling mechanism" (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-11-3` | Administrators "SHOULD keep original communities when they apply a community" (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-A-1` | "Any IXP member SHOULD make sure it filters prefixes more specific than X.Y.0.0/23 from all its EBGP peers", where X.Y.0.0/23 is the IXP LAN prefix (§A) | SHOULD | A | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-A-2` | "The IXP SHOULD originate X.Y.0.0/22 and advertise it to its members through an EBGP peering", where X.Y.0.0/22 is the IXP allocation (§A) | SHOULD | A | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-A-3` | "The IXP members SHOULD accept the IXP prefix only if it passes the IRR generated filters" (§A) | SHOULD | A | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-A-4` | "IXP members SHOULD then advertise X.Y.0.0/22 prefix to their downstreams" (§A) | SHOULD | A | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.2-1` | "Network administrators SHOULD NOT consider solutions described in this section if they are not capable of maintaining updated prefix filters", for unallocated-prefix filtering (§6.1.2) | SHOULD NOT | 6.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.4-2` | "In some cases, for example, multihoming scenarios, such filters SHOULD NOT be applied, as this would break the desired redundancy", for local-AS prefix filters (§6.1.4) | SHOULD NOT | 6.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.5.1-1` | A network on an IXP "SHOULD NOT accept more-specific prefixes for the IXP LAN prefix from any of its external BGP peers" (§6.1.5.1) | SHOULD NOT | 6.1.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-4` | "Network administrators SHOULD NOT accept prefixes with private AS numbers in the AS path unless the prefixes are from customers" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-6` | "Network administrators SHOULD NOT accept prefixes when the first AS number in the AS path is not the one of the peer's unless the peering is done toward a BGP route server ... with transparent AS path handling" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-7` | "Network administrators SHOULD NOT advertise prefixes with a nonempty AS path unless they intend to provide transit for these prefixes" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-8` | "Network administrators SHOULD NOT advertise prefixes with upstream AS numbers in the AS path to their peering AS unless they intend to provide transit for these prefixes" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-9` | Private AS numbers "SHOULD NOT be used in advertisements to BGP peers that are not party to such private arrangements" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-9-11` | "Network administrators SHOULD NOT override BGP's default behavior, i.e., they should not accept their own AS number in the AS path" (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-10-2` | "This policy SHOULD NOT be used on route-server peerings or on peerings where network administrators intentionally permit the other side to send third-party next hops" (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-11-2` | "Networks administrators SHOULD NOT remove other communities applied on received routes" (§11) | SHOULD NOT | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-11-4` | "network administrators SHOULD NOT (generally) remove the no-export community, as it is usually announced by their peer for a certain purpose" (§11) | SHOULD NOT | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-5.1-3` | "operators are RECOMMENDED to consider the trade-offs and to apply TCP session protection where appropriate" (§5.1) | RECOMMENDED | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.6.1-1` | "Typically, the 0.0.0.0/0 prefix is not intended to be accepted or advertised except in specific customer/provider configurations; general filtering outside of these is RECOMMENDED" (§6.1.6.1) | RECOMMENDED | 6.1.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.1.6.2-1` | "Typically, the ::/0 prefix is not intended to be accepted or advertised except in specific customer/provider configurations; general filtering outside of these is RECOMMENDED" (§6.1.6.2) | RECOMMENDED | 6.1.6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-6.2-1` | "It is RECOMMENDED that each Autonomous System configures rules for advertised and received routes at all its borders" (§6.2) | RECOMMENDED | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-7-1` | "This document RECOMMENDS following IETF and RIPE recommendations and using BGP route flap dampening with the adjusted configured thresholds" (§7) | RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-8-1` | "It is RECOMMENDED to configure a limit on the number of routes to be accepted from a peer" (§8) | RECOMMENDED | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-8-2` | "From peers, it is RECOMMENDED to have a limit lower than the number of routes in the Internet" (§8) | RECOMMENDED | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-8-3` | "From upstreams that provide full routing, it is RECOMMENDED to have a limit higher than the number of routes in the Internet" (§8) | RECOMMENDED | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-4-3` | "In addition to strict filtering, rate-limiting MAY be configured for accepted BGP traffic" (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7454-5.1-2` | "Protection of TCP sessions used by BGP is thus NOT REQUIRED even when peerings are established over shared networks where spoofing can be done (like IXPs)" (§5.1) | OPTIONAL | 5.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7454 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 7454 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

No extraction sign-off exists for RFC 7454, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7454, so its obligations are stated where they were written.
