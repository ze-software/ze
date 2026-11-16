# RFC 5838 - Support of Address Families in OSPFv3

OSPFv3 multiple address families. Every requirement listed for RFC 5838, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 16 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 14 | 14 of 20 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 9 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 27 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | OSPFv3 multiple address families |
| Automatic checks | Included |
| Requirements | 27 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tests linked to requirements | 20 |
| Recorded audit verdicts | 6 |
| Fault test records | 14 |
| Summary | `rfc/short/rfc5838.md` |
| Requirement table | `rfc/requirements/rfc5838.md` |
| RFC text | `rfc/full/rfc5838.txt` |

## Automatic checks

Included in automatic checks: Support of Address Families in OSPFv3 (RFC 5838): 4 MET (non-default-AF AF-bit discard + base-AF ignore, AF-conformance route-computation gate, IPv4 forwarding-address AF-width, global-IPv6 virtual-link endpoints) + 2 single-polarity positive (base-AF no-reject path, IPv4 forwarding-address remaining-bits-zero) + 10 gap (AF-bit not set in LSAs, IPv4 Link-LSA link-local, section 2.7 per-AF MTU / M6-bit machinery, the shared interface SA ze meets with no test, the IANA 128-255 Standards-Action process)

## Published support status

**Area:** OSPFv3 multiple address families

**Test verification:** 4 of 16 MUST proven

**Evidence:** unit 4

