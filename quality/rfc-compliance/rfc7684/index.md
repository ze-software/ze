# RFC 7684 - OSPFv2 Prefix/Link Attribute Advertisement

OSPF Extended Prefix and Link LSAs. Every requirement listed for RFC 7684, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 17 | 17 of 21 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 3 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 20 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | OSPF Extended Prefix and Link LSAs |
| Automatic checks | Included |
| Requirements | 20 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 21 |
| Tests linked to requirements | 21 |
| Recorded audit verdicts | 5 |
| Fault test records | 17 |
| Summary | `rfc/short/rfc7684.md` |
| Requirement table | `rfc/requirements/rfc7684.md` |
| RFC text | `rfc/full/rfc7684.txt` |

## Automatic checks

Included in automatic checks: OSPFv2 Extended Prefix and Extended Link Opaque LSAs. Ze implements the container codecs, prefix flags, flooding-scope selection and receive-side malformed-LSA rejection. The checklist names the requirement producers; tagged tests cover their stated outcomes, including router ingress for RFC7684-5-3. Codec-only malformed-body tests do not establish the storage, acknowledgement or flooding prohibition.

## Published support status

**Area:** OSPF Extended Prefix and Link LSAs

**Test verification:** 4 of 9 MUST proven

**Evidence:** unit 4

