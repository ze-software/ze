# RFC 5838 - Support of Address Families in OSPFv3

OSPFv3 multiple address families. Every requirement this repository extracted from RFC 5838, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 16 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 14 | 14 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 16 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 16 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10 | of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 16 gated MUSTs: they add up to 16. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Public row | OSPFv3 multiple address families |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 6 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc5838.md` |
| Requirement shard | `rfc/requirements/rfc5838.md` |
| RFC text | `rfc/full/rfc5838.txt` |

## Enrolment

Enrolled: Support of Address Families in OSPFv3 (RFC 5838): 4 MET (non-default-AF AF-bit discard + base-AF ignore, AF-conformance route-computation gate, IPv4 forwarding-address AF-width, global-IPv6 virtual-link endpoints) + 2 single-polarity positive (base-AF no-reject path, IPv4 forwarding-address remaining-bits-zero) + 10 gap (AF-bit not set in LSAs, IPv4 Link-LSA link-local, section 2.7 per-AF MTU / M6-bit machinery, the shared interface SA ze meets with no test, the IANA 128-255 Standards-Action process)

## What the public ledger says

**Area:** OSPFv3 multiple address families

**Proof:** 4 of 16 MUST proven

**Evidence:** unit 4

**Unmet:** RFC5838-2.2-1, RFC5838-2.5-1, RFC5838-2.7-1, RFC5838-2.7-11, RFC5838-2.7-2, RFC5838-2.7-3, RFC5838-2.7-4, RFC5838-2.7-8, RFC5838-4-1, RFC5838-5-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5838-2.3-1`](#rfc5838-2.3-1), [`RFC5838-2.4-1`](#rfc5838-2.4-1), [`RFC5838-2.6-1`](#rfc5838-2.6-1), [`RFC5838-2.6-2`](#rfc5838-2.6-2), [`RFC5838-2.8-1`](#rfc5838-2.8-1)

**Annotated (including scoped evidence) (11):** [`RFC5838-2.2-1`](#rfc5838-2.2-1), [`RFC5838-2.4-2`](#rfc5838-2.4-2), [`RFC5838-2.5-1`](#rfc5838-2.5-1), [`RFC5838-2.7-1`](#rfc5838-2.7-1), [`RFC5838-2.7-2`](#rfc5838-2.7-2), [`RFC5838-2.7-3`](#rfc5838-2.7-3), [`RFC5838-2.7-4`](#rfc5838-2.7-4), [`RFC5838-2.7-8`](#rfc5838-2.7-8), [`RFC5838-2.7-11`](#rfc5838-2.7-11), [`RFC5838-4-1`](#rfc5838-4-1), [`RFC5838-5-3`](#rfc5838-5-3)

## Figures

- **MUST bar**: Ze proven 4, Ze correctness gap 12
- **Overview bar**: Ze MUST proven 4, SHOULD not decided 11, Ze correctness gap 12

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze proven | 4 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 12 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 11 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc5838.json](../../../data/rfc/rfc5838.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5838-2.2-1` | When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the AF-bit is set in Hello and DD Options only; LSA origination never sets it -- encoder_v6.go:33 applies SetAF to the Hello/DD path only, and origination_v6.go:254 with origination_v6_link.go:51 build Router/Network/Link-LSA Options via neutralToV6Options, which omits OptAF |
| `RFC5838-2.3-1` | Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_afstrategy_v6_test.go#L283). **negative:** `unit/verify` [`TestRFC5838NonConformingExternalPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L117). **negative:** `unit/verify` [`TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L93). **negative:** `unit/verify` [`TestRFC5838NonConformingPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L23) |
| `RFC5838-2.4-1` | When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L156). **negative:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L155) |
| `RFC5838-2.4-2` | The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L181). **negative:** no negative test. **{single-polarity}:** the default IPv6-unicast AF has no reject path -- afBitAccepted at multiaf.go:181 returns true immediately for e.af.isDefault, so a base-AF Hello is never dropped for a missing AF-bit and there is no negative behavior to exercise |
| `RFC5838-2.5-1` | the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** v6OriginateLinkLSA at origination_v6_link.go:42-48 always encodes an IPv6 link-local address in the Link-LSA link-local field and returns false without one, so an IPv4-AF Link-LSA never carries the interface IPv4 address in the leading 32 bits |
| `RFC5838-2.6-1` | For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L63). **positive:** `unit/verify` [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L51). **negative:** `unit/verify` [`TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L84). **negative:** `unit/verify` [`TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L71) |
| `RFC5838-2.6-2` | the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L66). **positive:** `unit/verify` [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L54). **negative:** `unit/verify` [`TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L85) |
| `RFC5838-2.7-1` | For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks a single per-interface MTU -- neighbor/dd.go:170 sends cfg.InterfaceMTU and neighbor/dd.go:45 checks it -- with no separate address-family MTU versus IPv6 MTU, so the two are not considered independently |
| `RFC5838-2.7-2` | The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Database Description always carries the single cfg.InterfaceMTU at neighbor/dd.go:170 with no per-address-family MTU, so for a non-IPv6 AF whose MTU differs the DD does not carry the AF-specific MTU |
| `RFC5838-2.7-3` | For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no IPv4-address-family interface MTU; the DD MTU is the one configured interface MTU at neighbor/dd.go:170, never an IPv4-specific value |
| `RFC5838-2.7-4` | The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the adjacency MTU-compatibility gate at neighbor/dd.go:45 compares the single interface MTU only; ze does not derive an IPv6-MTU-based maximum packet size distinct from the AF MTU per RFC 5838 §2.7 |
| `RFC5838-2.7-8` | If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no M6-bit -- a grep for m6 across internal/plugins/ospf returns nothing -- so the DD encoder at encoder_v6.go:90 never sets it for a non-IPv6 AF |
| `RFC5838-2.7-11` | If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7) | MUST NOT | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither sets nor reads the M6-bit and performs only the single-MTU check at neighbor/dd.go:45, so the M6-conditioned suppression of an IPv6-MTU comparison is unimplemented |
| `RFC5838-2.8-1` | there MUST be a global IPv6 address associated with the virtual link so that OSPFv3 control packets are forwarded correctly by the intermediate hops between virtual link endpoints. (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L518). **negative:** `unit/verify` [`TestRFC5838VirtualLinkRequiresLocalGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L143). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L554) |
| `RFC5838-4-1` | When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies meets it, since the policy selects on upper-protocol 89 and the interface alone, never on the Instance ID, so every OSPFv3 instance on an interface uses the one SA, and validateConfigAF accepts an ipsec block only on the IPv6-unicast family; no test asserts it |
| `RFC5838-5-3` | Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an IANA assignment rule with no implementer role the RFC names; ze assigns no AF Instance ID and refuses one above 127 for AF use (internal/plugins/ospf/multiaf.go, ErrInstanceIDRange in config.go), and no test ties that refusal to this requirement |
| `RFC5838-2.5-2` | An implementation SHOULD resolve layer 3 to layer 2 mappings via the Address Resolution Protocol (ARP) [ARP] or Neighbor Discovery (ND) [ND] for a DIA even if the IPv4 address is not on the same subnet as the router's interface IP address. (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-5` | If the M6-bit is clear, the specified MTU SHOULD also be checked against the IPv6 MTU (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-6` | If the M6-bit is clear, the specified MTU SHOULD also be checked against the IPv6 MTU, and the Database Description packet SHOULD be rejected if the MTU is larger than the receiving interface's IPv6 MTU. (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-7` | An OSPFv3 router SHOULD NOT set the M6-bit if its IPv6 MTU and address family specific MTU are the same. (§2.7) | SHOULD NOT | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-9` | If this TLV is present, it carries the IPv6 MTU that SHOULD be compared with the local IPv6 MTU. (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-10` | If this TLV is absent, the minimum IPv6 MTU of 1280 octets SHOULD be used for the comparison (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-12` | The Interface MTU SHOULD be set to 0 in Database Description packets sent over virtual links. (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-14` | Instances subsequent to the first are not processed, and the LLS inconsistency SHOULD be logged. (§2.7) | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-5-2` | However, if it is being used for address families as described in this specification, the assignments herein SHOULD be honored. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-2.7-13` | Only one instance of the IPv6 MTU TLV MAY appear in the LLS block. (§2.7) | MAY | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5838-5-1` | Note that the Instance ID field MAY be used for applications other than the support of multiple address families. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5838-2.2-1`](#rfc5838-2.2-1) When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2) | {gap}, no test | the AF-bit is set in Hello and DD Options only; LSA origination never sets it -- encoder_v6.go:33 applies SetAF to the Hello/DD path only, and origination_v6.go:254 with origination_v6_link.go:51 build Router/Network/Link-LSA Options via neutralToV6Options, which omits OptAF |
| [`RFC5838-2.5-1`](#rfc5838-2.5-1) the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5) | {gap}, no test | v6OriginateLinkLSA at origination_v6_link.go:42-48 always encodes an IPv6 link-local address in the Link-LSA link-local field and returns false without one, so an IPv4-AF Link-LSA never carries the interface IPv4 address in the leading 32 bits |
| [`RFC5838-2.7-1`](#rfc5838-2.7-1) For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7) | {gap}, no test | ze tracks a single per-interface MTU -- neighbor/dd.go:170 sends cfg.InterfaceMTU and neighbor/dd.go:45 checks it -- with no separate address-family MTU versus IPv6 MTU, so the two are not considered independently |
| [`RFC5838-2.7-2`](#rfc5838-2.7-2) The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7) | {gap}, no test | the Database Description always carries the single cfg.InterfaceMTU at neighbor/dd.go:170 with no per-address-family MTU, so for a non-IPv6 AF whose MTU differs the DD does not carry the AF-specific MTU |
| [`RFC5838-2.7-3`](#rfc5838-2.7-3) For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7) | {gap}, no test | ze has no IPv4-address-family interface MTU; the DD MTU is the one configured interface MTU at neighbor/dd.go:170, never an IPv4-specific value |
| [`RFC5838-2.7-4`](#rfc5838-2.7-4) The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7) | {gap}, no test | the adjacency MTU-compatibility gate at neighbor/dd.go:45 compares the single interface MTU only; ze does not derive an IPv6-MTU-based maximum packet size distinct from the AF MTU per RFC 5838 §2.7 |
| [`RFC5838-2.7-8`](#rfc5838-2.7-8) If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7) | {gap}, no test | ze implements no M6-bit -- a grep for m6 across internal/plugins/ospf returns nothing -- so the DD encoder at encoder_v6.go:90 never sets it for a non-IPv6 AF |
| [`RFC5838-2.7-11`](#rfc5838-2.7-11) If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7) | {gap}, no test | ze neither sets nor reads the M6-bit and performs only the single-MTU check at neighbor/dd.go:45, so the M6-conditioned suppression of an IPv6-MTU comparison is unimplemented |
| [`RFC5838-4-1`](#rfc5838-4-1) When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4) | {gap}, no test | gap in proof: internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies meets it, since the policy selects on upper-protocol 89 and the interface alone, never on the Instance ID, so every OSPFv3 instance on an interface uses the one SA, and validateConfigAF accepts an ipsec block only on the IPv6-unicast family; no test asserts it |
| [`RFC5838-5-3`](#rfc5838-5-3) Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5) | {gap}, no test | an IANA assignment rule with no implementer role the RFC names; ze assigns no AF Instance ID and refuses one above 127 for AF use (internal/plugins/ospf/multiaf.go, ErrInstanceIDRange in config.go), and no test ties that refusal to this requirement |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5838-2.2-1`](#rfc5838-2.2-1)

When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.2-1, so no unit is bound to it.

### [`RFC5838-2.3-1`](#rfc5838-2.3-1)

Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). All three route-computation inputs on an IPv4-unicast AF now refuse a non-conforming (64-bit) prefix beside a conforming /24 that proves the reader ran: intra-area (TestRFC5838NonConformingPrefixNotInRouteComputation, v6BuildRoutes), inter-area (TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation, v6SummaryReader yields exactly the /24), and AS-External plus NSSA (TestRFC5838NonConformingExternalPrefixNotInRouteComputation, v6ExternalReader accepts the /24, refuses the 64-bit). Revert records on v6PrefixToNetip, v6SummaryReader and v6ExternalReader. Positive TestIPv4OverV3BuildRoutes is exact but carries no record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5838NonConformingExternalPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC5838NonConformingPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_afstrategy_v6_test.go#L283) | unit/verify | revert, verified |

### [`RFC5838-2.4-1`](#rfc5838-2.4-1)

When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a non-default-AF instance accepting a Hello with the AF-bit clear. (b) TestAFBitGatesFullNonDefault dispatches that Hello on an IPv4-unicast instance and asserts neighborSnapshot() is empty (a processed Hello would create an Init neighbor), then the same Hello with SetAF() yields one neighbor, isolating the AF-bit as the only difference.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L155) | unit/verify | unproven |
| positive | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L156) | unit/verify | unproven |

