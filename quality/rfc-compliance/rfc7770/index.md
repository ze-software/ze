# RFC 7770 - Extensions to OSPF for Advertising Optional Router Capabilities

OSPF Router Information LSA. Every requirement listed for RFC 7770, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 11 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 3 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 10 | 10 of 13 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 6 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 1 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 25 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | OSPF Router Information LSA |
| Automatic checks | Included |
| Requirements | 25 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tests linked to requirements | 13 |
| Recorded audit verdicts | 5 |
| Fault test records | 10 |
| Summary | `rfc/short/rfc7770.md` |
| Requirement table | `rfc/requirements/rfc7770.md` |
| RFC text | `rfc/full/rfc7770.txt` |

## Automatic checks

Included in automatic checks: OSPF Router Information LSA (RFC 7770): ze is an OSPF router that originates the RI LSA. 1 MET (capabilities reflect config) + 3 single-polarity positive (TLV ordering, functional-cap zero) + 1 not-applicable (2.6-3 binds the specifications for functional capabilities) + 6 gaps: five obligations of IETF document authors and the IANA process whose sentences the evidence shapes do not admit, with no ze producer to test, and the Vendor Private Use Enterprise Code rule, which holds by construction with no test

## Published support status

**Area:** OSPF Router Information LSA

**Test verification:** 1 of 11 MUST proven

**Evidence:** unit 1

