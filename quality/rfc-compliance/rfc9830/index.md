# RFC 9830 - BGP Extensions for the Advertisement of Segment Routing (SR) Policies

Partial. Every requirement this repository extracted from RFC 9830, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 64.9% | 63 of 97 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.1% | 4 of 97 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 97 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 97 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 20.9% | 44 of 211 tagged units, 0 escaped and 48 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 97 | of 138 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 12 | of 97 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.4% | 12 of 97 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 97 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 97 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.6% | 18 of 97 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 61 | of 97 gated MUSTs judged | 3 weak, wrong or unimplemented, 22 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 97 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 138 |
| Gated MUST-level | 97 |
| Not applicable, so out of scope | 12 |
| Declared gaps | 18 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 211 |
| Tagged units | 211 |
| Recorded audit verdicts | 61 |
| Discrimination records | 92 |
| Summary | `rfc/short/rfc9830.md` |
| Requirement shard | `rfc/requirements/rfc9830.md` |
| RFC text | `rfc/full/rfc9830.txt` |

## Enrolment

Enrolled: BGP Extensions for SR Policy

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze encodes, parses and splits SAFI 73 NLRIs and originates a type-15 Tunnel Encapsulation TLV containing configured Preference, MPLS or SRv6 Binding SID, Priority, weighted segment lists and names (internal/component/bgp/plugins/nlri/srpolicy). Receive enforcement checks outer and top-level sub-TLV framing, requires exactly one type-15 TLV on actual AFI 1/73 and 2/73 carriers, and ignores inapplicable endpoint/color sub-TLVs ([`internal/component/bgp/reactor/session_tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_tunnel_encap.go)). Repaired receipt fixtures use a valid EBGP AS_PATH and IPv4-format Route Target, preserve assigned SRv6 Binding SID S/I/B bits and Type B V/B bits, and exercise ignored octets separately through receive dispatch, actual cached export and a downstream receive consumer. The codec's Preference readback and opaque byte retention are not SRPM interpretation.

**What the ledger says remains**

Full RFC support is not established. Reserved MPLS binding-label validation, SID-structure length-sum validation and origination without Route Target/NO_ADVERTISE remain separately recorded encoding gaps or defects. The Binding SID unassigned transmit bit is corrected; its regression reproduced the invalid 0x10 Flags octet before the fix. Type-13 SRv6 BSID and type-20 BSID structure origination are absent forms. Receive framing/type enforcement is implemented, but complete SAFI 73 NLRI and nested sub-TLV validation, mandatory target-community validation and the policy-specific default EBGP propagation restriction are not established by these proofs. SRPM candidate-path validation/selection, BSID/Weight/segment interpretation, eligibility and steering remain explicit implementation gaps outside this proof repair; no BGP proof credits the separate configured VPP policy lifecycle. Canonical discrimination recording and independent rejudgment of the repaired tests are still required; historical per-row gap counts are not current verification results.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 63 | one part of the gated population |
| Annotated (including scoped evidence) | 34 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **97** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (63):** [`RFC9830-2.1-1`](#rfc9830-2.1-1), [`RFC9830-2.1-2`](#rfc9830-2.1-2), [`RFC9830-2.1-3`](#rfc9830-2.1-3), [`RFC9830-2.2-1`](#rfc9830-2.2-1), [`RFC9830-2.2-3`](#rfc9830-2.2-3), [`RFC9830-2.3-1`](#rfc9830-2.3-1), [`RFC9830-2.3-3`](#rfc9830-2.3-3), [`RFC9830-2.4-1`](#rfc9830-2.4-1), [`RFC9830-2.4-2`](#rfc9830-2.4-2), [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2), [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3), [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4), [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5), [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6), [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7), [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2), [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4), [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5), [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6), [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7), [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8), [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9), [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10), [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4), [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5), [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6), [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7), [`RFC9830-2.4.4-4`](#rfc9830-2.4.4-4), [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5), [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2), [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3), [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4), [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5), [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6), [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7), [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1), [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2), [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3), [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4), [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5), [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1), [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2), [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3), [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4), [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1), [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2), [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3), [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2), [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3), [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3), [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4), [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5), [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6), [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5), [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6), [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7), [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5), [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6), [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7), [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2), [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7), [`RFC9830-4.2.3-6`](#rfc9830-4.2.3-6), [`RFC9830-5-9`](#rfc9830-5-9)

**Annotated (including scoped evidence) (34):** [`RFC9830-2.2-2`](#rfc9830-2.2-2), [`RFC9830-2.4.2-12`](#rfc9830-2.4.2-12), [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11), [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3), [`RFC9830-2.4.3-9`](#rfc9830-2.4.3-9), [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4), [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2), [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3), [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4), [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5), [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6), [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7), [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8), [`RFC9830-3-3`](#rfc9830-3-3), [`RFC9830-4.1-2`](#rfc9830-4.1-2), [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1), [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3), [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4), [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5), [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6), [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8), [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9), [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1), [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2), [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5), [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1), [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2), [`RFC9830-5-1`](#rfc9830-5-1), [`RFC9830-5-2`](#rfc9830-5-2), [`RFC9830-5-4`](#rfc9830-5-4), [`RFC9830-5-5`](#rfc9830-5-5), [`RFC9830-5-6`](#rfc9830-5-6), [`RFC9830-5-7`](#rfc9830-5-7), [`RFC9830-5-8`](#rfc9830-5-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9830-2.1-1` | The AFI used MUST be IPv4(1) or IPv6(2) (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L92). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L120) |
| `RFC9830-2.1-2` | When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) |
| `RFC9830-2.1-3` | A BGP UPDATE message that carries the MP_REACH_NLRI or MP_UNREACH_NLRI attribute with the SR Policy SAFI MUST also carry the BGP mandatory attributes. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830SRPolicyAnnounceCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_update_attrs_test.go#L47). **positive:** `unit/verify` [`TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_reactor_a_withdraw_attrs_test.go#L38). **positive:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178). **negative:** `unit/verify` [`TestRFC9830SRPolicyIBGPAnnounceKeepsAnEmptyASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_update_attrs_test.go#L90). **negative:** `unit/verify` [`TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_reactor_a_withdraw_attrs_test.go#L39). **negative:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) |
| `RFC9830-2.2-1` | This document specifies the use of the Tunnel Encapsulation Attribute with the SR Policy Tunnel Type and the use of any other Tunnel Type with the SR Policy SAFI MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L201). **negative:** `unit/verify` [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L202) |
| `RFC9830-2.2-2` | A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2) | MUST NOT | 2.2 | **positive:** `unit/verify` [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194). **negative:** no negative test. **{single-polarity}:** buildTunnelEncap assembles every configured sub-TLV into a single Tunnel Type 15 TLV and has no path that appends a second one (internal/component/bgp/plugins/nlri/srpolicy/config.go:365-370), so no input to ze's encoder produces the forbidden encoding for a negative to assert. The receive-side obligation to treat two such TLVs as malformed is the separate RFC9830-2.2-3 |
| `RFC9830-2.2-3` | A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy"; such updates MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L203). **negative:** `unit/verify` [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L204) |
| `RFC9830-2.3-1` | If these sub-TLVs are present, a BGP speaker MUST ignore them (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293). **positive:** `unit/verify` [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L144). **negative:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L304). **negative:** `unit/verify` [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L145) |
| `RFC9830-2.3-3` | Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L301). **positive:** `unit/verify` [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L146). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L309). **negative:** `unit/verify` [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L147) |
| `RFC9830-2.4-1` | For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263). **negative:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L277) |
| `RFC9830-2.4-2` | For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored and MUST NOT considered to be malformed. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L156) |
| `RFC9830-2.4.1-2` | The Preference sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.1) | MUST NOT | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) |
| `RFC9830-2.4.1-3` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L273) |
| `RFC9830-2.4.1-4` | The Flags field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) |
| `RFC9830-2.4.1-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L24). **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L444). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L25). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L265). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L445) |
| `RFC9830-2.4.1-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) |
| `RFC9830-2.4.1-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L26). **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L260). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L446). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L27). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L266). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L447) |
| `RFC9830-2.4.2-2` | The Binding SID sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.2) | MUST NOT | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) |
| `RFC9830-2.4.2-4` | 6 when an SR-MPLS BSID is present, or 2 when no BSID is present. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) |
| `RFC9830-2.4.2-12` | The value MUST be 18 when a SRv6 BSID is present (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze writes no SRv6 BSID into the Binding SID sub-TLV (type 13). buildBindingSIDSubTLV and buildBindingSIDNullSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go) emit only the 6-octet SR-MPLS form and the 2-octet form, and a configured srv6-binding-sid goes out as the SRv6 Binding SID sub-TLV (type 20) instead, so no 18-octet value is ever produced |
| `RFC9830-2.4.2-5` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L28). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L29) |
| `RFC9830-2.4.2-6` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L30). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L31). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) |
| `RFC9830-2.4.2-7` | RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) |
| `RFC9830-2.4.2-8` | RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L32). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L120). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L448). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L33). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L449) |
| `RFC9830-2.4.2-9` | Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) |
| `RFC9830-2.4.2-10` | Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero and MUST be ignored. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L34). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L450). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L35). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L451) |
| `RFC9830-2.4.2-11` | The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2) | MUST NOT | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| `RFC9830-2.4.3-3` | The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315). **negative:** no negative test. **{single-polarity}:** buildSRv6BindingSIDSubTLV always writes the 18-octet form and never appends the SRv6 Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the 26-octet case has no producer and there is no contrasting length to assert. The same 18-versus-26 rule for the Type B segment sub-TLV, where ze DOES produce both, is covered with both polarities under RFC9830-2.4.4.2.2-1 |
| `RFC9830-2.4.3-4` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) |
| `RFC9830-2.4.3-5` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L36). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L124). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L452). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L37). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L453) |
| `RFC9830-2.4.3-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) |
| `RFC9830-2.4.3-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L38). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L128). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L454). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L39). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L455) |
| `RFC9830-2.4.3-9` | The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.3) | MUST NOT | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L318). **negative:** no negative test. **{single-polarity}:** the SRv6 Binding SID sub-TLV ze writes always carries the 16-octet SID and never the Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the forbidden combination has no producer to drive a negative from. The parallel Type B rule, where an endpoint-behavior token without a preceding SRv6 SID IS refused, is covered with both polarities under RFC9830-2.4.4.2.2-4 |
| `RFC9830-2.4.4-4` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) |
| `RFC9830-2.4.4-5` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L40). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L456). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L41). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L457) |
| `RFC9830-2.4.4.1-2` | The Weight sub-TLV is OPTIONAL; it MUST NOT appear more than once inside the Segment List sub-TLV. (§2.4.4.1) | MUST NOT | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) |
| `RFC9830-2.4.4.1-3` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) |
| `RFC9830-2.4.4.1-4` | The Flags field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) |
| `RFC9830-2.4.4.1-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L42). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L458). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L43). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L211). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L459) |
| `RFC9830-2.4.4.1-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) |
| `RFC9830-2.4.4.1-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L44). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L460). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L45). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L212). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L461) |
| `RFC9830-2.4.4.2.1-1` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) |
| `RFC9830-2.4.4.2.1-2` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) |
| `RFC9830-2.4.4.2.1-3` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L46). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L462). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L47). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L213). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L463) |
| `RFC9830-2.4.4.2.1-4` | The S bit MUST be zero upon transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) |
| `RFC9830-2.4.4.2.1-5` | The S bit MUST be zero upon transmission and MUST be ignored upon reception. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L48). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L464). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L49). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L214). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L465) |
| `RFC9830-2.4.4.2.2-1` | The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) |
| `RFC9830-2.4.4.2.2-2` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) |
| `RFC9830-2.4.4.2.2-3` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L50). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L161). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L466). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L51). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L215). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L467) |
| `RFC9830-2.4.4.2.2-4` | The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included. (§2.4.4.2.2) | MUST NOT | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) |
| `RFC9830-2.4.4.2.3-1` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425). **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) |
| `RFC9830-2.4.4.2.3-2` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L52). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L468). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L53). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L216). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L469) |
| `RFC9830-2.4.4.2.3-3` | If B-Flag appears with Segment Type A, it MUST be ignored. (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L157). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L470). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L217). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L471) |
| `RFC9830-2.4.4.2.4-2` | Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) |
| `RFC9830-2.4.4.2.4-3` | Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L54). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L165). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L472). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L55). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L218). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L473) |
| `RFC9830-2.4.4.2.4-4` | The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| `RFC9830-2.4.5-2` | The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5) | MUST NOT | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| `RFC9830-2.4.5-3` | Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-4` | The Flags field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| `RFC9830-2.4.5-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| `RFC9830-2.4.5-8` | Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| `RFC9830-2.4.6-3` | The Priority sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.6) | MUST NOT | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) |
| `RFC9830-2.4.6-4` | Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 2. (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339). **negative:** `unit/verify` [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) |
| `RFC9830-2.4.6-5` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) |
| `RFC9830-2.4.6-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L56). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L169). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L474). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L57). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L219). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L475) |
| `RFC9830-2.4.7-5` | The SR Policy Candidate Path Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.7) | MUST NOT | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) |
| `RFC9830-2.4.7-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) |
| `RFC9830-2.4.7-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L58). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L171). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L476). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L59). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L220). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L477) |
| `RFC9830-2.4.8-5` | The SR Policy Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.8) | MUST NOT | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) |
| `RFC9830-2.4.8-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) |
| `RFC9830-2.4.8-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L60). **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L173). **positive:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L478). **negative:** `unit/verify` [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L61). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L221). **negative:** `unit/verify` [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L479) |
| `RFC9830-3-3` | Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| `RFC9830-4.1-2` | In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| `RFC9830-4.2.1-1` | When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| `RFC9830-4.2.1-2` | The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143). **negative:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) |
| `RFC9830-4.2.1-3` | the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| `RFC9830-4.2.1-4` | * The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| `RFC9830-4.2.1-5` | If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| `RFC9830-4.2.1-6` | The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215). **negative:** no negative test. **{single-polarity}:** parseConfigRoute attaches the Tunnel Encapsulation attribute to every SR Policy route it builds -- buildTunnelEncap always returns at least the 4-octet TLV header, so the len(tunnelEncapValue) > 0 guard never fails (internal/component/bgp/plugins/nlri/srpolicy/config.go:209-225, :365-370) -- leaving no update-without-the-attribute for a negative to observe. The receive-side obligation to call such an update malformed is the separate RFC9830-4.2.1-8, which is annotated as a gap |
| `RFC9830-4.2.1-7` | The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15). (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226). **negative:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) |
| `RFC9830-4.2.1-8` | A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-4.2.1-9` | A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1) | MUST NOT | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| `RFC9830-4.2.2-1` | If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| `RFC9830-4.2.2-2` | The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| `RFC9830-4.2.2-5` | When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| `RFC9830-4.2.3-1` | SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| `RFC9830-4.2.3-2` | By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| `RFC9830-4.2.3-6` | A BGP node MUST NOT alter the SR Policy information carried in the Tunnel Encapsulation Attribute during propagation (§4.2.3) | MUST NOT | 4.2.3 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L180). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L194) |
| `RFC9830-5-1` | A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. This includes the validation of the length of each NLRI and the total length of the MP_REACH_NLRI and MP_UNREACH_NLRI attributes. It also includes the validation of the consistency of the NLRI length with the AFI and the endpoint address as specified in Section 2.1. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| `RFC9830-5-2` | When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| `RFC9830-5-4` | Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-5` | The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| `RFC9830-5-6` | The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-5-7` | In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| `RFC9830-5-8` | An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-9` | A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L336). **negative:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L348) |
| `RFC9830-2.1-5` | It has to be noted that if several CPs of the same SR Policy (Endpoint, Color) are signaled via BGP to a headend, then it is RECOMMENDED that each NLRI use a different distinguisher. (§2.1) | RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-3` | It is RECOMMENDED that the SRv6 Binding SID sub-TLV, as defined in Section 2.4.3, be used when signaling an SRv6 BSID for an SR Policy CP. (§2.4.2) | RECOMMENDED | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-2` | It is RECOMMENDED that the size of the symbolic name for the CP be limited to 255 bytes (§2.4.7) | RECOMMENDED | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-2` | It is RECOMMENDED that the size of the symbolic name for the SR Policy be limited to 255 bytes (§2.4.8) | RECOMMENDED | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-2` | Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.1-1` | Moreover, one or more route targets SHOULD be attached to the advertisement, where each route target identifies one or more intended headends for the advertised SR Policy update. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-3` | When the SR Policy tunnel type includes any sub-TLV that is unrecognized or unsupported, the update SHOULD NOT be considered usable. (§4.2.2) | SHOULD NOT | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-4` | By default, a BGP node receiving an SR Policy NLRI SHOULD NOT remove the Route Target extended community before propagation (§4.2.3) | SHOULD NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-3` | In other cases, where the error in the NLRI encoding results in the inability to process the BGP UPDATE message (e.g., length-related encoding errors), then the router SHOULD handle such malformed NLRIs as "AFI/SAFI disable" when other AFI/SAFIs besides SR Policy are being advertised over the same session. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-10` | An implementation SHOULD log any errors found during the above validation (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.1-4` | In addition, the BGP UPDATE message MAY also contain any of the BGP optional attributes. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-2` | If these sub-TLVs are present, a BGP speaker MUST ignore them and MAY remove them from the Tunnel Encapsulation Attribute during propagation. (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-4` | Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker and MAY be removed from the Tunnel Encapsulation Attribute during propagation. (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.1-1` | The Preference sub-TLV is OPTIONAL (§2.4.1) | OPTIONAL | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-1` | The Binding SID sub-TLV is OPTIONAL (§2.4.2) | OPTIONAL | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-1` | The SRv6 Binding SID sub-TLV is OPTIONAL (§2.4.3) | OPTIONAL | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-2` | More than one SRv6 Binding SID sub-TLV MAY be signaled in the same SR Policy encoding (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-8` | SRv6 Binding SID: Contains a 16-octet SRv6 SID. The value 0 MAY be used when the controller wants to indicate the desired SRv6 Endpoint Behavior, SID Structure, or flags without specifying the BSID. (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-1` | The Segment List sub-TLV is OPTIONAL (§2.4.4) | OPTIONAL | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-2` | The Segment List sub-TLV is OPTIONAL and MAY appear multiple times in the SR Policy encoding. (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-3` | The Segment List sub-TLV contains zero or more Segment sub-TLVs and MAY contain a Weight sub-TLV. (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.1-1` | The Weight sub-TLV is OPTIONAL (§2.4.4.1) | OPTIONAL | 2.4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-1` | The Segment sub-TLVs are OPTIONAL (§2.4.4.2) | OPTIONAL | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-2` | The Segment sub-TLVs are OPTIONAL and MAY appear multiple times in the Segment List sub-TLV. (§2.4.4.2) | MAY | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.1-6` | The receiver MAY override the originator's values for these fields. This would be determined by local policy at the receiver. (§2.4.4.2.1) | MAY | 2.4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.4-1` | The Segment Type sub-TLVs described above MAY contain the SRv6 Endpoint Behavior and SID Structure [RFC8986] encoding as described below (§2.4.4.2.4) | MAY | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-1` | The ENLP sub-TLV is OPTIONAL (§2.4.5) | OPTIONAL | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-9` | The behavior signaled in this sub-TLV MAY be overridden by local configuration by the network operator based on their deployment requirements. (§2.4.5) | MAY | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-1` | An operator MAY set the SR Policy Priority sub-TLV to indicate the order in which the SR policies are recomputed upon topological change. (§2.4.6) | MAY | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-2` | The Priority sub-TLV is OPTIONAL (§2.4.6) | OPTIONAL | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-1` | An operator MAY set the SR Policy Candidate Path Name sub-TLV to attach a symbolic name to the SR Policy CP (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-3` | Implementations MAY choose to truncate long names to 255 bytes when signaling via BGP. (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-4` | The SR Policy Candidate Path Name sub-TLV is OPTIONAL (§2.4.7) | OPTIONAL | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-1` | An operator MAY set the SR Policy Name sub-TLV to associate a symbolic name with the SR Policy (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-3` | Implementations MAY choose to truncate long names to 255 bytes when signaling via BGP. (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-4` | The SR Policy Name sub-TLV is OPTIONAL (§2.4.8) | OPTIONAL | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-1` | The Color Extended Community MAY be carried in any BGP UPDATE message whose AFI/SAFI is 1/1 (IPv4 Unicast), 2/1 (IPv6 Unicast), 1/4 (IPv4 Labeled Unicast), 2/4 (IPv6 Labeled Unicast), 1/128 (VPN-IPv4 Labeled Unicast), 2/128 (VPN-IPv6 Labeled Unicast), or 25/70 (Ethernet VPN, usually known as EVPN). (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-4` | One or more Color Extended Communities MAY be associated with a BGP route update (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-4` | An implementation MAY provide an option for ignoring unsupported sub-TLVs (§4.2.2) | MAY | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-3` | An implementation MAY provide an explicit configuration to override this and enable the propagation of valid SR Policy NLRIs to specific EBGP neighbors where the SR domain comprises multiple ASes within a single service provider domain (see Section 7 for details). (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-5` | An implementation MAY provide support for configuration to filter and/or remove the Route Target extended community before propagation (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9830-2.4.2-12`](#rfc9830-2.4.2-12) The value MUST be 18 when a SRv6 BSID is present (§2.4.2) | {gap}, no test | ze writes no SRv6 BSID into the Binding SID sub-TLV (type 13). buildBindingSIDSubTLV and buildBindingSIDNullSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go) emit only the 6-octet SR-MPLS form and the 2-octet form, and a configured srv6-binding-sid goes out as the SRv6 Binding SID sub-TLV (type 20) instead, so no 18-octet value is ever produced |
| [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11) The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2) | {gap}, no test | the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4) The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | {gap}, no test | the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2) The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3) Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4) The Flags field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5) The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6) RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7) RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8) Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| [`RFC9830-3-3`](#rfc9830-3-3) Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| [`RFC9830-4.1-2`](#rfc9830-4.1-2) In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1) | {gap}, no test | ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1) When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1) | {gap}, no test | no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3) the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1) | {gap}, no test | SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4) * The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1) | {gap}, no test | nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5) If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1) | {gap}, no test | an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8) A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1) | {gap}, no test | no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9) A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1) If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2) The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5) When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1) SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3) | {gap}, no test | the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2) By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3) | {gap}, no test | propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| [`RFC9830-5-1`](#rfc9830-5-1) A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. This includes the validation of the length of each NLRI and the total length of the MP_REACH_NLRI and MP_UNREACH_NLRI attributes. It also includes the validation of the consistency of the NLRI length with the AFI and the endpoint address as specified in Section 2.1. (§5) | {gap}, no test | the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| [`RFC9830-5-2`](#rfc9830-5-2) When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5) | {gap}, no test | no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| [`RFC9830-5-4`](#rfc9830-5-4) Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5) | {gap}, no test | no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| [`RFC9830-5-5`](#rfc9830-5-5) The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5) | {gap}, no test | only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| [`RFC9830-5-6`](#rfc9830-5-6) The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5) | {gap}, no test | the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-5-7`](#rfc9830-5-7) In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5) | {gap}, no test | an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| [`RFC9830-5-8`](#rfc9830-5-8) An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5) | {gap}, no test | the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9830-2.1-1`](#rfc9830-2.1-1)

The AFI used MUST be IPv4(1) or IPv6(2) (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L92) | unit/verify | unproven |

### [`RFC9830-2.1-2`](#rfc9830-2.1-2)

When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NLRIAddressFamilies. Forbidden: a length octet other than 96 for AFI 1 or other than 192 for AFI 2. Red: assert.Equal(byte(96), v4[0]) and assert.Equal(byte(192), v6[0]) on EncodeNLRIHex output, with assert.Len on the 12/24-octet body. Negative: Parse(AFIIPv6, 12-octet body) require.ErrorIs ErrSRPolicyTruncated, and SplitSRPolicy refuses a 100-bit length.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114) | unit/verify | unproven |

