# RFC 5838 - Support of Address Families in OSPFv3

Experimental. Every requirement this repository extracted from RFC 5838, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.0% | 4 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.5% | 2 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.5% | 2 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 50.0% | 8 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 6 | of 16 gated MUSTs judged | 4 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 6 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5838.md` |
| Requirement shard | `rfc/requirements/rfc5838.md` |
| RFC text | `rfc/full/rfc5838.txt` |

## Enrolment

Enrolled: Support of Address Families in OSPFv3 (RFC 5838): 4 MET (non-default-AF AF-bit discard + base-AF ignore, AF-conformance route-computation gate, IPv4 forwarding-address AF-width, global-IPv6 virtual-link endpoints) + 2 single-polarity positive (base-AF no-reject path, IPv4 forwarding-address remaining-bits-zero) + 8 gap (AF-bit not set in LSAs, IPv4 Link-LSA link-local, section 2.7 per-AF MTU / M6-bit machinery) + 2 not-applicable (IPsec restricted to default IPv6-unicast AF, IANA 128-255 Standards-Action process)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- AF-bit set in Hello/DD Options plus the AF-bit adjacency gate (non-default AF requires it, base IPv6-unicast AF ignores it), the RFC 5838 §2.1 Instance-ID-range to address-family mapping with one per-AF engine owning its LSDB/neighbors/SPF/install-family, IPv4-over-OSPFv3 prefix decode and route build, IPv4 forwarding-address encode/decode for AS-external and NSSA LSAs, and global-IPv6 virtual-link endpoints
- requirements gated in [`rfc/short/rfc5838.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5838.md) via `./le rfc check`.


**What the ledger says remains**

