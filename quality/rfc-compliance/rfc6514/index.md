# RFC 6514 - BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs

Future. Every requirement this repository extracted from RFC 6514, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 133 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 133 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 133 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 133 | of 201 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 133 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 133 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 133 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 133 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 133 of 133 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 133 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 201 |
| Gated MUST-level | 133 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 133 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6514.md` |
| Requirement shard | `rfc/requirements/rfc6514.md` |
| RFC text | `rfc/full/rfc6514.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs. OUT OF SCOPE by owner decision, 2026-09-01, marked for future development. The extraction is COMPLETE: the source text is at rfc/full/rfc6514.txt and this summary declares all 201 requirements, 133 of them MUST-level, so a later decision to build MVPN starts from the obligations rather than from nothing. What Ze has today is NLRI plumbing and nothing the RFC is about: the Section 4 route-type split (splitMVPN, internal/core/bgp/nlri/nlrisplit/mvpn.go), an NLRI codec, a config route parser for three of the seven route types, and opaque Adj-RIB-In storage. Exactly one MUST-level requirement is met and it is met vacuously -- RFC6514-9.1.1-10 says the Leaf Information Required flag "MUST be set to zero and MUST be ignored on receipt", and Ze ignores it by never parsing a PMSI byte, because knownAttrParsers leaves attribute code 22 nil. Absent entirely: the PMSI Tunnel attribute, the PE Distinguisher Labels attribute (code 27 has no constant), the Source AS and VRF Route Import extended communities, auto-discovery, the C-multicast route exchange, S-PMSI routes, inter-AS and ASBR operation, upstream multicast hop selection (SAFI 129 is unregistered), and every protocol the document leans on -- PIM, mLDP, RSVP-TE P2MP and MSDP. No {gap} annotation is written for any of it: a gap is an ISSUE and this is a DECISION (ai/rules/rfc-compliance.md), and 132 gap rows would record a feature nobody chose to build as 132 conformance failures.

## What the public ledger says

**Status:** Future

**What the ledger says is covered**