### [`RFC9830-2.1-3`](#rfc9830-2.1-3)

A BGP UPDATE message that carries the MP_REACH_NLRI or MP_UNREACH_NLRI attribute with the SR Policy SAFI MUST also carry the BGP mandatory attributes. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves now proven. MP_UNREACH_NLRI: TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes (planBatchAttrs records, unchanged). MP_REACH_NLRI: TestRFC9830SRPolicyAnnounceCarriesMandatoryAttributes finds MP_REACH 00 01 49, ORIGIN 40/00 and the eBGP AS_PATH 40/02 01 0000FDE8 by attribute.AttrFind over the parsed path attribute section, iBGP MP_REACH and ORIGIN likewise; TestRFC9830SRPolicyIBGPAnnounceKeepsAnEmptyASPath forces the iBGP empty-path input (R1(b)) and requires AS_PATH present, flags 40, zero-length value. BuildPlugin is the producer the reactor's config send shares (peer_initial_sync.go). Judge break (overlay, update_build_plugin.go: drop every attribute appendASPath wrote on iBGP) turned the new negative red and the new positive green; the c32 author's probes cover dropped ORIGIN (positive red). Revert records on BuildPlugin for the two new units and the two HEAD tags of TestRFC9830UpdateCarriesMandatoryAttributes. Residual: the HEAD negative (LOCAL_PREF only on iBGP) proves attribute categorisation rather than presence of the mandatory set, supplementary here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830SRPolicyIBGPAnnounceKeepsAnEmptyASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_update_attrs_test.go#L90) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_reactor_a_withdraw_attrs_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830SRPolicyAnnounceCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_update_attrs_test.go#L47) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830SRPolicyWithdrawalCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_reactor_a_withdraw_attrs_test.go#L38) | unit/verify | revert, verified |