### [`RFC5838-2.4-2`](#rfc5838-2.4-2)

The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): the base IPv6-unicast AF discarding a Hello whose AF-bit is clear. (b) TestAFBitIgnoredDefaultAF dispatches an AF-bit-less Hello to the default IPv6-unicast engine and asserts neighborSnapshot() has one row, so an applied AF-bit check goes red. Single-polarity marker holds (the base AF has no reject path).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L181) | unit/verify | unproven |

### [`RFC5838-2.5-1`](#rfc5838-2.5-1)

the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.5-1, so no unit is bound to it.

### [`RFC5838-2.6-1`](#rfc5838-2.6-1)

For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). Both LSA kinds that can carry an FA on an IPv4 AF are now proven both ways. NSSA-LSA origination: TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4 / TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress (records on forwardingAddressForAF). AS-external-LSA: Ze's redistributed Type-5 (v6OriginateExternalLSA) carries no FA, and the translated Type-5 copies the Type-7 FA; TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4 reads 192.0.2.7 in the translated Type-5 for both IPv4 AFs, TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress feeds a Type-7 with FA 2001:db8:9::2 and finds no translated Type-5. Producer checked: nssa.go translatableForwardingAddressV6 (D-8 fix) gates the copy on the AF with the RFC 5838 s2.6 quote; revert records on it, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L51) | unit/verify | revert, verified |