**Unmet:** RFC7770-2-1, RFC7770-2.3-1, RFC7770-2.7-1, RFC7770-5.2-1, RFC7770-5.2-2, RFC7770-5.3-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 10 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC7770-2.4-2`](#rfc7770-2.4-2)

**Annotated (including scoped evidence) (10):** [`RFC7770-2.4-1`](#rfc7770-2.4-1), [`RFC7770-2.6-1`](#rfc7770-2.6-1), [`RFC7770-2.6-2`](#rfc7770-2.6-2), [`RFC7770-2.3-1`](#rfc7770-2.3-1), [`RFC7770-2.6-3`](#rfc7770-2.6-3), [`RFC7770-2.7-1`](#rfc7770-2.7-1), [`RFC7770-5.2-1`](#rfc7770-5.2-1), [`RFC7770-2-1`](#rfc7770-2-1), [`RFC7770-5.3-1`](#rfc7770-5.3-1), [`RFC7770-5.2-2`](#rfc7770-5.2-2)

## Figures

- **MUST bar**: Ze verified 1, Tests awaiting verification 3, Not built 6
- **Overview bar**: Ze MUST verified 1, SHOULD verified 1, Not owed 1, Tests awaiting verification 3, Not assessed 13, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 10 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 14 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 13 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7770.json](../../../data/rfc/rfc7770.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7770-2.4-1` | If included, it MUST be the first TLV in the first instance, i.e., Instance 0, of the OSPF RI LSA. (§2.4) -- Ze emits the type-1 TLV first, spec-ospf-ext-3, `buildRIInstances` | MUST | 2.4 | **positive:** `unit/verify` [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L124). **negative:** no negative test. **{single-polarity}:** ze always emits the type-1 Informational Capabilities TLV first in Instance 0 and, being informational-only on receive, never rejects a peer that misorders it, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:153) |
| `RFC7770-2.4-2` | Additionally, the TLV MUST accurately reflect the OSPF router's capabilities in the scope advertised. (§2.4) -- derived from live config, `deriveRICapabilities` | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L147). **positive:** `unit/verify` [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L49). **positive:** `unit/verify` [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L108). **positive:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L63). **positive:** `unit/verify` [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L79). **negative:** `unit/verify` [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L135). **negative:** `unit/verify` [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L55). **negative:** `unit/verify` [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L113). **negative:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L57) |
| `RFC7770-2.6-1` | If included, it MUST be the included in the first instance of the LSA. (§2.6) -- Ze carries the empty type-2 TLV in Instance 0 | MUST | 2.6 | **positive:** `unit/verify` [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_registry_test.go#L51). **negative:** no negative test. **{single-polarity}:** ze carries the type-2 Functional Capabilities TLV in Instance 0's lead on every origination and does not police peer placement on receive, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.6-2` | Additionally, the TLV MUST reflect the advertising OSPF router's actual functional capabilities since the information will be used to dictate OSPF protocol operation in the flooding scope of the containing OSPF RI LSA. (§2.6) -- carried empty, no functional capability supported | MUST | 2.6 | **positive:** `unit/verify` [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L154). **negative:** no negative test. **{single-polarity}:** ze supports no functional capability, so it emits the constant all-zero type-2 value (accurately none-supported), and there is no variable capability to exercise the opposite direction (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.3-1` | When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on the specification that defines a new Router Information LSA TLV, and ze writes none; the sentence names its party after a conditional clause, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| `RFC7770-2.6-3` | The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility."; binds specifications for functional capabilities advertised in this TLV; ze writes no specification: it is an OSPF router that originates the Router Information LSA (internal/plugins/ospf/ri_registry.go) and advertises no functional capability |
| `RFC7770-2.7-1` | TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on the specifications that define future Router Information LSA TLVs, and ze writes none; the sentence is in the passive voice, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| `RFC7770-5.2-1` | o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates no OSPFv3 LSA with a Vendor Private Use function code 8184-8190 (no such function code is used in internal/plugins/ospf), so no Enterprise Code is owed today; the rule holds by construction, with no test |
| `RFC7770-2-1` | If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on whoever documents a new OSPFv3 LSA function code, and ze implements the existing function code 12 and documents no new one; the sentence names its party after a conditional clause, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| `RFC7770-5.3-1` | Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on the IETF and IANA assignment process for the reserved range 32778-65535, and ze registers no TLV type; the sentence names no party that could be quoted, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| `RFC7770-5.2-2` | New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on RFCs, which may not mention the experimental function codes 8176-8183, and ze writes no RFC; the sentence is in the passive voice, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| `RFC7770-2.1-1` | The first Opaque ID, i.e., 0, SHOULD always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV. (§2.1) -- Instance 0 always carries type-1 then type-2, spec-ospf-ext-3 | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.2-2` | The first Instance ID, i.e., 0, SHOULD always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV. (§2.2) | SHOULD | 2.2 | **positive:** `unit/verify` [`TestRFC7770V3InstanceZeroCarriesCapabilities`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_v3_instance0_test.go#L44). **negative:** no negative test. **{single-polarity}:** ze originates the OSPFv3 RI LSA with Instance ID 0 leading with the type-1 Informational Capabilities TLV and then the type-2 Functional Capabilities TLV on every origination, and, being informational-only on receive, never rejects a peer whose Instance 0 lacks them, so only the positive direction is meaningful (internal/plugins/ospf/origination_v6_ri.go::v6OriginateRIScope) |
| `RFC7770-2.7-2` | If AS-wide flooding scope is chosen, the originating router should also advertise area-scoped LSA(s) into any attached Not-So-Stubby Area (NSSA) area(s). (§2.7) -- Ze originates area-scoped RI into attached NSSAs when AS scope is selected | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-5.2-3` | If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. It SHOULD also describe how the Link State ID is to be assigned. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-3-1` | For backwards compatibility, previously advertised Router Information TLVs SHOULD continue to be advertised in the first instance, i.e., 0, of the Router Information LSA. (§3) -- Instance 0 retains the type-1 TLV; overflow spills to Instance 1+ | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-1-1` | For future OSPF extensions, this advertisement MAY be used as the sole mechanism for advertisement and discovery. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.2-1` | OSPFv3 routers MAY advertise multiple RI LSAs per flooding scope. (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.4-3` | An OSPF router advertising an OSPF RI LSA MAY include the Router Informational Capabilities TLV. (§2.4) -- Ze always includes it | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.4-4` | The Router Informational Capabilities TLV MAY be followed by optional TLVs that further specify a capability. (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-4` | An OSPF router advertising an OSPF RI LSA MAY include the Router Functional Capabilities TLV. (§2.6) -- Ze carries it, empty by default | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-5` | In contrast to the Router Informational Capabilities TLV, the OSPF extensions advertised in this TLV MAY be used by other OSPF routers to dictate protocol operation. (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-6` | The Router Functional Capabilities TLV MAY be followed by optional TLVs that further specify a capability. (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.7-3` | An OSPF router MAY advertise different capabilities when both NSSA area-scoped LSA(s) and an AS-scoped LSA are advertised. This allows functional capabilities to be limited in scope. (§2.7) | MAY | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.7-4` | The originating router MAY advertise multiple RI LSAs with the same Instance ID as long as the flooding scopes differ. (§2.7) | MAY | 2.7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7770-2.3-1`](#rfc7770-2.3-1) When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3) | {gap}, no test | the obligation falls on the specification that defines a new Router Information LSA TLV, and ze writes none; the sentence names its party after a conditional clause, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| [`RFC7770-2.6-3`](#rfc7770-2.6-3) The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: "The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility."; binds specifications for functional capabilities advertised in this TLV; ze writes no specification: it is an OSPF router that originates the Router Information LSA (internal/plugins/ospf/ri_registry.go) and advertises no functional capability |
| [`RFC7770-2.7-1`](#rfc7770-2.7-1) TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7) | {gap}, no test | the obligation falls on the specifications that define future Router Information LSA TLVs, and ze writes none; the sentence is in the passive voice, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| [`RFC7770-5.2-1`](#rfc7770-5.2-1) o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2) | {gap}, no test | ze originates no OSPFv3 LSA with a Vendor Private Use function code 8184-8190 (no such function code is used in internal/plugins/ospf), so no Enterprise Code is owed today; the rule holds by construction, with no test |
| [`RFC7770-2-1`](#rfc7770-2-1) If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md | {gap}, no test | the obligation falls on whoever documents a new OSPFv3 LSA function code, and ze implements the existing function code 12 and documents no new one; the sentence names its party after a conditional clause, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| [`RFC7770-5.3-1`](#rfc7770-5.3-1) Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3) | {gap}, no test | the obligation falls on the IETF and IANA assignment process for the reserved range 32778-65535, and ze registers no TLV type; the sentence names no party that could be quoted, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |
| [`RFC7770-5.2-2`](#rfc7770-5.2-2) New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2) | {gap}, no test | the obligation falls on RFCs, which may not mention the experimental function codes 8176-8183, and ze writes no RFC; the sentence is in the passive voice, so neither evidence shape admits a not-applicable quote, and the row stays a gap with no ze producer to test |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7770-2.4-1`](#rfc7770-2.4-1)

If included, it MUST be the first TLV in the first instance, i.e., Instance 0, of the OSPF RI LSA. (§2.4) -- Ze emits the type-1 TLV first, spec-ospf-ext-3, `buildRIInstances`

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the type-1 Informational Capabilities TLV not being the first TLV of Instance 0. ri_test.go TestRITLVType1First registers a type-8 builder, decodes buildRIInstances(area scope, backbone)[0] (Instance 0) and fails unless decoded[0].Type is RITLVInformationalCapabilities. Re-judged 2026-09-30 after the call site gained the (area, iface) scope target; the assertion is unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L124) | unit/verify | not verified |

### [`RFC7770-2.4-2`](#rfc7770-2.4-2)

Additionally, the TLV MUST accurately reflect the OSPF router's capabilities in the scope advertised. (§2.4) -- derived from live config, `deriveRICapabilities`

Audit verdict: enforced (the tests do what the requirement demands), fresh. All three scopes proven both ways. Area: TestRFC7770AreaRICapabilitiesFollowTheArea (backbone RI with TE on eth0 sets TE; area 0.0.0.5 RI with no TE interface clear; records areaHasTE +, deriveRICapabilities -). Link: TestRFC7770LinkRICapabilitiesFollowTheInterface (same area, eth0 TE Type-9 RI sets TE, eth1 Type-9 RI clear; records riScopeHasTE +/-). AS: TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns isolates each source: router-address 9.9.9.9 alone (premise asserted: teOriginateType1 originates no TE LSA) leaves the AS RI TE clear, a TE interface alone sets it (records anyInterfaceHasTE +, riScopeHasTE -). D-8 defect verified at the producer and fixed: the AS case was HasTERouterAddress || anyInterfaceHasTE while teOriginateType1 gates on anyTEActive, so a router address alone claimed TE with no TE LSA anywhere; now anyInterfaceHasTE (TE.active(), the same test as anyTEActive). Older whole-router tags (TestRICapabilityBitsFromState, TestRICapabilityTEBitFromConfig) remain as supplementary AS-scope proof.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L135) | unit/verify | revert, verified |
| negative | [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L113) | unit/verify | revert, verified |
| negative | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L79) | unit/verify | revert, verified |