### [`RFC9830-2.2-1`](#rfc9830-2.2-1)

This document specifies the use of the Tunnel Encapsulation Attribute with the SR Policy Tunnel Type and the use of any other Tunnel Type with the SR Policy SAFI MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L202) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L201) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC9830-2.2-2`](#rfc9830-2.2-2)

A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194) | unit/verify | unproven |

### [`RFC9830-2.2-3`](#rfc9830-2.2-3)

A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy"; such updates MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L204) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830TunnelTypeReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L203) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC9830-2.3-1`](#rfc9830-2.3-1)

If these sub-TLVs are present, a BGP speaker MUST ignore them (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go::TestRFC9830InapplicableSubTLVsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go::TestRFC9830InapplicableSubTLVsIgnoredOnReceipt#2 moved. Independent semantic rejudgment: RFC9830 Section 2.3: "If these sub-TLVs are present, a BGP speaker MUST ignore them and MAY remove them from the Tunnel Encapsulation attribute during propagation." Current TestRFC9830InapplicableSubTLVsIgnoredOnReceipt isolates endpoint length, duplicate count, and Color on a valid RT/ASN4 SR Policy carrier. Original selective endpoint-length, endpoint-count and Color receive classifiers each fail exact ActionNone and downstream announcement checks while clean policy controls survive. Preference remains 100 and MP_REACH bytes are unchanged through real receive/cache/export. Retention is Ze policy, not an RFC MUST; the MAY-remove clause is not elevated to a preservation mandate. No SRPM consumer is inferred. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L145) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L304) | unit/verify | unproven |
| positive | [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L144) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293) | unit/verify | unproven |

### [`RFC9830-2.3-3`](#rfc9830-2.3-3)

Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012MeaninglessSubTLVIgnoredNotRemoved, internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012MeaninglessSubTLVIgnoredNotRemoved#2 moved. Independent semantic rejudgment: RFC9830 Section 2.3: "Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly specified applicability to the SR Policy SAFI MUST be ignored by the BGP speaker and MAY be removed from the Tunnel Encapsulation attribute during propagation." The current policy-admission unit isolates listed Embedded Label Handling, valid-length UDP, and VXLAN sub-TLVs. Each selective policy-only classifier now fails the exact no-action assertion and route delivery while clean policy controls survive; Preference and MP_REACH remain unchanged on actual receive/cache/export/downstream traversal. Source tunnelTLVLayout grants the policy carrier exception without interpreting these fields, so the common ignore policy is observed rather than inferred from a local Preference getter. MAY removal is not forbidden; no tunnel/header consumer or SRPM behavior is claimed. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L147) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L309) | unit/verify | revert, verified |
| positive | [`TestRFC9830InapplicableSubTLVsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L146) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L301) | unit/verify | revert, verified |

### [`RFC9830-2.4-1`](#rfc9830-2.4-1)

For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012DuplicateSingleInstanceSubTLVs, internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012DuplicateSingleInstanceSubTLVs#2 moved. Judge 2026-09-30, re-read after the RFC9012-13-7 tag lines left the unit (assertions unchanged). TestRFC9012DuplicateSingleInstanceSubTLVs: + with two Preference sub-TLVs Preference() returns the first (100); - the second value standing alone IS returned (200), so the choice is positional.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L277) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4-2`](#rfc9830-2.4-2)

For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored and MUST NOT considered to be malformed. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012DuplicateSingleInstanceSubTLVs, internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012MalformedAttributeIsRejected moved. Judge 2026-09-30, re-read after the RFC9012-13-7 tag lines left the unit (assertions unchanged). TestRFC9012DuplicateSingleInstanceSubTLVs: the duplicate Preference parses without error, both sub-TLVs are enumerated and the attribute round-trips octet-equal, so the ignored instance is not treated as malformed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L156) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269) | unit/verify | unproven |