**Unmet:** RFC5838-2.2-1, RFC5838-2.7-1, RFC5838-2.7-11, RFC5838-2.7-2, RFC5838-2.7-3, RFC5838-2.7-4, RFC5838-2.7-8, RFC5838-4-1, RFC5838-5-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 10 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`RFC5838-2.3-1`](#rfc5838-2.3-1), [`RFC5838-2.4-1`](#rfc5838-2.4-1), [`RFC5838-2.5-1`](#rfc5838-2.5-1), [`RFC5838-2.6-1`](#rfc5838-2.6-1), [`RFC5838-2.6-2`](#rfc5838-2.6-2), [`RFC5838-2.8-1`](#rfc5838-2.8-1)

**Annotated (including scoped evidence) (10):** [`RFC5838-2.2-1`](#rfc5838-2.2-1), [`RFC5838-2.4-2`](#rfc5838-2.4-2), [`RFC5838-2.7-1`](#rfc5838-2.7-1), [`RFC5838-2.7-2`](#rfc5838-2.7-2), [`RFC5838-2.7-3`](#rfc5838-2.7-3), [`RFC5838-2.7-4`](#rfc5838-2.7-4), [`RFC5838-2.7-8`](#rfc5838-2.7-8), [`RFC5838-2.7-11`](#rfc5838-2.7-11), [`RFC5838-4-1`](#rfc5838-4-1), [`RFC5838-5-3`](#rfc5838-5-3)

## Figures

- **MUST bar**: Ze verified 4, Tests awaiting verification 3, Not built 9
- **Overview bar**: Ze MUST verified 4, Tests awaiting verification 3, Not assessed 11, Not built 9

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze verified | 4 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 9 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 11 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5838.json](../../../data/rfc/rfc5838.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5838-2.2-1` | When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the AF-bit is set in Hello and DD Options only; LSA origination never sets it -- encoder_v6.go:33 applies SetAF to the Hello/DD path only, and origination_v6.go:254 with origination_v6_link.go:51 build Router/Network/Link-LSA Options via neutralToV6Options, which omits OptAF |
| `RFC5838-2.3-1` | Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_afstrategy_v6_test.go#L283). **negative:** `unit/verify` [`TestRFC5838NonConformingExternalPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L117). **negative:** `unit/verify` [`TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L93). **negative:** `unit/verify` [`TestRFC5838NonConformingPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L23) |
| `RFC5838-2.4-1` | When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L156). **negative:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L155) |
| `RFC5838-2.4-2` | The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L181). **negative:** no negative test. **{single-polarity}:** the default IPv6-unicast AF has no reject path -- afBitAccepted at multiaf.go:181 returns true immediately for e.af.isDefault, so a base-AF Hello is never dropped for a missing AF-bit and there is no negative behavior to exercise |
| `RFC5838-2.5-1` | the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestRFC5838IPv4LinkLSACarriesTheLinkIPv4Address`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L34). **positive:** `unit/verify` [`TestRFC5838IPv4NextHopFromNeighborLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L117). **negative:** `unit/verify` [`TestRFC5838IPv4NextHopFromNeighborLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L97) |
| `RFC5838-2.6-1` | For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L63). **positive:** `unit/verify` [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L51). **negative:** `unit/verify` [`TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L84). **negative:** `unit/verify` [`TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L71) |
| `RFC5838-2.6-2` | the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L66). **positive:** `unit/verify` [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L54). **negative:** `unit/verify` [`TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L85) |
| `RFC5838-2.7-1` | For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks a single per-interface MTU -- neighbor/dd.go:170 sends cfg.InterfaceMTU and neighbor/dd.go:45 checks it -- with no separate address-family MTU versus IPv6 MTU, so the two are not considered independently |
| `RFC5838-2.7-2` | The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Database Description always carries the single cfg.InterfaceMTU at neighbor/dd.go:170 with no per-address-family MTU, so for a non-IPv6 AF whose MTU differs the DD does not carry the AF-specific MTU |
| `RFC5838-2.7-3` | For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no IPv4-address-family interface MTU; the DD MTU is the one configured interface MTU at neighbor/dd.go:170, never an IPv4-specific value |
| `RFC5838-2.7-4` | The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the adjacency MTU-compatibility gate at neighbor/dd.go:45 compares the single interface MTU only; ze does not derive an IPv6-MTU-based maximum packet size distinct from the AF MTU per RFC 5838 §2.7 |
| `RFC5838-2.7-8` | If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no M6-bit -- a grep for m6 across internal/plugins/ospf returns nothing -- so the DD encoder at encoder_v6.go:90 never sets it for a non-IPv6 AF |
| `RFC5838-2.7-11` | If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7) | MUST NOT | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither sets nor reads the M6-bit and performs only the single-MTU check at neighbor/dd.go:45, so the M6-conditioned suppression of an IPv6-MTU comparison is unimplemented |
| `RFC5838-2.8-1` | there MUST be a global IPv6 address associated with the virtual link so that OSPFv3 control packets are forwarded correctly by the intermediate hops between virtual link endpoints. (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L520). **negative:** `unit/verify` [`TestRFC5838VirtualLinkRequiresLocalGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L143). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L556) |
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
| [`RFC5838-2.7-1`](#rfc5838-2.7-1) For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7) | {gap}, no test | ze tracks a single per-interface MTU -- neighbor/dd.go:170 sends cfg.InterfaceMTU and neighbor/dd.go:45 checks it -- with no separate address-family MTU versus IPv6 MTU, so the two are not considered independently |
| [`RFC5838-2.7-2`](#rfc5838-2.7-2) The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7) | {gap}, no test | the Database Description always carries the single cfg.InterfaceMTU at neighbor/dd.go:170 with no per-address-family MTU, so for a non-IPv6 AF whose MTU differs the DD does not carry the AF-specific MTU |
| [`RFC5838-2.7-3`](#rfc5838-2.7-3) For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7) | {gap}, no test | ze has no IPv4-address-family interface MTU; the DD MTU is the one configured interface MTU at neighbor/dd.go:170, never an IPv4-specific value |
| [`RFC5838-2.7-4`](#rfc5838-2.7-4) The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7) | {gap}, no test | the adjacency MTU-compatibility gate at neighbor/dd.go:45 compares the single interface MTU only; ze does not derive an IPv6-MTU-based maximum packet size distinct from the AF MTU per RFC 5838 §2.7 |
| [`RFC5838-2.7-8`](#rfc5838-2.7-8) If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7) | {gap}, no test | ze implements no M6-bit -- a grep for m6 across internal/plugins/ospf returns nothing -- so the DD encoder at encoder_v6.go:90 never sets it for a non-IPv6 AF |
| [`RFC5838-2.7-11`](#rfc5838-2.7-11) If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7) | {gap}, no test | ze neither sets nor reads the M6-bit and performs only the single-MTU check at neighbor/dd.go:45, so the M6-conditioned suppression of an IPv6-MTU comparison is unimplemented |
| [`RFC5838-4-1`](#rfc5838-4-1) When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4) | {gap}, no test | gap in proof: internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies meets it, since the policy selects on upper-protocol 89 and the interface alone, never on the Instance ID, so every OSPFv3 instance on an interface uses the one SA, and validateConfigAF accepts an ipsec block only on the IPv6-unicast family; no test asserts it |
| [`RFC5838-5-3`](#rfc5838-5-3) Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5) | {gap}, no test | an IANA assignment rule with no implementer role the RFC names; ze assigns no AF Instance ID and refuses one above 127 for AF use (internal/plugins/ospf/multiaf.go, ErrInstanceIDRange in config.go), and no test ties that refusal to this requirement |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5838-2.2-1`](#rfc5838-2.2-1)

When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.2-1, so there is no linked test.

### [`RFC5838-2.3-1`](#rfc5838-2.3-1)

Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). All three route-computation inputs on an IPv4-unicast AF now refuse a non-conforming (64-bit) prefix beside a conforming /24 that proves the reader ran: intra-area (TestRFC5838NonConformingPrefixNotInRouteComputation, v6BuildRoutes), inter-area (TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation, v6SummaryReader yields exactly the /24), and AS-External plus NSSA (TestRFC5838NonConformingExternalPrefixNotInRouteComputation, v6ExternalReader accepts the /24, refuses the 64-bit). Revert records on v6PrefixToNetip, v6SummaryReader and v6ExternalReader. Positive TestIPv4OverV3BuildRoutes is exact but carries no record.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5838NonConformingExternalPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestRFC5838NonConformingInterAreaPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC5838NonConformingPrefixNotInRouteComputation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_afstrategy_v6_test.go#L283) | unit/verify | revert, verified |