MCAST-VPN NLRI decode and encode primitives, the Section 4 type-and-length NLRI split (`splitMVPN`, [`internal/core/bgp/nlri/nlrisplit/mvpn.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/mvpn.go)), a config route parser for source-active, shared-tree join and source-tree join routes, the opaque Adj-RIB-In storage that split enables, and the RFC 7606 Section 5.4 ruling that discards a route type outside 1..7 at ingress. This is NLRI carriage, not MVPN: no PMSI tunnel is built or parsed, and no multicast state is created.

**What the ledger says remains**

Out of scope by owner decision, 2026-09-01, and tracked for future development. Requirements bound per line in [`rfc/short/rfc6514.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6514.md), which declares all 133 MUST-level obligations: 132 have no producer in Ze. MVPN is not offered, so the absence is an implementation gap a later scope decision can revisit, and no conformance gap is claimed.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 133 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **133** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (133):** [`RFC6514-4.5-1`](#rfc6514-4.5-1), [`RFC6514-4.5-2`](#rfc6514-4.5-2), [`RFC6514-5-1`](#rfc6514-5-1), [`RFC6514-5-2`](#rfc6514-5-2), [`RFC6514-5-3`](#rfc6514-5-3), [`RFC6514-5-4`](#rfc6514-5-4), [`RFC6514-5-7`](#rfc6514-5-7), [`RFC6514-5-8`](#rfc6514-5-8), [`RFC6514-6-1`](#rfc6514-6-1), [`RFC6514-6-2`](#rfc6514-6-2), [`RFC6514-6-3`](#rfc6514-6-3), [`RFC6514-7-1`](#rfc6514-7-1), [`RFC6514-7-2`](#rfc6514-7-2), [`RFC6514-7-4`](#rfc6514-7-4), [`RFC6514-7-5`](#rfc6514-7-5), [`RFC6514-8-1`](#rfc6514-8-1), [`RFC6514-8-3`](#rfc6514-8-3), [`RFC6514-8-4`](#rfc6514-8-4), [`RFC6514-9.1.1-1`](#rfc6514-9.1.1-1), [`RFC6514-9.1.1-2`](#rfc6514-9.1.1-2), [`RFC6514-9.1.1-4`](#rfc6514-9.1.1-4), [`RFC6514-9.1.1-5`](#rfc6514-9.1.1-5), [`RFC6514-9.1.1-6`](#rfc6514-9.1.1-6), [`RFC6514-9.1.1-7`](#rfc6514-9.1.1-7), [`RFC6514-9.1.1-8`](#rfc6514-9.1.1-8), [`RFC6514-9.1.1-9`](#rfc6514-9.1.1-9), [`RFC6514-9.1.1-10`](#rfc6514-9.1.1-10), [`RFC6514-9.1.1-11`](#rfc6514-9.1.1-11), [`RFC6514-9.1.1-12`](#rfc6514-9.1.1-12), [`RFC6514-9.1.1-14`](#rfc6514-9.1.1-14), [`RFC6514-9.1.2-2`](#rfc6514-9.1.2-2), [`RFC6514-9.2-1`](#rfc6514-9.2-1), [`RFC6514-9.2-3`](#rfc6514-9.2-3), [`RFC6514-9.2-4`](#rfc6514-9.2-4), [`RFC6514-9.2-5`](#rfc6514-9.2-5), [`RFC6514-9.2-6`](#rfc6514-9.2-6), [`RFC6514-9.2-9`](#rfc6514-9.2-9), [`RFC6514-9.2-11`](#rfc6514-9.2-11), [`RFC6514-9.2-14`](#rfc6514-9.2-14), [`RFC6514-9.2.1-1`](#rfc6514-9.2.1-1), [`RFC6514-9.2.1-3`](#rfc6514-9.2.1-3), [`RFC6514-9.2.3.2-1`](#rfc6514-9.2.3.2-1), [`RFC6514-9.2.3.2-2`](#rfc6514-9.2.3.2-2), [`RFC6514-9.2.3.2-3`](#rfc6514-9.2.3.2-3), [`RFC6514-9.2.3.2-5`](#rfc6514-9.2.3.2-5), [`RFC6514-9.2.3.2-6`](#rfc6514-9.2.3.2-6), [`RFC6514-9.2.3.2-7`](#rfc6514-9.2.3.2-7), [`RFC6514-9.2.3.2.1-1`](#rfc6514-9.2.3.2.1-1), [`RFC6514-9.2.3.2.1-2`](#rfc6514-9.2.3.2.1-2), [`RFC6514-9.2.3.2.1-3`](#rfc6514-9.2.3.2.1-3), [`RFC6514-9.2.3.2.1-4`](#rfc6514-9.2.3.2.1-4), [`RFC6514-9.2.3.2.1-5`](#rfc6514-9.2.3.2.1-5), [`RFC6514-9.2.3.2.1-6`](#rfc6514-9.2.3.2.1-6), [`RFC6514-9.2.3.2.1-7`](#rfc6514-9.2.3.2.1-7), [`RFC6514-9.2.3.3-2`](#rfc6514-9.2.3.3-2), [`RFC6514-9.2.3.4-1`](#rfc6514-9.2.3.4-1), [`RFC6514-9.2.3.4-2`](#rfc6514-9.2.3.4-2), [`RFC6514-9.2.3.4-3`](#rfc6514-9.2.3.4-3), [`RFC6514-9.2.3.4-5`](#rfc6514-9.2.3.4-5), [`RFC6514-9.2.3.4.1-1`](#rfc6514-9.2.3.4.1-1), [`RFC6514-9.2.3.4.1-2`](#rfc6514-9.2.3.4.1-2), [`RFC6514-9.2.3.4.1-3`](#rfc6514-9.2.3.4.1-3), [`RFC6514-9.2.3.4.1-4`](#rfc6514-9.2.3.4.1-4), [`RFC6514-9.2.3.4.1-5`](#rfc6514-9.2.3.4.1-5), [`RFC6514-9.2.3.4.1-6`](#rfc6514-9.2.3.4.1-6), [`RFC6514-9.2.3.4.1-7`](#rfc6514-9.2.3.4.1-7), [`RFC6514-10-2`](#rfc6514-10-2), [`RFC6514-10-4`](#rfc6514-10-4), [`RFC6514-10-6`](#rfc6514-10-6), [`RFC6514-10-7`](#rfc6514-10-7), [`RFC6514-11.1.1.1-1`](#rfc6514-11.1.1.1-1), [`RFC6514-11.1.1.1-2`](#rfc6514-11.1.1.1-2), [`RFC6514-11.1.1.2-1`](#rfc6514-11.1.1.2-1), [`RFC6514-11.1.1.2-2`](#rfc6514-11.1.1.2-2), [`RFC6514-11.1.3-1`](#rfc6514-11.1.3-1), [`RFC6514-11.1.4-1`](#rfc6514-11.1.4-1), [`RFC6514-11.1.4-2`](#rfc6514-11.1.4-2), [`RFC6514-11.1.4-3`](#rfc6514-11.1.4-3), [`RFC6514-11.2-1`](#rfc6514-11.2-1), [`RFC6514-11.2-2`](#rfc6514-11.2-2), [`RFC6514-11.3.1.1-1`](#rfc6514-11.3.1.1-1), [`RFC6514-11.3.1.1-3`](#rfc6514-11.3.1.1-3), [`RFC6514-11.3.1.2-1`](#rfc6514-11.3.1.2-1), [`RFC6514-12.1-1`](#rfc6514-12.1-1), [`RFC6514-12.1-2`](#rfc6514-12.1-2), [`RFC6514-12.1-3`](#rfc6514-12.1-3), [`RFC6514-12.1-4`](#rfc6514-12.1-4), [`RFC6514-12.1-5`](#rfc6514-12.1-5), [`RFC6514-12.1-6`](#rfc6514-12.1-6), [`RFC6514-12.1-7`](#rfc6514-12.1-7), [`RFC6514-12.1-8`](#rfc6514-12.1-8), [`RFC6514-12.1-12`](#rfc6514-12.1-12), [`RFC6514-12.1-13`](#rfc6514-12.1-13), [`RFC6514-12.1-14`](#rfc6514-12.1-14), [`RFC6514-12.1-16`](#rfc6514-12.1-16), [`RFC6514-12.1-18`](#rfc6514-12.1-18), [`RFC6514-12.1-19`](#rfc6514-12.1-19), [`RFC6514-12.1-20`](#rfc6514-12.1-20), [`RFC6514-12.1-21`](#rfc6514-12.1-21), [`RFC6514-12.1-22`](#rfc6514-12.1-22), [`RFC6514-12.2.1-3`](#rfc6514-12.2.1-3), [`RFC6514-12.2.1-4`](#rfc6514-12.2.1-4), [`RFC6514-12.3-1`](#rfc6514-12.3-1), [`RFC6514-13-1`](#rfc6514-13-1), [`RFC6514-13-2`](#rfc6514-13-2), [`RFC6514-13.1-1`](#rfc6514-13.1-1), [`RFC6514-13.1-2`](#rfc6514-13.1-2), [`RFC6514-13.1-3`](#rfc6514-13.1-3), [`RFC6514-13.1-4`](#rfc6514-13.1-4), [`RFC6514-13.1-6`](#rfc6514-13.1-6), [`RFC6514-13.2-1`](#rfc6514-13.2-1), [`RFC6514-13.2-2`](#rfc6514-13.2-2), [`RFC6514-13.2.1-1`](#rfc6514-13.2.1-1), [`RFC6514-13.2.1-3`](#rfc6514-13.2.1-3), [`RFC6514-13.2.1-4`](#rfc6514-13.2.1-4), [`RFC6514-13.2.1-5`](#rfc6514-13.2.1-5), [`RFC6514-14-1`](#rfc6514-14-1), [`RFC6514-14-2`](#rfc6514-14-2), [`RFC6514-14.1-1`](#rfc6514-14.1-1), [`RFC6514-14.1-2`](#rfc6514-14.1-2), [`RFC6514-14.1-3`](#rfc6514-14.1-3), [`RFC6514-14.2-1`](#rfc6514-14.2-1), [`RFC6514-14.2-2`](#rfc6514-14.2-2), [`RFC6514-14.2-3`](#rfc6514-14.2-3), [`RFC6514-14.2-4`](#rfc6514-14.2-4), [`RFC6514-14.2-5`](#rfc6514-14.2-5), [`RFC6514-14.2-6`](#rfc6514-14.2-6), [`RFC6514-14.2-7`](#rfc6514-14.2-7), [`RFC6514-14.2-8`](#rfc6514-14.2-8), [`RFC6514-14.2-9`](#rfc6514-14.2-9), [`RFC6514-17-2`](#rfc6514-17-2), [`RFC6514-17-3`](#rfc6514-17-3), [`RFC6514-17-4`](#rfc6514-17-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6514-4.5-1` | Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router (§4.5) | MUST NOT | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-4.5-2` | Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router and MUST be discarded if received. (§4.5) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-1` | When the Tunnel Type is set to Protocol Independent Multicast - Sparse Mode (PIM-SM) tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE (Generic Routing Encapsulation) encapsulation of the MVPN data. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-2` | When the Tunnel Type is set to PIM-SSM tree, the Tunnel Identifier is <P-Root Node Address, P-Multicast Group>. The node that originates the attribute MUST use the address carried in the P-Root Node Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-3` | The P-Multicast Group in the Tunnel Identifier of the Tunnel attribute MUST NOT be expected to be the same group for all Intra-AS A-D routes for the same MVPN. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-4` | When the Tunnel Type is set to BIDIR-PIM tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-7` | An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-8` | An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-6-1` | The Global Administrator field of this Community MUST be set to the ASN of the PE. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-6-2` | The Local Administrator field of this Community MUST be set to 0. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-6-3` | A PE that has sites of that MVPN connected to it, and originates a (unicast) route to VPN-IP addresses associated with the destinations within these sites, MUST include in the BGP Update message that carries this route the Source AS Extended Community. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-1` | To support MVPN in addition to the import/export Route Target(s) Extended Communities used by the unicast routing, each VRF on a PE MUST have an import Route Target Extended Community, except if it is known a priori that none of the (local) MVPN sites associated with the VRF contain multicast source(s) and/or C-RP; in which case, the VRF need not have this import Route Target. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-2` | The Global Administrator field of the C-multicast Import RT MUST be set to an IP address of the PE. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-4` | To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-5` | To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route, except if it is known a priori (e.g., via provisioning) that none of these addresses could act as multicast sources and/or RP; in which case, the (unicast) route MUST NOT carry the VRF Route Import Extended Community. (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-8-1` | Each of the PE addresses in the PE Distinguisher Labels attribute MUST be of the same address family as the "Originating Router's IP Address" of the route that is carrying the attribute. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-8-3` | An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-8-4` | An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-1` | To participate in the MVPN auto-discovery/binding, a PE router that has a given VRF of a given MVPN MUST, except for the cases specified in this section, originate an Intra-AS I-PMSI A-D route and advertises this route in IBGP. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-2` | If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for the MVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-4` | A PE that uses a P-multicast tree for the P-tunnel MAY aggregate two or more MVPNs present on the PE onto the same tree. In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute of the Intra-AS I-PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-5` | If the PE has already advertised Intra-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-6` | The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-7` | If the PE that originates the advertisement uses ingress replication for the P-tunnel for the MVPN, the route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and Tunnel Identifier set to a routable address of the PE. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-8` | The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label. This label is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-9` | The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-10` | The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-11` | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-12` | That is, by default, the Intra-AS I-PMSI A-D route MUST carry the export Route Target used by the unicast routing. (§9.1.1) | MUST | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-14` | Note that if non-segmented inter-AS P-tunnels are being used, then the Intra-AS I-PMSI routes need to be distributed to other ASes and MUST NOT carry the NO_EXPORT Community. (§9.1.1) | MUST NOT | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.2-2` | If the Tunnel Type in the PMSI Tunnel attribute is set to RSVP-TE P2MP LSP, then the PE that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE as a leaf. (§9.1.2) | MUST | 9.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-1` | An ASBR MUST be configured with a set of (import) Route Targets (RTs) that specifies the set of MVPNs supported by the ASBR. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-3` | The ASBR MUST be (auto-)configured with an import Route Target called "ASBR Import RT". (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-4` | The Global Administrator field of the ASBR Import RT MUST be set to the IP address carried in the Next Hop of all the Inter-AS I-PMSI A-D routes and S-PMSI A-D routes advertised by this ASBR (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-5` | if the ASBR uses different Next Hops, then the ASBR MUST be (auto-)configured with multiple ASBR Import RTs, one per each such Next Hop (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-6` | The Local Administrator field of the ASBR Import RT MUST be set to 0. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-9` | The ASBR MUST be configured with the tunnel types for the intra-AS segments of the MVPNs supported by the ASBR, as well as (depending on the tunnel type) the information needed to create the PMSI attribute for these tunnel types. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-11` | If the ASBR originates an Inter-AS I-PMSI A-D route for a particular MVPN present on some of the PEs within its own AS, the ASBR MUST be (auto-)configured with an RD for that MVPN. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-14` | If an ASBR is configured to support a particular MVPN, the ASBR MUST participate in the intra-AS MVPN auto-discovery/binding procedures for that MVPN within the ASBR's own AS, as specified in Section 9.1. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.1-1` | An implementation MUST support the default policy for aggregation of Intra-AS I-PMSI A-D routes into an Inter-AS I-PMSI A-D route. (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.1-3` | Modified policy MUST include rules for constructing RTs carried by the Inter-AS I-PMSI A-D routes originated by the ASBR. (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-1` | When re-advertising an Inter-AS I-PMSI A-D route, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-2` | If the ASBR uses ingress replication for the intra-AS segment of the inter-AS tunnel, the re-advertised route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, but no MPLS labels. (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-3` | If the ASBR uses a P-multicast tree for the intra-AS segment of the inter-AS tunnel, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the ASBR could create the identity of the tree prior to the actual instantiation of the tree). (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-5` | An ASBR that uses a P-multicast tree as the intra-AS segment of the inter-AS tunnel MAY aggregate two or more MVPNs present on the ASBR onto the same tree. In this case, in addition to the identity of the tree, the PMSI Tunnel attribute of the Inter-AS I- PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-6` | If the ASBR has already advertised Inter-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the ASBR MUST re-advertise those routes. (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-7` | The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the MVPN label. (§9.2.3.2) | MUST | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-1` | In addition, the ASBR MUST send to the EBGP neighbor from whom it received the Inter-AS I-PMSI A-D route, a BGP Update message that carries a Leaf A-D route constructed as follows. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-2` | The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the ASBR (this MUST be a routable IP address). (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-3` | The Leaf A-D route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and the Tunnel Identifier set to a routable address of the advertising router. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-4` | The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used by the advertising router to demultiplex the MVPN traffic received over a unicast tunnel from the EBGP neighbor. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-5` | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-6` | To constrain the distribution scope of this route, the route MUST carry the NO_ADVERTISE BGP Community [RFC1997]. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-7` | The ASBR MUST set up its forwarding state such that packets that arrive on the one-hop ASBR-ASBR LSP, as specified in the PMSI Tunnel attribute of the Leaf A-D route, are transmitted on the intra-AS segment, as specified in the PMSI Tunnel attribute of the Inter-AS I-PMSI A-D route that the ASBR re-advertises in its own AS. (§9.2.3.2.1) | MUST | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.3-2` | For each of these intra-AS tunnels, if a non-zero MPLS label is carried in the PMSI Tunnel attribute (i.e., aggregation is used), then only packets received on the inner LSP corresponding to that label MUST be forwarded, not the packets received on the outer LSP, as the outer LSP possibly carries the traffic of other VPNs. (§9.2.3.3) | MUST | 9.2.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-1` | If a given Inter-AS I-PMSI A-D route is received via IBGP by a BGP route reflector, the BGP route reflector MUST NOT modify the Next Hop field of the MP_REACH_NLRI attribute when re-advertising the route into IBGP (§9.2.3.4) | MUST NOT | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-2` | When propagating the route to the EBGP neighbors, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.4) | MUST | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-3` | If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute, then the propagated route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication; the attribute carries no MPLS labels. (§9.2.3.4) | MUST | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-5` | If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Identifier set to RSVP-TE P2MP LSP, then the ASBR that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE/ASBR as a leaf. (§9.2.3.4) | MUST | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-1` | If the Leaf Information Required flag in the PMSI Tunnel attribute of the received Inter-AS I-PMSI A-D route is set to 1, then the PE/ASBR MUST originate a new Leaf A-D route as follows. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-2` | The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the PE/ASBR (this MUST be a routable IP address). (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-3` | If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, then the Leaf A-D route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-4` | The Tunnel Identifier MUST carry a routable address of the PE/ASBR. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-5` | The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE/ASBR. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-6` | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4.1-7` | To constrain the distribution scope of this route, the route MUST carry the NO_EXPORT Community [RFC1997]. (§9.2.3.4.1) | MUST | 9.2.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-2` | If there is a separate UMH VRF, it MAY have its own import and export Route Targets, different from the ones used by the unicast VRF. These Route Targets MUST be used to control distribution of auto- discovery routes. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-4` | If an MVPN site is multihomed to several PEs, then to support non- congruent unicast and multicast connectivity, on each of these PEs, the UMH VRF of the MVPN MUST use its own distinct RD (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-6` | These routes MUST carry the VRF Route Import Extended Community. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-7` | If, for a given MVPN, BGP is used for exchanging C-multicast routes, or if segmented inter-AS tunnels are used, then these routes MUST also carry the Source AS Extended Community. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.1.1-1` | Whenever (a) a C-PIM instance on a particular PE creates a new (C-S,C-G) state, and (b) the selected upstream PE for C-S (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Source Tree Join. (§11.1.1.1) | MUST | 11.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.1.1-2` | Whenever a C-PIM instance on a particular PE deletes a (C-S,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.1) | MUST | 11.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.1.2-1` | Whenever (a) a C-PIM instance on a particular PE creates a new (C-*,C-G) state, and (b) the selected upstream PE for the C-RP corresponding to the C-G (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Shared Tree Join. (§11.1.1.2) | MUST | 11.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.1.2-2` | Whenever a C-PIM instance on a particular PE deletes a (C-*,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.2) | MUST | 11.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.3-1` | The Next Hop field of the MP_REACH_NLRI attribute MUST be set to a routable IP address of the local PE. (§11.1.3) | MUST | 11.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.4-1` | In that case, a change in the unicast routing may invalidate prior choices of the UMH route for some C-S. If this happens, the local PE MUST execute the UMH route selection procedures for C-S again. (§11.1.4) | MUST | 11.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.4-2` | If the result is that a different UMH route is selected, then for all C-G, any previously originated C-multicast routes for (C-S,C-G) MUST be re-originated. (§11.1.4) | MUST | 11.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.1.4-3` | Similarly, if a unicast routing change results in a change of the UMH route for a C-RP, then for all C-G such that C-RP is the RP associated with C-G, any previously originated C-multicast routes for (C-*,C-G) MUST be re-originated. (§11.1.4) | MUST | 11.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.2-1` | The ASBR first checks if it already has one or more C-multicast routes that have the same MCAST-VPN NLRI as the newly received route. If such a route(s) already exists, the ASBR keeps the newly received route, but SHALL NOT re-advertise the newly received route. (§11.2) | SHALL NOT | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.2-2` | When an ASBR receives a BGP Update message that carries a withdrawal of a previously advertised C-multicast route, the ASBR first checks if it already has at least one other C-multicast route that has the same MCAST-VPN NLRI. If such a route already exists, the ASBR processes the withdrawn route, but SHALL NOT re-advertise the withdrawal. (§11.2) | SHALL NOT | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3.1.1-1` | When, for a said VRF, the last Source Tree Join C-multicast route for (C-S,C-G) is withdrawn, resulting in the situation where the VRF contains no Source Tree Join C-multicast route for (C-S,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state. (§11.3.1.1) | MUST | 11.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3.1.1-3` | If C-G is not in the SSM range for the VRF, then removing the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§11.3.1.1) | MUST | 11.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3.1.2-1` | When, for a said VRF, the last Shared Tree Join C-multicast route for (C-*,C-G) is withdrawn, resulting in the situation where the VRF contains no Shared Tree Join C-multicast route for (C-*,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-*,C-G) state. (§11.3.1.2) | MUST | 11.3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-1` | The Multicast Source field MUST contain the source address associated with the C-multicast stream, and the Multicast Source Length field is set appropriately to reflect this. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-2` | The Multicast Group field MUST contain the group address associated with the C-multicast stream, and the Multicast Group Length field is set appropriately to reflect this. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-3` | The Originating Router's IP Address field MUST be set to the IP address that the (local) PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-4` | The PMSI Tunnel attribute MUST contain the identity of the P-multicast tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-5` | If a PE originates S-PMSI A-D routes with the Leaf Information Required flag in the PMSI Tunnel attribute set to 1, then the PE MUST be (auto-)configured with an import Route Target, which controls acceptance of Leaf A-D routes by the PE. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-6` | The Global Administrator field of this Route Target MUST be set to the IP address carried in the Next Hop of all the S-PMSI A-D routes advertised by this PE (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-7` | if the PE uses different Next Hops, then the PE MUST be (auto-)configured with multiple import RTs, one per each such Next Hop (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-8` | The Local Administrator field of this Route Target MUST be set to 0. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-12` | If the PE already advertises S-PMSI A-D routes for these S-PMSIs, then aggregation requires the PE to re-advertise these routes. The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-13` | The PMSI Tunnel attribute in the newly advertised/re-advertised routes MUST carry the identity of the P-multicast tree that aggregates the S-PMSIs. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-14` | If at least some of the S-PMSIs aggregated onto the same P-multicast tree belong to different MVPNs, then all these routes MUST carry an MPLS upstream-assigned label [RFC5331]. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-16` | If all these aggregated S-PMSIs belong to the same MVPN, and this MVPN uses PIM as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MAY carry an MPLS upstream-assigned label [RFC5331]. Moreover, in this case, the labels MUST be distinct on a per-MVPN basis and MAY be distinct on a per-route basis. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-18` | If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331] (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-19` | If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331], and these labels MUST be distinct on a per-route (per-mLDP FEC) basis, irrespective of whether the aggregated S-PMSIs belong to the same or different MVPNs. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-20` | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-21` | In each of the above cases, an implementation MUST allow the set of Route Targets carried by the route to be specified by configuration. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-22` | In the absence of a configured set of Route Targets, the route MUST carry the default set of Route Targets, as specified above. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.2.1-3` | If an ASBR merges a (C-S,C-G) S-PMSI A-D route into an Inter-AS I-PMSI A-D route, the ASBR MUST discard all (C-S,C-G) traffic it receives on the tunnel advertised in the I-PMSI A-D route. (§12.2.1) | MUST | 12.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.2.1-4` | An ASBR that merges an S-PMSI A-D route into an Inter-AS I-PMSI A-D route MUST NOT re-advertise the S-PMSI A-D route. (§12.2.1) | MUST NOT | 12.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.3-1` | In addition to the procedures specified in Section 9.2.3.4.1, the PE MUST set up its forwarding path to receive (C-S,C-G) traffic from the tunnel advertised by the S-PMSI A-D route (the PE MUST switch to the S-PMSI). (§12.3) | MUST | 12.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13-1` | The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, they only apply for the multicast ASM mode and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13-2` | The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-1` | When, as a result of receiving a Source Tree Join C-multicast route for (C-S,C-G) from some other PE the local PE adds either the S-PMSI or the I-PMSI to the outgoing interface list of the (C-S,C-G) state (see Section 11.3.1.1), the local PE MUST originate a Source Active A-D route if the PE has not originated such route already. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-2` | The Multicast Source field MUST be set to C-S. The Multicast Source Length field is set appropriately to reflect this. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-3` | The Multicast Group field MUST be set to C-G. The Multicast Group Length field is set appropriately to reflect this. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-4` | The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE from the MVPN's VRF. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-6` | When, as a result of receiving a withdrawal of the previously advertised Source Tree Join C-multicast route for (C-S,C-G), the PE is going to remove the S-PMSI/I-PMSI from the outgoing interface list of the (C-S,C-G) state. The local PE MUST also withdraw the Source Active A-D route for (C-S,C-G), if such a route has been advertised. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2-1` | When (as a result of receiving PIM messages from one of its CEs) a PE creates in one of its MVPN-TIBs a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§13.2) | MUST | 13.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2-2` | When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§13.2) | MUST | 13.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2.1-1` | If the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, and the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G) (where C-S is address carried in the Multicast Source field and C-G is the address carried in the Multicast Group field of the received Source Active A-D route), then the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the Prune state. (§13.2.1) | MUST | 13.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2.1-3` | Transitioning the state machine to the Prune state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§13.2.1) | MUST | 13.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2.1-4` | The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). (§13.2.1) | MUST | 13.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2.1-5` | The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). Once any of these conditions become no longer valid, the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the NoInfo state. (§13.2.1) | MUST | 13.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14-1` | The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, only apply for the multicast ASM mode, and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§14) | MUST NOT | 14 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14-2` | The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§14) | MUST NOT | 14 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.1-1` | The Multicast Source field MUST be set to the source IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1) | MUST | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.1-2` | The Multicast Group field MUST be set to the group IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1) | MUST | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.1-3` | The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§14.1) | MUST | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-1` | When (as a result of receiving PIM messages from one of its CEs) a PE creates, in one of its MVPN-TIBs, a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-2` | If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through some other PE, then for each such route the PE MUST originate a Source Tree Join C-multicast route. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-3` | If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through a CE connected to the PE, then for each such route the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-4` | When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-5` | If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through some other PE, the PE MUST originate a Source Tree Join C-multicast route for the (C-S,C-G) carried by the route. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-6` | If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through a CE connected to the PE, the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-7` | A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if, as a result of having received PIM messages from one of its CEs, the PE creates a Prune (C-S,C-G,rpt) upstream state in one of its MVPN-TIBs but has no (C-S,C-G) Joined state in that MVPN-TIB and had previously advertised the said route. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-8` | A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if the Source Active A-D route that triggered the advertisement of the C-multicast route is withdrawn. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.2-9` | When a PE deletes the (C-*,C-G) state (e.g., due to receiving PIM Prune (C-*,C-G) from its CEs), the PE MUST withdraw all the Source Tree Join C-multicast routes for C-G that have been advertised by the PE, except for the routes for which the PE still maintains the corresponding (C-S,C-G) state. (§14.2) | MUST | 14.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-2` | A PE router MUST NOT accept, from CEs routes, with MCAST-VPN SAFI. (§17) | MUST NOT | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-3` | If BGP is used as a CE-PE routing protocol, then when a PE receives a route from a CE, if this route carries the VRF Route Import Extended Community, the PE MUST remove this Community from the route before turning it into a VPN-IP route. (§17) | MUST | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-4` | Routes that a PE advertises to a CE MUST NOT carry the VRF Route Import Extended Community. (§17) | MUST NOT | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-5` | When the Tunnel Type is set to PIM-SM or BIDIR-PIM tree, then the P-Multicast Group in the Tunnel Identifier of the Tunnel attribute SHOULD contain the same multicast group address for all Intra-AS I-PMSI A-D routes for the same MVPN originated by PEs within a given AS. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-5-6` | When a router that receives a BGP Update that contains the PMSI Tunnel attribute with its Partial bit set determines that the attribute is malformed, the router SHOULD treat this Update as though all the routes contained in this Update had been withdrawn. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-3` | The Global Administrator field of the C-multicast Import RT MUST be set to an IP address of the PE.  This address SHOULD be common for all the VRFs on the PE (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-6` | If a PE uses Route Target Constraint [RT-CONSTRAIN], the PE SHOULD advertise all such C-multicast Import RTs using Route Target Constraints (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-7-7` | To constrain distribution of the Route Target Constraint routes to the AS of the advertising PE, these routes SHOULD carry the NO_EXPORT Community [RFC1997]. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-8-2` | When a router that receives a BGP Update that contains the PE Distinguisher Labels attribute with its Partial bit set determines that the attribute is malformed, the router SHOULD treat this Update as though all the routes contained in this Update had been withdrawn. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-13` | To constrain distribution of the intra-AS membership/binding information to the AS of the advertising PE, the BGP Update message originated by the advertising PE SHOULD carry the NO_EXPORT Community [RFC1997]. (§9.1.1) | SHOULD | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-15` | When BGP is used to exchange C-multicast routes, if (a) it is known a priori that, as a matter of policy, none of the MVPN sites connected to a given PE are allowed to send multicast traffic to other sites of that MVPN (in other words, all these sites are only in the Receiver Sites set), (b) the PE does not use ingress replication for the incoming traffic of that MVPN, and (c) none of the other PEs that have VRFs of that MVPN use RSVP-TE P2MP LSP for that MVPN, then the local PE SHOULD NOT originate an Intra-AS I-PMSI A-D route. (§9.1.1) | SHOULD NOT | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-16` | When BGP is used to exchange C-multicast routes, if it is known a priori that, as a matter of policy, none of the MVPN sites connected to a given PE can receive multicast traffic from other sites of that MVPN (in other words, all these sites are only in the Sender Sites set), and the PE uses ingress replication for that MVPN, then the PE SHOULD NOT originate an Intra-AS I-PMSI A-D route for that MVPN. (§9.1.1) | SHOULD NOT | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.2-1` | If the Tunnel Type in the PMSI Tunnel attribute is set to mLDP P2MP LSP, mLDP MP2MP LSP, PIM-SSM tree, PIM-SM tree, or BIDIR-PIM tree, the PE SHOULD join as soon as possible the P-multicast tree whose identity is carried in the Tunnel Identifier. (§9.1.2) | SHOULD | 9.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-7` | If the ASBR supports Route Target Constraint [RT-CONSTRAIN], the ASBR SHOULD advertise its ASBR Import RT within its own AS using Route Target Constraints. (§9.2) | SHOULD | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-8` | To constrain distribution of the Route Target Constraint routes to the AS of the advertising ASBR, these routes SHOULD carry the NO_EXPORT Community [RFC1997]. (§9.2) | SHOULD | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-13` | To allow more control over spreading MVPN traffic among multiple ASBRs within a given AS, it is recommended that each ASBR have a distinct RD per each MVPN; in which case, such an RD SHOULD be auto-configured. (§9.2) | SHOULD | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-4` | If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Type set to mLDP P2MP LSP, PIM-SSM tree, PIM-SM tree, or BIDIR-PIM tree, the PE/ASBR SHOULD join as soon as possible the P-multicast tree whose identity is carried in the Tunnel Identifier. (§9.2.3.4) | SHOULD | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-5` | on a given PE, the RD used by the UMH VRF SHOULD be the same as the one used by the unicast VRF (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.2-3` | The rest of the Extended Communities attribute of the route SHOULD be passed unmodified. (§11.2) | SHOULD | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.2-4` | The Next Hop field of the MP_REACH_NLRI attribute SHOULD be set to an IP address of the ASBR. (§11.2) | SHOULD | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3-1` | When a PE receives a C-multicast route the PE checks if any of the Route Target Extended Communities carried in the Extended Communities attribute of the route match any of the C-multicast Import RTs associated with the VRFs of any MVPN maintained by the PE.  If no match is found, the PE SHOULD discard the route. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3-2` | Otherwise, (if a match is found), the PE checks if the address carried in the Multicast Source field of the C-multicast route matches one of the (unicast) VPN-IP routes advertised by PE from the VRF.  If no match is found the PE SHOULD discard the route. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3.1.1-2` | If C-G is not in the SSM range for the VRF, then removing the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state SHOULD be done after a delay that is controlled by a timer. (§11.3.1.1) | SHOULD | 11.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.4-1` | a route reflector that re-advertises a C-multicast route SHOULD set the Next Hop field of the MP_REACH_NLRI attribute of the route to an IP address of the route reflector. (§11.4) | SHOULD | 11.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.4-2` | an ASBR that re-advertises a C-multicast route SHOULD set the Next Hop field of the MP_REACH_NLRI attribute of the route to an IP address of the ASBR. (§11.4) | SHOULD | 11.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-9` | If the PE supports Route Target Constraint [RT-CONSTRAIN], the PE SHOULD advertise this import Route Target within its own AS using Route Target Constraints. (§12.1) | SHOULD | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-10` | To constrain distribution of the Route Target Constraint routes to the AS of the advertising PE, these routes SHOULD carry the NO_EXPORT Community [RFC1997]. (§12.1) | SHOULD | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.3-2` | If a PE that is a leaf node of a particular Selective tunnel determines that it no longer needs to receive any of (C-S,C-G)s carried over that tunnel, the PE SHOULD prune itself off that tunnel. (§12.3) | SHOULD | 12.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.1-5` | The route SHOULD carry the same set of Route Targets as the Intra-AS I-PMSI A-D route of the MVPN originated by the PE. (§13.1) | SHOULD | 13.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-13.2.1-2` | Transitioning the state machine to the Prune state SHOULD be done after a delay that is controlled by a timer. (§13.2.1) | SHOULD | 13.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.1-4` | The route SHOULD carry the same set of Route Targets as the Intra-AS I-PMSI A-D route of the MVPN originated by the PE. (§14.1) | SHOULD | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-14.1-5` | When a PE that previously advertised a Source Active A-D route for a given (multicast) source learns that the source is no longer active (the PE learns this by using the same mechanism by which the PE learned that the source was active), the PE SHOULD withdraw the previously advertised Source Active route. (§14.1) | SHOULD | 14.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16-1` | To keep the intra-AS membership/binding information within the AS of the advertising router the BGP Update message originated by the advertising router SHOULD carry the NO_EXPORT Community [RFC1997]. (§16) | SHOULD | 16 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16.1.1-2` | A PE/ASBR/route reflector can OPTIONALLY delay the advertisement of withdrawals of C-multicast routes.  An implementation SHOULD provide the ability to control the delay via a configurable timer, possibly with some backoff algorithm to adapt the delay to multicast routing activity. (§16.1.1) | SHOULD | 16.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16.1.2-2` | A PE/ASBR/route reflector can OPTIONALLY delay the advertisement of Source/Shared Tree Join C-multicast routes.  An implementation SHOULD provide the ability to control the delay via a configurable timer, possibly with some backoff algorithm to adapt the delay to multicast routing activity. (§16.1.2) | SHOULD | 16.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-1` | When the document is finalized as an RFC, the method defined in [RFC5925] SHOULD be used where authentication of BGP control packets is needed. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-5` | When C-multicast routing information is exchanged among PEs using BGP, an implementation SHOULD provide the ability to rate limit BGP messages used for this exchange. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-6` | When C-multicast routing information is exchanged among PEs using BGP, an implementation SHOULD provide the ability to rate limit BGP messages used for this exchange.  This SHOULD be provided on a per- PE, per-MVPN granularity. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-7` | An implementation SHOULD provide capabilities to impose an upper bound on the number of S-PMSI A-D routes, as well as on how frequently they may be originated. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-8` | An implementation SHOULD provide capabilities to impose an upper bound on the number of S-PMSI A-D routes, as well as on how frequently they may be originated.  This SHOULD be provided on a per- PE, per-MVPN granularity. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-9` | In conjunction with the procedures specified in Section 14, an implementation SHOULD provide capabilities to impose an upper bound on the number of Source Active A-D routes, as well as on how frequently they may be originated. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-10` | In conjunction with the procedures specified in Section 14, an implementation SHOULD provide capabilities to impose an upper bound on the number of Source Active A-D routes, as well as on how frequently they may be originated.  This SHOULD be provided on a per- PE, per-MVPN granularity. (§17) | SHOULD | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-12` | If the ASBR originates an Inter-AS I-PMSI A-D route for a particular MVPN present on some of the PEs within its own AS, the ASBR MUST be (auto-)configured with an RD for that MVPN.  It is RECOMMENDED that one of the following two options be used (§9.2) | RECOMMENDED | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-3` | If a PE maintains an UMH VRF for that MVPN, then it is RECOMMENDED that the UMH VRF use the same RD as the one used by the unicast VRF of that MVPN. (§10) | RECOMMENDED | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-15-1` | To improve scalability, it is RECOMMENDED that for the Carrier's Carrier scenario within an AS, all the S-PMSIs of a given MVPN be aggregated into a single P-multicast tree (by using upstream-assigned labels). (§15) | RECOMMENDED | 15 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-13` | Section 16.1.1 describes optional procedures for dampening withdrawals of C-multicast routes.  It is RECOMMENDED that an implementation support such procedures. (§17) | RECOMMENDED | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-14` | Section 16.1.1 describes optional procedures for dampening withdrawals of Leaf A-D routes.  It is RECOMMENDED that an implementation support such procedures. (§17) | RECOMMENDED | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.1.1-3` | A PE that uses a P-multicast tree for the P-tunnel MAY aggregate two or more MVPNs present on the PE onto the same tree. (§9.1.1) | MAY | 9.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-2` | Note that instead of being configured, the ASBR MAY obtain this set of (import) Route Targets (RTs) by using Route Target Constraint [RT-CONSTRAIN]. (§9.2) | MAY | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2-10` | Note that instead of being configured, the ASBR MAY derive the tunnel types from the Intra-AS I-PMSI A-D routes received by the ASBR. (§9.2) | MAY | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.1-2` | The default policy for aggregation could be modified via configuration on the ASBR.  An implementation MAY support such functionality. (§9.2.1) | MAY | 9.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.2-1` | If, for a given MVPN and a given AS, all of the sites connected to the PEs within the AS are known a priori to have no multicast sources, then ASBRs of that AS MAY refrain from originating an Inter- AS I-PMSI A-D route for that MVPN at all. (§9.2.2) | MAY | 9.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2-4` | An ASBR that uses a P-multicast tree as the intra-AS segment of the inter-AS tunnel MAY aggregate two or more MVPNs present on the ASBR onto the same tree. (§9.2.3.2) | MAY | 9.2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.2.1-8` | However, the packets MAY be filtered before forwarding, as specified in Section 9.2.3.6. (§9.2.3.2.1) | MAY | 9.2.3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.3-1` | However, the packets MAY be filtered before transmission as specified in Section 9.2.3.6 (§9.2.3.3) | MAY | 9.2.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.4-6` | This LSP MAY have been established before the local PE/ASBR receives the route, or it MAY be established after the local PE receives the route. (§9.2.3.4) | MAY | 9.2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.6-1` | An ASBR that has a given Inter-AS I-PMSI A-D route MAY discard some of the traffic carried in the tunnel specified in the PMSI Tunnel attribute of this route, if the ASBR determines that there are no downstream receivers for that traffic. (§9.2.3.6) | MAY | 9.2.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.6-2` | When BGP is being used to distribute C-multicast routes, an ASBR that has a given Inter-AS I-PMSI A-D route MAY discard traffic from a particular customer multicast source C-S and destined to a particular customer multicast group address C-G that is carried over the tunnel specified in the PMSI Tunnel attribute of the route, if none of the C-multicast routes on the ASBR with RD and Source AS being the same as the RD and Source AS of the Inter-AS I-PMSI A-D route matches the (C-S,C-G) tuple. (§9.2.3.6) | MAY | 9.2.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-9.2.3.6-3` | The above procedures MAY also apply to an ASBR that originates a given Inter-AS I-PMSI A-D route. (§9.2.3.6) | MAY | 9.2.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-10-1` | If there is a separate UMH VRF, it MAY have its own import and export Route Targets, different from the ones used by the unicast VRF. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-11.3.2-1` | To improve scalability when mLDP is used as the C-Multicast protocol for a given MVPN, within each AS that has sites of that MVPN connected to the PEs of that AS, all the S-PMSIs of that MVPN MAY be aggregated into a single P-multicast tree (by using upstream-assigned labels). (§11.3.2) | MAY | 11.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-11` | A PE MAY aggregate two or more S-PMSIs originated by the PE onto the same P-multicast tree. (§12.1) | MAY | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-15` | If all these aggregated S-PMSIs belong to the same MVPN, and this MVPN uses PIM as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MAY carry an MPLS upstream-assigned label [RFC5331]. (§12.1) | MAY | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.1-17` | Moreover, in this case, the labels MUST be distinct on a per-MVPN basis and MAY be distinct on a per-route basis. (§12.1) | MAY | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.2.1-1` | To support this situation, an ASBR MAY, under certain conditions, merge one or more upstream S-PMSIs into a downstream I-PMSI. (§12.2.1) | MAY | 12.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-12.2.1-2` | An S-PMSI (corresponding to a particular S-PMSI A-D route) MAY be merged by a particular ASBR into an I-PMSI (corresponding to a particular Inter-AS I-PMSI A-D route) if and only if the following conditions all hold (§12.2.1) | MAY | 12.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-11` | However, to provide an extra level of robustness in the context of these procedures, an implementation MAY provide capabilities to impose an upper bound on the number of Source Active A-D routes, as well as on how frequently they may be originated. (§17) | MAY | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-17-12` | However, to provide an extra level of robustness in the context of these procedures, an implementation MAY provide capabilities to impose an upper bound on the number of Source Active A-D routes, as well as on how frequently they may be originated.  This MAY be provided on a per-PE, per-MVPN granularity. (§17) | MAY | 17 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16.1-1` | To lessen the control plane overhead associated with the processing of C-multicast routes, this document proposes OPTIONAL route dampening procedures similar to what is described in [RFC2439].  The following OPTIONAL procedures can be enabled on a PE, ASBR, or BGP Route Reflector advertising or receiving C-multicast routes. (§16.1) | OPTIONAL | 16.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16.1.1-1` | A PE/ASBR/route reflector can OPTIONALLY delay the advertisement of withdrawals of C-multicast routes. (§16.1.1) | OPTIONAL | 16.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6514-16.1.2-1` | A PE/ASBR/route reflector can OPTIONALLY delay the advertisement of Source/Shared Tree Join C-multicast routes. (§16.1.2) | OPTIONAL | 16.1.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6514-4.5-1`](#rfc6514-4.5-1) Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router (§4.5) | no test | no test carries this requirement id |
| [`RFC6514-4.5-2`](#rfc6514-4.5-2) Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router and MUST be discarded if received. (§4.5) | no test | no test carries this requirement id |
| [`RFC6514-5-1`](#rfc6514-5-1) When the Tunnel Type is set to Protocol Independent Multicast - Sparse Mode (PIM-SM) tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE (Generic Routing Encapsulation) encapsulation of the MVPN data. (§5) | no test | no test carries this requirement id |
| [`RFC6514-5-2`](#rfc6514-5-2) When the Tunnel Type is set to PIM-SSM tree, the Tunnel Identifier is <P-Root Node Address, P-Multicast Group>. The node that originates the attribute MUST use the address carried in the P-Root Node Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5) | no test | no test carries this requirement id |
| [`RFC6514-5-3`](#rfc6514-5-3) The P-Multicast Group in the Tunnel Identifier of the Tunnel attribute MUST NOT be expected to be the same group for all Intra-AS A-D routes for the same MVPN. (§5) | no test | no test carries this requirement id |
| [`RFC6514-5-4`](#rfc6514-5-4) When the Tunnel Type is set to BIDIR-PIM tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5) | no test | no test carries this requirement id |
| [`RFC6514-5-7`](#rfc6514-5-7) An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. (§5) | no test | no test carries this requirement id |
| [`RFC6514-5-8`](#rfc6514-5-8) An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§5) | no test | no test carries this requirement id |
| [`RFC6514-6-1`](#rfc6514-6-1) The Global Administrator field of this Community MUST be set to the ASN of the PE. (§6) | no test | no test carries this requirement id |
| [`RFC6514-6-2`](#rfc6514-6-2) The Local Administrator field of this Community MUST be set to 0. (§6) | no test | no test carries this requirement id |
| [`RFC6514-6-3`](#rfc6514-6-3) A PE that has sites of that MVPN connected to it, and originates a (unicast) route to VPN-IP addresses associated with the destinations within these sites, MUST include in the BGP Update message that carries this route the Source AS Extended Community. (§6) | no test | no test carries this requirement id |
| [`RFC6514-7-1`](#rfc6514-7-1) To support MVPN in addition to the import/export Route Target(s) Extended Communities used by the unicast routing, each VRF on a PE MUST have an import Route Target Extended Community, except if it is known a priori that none of the (local) MVPN sites associated with the VRF contain multicast source(s) and/or C-RP; in which case, the VRF need not have this import Route Target. (§7) | no test | no test carries this requirement id |
| [`RFC6514-7-2`](#rfc6514-7-2) The Global Administrator field of the C-multicast Import RT MUST be set to an IP address of the PE. (§7) | no test | no test carries this requirement id |
| [`RFC6514-7-4`](#rfc6514-7-4) To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route (§7) | no test | no test carries this requirement id |
| [`RFC6514-7-5`](#rfc6514-7-5) To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route, except if it is known a priori (e.g., via provisioning) that none of these addresses could act as multicast sources and/or RP; in which case, the (unicast) route MUST NOT carry the VRF Route Import Extended Community. (§7) | no test | no test carries this requirement id |
| [`RFC6514-8-1`](#rfc6514-8-1) Each of the PE addresses in the PE Distinguisher Labels attribute MUST be of the same address family as the "Originating Router's IP Address" of the route that is carrying the attribute. (§8) | no test | no test carries this requirement id |
| [`RFC6514-8-3`](#rfc6514-8-3) An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. (§8) | no test | no test carries this requirement id |
| [`RFC6514-8-4`](#rfc6514-8-4) An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§8) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-1`](#rfc6514-9.1.1-1) To participate in the MVPN auto-discovery/binding, a PE router that has a given VRF of a given MVPN MUST, except for the cases specified in this section, originate an Intra-AS I-PMSI A-D route and advertises this route in IBGP. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-2`](#rfc6514-9.1.1-2) If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for the MVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-4`](#rfc6514-9.1.1-4) A PE that uses a P-multicast tree for the P-tunnel MAY aggregate two or more MVPNs present on the PE onto the same tree. In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute of the Intra-AS I-PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-5`](#rfc6514-9.1.1-5) If the PE has already advertised Intra-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-6`](#rfc6514-9.1.1-6) The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-7`](#rfc6514-9.1.1-7) If the PE that originates the advertisement uses ingress replication for the P-tunnel for the MVPN, the route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and Tunnel Identifier set to a routable address of the PE. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-8`](#rfc6514-9.1.1-8) The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label. This label is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-9`](#rfc6514-9.1.1-9) The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-10`](#rfc6514-9.1.1-10) The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-11`](#rfc6514-9.1.1-11) The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-12`](#rfc6514-9.1.1-12) That is, by default, the Intra-AS I-PMSI A-D route MUST carry the export Route Target used by the unicast routing. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.1-14`](#rfc6514-9.1.1-14) Note that if non-segmented inter-AS P-tunnels are being used, then the Intra-AS I-PMSI routes need to be distributed to other ASes and MUST NOT carry the NO_EXPORT Community. (§9.1.1) | no test | no test carries this requirement id |
| [`RFC6514-9.1.2-2`](#rfc6514-9.1.2-2) If the Tunnel Type in the PMSI Tunnel attribute is set to RSVP-TE P2MP LSP, then the PE that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE as a leaf. (§9.1.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-1`](#rfc6514-9.2-1) An ASBR MUST be configured with a set of (import) Route Targets (RTs) that specifies the set of MVPNs supported by the ASBR. (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-3`](#rfc6514-9.2-3) The ASBR MUST be (auto-)configured with an import Route Target called "ASBR Import RT". (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-4`](#rfc6514-9.2-4) The Global Administrator field of the ASBR Import RT MUST be set to the IP address carried in the Next Hop of all the Inter-AS I-PMSI A-D routes and S-PMSI A-D routes advertised by this ASBR (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-5`](#rfc6514-9.2-5) if the ASBR uses different Next Hops, then the ASBR MUST be (auto-)configured with multiple ASBR Import RTs, one per each such Next Hop (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-6`](#rfc6514-9.2-6) The Local Administrator field of the ASBR Import RT MUST be set to 0. (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-9`](#rfc6514-9.2-9) The ASBR MUST be configured with the tunnel types for the intra-AS segments of the MVPNs supported by the ASBR, as well as (depending on the tunnel type) the information needed to create the PMSI attribute for these tunnel types. (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-11`](#rfc6514-9.2-11) If the ASBR originates an Inter-AS I-PMSI A-D route for a particular MVPN present on some of the PEs within its own AS, the ASBR MUST be (auto-)configured with an RD for that MVPN. (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2-14`](#rfc6514-9.2-14) If an ASBR is configured to support a particular MVPN, the ASBR MUST participate in the intra-AS MVPN auto-discovery/binding procedures for that MVPN within the ASBR's own AS, as specified in Section 9.1. (§9.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.1-1`](#rfc6514-9.2.1-1) An implementation MUST support the default policy for aggregation of Intra-AS I-PMSI A-D routes into an Inter-AS I-PMSI A-D route. (§9.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.1-3`](#rfc6514-9.2.1-3) Modified policy MUST include rules for constructing RTs carried by the Inter-AS I-PMSI A-D routes originated by the ASBR. (§9.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-1`](#rfc6514-9.2.3.2-1) When re-advertising an Inter-AS I-PMSI A-D route, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-2`](#rfc6514-9.2.3.2-2) If the ASBR uses ingress replication for the intra-AS segment of the inter-AS tunnel, the re-advertised route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, but no MPLS labels. (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-3`](#rfc6514-9.2.3.2-3) If the ASBR uses a P-multicast tree for the intra-AS segment of the inter-AS tunnel, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the ASBR could create the identity of the tree prior to the actual instantiation of the tree). (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-5`](#rfc6514-9.2.3.2-5) An ASBR that uses a P-multicast tree as the intra-AS segment of the inter-AS tunnel MAY aggregate two or more MVPNs present on the ASBR onto the same tree. In this case, in addition to the identity of the tree, the PMSI Tunnel attribute of the Inter-AS I- PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-6`](#rfc6514-9.2.3.2-6) If the ASBR has already advertised Inter-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the ASBR MUST re-advertise those routes. (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2-7`](#rfc6514-9.2.3.2-7) The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the MVPN label. (§9.2.3.2) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-1`](#rfc6514-9.2.3.2.1-1) In addition, the ASBR MUST send to the EBGP neighbor from whom it received the Inter-AS I-PMSI A-D route, a BGP Update message that carries a Leaf A-D route constructed as follows. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-2`](#rfc6514-9.2.3.2.1-2) The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the ASBR (this MUST be a routable IP address). (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-3`](#rfc6514-9.2.3.2.1-3) The Leaf A-D route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and the Tunnel Identifier set to a routable address of the advertising router. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-4`](#rfc6514-9.2.3.2.1-4) The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used by the advertising router to demultiplex the MVPN traffic received over a unicast tunnel from the EBGP neighbor. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-5`](#rfc6514-9.2.3.2.1-5) The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-6`](#rfc6514-9.2.3.2.1-6) To constrain the distribution scope of this route, the route MUST carry the NO_ADVERTISE BGP Community [RFC1997]. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.2.1-7`](#rfc6514-9.2.3.2.1-7) The ASBR MUST set up its forwarding state such that packets that arrive on the one-hop ASBR-ASBR LSP, as specified in the PMSI Tunnel attribute of the Leaf A-D route, are transmitted on the intra-AS segment, as specified in the PMSI Tunnel attribute of the Inter-AS I-PMSI A-D route that the ASBR re-advertises in its own AS. (§9.2.3.2.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.3-2`](#rfc6514-9.2.3.3-2) For each of these intra-AS tunnels, if a non-zero MPLS label is carried in the PMSI Tunnel attribute (i.e., aggregation is used), then only packets received on the inner LSP corresponding to that label MUST be forwarded, not the packets received on the outer LSP, as the outer LSP possibly carries the traffic of other VPNs. (§9.2.3.3) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4-1`](#rfc6514-9.2.3.4-1) If a given Inter-AS I-PMSI A-D route is received via IBGP by a BGP route reflector, the BGP route reflector MUST NOT modify the Next Hop field of the MP_REACH_NLRI attribute when re-advertising the route into IBGP (§9.2.3.4) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4-2`](#rfc6514-9.2.3.4-2) When propagating the route to the EBGP neighbors, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.4) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4-3`](#rfc6514-9.2.3.4-3) If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute, then the propagated route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication; the attribute carries no MPLS labels. (§9.2.3.4) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4-5`](#rfc6514-9.2.3.4-5) If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Identifier set to RSVP-TE P2MP LSP, then the ASBR that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE/ASBR as a leaf. (§9.2.3.4) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-1`](#rfc6514-9.2.3.4.1-1) If the Leaf Information Required flag in the PMSI Tunnel attribute of the received Inter-AS I-PMSI A-D route is set to 1, then the PE/ASBR MUST originate a new Leaf A-D route as follows. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-2`](#rfc6514-9.2.3.4.1-2) The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the PE/ASBR (this MUST be a routable IP address). (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-3`](#rfc6514-9.2.3.4.1-3) If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, then the Leaf A-D route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-4`](#rfc6514-9.2.3.4.1-4) The Tunnel Identifier MUST carry a routable address of the PE/ASBR. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-5`](#rfc6514-9.2.3.4.1-5) The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE/ASBR. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-6`](#rfc6514-9.2.3.4.1-6) The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-9.2.3.4.1-7`](#rfc6514-9.2.3.4.1-7) To constrain the distribution scope of this route, the route MUST carry the NO_EXPORT Community [RFC1997]. (§9.2.3.4.1) | no test | no test carries this requirement id |
| [`RFC6514-10-2`](#rfc6514-10-2) If there is a separate UMH VRF, it MAY have its own import and export Route Targets, different from the ones used by the unicast VRF. These Route Targets MUST be used to control distribution of auto- discovery routes. (§10) | no test | no test carries this requirement id |
| [`RFC6514-10-4`](#rfc6514-10-4) If an MVPN site is multihomed to several PEs, then to support non- congruent unicast and multicast connectivity, on each of these PEs, the UMH VRF of the MVPN MUST use its own distinct RD (§10) | no test | no test carries this requirement id |
| [`RFC6514-10-6`](#rfc6514-10-6) These routes MUST carry the VRF Route Import Extended Community. (§10) | no test | no test carries this requirement id |
| [`RFC6514-10-7`](#rfc6514-10-7) If, for a given MVPN, BGP is used for exchanging C-multicast routes, or if segmented inter-AS tunnels are used, then these routes MUST also carry the Source AS Extended Community. (§10) | no test | no test carries this requirement id |
| [`RFC6514-11.1.1.1-1`](#rfc6514-11.1.1.1-1) Whenever (a) a C-PIM instance on a particular PE creates a new (C-S,C-G) state, and (b) the selected upstream PE for C-S (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Source Tree Join. (§11.1.1.1) | no test | no test carries this requirement id |
| [`RFC6514-11.1.1.1-2`](#rfc6514-11.1.1.1-2) Whenever a C-PIM instance on a particular PE deletes a (C-S,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.1) | no test | no test carries this requirement id |
| [`RFC6514-11.1.1.2-1`](#rfc6514-11.1.1.2-1) Whenever (a) a C-PIM instance on a particular PE creates a new (C-*,C-G) state, and (b) the selected upstream PE for the C-RP corresponding to the C-G (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Shared Tree Join. (§11.1.1.2) | no test | no test carries this requirement id |
| [`RFC6514-11.1.1.2-2`](#rfc6514-11.1.1.2-2) Whenever a C-PIM instance on a particular PE deletes a (C-*,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.2) | no test | no test carries this requirement id |
| [`RFC6514-11.1.3-1`](#rfc6514-11.1.3-1) The Next Hop field of the MP_REACH_NLRI attribute MUST be set to a routable IP address of the local PE. (§11.1.3) | no test | no test carries this requirement id |
| [`RFC6514-11.1.4-1`](#rfc6514-11.1.4-1) In that case, a change in the unicast routing may invalidate prior choices of the UMH route for some C-S. If this happens, the local PE MUST execute the UMH route selection procedures for C-S again. (§11.1.4) | no test | no test carries this requirement id |
| [`RFC6514-11.1.4-2`](#rfc6514-11.1.4-2) If the result is that a different UMH route is selected, then for all C-G, any previously originated C-multicast routes for (C-S,C-G) MUST be re-originated. (§11.1.4) | no test | no test carries this requirement id |
| [`RFC6514-11.1.4-3`](#rfc6514-11.1.4-3) Similarly, if a unicast routing change results in a change of the UMH route for a C-RP, then for all C-G such that C-RP is the RP associated with C-G, any previously originated C-multicast routes for (C-*,C-G) MUST be re-originated. (§11.1.4) | no test | no test carries this requirement id |
| [`RFC6514-11.2-1`](#rfc6514-11.2-1) The ASBR first checks if it already has one or more C-multicast routes that have the same MCAST-VPN NLRI as the newly received route. If such a route(s) already exists, the ASBR keeps the newly received route, but SHALL NOT re-advertise the newly received route. (§11.2) | no test | no test carries this requirement id |
| [`RFC6514-11.2-2`](#rfc6514-11.2-2) When an ASBR receives a BGP Update message that carries a withdrawal of a previously advertised C-multicast route, the ASBR first checks if it already has at least one other C-multicast route that has the same MCAST-VPN NLRI. If such a route already exists, the ASBR processes the withdrawn route, but SHALL NOT re-advertise the withdrawal. (§11.2) | no test | no test carries this requirement id |
| [`RFC6514-11.3.1.1-1`](#rfc6514-11.3.1.1-1) When, for a said VRF, the last Source Tree Join C-multicast route for (C-S,C-G) is withdrawn, resulting in the situation where the VRF contains no Source Tree Join C-multicast route for (C-S,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state. (§11.3.1.1) | no test | no test carries this requirement id |
| [`RFC6514-11.3.1.1-3`](#rfc6514-11.3.1.1-3) If C-G is not in the SSM range for the VRF, then removing the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§11.3.1.1) | no test | no test carries this requirement id |
| [`RFC6514-11.3.1.2-1`](#rfc6514-11.3.1.2-1) When, for a said VRF, the last Shared Tree Join C-multicast route for (C-*,C-G) is withdrawn, resulting in the situation where the VRF contains no Shared Tree Join C-multicast route for (C-*,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-*,C-G) state. (§11.3.1.2) | no test | no test carries this requirement id |
| [`RFC6514-12.1-1`](#rfc6514-12.1-1) The Multicast Source field MUST contain the source address associated with the C-multicast stream, and the Multicast Source Length field is set appropriately to reflect this. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-2`](#rfc6514-12.1-2) The Multicast Group field MUST contain the group address associated with the C-multicast stream, and the Multicast Group Length field is set appropriately to reflect this. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-3`](#rfc6514-12.1-3) The Originating Router's IP Address field MUST be set to the IP address that the (local) PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-4`](#rfc6514-12.1-4) The PMSI Tunnel attribute MUST contain the identity of the P-multicast tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-5`](#rfc6514-12.1-5) If a PE originates S-PMSI A-D routes with the Leaf Information Required flag in the PMSI Tunnel attribute set to 1, then the PE MUST be (auto-)configured with an import Route Target, which controls acceptance of Leaf A-D routes by the PE. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-6`](#rfc6514-12.1-6) The Global Administrator field of this Route Target MUST be set to the IP address carried in the Next Hop of all the S-PMSI A-D routes advertised by this PE (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-7`](#rfc6514-12.1-7) if the PE uses different Next Hops, then the PE MUST be (auto-)configured with multiple import RTs, one per each such Next Hop (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-8`](#rfc6514-12.1-8) The Local Administrator field of this Route Target MUST be set to 0. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-12`](#rfc6514-12.1-12) If the PE already advertises S-PMSI A-D routes for these S-PMSIs, then aggregation requires the PE to re-advertise these routes. The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-13`](#rfc6514-12.1-13) The PMSI Tunnel attribute in the newly advertised/re-advertised routes MUST carry the identity of the P-multicast tree that aggregates the S-PMSIs. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-14`](#rfc6514-12.1-14) If at least some of the S-PMSIs aggregated onto the same P-multicast tree belong to different MVPNs, then all these routes MUST carry an MPLS upstream-assigned label [RFC5331]. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-16`](#rfc6514-12.1-16) If all these aggregated S-PMSIs belong to the same MVPN, and this MVPN uses PIM as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MAY carry an MPLS upstream-assigned label [RFC5331]. Moreover, in this case, the labels MUST be distinct on a per-MVPN basis and MAY be distinct on a per-route basis. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-18`](#rfc6514-12.1-18) If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331] (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-19`](#rfc6514-12.1-19) If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331], and these labels MUST be distinct on a per-route (per-mLDP FEC) basis, irrespective of whether the aggregated S-PMSIs belong to the same or different MVPNs. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-20`](#rfc6514-12.1-20) The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-21`](#rfc6514-12.1-21) In each of the above cases, an implementation MUST allow the set of Route Targets carried by the route to be specified by configuration. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.1-22`](#rfc6514-12.1-22) In the absence of a configured set of Route Targets, the route MUST carry the default set of Route Targets, as specified above. (§12.1) | no test | no test carries this requirement id |
| [`RFC6514-12.2.1-3`](#rfc6514-12.2.1-3) If an ASBR merges a (C-S,C-G) S-PMSI A-D route into an Inter-AS I-PMSI A-D route, the ASBR MUST discard all (C-S,C-G) traffic it receives on the tunnel advertised in the I-PMSI A-D route. (§12.2.1) | no test | no test carries this requirement id |
| [`RFC6514-12.2.1-4`](#rfc6514-12.2.1-4) An ASBR that merges an S-PMSI A-D route into an Inter-AS I-PMSI A-D route MUST NOT re-advertise the S-PMSI A-D route. (§12.2.1) | no test | no test carries this requirement id |
| [`RFC6514-12.3-1`](#rfc6514-12.3-1) In addition to the procedures specified in Section 9.2.3.4.1, the PE MUST set up its forwarding path to receive (C-S,C-G) traffic from the tunnel advertised by the S-PMSI A-D route (the PE MUST switch to the S-PMSI). (§12.3) | no test | no test carries this requirement id |
| [`RFC6514-13-1`](#rfc6514-13-1) The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, they only apply for the multicast ASM mode and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§13) | no test | no test carries this requirement id |
| [`RFC6514-13-2`](#rfc6514-13-2) The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§13) | no test | no test carries this requirement id |
| [`RFC6514-13.1-1`](#rfc6514-13.1-1) When, as a result of receiving a Source Tree Join C-multicast route for (C-S,C-G) from some other PE the local PE adds either the S-PMSI or the I-PMSI to the outgoing interface list of the (C-S,C-G) state (see Section 11.3.1.1), the local PE MUST originate a Source Active A-D route if the PE has not originated such route already. (§13.1) | no test | no test carries this requirement id |
| [`RFC6514-13.1-2`](#rfc6514-13.1-2) The Multicast Source field MUST be set to C-S. The Multicast Source Length field is set appropriately to reflect this. (§13.1) | no test | no test carries this requirement id |
| [`RFC6514-13.1-3`](#rfc6514-13.1-3) The Multicast Group field MUST be set to C-G. The Multicast Group Length field is set appropriately to reflect this. (§13.1) | no test | no test carries this requirement id |
| [`RFC6514-13.1-4`](#rfc6514-13.1-4) The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE from the MVPN's VRF. (§13.1) | no test | no test carries this requirement id |
| [`RFC6514-13.1-6`](#rfc6514-13.1-6) When, as a result of receiving a withdrawal of the previously advertised Source Tree Join C-multicast route for (C-S,C-G), the PE is going to remove the S-PMSI/I-PMSI from the outgoing interface list of the (C-S,C-G) state. The local PE MUST also withdraw the Source Active A-D route for (C-S,C-G), if such a route has been advertised. (§13.1) | no test | no test carries this requirement id |
| [`RFC6514-13.2-1`](#rfc6514-13.2-1) When (as a result of receiving PIM messages from one of its CEs) a PE creates in one of its MVPN-TIBs a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§13.2) | no test | no test carries this requirement id |
| [`RFC6514-13.2-2`](#rfc6514-13.2-2) When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§13.2) | no test | no test carries this requirement id |
| [`RFC6514-13.2.1-1`](#rfc6514-13.2.1-1) If the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, and the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G) (where C-S is address carried in the Multicast Source field and C-G is the address carried in the Multicast Group field of the received Source Active A-D route), then the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the Prune state. (§13.2.1) | no test | no test carries this requirement id |
| [`RFC6514-13.2.1-3`](#rfc6514-13.2.1-3) Transitioning the state machine to the Prune state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§13.2.1) | no test | no test carries this requirement id |
| [`RFC6514-13.2.1-4`](#rfc6514-13.2.1-4) The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). (§13.2.1) | no test | no test carries this requirement id |
| [`RFC6514-13.2.1-5`](#rfc6514-13.2.1-5) The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). Once any of these conditions become no longer valid, the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the NoInfo state. (§13.2.1) | no test | no test carries this requirement id |
| [`RFC6514-14-1`](#rfc6514-14-1) The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, only apply for the multicast ASM mode, and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§14) | no test | no test carries this requirement id |
| [`RFC6514-14-2`](#rfc6514-14-2) The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§14) | no test | no test carries this requirement id |
| [`RFC6514-14.1-1`](#rfc6514-14.1-1) The Multicast Source field MUST be set to the source IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1) | no test | no test carries this requirement id |
| [`RFC6514-14.1-2`](#rfc6514-14.1-2) The Multicast Group field MUST be set to the group IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1) | no test | no test carries this requirement id |
| [`RFC6514-14.1-3`](#rfc6514-14.1-3) The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§14.1) | no test | no test carries this requirement id |
| [`RFC6514-14.2-1`](#rfc6514-14.2-1) When (as a result of receiving PIM messages from one of its CEs) a PE creates, in one of its MVPN-TIBs, a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-2`](#rfc6514-14.2-2) If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through some other PE, then for each such route the PE MUST originate a Source Tree Join C-multicast route. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-3`](#rfc6514-14.2-3) If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through a CE connected to the PE, then for each such route the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-4`](#rfc6514-14.2-4) When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-5`](#rfc6514-14.2-5) If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through some other PE, the PE MUST originate a Source Tree Join C-multicast route for the (C-S,C-G) carried by the route. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-6`](#rfc6514-14.2-6) If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through a CE connected to the PE, the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-7`](#rfc6514-14.2-7) A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if, as a result of having received PIM messages from one of its CEs, the PE creates a Prune (C-S,C-G,rpt) upstream state in one of its MVPN-TIBs but has no (C-S,C-G) Joined state in that MVPN-TIB and had previously advertised the said route. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-8`](#rfc6514-14.2-8) A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if the Source Active A-D route that triggered the advertisement of the C-multicast route is withdrawn. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-14.2-9`](#rfc6514-14.2-9) When a PE deletes the (C-*,C-G) state (e.g., due to receiving PIM Prune (C-*,C-G) from its CEs), the PE MUST withdraw all the Source Tree Join C-multicast routes for C-G that have been advertised by the PE, except for the routes for which the PE still maintains the corresponding (C-S,C-G) state. (§14.2) | no test | no test carries this requirement id |
| [`RFC6514-17-2`](#rfc6514-17-2) A PE router MUST NOT accept, from CEs routes, with MCAST-VPN SAFI. (§17) | no test | no test carries this requirement id |
| [`RFC6514-17-3`](#rfc6514-17-3) If BGP is used as a CE-PE routing protocol, then when a PE receives a route from a CE, if this route carries the VRF Route Import Extended Community, the PE MUST remove this Community from the route before turning it into a VPN-IP route. (§17) | no test | no test carries this requirement id |
| [`RFC6514-17-4`](#rfc6514-17-4) Routes that a PE advertises to a CE MUST NOT carry the VRF Route Import Extended Community. (§17) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6514-4.5-1`](#rfc6514-4.5-1)

Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-4.5-1, so no unit is bound to it.

### [`RFC6514-4.5-2`](#rfc6514-4.5-2)

Source Active A-D routes with a Multicast group belonging to the Source Specific Multicast (SSM) range (as defined in [RFC4607], and potentially extended locally on a router) MUST NOT be advertised by a router and MUST be discarded if received. (§4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-4.5-2, so no unit is bound to it.

### [`RFC6514-5-1`](#rfc6514-5-1)

When the Tunnel Type is set to Protocol Independent Multicast - Sparse Mode (PIM-SM) tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE (Generic Routing Encapsulation) encapsulation of the MVPN data. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-1, so no unit is bound to it.

### [`RFC6514-5-2`](#rfc6514-5-2)

When the Tunnel Type is set to PIM-SSM tree, the Tunnel Identifier is <P-Root Node Address, P-Multicast Group>. The node that originates the attribute MUST use the address carried in the P-Root Node Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-2, so no unit is bound to it.

### [`RFC6514-5-3`](#rfc6514-5-3)

The P-Multicast Group in the Tunnel Identifier of the Tunnel attribute MUST NOT be expected to be the same group for all Intra-AS A-D routes for the same MVPN. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-3, so no unit is bound to it.

### [`RFC6514-5-4`](#rfc6514-5-4)

When the Tunnel Type is set to BIDIR-PIM tree, the Tunnel Identifier is <Sender Address, P-Multicast Group>. The node that originated the attribute MUST use the address carried in the Sender Address as the source IP address for the IP/GRE encapsulation of the MVPN data. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-4, so no unit is bound to it.

### [`RFC6514-5-7`](#rfc6514-5-7)

An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-7, so no unit is bound to it.

### [`RFC6514-5-8`](#rfc6514-5-8)

An implementation MUST provide debugging facilities to permit issues caused by a malformed PMSI Tunnel attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-5-8, so no unit is bound to it.

### [`RFC6514-6-1`](#rfc6514-6-1)

The Global Administrator field of this Community MUST be set to the ASN of the PE. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-6-1, so no unit is bound to it.

### [`RFC6514-6-2`](#rfc6514-6-2)

The Local Administrator field of this Community MUST be set to 0. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-6-2, so no unit is bound to it.

### [`RFC6514-6-3`](#rfc6514-6-3)

A PE that has sites of that MVPN connected to it, and originates a (unicast) route to VPN-IP addresses associated with the destinations within these sites, MUST include in the BGP Update message that carries this route the Source AS Extended Community. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-6-3, so no unit is bound to it.

### [`RFC6514-7-1`](#rfc6514-7-1)

To support MVPN in addition to the import/export Route Target(s) Extended Communities used by the unicast routing, each VRF on a PE MUST have an import Route Target Extended Community, except if it is known a priori that none of the (local) MVPN sites associated with the VRF contain multicast source(s) and/or C-RP; in which case, the VRF need not have this import Route Target. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-7-1, so no unit is bound to it.

### [`RFC6514-7-2`](#rfc6514-7-2)

The Global Administrator field of the C-multicast Import RT MUST be set to an IP address of the PE. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-7-2, so no unit is bound to it.

### [`RFC6514-7-4`](#rfc6514-7-4)

To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-7-4, so no unit is bound to it.

### [`RFC6514-7-5`](#rfc6514-7-5)

To accomplish this, a PE that originates a (unicast) route to VPN-IP addresses MUST include in the BGP Updates message that carries this route the VRF Route Import Extended Community that has the value of the C-multicast Import RT of the VRF associated with the route, except if it is known a priori (e.g., via provisioning) that none of these addresses could act as multicast sources and/or RP; in which case, the (unicast) route MUST NOT carry the VRF Route Import Extended Community. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-7-5, so no unit is bound to it.

### [`RFC6514-8-1`](#rfc6514-8-1)

Each of the PE addresses in the PE Distinguisher Labels attribute MUST be of the same address family as the "Originating Router's IP Address" of the route that is carrying the attribute. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-8-1, so no unit is bound to it.

### [`RFC6514-8-3`](#rfc6514-8-3)

An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-8-3, so no unit is bound to it.

### [`RFC6514-8-4`](#rfc6514-8-4)

An implementation MUST provide debugging facilities to permit issues caused by malformed PE Distinguisher Label attribute to be diagnosed. At a minimum, such facilities MUST include logging an error when such an attribute is detected. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-8-4, so no unit is bound to it.

### [`RFC6514-9.1.1-1`](#rfc6514-9.1.1-1)

To participate in the MVPN auto-discovery/binding, a PE router that has a given VRF of a given MVPN MUST, except for the cases specified in this section, originate an Intra-AS I-PMSI A-D route and advertises this route in IBGP. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-1, so no unit is bound to it.

### [`RFC6514-9.1.1-2`](#rfc6514-9.1.1-2)

If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for the MVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-2, so no unit is bound to it.

### [`RFC6514-9.1.1-4`](#rfc6514-9.1.1-4)

A PE that uses a P-multicast tree for the P-tunnel MAY aggregate two or more MVPNs present on the PE onto the same tree. In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute of the Intra-AS I-PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-4, so no unit is bound to it.

### [`RFC6514-9.1.1-5`](#rfc6514-9.1.1-5)

If the PE has already advertised Intra-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-5, so no unit is bound to it.

### [`RFC6514-9.1.1-6`](#rfc6514-9.1.1-6)

The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-6, so no unit is bound to it.

### [`RFC6514-9.1.1-7`](#rfc6514-9.1.1-7)

If the PE that originates the advertisement uses ingress replication for the P-tunnel for the MVPN, the route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and Tunnel Identifier set to a routable address of the PE. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-7, so no unit is bound to it.

### [`RFC6514-9.1.1-8`](#rfc6514-9.1.1-8)

The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label. This label is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-8, so no unit is bound to it.

### [`RFC6514-9.1.1-9`](#rfc6514-9.1.1-9)

The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-9, so no unit is bound to it.

### [`RFC6514-9.1.1-10`](#rfc6514-9.1.1-10)

The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-10, so no unit is bound to it.

### [`RFC6514-9.1.1-11`](#rfc6514-9.1.1-11)

The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-11, so no unit is bound to it.

### [`RFC6514-9.1.1-12`](#rfc6514-9.1.1-12)

That is, by default, the Intra-AS I-PMSI A-D route MUST carry the export Route Target used by the unicast routing. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-12, so no unit is bound to it.

### [`RFC6514-9.1.1-14`](#rfc6514-9.1.1-14)

Note that if non-segmented inter-AS P-tunnels are being used, then the Intra-AS I-PMSI routes need to be distributed to other ASes and MUST NOT carry the NO_EXPORT Community. (§9.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.1-14, so no unit is bound to it.

### [`RFC6514-9.1.2-2`](#rfc6514-9.1.2-2)

If the Tunnel Type in the PMSI Tunnel attribute is set to RSVP-TE P2MP LSP, then the PE that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE as a leaf. (§9.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.1.2-2, so no unit is bound to it.

### [`RFC6514-9.2-1`](#rfc6514-9.2-1)

An ASBR MUST be configured with a set of (import) Route Targets (RTs) that specifies the set of MVPNs supported by the ASBR. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-1, so no unit is bound to it.

### [`RFC6514-9.2-3`](#rfc6514-9.2-3)

The ASBR MUST be (auto-)configured with an import Route Target called "ASBR Import RT". (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-3, so no unit is bound to it.

### [`RFC6514-9.2-4`](#rfc6514-9.2-4)

The Global Administrator field of the ASBR Import RT MUST be set to the IP address carried in the Next Hop of all the Inter-AS I-PMSI A-D routes and S-PMSI A-D routes advertised by this ASBR (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-4, so no unit is bound to it.

### [`RFC6514-9.2-5`](#rfc6514-9.2-5)

if the ASBR uses different Next Hops, then the ASBR MUST be (auto-)configured with multiple ASBR Import RTs, one per each such Next Hop (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-5, so no unit is bound to it.

### [`RFC6514-9.2-6`](#rfc6514-9.2-6)

The Local Administrator field of the ASBR Import RT MUST be set to 0. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-6, so no unit is bound to it.

### [`RFC6514-9.2-9`](#rfc6514-9.2-9)

The ASBR MUST be configured with the tunnel types for the intra-AS segments of the MVPNs supported by the ASBR, as well as (depending on the tunnel type) the information needed to create the PMSI attribute for these tunnel types. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-9, so no unit is bound to it.

### [`RFC6514-9.2-11`](#rfc6514-9.2-11)

If the ASBR originates an Inter-AS I-PMSI A-D route for a particular MVPN present on some of the PEs within its own AS, the ASBR MUST be (auto-)configured with an RD for that MVPN. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-11, so no unit is bound to it.

### [`RFC6514-9.2-14`](#rfc6514-9.2-14)

If an ASBR is configured to support a particular MVPN, the ASBR MUST participate in the intra-AS MVPN auto-discovery/binding procedures for that MVPN within the ASBR's own AS, as specified in Section 9.1. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2-14, so no unit is bound to it.

### [`RFC6514-9.2.1-1`](#rfc6514-9.2.1-1)

An implementation MUST support the default policy for aggregation of Intra-AS I-PMSI A-D routes into an Inter-AS I-PMSI A-D route. (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.1-1, so no unit is bound to it.

### [`RFC6514-9.2.1-3`](#rfc6514-9.2.1-3)

Modified policy MUST include rules for constructing RTs carried by the Inter-AS I-PMSI A-D routes originated by the ASBR. (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.1-3, so no unit is bound to it.

### [`RFC6514-9.2.3.2-1`](#rfc6514-9.2.3.2-1)

When re-advertising an Inter-AS I-PMSI A-D route, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-1, so no unit is bound to it.

### [`RFC6514-9.2.3.2-2`](#rfc6514-9.2.3.2-2)

If the ASBR uses ingress replication for the intra-AS segment of the inter-AS tunnel, the re-advertised route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, but no MPLS labels. (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-2, so no unit is bound to it.

### [`RFC6514-9.2.3.2-3`](#rfc6514-9.2.3.2-3)

If the ASBR uses a P-multicast tree for the intra-AS segment of the inter-AS tunnel, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the ASBR could create the identity of the tree prior to the actual instantiation of the tree). (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-3, so no unit is bound to it.

### [`RFC6514-9.2.3.2-5`](#rfc6514-9.2.3.2-5)

An ASBR that uses a P-multicast tree as the intra-AS segment of the inter-AS tunnel MAY aggregate two or more MVPNs present on the ASBR onto the same tree. In this case, in addition to the identity of the tree, the PMSI Tunnel attribute of the Inter-AS I- PMSI A-D route MUST carry an MPLS upstream-assigned label that the PE has bound uniquely to the MVPN associated with this route (as determined by its RTs). (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-5, so no unit is bound to it.

### [`RFC6514-9.2.3.2-6`](#rfc6514-9.2.3.2-6)

If the ASBR has already advertised Inter-AS I-PMSI A-D routes for two or more MVPNs that it now desires to aggregate, then the ASBR MUST re-advertise those routes. (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-6, so no unit is bound to it.

### [`RFC6514-9.2.3.2-7`](#rfc6514-9.2.3.2-7)

The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the MVPN label. (§9.2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2-7, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-1`](#rfc6514-9.2.3.2.1-1)

In addition, the ASBR MUST send to the EBGP neighbor from whom it received the Inter-AS I-PMSI A-D route, a BGP Update message that carries a Leaf A-D route constructed as follows. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-1, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-2`](#rfc6514-9.2.3.2.1-2)

The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the ASBR (this MUST be a routable IP address). (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-2, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-3`](#rfc6514-9.2.3.2.1-3)

The Leaf A-D route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and the Tunnel Identifier set to a routable address of the advertising router. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-3, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-4`](#rfc6514-9.2.3.2.1-4)

The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used by the advertising router to demultiplex the MVPN traffic received over a unicast tunnel from the EBGP neighbor. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-4, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-5`](#rfc6514-9.2.3.2.1-5)

The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-5, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-6`](#rfc6514-9.2.3.2.1-6)

To constrain the distribution scope of this route, the route MUST carry the NO_ADVERTISE BGP Community [RFC1997]. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-6, so no unit is bound to it.

### [`RFC6514-9.2.3.2.1-7`](#rfc6514-9.2.3.2.1-7)

The ASBR MUST set up its forwarding state such that packets that arrive on the one-hop ASBR-ASBR LSP, as specified in the PMSI Tunnel attribute of the Leaf A-D route, are transmitted on the intra-AS segment, as specified in the PMSI Tunnel attribute of the Inter-AS I-PMSI A-D route that the ASBR re-advertises in its own AS. (§9.2.3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.2.1-7, so no unit is bound to it.

### [`RFC6514-9.2.3.3-2`](#rfc6514-9.2.3.3-2)

For each of these intra-AS tunnels, if a non-zero MPLS label is carried in the PMSI Tunnel attribute (i.e., aggregation is used), then only packets received on the inner LSP corresponding to that label MUST be forwarded, not the packets received on the outer LSP, as the outer LSP possibly carries the traffic of other VPNs. (§9.2.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.3-2, so no unit is bound to it.

### [`RFC6514-9.2.3.4-1`](#rfc6514-9.2.3.4-1)

If a given Inter-AS I-PMSI A-D route is received via IBGP by a BGP route reflector, the BGP route reflector MUST NOT modify the Next Hop field of the MP_REACH_NLRI attribute when re-advertising the route into IBGP (§9.2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4-1, so no unit is bound to it.

### [`RFC6514-9.2.3.4-2`](#rfc6514-9.2.3.4-2)

When propagating the route to the EBGP neighbors, the ASBR MUST set the Next Hop field of the MP_REACH_NLRI attribute to a routable IP address of the ASBR. (§9.2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4-2, so no unit is bound to it.

### [`RFC6514-9.2.3.4-3`](#rfc6514-9.2.3.4-3)

If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute, then the propagated route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication; the attribute carries no MPLS labels. (§9.2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4-3, so no unit is bound to it.

### [`RFC6514-9.2.3.4-5`](#rfc6514-9.2.3.4-5)

If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Identifier set to RSVP-TE P2MP LSP, then the ASBR that originated the route MUST establish an RSVP-TE P2MP LSP with the local PE/ASBR as a leaf. (§9.2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4-5, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-1`](#rfc6514-9.2.3.4.1-1)

If the Leaf Information Required flag in the PMSI Tunnel attribute of the received Inter-AS I-PMSI A-D route is set to 1, then the PE/ASBR MUST originate a new Leaf A-D route as follows. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-1, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-2`](#rfc6514-9.2.3.4.1-2)

The route carries a single MCAST-VPN NLRI with the Route Key field set to the MCAST-VPN NLRI of the Inter-AS I-PMSI A-D route received from that neighbor and the Originating Router's IP address set to the IP address of the PE/ASBR (this MUST be a routable IP address). (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-2, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-3`](#rfc6514-9.2.3.4.1-3)

If the received Inter-AS I-PMSI A-D route carries the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication, then the Leaf A-D route MUST carry the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-3, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-4`](#rfc6514-9.2.3.4.1-4)

The Tunnel Identifier MUST carry a routable address of the PE/ASBR. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-4, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-5`](#rfc6514-9.2.3.4.1-5)

The PMSI Tunnel attribute MUST carry a downstream-assigned MPLS label that is used to demultiplex the MVPN traffic received over a unicast tunnel by the PE/ASBR. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-5, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-6`](#rfc6514-9.2.3.4.1-6)

The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field of the route. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-6, so no unit is bound to it.

### [`RFC6514-9.2.3.4.1-7`](#rfc6514-9.2.3.4.1-7)

To constrain the distribution scope of this route, the route MUST carry the NO_EXPORT Community [RFC1997]. (§9.2.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-9.2.3.4.1-7, so no unit is bound to it.

### [`RFC6514-10-2`](#rfc6514-10-2)

If there is a separate UMH VRF, it MAY have its own import and export Route Targets, different from the ones used by the unicast VRF. These Route Targets MUST be used to control distribution of auto- discovery routes. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-10-2, so no unit is bound to it.

### [`RFC6514-10-4`](#rfc6514-10-4)

If an MVPN site is multihomed to several PEs, then to support non- congruent unicast and multicast connectivity, on each of these PEs, the UMH VRF of the MVPN MUST use its own distinct RD (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-10-4, so no unit is bound to it.

### [`RFC6514-10-6`](#rfc6514-10-6)

These routes MUST carry the VRF Route Import Extended Community. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-10-6, so no unit is bound to it.

### [`RFC6514-10-7`](#rfc6514-10-7)

If, for a given MVPN, BGP is used for exchanging C-multicast routes, or if segmented inter-AS tunnels are used, then these routes MUST also carry the Source AS Extended Community. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-10-7, so no unit is bound to it.

### [`RFC6514-11.1.1.1-1`](#rfc6514-11.1.1.1-1)

Whenever (a) a C-PIM instance on a particular PE creates a new (C-S,C-G) state, and (b) the selected upstream PE for C-S (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Source Tree Join. (§11.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.1.1-1, so no unit is bound to it.

### [`RFC6514-11.1.1.1-2`](#rfc6514-11.1.1.1-2)

Whenever a C-PIM instance on a particular PE deletes a (C-S,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.1.1-2, so no unit is bound to it.

### [`RFC6514-11.1.1.2-1`](#rfc6514-11.1.1.2-1)

Whenever (a) a C-PIM instance on a particular PE creates a new (C-*,C-G) state, and (b) the selected upstream PE for the C-RP corresponding to the C-G (see [MVPN]) is not the local PE, then the local PE MUST originate a C-multicast route of type Shared Tree Join. (§11.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.1.2-1, so no unit is bound to it.

### [`RFC6514-11.1.1.2-2`](#rfc6514-11.1.1.2-2)

Whenever a C-PIM instance on a particular PE deletes a (C-*,C-G) state, the corresponding C-multicast route MUST be withdrawn. (§11.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.1.2-2, so no unit is bound to it.

### [`RFC6514-11.1.3-1`](#rfc6514-11.1.3-1)

The Next Hop field of the MP_REACH_NLRI attribute MUST be set to a routable IP address of the local PE. (§11.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.3-1, so no unit is bound to it.

### [`RFC6514-11.1.4-1`](#rfc6514-11.1.4-1)

In that case, a change in the unicast routing may invalidate prior choices of the UMH route for some C-S. If this happens, the local PE MUST execute the UMH route selection procedures for C-S again. (§11.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.4-1, so no unit is bound to it.

### [`RFC6514-11.1.4-2`](#rfc6514-11.1.4-2)

If the result is that a different UMH route is selected, then for all C-G, any previously originated C-multicast routes for (C-S,C-G) MUST be re-originated. (§11.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.4-2, so no unit is bound to it.

### [`RFC6514-11.1.4-3`](#rfc6514-11.1.4-3)

Similarly, if a unicast routing change results in a change of the UMH route for a C-RP, then for all C-G such that C-RP is the RP associated with C-G, any previously originated C-multicast routes for (C-*,C-G) MUST be re-originated. (§11.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.1.4-3, so no unit is bound to it.

### [`RFC6514-11.2-1`](#rfc6514-11.2-1)

The ASBR first checks if it already has one or more C-multicast routes that have the same MCAST-VPN NLRI as the newly received route. If such a route(s) already exists, the ASBR keeps the newly received route, but SHALL NOT re-advertise the newly received route. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.2-1, so no unit is bound to it.

### [`RFC6514-11.2-2`](#rfc6514-11.2-2)

When an ASBR receives a BGP Update message that carries a withdrawal of a previously advertised C-multicast route, the ASBR first checks if it already has at least one other C-multicast route that has the same MCAST-VPN NLRI. If such a route already exists, the ASBR processes the withdrawn route, but SHALL NOT re-advertise the withdrawal. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.2-2, so no unit is bound to it.

### [`RFC6514-11.3.1.1-1`](#rfc6514-11.3.1.1-1)

When, for a said VRF, the last Source Tree Join C-multicast route for (C-S,C-G) is withdrawn, resulting in the situation where the VRF contains no Source Tree Join C-multicast route for (C-S,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state. (§11.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.3.1.1-1, so no unit is bound to it.

### [`RFC6514-11.3.1.1-3`](#rfc6514-11.3.1.1-3)

If C-G is not in the SSM range for the VRF, then removing the I-PMSI/S-PMSI from the outgoing interface list of the (C-S,C-G) state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§11.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.3.1.1-3, so no unit is bound to it.

### [`RFC6514-11.3.1.2-1`](#rfc6514-11.3.1.2-1)

When, for a said VRF, the last Shared Tree Join C-multicast route for (C-*,C-G) is withdrawn, resulting in the situation where the VRF contains no Shared Tree Join C-multicast route for (C-*,C-G), the PE MUST remove the I-PMSI/S-PMSI from the outgoing interface list of the (C-*,C-G) state. (§11.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-11.3.1.2-1, so no unit is bound to it.

### [`RFC6514-12.1-1`](#rfc6514-12.1-1)

The Multicast Source field MUST contain the source address associated with the C-multicast stream, and the Multicast Source Length field is set appropriately to reflect this. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-1, so no unit is bound to it.

### [`RFC6514-12.1-2`](#rfc6514-12.1-2)

The Multicast Group field MUST contain the group address associated with the C-multicast stream, and the Multicast Group Length field is set appropriately to reflect this. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-2, so no unit is bound to it.

### [`RFC6514-12.1-3`](#rfc6514-12.1-3)

The Originating Router's IP Address field MUST be set to the IP address that the (local) PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-3, so no unit is bound to it.

### [`RFC6514-12.1-4`](#rfc6514-12.1-4)

The PMSI Tunnel attribute MUST contain the identity of the P-multicast tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-4, so no unit is bound to it.

### [`RFC6514-12.1-5`](#rfc6514-12.1-5)

If a PE originates S-PMSI A-D routes with the Leaf Information Required flag in the PMSI Tunnel attribute set to 1, then the PE MUST be (auto-)configured with an import Route Target, which controls acceptance of Leaf A-D routes by the PE. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-5, so no unit is bound to it.

### [`RFC6514-12.1-6`](#rfc6514-12.1-6)

The Global Administrator field of this Route Target MUST be set to the IP address carried in the Next Hop of all the S-PMSI A-D routes advertised by this PE (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-6, so no unit is bound to it.

### [`RFC6514-12.1-7`](#rfc6514-12.1-7)

if the PE uses different Next Hops, then the PE MUST be (auto-)configured with multiple import RTs, one per each such Next Hop (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-7, so no unit is bound to it.

### [`RFC6514-12.1-8`](#rfc6514-12.1-8)

The Local Administrator field of this Route Target MUST be set to 0. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-8, so no unit is bound to it.

### [`RFC6514-12.1-12`](#rfc6514-12.1-12)

If the PE already advertises S-PMSI A-D routes for these S-PMSIs, then aggregation requires the PE to re-advertise these routes. The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-12, so no unit is bound to it.

### [`RFC6514-12.1-13`](#rfc6514-12.1-13)

The PMSI Tunnel attribute in the newly advertised/re-advertised routes MUST carry the identity of the P-multicast tree that aggregates the S-PMSIs. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-13, so no unit is bound to it.

### [`RFC6514-12.1-14`](#rfc6514-12.1-14)

If at least some of the S-PMSIs aggregated onto the same P-multicast tree belong to different MVPNs, then all these routes MUST carry an MPLS upstream-assigned label [RFC5331]. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-14, so no unit is bound to it.

### [`RFC6514-12.1-16`](#rfc6514-12.1-16)

If all these aggregated S-PMSIs belong to the same MVPN, and this MVPN uses PIM as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MAY carry an MPLS upstream-assigned label [RFC5331]. Moreover, in this case, the labels MUST be distinct on a per-MVPN basis and MAY be distinct on a per-route basis. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-16, so no unit is bound to it.

### [`RFC6514-12.1-18`](#rfc6514-12.1-18)

If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331] (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-18, so no unit is bound to it.

### [`RFC6514-12.1-19`](#rfc6514-12.1-19)

If all these aggregated S-PMSIs belong to the MVPN(s) that uses mLDP as its C-multicast routing protocol, then the corresponding S-PMSI A-D routes MUST carry an MPLS upstream-assigned label [RFC5331], and these labels MUST be distinct on a per-route (per-mLDP FEC) basis, irrespective of whether the aggregated S-PMSIs belong to the same or different MVPNs. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-19, so no unit is bound to it.

### [`RFC6514-12.1-20`](#rfc6514-12.1-20)

The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the same IP address as the one carried in the Originating Router's IP Address field. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-20, so no unit is bound to it.

### [`RFC6514-12.1-21`](#rfc6514-12.1-21)

In each of the above cases, an implementation MUST allow the set of Route Targets carried by the route to be specified by configuration. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-21, so no unit is bound to it.

### [`RFC6514-12.1-22`](#rfc6514-12.1-22)

In the absence of a configured set of Route Targets, the route MUST carry the default set of Route Targets, as specified above. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.1-22, so no unit is bound to it.

### [`RFC6514-12.2.1-3`](#rfc6514-12.2.1-3)

If an ASBR merges a (C-S,C-G) S-PMSI A-D route into an Inter-AS I-PMSI A-D route, the ASBR MUST discard all (C-S,C-G) traffic it receives on the tunnel advertised in the I-PMSI A-D route. (§12.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.2.1-3, so no unit is bound to it.

### [`RFC6514-12.2.1-4`](#rfc6514-12.2.1-4)

An ASBR that merges an S-PMSI A-D route into an Inter-AS I-PMSI A-D route MUST NOT re-advertise the S-PMSI A-D route. (§12.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.2.1-4, so no unit is bound to it.

### [`RFC6514-12.3-1`](#rfc6514-12.3-1)

In addition to the procedures specified in Section 9.2.3.4.1, the PE MUST set up its forwarding path to receive (C-S,C-G) traffic from the tunnel advertised by the S-PMSI A-D route (the PE MUST switch to the S-PMSI). (§12.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-12.3-1, so no unit is bound to it.

### [`RFC6514-13-1`](#rfc6514-13-1)

The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, they only apply for the multicast ASM mode and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13-1, so no unit is bound to it.

### [`RFC6514-13-2`](#rfc6514-13-2)

The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13-2, so no unit is bound to it.

### [`RFC6514-13.1-1`](#rfc6514-13.1-1)

When, as a result of receiving a Source Tree Join C-multicast route for (C-S,C-G) from some other PE the local PE adds either the S-PMSI or the I-PMSI to the outgoing interface list of the (C-S,C-G) state (see Section 11.3.1.1), the local PE MUST originate a Source Active A-D route if the PE has not originated such route already. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.1-1, so no unit is bound to it.

### [`RFC6514-13.1-2`](#rfc6514-13.1-2)

The Multicast Source field MUST be set to C-S. The Multicast Source Length field is set appropriately to reflect this. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.1-2, so no unit is bound to it.

### [`RFC6514-13.1-3`](#rfc6514-13.1-3)

The Multicast Group field MUST be set to C-G. The Multicast Group Length field is set appropriately to reflect this. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.1-3, so no unit is bound to it.

### [`RFC6514-13.1-4`](#rfc6514-13.1-4)

The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE from the MVPN's VRF. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.1-4, so no unit is bound to it.

### [`RFC6514-13.1-6`](#rfc6514-13.1-6)

When, as a result of receiving a withdrawal of the previously advertised Source Tree Join C-multicast route for (C-S,C-G), the PE is going to remove the S-PMSI/I-PMSI from the outgoing interface list of the (C-S,C-G) state. The local PE MUST also withdraw the Source Active A-D route for (C-S,C-G), if such a route has been advertised. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.1-6, so no unit is bound to it.

### [`RFC6514-13.2-1`](#rfc6514-13.2-1)

When (as a result of receiving PIM messages from one of its CEs) a PE creates in one of its MVPN-TIBs a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§13.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2-1, so no unit is bound to it.

### [`RFC6514-13.2-2`](#rfc6514-13.2-2)

When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§13.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2-2, so no unit is bound to it.

### [`RFC6514-13.2.1-1`](#rfc6514-13.2.1-1)

If the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, and the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G) (where C-S is address carried in the Multicast Source field and C-G is the address carried in the Multicast Group field of the received Source Active A-D route), then the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the Prune state. (§13.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2.1-1, so no unit is bound to it.

### [`RFC6514-13.2.1-3`](#rfc6514-13.2.1-3)

Transitioning the state machine to the Prune state SHOULD be done after a delay that is controlled by a timer. The value of the timer MUST be configurable. (§13.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2.1-3, so no unit is bound to it.

### [`RFC6514-13.2.1-4`](#rfc6514-13.2.1-4)

The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). (§13.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2.1-4, so no unit is bound to it.

### [`RFC6514-13.2.1-5`](#rfc6514-13.2.1-5)

The PE MUST keep the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI in the Prune state for as long as (a) the outgoing interface list (oif) for the found (C-*,C-G) entry in the MVPN-TIB on the PE contains either I-PMSI or S-PMSI, (b) the PE has at least one Source Active A-D route for (C-S,C-G), and (c) the PE does not originate the Source Tree Join C-multicast route for (C-S,C-G). Once any of these conditions become no longer valid, the PE MUST transition the (C-S,C-G,rpt) downstream state machine on I-PMSI/S-PMSI to the NoInfo state. (§13.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-13.2.1-5, so no unit is bound to it.

### [`RFC6514-14-1`](#rfc6514-14-1)

The procedures defined in this section only apply when the C-multicast routing protocol is PIM [RFC4601]; moreover, only apply for the multicast ASM mode, and MUST NOT be applied to multicast group addresses belonging to the SSM range. (§14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14-1, so no unit is bound to it.

### [`RFC6514-14-2`](#rfc6514-14-2)

The procedures also MUST NOT be applied when the C-multicast routing protocol is BIDIR-PIM [RFC5015]. (§14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14-2, so no unit is bound to it.

### [`RFC6514-14.1-1`](#rfc6514-14.1-1)

The Multicast Source field MUST be set to the source IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.1-1, so no unit is bound to it.

### [`RFC6514-14.1-2`](#rfc6514-14.1-2)

The Multicast Group field MUST be set to the group IP address of the multicast data packet carried in the PIM Register message (RP/PIM register case) or of the MSDP Source-Active message (MSDP case). (§14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.1-2, so no unit is bound to it.

### [`RFC6514-14.1-3`](#rfc6514-14.1-3)

The Next Hop field of the MP_REACH_NLRI attribute MUST be set to the IP address that the PE places in the Global Administrator field of the VRF Route Import Extended Community of the VPN-IP routes advertised by the PE. (§14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.1-3, so no unit is bound to it.

### [`RFC6514-14.2-1`](#rfc6514-14.2-1)

When (as a result of receiving PIM messages from one of its CEs) a PE creates, in one of its MVPN-TIBs, a (new) (C-*,C-G) entry with a non- empty outgoing interface list that contains one or more PE-CE interfaces, the PE MUST check if it has any matching Source Active A-D routes. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-1, so no unit is bound to it.

### [`RFC6514-14.2-2`](#rfc6514-14.2-2)

If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through some other PE, then for each such route the PE MUST originate a Source Tree Join C-multicast route. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-2, so no unit is bound to it.

### [`RFC6514-14.2-3`](#rfc6514-14.2-3)

If there is one or more such matching routes, and the best path to C-S carried in the matching route(s) is reachable through a CE connected to the PE, then for each such route the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-3, so no unit is bound to it.

### [`RFC6514-14.2-4`](#rfc6514-14.2-4)

When, as a result of receiving a new Source Active A-D route, a PE updates its VRF with the route, the PE MUST check if the newly received route matches any (C-*,C-G) entries. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-4, so no unit is bound to it.

### [`RFC6514-14.2-5`](#rfc6514-14.2-5)

If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through some other PE, the PE MUST originate a Source Tree Join C-multicast route for the (C-S,C-G) carried by the route. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-5, so no unit is bound to it.

### [`RFC6514-14.2-6`](#rfc6514-14.2-6)

If there is a matching entry, and the best path to C-S carried in the (A-D) route is reachable through a CE connected to the PE, the PE MUST originate a PIM Join (C-S,C-G) towards the CE. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-6, so no unit is bound to it.

### [`RFC6514-14.2-7`](#rfc6514-14.2-7)

A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if, as a result of having received PIM messages from one of its CEs, the PE creates a Prune (C-S,C-G,rpt) upstream state in one of its MVPN-TIBs but has no (C-S,C-G) Joined state in that MVPN-TIB and had previously advertised the said route. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-7, so no unit is bound to it.

### [`RFC6514-14.2-8`](#rfc6514-14.2-8)

A PE MUST withdraw a Source Tree Join C-multicast route for (C-S,C-G) if the Source Active A-D route that triggered the advertisement of the C-multicast route is withdrawn. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-8, so no unit is bound to it.

### [`RFC6514-14.2-9`](#rfc6514-14.2-9)

When a PE deletes the (C-*,C-G) state (e.g., due to receiving PIM Prune (C-*,C-G) from its CEs), the PE MUST withdraw all the Source Tree Join C-multicast routes for C-G that have been advertised by the PE, except for the routes for which the PE still maintains the corresponding (C-S,C-G) state. (§14.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-14.2-9, so no unit is bound to it.

### [`RFC6514-17-2`](#rfc6514-17-2)

A PE router MUST NOT accept, from CEs routes, with MCAST-VPN SAFI. (§17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-17-2, so no unit is bound to it.

### [`RFC6514-17-3`](#rfc6514-17-3)

If BGP is used as a CE-PE routing protocol, then when a PE receives a route from a CE, if this route carries the VRF Route Import Extended Community, the PE MUST remove this Community from the route before turning it into a VPN-IP route. (§17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-17-3, so no unit is bound to it.

### [`RFC6514-17-4`](#rfc6514-17-4)

Routes that a PE advertises to a CE MUST NOT carry the VRF Route Import Extended Community. (§17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6514-17-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6514, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6514, so its obligations are stated where they were written.
