# RFC 9514 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing over IPv6 (SRv6)

Partial. Every requirement this repository extracted from RFC 9514, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 13 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9514.md` |
| Requirement shard | `rfc/requirements/rfc9514.md` |
| RFC text | `rfc/full/rfc9514.txt` |

## Enrolment

Enrolled: BGP-LS SRv6 extensions: 13 gap (all origination/encode MUSTs plus one receive-side SID-Structure sum validation; decode-only plugin, TLVs 1038/1162 unimplemented)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

SRv6 End.X SID, Endpoint Behavior, BGP PeerNode SID, and SID Structure TLVs decode as part of BGP-LS SRv6 TLV coverage.

**What the ledger says remains**

Thirteen origination/encode MUSTs unmet (decode-only plugin, no config surface); the SRv6 Capabilities (TLV 1038) and SRv6 Locator (TLV 1162) TLVs are not implemented at all, and the SID Structure sum-at-most-128 validation is absent on receipt (see [`rfc/short/rfc9514.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9514.md)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (13):** [`RFC9514-3.1-1`](#rfc9514-3.1-1), [`RFC9514-3.1-2`](#rfc9514-3.1-2), [`RFC9514-4.1-1`](#rfc9514-4.1-1), [`RFC9514-4.2-1`](#rfc9514-4.2-1), [`RFC9514-5.1-1`](#rfc9514-5.1-1), [`RFC9514-6-1`](#rfc9514-6-1), [`RFC9514-7.1-1`](#rfc9514-7.1-1), [`RFC9514-7.1-2`](#rfc9514-7.1-2), [`RFC9514-7.1-3`](#rfc9514-7.1-3), [`RFC9514-7.2-1`](#rfc9514-7.2-1), [`RFC9514-7.2-2`](#rfc9514-7.2-2), [`RFC9514-7.2-3`](#rfc9514-7.2-3), [`RFC9514-8-1`](#rfc9514-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9514-3.1-1` | A single instance of the SRv6 Capabilities TLV (1038) MUST be included in the BGP-LS Attribute for each SRv6-capable node (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** origination/inclusion obligation; ze originates no BGP-LS and does not even decode TLV 1038, which is unregistered (internal/component/bgp/plugins/nlri/ls/register_attr.go:54-67, plugin.go:70-71) |
| `RFC9514-3.1-2` | Reserved field in SRv6 Capabilities TLV MUST be set to 0 when originated (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** TLV 1038 is neither decoded nor encoded (no struct, unregistered), so this transmit reserved-zero MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go:54-67) |
| `RFC9514-4.1-1` | Reserved field in SRv6 End.X SID TLV MUST be set to 0 when originated (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LsSRv6EndXSID encoder hardcodes the reserved octet to 0 but ze originates no BGP-LS End.X SID TLV (internal/component/bgp/plugins/nlri/ls/attr_link.go:623, plugin.go:70-71) |
| `RFC9514-4.2-1` | Reserved field in SRv6 LAN End.X SID TLV MUST be set to 0 when originated (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LAN variants share the same LsSRv6EndXSID.WriteTo that hardcodes reserved to 0, but no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_link.go:623, register_attr.go:56-57) |
| `RFC9514-5.1-1` | Reserved field in SRv6 Locator TLV MUST be set to 0 when originated (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither decodes nor encodes TLV 1162 (SRv6 Locator); it is unregistered with no struct, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| `RFC9514-6-1` | SRv6 SID Descriptors MUST contain a single SRv6 SID Information TLV (518) (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the decoder recognizes TLV 518 but appends without enforcing single-cardinality on receipt, and no production path originates the SRv6 SID NLRI descriptor (internal/component/bgp/plugins/nlri/ls/types.go:434-437, types_srv6.go:58, plugin.go:70-71) |
| `RFC9514-7.1-1` | The SRv6 Endpoint Behavior TLV (1250) MUST be included in the BGP-LS Attribute associated with the SRv6 SID NLRI (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LsSRv6EndpointBehavior decoder and encoder exist but no production path originates the SRv6 SID NLRI attribute, so it is never included (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:33, :52, plugin.go:70-71) |
| `RFC9514-7.1-2` | Undefined flags in SRv6 Endpoint Behavior TLV MUST be set to 0 when originating (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder writes the Flags octet verbatim (no masking) and no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:37, plugin.go:70-71) |
| `RFC9514-7.1-3` | Algorithm value in SRv6 Endpoint Behavior TLV MUST be 0 unless an algorithm is associated locally with the SRv6 Locator (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this origination-time semantic choice is unimplemented; ze originates no such TLV and the encoder writes Algorithm verbatim with no locator-association logic (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:38, plugin.go:70-71) |
| `RFC9514-7.2-1` | SRv6 BGP PeerNode SID TLV (1251) MUST be included along with SRv6 SIDs associated with BGP PeerNode or PeerSet functionality (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LsSRv6BGPPeerNodeSID decoder and encoder exist but no EPE origination path instantiates or includes the TLV from a live session (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:78, :102, plugin.go:70-71) |
| `RFC9514-7.2-2` | Reserved bits (3-7) in SRv6 BGP PeerNode SID flags MUST be set to 0 when originating (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder writes the Flags byte verbatim without masking bits 3-7 and no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:81, plugin.go:70-71) |
| `RFC9514-7.2-3` | Reserved field in SRv6 BGP PeerNode SID TLV MUST be set to 0 when originated (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes both reserved octets to 0 but ze originates no SRv6 BGP PeerNode SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:83-84, plugin.go:70-71) |
| `RFC9514-8-1` | SRv6 SID Structure sum (LB Length + LN Length + Fun. Length + Arg. Length) MUST be less than or equal to 128 (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the SID Structure decoder reads the four length octets but never checks their sum is at most 128, accepting oversized values on receipt, and ze originates no SID Structure TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:151-160, attr_link.go:702-708) |
| `RFC9514-11-1` | Isolation of BGP-LS peering sessions is RECOMMENDED to ensure SRv6 topology information is not advertised to external BGP peers outside the SR domain (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9514-6-2` | SRv6 SID Descriptors MAY contain the Multi-Topology Identifier TLV (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9514-7.2-4` | A PeerSet SID MAY be assigned to one or more End.X SIDs (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9514-3.1-1`](#rfc9514-3.1-1) A single instance of the SRv6 Capabilities TLV (1038) MUST be included in the BGP-LS Attribute for each SRv6-capable node (§3.1) | {gap}, no test | origination/inclusion obligation; ze originates no BGP-LS and does not even decode TLV 1038, which is unregistered (internal/component/bgp/plugins/nlri/ls/register_attr.go:54-67, plugin.go:70-71) |
| [`RFC9514-3.1-2`](#rfc9514-3.1-2) Reserved field in SRv6 Capabilities TLV MUST be set to 0 when originated (§3.1) | {gap}, no test | TLV 1038 is neither decoded nor encoded (no struct, unregistered), so this transmit reserved-zero MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go:54-67) |
| [`RFC9514-4.1-1`](#rfc9514-4.1-1) Reserved field in SRv6 End.X SID TLV MUST be set to 0 when originated (§4.1) | {gap}, no test | the LsSRv6EndXSID encoder hardcodes the reserved octet to 0 but ze originates no BGP-LS End.X SID TLV (internal/component/bgp/plugins/nlri/ls/attr_link.go:623, plugin.go:70-71) |
| [`RFC9514-4.2-1`](#rfc9514-4.2-1) Reserved field in SRv6 LAN End.X SID TLV MUST be set to 0 when originated (§4.2) | {gap}, no test | the LAN variants share the same LsSRv6EndXSID.WriteTo that hardcodes reserved to 0, but no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_link.go:623, register_attr.go:56-57) |
| [`RFC9514-5.1-1`](#rfc9514-5.1-1) Reserved field in SRv6 Locator TLV MUST be set to 0 when originated (§5.1) | {gap}, no test | ze neither decodes nor encodes TLV 1162 (SRv6 Locator); it is unregistered with no struct, so this transmit MUST is entirely unimplemented (internal/component/bgp/plugins/nlri/ls/register_attr.go) |
| [`RFC9514-6-1`](#rfc9514-6-1) SRv6 SID Descriptors MUST contain a single SRv6 SID Information TLV (518) (§6) | {gap}, no test | the decoder recognizes TLV 518 but appends without enforcing single-cardinality on receipt, and no production path originates the SRv6 SID NLRI descriptor (internal/component/bgp/plugins/nlri/ls/types.go:434-437, types_srv6.go:58, plugin.go:70-71) |
| [`RFC9514-7.1-1`](#rfc9514-7.1-1) The SRv6 Endpoint Behavior TLV (1250) MUST be included in the BGP-LS Attribute associated with the SRv6 SID NLRI (§7.1) | {gap}, no test | the LsSRv6EndpointBehavior decoder and encoder exist but no production path originates the SRv6 SID NLRI attribute, so it is never included (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:33, :52, plugin.go:70-71) |
| [`RFC9514-7.1-2`](#rfc9514-7.1-2) Undefined flags in SRv6 Endpoint Behavior TLV MUST be set to 0 when originating (§7.1) | {gap}, no test | the encoder writes the Flags octet verbatim (no masking) and no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:37, plugin.go:70-71) |
| [`RFC9514-7.1-3`](#rfc9514-7.1-3) Algorithm value in SRv6 Endpoint Behavior TLV MUST be 0 unless an algorithm is associated locally with the SRv6 Locator (§7.1) | {gap}, no test | this origination-time semantic choice is unimplemented; ze originates no such TLV and the encoder writes Algorithm verbatim with no locator-association logic (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:38, plugin.go:70-71) |
| [`RFC9514-7.2-1`](#rfc9514-7.2-1) SRv6 BGP PeerNode SID TLV (1251) MUST be included along with SRv6 SIDs associated with BGP PeerNode or PeerSet functionality (§7.2) | {gap}, no test | the LsSRv6BGPPeerNodeSID decoder and encoder exist but no EPE origination path instantiates or includes the TLV from a live session (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:78, :102, plugin.go:70-71) |
| [`RFC9514-7.2-2`](#rfc9514-7.2-2) Reserved bits (3-7) in SRv6 BGP PeerNode SID flags MUST be set to 0 when originating (§7.2) | {gap}, no test | the encoder writes the Flags byte verbatim without masking bits 3-7 and no production path originates the TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:81, plugin.go:70-71) |
| [`RFC9514-7.2-3`](#rfc9514-7.2-3) Reserved field in SRv6 BGP PeerNode SID TLV MUST be set to 0 when originated (§7.2) | {gap}, no test | the encoder hardcodes both reserved octets to 0 but ze originates no SRv6 BGP PeerNode SID TLV in production (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:83-84, plugin.go:70-71) |
| [`RFC9514-8-1`](#rfc9514-8-1) SRv6 SID Structure sum (LB Length + LN Length + Fun. Length + Arg. Length) MUST be less than or equal to 128 (§8) | {gap}, no test | the SID Structure decoder reads the four length octets but never checks their sum is at most 128, accepting oversized values on receipt, and ze originates no SID Structure TLV (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:151-160, attr_link.go:702-708) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9514-3.1-1`](#rfc9514-3.1-1)

A single instance of the SRv6 Capabilities TLV (1038) MUST be included in the BGP-LS Attribute for each SRv6-capable node (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-3.1-1, so no unit is bound to it.

### [`RFC9514-3.1-2`](#rfc9514-3.1-2)

Reserved field in SRv6 Capabilities TLV MUST be set to 0 when originated (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-3.1-2, so no unit is bound to it.

### [`RFC9514-4.1-1`](#rfc9514-4.1-1)

Reserved field in SRv6 End.X SID TLV MUST be set to 0 when originated (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-4.1-1, so no unit is bound to it.

### [`RFC9514-4.2-1`](#rfc9514-4.2-1)

Reserved field in SRv6 LAN End.X SID TLV MUST be set to 0 when originated (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-4.2-1, so no unit is bound to it.

### [`RFC9514-5.1-1`](#rfc9514-5.1-1)

Reserved field in SRv6 Locator TLV MUST be set to 0 when originated (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-5.1-1, so no unit is bound to it.

### [`RFC9514-6-1`](#rfc9514-6-1)

SRv6 SID Descriptors MUST contain a single SRv6 SID Information TLV (518) (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-6-1, so no unit is bound to it.

### [`RFC9514-7.1-1`](#rfc9514-7.1-1)

The SRv6 Endpoint Behavior TLV (1250) MUST be included in the BGP-LS Attribute associated with the SRv6 SID NLRI (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.1-1, so no unit is bound to it.

### [`RFC9514-7.1-2`](#rfc9514-7.1-2)

Undefined flags in SRv6 Endpoint Behavior TLV MUST be set to 0 when originating (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.1-2, so no unit is bound to it.

### [`RFC9514-7.1-3`](#rfc9514-7.1-3)

Algorithm value in SRv6 Endpoint Behavior TLV MUST be 0 unless an algorithm is associated locally with the SRv6 Locator (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.1-3, so no unit is bound to it.

### [`RFC9514-7.2-1`](#rfc9514-7.2-1)

SRv6 BGP PeerNode SID TLV (1251) MUST be included along with SRv6 SIDs associated with BGP PeerNode or PeerSet functionality (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-1, so no unit is bound to it.

### [`RFC9514-7.2-2`](#rfc9514-7.2-2)

Reserved bits (3-7) in SRv6 BGP PeerNode SID flags MUST be set to 0 when originating (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-2, so no unit is bound to it.

### [`RFC9514-7.2-3`](#rfc9514-7.2-3)

Reserved field in SRv6 BGP PeerNode SID TLV MUST be set to 0 when originated (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-3, so no unit is bound to it.

### [`RFC9514-8-1`](#rfc9514-8-1)

SRv6 SID Structure sum (LB Length + LN Length + Fun. Length + Arg. Length) MUST be less than or equal to 128 (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-8-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9514, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9514, so its obligations are stated where they were written.