### [`RFC5838-2.4-1`](#rfc5838-2.4-1)

When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a non-default-AF instance accepting a Hello with the AF-bit clear. (b) TestAFBitGatesFullNonDefault dispatches that Hello on an IPv4-unicast instance and asserts neighborSnapshot() is empty (a processed Hello would create an Init neighbor), then the same Hello with SetAF() yields one neighbor, isolating the AF-bit as the only difference.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L155) | unit/verify | not verified |
| positive | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L156) | unit/verify | not verified |

### [`RFC5838-2.4-2`](#rfc5838-2.4-2)

The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): the base IPv6-unicast AF discarding a Hello whose AF-bit is clear. (b) TestAFBitIgnoredDefaultAF dispatches an AF-bit-less Hello to the default IPv6-unicast engine and asserts neighborSnapshot() has one row, so an applied AF-bit check goes red. Single-polarity marker holds (the base AF has no reject path).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_multiaf_engine_test.go#L181) | unit/verify | not verified |

### [`RFC5838-2.5-1`](#rfc5838-2.5-1)

the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5838IPv4NextHopFromNeighborLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L97) | unit/verify | not verified |
| positive | [`TestRFC5838IPv4LinkLSACarriesTheLinkIPv4Address`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L34) | unit/verify | not verified |
| positive | [`TestRFC5838IPv4NextHopFromNeighborLinkLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_ipv4_nexthop_test.go#L117) | unit/verify | not verified |

### [`RFC5838-2.6-1`](#rfc5838-2.6-1)

For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). Both LSA kinds that can carry an FA on an IPv4 AF are now proven both ways. NSSA-LSA origination: TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4 / TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress (records on forwardingAddressForAF). AS-external-LSA: Ze's redistributed Type-5 (v6OriginateExternalLSA) carries no FA, and the translated Type-5 copies the Type-7 FA; TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4 reads 192.0.2.7 in the translated Type-5 for both IPv4 AFs, TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress feeds a Type-7 with FA 2001:db8:9::2 and finds no translated Type-5. Producer checked: nssa.go translatableForwardingAddressV6 (D-8 fix) gates the copy on the AF with the RFC 5838 s2.6 quote; revert records on it, observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5838IPv4AFNSSANeverEncodesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC5838IPv4AFTranslatorRefusesIPv6ForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L51) | unit/verify | revert, verified |

### [`RFC5838-2.6-2`](#rfc5838-2.6-2)

the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge, continuation 2). The {single-polarity} annotation is gone (the translation path could carry non-zero trailing bits, D-8). Positive: the originated NSSA-LSA and the translated AS-external-LSA each read all 16 FA bytes as 192.0.2.7 plus 96 zero bits, both IPv4 AFs. Negative: TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits feeds a Type-7 with 192.0.2.7 and bit 127 set and finds no translated Type-5, isolated (the same FA without the bit translates in the positive). Revert records on forwardingAddressForAF and translatableForwardingAddressV6.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5838IPv4AFTranslatorRefusesNonZeroTrailingBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFNSSAForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_forwarding_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5838IPv4AFTranslatedForwardingAddressIsIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_translate_test.go#L54) | unit/verify | revert, verified |

### [`RFC5838-2.7-1`](#rfc5838-2.7-1)

For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-1, so there is no linked test.

### [`RFC5838-2.7-2`](#rfc5838-2.7-2)

The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-2, so there is no linked test.

### [`RFC5838-2.7-3`](#rfc5838-2.7-3)

For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-3, so there is no linked test.

### [`RFC5838-2.7-4`](#rfc5838-2.7-4)

The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-4, so there is no linked test.

### [`RFC5838-2.7-8`](#rfc5838-2.7-8)

If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-8, so there is no linked test.

### [`RFC5838-2.7-11`](#rfc5838-2.7-11)

If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-2.7-11, so there is no linked test.

### [`RFC5838-2.8-1`](#rfc5838-2.8-1)

there MUST be a global IPv6 address associated with the virtual link so that OSPFv3 control packets are forwarded correctly by the intermediate hops between virtual link endpoints. (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. Positive TestV6VirtualEndpointResolvesGlobalAddress resolves global /128 LA addresses on both ends. Negatives cover both ends: the existing neighbor-side link-local-only case and new TestRFC5838VirtualLinkRequiresLocalGlobalAddress (this router advertises only fe80::1, neighbor global) which fails to resolve. v6ResolveVirtualEndpointLocked requires sok && dok and v6RouterGlobalAddr refuses non-global addresses; revert record on the resolver.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5838VirtualLinkRequiresLocalGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5838_af_test.go#L143) | unit/verify | revert, verified |
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L556) | unit/verify | revert, verified |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L520) | unit/verify | revert, verified |

### [`RFC5838-4-1`](#rfc5838-4-1)

When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-4-1, so there is no linked test.

### [`RFC5838-5-3`](#rfc5838-5-3)

Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5838-5-3, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 1 | reviewed | not stated |
| `2.3` | not stated | 1 | reviewed | not stated |
| `2.4` | not stated | 2 | reviewed | not stated |
| `2.5` | not stated | 1 | reviewed | not stated |
| `2.6` | not stated | 2 | reviewed | not stated |
| `2.7` | not stated | 6 | reviewed | not stated |
| `2.8` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 5838 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 5838, so its obligations are stated where they were written.