### [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2)

The Preference sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: two Preference sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPreference), 1) for a config naming preference twice. Negative: a config with no preference emits none (assert.Empty), so the count is real.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a Preference length other than 6. Red: require.Len(value, 6) on a value srpSubs cut at the declared length (a wrong declared length also fails the walk's require.NoError), with the preference in value[2:6]. Negative (internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830PreferenceFlagsAndReservedIgnored): 4- and 8-octet values are not read.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L273) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250) | unit/verify | unproven |

### [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4)

The Flags field MUST be set to zero on transmission (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a non-zero Preference Flags octet on transmission. Red: assert.Equal(byte(0), value[0]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), value), preference 0xDEADBEEF beside it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254) | unit/verify | unproven |

### [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.1: "The Flags field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Preference Flags now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-01-preference-flags changes buf[2] = 1 in buildPreferenceSubTLV and produces only the field diagnostics ["Preference Flags = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L445) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L265) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L444) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255) | unit/verify | unproven |

### [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a non-zero Preference RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), value).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256) | unit/verify | unproven |

### [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.1: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Preference RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-02-preference-reserved changes buf[3] = 1 in buildPreferenceSubTLV and produces only the field diagnostics ["Preference RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L447) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L266) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L446) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L260) | unit/verify | unproven |

