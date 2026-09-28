# RFC 7684 - OSPFv2 Prefix/Link Attribute Advertisement

Experimental. Every requirement this repository extracted from RFC 7684, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 6 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 33.3% | 3 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 5 | of 9 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 5 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7684.md` |
| Requirement shard | `rfc/requirements/rfc7684.md` |
| RFC text | `rfc/full/rfc7684.txt` |

## Enrolment

Enrolled: OSPFv2 Extended Prefix and Extended Link Opaque LSAs. Ze implements the container codecs, prefix flags, flooding-scope selection and receive-side malformed-LSA rejection. The checklist names the requirement producers; tagged tests cover their stated outcomes, including router ingress for RFC7684-5-3. Codec-only malformed-body tests do not establish the storage, acknowledgement or flooding prohibition.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Extended Prefix and Extended Link LSA bodies and malformed TLV handling.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC7684-5-3`](#rfc7684-5-3), [`RFC7684-2.1-1`](#rfc7684-2.1-1), [`RFC7684-2.1-2`](#rfc7684-2.1-2), [`RFC7684-2.1-3`](#rfc7684-2.1-3), [`RFC7684-3.1-1`](#rfc7684-3.1-1), [`RFC7684-5-1`](#rfc7684-5-1)

**Annotated instead of tested (3):** [`RFC7684-4-1`](#rfc7684-4-1), [`RFC7684-6.5-1`](#rfc7684-6.5-1), [`RFC7684-6.5-2`](#rfc7684-6.5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7684-5-3` | Malformed LSAs MUST NOT be stored in the Link State Database (LSDB), acknowledged, or reflooded. (§5). `opaque.go:wireOpaqueDelivery` installs `ext.go:validateExtLSA` through `lsdb.SetReceiveValidator`; `lsdb.ReceiveUpdate` invokes it before every scope, self-originated and MaxAge branch. `packet.ValidateExtLSABody` checks top-level and nested framing without allocating attributes, including duplicate containers. Rejection increments `ze_ospf_ext_malformed_total`. `TestExtIngressMalformedDiscard` and `TestExtIngressValidCarriage` exercise the router dispatcher, LSDB and emitted acknowledgement/flood packets. Unknown opaque applications retain carrier behaviour. | MUST NOT | 5 | **positive:** `unit/verify` [`TestExtIngressValidCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_receive_test.go#L198). **negative:** `unit/verify` [`TestExtIngressMalformedDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_receive_test.go#L135) |
| `RFC7684-2.1-1` | If the flag is set and the prefix length is not a host prefix, then the flag MUST be ignored. (§2.1) -- `extNormalizeFlags` clears it on receive | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L77). **negative:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L66) |
| `RFC7684-2.1-2` | The flag is preserved when the OSPFv2 Extended Prefix Opaque LSA is propagated between areas. (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L121). **negative:** `unit/verify` [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L150) |
| `RFC7684-2.1-3` | However, since the Opaque LSA type defines the flooding scope, the LSA flooding scope MUST satisfy the application-specific requirements for all the prefixes included in a single OSPFv2 Extended Prefix Opaque LSA. (§2.1) -- `extPrefixScope`: area for intra/inter, AS for external | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L167). **negative:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L170) |
| `RFC7684-3.1-1` | Only one OSPFv2 Extended Link TLV SHALL be advertised in each OSPFv2 Extended Link Opaque LSA (§3.1) -- origination emits one per LSA; decode uses the first and logs extras | SHALL | 3.1 | **positive:** `unit/verify` [`TestExtLinkMirrorsRouterLSALink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_link_origin_test.go#L46). **negative:** `unit/verify` [`TestExtLinkSingleTLVEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L74) |
| `RFC7684-4-1` | However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this directive binds downstream application specifications (e.g. RFC 8665 segment routing) that define the sub-TLVs these LSAs carry, not a ze wire behavior; ze originates backward-compatible empty Extended Prefix/Link containers per RFC 5250 (internal/plugins/ospf/ext_prefix.go:72-75, internal/plugins/ospf/ext_link.go:56-59) |
| `RFC7684-5-1` | Additionally, implementations must assure that malformed TLV and sub- TLV permutations are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target | MUST | 5 | **positive:** `unit/verify` [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L25). **positive:** `unit/verify` [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_prefix_test.go#L28). **negative:** `unit/verify` [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L111). **negative:** `unit/verify` [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L91). **negative:** `unit/verify` [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L163) |
| `RFC7684-6.5-1` | Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | MUST NOT | 6.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA registry documentation policy binding RFC authors, not a ze code or wire behavior; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range |
| `RFC7684-6.5-2` | Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA allocation policy binding IETF specifications, not a ze code or wire behavior; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range |
| `RFC7684-2-1` | If multiple OSPFv2 Extended Prefix Opaque LSAs include the same prefix, the attributes from the Opaque LSA with the lowest Opaque ID SHOULD be used. (§2) -- `extReceiver.applyPrefix` | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-4` | A-Flag (Attach Flag): An Area Border Router (ABR) generating an OSPFv2 Extended Prefix TLV for an inter-area prefix that is locally connected or attached in another connected area SHOULD set this flag. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-5` | If this TLV is advertised multiple times for the same prefix in the same OSPFv2 Extended Prefix Opaque LSA, only the first instance of the TLV is used by receiving OSPFv2 routers. This situation SHOULD be logged as an error. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-2` | If this TLV is advertised multiple times in the same OSPFv2 Extended Link Opaque LSA, only the first instance of the TLV is used by receiving OSPFv2 routers. This situation SHOULD be logged as an error. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-5-2` | Reception of malformed LSAs SHOULD be counted and/or logged for further analysis (§5) -- `ze_ospf_ext_malformed_total` | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-6` | It is RECOMMENDED that OSPFv2 routers advertising OSPFv2 Extended Prefix TLVs in different OSPFv2 Extended Prefix Opaque LSAs re-originate these LSAs in ascending order of Opaque ID to minimize the disruption. (§2.1) -- stable ascending Opaque-ID allocator | RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-3` | It is RECOMMENDED that OSPFv2 routers advertising OSPFv2 Extended Link TLVs in different OSPFv2 Extended Link Opaque LSAs re-originate these LSAs in ascending order of Opaque ID to minimize the disruption. (§3.1) | RECOMMENDED | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-7` | Multiple OSPFv2 Extended Prefix TLVs MAY be advertised in each OSPFv2 Extended Prefix Opaque LSA (§2.1) -- the decoder accepts many; origination uses one LSA per prefix | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-8` | N-Flag (Node Flag): Set when the prefix identifies the advertising router, i.e., the prefix is a host prefix advertising a globally reachable address typically associated with a loopback address. The advertising router MAY choose to not set this flag even when the above conditions are met. (§2.1) -- Ze sets N on host /32 stub prefixes | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-9` | If this TLV is advertised multiple times for the same prefix in different OSPFv2 Extended Prefix Opaque LSAs originated by the same OSPFv2 router, the OSPFv2 advertising router is re-originating OSPFv2 Extended Prefix Opaque LSAs for multiple prefixes and is most likely repacking Extended-Prefix-TLVs in OSPFv2 Extended Prefix Opaque LSAs. In this case, the Extended-Prefix-TLV in the OSPFv2 Extended Prefix Opaque LSA with the smallest Opaque ID is used by receiving OSPFv2 routers. This situation may be logged as a warning. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-4` | If this TLV is advertised multiple times for the same link in different OSPFv2 Extended Link Opaque LSAs originated by the same OSPFv2 router, the OSPFv2 Extended Link TLV in the OSPFv2 Extended Link Opaque LSA with the smallest Opaque ID is used by receiving OSPFv2 routers. This situation may be logged as a warning. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7684-4-1`](#rfc7684-4-1) However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | no test | no test carries this requirement id; annotated {not-applicable}: this directive binds downstream application specifications (e.g. RFC 8665 segment routing) that define the sub-TLVs these LSAs carry, not a ze wire behavior; ze originates backward-compatible empty Extended Prefix/Link containers per RFC 5250 (internal/plugins/ospf/ext_prefix.go:72-75, internal/plugins/ospf/ext_link.go:56-59) |
| [`RFC7684-6.5-1`](#rfc7684-6.5-1) Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA registry documentation policy binding RFC authors, not a ze code or wire behavior; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range |
| [`RFC7684-6.5-2`](#rfc7684-6.5-2) Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA allocation policy binding IETF specifications, not a ze code or wire behavior; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7684-5-3`](#rfc7684-5-3)

Malformed LSAs MUST NOT be stored in the Link State Database (LSDB), acknowledged, or reflooded. (§5). `opaque.go:wireOpaqueDelivery` installs `ext.go:validateExtLSA` through `lsdb.SetReceiveValidator`; `lsdb.ReceiveUpdate` invokes it before every scope, self-originated and MaxAge branch. `packet.ValidateExtLSABody` checks top-level and nested framing without allocating attributes, including duplicate containers. Rejection increments `ze_ospf_ext_malformed_total`. `TestExtIngressMalformedDiscard` and `TestExtIngressValidCarriage` exercise the router dispatcher, LSDB and emitted acknowledgement/flood packets. Unknown opaque applications retain carrier behaviour.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: a malformed Extended Prefix or Extended Link LSA stored, acknowledged, or reflooded. TestExtIngressMalformedDiscard drives both opaque types in link, area and AS scope through the router ingress with a TLV overrun, trailing data shorter than a TLV header, sub-TLV overrun, short sub-TLV header, duplicate malformed container, MaxAge and self-originated input, and fails on any LSDB lookup hit (stored) or any send after RetransmitTick (acknowledged or reflooded), so each of the three clauses goes red. TestExtIngressValidCarriage is the positive: valid bodies are stored byte-for-byte, acknowledged and flooded to the expected interfaces.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtIngressMalformedDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_receive_test.go#L135) | unit/verify | unproven |
| positive | [`TestExtIngressValidCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_receive_test.go#L198) | unit/verify | unproven |

### [`RFC7684-2.1-1`](#rfc7684-2.1-1)

If the flag is set and the prefix length is not a host prefix, then the flag MUST be ignored. (§2.1) -- `extNormalizeFlags` clears it on receive

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: honouring an N-Flag received on a prefix that is not a host prefix. TestExtPrefixNFlagIgnoredNonHost drives extPrefixOnReceive with N set on a /24 and fails if the stored entry's flags still carry ExtPrefixFlagN, which is what every consumer reads; the positive half fails if N is dropped from a /32 host prefix, so the ignore is confined to the non-host case. Both polarities present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L66) | unit/verify | unproven |
| positive | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L77) | unit/verify | unproven |

### [`RFC7684-2.1-2`](#rfc7684-2.1-2)

The flag is preserved when the OSPFv2 Extended Prefix Opaque LSA is propagated between areas. (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: an ABR dropping the N-Flag when the Extended Prefix information is propagated into another area. TestExtPrefixNFlagPreservedInterArea proves it only for the ABR's own locally connected host /32 (N set by Ze's own origination): no unit propagates an N-Flag that another router advertised in its intra-area Extended Prefix LSA, which is the propagation the sentence is about. The negative (TestExtPrefixNFlagNotSetNonHostInterArea) asserts a /24 gains no N and carries A, a neighbouring rule (RFC7684-2.1-1 and the A-Flag row), not a violation of preservation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L150) | unit/verify | unproven |
| positive | [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L121) | unit/verify | unproven |

### [`RFC7684-2.1-3`](#rfc7684-2.1-3)

However, since the Opaque LSA type defines the flooding scope, the LSA flooding scope MUST satisfy the application-specific requirements for all the prefixes included in a single OSPFv2 Extended Prefix Opaque LSA. (§2.1) -- `extPrefixScope`: area for intra/inter, AS for external

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: an Extended Prefix Opaque LSA whose flooding scope is narrower than a prefix it carries requires, e.g. an AS-external prefix in an LS Type 10 LSA. TestExtPrefixScopeSelection asserts the per-route-type mapping of extPrefixScope (intra and inter to area, external and NSSA-external to AS), which goes red on a wrong mapping. The clause 'for all the prefixes included in a single' LSA has no assertion: no unit shows an originated LSA never mixes prefixes needing different scopes; it holds today only because origination emits one prefix per LSA, and nothing pins that.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L170) | unit/verify | unproven |
| positive | [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L167) | unit/verify | unproven |

### [`RFC7684-3.1-1`](#rfc7684-3.1-1)

Only one OSPFv2 Extended Link TLV SHALL be advertised in each OSPFv2 Extended Link Opaque LSA (§3.1) -- origination emits one per LSA; decode uses the first and logs extras

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtLinkSingleTLVEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L74) | unit/verify | unproven |
| positive | [`TestExtLinkMirrorsRouterLSALink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_link_origin_test.go#L46) | unit/verify | unproven |

### [`RFC7684-4-1`](#rfc7684-4-1)

However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-4-1, so no unit is bound to it.

### [`RFC7684-5-1`](#rfc7684-5-1)

Additionally, implementations must assure that malformed TLV and sub- TLV permutations are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: a malformed TLV or sub-TLV permutation that goes undetected or crashes the process. TestExtPrefixMalformedCounted asserts a top-level TLV overrun of an Extended Prefix body is counted and stores nothing, and the two decode tests pin that valid bodies decode. Detection of a sub-TLV overrun, and of any malformed Extended Link body, has no 5-1 tagged assertion: FuzzOSPFExtPrefixBody and FuzzOSPFExtLinkBody discard err and assert only no panic, over three seeds each under go test, so a decoder that silently accepted an overrunning sub-TLV stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L163) | unit/verify | unproven |
| negative | [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L111) | unit/verify | unproven |
| negative | [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L91) | unit/verify | unproven |
| positive | [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L25) | unit/verify | unproven |
| positive | [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_prefix_test.go#L28) | unit/verify | unproven |

### [`RFC7684-6.5-1`](#rfc7684-6.5-1)

Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-6.5-1, so no unit is bound to it.

### [`RFC7684-6.5-2`](#rfc7684-6.5-2)

Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-6.5-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7684.txt |
| Source fingerprint | 4bc9e41965e4e0ac |
| Record | rfc/extraction/rfc7684.json |
| Mapped sentences | 13 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 2 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 2 | walked | not stated |
| `6.5` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 7684 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7684, so its obligations are stated where they were written.
