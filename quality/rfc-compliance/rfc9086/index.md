# RFC 9086 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering

Partial. Every requirement this repository extracted from RFC 9086, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 72.7% | 8 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.1% | 1 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 94.1% | 16 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.2% | 2 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 16 |
| Summary | `rfc/short/rfc9086.md` |
| Requirement shard | `rfc/requirements/rfc9086.md` |
| RFC text | `rfc/full/rfc9086.txt` |

## Enrolment

Enrolled: BGP-LS Egress Peer Engineering SID decoding and native PeerNode SID origination from configured local SRGB assignments and established BGP sessions.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

PeerNode/Adj/Set SID TLVs and BGP Router-ID/Member-ASN descriptors decode. The native `bgp-epe` producer installs configured PeerNode labels through the MPLS FIB owner, waits for installation acknowledgement, and publishes live session identities with its SRGB. `bgp-ls-export` advertises that state and withdraws replaced or removed identities.

**What the ledger says remains:**

PeerAdj and PeerSet segment assignment are not implemented. Native EPE uses persistent operator-assigned indices, not three-octet label assignments. Centralized runtime verification and discrimination of the new producer remain required.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC9086-3-1`](#rfc9086-3-1), [`RFC9086-3-2`](#rfc9086-3-2), [`RFC9086-4.2-1`](#rfc9086-4.2-1), [`RFC9086-4.2-2`](#rfc9086-4.2-2), [`RFC9086-5-1`](#rfc9086-5-1), [`RFC9086-5-3`](#rfc9086-5-3), [`RFC9086-5-10`](#rfc9086-5-10), [`RFC9086-7-1`](#rfc9086-7-1)

**Annotated instead of tested (3):** [`RFC9086-5.2-1`](#rfc9086-5.2-1), [`RFC9086-5-2`](#rfc9086-5-2), [`RFC9086-5-5`](#rfc9086-5-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9086-3-1` | Each BGP session MUST be described by a PeerNode SID (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L89). **negative:** `unit/verify` [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L195) |
| `RFC9086-3-2` | One PeerNode SID MUST be instantiated to describe the BGP peer session (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L90). **negative:** `unit/verify` [`TestRFC9086NativeRepeatedUpKeepsOnePeerNodeSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L211) |
| `RFC9086-4.2-1` | BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Local Node Descriptors (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L92). **negative:** `unit/verify` [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L184) |
| `RFC9086-4.2-2` | BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Remote Node Descriptors (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L93). **negative:** `unit/verify` [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L185) |
| `RFC9086-5-1` | When enabled for Egress Peer Engineering, the BGP router MUST include the PeerNode SID TLV in the BGP-LS Attribute for the BGP-LS Link NLRI corresponding to its BGP peering sessions. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L91). **negative:** `unit/verify` [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L196) |
| `RFC9086-5.2-1` | Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:203-206) |
| `RFC9086-5-2` | V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| `RFC9086-5-3` | - Rsvd bits: Reserved for future use and MUST be zero when originated and ignored when received. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L254). **negative:** `unit/verify` [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L257) |
| `RFC9086-5-10` | When the SID/Index/Label field carries a 4-octet index into the Segment Routing Global Block, "the SRGB MUST be advertised using the extensions defined in [RFC9085]" (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L88). **negative:** `unit/verify` [`TestRFC9086NativePeerIndexRequiresSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L132) |
| `RFC9086-7-1` | The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L210). **negative:** `unit/verify` [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L211) |
| `RFC9086-5-5` | Reserved bits in Flags ignored when received (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L854). **negative:** no negative test. **{single-polarity}:** decodePeerSID reads Flags without branching on reserved bits and never rejects on them, so only a positive test is meaningful (internal/component/bgp/plugins/nlri/ls/attr_link.go:559-572) |
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
| [`RFC9086-5.2-1`](#rfc9086-5.2-1) Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2) | {gap}, no test | LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/component/bgp/plugins/nlri/ls/types_descriptor.go:203-206) |
| [`RFC9086-5-2`](#rfc9086-5-2) V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5) | {gap}, no test | the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9086-3-1`](#rfc9086-3-1)

Each BGP session MUST be described by a PeerNode SID (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L89) | unit/verify | revert, verified |

### [`RFC9086-3-2`](#rfc9086-3-2)

One PeerNode SID MUST be instantiated to describe the BGP peer session (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativeRepeatedUpKeepsOnePeerNodeSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L90) | unit/verify | revert, verified |

### [`RFC9086-4.2-1`](#rfc9086-4.2-1)

BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Local Node Descriptors (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L184) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L92) | unit/verify | revert, verified |

### [`RFC9086-4.2-2`](#rfc9086-4.2-2)

BGP Router-ID (TLV 516) and ASN (TLV 512) MUST be included as Remote Node Descriptors (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L185) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L93) | unit/verify | revert, verified |

### [`RFC9086-5-1`](#rfc9086-5-1)

When enabled for Egress Peer Engineering, the BGP router MUST include the PeerNode SID TLV in the BGP-LS Attribute for the BGP-LS Link NLRI corresponding to its BGP peering sessions. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L196) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L91) | unit/verify | revert, verified |

### [`RFC9086-5.2-1`](#rfc9086-5.2-1)

Link Local/Remote Identifiers (TLV 258) MUST be included in Link Descriptors for PeerAdj SID (S5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5.2-1, so no unit is bound to it.

### [`RFC9086-5-2`](#rfc9086-5-2)

V-Flag and L-Flag MUST be SET for 3-octet local label encoding (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9086-5-2, so no unit is bound to it.

### [`RFC9086-5-3`](#rfc9086-5-3)

- Rsvd bits: Reserved for future use and MUST be zero when originated and ignored when received. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L257) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L254) | unit/verify | revert, verified |

### [`RFC9086-5-10`](#rfc9086-5-10)

When the SID/Index/Label field carries a 4-octet index into the Segment Routing Global Block, "the SRGB MUST be advertised using the extensions defined in [RFC9085]" (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerIndexRequiresSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L88) | unit/verify | revert, verified |

### [`RFC9086-7-1`](#rfc9086-7-1)

The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/epe_rfc9086_test.go#L210) | unit/verify | revert, verified |

### [`RFC9086-5-5`](#rfc9086-5-5)

Reserved bits in Flags ignored when received (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L854) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9086.txt |
| Source fingerprint | 0e661e69fa88a113 |
| Record | rfc/extraction/rfc9086.json |
| Mapped sentences | 10 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 4 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `7` | not stated | 2 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Simplified BSD License for extracted Code Components; it binds republication of the document, not any protocol behaviour. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A comparative remark closing the operator-control paragraph: 'what is required by a BGP speaker' is lowercase and states that this control is nothing new, imposing no behaviour of its own. | This is not different from what is required by a BGP speaker in terms of information origination and advertisement. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations prose: 'is also required to ensure' is lowercase and describes a deployment expectation about session isolation that RFC 7752 Section 8 defines, not an obligation this document places on the encoder or decoder. | The isolation of BGP-LS peering sessions is also required to ensure that BGP-LS topology information (including the newly added BGP peering topology) is not advertised to an external BGP peering session outside an administrative domain. |

## Superseded

No document obsoletes RFC 9086, so its obligations are stated where they were written.