### [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2)

The Binding SID sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: two Binding SID sub-TLVs. Red: assert.Len(srpAll(repeated, subTLVBindingSID), 1) for 'binding-sid mpls 24000 binding-sid null'. Negative: none configured, none emitted (assert.Empty).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300) | unit/verify | unproven |

### [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4)

6 when an SR-MPLS BSID is present, or 2 when no BSID is present. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row narrowed to the two clauses ze produces: 6 with an SR-MPLS BSID, 2 with none (the 18-octet SRv6 clause split to RFC9830-2.4.2-12, a {gap}). TestRFC9830BindingSIDSubTLV asserts require.Len(withBSID, 6) from a binding-sid mpls config and require.Len(noBSID, 2) from binding-sid null, through the sub-TLV the config encoder emits; a constant-length or swapped encoder fails one of them. Revert records observed red for both tags (buildBindingSIDSubTLV, buildBindingSIDNullSubTLV). The label-width defect (spec-bgp-sr-policy-rfc-defects D2) will change buildBindingSIDSubTLV and stale the positive record

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) | unit/verify | revert, verified |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280) | unit/verify | revert, verified |

### [`RFC9830-2.4.2-12`](#rfc9830-2.4.2-12)

The value MUST be 18 when a SRv6 BSID is present (§2.4.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-read RFC 9830 in full, including Sections 2.4, 2.4.2 and 2.4.3, the checklist gap and public Support remaining disclosure, and parseConfigRoute/buildTunnelEncap plus all three cited BSID producers. Section 2.4.2 requires value length 18 when an SRv6 BSID is carried in the Binding SID sub-TLV (type 13); it separately recommends type 20 for SRv6 and retains the type-13 form for backward compatibility. buildBindingSIDSubTLV still emits only type 13 length 6 with an MPLS label; buildBindingSIDNullSubTLV emits type 13 length 2; configured srv6-binding-sid is routed by buildTunnelEncap to buildSRv6BindingSIDSubTLV, which emits type 20 length 18, not type 13. No tagged unit claims this requirement. Preserve the existing unimplemented verdict and disclosed absent type-13 SRv6 origination form, rather than crediting type-20 output as enforcement of a different sub-TLV. The current Flags fix merely removes reserved 0x10 from the MPLS type-13 producer; S/I remain zero and it adds neither an SRv6 type-13 producer nor S/I behavior or SRPM interpretation. This is an existing capability gap, not a new defect caused by the Flags fix, and does not authorize feature expansion. No canonical producer-halt record or live named-peer proof was treated as semantic conformance evidence.

No test carries RFC9830-2.4.2-12, so no unit is bound to it.

### [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent TX-only judgment of RFC9830-2.4.2-5 after removing its obsolete gap annotation. RFC9830 Section 2.4.2 states the whole sentence: "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." This row extracts the transmission conjunct only; the receive conjunct remains the separately judged RFC9830-2.4.2-6. Figure 5 assigns S/I to 0x80/0x40, leaving 0x3F unassigned. buildBindingSIDSubTLV now leaves Flags zero instead of writing 0x10, and buildBindingSIDNullSubTLV already leaves Flags zero. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission carries both -5 polarities: bsid[0]&0x3F must be zero beside the exact configured 0xFFFFF label, so an empty/all-zero payload cannot satisfy the pair. The actual pre-fix log job-enum-bsid-unassigned-before-4d050848.log fails precisely the Flags assertion. The repaired tagged test passes in job-enum-acl-bsid-repaired-race-f0d445d5.log, whose separate old-golden package failures are not erased. In semantic-observations-9r9vkava, clean selected baseline passes and rfc9830-tx-22-binding-sid-unassigned-flags restores only buf[2]=0x10; the observed sole tagged-unit failure is Binding SID unassigned Flags = 0x10 on transmission, want 0, with the single-TLV control passing. These executions precede only the new -5 tag comments; the assertion and producer behavior are unchanged, but fresh canonical tagged-unit/claim records remain parent-owned. This proves implemented TX zeroing, not exhaustive per-bit fault enumeration, S/I behavior, SRPM, dataplane, live FRR or full RFC conformance.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L28) | unit/verify | revert, verified |

### [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent bounded new-row rejudgment. RFC9830 Section 2.4.2: "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." RFC9830 Figure 5 assigns only S/I (0x80/0x40), leaving mask 0x3F unassigned. buildBindingSIDSubTLV now leaves the zeroed Flags octet untouched; buildBindingSIDNullSubTLV already does so. The change removes the unconditional 0x10 write without changing label bits, framing, assigned S/I behavior or buffer allocation. New tagged TX assertions require bsid[0]&0x3F==0 beside label 0xFFFFF. The actual pre-fix job-enum-bsid-unassigned-before-4d050848.log fails exactly Binding SID unassigned Flags = 0x10; the repaired tagged unit passes in job-enum-acl-bsid-repaired-race-f0d445d5.log (that package run separately failed old Go goldens, so no full-package success is inferred). The current selected semantic baseline passes, and rfc9830-tx-22-binding-sid-unassigned-flags reinstates only buf[2]=0x10 and fails exactly the new Flags assertion while the single-TLV control passes. Existing tagged TestRFC9830ReceivedFieldsAreIgnoredNotRead contrasts 0x3F and zero with fixed label and exact Preference/round-trip controls. Supplementary actual-carrier rx-bsid-unassigned in semantic-observations-nsr9c7y4 rejects only policy type13 flags&0x3F: both AFIs fail the isolated ignored-octet-14 and combined-dirty receive-action assertion while clean and every other isolated leaf survive. That reactor witness is supplementary, not falsely claimed as a new RFC9830-2.4.2-6 reactor tag. TX zeroing and implemented BGP RX disregard are now observed; no Binding SID/SRPM interpretation is invented. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118) | unit/verify | unproven |

### [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7)

RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: a non-zero Binding SID RESERVED octet. Red: assert.Equal(byte(0), withBSID[1]) and assert.Equal(byte(0), noBSID[1]). Negative: assert.NotEqual(make([]byte, 6), withBSID) beside label 0xFFFFF.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285) | unit/verify | unproven |

