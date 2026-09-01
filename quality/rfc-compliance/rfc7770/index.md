# RFC 7770 - Extensions to OSPF for Advertising Optional Router Capabilities

Experimental. Every requirement this repository extracted from RFC 7770, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.0% | 1 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 75.0% | 3 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7770-2.4-2`](#rfc7770-2.4-2)

**Annotated instead of tested (10):** [`RFC7770-2.4-1`](#rfc7770-2.4-1), [`RFC7770-2.6-1`](#rfc7770-2.6-1), [`RFC7770-2.6-2`](#rfc7770-2.6-2), [`RFC7770-2.3-1`](#rfc7770-2.3-1), [`RFC7770-2.6-3`](#rfc7770-2.6-3), [`RFC7770-2.7-1`](#rfc7770-2.7-1), [`RFC7770-5.2-1`](#rfc7770-5.2-1), [`RFC7770-2-1`](#rfc7770-2-1), [`RFC7770-5.3-1`](#rfc7770-5.3-1), [`RFC7770-5.2-2`](#rfc7770-5.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7770-2.4-1` | If the Router Informational Capabilities TLV is included, it must be the first TLV in the first instance (Instance 0) of the OSPF RI LSA (§2.4) -- Ze emits the type-1 TLV first (spec-ospf-ext-3, `buildRIInstances`) | MUST | 2.4 | **positive:** `unit/verify` [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L124). **negative:** no negative test. **{single-polarity}:** ze always emits the type-1 Informational Capabilities TLV first in Instance 0 and, being informational-only on receive, never rejects a peer that misorders it, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:153) |
| `RFC7770-2.4-2` | The Router Informational Capabilities TLV must accurately reflect the OSPF router's capabilities in the scope advertised (§2.4) -- derived from live config (`deriveRICapabilities`) | MUST | 2.4 | **positive:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L63). **positive:** `unit/verify` [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L79). **negative:** `unit/verify` [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L57) |
| `RFC7770-2.6-1` | If the Router Functional Capabilities TLV is included, it must be included in the first instance of the LSA (§2.6) -- Ze carries the empty type-2 TLV in Instance 0 | MUST | 2.6 | **positive:** `unit/verify` [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_registry_test.go#L51). **negative:** no negative test. **{single-polarity}:** ze carries the type-2 Functional Capabilities TLV in Instance 0's lead on every origination and does not police peer placement on receive, so only the positive direction is meaningful (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.6-2` | The Router Functional Capabilities TLV must reflect the advertising OSPF router's actual functional capabilities (§2.6) -- carried empty (no functional capability supported) | MUST | 2.6 | **positive:** `unit/verify` [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L154). **negative:** no negative test. **{single-polarity}:** ze supports no functional capability, so it emits the constant all-zero type-2 value (accurately none-supported), and there is no variable capability to exercise the opposite direction (internal/plugins/ospf/ri.go:155) |
| `RFC7770-2.3-1` | When a new Router Information LSA TLV is defined, the specification must explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of an IETF specification that defines a new RI TLV; ze implements TLVs but publishes no such specification, so it plays no role this MUST governs |
| `RFC7770-2.6-3` | The specifications for functional capabilities advertised in the Functional Capabilities TLV must describe protocol behavior and address backwards compatibility (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of a functional-capability specification; ze advertises no functional capability and authors no such document |
| `RFC7770-2.7-1` | TLV flooding-scope rules must be specified in the accompanying specifications for future Router Information LSA TLVs (§2.7) | MUST | 2.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of a specification that defines a new RI-TLV; ze selects flooding scope per configuration and does not specify TLV scope rules in a standards document |
| `RFC7770-5.2-1` | OSPFv3 LSAs with a function code in the Vendor Private Use range 8184-8190 must include the Enterprise Code as the first 4 octets following the 20 octets of LSA header (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no OSPFv3 Vendor Private Use LSA (no reference to function codes 8184-8190 in internal/plugins/ospf/), so the requirement's antecedent never holds |
| `RFC7770-2-1` | If a new OSPFv3 LSA Function Code is documented, the documentation must include the valid combinations of the U, S2, and S1 bits for the LSA (§5.2) -- U=1 with S2/S1 = link/area/AS (0x800C/0xA00C/0xC00C), documented in docs/architecture/wire/ospf.md | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the party documenting a new OSPFv3 function code; ze implements the already-defined function code 12 rather than documenting a new one, so the antecedent is false (the U/S2/S1 combinations are recorded at docs/architecture/wire/ospf.md) |
| `RFC7770-5.3-1` | Before any assignments can be made in the reserved RI TLV range 32778-65535, there must be a Standards Track RFC that specifies IANA Considerations covering the range (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the IANA/IETF assignment process; ze registers no TLV type in the reserved 32778-65535 range and cannot author a Standards Track RFC |
| `RFC7770-5.2-2` | OSPFv3 LSA function codes in the experimental range 8176-8183 must not be mentioned by RFCs (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds RFC authors; ze is an implementation, not an RFC, and references no experimental function code 8176-8183 in internal/plugins/ospf/ |
| `RFC7770-2.1-1` | The first Opaque ID / Instance ID (0) should always contain the Router Informational Capabilities TLV and, if advertised, the Router Functional Capabilities TLV (§2.1, §2.2) -- Instance 0 always carries type-1 then type-2 (spec-ospf-ext-3) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.7-2` | If AS-wide flooding scope is chosen, the originating router should also advertise area-scoped LSA(s) into any attached NSSA area(s) (§2.7) -- Ze originates area-scoped RI into attached NSSAs when AS scope is selected | SHOULD | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-5.2-3` | If a new OSPFv3 LSA Function Code is documented, it should also describe how the Link State ID is to be assigned (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-3-1` | For backwards compatibility, previously advertised Router Information TLVs should continue to be advertised in the first instance (0) of the RI LSA (§3) -- Instance 0 retains the type-1 TLV; overflow spills to Instance 1+ | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-1-1` | For future OSPF extensions, the RI LSA advertisement may be used as the sole mechanism for advertisement and discovery (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.2-1` | OSPFv3 routers may advertise multiple RI LSAs per flooding scope (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.4-3` | An OSPF router advertising an RI LSA may include the Router Informational Capabilities TLV (§2.4) -- Ze always includes it | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.4-4` | The Router Informational Capabilities TLV may be followed by optional TLVs that further specify a capability (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-4` | An OSPF router advertising an RI LSA may include the Router Functional Capabilities TLV (§2.6) -- Ze carries it (empty by default) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-5` | The OSPF extensions advertised in the Functional Capabilities TLV may be used by other OSPF routers to dictate protocol operation (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.6-6` | The Router Functional Capabilities TLV may be followed by optional TLVs that further specify a capability (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.7-3` | An OSPF router may advertise different capabilities when both NSSA area-scoped LSA(s) and an AS-scoped LSA are advertised (§2.7) | MAY | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7770-2.7-4` | The originating router may advertise multiple RI LSAs with the same Instance ID as long as the flooding scopes differ (§2.7) | MAY | 2.7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7770-2.3-1`](#rfc7770-2.3-1) When a new Router Information LSA TLV is defined, the specification must explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of an IETF specification that defines a new RI TLV; ze implements TLVs but publishes no such specification, so it plays no role this MUST governs |
| [`RFC7770-2.6-3`](#rfc7770-2.6-3) The specifications for functional capabilities advertised in the Functional Capabilities TLV must describe protocol behavior and address backwards compatibility (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of a functional-capability specification; ze advertises no functional capability and authors no such document |
| [`RFC7770-2.7-1`](#rfc7770-2.7-1) TLV flooding-scope rules must be specified in the accompanying specifications for future Router Information LSA TLVs (§2.7) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of a specification that defines a new RI-TLV; ze selects flooding scope per configuration and does not specify TLV scope rules in a standards document |
| [`RFC7770-5.2-1`](#rfc7770-5.2-1) OSPFv3 LSAs with a function code in the Vendor Private Use range 8184-8190 must include the Enterprise Code as the first 4 octets following the 20 octets of LSA header (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no OSPFv3 Vendor Private Use LSA (no reference to function codes 8184-8190 in internal/plugins/ospf/), so the requirement's antecedent never holds |
| [`RFC7770-2-1`](#rfc7770-2-1) If a new OSPFv3 LSA Function Code is documented, the documentation must include the valid combinations of the U, S2, and S1 bits for the LSA (§5.2) -- U=1 with S2/S1 = link/area/AS (0x800C/0xA00C/0xC00C), documented in docs/architecture/wire/ospf.md | no test | no test carries this requirement id; annotated {not-applicable}: this binds the party documenting a new OSPFv3 function code; ze implements the already-defined function code 12 rather than documenting a new one, so the antecedent is false (the U/S2/S1 combinations are recorded at docs/architecture/wire/ospf.md) |
| [`RFC7770-5.3-1`](#rfc7770-5.3-1) Before any assignments can be made in the reserved RI TLV range 32778-65535, there must be a Standards Track RFC that specifies IANA Considerations covering the range (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the IANA/IETF assignment process; ze registers no TLV type in the reserved 32778-65535 range and cannot author a Standards Track RFC |
| [`RFC7770-5.2-2`](#rfc7770-5.2-2) OSPFv3 LSA function codes in the experimental range 8176-8183 must not be mentioned by RFCs (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: this binds RFC authors; ze is an implementation, not an RFC, and references no experimental function code 8176-8183 in internal/plugins/ospf/ |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7770-2.4-1`](#rfc7770-2.4-1)

If the Router Informational Capabilities TLV is included, it must be the first TLV in the first instance (Instance 0) of the OSPF RI LSA (§2.4) -- Ze emits the type-1 TLV first (spec-ospf-ext-3, `buildRIInstances`)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRITLVType1First`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L124) | unit/verify | unproven |

### [`RFC7770-2.4-2`](#rfc7770-2.4-2)

The Router Informational Capabilities TLV must accurately reflect the OSPF router's capabilities in the scope advertised (§2.4) -- derived from live config (`deriveRICapabilities`)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L57) | unit/verify | unproven |
| positive | [`TestRICapabilityBitsFromState`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L63) | unit/verify | unproven |
| positive | [`TestRICapabilityTEBitFromConfig`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L79) | unit/verify | unproven |

### [`RFC7770-2.6-1`](#rfc7770-2.6-1)

If the Router Functional Capabilities TLV is included, it must be included in the first instance of the LSA (§2.6) -- Ze carries the empty type-2 TLV in Instance 0

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRITLVRegistered`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_registry_test.go#L51) | unit/verify | unproven |

### [`RFC7770-2.6-2`](#rfc7770-2.6-2)

The Router Functional Capabilities TLV must reflect the advertising OSPF router's actual functional capabilities (§2.6) -- carried empty (no functional capability supported)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIFunctionalCapabilitiesEmittedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ri_test.go#L154) | unit/verify | unproven |

### [`RFC7770-2.3-1`](#rfc7770-2.3-1)

When a new Router Information LSA TLV is defined, the specification must explicitly state whether the TLV is applicable to OSPFv2 only, OSPFv3 only, or both (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.3-1, so no unit is bound to it.

### [`RFC7770-2.6-3`](#rfc7770-2.6-3)

The specifications for functional capabilities advertised in the Functional Capabilities TLV must describe protocol behavior and address backwards compatibility (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.6-3, so no unit is bound to it.

### [`RFC7770-2.7-1`](#rfc7770-2.7-1)

TLV flooding-scope rules must be specified in the accompanying specifications for future Router Information LSA TLVs (§2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2.7-1, so no unit is bound to it.

### [`RFC7770-5.2-1`](#rfc7770-5.2-1)

OSPFv3 LSAs with a function code in the Vendor Private Use range 8184-8190 must include the Enterprise Code as the first 4 octets following the 20 octets of LSA header (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.2-1, so no unit is bound to it.

### [`RFC7770-2-1`](#rfc7770-2-1)

If a new OSPFv3 LSA Function Code is documented, the documentation must include the valid combinations of the U, S2, and S1 bits for the LSA (§5.2) -- U=1 with S2/S1 = link/area/AS (0x800C/0xA00C/0xC00C), documented in docs/architecture/wire/ospf.md

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-2-1, so no unit is bound to it.

### [`RFC7770-5.3-1`](#rfc7770-5.3-1)

Before any assignments can be made in the reserved RI TLV range 32778-65535, there must be a Standards Track RFC that specifies IANA Considerations covering the range (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.3-1, so no unit is bound to it.

### [`RFC7770-5.2-2`](#rfc7770-5.2-2)

OSPFv3 LSA function codes in the experimental range 8176-8183 must not be mentioned by RFCs (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7770-5.2-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7770, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7770, so its obligations are stated where they were written.
