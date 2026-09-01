# RFC 9086 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering

Partial. Every requirement this repository extracted from RFC 9086, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 12 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 2 of 12 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 21 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 83.3% | 10 of 12 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 12 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 21 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9086.md` |
| Requirement shard | `rfc/requirements/rfc9086.md` |
| RFC text | `rfc/full/rfc9086.txt` |

## Enrolment

Enrolled: BGP-LS Egress Peer Engineering SIDs: 2 single-polarity positive (reserved ignored on receipt) + 10 gap (EPE SID origination not implemented; decode only)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- PeerNode/Adj/Set SID TLVs (1101-1103) and BGP Router-ID (516) / Member-ASN (517) node descriptors decode as part of BGP-LS TLV coverage
- reserved fields ignored on receipt.


**What the ledger says remains**

EPE SID origination is not implemented: no code instantiates PeerNode/Adj/Set SIDs from live sessions, the BGP-LS plugin registers decode mode only, and there is no config surface to enable or disable EPE advertisement. Same BGP-LS encode gap as RFC 7752.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (12):** [`RFC9086-3-1`](#rfc9086-3-1), [`RFC9086-3-2`](#rfc9086-3-2), [`RFC9086-4.2-1`](#rfc9086-4.2-1), [`RFC9086-4.2-2`](#rfc9086-4.2-2), [`RFC9086-5-1`](#rfc9086-5-1), [`RFC9086-5.2-1`](#rfc9086-5.2-1), [`RFC9086-5-2`](#rfc9086-5-2), [`RFC9086-5-3`](#rfc9086-5-3), [`RFC9086-5-4`](#rfc9086-5-4), [`RFC9086-7-1`](#rfc9086-7-1), [`RFC9086-5-5`](#rfc9086-5-5), [`RFC9086-5-6`](#rfc9086-5-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9086-3-1` | Each BGP session MUST be described by a PeerNode SID (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes PeerNode SID TLVs but no production code instantiates a PeerNode SID from a live BGP session (decoder internal/component/bgp/plugins/nlri/ls/attr_link.go:554; no non-test caller of the encoder) |
| `RFC9086-3-2` | One PeerNode SID MUST be instantiated to describe the BGP peer session (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Peer SID encoder exists but nothing instantiates one per session in any production path; the plugin registers decode only (internal/component/bgp/plugins/nlri/ls/attr_link.go:517, plugin.go:70-71) |
| `RFC9086-4.2-1` | BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Local Node Descriptors (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the NodeDescriptor encoder emits TLV 516/512 but no origination path builds an EPE Link NLRI to carry them (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:101, :116) |
| `RFC9086-4.2-2` | BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Remote Node Descriptors (S4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same encoder covers remote descriptors, but no origination path constructs the NLRI for a live peer (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98-128) |
| `RFC9086-5-1` | BGP router MUST include PeerNode SID TLV in BGP-LS Attribute when EPE enabled (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no EPE-enable path and the BGP-LS plugin never encodes or advertises the attribute (internal/component/bgp/plugins/nlri/ls/attr_link.go:517, plugin.go:70-71) |
| `RFC9086-5.2-1` | Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:203-206) |
| `RFC9086-5-2` | V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| `RFC9086-5-3` | Reserved bits in Flags MUST be zero when originated (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no production path originates Peer SIDs and the encoder writes Flags unmasked (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| `RFC9086-5-4` | Reserved field (2 octets) MUST be set to 0 on transmit (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder hardcodes the reserved octets to 0 but ze transmits no Peer SIDs in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:523-524) |
| `RFC9086-7-1` | Operator MUST be provided with options to configure, enable, and disable the advertisement (S7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the BGP-LS plugin augments no config schema; no YANG or CLI surface enables or disables EPE advertisement (internal/component/bgp/plugins/nlri/ls/plugin.go:94-96) |
| `RFC9086-5-5` | Reserved bits in Flags ignored when received (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L852). **negative:** no negative test. **{single-polarity}:** decodePeerSID reads Flags without branching on reserved bits and never rejects on them, so only a positive test is meaningful (internal/component/bgp/plugins/nlri/ls/attr_link.go:559-572) |
| `RFC9086-5-6` | Reserved field (2 octets) ignored on receipt (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L856). **negative:** no negative test. **{single-polarity}:** decodePeerSID reads data[0]/data[1] then jumps to data[4:] for the SID, never inspecting the reserved octets, so it inherently ignores them (internal/component/bgp/plugins/nlri/ls/attr_link.go:559-570) |
| `RFC9086-5-7` | PeerNode SID, PeerAdj SID, PeerSet SID values SHOULD be persistent across router restart (S5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-3` | BGP router SHOULD NOT instantiate BGP Peering SID for IBGP sessions to route reflectors not in forwarding path (S3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-4` | PeerAdj SID MAY be instantiated for underlying link(s) to directly connected BGP peer (S3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-5` | PeerSet SID MAY be instantiated and shared between PeerNode SIDs or PeerAdj SIDs (S3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-4.3-1` | Member-ASN (TLV 517) MAY be included in Local/Remote Node Descriptors (S4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-4.3-2` | Other Node Descriptors per RFC 7752 MAY be included (S4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5-8` | PeerAdj SID and PeerSet SID TLVs MAY be included in BGP-LS Attribute (S5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5-9` | Additional Link Attribute TLVs per RFC 7752 MAY be included (S5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5.2-2` | Additional Link Descriptor TLVs MAY be included for PeerAdj SID (S5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9086-3-1`](#rfc9086-3-1) Each BGP session MUST be described by a PeerNode SID (S3) | {gap}, no test | ze decodes PeerNode SID TLVs but no production code instantiates a PeerNode SID from a live BGP session (decoder internal/component/bgp/plugins/nlri/ls/attr_link.go:554; no non-test caller of the encoder) |
| [`RFC9086-3-2`](#rfc9086-3-2) One PeerNode SID MUST be instantiated to describe the BGP peer session (S3) | {gap}, no test | the Peer SID encoder exists but nothing instantiates one per session in any production path; the plugin registers decode only (internal/component/bgp/plugins/nlri/ls/attr_link.go:517, plugin.go:70-71) |
| [`RFC9086-4.2-1`](#rfc9086-4.2-1) BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Local Node Descriptors (S4.2) | {gap}, no test | the NodeDescriptor encoder emits TLV 516/512 but no origination path builds an EPE Link NLRI to carry them (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:101, :116) |
| [`RFC9086-4.2-2`](#rfc9086-4.2-2) BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Remote Node Descriptors (S4.2) | {gap}, no test | the same encoder covers remote descriptors, but no origination path constructs the NLRI for a live peer (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:98-128) |
| [`RFC9086-5-1`](#rfc9086-5-1) BGP router MUST include PeerNode SID TLV in BGP-LS Attribute when EPE enabled (S5) | {gap}, no test | there is no EPE-enable path and the BGP-LS plugin never encodes or advertises the attribute (internal/component/bgp/plugins/nlri/ls/attr_link.go:517, plugin.go:70-71) |
| [`RFC9086-5.2-1`](#rfc9086-5.2-1) Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2) | {gap}, no test | LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:203-206) |
| [`RFC9086-5-2`](#rfc9086-5-2) V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5) | {gap}, no test | the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| [`RFC9086-5-3`](#rfc9086-5-3) Reserved bits in Flags MUST be zero when originated (S5) | {gap}, no test | no production path originates Peer SIDs and the encoder writes Flags unmasked (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| [`RFC9086-5-4`](#rfc9086-5-4) Reserved field (2 octets) MUST be set to 0 on transmit (S5) | {gap}, no test | the encoder hardcodes the reserved octets to 0 but ze transmits no Peer SIDs in production (internal/component/bgp/plugins/nlri/ls/attr_link.go:523-524) |
| [`RFC9086-7-1`](#rfc9086-7-1) Operator MUST be provided with options to configure, enable, and disable the advertisement (S7) | {gap}, no test | the BGP-LS plugin augments no config schema; no YANG or CLI surface enables or disables EPE advertisement (internal/component/bgp/plugins/nlri/ls/plugin.go:94-96) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9086-3-1`](#rfc9086-3-1)

Each BGP session MUST be described by a PeerNode SID (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-3-1, so no unit is bound to it.

### [`RFC9086-3-2`](#rfc9086-3-2)

One PeerNode SID MUST be instantiated to describe the BGP peer session (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-3-2, so no unit is bound to it.

### [`RFC9086-4.2-1`](#rfc9086-4.2-1)

BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Local Node Descriptors (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-4.2-1, so no unit is bound to it.

### [`RFC9086-4.2-2`](#rfc9086-4.2-2)

BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Remote Node Descriptors (S4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-4.2-2, so no unit is bound to it.

### [`RFC9086-5-1`](#rfc9086-5-1)

BGP router MUST include PeerNode SID TLV in BGP-LS Attribute when EPE enabled (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5-1, so no unit is bound to it.

### [`RFC9086-5.2-1`](#rfc9086-5.2-1)

Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5.2-1, so no unit is bound to it.

### [`RFC9086-5-2`](#rfc9086-5-2)

V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5-2, so no unit is bound to it.

### [`RFC9086-5-3`](#rfc9086-5-3)

Reserved bits in Flags MUST be zero when originated (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5-3, so no unit is bound to it.

### [`RFC9086-5-4`](#rfc9086-5-4)

Reserved field (2 octets) MUST be set to 0 on transmit (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5-4, so no unit is bound to it.

### [`RFC9086-7-1`](#rfc9086-7-1)

Operator MUST be provided with options to configure, enable, and disable the advertisement (S7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-7-1, so no unit is bound to it.

### [`RFC9086-5-5`](#rfc9086-5-5)

Reserved bits in Flags ignored when received (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L852) | unit/verify | unproven |

### [`RFC9086-5-6`](#rfc9086-5-6)

Reserved field (2 octets) ignored on receipt (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L856) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9086, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9086, so its obligations are stated where they were written.