### [`RFC5838-2.6-2`](#rfc5838-2.6-2)

the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). The {single-polarity} annotation is gone (the translation path could carry non-zero trailing bits, D-8). Positive: the originated NSSA-LSA and the translated AS-external-LSA each read all 16 FA bytes as 192.0.2.7 plus 96 zero bits, both IPv4 AFs. Negative: TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits feeds a Type-7 with 192.0.2.7 and bit 127 set and finds no translated Type-5, isolated (the same FA without the bit translates in the positive). Revert records on forwardingAddressForAF and translatableForwardingAddressV6.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L54) | unit/verify | revert, verified |

### [`RFC5838-2.7-1`](#rfc5838-2.7-1)

For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-1, so no unit is bound to it.

### [`RFC5838-2.7-2`](#rfc5838-2.7-2)

The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-2, so no unit is bound to it.

### [`RFC5838-2.7-3`](#rfc5838-2.7-3)

For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-3, so no unit is bound to it.

### [`RFC5838-2.7-4`](#rfc5838-2.7-4)

The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-4, so no unit is bound to it.

### [`RFC5838-2.7-8`](#rfc5838-2.7-8)

If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-8, so no unit is bound to it.

### [`RFC5838-2.7-11`](#rfc5838-2.7-11)

If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.7-11, so no unit is bound to it.