### [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8)

RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.2: "MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Binding SID RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-03-binding-sid-reserved changes buf[3] = 1 in buildBindingSIDSubTLV and produces only the field diagnostics ["Binding SID RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L449) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L448) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L120) | unit/verify | unproven |

### [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9)

Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: non-zero TC, S or TTL in the transmitted label stack entry. Red: assert.Equal(byte(0), withBSID[4]&0x0E) (TC), withBSID[4]&0x01 (S), withBSID[5] (TTL). Negative: the 20-bit label reads 0xFFFFF, so the zeroes are field layout, not a small label.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289) | unit/verify | unproven |

### [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10)

Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero and MUST be ignored. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.2: "Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero and MUST be ignored." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Binding SID TC/S/TTL now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-04-binding-sid-tc changes buf[6] |= 2 in buildBindingSIDSubTLV and produces only the field diagnostics ["Binding SID TC and S = 0x02 on transmission, want 0"]. rfc9830-tx-05-binding-sid-s changes buf[6] |= 1 in buildBindingSIDSubTLV and produces only the field diagnostics ["Binding SID TC and S = 0x01 on transmission, want 0"]. rfc9830-tx-06-binding-sid-ttl changes buf[7] = 1 in buildBindingSIDSubTLV and produces only the field diagnostics ["Binding SID TTL = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L451) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L450) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122) | unit/verify | unproven |

### [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11)

The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.2-11, so no unit is bound to it.

### [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3)

The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: a length other than 18 without the Endpoint Behavior and SID Structure. Red: require.Len(value, 18). The 26 case has no producer (buildSRv6BindingSIDSubTLV never appends the structure), carried by the row's single-polarity annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315) | unit/verify | unproven |

### [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: non-zero unassigned Flags bits on transmission (ze sets no assigned flag). Red: assert.Equal(byte(0), value[0]). Negative: assert.NotEqual(make([]byte, 18), value), SID octets non-zero.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322) | unit/verify | unproven |

### [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.3: "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of SRv6 Binding SID unassigned Flags now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-07-srv6-binding-sid-unassigned-flags changes buf[2] = 1 in buildSRv6BindingSIDSubTLV and produces only the field diagnostics ["SRv6 Binding SID Flags = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L453) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L452) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L124) | unit/verify | unproven |

### [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: a non-zero RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual(make([]byte, 18), value).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324) | unit/verify | unproven |

### [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.3: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of SRv6 Binding SID RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-08-srv6-binding-sid-reserved changes buf[3] = 1 in buildSRv6BindingSIDSubTLV and produces only the field diagnostics ["SRv6 Binding SID RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L455) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L454) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L128) | unit/verify | unproven |

### [`RFC9830-2.4.3-9`](#rfc9830-2.4.3-9)

The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L318) | unit/verify | unproven |

### [`RFC9830-2.4.4-4`](#rfc9830-2.4.4-4)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Segment List RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]). Negative: the following sub-TLVs are non-empty (require.NotEmpty(inner)) and the value is not blank.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364) | unit/verify | unproven |

### [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of both MPLS and SRv6 Segment List RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-09-segment-list-reserved changes payload[0] = 1 in buildSegmentListSubTLV and produces only the field diagnostics ["Segment List RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L41) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L457) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L456) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2)

The Weight sub-TLV is OPTIONAL; it MUST NOT appear more than once inside the Segment List sub-TLV. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a second Weight sub-TLV in one Segment List. Red: require.Len(weights, 1) on the srpDirty encoding, and require.Error on parseConfigRoute for 'segment-list weight 1 weight 2 ...', so the config path cannot produce a second one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a Weight length other than 6 (e.g. the 8-octet total). Red: require.Len(weight, 6) on a value cut at the declared length, srpSegSubs require.NoError on the walk, and assert.Len(value, 1+8+8+28) on the whole segment list.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4)

The Flags field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Weight Flags octet. Red: assert.Equal(byte(0), weight[0]). Negative: assert.NotEqual(make([]byte, 6), weight), weight 0xCAFEBABE.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.1: "The Flags field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of both nested Weight Flags now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-10-weight-flags changes wbuf[2] = 1 in buildSegmentListSubTLV and produces only the field diagnostics ["Weight Flags = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L43) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L459) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L211) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L458) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Weight RESERVED octet on transmission. Red: assert.Equal(byte(0), weight[1]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), weight) with weight[2:6] pinned to 0xCAFEBABE, so the zero is written, not a blank buffer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.1: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of both nested Weight RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-11-weight-reserved changes wbuf[3] = 1 in buildSegmentListSubTLV and produces only the field diagnostics ["Weight RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L45) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L461) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L212) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L460) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: a Type A length other than 6 (e.g. the 8-octet total). Red: require.Len(value, 6) on a value srpSegSubs cut at the declared length. Negative: assert.Len(inner, 3), which a length of 8 breaks by swallowing the Type B sub-TLV header that follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: a non-zero Type A RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual(make([]byte, 6), value) with the label pinned to 0xFFFFF.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.2.1: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Type A RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-12-type-a-reserved changes buf[3] = 1 in buildSegmentTypeA and produces only the field diagnostics ["Type A RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L47) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L463) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L213) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L462) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4)

The S bit MUST be zero upon transmission (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: S bit set in the transmitted Type A label stack entry. Red: assert.Equal(byte(0), value[4]&0x01). Negative: assert.Equal(uint32(0xFFFFF), label), so every label bit is set beside the clear S bit and the zero is not a blank low octet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5)

