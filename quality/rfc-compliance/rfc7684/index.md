# RFC 7684 - OSPFv2 Prefix/Link Attribute Advertisement

Experimental. Every requirement this repository extracted from RFC 7684, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 5 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 19 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7684.md` |
| Requirement shard | `rfc/requirements/rfc7684.md` |
| RFC text | `rfc/full/rfc7684.txt` |

## Enrolment

Enrolled: OSPFv2 Prefix/Link Attribute Advertisement (Extended Prefix and Extended Link Opaque LSAs): eight MUST-level requirements, all met (five with positive+negative test tags, three not-applicable). 2.1-1 (clear the N-flag for non-host prefixes, keep it for /32 hosts), 2.1-2 (an inter-area host /32 keeps the N-flag), 2.1-3 (route type selects the LSA flooding scope), 3.1-1 (one Extended Link TLV per link; on receipt use the first and count extras), and 5-1 (bounds-checked TLV parsing returns an error, never panics, and counts and drops overruns) carry positive+negative tags in internal/plugins/ospf and internal/plugins/ospf/packet. 4-1 (carry backward-compatible empty containers) is {not-applicable}: it directs downstream application specifications (for example RFC 8665) rather than a ze wire behavior. 6.5-1 and 6.5-2 are {not-applicable}: they are IANA registry allocation and documentation policy, not ze code behavior.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Extended Prefix and Extended Link LSA bodies and malformed TLV handling.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC7684-2.1-1`](#rfc7684-2.1-1), [`RFC7684-2.1-2`](#rfc7684-2.1-2), [`RFC7684-2.1-3`](#rfc7684-2.1-3), [`RFC7684-3.1-1`](#rfc7684-3.1-1), [`RFC7684-5-1`](#rfc7684-5-1)

**Annotated instead of tested (3):** [`RFC7684-4-1`](#rfc7684-4-1), [`RFC7684-6.5-1`](#rfc7684-6.5-1), [`RFC7684-6.5-2`](#rfc7684-6.5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7684-2.1-1` | If the N-Flag is set and the prefix length is not a host prefix, the flag MUST be ignored (§2.1) -- `extNormalizeFlags` clears it on receive | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L77). **negative:** `unit/verify` [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L66) |
| `RFC7684-2.1-2` | Preserve the N-Flag when the Extended Prefix Opaque LSA is propagated between areas (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L121). **negative:** `unit/verify` [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L150) |
| `RFC7684-2.1-3` | The Extended Prefix Opaque LSA flooding scope (LS Type 10/11) MUST satisfy the application-specific scope requirements for all prefixes in the LSA (§2.1) -- `extPrefixScope`: area for intra/inter, AS for external | MUST | 2.1 | **positive:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L167). **negative:** `unit/verify` [`TestExtPrefixScopeSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L170) |
| `RFC7684-3.1-1` | Only one OSPFv2 Extended Link TLV SHALL be advertised in each OSPFv2 Extended Link Opaque LSA (§3.1) -- origination emits one per LSA; decode uses the first and logs extras | SHALL | 3.1 | **positive:** `unit/verify` [`TestExtLinkMirrorsRouterLSALink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_link_origin_test.go#L46). **negative:** `unit/verify` [`TestExtLinkSingleTLVEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L74) |
| `RFC7684-4-1` | Future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this directive binds downstream application specifications (e.g. RFC 8665 segment routing) that define the sub-TLVs these LSAs carry, not a ze wire behavior; ze originates backward-compatible empty Extended Prefix/Link containers per RFC 5250 (internal/plugins/ospf/ext_prefix.go:72-75, internal/plugins/ospf/ext_link.go:56-59) |
| `RFC7684-5-1` | Detect malformed TLV and sub-TLV permutations so they cannot crash the router or routing process (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target | MUST | 5 | **positive:** `unit/verify` [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L25). **positive:** `unit/verify` [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_prefix_test.go#L28). **negative:** `unit/verify` [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L111). **negative:** `unit/verify` [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L91). **negative:** `unit/verify` [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L163) |
| `RFC7684-6.5-1` | Experimental Use types (32768-33023) MUST NOT be mentioned by RFCs -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5) | MUST NOT | 6.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA registry documentation policy binding RFC authors, not a ze code or wire behavior; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range |
| `RFC7684-6.5-2` | Before any assignment in the 33024-65535 range there MUST be an IETF specification specifying IANA considerations covering that range -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5) | MUST | 6.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA allocation policy binding IETF specifications, not a ze code or wire behavior; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range |
| `RFC7684-2-1` | If multiple Extended Prefix Opaque LSAs include the same prefix, use the attributes from the LSA with the lowest Opaque ID (§2) -- `extReceiver.applyPrefix` | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-4` | An ABR generating an Extended Prefix TLV for an inter-area prefix locally connected/attached in another connected area SHOULD set the A-Flag (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-5` | Duplicate Extended Prefix TLV for the same prefix in the same LSA: use only the first instance and log the situation as an error (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-2` | Duplicate Extended Link TLV in the same LSA: use only the first instance and log the situation as an error (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-5-2` | Reception of malformed LSAs SHOULD be counted and/or logged for further analysis (§5) -- `ze_ospf_ext_malformed_total` | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-6` | Routers advertising Extended Prefix TLVs in different Extended Prefix Opaque LSAs re-originate these LSAs in ascending Opaque ID order to minimize disruption (§2.1) -- stable ascending Opaque-ID allocator | RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-3` | Routers advertising Extended Link TLVs in different Extended Link Opaque LSAs re-originate these LSAs in ascending Opaque ID order to minimize disruption (§3.1) | RECOMMENDED | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-7` | Multiple OSPFv2 Extended Prefix TLVs MAY be advertised in each OSPFv2 Extended Prefix Opaque LSA (§2.1) -- the decoder accepts many; origination uses one LSA per prefix | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-8` | The advertising router MAY choose not to set the N-Flag even when the host-loopback conditions are met (§2.1) -- Ze sets N on host /32 stub prefixes | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-2.1-9` | Duplicate Extended Prefix TLV for the same prefix across different LSAs from the same router (smallest Opaque ID used): the situation may be logged as a warning (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7684-3.1-4` | Duplicate Extended Link TLV across different LSAs from the same router (smallest Opaque ID used): the situation may be logged as a warning (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7684-4-1`](#rfc7684-4-1) Future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665 | no test | no test carries this requirement id; annotated {not-applicable}: this directive binds downstream application specifications (e.g. RFC 8665 segment routing) that define the sub-TLVs these LSAs carry, not a ze wire behavior; ze originates backward-compatible empty Extended Prefix/Link containers per RFC 5250 (internal/plugins/ospf/ext_prefix.go:72-75, internal/plugins/ospf/ext_link.go:56-59) |
| [`RFC7684-6.5-1`](#rfc7684-6.5-1) Experimental Use types (32768-33023) MUST NOT be mentioned by RFCs -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5) | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA registry documentation policy binding RFC authors, not a ze code or wire behavior; ze defines and emits no TLV or sub-TLV type in the 32768-33023 Experimental Use range |
| [`RFC7684-6.5-2`](#rfc7684-6.5-2) Before any assignment in the 33024-65535 range there MUST be an IETF specification specifying IANA considerations covering that range -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5) | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA allocation policy binding IETF specifications, not a ze code or wire behavior; ze assigns and emits no TLV or sub-TLV type in the 33024-65535 range |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7684-2.1-1`](#rfc7684-2.1-1)

If the N-Flag is set and the prefix length is not a host prefix, the flag MUST be ignored (§2.1) -- `extNormalizeFlags` clears it on receive

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L66) | unit/verify | unproven |
| positive | [`TestExtPrefixNFlagIgnoredNonHost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L77) | unit/verify | unproven |

### [`RFC7684-2.1-2`](#rfc7684-2.1-2)

Preserve the N-Flag when the Extended Prefix Opaque LSA is propagated between areas (§2.1) -- an ABR preserves N on the inter-area advertisement of a host prefix

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixNFlagNotSetNonHostInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L150) | unit/verify | unproven |
| positive | [`TestExtPrefixNFlagPreservedInterArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_origin_test.go#L121) | unit/verify | unproven |

### [`RFC7684-2.1-3`](#rfc7684-2.1-3)

The Extended Prefix Opaque LSA flooding scope (LS Type 10/11) MUST satisfy the application-specific scope requirements for all prefixes in the LSA (§2.1) -- `extPrefixScope`: area for intra/inter, AS for external

Audit verdict: not audited: no reader has judged these tests

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

Future OSPFv2 applications utilizing these extensions MUST address backward compatibility of the corresponding functionality (§4) -- containers only; empty-container LSAs are conformant, sub-TLV values left to RFC 8665

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-4-1, so no unit is bound to it.

### [`RFC7684-5-1`](#rfc7684-5-1)

Detect malformed TLV and sub-TLV permutations so they cannot crash the router or routing process (§5) -- bound-checked decode returns an error, never panics; extended in the packet fuzz target

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtPrefixMalformedCounted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ext_prefix_recv_test.go#L163) | unit/verify | unproven |
| negative | [`FuzzOSPFExtLinkBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L111) | unit/verify | unproven |
| negative | [`FuzzOSPFExtPrefixBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/fuzz_test.go#L91) | unit/verify | unproven |
| positive | [`TestExtLinkTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_link_test.go#L25) | unit/verify | unproven |
| positive | [`TestExtPrefixTLVRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/ext_prefix_test.go#L28) | unit/verify | unproven |

### [`RFC7684-6.5-1`](#rfc7684-6.5-1)

Experimental Use types (32768-33023) MUST NOT be mentioned by RFCs -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-6.5-1, so no unit is bound to it.

### [`RFC7684-6.5-2`](#rfc7684-6.5-2)

Before any assignment in the 33024-65535 range there MUST be an IETF specification specifying IANA considerations covering that range -- Extended Prefix Opaque LSA TLVs (§6.1), Extended Prefix TLV Sub-TLVs (§6.2), Extended Link Opaque LSA TLVs (§6.4), Extended Link TLV Sub-TLVs (§6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7684-6.5-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7684, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7684, so its obligations are stated where they were written.