### [`RFC5838-2.8-1`](#rfc5838-2.8-1)

there MUST be a global IPv6 address associated with the virtual link so that OSPFv3 control packets are forwarded correctly by the intermediate hops between virtual link endpoints. (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. Positive TestV6VirtualEndpointResolvesGlobalAddress resolves global /128 LA addresses on both ends. Negatives cover both ends: the existing neighbor-side link-local-only case and new TestRFC5838VirtualLinkRequiresLocalGlobalAddress (this router advertises only fe80::1, neighbor global) which fails to resolve. v6ResolveVirtualEndpointLocked requires sok && dok and v6RouterGlobalAddr refuses non-global addresses; revert record on the resolver.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5838VirtualLinkRequiresLocalGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L143) | unit/verify | revert, verified |
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L554) | unit/verify | revert, verified |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L518) | unit/verify | revert, verified |

### [`RFC5838-4-1`](#rfc5838-4-1)

When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-4-1, so no unit is bound to it.

### [`RFC5838-5-3`](#rfc5838-5-3)

Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-5-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5838.txt |
| Source fingerprint | 1bfe0cda3cd218f9 |
| Record | rfc/extraction/rfc5838.json |
| Mapped sentences | 16 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 2 | walked | not stated |
| `2.5` | not stated | 1 | walked | not stated |
| `2.6` | not stated | 2 | walked | not stated |
| `2.7` | not stated | 6 | walked | not stated |
| `2.8` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5838 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5838, so its obligations are stated where they were written.