The S bit MUST be zero upon transmission and MUST be ignored upon reception. (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.2.1: "The S bit MUST be zero upon transmission and MUST be ignored upon reception." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Type A S now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-13-type-a-s changes buf[6] |= 1 in buildSegmentTypeA and produces only the field diagnostics ["Type A S bit = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L465) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L214) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L464) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1)

The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: a length other than 26 with the Endpoint Behavior and SID Structure, or other than 18 without it. Red: require.Len(value, 26) on the srpDirty segment and require.Len(bare[0], 18) on the bare type-b segment; both clauses have an assertion and the pair shows the length tracks the content.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: a non-zero Type B RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]) and assert.Equal(byte(0), bare[0][1]), with and without the structure. Negative: assert.NotEqual(make([]byte, 26), value) and value[2] pinned to 0xFC.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.2.2: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Type B RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-14-type-b-reserved changes buf[3] = 1 in buildSegmentTypeB and produces only the field diagnostics ["Type B RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L51) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L467) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L215) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L466) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L161) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4)

The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included. (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: the Endpoint Behavior and SID Structure encoded without an SRv6 SID. Red: require.Error on parseSegmentList and on parseConfigRoute for 'segment endpoint-behavior ...' with no preceding type-b srv6 SID, the only route to a structure ze has. Positive: the structure follows a 16-octet SID (assert.Equal(byte(0xEF), value[17])).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV and TestRFC9830SegmentTypeBSubTLV. Forbidden: any unassigned Segment Flags bit set on transmission. Red: assert.Equal(byte(0), value[0]) on Type A and assert.Equal(byte(0), value[0]&^byte(0x10)) on Type B. Negative: the Type B Flags octet is 0x10 (the assigned B-Flag, bit 3 per Figure 13) and not zero, and 0 without the structure, so the assertion is on which bits are set, not on a blank byte.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.2.3: "The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Type A and Type B unassigned Flags now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-15-type-a-unassigned-flags changes buf[2] = 1 in buildSegmentTypeA and produces only the field diagnostics ["Type A Flags = 0x01 on transmission, want 0"]. rfc9830-tx-16-type-b-unassigned-flags changes buf[2] |= 1 in buildSegmentTypeB and produces only the field diagnostics ["Type B unassigned Flags bits = 0x01 on transmission, want 0", "Type B Flags (B-Flag only) = 11, want 10"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L469) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L216) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L468) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3)

If B-Flag appears with Segment Type A, it MUST be ignored. (§2.4.4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent semantic rejudgment: RFC9830 Section 2.4.4.2.3: "If B-Flag appears with Segment Type A, it MUST be ignored." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. The source-mapped rx-typea-b mutant is bounded to nested Type A value[0]&0x10. Both AFIs fail their isolated Type A B-bit victim and combined dirty input at the receive-action assertion; unchanged B=0 controls and valid Type B B=1/structure controls pass. This is the receive-only obligation: no missing transmit conjunct is invented, and assigned Type B B is not rejected. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L471) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L217) | unit/verify | unproven |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L470) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L157) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2)

Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: non-zero Reserved octets in the Endpoint Behavior and SID Structure on transmission. Red: assert.Equal([]byte{0x00, 0x00}, value[20:22]). Negative: endpoint behavior 0xFFFF at value[18:20] and structure 32/16/16/64 at value[22:26], so the zeros are written at the right offset.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3)

Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.4.2.4: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of both SRv6 endpoint-behavior Reserved octets in Type B and top-level SRv6 Binding SID now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-17-endpoint-behavior-reserved-first-octet changes if seg.hasEndpointBehavior {
		buf[22] = 1
	} in buildSegmentTypeB and produces only the field diagnostics ["SRv6 Endpoint Behavior Reserved = 0100, want 0000"]. rfc9830-tx-18-endpoint-behavior-reserved-second-octet changes if seg.hasEndpointBehavior {
		buf[23] = 1
	} in buildSegmentTypeB and produces only the field diagnostics ["SRv6 Endpoint Behavior Reserved = 0001, want 0000"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L473) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L218) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L472) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L165) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4)

The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.4.2.4-4, so no unit is bound to it.

### [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2)

The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-2, so no unit is bound to it.

### [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3)

Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-3, so no unit is bound to it.

### [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4)

The Flags field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-4, so no unit is bound to it.

### [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-5, so no unit is bound to it.

### [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-6, so no unit is bound to it.

### [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-7, so no unit is bound to it.

### [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8)

Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-8, so no unit is bound to it.

### [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3)

The Priority sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV. Forbidden: two Priority sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPriority), 1) for a config naming priority twice. Negative (control): a config with no priority emits none (assert.Empty), so the instance is counted rather than always written; a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348) | unit/verify | unproven |

### [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4)

Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 2. (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV and TestRFC9830PriorityLengthIsValueLength. Forbidden: a Priority length other than 2 (e.g. the 4-octet total). Red: require.Len(value, 2) on a value cut at the declared length, and assert.Equal([]byte{15, 2, 255, 0}, only[4:]) pinning the length octet itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339) | unit/verify | unproven |

### [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV. Forbidden: a non-zero Priority RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual([]byte{0,0}, value) with value[0] pinned to 255, so the zero is written and the fields are not transposed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343) | unit/verify | unproven |

### [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.6: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Priority RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-19-priority-reserved changes buf[3] = 1 in buildPrioritySubTLV and produces only the field diagnostics ["Priority RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L475) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L474) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L169) | unit/verify | unproven |

### [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5)

The SR Policy Candidate Path Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: two Candidate Path Name sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVCandidatePathNam), 1) for a config naming the candidate path twice. Negative (control): an unnamed candidate path emits none (assert.Empty); a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506) | unit/verify | unproven |

### [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: a non-zero Candidate Path Name RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]) in the loop case for 'primary'. Negative: require.Len(value, 1+len(want)) and assert.Equal(want, string(value[1:])), so the zero is a separate written octet ahead of the whole name.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495) | unit/verify | unproven |

### [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.7: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Candidate Path Name RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-20-candidate-path-name-reserved changes if stype == subTLVCandidatePathNam {
		buf[3] = 1
	} in buildNameSubTLV and produces only the field diagnostics ["Candidate Path Name RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L477) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L476) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L171) | unit/verify | unproven |

### [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5)

The SR Policy Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: two SR Policy Name sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPolicyName), 1) for a config naming the policy twice. Negative (control): an unnamed policy emits none (assert.Empty); a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507) | unit/verify | unproven |

### [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: a non-zero SR Policy Name RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]) in the loop case for 'alpha'. Negative: assert.NotEqual(make([]byte, 6), ...) and assert.Equal(want, string(value[1:])), so the zero is written ahead of the whole name.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496) | unit/verify | unproven |

### [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.8)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt, internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9830ReservedFieldsIgnoredOnReceipt#2 moved. Independent TX-completion rejudgment. RFC9830 Section 2.4.8: "This field MUST be set to zero on transmission and MUST be ignored on receipt." TestRFC9830ReservedFieldsIgnoredOnReceipt uses valid route-target/ASN4 carriers for both SAFI-73 AFIs, exact no-action and one-attribute assertions before real cache/export/downstream announcement, and nonzero meaningful neighbors. Renewed receive-source-mapped-assessment.json maps actual teSRPolicyValue offsets to immutable mutant/log victim names; its corrected receive victims, not original classifier false flags, are credited. Selective overvalidation of Policy Name RESERVED now fails the intended receive-action assertion while clean and structurally valid B=1 controls survive. The corrected 14-case aggregate is 34 isolated victim leaves plus 28 all-ignored-fields failures and 56 clean controls, not 62 isolated field tests. The missing transmission counterfactual is now executed: rfc9830-tx-21-policy-name-reserved changes if stype == subTLVPolicyName {
		buf[3] = 1
	} in buildNameSubTLV and produces only the field diagnostics ["Policy Name RESERVED = 0x01 on transmission, want 0"]. Current TestRFC9830FieldsToIgnoreAreZeroOnTransmission traverses config parsing/buildTunnelEncap, pins zero/mask fields beside the configured Preference, labels, SID, Weight, endpoint behavior/structure, Priority and names, and keeps the positive neighbors intact. Only that tagged unit fails under each TX fault; TestRFC9830SinglePolicyTLVPerAttribute passes. The selected baseline unit passes without overlays. Both clauses are therefore independently discriminated, replacing the prior TX-evidence hold; the earlier RX mapping limitations are unchanged. This is executed host semantic evidence, not a canonical native discrimination renewal. Native recording/stamping and the five FRR scenarios remain parent-owned and are not declared verified here. No SRPM, dataplane, absent encoding form, full-RFC, or exhaustive-per-bit claim follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L61) | unit/verify | revert, verified |