Eight MUST gaps annotated in [`rfc/short/rfc5838.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5838.md): the AF-bit is not set in originated LSAs ([`RFC5838-2.2-1`](#rfc5838-2.2-1)); an IPv4-AF Link-LSA carries no IPv4 link-local address ([`RFC5838-2.5-1`](#rfc5838-2.5-1)); and the RFC 5838 §2.7 per-address-family MTU handling is unimplemented -- no separate AF-versus-IPv6 MTU, no IPv4 interface MTU, and no M6-bit ([`RFC5838-2.7-1`](#rfc5838-2.7-1), 2.7-2, 2.7-3, 2.7-4, 2.7-8, 2.7-11). Feature remains under OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5838-2.3-1`](#rfc5838-2.3-1), [`RFC5838-2.4-1`](#rfc5838-2.4-1), [`RFC5838-2.6-1`](#rfc5838-2.6-1), [`RFC5838-2.8-1`](#rfc5838-2.8-1)

**Annotated instead of tested (12):** [`RFC5838-2.2-1`](#rfc5838-2.2-1), [`RFC5838-2.4-2`](#rfc5838-2.4-2), [`RFC5838-2.5-1`](#rfc5838-2.5-1), [`RFC5838-2.6-2`](#rfc5838-2.6-2), [`RFC5838-2.7-1`](#rfc5838-2.7-1), [`RFC5838-2.7-2`](#rfc5838-2.7-2), [`RFC5838-2.7-3`](#rfc5838-2.7-3), [`RFC5838-2.7-4`](#rfc5838-2.7-4), [`RFC5838-2.7-8`](#rfc5838-2.7-8), [`RFC5838-2.7-11`](#rfc5838-2.7-11), [`RFC5838-4-1`](#rfc5838-4-1), [`RFC5838-5-3`](#rfc5838-5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5838-2.2-1` | When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the AF-bit is set in Hello and DD Options only; LSA origination never sets it -- encoder_v6.go:33 applies SetAF to the Hello/DD path only, and origination_v6.go:254 with origination_v6_link.go:51 build Router/Network/Link-LSA Options via neutralToV6Options, which omits OptAF |
| `RFC5838-2.3-1` | Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L283). **negative:** `unit/verify` [`TestV6PrefixToNetipAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L256) |
| `RFC5838-2.4-1` | When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L156). **negative:** `unit/verify` [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L155) |
| `RFC5838-2.4-2` | The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L181). **negative:** no negative test. **{single-polarity}:** the default IPv6-unicast AF has no reject path -- afBitAccepted at multiaf.go:181 returns true immediately for e.af.isDefault, so a base-AF Hello is never dropped for a missing AF-bit and there is no negative behavior to exercise |
| `RFC5838-2.5-1` | the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** v6OriginateLinkLSA at origination_v6_link.go:42-48 always encodes an IPv6 link-local address in the Link-LSA link-local field and returns false without one, so an IPv4-AF Link-LSA never carries the interface IPv4 address in the leading 32 bits |
| `RFC5838-2.6-1` | For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L336). **negative:** `unit/verify` [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L337) |
| `RFC5838-2.6-2` | the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L338). **negative:** no negative test. **{single-polarity}:** forwardingAddressForAF at origination_v6_nssa.go:29-31 zero-initialises the 16-byte field and writes only the leading 4 IPv4 octets, so the remaining bits are structurally zero and no non-zero-trailing path exists to reject |
| `RFC5838-2.7-1` | For address families other than IPv6, both the MTU for the instance address family and the IPv6 MTU used for OSPFv3 maximum packet determination MUST be considered. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks a single per-interface MTU -- neighbor/dd.go:170 sends cfg.InterfaceMTU and neighbor/dd.go:45 checks it -- with no separate address-family MTU versus IPv6 MTU, so the two are not considered independently |
| `RFC5838-2.7-2` | The MTU in the Database Description packet MUST always contain the MTU corresponding to the advertised address family. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Database Description always carries the single cfg.InterfaceMTU at neighbor/dd.go:170 with no per-address-family MTU, so for a non-IPv6 AF whose MTU differs the DD does not carry the AF-specific MTU |
| `RFC5838-2.7-3` | For example, if the instance corresponds to an IPv4 address family, the IPv4 MTU for the interface MUST be specified in the interface MTU field. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no IPv4-address-family interface MTU; the DD MTU is the one configured interface MTU at neighbor/dd.go:170, never an IPv4-specific value |
| `RFC5838-2.7-4` | The value used for OSPFv3 maximum packet size determination MUST also be compatible for an adjacency to be established. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the adjacency MTU-compatibility gate at neighbor/dd.go:45 compares the single interface MTU only; ze does not derive an IPv6-MTU-based maximum packet size distinct from the AF MTU per RFC 5838 §2.7 |
| `RFC5838-2.7-8` | If the IPv6 and IPv4 MTUs differ, the M6-bit MUST be set for non-IPv6 address families. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no M6-bit -- a grep for m6 across internal/plugins/ospf returns nothing -- so the DD encoder at encoder_v6.go:90 never sets it for a non-IPv6 AF |
| `RFC5838-2.7-11` | If the M6-bit is set in a received Database Description packet for a non-IPv6 address family, the receiving router MUST NOT check the Interface MTU in the Database Description packet against the receiving interface's IPv6 MTU. (§2.7) | MUST NOT | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither sets nor reads the M6-bit and performs only the single-MTU check at neighbor/dd.go:45, so the M6-conditioned suppression of an IPv6-MTU comparison is unimplemented |
| `RFC5838-2.8-1` | there MUST be a global IPv6 address associated with the virtual link so that OSPFv3 control packets are forwarded correctly by the intermediate hops between virtual link endpoints. (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L518). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L554) |
| `RFC5838-4-1` | When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs OSPFv3 IPsec on the default IPv6-unicast AF only -- validateConfigAF at config.go:907-909 rejects an ipsec block on any non-IPv6 family -- so multiple AF instances never share an interface SA and the requirement's precondition never arises |
| `RFC5838-5-3` | Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the IANA registration process, not an implementation; ze has no code path that assigns AF Instance IDs and it rejects Instance IDs above 127 for AF use at multiaf.go:71 and via ErrInstanceIDRange in config.go |
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
| [`RFC5838-4-1`](#rfc5838-4-1) When multiple OSPFv3 instances use the same interface, they all MUST use the same Security Association (SA), since the SA selectors do not provide selection based on data in OSPFv3 Header fields (e.g., the Instance ID). (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs OSPFv3 IPsec on the default IPv6-unicast AF only -- validateConfigAF at config.go:907-909 rejects an ipsec block on any non-IPv6 family -- so multiple AF instances never share an interface SA and the requirement's precondition never arises |
| [`RFC5838-5-3`](#rfc5838-5-3) Instance IDs in the range 128-255 are not assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC including an IANA Considerations section explicitly specifying the AF Instance IDs being assigned. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the IANA registration process, not an implementation; ze has no code path that assigns AF Instance IDs and it rejects Instance IDs above 127 for AF use at multiaf.go:71 and via ErrInstanceIDRange in config.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5838-2.2-1`](#rfc5838-2.2-1)

When an OSPFv3 router is supporting AFs as described in this specification, it MUST set the AF-bit in the OSPFv3 Options field of Hello packets, Database Description packets, and LSAs. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.2-1, so no unit is bound to it.

### [`RFC5838-2.3-1`](#rfc5838-2.3-1)

Prefixes that don't conform to the AF of an OSPFv3 instance MUST NOT be used in the route computation for that instance. (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: a prefix not conforming to the instance AF (e.g. length > 32 under IPv4) used in the route computation. The negative in TestV6PrefixToNetipAFWidth asserts only the converter v6PrefixToNetip returns ok=false for a 64-bit prefix under an IPv4 AF; no tagged unit feeds a non-conforming prefix through v6BuildRoutes and asserts it yields no route, so a route builder that ignored the ok flag keeps both units green. The positive (TestIPv4OverV3BuildRoutes) does go through v6BuildRoutes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestV6PrefixToNetipAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L256) | unit/verify | unproven |
| positive | [`TestIPv4OverV3BuildRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L283) | unit/verify | unproven |

### [`RFC5838-2.4-1`](#rfc5838-2.4-1)

When an OSPFv3 router participates in an AF (sets the AF-bit in the Options field), it MUST discard Hello packets having the AF- bit clear in the Options field. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a non-default-AF instance accepting a Hello with the AF-bit clear. (b) TestAFBitGatesFullNonDefault dispatches that Hello on an IPv4-unicast instance and asserts neighborSnapshot() is empty (a processed Hello would create an Init neighbor), then the same Hello with SetAF() yields one neighbor, isolating the AF-bit as the only difference.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L155) | unit/verify | unproven |
| positive | [`TestAFBitGatesFullNonDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L156) | unit/verify | unproven |

### [`RFC5838-2.4-2`](#rfc5838-2.4-2)

The only exception is the Base IPv6 unicast AF, where this check MUST NOT be done (for backward compatibility). (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): the base IPv6-unicast AF discarding a Hello whose AF-bit is clear. (b) TestAFBitIgnoredDefaultAF dispatches an AF-bit-less Hello to the default IPv6-unicast engine and asserts neighborSnapshot() has one row, so an applied AF-bit check goes red. Single-polarity marker holds (the base AF has no reject path).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAFBitIgnoredDefaultAF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/multiaf_engine_test.go#L181) | unit/verify | unproven |

### [`RFC5838-2.5-1`](#rfc5838-2.5-1)

the link's IPv4 address will be advertised in the "link local address" field of the IPv4 instance's Link-LSA. This address is placed in the first 32 bits of the "link local address" field and is used for IPv4 next-hop calculations. The remaining bits MUST be set to zero. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5838-2.5-1, so no unit is bound to it.

### [`RFC5838-2.6-1`](#rfc5838-2.6-1)

For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. (§2.6)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The MUST binds the originator: the Forwarding Address in AS-external and NSSA LSAs of an IPv4 AF must encode an IPv4 address. TestV6ForwardingAddrAFWidth tests v6ForwardingAddr, the RECEIVE-side renderer, and never encodes an LSA; the producer forwardingAddressForAF (origination_v6_nssa.go) is untested by these tags. The IPv4 multicast AF is not covered either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L337) | unit/verify | unproven |
| positive | [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L336) | unit/verify | unproven |

### [`RFC5838-2.6-2`](#rfc5838-2.6-2)

the IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be set to zero. (§2.6)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The MUST is that the originator sets the bits after the leading 32 to zero. The tagged unit asserts the receiver renders 192.0.2.7 even when trailing octets are 0xff, i.e. it proves receive-side tolerance of non-zero bits, a neighbouring property; the producer forwardingAddressForAF (origination_v6_nssa.go) is never exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestV6ForwardingAddrAFWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6_test.go#L338) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: a virtual link formed without a global IPv6 address. The negative TestV6VirtualEndpointRequiresGlobalAddress covers only the neighbor side (neighbor advertises only fe80::2, resolution fails). No tagged unit removes THIS router's global address and asserts resolution fails, so v6ResolveVirtualEndpointLocked falling back to a link-local local source keeps both units green (the positive asserts src only when a global exists).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L554) | unit/verify | unproven |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L518) | unit/verify | unproven |

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