### [`RFC7770-2.6-1`](#rfc7770-2.6-1)

If included, it MUST be the included in the first instance of the LSA. (§2.6) -- Ze carries the empty type-2 TLV in Instance 0

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the type-2 Functional Capabilities TLV carried outside the first instance. ri_registry_test.go TestRITLVRegistered requires buildRIInstances(area scope, backbone) to return exactly one instance whose TLV types are [1, 2, 8], so type-2 is in Instance 0. Re-judged 2026-09-30 after the call-site signature change; assertion unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_registry_test.go#L51) | unit/verify | not verified |

### [`RFC7770-2.6-2`](#rfc7770-2.6-2)

Additionally, the TLV MUST reflect the advertising OSPF router's actual functional capabilities since the information will be used to dictate OSPF protocol operation in the flooding scope of the containing OSPF RI LSA. (§2.6) -- carried empty, no functional capability supported

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising a functional capability Ze does not have. Ze supports none, so any set bit is non-compliant; ri_test.go TestRIFunctionalCapabilitiesEmittedZero fails if any byte of the type-2 value in Instance 0 is non-zero or RIReadCapabilities is non-zero. Re-judged 2026-09-30 after the call-site signature change; assertion unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L154) | unit/verify | not verified |

### [`RFC7770-2.3-1`](#rfc7770-2.3-1)