| negative | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L479) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC9830FieldsToIgnoreAreZeroOnTransmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC9830ReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L478) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L173) | unit/verify | unproven |

### [`RFC9830-3-3`](#rfc9830-3-3)

Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-3-3, so no unit is bound to it.

### [`RFC9830-4.1-2`](#rfc9830-4.1-2)

In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.1-2, so no unit is bound to it.

### [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1)

When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-1, so no unit is bound to it.

### [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2)

The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field (§4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NLRICarriesAllThreeFields. WEAK: the sentence is a receive-validation rule (Section 4.2.1 'When a BGP speaker receives'). The tagged unit proves the encoder writes Distinguisher, Color and Endpoint at their offsets and that Parse (require.ErrorIs ErrSRPolicyTruncated) and parseConfigRoute (errSRPolicyMissingFields) refuse a body or config missing one, but the session receive path splits with SplitSRPolicy (split.go), which accepts any byte-aligned length, so a received body lacking the Endpoint is carried; no tagged unit drives a received UPDATE, and no assertion goes red on a receiver that accepts it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143) | unit/verify | unproven |

### [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3)

the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-3, so no unit is bound to it.

### [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4)

* The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-4, so no unit is bound to it.

### [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5)

If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-5, so no unit is bound to it.

### [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6)

The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215) | unit/verify | unproven |

### [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7)

The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15). (§4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830TunnelTypeIsSRPolicy. WEAK: the sentence is a receive-validation rule. The tagged unit proves only that ze's own encoder wraps the sub-TLVs in a Tunnel Type 15 TLV (assert.Equal([]byte{0x00,0x0F}, value[0:2])); nothing checks a received SR Policy update for a type-15 TLV or for the attribute at all (attribute 23 has no validator, per the RFC9830-4.2.1-8 gap). The negative is conditional: when ParseTunnelEncap errors on the bare payload it asserts nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) | unit/verify | unproven |
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226) | unit/verify | unproven |

### [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8)

A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-8, so no unit is bound to it.

### [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9)

A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-9, so no unit is bound to it.

### [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1)

If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-1, so no unit is bound to it.

### [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2)

The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-2, so no unit is bound to it.

### [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5)

When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-5, so no unit is bound to it.

### [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1)

SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.3-1, so no unit is bound to it.

### [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2)

By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.3-2, so no unit is bound to it.

### [`RFC9830-4.2.3-6`](#rfc9830-4.2.3-6)

A BGP node MUST NOT alter the SR Policy information carried in the Tunnel Encapsulation Attribute during propagation (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L180) | unit/verify | unproven |

### [`RFC9830-5-1`](#rfc9830-5-1)

A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. This includes the validation of the length of each NLRI and the total length of the MP_REACH_NLRI and MP_UNREACH_NLRI attributes. It also includes the validation of the consistency of the NLRI length with the AFI and the endpoint address as specified in Section 2.1. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-1, so no unit is bound to it.

### [`RFC9830-5-2`](#rfc9830-5-2)

When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-2, so no unit is bound to it.

### [`RFC9830-5-4`](#rfc9830-5-4)

Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-4, so no unit is bound to it.

### [`RFC9830-5-5`](#rfc9830-5-5)

The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-5, so no unit is bound to it.

### [`RFC9830-5-6`](#rfc9830-5-6)

The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-6, so no unit is bound to it.

### [`RFC9830-5-7`](#rfc9830-5-7)

In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-7, so no unit is bound to it.

### [`RFC9830-5-8`](#rfc9830-5-8)

An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-8, so no unit is bound to it.

### [`RFC9830-5-9`](#rfc9830-5-9)

A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830NoSemanticVerification. ParseTunnelEncap is the only receive parser of attribute 23 (wire.go knownAttrParsers) and Preference the only field ze consumes. Forbidden: refusing, or not using, an SR Policy update because a field value is semantically wrong. Red: require.NoError(ParseTunnelEncap), require.NoError(SubTLVs) with assert.Len(stlvs, 5), assert.Equal(uint32(4242), pref) and the byte-identical round trip, over weight 0, reserved label 3, a SID structure totalling 255, ENLP 99 and an empty segment list. Negative: framing errors are still refused (require.Error twice), so the skip is limited to field semantics. A verification with no observable effect cannot be asserted; every effect it could have is.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L348) | unit/verify | unproven |
| positive | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L336) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9830.txt |
| Source fingerprint | 35ee1db5cdbf6fc4 |
| Record | rfc/extraction/rfc9830.json |
| Mapped sentences | 71 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `2.3` | not stated | 2 | walked | not stated |
| `2.4` | not stated | 1 | walked | not stated |
| `2.4.1` | not stated | 4 | walked | not stated |
| `2.4.2` | not stated | 6 | walked | not stated |
| `2.4.3` | not stated | 4 | walked | not stated |
| `2.4.4` | not stated | 2 | walked | not stated |
| `2.4.4.1` | not stated | 4 | walked | not stated |
| `2.4.4.2` | not stated | 0 | walked | not stated |
| `2.4.4.2.1` | not stated | 3 | walked | not stated |
| `2.4.4.2.2` | not stated | 3 | walked | not stated |
| `2.4.4.2.3` | not stated | 2 | walked | not stated |
| `2.4.4.2.4` | not stated | 2 | walked | not stated |
| `2.4.5` | not stated | 6 | walked | not stated |
| `2.4.6` | not stated | 3 | walked | not stated |
| `2.4.7` | not stated | 3 | walked | not stated |
| `2.4.8` | not stated | 3 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 6 | walked | not stated |
| `4.2.2` | not stated | 3 | walked | not stated |
| `4.2.3` | not stated | 3 | walked | not stated |
| `5` | not stated | 8 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `6.7` | not stated | 0 | walked | not stated |
| `6.8` | not stated | 0 | walked | not stated |
| `6.9` | not stated | 0 | walked | not stated |
| `6.10` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Revised BSD License for extracted Code Components; it states no protocol obligation. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `2.4.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. The Segment List sub-TLV length field width is the diagram's, not an obligation on a speaker. | A 2-octet length is thus required. |
| `2.4.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introductory description of what the ENLP sub-TLV indicates; "must" is lowercase and is not an RFC 2119 keyword. The normative ENLP obligations are the sites 2.4.5:2 through 2.4.5:6. | The Explicit NULL Label Policy (ENLP) sub-TLV is used to indicate whether an Explicit NULL Label [RFC3032] must be pushed on an unlabeled IP packet before any other labels. |
| `2.4.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. | A 2-octet length is thus required. |
| `2.4.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. | A 2-octet length is thus required. |

## Superseded

No document obsoletes RFC 9830, so its obligations are stated where they were written.