**Unmet:** RFC7684-4-1, RFC7684-6.5-1, RFC7684-6.5-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 3 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`RFC7684-5-3`](#rfc7684-5-3), [`RFC7684-2.1-1`](#rfc7684-2.1-1), [`RFC7684-2.1-2`](#rfc7684-2.1-2), [`RFC7684-2.1-3`](#rfc7684-2.1-3), [`RFC7684-3.1-1`](#rfc7684-3.1-1), [`RFC7684-5-1`](#rfc7684-5-1)

**Annotated (including scoped evidence) (3):** [`RFC7684-4-1`](#rfc7684-4-1), [`RFC7684-6.5-1`](#rfc7684-6.5-1), [`RFC7684-6.5-2`](#rfc7684-6.5-2)

## Figures

- **MUST bar**: Ze verified 4, Tests awaiting verification 2, Not built 3
- **Overview bar**: Ze MUST verified 4, Tests awaiting verification 2, Not assessed 11, Not built 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 4 |
| OS verified | 0 |
| Tests awaiting verification | 2 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 3 |
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

Source: [data/rfc/rfc7684.json](../../../data/rfc/rfc7684.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7684-5-3` | Malformed LSAs MUST NOT be stored in the Link State Database (LSDB), acknowledged, or reflooded. (§5). `opaque.go:wireOpaqueDelivery` installs `ext.go:validateExtLSA` through `lsdb.SetReceiveValidator`; `lsdb.ReceiveUpdate` invokes it before every scope, self-originated and MaxAge branch. `packet.ValidateExtLSABody` checks top-level and nested framing without allocating attributes, including duplicate containers. Rejection increments `ze_ospf_ext_malformed_total`. `TestExtIngressMalformedDiscard` and `TestExtIngressValidCarriage` exercise the router dispatcher, LSDB and emitted acknowledgement/flood packets. Unknown opaque applications retain carrier behaviour. | MUST NOT | 5 | **positive:** `unit/verify` [`TestExtIngressValidCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_receive_test.go#L198). **negative:** `unit/verify` [`TestExtIngressMalformedDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_receive_test.go#L135) |
| `RFC7684-2.1-1` | If the flag is set and the prefix length is not a host prefix, then the flag MUST be ignored. (§2.1) -- `extNormalizeFlags` clears it on receive | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L77). **negative:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L66) |
| `RFC7684-2.1-2` | The flag is preserved when the OSPFv2 Extended Prefix Opaque LSA is propagated between areas. (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L121). **positive:** `unit/verify` [`TestRFC7684NFlagOfAnotherRouterPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_nflag_propagation_red_test.go#L64). **negative:** `unit/verify` [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L150). **negative:** `unit/verify` [`TestRFC7684NFlagNotInventedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_nflag_propagation_red_test.go#L83) |
| `RFC7684-2.1-3` | However, since the Opaque LSA type defines the flooding scope, the LSA flooding scope MUST satisfy the application-specific requirements for all the prefixes included in a single OSPFv2 Extended Prefix Opaque LSA. (§2.1) -- `extPrefixScope`: area scope for the intra-area and inter-area prefixes Ze originates, AS scope for AS-external; Ze originates no NSSA-external, route type 7, Extended Prefix TLV | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L170). **positive:** `unit/verify` [`TestRFC7684EveryPrefixInAnLSAFitsItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_scope_per_lsa_test.go#L63). **negative:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L173). **negative:** `unit/verify` [`TestRFC7684PrefixNeedingTwoScopesNeverSharesAnLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_scope_per_lsa_test.go#L100) |
| `RFC7684-3.1-1` | Only one OSPFv2 Extended Link TLV SHALL be advertised in each OSPFv2 Extended Link Opaque LSA (§3.1) -- origination emits one per LSA; decode uses the first and logs extras | SHALL | 3.1 | **positive:** `unit/verify` [`TestExtLinkMirrorsRouterLSALink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_link_origin_test.go#L46). **negative:** `unit/verify` [`TestExtLinkSingleTLVEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_link_test.go#L74) |
| `RFC7684-4-1` | However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an obligation on future OSPFv2 application specifications, which no sentence of Section 4 scopes to a role in a form the gate accepts (the row's own sentence opens with However and a comma); ze originates backward-compatible empty Extended Prefix and Link containers (internal/plugins/ospf/ext_prefix.go::extPrefixOnOriginate, internal/plugins/ospf/ext_link.go), and no test asserts it |
| `RFC7684-5-1` | Additionally, implementations must assure that malformed TLV and sub-TLV permutations are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target | MUST | 5 | **positive:** `unit/verify` [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_link_test.go#L25). **positive:** `unit/verify` [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_prefix_test.go#L28). **positive:** `unit/verify` [`TestRFC7684MalformedPermutationsDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_malformed_test.go#L27). **negative:** `unit/verify` [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_fuzz_test.go#L111). **negative:** `unit/verify` [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_fuzz_test.go#L91). **negative:** `unit/verify` [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L160). **negative:** `unit/verify` [`TestRFC7684MalformedPermutationsDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_malformed_test.go#L30) |
| `RFC7684-6.5-1` | Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | MUST NOT | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an IANA registry rule with no implementer role the RFC names; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range (internal/plugins/ospf/ext_prefix.go, internal/plugins/ospf/ext_link.go), and no test asserts it |
| `RFC7684-6.5-2` | Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an IANA allocation rule with no implementer role the RFC names; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range (internal/plugins/ospf/ext_prefix.go, internal/plugins/ospf/ext_link.go), and no test asserts it |
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
| [`RFC7684-4-1`](#rfc7684-4-1) However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | {gap}, no test | an obligation on future OSPFv2 application specifications, which no sentence of Section 4 scopes to a role in a form the gate accepts (the row's own sentence opens with However and a comma); ze originates backward-compatible empty Extended Prefix and Link containers (internal/plugins/ospf/ext_prefix.go::extPrefixOnOriginate, internal/plugins/ospf/ext_link.go), and no test asserts it |
| [`RFC7684-6.5-1`](#rfc7684-6.5-1) Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | {gap}, no test | an IANA registry rule with no implementer role the RFC names; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range (internal/plugins/ospf/ext_prefix.go, internal/plugins/ospf/ext_link.go), and no test asserts it |
| [`RFC7684-6.5-2`](#rfc7684-6.5-2) Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4 | {gap}, no test | an IANA allocation rule with no implementer role the RFC names; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range (internal/plugins/ospf/ext_prefix.go, internal/plugins/ospf/ext_link.go), and no test asserts it |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7684-5-3`](#rfc7684-5-3)

Malformed LSAs MUST NOT be stored in the Link State Database (LSDB), acknowledged, or reflooded. (§5). `opaque.go:wireOpaqueDelivery` installs `ext.go:validateExtLSA` through `lsdb.SetReceiveValidator`; `lsdb.ReceiveUpdate` invokes it before every scope, self-originated and MaxAge branch. `packet.ValidateExtLSABody` checks top-level and nested framing without allocating attributes, including duplicate containers. Rejection increments `ze_ospf_ext_malformed_total`. `TestExtIngressMalformedDiscard` and `TestExtIngressValidCarriage` exercise the router dispatcher, LSDB and emitted acknowledgement/flood packets. Unknown opaque applications retain carrier behaviour.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged after the D-15 body fix in TestExtIngressValidCarriage (unknown-application body ff -> ff 00 00 00, assertions unchanged): a 1-octet Opaque body is no Opaque LSA under RFC 5250 Section 3, so the positive now carries a valid unknown application; it still proves valid extended LSAs and unknown applications are stored byte-for-byte, acked and flooded by scope after a malformed companion. Negative TestExtIngressMalformedDiscard unchanged: TLV overrun, sub-TLV overrun, short sub-TLV header, duplicate malformed container, MaxAge and self-originated cases are 4-octet aligned and reach validateExtLSA; revert record on ext.go validateExtLSA observed red. Its trailing-header case (valid + 1 octet) is now discarded first by the RFC 5250 alignment check, still the outcome this row forbids. Positive revert record on ReceiveUpdate.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtIngressMalformedDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_receive_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestExtIngressValidCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_receive_test.go#L198) | unit/verify | revert, verified |

### [`RFC7684-2.1-1`](#rfc7684-2.1-1)

If the flag is set and the prefix length is not a host prefix, then the flag MUST be ignored. (§2.1) -- `extNormalizeFlags` clears it on receive

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: unit changed only by the lookupPrefix area argument (types.BackboneArea, the zero AreaID the receive path keys an unset area with); assertions unchanged. Forbidden behaviour: honouring an N-Flag received on a prefix that is not a host prefix. TestExtPrefixNFlagIgnoredNonHost drives extPrefixOnReceive with N set on a /24 and fails if the stored entry's flags still carry ExtPrefixFlagN; the positive half fails if N is dropped from a /32 host prefix. Both polarities present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L66) | unit/verify | not verified |
| positive | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L77) | unit/verify | not verified |

### [`RFC7684-2.1-2`](#rfc7684-2.1-2)

The flag is preserved when the OSPFv2 Extended Prefix Opaque LSA is propagated between areas. (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the D-8 fix. Forbidden: an ABR dropping (or inventing) the N-Flag when Extended Prefix information crosses areas. extRecv now keys received entries by source area (extRecvKey.area; AS scope keys the zero area), and selfPrefixAdverts ORs N into the inter-area TLV when hostFlagOutside finds an area-scope entry for the prefix, in an area other than the summary's target, carrying N (non-host N already cleared on receipt). + TestRFC7684NFlagOfAnotherRouterPreservedInterArea: 2.2.2.2's N /32 received in area 0, this ABR's Type-3 into area 1 carries N (prefix not connected locally, so only the received flag can set it; red before the fix). - TestRFC7684NFlagNotInventedInterArea: no N when the area-0 advertisement lacks it, when N is advertised only in the target area 1, or on a non-host /24. Records on ext.go::hostFlagOutside (+/-) observed red. Older ext_prefix_origin_test tags (own connected host /32) stay supplementary and carry no record. The handoff's suggested 'via that router' condition is not applied: any other-area N advertisement for the same host prefix sets N, which matches 'preserved' for a prefix identifying one router.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L150) | unit/verify | revert, verified |
| negative | [`TestRFC7684NFlagNotInventedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_nflag_propagation_red_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC7684NFlagOfAnotherRouterPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_nflag_propagation_red_test.go#L64) | unit/verify | revert, verified |

### [`RFC7684-2.1-3`](#rfc7684-2.1-3)

However, since the Opaque LSA type defines the flooding scope, the LSA flooding scope MUST satisfy the application-specific requirements for all the prefixes included in a single OSPFv2 Extended Prefix Opaque LSA. (§2.1) -- `extPrefixScope`: area scope for the intra-area and inter-area prefixes Ze originates, AS scope for AS-external; Ze originates no NSSA-external, route type 7, Extended Prefix TLV

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged over the reworded note. RFC 7684 Section 2.1: "the LSA flooding scope MUST satisfy the application-specific requirements for all the prefixes included in a single OSPFv2 Extended Prefix Opaque LSA." TestExtPrefixScopeSelection pins extPrefixScope (ext.go) literally: types 1, 3 and 7 map to area, type 5 to AS. The per-LSA pair drives the real extPrefixOnOriginate over stub, Type-3 and Type-5 self LSAs, checks every decoded TLV against its LSA's scope with literal counts (3 area, 2 AS), and proves the prefix needing two scopes never shares an LSA. Note verified: selfPrefixAdverts reads only Router, Summary-Network and AS-External self LSAs, so no route type 7 Extended Prefix TLV is originated, although Ze originates Type-7 LSAs (lsdb/nssa.go OriginateNSSA). RFC 7684 puts no obligation on originating an Extended Prefix TLV for NSSA prefixes: Section 2 leaves the LSA under the control of the advertising router.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L173) | unit/verify | revert, verified |
| negative | [`TestRFC7684PrefixNeedingTwoScopesNeverSharesAnLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_scope_per_lsa_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_origin_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestRFC7684EveryPrefixInAnLSAFitsItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_scope_per_lsa_test.go#L63) | unit/verify | revert, verified |

### [`RFC7684-3.1-1`](#rfc7684-3.1-1)

Only one OSPFv2 Extended Link TLV SHALL be advertised in each OSPFv2 Extended Link Opaque LSA (§3.1) -- origination emits one per LSA; decode uses the first and logs extras

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtLinkSingleTLVEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_link_test.go#L74) | unit/verify | not verified |
| positive | [`TestExtLinkMirrorsRouterLSALink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_link_origin_test.go#L46) | unit/verify | not verified |

### [`RFC7684-4-1`](#rfc7684-4-1)

However, future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality. (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7684-4-1, so there is no linked test.

### [`RFC7684-5-1`](#rfc7684-5-1)

Additionally, implementations must assure that malformed TLV and sub-TLV permutations are detected and do not provide a vulnerability for attackers to crash the OSPFv2 router or routing process. (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-10-21 over current tags, units and records. Row text correction only ('sub- TLV' to 'sub-TLV', rfc/corrections/rfc7684.md), checked against RFC 7684 Section 5. Units unchanged: the malformed-permutation table and the fuzz targets detect without panic, and well-formed bodies decode.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_fuzz_test.go#L111) | unit/verify | revert, verified |
| negative | [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_fuzz_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestRFC7684MalformedPermutationsDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_malformed_test.go#L30) | unit/verify | revert, verified |
| negative | [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7684_ext_prefix_recv_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_link_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_ext_prefix_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC7684MalformedPermutationsDetected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7684_malformed_test.go#L27) | unit/verify | revert, verified |

### [`RFC7684-6.5-1`](#rfc7684-6.5-1)

Types in the range 32768-33023 are for Experimental Use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7684-6.5-1, so there is no linked test.

### [`RFC7684-6.5-2`](#rfc7684-6.5-2)

Before any assignments can be made in the 33024-65535 range, there MUST be an IETF specification that specifies IANA considerations covering the range being assigned. (§6.5) -- the same sentence governs the Extended Prefix Opaque LSA TLVs, Extended Prefix TLV Sub-TLVs and Extended Link Opaque LSA TLVs registries of sections 6.1, 6.2 and 6.4

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7684-6.5-2, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 2 | reviewed | not stated |
| `6.2` | not stated | 2 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `6.4` | not stated | 2 | reviewed | not stated |
| `6.5` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 7684 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7684, so its obligations are stated where they were written.