When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-2.3-1, so there is no linked test.

### [`RFC7770-2.6-3`](#rfc7770-2.6-3)

The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-2.6-3, so there is no linked test.

### [`RFC7770-2.7-1`](#rfc7770-2.7-1)

TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-2.7-1, so there is no linked test.

### [`RFC7770-5.2-1`](#rfc7770-5.2-1)

o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-5.2-1, so there is no linked test.

### [`RFC7770-2-1`](#rfc7770-2-1)

If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-2-1, so there is no linked test.

### [`RFC7770-5.3-1`](#rfc7770-5.3-1)

Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-5.3-1, so there is no linked test.

### [`RFC7770-5.2-2`](#rfc7770-5.2-2)

New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7770-5.2-2, so there is no linked test.

### [`RFC7770-2.2-2`](#rfc7770-2.2-2)

The first Instance ID, i.e., 0, SHOULD always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7770 Section 2.2: 'The first Instance ID, i.e., 0, SHOULD always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV.' The test originates through the real OSPFv3 producer v6OriginateRIScope, reads the installed LSA with Link State ID 0 back from the LSDB and asserts TLV type 1 then type 2; a body that dropped or displaced either TLV turns it red (Ze always advertises the functional TLV, so 'if advertised' holds). Order is stricter than the RFC asks but cannot pass a non-conforming LSA. Single-polarity positive is sound: the SHOULD binds the originator, and receive is informational. Covers area scope with no overflow; the lead placement is in the shared packRIInstances that RFC7770-2.1-1 and 3-1 exercise.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC7770V3InstanceZeroCarriesCapabilities`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_v3_instance0_test.go#L44) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7770.txt |
| Source fingerprint | 7eec57f4cf8e66b8 |
| Record | rfc/extraction/rfc7770.json |
| Mapped sentences | 11 |
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
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 1 | reviewed | not stated |
| `2.4` | not stated | 2 | reviewed | not stated |
| `2.5` | not stated | 0 | reviewed | not stated |
| `2.6` | not stated | 3 | reviewed | not stated |
| `2.7` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 3 | reviewed | not stated |
| `5.3` | not stated | 1 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `5.5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 7770 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7770, so its obligations are stated where they were written.
