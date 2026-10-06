# RFC 7770 - Extensions to OSPF for Advertising Optional Router Capabilities

Experimental. Every requirement this repository extracted from RFC 7770, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9.1% | 1 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 27.3% | 3 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 50.0% | 6 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 63.6% | 7 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 4 |
| Discrimination records | 6 |
| Summary | `rfc/short/rfc7770.md` |
| Requirement shard | `rfc/requirements/rfc7770.md` |
| RFC text | `rfc/full/rfc7770.txt` |

## Enrolment

Enrolled: OSPF Router Information LSA (RFC 7770): 1 MET (capabilities reflect config) + 3 single-polarity positive (TLV ordering, functional-cap zero) + 7 not-applicable (IETF document-author / IANA-process obligations)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Router Information LSA body and multi-instance ordering.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7770-2.4-2`](#rfc7770-2.4-2)

**Annotated (including scoped evidence) (10):** [`RFC7770-2.4-1`](#rfc7770-2.4-1), [`RFC7770-2.6-1`](#rfc7770-2.6-1), [`RFC7770-2.6-2`](#rfc7770-2.6-2), [`RFC7770-2.3-1`](#rfc7770-2.3-1), [`RFC7770-2.6-3`](#rfc7770-2.6-3), [`RFC7770-2.7-1`](#rfc7770-2.7-1), [`RFC7770-5.2-1`](#rfc7770-5.2-1), [`RFC7770-2-1`](#rfc7770-2-1), [`RFC7770-5.3-1`](#rfc7770-5.3-1), [`RFC7770-5.2-2`](#rfc7770-5.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7770-2.4-1` | If included, it MUST be the first TLV in the first instance, i.e., Instance 0, of the OSPF RI LSA. (§2.4) -- Ze emits the type-1 TLV first, spec-ospf-ext-3, `buildRIInstances` | MUST | 2.4 | **positive:** `unit/verify` [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L124). **negative:** no negative test. **{single-polarity}:** ze always emits the type-1 Informational Capabilities TLV first in Instance 0 and, being informational-only on receive, never rejects a peer that misorders it, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:153) |
| `RFC7770-2.4-2` | Additionally, the TLV MUST accurately reflect the OSPF router's capabilities in the scope advertised. (§2.4) -- derived from live config, `deriveRICapabilities` | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L147). **positive:** `unit/verify` [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L49). **positive:** `unit/verify` [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L108). **positive:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L63). **positive:** `unit/verify` [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L79). **negative:** `unit/verify` [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L135). **negative:** `unit/verify` [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L55). **negative:** `unit/verify` [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L113). **negative:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L57) |
| `RFC7770-2.6-1` | If included, it MUST be the included in the first instance of the LSA. (§2.6) -- Ze carries the empty type-2 TLV in Instance 0 | MUST | 2.6 | **positive:** `unit/verify` [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_registry_test.go#L51). **negative:** no negative test. **{single-polarity}:** ze carries the type-2 Functional Capabilities TLV in Instance 0's lead on every origination and does not police peer placement on receive, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.6-2` | Additionally, the TLV MUST reflect the advertising OSPF router's actual functional capabilities since the information will be used to dictate OSPF protocol operation in the flooding scope of the containing OSPF RI LSA. (§2.6) -- carried empty, no functional capability supported | MUST | 2.6 | **positive:** `unit/verify` [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L154). **negative:** no negative test. **{single-polarity}:** ze supports no functional capability, so it emits the constant all-zero type-2 value (accurately none-supported), and there is no variable capability to exercise the opposite direction (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.3-1` | When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of an IETF specification that defines a new RI TLV; ze implements TLVs but publishes no such specification, so it plays no role this MUST governs |
| `RFC7770-2.6-3` | The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of a functional-capability specification; ze advertises no functional capability and authors no such document |
| `RFC7770-2.7-1` | TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of a specification that defines a new RI-TLV; ze selects flooding scope per configuration and does not specify TLV scope rules in a standards document |
| `RFC7770-5.2-1` | o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no OSPFv3 Vendor Private Use LSA (no reference to function codes 8184-8190 in internal/plugins/ospf/), so the requirement's antecedent never holds |
| `RFC7770-2-1` | If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the party documenting a new OSPFv3 function code; ze implements the already-defined function code 12 rather than documenting a new one, so the antecedent is false (the U/S2/S1 combinations are recorded at docs/architecture/wire/ospf.md) |
| `RFC7770-5.3-1` | Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the IANA/IETF assignment process; ze registers no TLV type in the reserved 32778-65535 range and cannot author a Standards Track RFC |
| `RFC7770-5.2-2` | New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds RFC authors; ze is an implementation, not an RFC, and references no experimental function code 8176-8183 in internal/plugins/ospf/ |
| `RFC7770-2.1-1` | The first Opaque ID, i.e., 0, SHOULD always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV. (§2.1) -- Instance 0 always carries type-1 then type-2, spec-ospf-ext-3 | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
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
| [`RFC7770-2.3-1`](#rfc7770-2.3-1) When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of an IETF specification that defines a new RI TLV; ze implements TLVs but publishes no such specification, so it plays no role this MUST governs |
| [`RFC7770-2.6-3`](#rfc7770-2.6-3) The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of a functional-capability specification; ze advertises no functional capability and authors no such document |
| [`RFC7770-2.7-1`](#rfc7770-2.7-1) TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of a specification that defines a new RI-TLV; ze selects flooding scope per configuration and does not specify TLV scope rules in a standards document |
| [`RFC7770-5.2-1`](#rfc7770-5.2-1) o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no OSPFv3 Vendor Private Use LSA (no reference to function codes 8184-8190 in internal/plugins/ospf/), so the requirement's antecedent never holds |
| [`RFC7770-2-1`](#rfc7770-2-1) If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md | no test | no test carries this requirement id; annotated {not-applicable}: this binds the party documenting a new OSPFv3 function code; ze implements the already-defined function code 12 rather than documenting a new one, so the antecedent is false (the U/S2/S1 combinations are recorded at docs/architecture/wire/ospf.md) |
| [`RFC7770-5.3-1`](#rfc7770-5.3-1) Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the IANA/IETF assignment process; ze registers no TLV type in the reserved 32778-65535 range and cannot author a Standards Track RFC |
| [`RFC7770-5.2-2`](#rfc7770-5.2-2) New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: this binds RFC authors; ze is an implementation, not an RFC, and references no experimental function code 8176-8183 in internal/plugins/ospf/ |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7770-2.4-1`](#rfc7770-2.4-1)

If included, it MUST be the first TLV in the first instance, i.e., Instance 0, of the OSPF RI LSA. (§2.4) -- Ze emits the type-1 TLV first, spec-ospf-ext-3, `buildRIInstances`

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the type-1 Informational Capabilities TLV not being the first TLV of Instance 0. ri_test.go TestRITLVType1First registers a type-8 builder, decodes buildRIInstances(area scope, backbone)[0] (Instance 0) and fails unless decoded[0].Type is RITLVInformationalCapabilities. Re-judged 2026-09-30 after the call site gained the (area, iface) scope target; the assertion is unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L124) | unit/verify | unproven |

### [`RFC7770-2.4-2`](#rfc7770-2.4-2)

Additionally, the TLV MUST accurately reflect the OSPF router's capabilities in the scope advertised. (§2.4) -- derived from live config, `deriveRICapabilities`

Audit verdict: enforced (the tests do what the requirement demands), fresh. All three scopes proven both ways. Area: TestRFC7770AreaRICapabilitiesFollowTheArea (backbone RI with TE on eth0 sets TE; area 0.0.0.5 RI with no TE interface clear; records areaHasTE +, deriveRICapabilities -). Link: TestRFC7770LinkRICapabilitiesFollowTheInterface (same area, eth0 TE Type-9 RI sets TE, eth1 Type-9 RI clear; records riScopeHasTE +/-). AS: TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns isolates each source: router-address 9.9.9.9 alone (premise asserted: teOriginateType1 originates no TE LSA) leaves the AS RI TE clear, a TE interface alone sets it (records anyInterfaceHasTE +, riScopeHasTE -). D-8 defect verified at the producer and fixed: the AS case was HasTERouterAddress || anyInterfaceHasTE while teOriginateType1 gates on anyTEActive, so a router address alone claimed TE with no TE LSA anywhere; now anyInterfaceHasTE (TE.active(), the same test as anyTEActive). Older whole-router tags (TestRICapabilityBitsFromState, TestRICapabilityTEBitFromConfig) remain as supplementary AS-scope proof.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L135) | unit/verify | revert, verified |
| negative | [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L113) | unit/verify | revert, verified |
| negative | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L57) | unit/verify | unproven |
| positive | [`TestRFC7770ASRICapabilitiesClaimTEOnlyWhenTERuns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestRFC7770AreaRICapabilitiesFollowTheArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC7770LinkRICapabilitiesFollowTheInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_scope_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L63) | unit/verify | unproven |
| positive | [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L79) | unit/verify | unproven |

### [`RFC7770-2.6-1`](#rfc7770-2.6-1)

If included, it MUST be the included in the first instance of the LSA. (§2.6) -- Ze carries the empty type-2 TLV in Instance 0

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the type-2 Functional Capabilities TLV carried outside the first instance. ri_registry_test.go TestRITLVRegistered requires buildRIInstances(area scope, backbone) to return exactly one instance whose TLV types are [1, 2, 8], so type-2 is in Instance 0. Re-judged 2026-09-30 after the call-site signature change; assertion unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_registry_test.go#L51) | unit/verify | unproven |

### [`RFC7770-2.6-2`](#rfc7770-2.6-2)

Additionally, the TLV MUST reflect the advertising OSPF router's actual functional capabilities since the information will be used to dictate OSPF protocol operation in the flooding scope of the containing OSPF RI LSA. (§2.6) -- carried empty, no functional capability supported

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising a functional capability Ze does not have. Ze supports none, so any set bit is non-compliant; ri_test.go TestRIFunctionalCapabilitiesEmittedZero fails if any byte of the type-2 value in Instance 0 is non-zero or RIReadCapabilities is non-zero. Re-judged 2026-09-30 after the call-site signature change; assertion unchanged. Single-polarity marker on the row covers the absent negative.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7770_ri_test.go#L154) | unit/verify | unproven |

### [`RFC7770-2.3-1`](#rfc7770-2.3-1)

When a new Router Information LSA TLV is defined, the specification MUST explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both OSPFv2 and OSPFv3. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.3-1, so no unit is bound to it.

### [`RFC7770-2.6-3`](#rfc7770-2.6-3)

The specifications for functional capabilities advertised in this TLV MUST describe protocol behavior and address backwards compatibility. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.6-3, so no unit is bound to it.

### [`RFC7770-2.7-1`](#rfc7770-2.7-1)

TLV flooding-scope rules will be specified on a per- TLV basis and MUST be specified in the accompanying specifications for future Router Information LSA TLVs. (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.7-1, so no unit is bound to it.

### [`RFC7770-5.2-1`](#rfc7770-5.2-1)

o OSPFv3 LSAs with an LSA Function Code in the Vendor Private Use range 8184-8190 MUST include the Enterprise Code [ENTERPRISE-CODE] as the first 4 octets following the 20 octets of LSA header. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.2-1, so no unit is bound to it.

### [`RFC7770-2-1`](#rfc7770-2-1)

If a new LSA Function Code is documented, the documentation MUST include the valid combinations of the U, S2, and S1 bits for the LSA. (§5.2) -- U=1 with S2/S1 = link/area/AS, 0x800C/0xA00C/0xC00C, documented in docs/architecture/wire/ospf.md

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2-1, so no unit is bound to it.

### [`RFC7770-5.3-1`](#rfc7770-5.3-1)

Types in the range 32778-65535 are reserved and are not to be assigned at this time. Before any assignments can be made in this range, there MUST be a Standards Track RFC that specifies IANA Considerations that cover the range being assigned. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.3-1, so no unit is bound to it.

### [`RFC7770-5.2-2`](#rfc7770-5.2-2)

New values are assigned through RFCs that have been shepherded through the IESG as AD-Sponsored or IETF WG documents [IANA-GUIDE]. o OSPFv3 LSA function codes in the range 8176-8183 are for experimental use; these will not be registered with IANA and MUST NOT be mentioned by RFCs. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.2-2, so no unit is bound to it.

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 2 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 3 | walked | not stated |
| `2.7` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 1 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 7770 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7770, so its obligations are stated where they were written.
