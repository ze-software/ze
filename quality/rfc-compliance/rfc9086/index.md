# RFC 9086 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering

BGP-LS Egress Peer Engineering SIDs. Every requirement listed for RFC 9086, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 11 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Tests shown to detect a fault | 22 | 22 of 22 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial test coverage | 1 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 3 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 7 | of 11 mandatory requirements reviewed | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 20 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | bad | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP-LS Egress Peer Engineering SIDs |
| Automatic checks | Included |
| Requirements | 20 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tests linked to requirements | 22 |
| Recorded audit verdicts | 7 |
| Fault test records | 22 |
| Summary | `rfc/short/rfc9086.md` |
| Requirement table | `rfc/requirements/rfc9086.md` |
| RFC text | `rfc/full/rfc9086.txt` |

## Automatic checks

Included in automatic checks: BGP-LS Egress Peer Engineering SID decoding and native PeerNode SID origination from configured local SRGB assignments and established BGP sessions.

## Published support status

**Area:** BGP-LS Egress Peer Engineering SIDs

**Test verification:** 7 of 11 MUST proven

**Evidence:** unit 7

**Unmet:** RFC9086-5-2, RFC9086-5.2-1, RFC9086-7-1, RFC9086-7-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 1 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC9086-3-1`](#rfc9086-3-1), [`RFC9086-3-2`](#rfc9086-3-2), [`RFC9086-4.2-1`](#rfc9086-4.2-1), [`RFC9086-4.2-2`](#rfc9086-4.2-2), [`RFC9086-5-1`](#rfc9086-5-1), [`RFC9086-5-3`](#rfc9086-5-3), [`RFC9086-5-10`](#rfc9086-5-10)

**Annotated (including scoped evidence) (4):** [`RFC9086-5.2-1`](#rfc9086-5.2-1), [`RFC9086-5-2`](#rfc9086-5-2), [`RFC9086-7-1`](#rfc9086-7-1), [`RFC9086-7-2`](#rfc9086-7-2)

**Partial test coverage (subset of annotated; zero whole-requirement credit) (1):** [`RFC9086-7-1`](#rfc9086-7-1)

## Figures

- **MUST bar**: Ze verified 7, Not built 4
- **Overview bar**: Ze MUST verified 7, Not assessed 9, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze verified | 7 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 9 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 9 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9086.json](../../../data/rfc/rfc9086.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9086-3-1` | Each BGP session MUST be described by a PeerNode SID (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L91). **negative:** `unit/verify` [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L307) |
| `RFC9086-3-2` | One PeerNode SID MUST be instantiated to describe the BGP peer session (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L92). **negative:** `unit/verify` [`TestRFC9086NativeRepeatedUpKeepsOnePeerNodeSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L323) |
| `RFC9086-4.2-1` | The following Node Descriptor TLVs MUST be included in BGP-LS NLRI as Local Node Descriptors when distributing BGP information: * BGP Router-ID (TLV 516), which contains a valid BGP Identifier of the local BGP node. * Autonomous System Number (TLV 512) [RFC7752], which contains the Autonomous System Number (ASN) or AS Confederation Identifier (an ASN) [RFC5065], if confederations are used, of the local BGP node. (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9086EveryBGPNLRICarriesLocalRouterIDAndASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_local_descriptors_test.go#L26). **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L94). **negative:** `unit/verify` [`TestRFC9086NativePeerRequiresLocalRouterID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_local_descriptors_test.go#L57). **negative:** `unit/verify` [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L186) |
| `RFC9086-4.2-2` | The following Node Descriptor TLVs MUST be included in BGP-LS Link NLRI as Remote Node Descriptors when distributing BGP information: * BGP Router-ID (TLV 516), which contains the valid BGP Identifier of the peer BGP node. * Autonomous System Number (TLV 512) [RFC7752], which contains the ASN or the AS Confederation Identifier (an ASN) [RFC5065], if confederations are used, of the peer BGP node. (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L95). **negative:** `unit/verify` [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L187) |
| `RFC9086-5-1` | When enabled for Egress Peer Engineering, the BGP router MUST include the PeerNode SID TLV in the BGP-LS Attribute for the BGP-LS Link NLRI corresponding to its BGP peering sessions. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L93). **negative:** `unit/verify` [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L308) |
| `RFC9086-5.2-1` | Link Descriptors MUST include the following TLV, as defined in [RFC7752]: - Link Local/Remote Identifiers (TLV 258) contains the 4-octet Link Local Identifier followed by the 4-octet Link Remote Identifier. (S5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/core/bgp/nlri/bgpls/types_descriptor.go:203-206) |
| `RFC9086-5-2` | A 3-octet local label where the 20 rightmost bits are used for encoding the label value. In this case, the V- and L-Flags MUST be SET. (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| `RFC9086-5-3` | - Rsvd bits: Reserved for future use and MUST be zero when originated and ignored when received. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L366). **positive:** `unit/verify` [`TestRFC9086OriginatedPeerSIDRsvdBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_export_sentence_test.go#L57). **positive:** `unit/verify` [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L857). **positive:** `unit/verify` [`TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9086_rsvd_receive_test.go#L28). **negative:** `unit/verify` [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L369). **negative:** `unit/verify` [`TestRFC9086SourceRsvdBitsNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_export_sentence_test.go#L65) |
| `RFC9086-5-10` | A 4-octet index defining the offset in the Segment Routing Global Block (SRGB) [RFC8402] advertised by this router. In this case, the SRGB MUST be advertised using the extensions defined in [RFC9085]. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L90). **negative:** `unit/verify` [`TestRFC9086NativePeerIndexRequiresSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L134) |
| `RFC9086-7-1` | The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L212). **negative:** `unit/verify` [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L213). **{partial}:** tested "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID,"; gap "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer."; Owner R3 preserves the tested PeerNode configuration scope. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig provides PeerNode assignments only. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig provides no per-recipient EPE information controls. PeerAdj and PeerSet controls remain absent, as recorded by RFC9086-7-2. No whole-requirement conformance is claimed.. **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent EPEReconcileNow judgment against RFC9086 Section 7 and owner R3. Tested: "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID,". Gap: "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer." TestRFC9086NativeConfigurationEnableDisable parses PeerNode configuration, asserts label 16007 and exact PeerNode TLV bytes, then removes configuration and requires both collector withdrawals and label removal while the session stays up. The registered producer/exporter path connects these controls to native advertisements. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig provides only PeerNode assignments. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig exposes no per-recipient EPE control, and exporter reconciliation supplies the same desired topology to attached collectors. Existing publishLocked breaks establish producer reachability for both retained claims, not complete Section 7 conformance. The complete sentence remains unmet; no absent PeerAdj, PeerSet, or per-recipient capability is implemented by this correction. |
| `RFC9086-7-2` | PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** R3 split of the operator-control sentence; native bgp-epe implements only PeerNode assignment, not PeerAdj/PeerSet advertisement options or per-recipient information controls (internal/component/bgp/plugins/epe/epe_config.go ParseConfig; internal/component/bgp/plugins/ls_export/export_config.go parseExportConfig). |
| `RFC9086-5-7` | The values of the PeerNode SID, PeerAdj SID, and PeerSet SID Sub-TLVs SHOULD be persistent across router restart. (S5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-3` | As such, when the IBGP design includes sessions with route reflectors, a BGP router SHOULD NOT instantiate a BGP Peering SID for those sessions to peer nodes that are not in the forwarding path since the purpose of BGP Peering SID is to steer traffic to those specific peers. (S3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-4` | One or more PeerAdj SID MAY be instantiated corresponding to the underlying link(s) to the directly connected BGP peer session. (S3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-3-5` | A PeerSet SID MAY be instantiated and additionally associated and shared between one or more PeerNode SIDs or PeerAdj SIDs. (S3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-4.3-1` | The following Node Descriptor TLVs MAY be included in BGP-LS NLRI as Local Node Descriptors when distributing BGP information: * Member-ASN (TLV 517), which contains the ASN of the confederation member (i.e., Member-AS Number), if BGP confederations are used, of the local BGP node. * Node Descriptors as defined in [RFC7752]. The following Node Descriptor TLVs MAY be included in BGP-LS Link NLRI as Remote Node Descriptors when distributing BGP information: * Member-ASN (TLV 517), which contains the ASN of the confederation member (i.e., Member-AS Number), if BGP confederations are used, of the peer BGP node. (S4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-4.3-2` | The following Node Descriptor TLVs MAY be included in BGP-LS NLRI as Local Node Descriptors when distributing BGP information: * Member-ASN (TLV 517), which contains the ASN of the confederation member (i.e., Member-AS Number), if BGP confederations are used, of the local BGP node. * Node Descriptors as defined in [RFC7752]. The following Node Descriptor TLVs MAY be included in BGP-LS Link NLRI as Remote Node Descriptors when distributing BGP information: * Member-ASN (TLV 517), which contains the ASN of the confederation member (i.e., Member-AS Number), if BGP confederations are used, of the peer BGP node. * Node Descriptors as defined in [RFC7752]. (S4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5-8` | The PeerAdj SID and PeerSet SID TLVs MAY be included in the BGP-LS Attribute (S5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5-9` | Additional BGP-LS Link Attribute TLVs as defined in [RFC7752] MAY be included with the BGP-LS Link NLRI in order to advertise the characteristics of the peering link (S5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9086-5.2-2` | Additional Link Descriptors TLVs, as defined in [RFC7752], MAY also be included to describe the addresses corresponding to the link between the BGP routers (S5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9086-5.2-1`](#rfc9086-5.2-1) Link Descriptors MUST include the following TLV, as defined in [RFC7752]: - Link Local/Remote Identifiers (TLV 258) contains the 4-octet Link Local Identifier followed by the 4-octet Link Remote Identifier. (S5.2) | {gap}, no test | LinkDescriptor.WriteTo emits TLV 258 but no origination path builds a PeerAdj SID advertisement (internal/core/bgp/nlri/bgpls/types_descriptor.go:203-206) |
| [`RFC9086-5-2`](#rfc9086-5-2) A 3-octet local label where the 20 rightmost bits are used for encoding the label value. In this case, the V- and L-Flags MUST be SET. (S5) | {gap}, no test | the encoder writes the Flags byte verbatim and never originates a label-encoded SID, so no path sets V/L (internal/component/bgp/plugins/nlri/ls/attr_link.go:521) |
| [`RFC9086-7-1`](#rfc9086-7-1) The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7) | {partial}, Partial test coverage | tested "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID,"; gap "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer."; Owner R3 preserves the tested PeerNode configuration scope. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig provides PeerNode assignments only. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig provides no per-recipient EPE information controls. PeerAdj and PeerSet controls remain absent, as recorded by RFC9086-7-2. No whole-requirement conformance is claimed. |
| [`RFC9086-7-2`](#rfc9086-7-2) PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7) | {gap}, no test | R3 split of the operator-control sentence; native bgp-epe implements only PeerNode assignment, not PeerAdj/PeerSet advertisement options or per-recipient information controls (internal/component/bgp/plugins/epe/epe_config.go ParseConfig; internal/component/bgp/plugins/ls_export/export_config.go parseExportConfig). |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9086-3-1`](#rfc9086-3-1)

Each BGP session MUST be described by a PeerNode SID (S3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L307) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L91) | unit/verify | revert, verified |

### [`RFC9086-3-2`](#rfc9086-3-2)

One PeerNode SID MUST be instantiated to describe the BGP peer session (S3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativeRepeatedUpKeepsOnePeerNodeSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L323) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L92) | unit/verify | revert, verified |

### [`RFC9086-4.2-1`](#rfc9086-4.2-1)

The following Node Descriptor TLVs MUST be included in BGP-LS NLRI as Local Node Descriptors when distributing BGP information: * BGP Router-ID (TLV 516), which contains a valid BGP Identifier of the local BGP node. * Autonomous System Number (TLV 512) [RFC7752], which contains the Autonomous System Number (ASN) or AS Confederation Identifier (an ASN) [RFC5065], if confederations are used, of the local BGP node. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-10-02 (c33 judge): + TestRFC9086EveryBGPNLRICarriesLocalRouterIDAndASN reads both emitted NLRIs (Node type 1 and Link type 2, Protocol-ID 7) and asserts one Local Node Descriptors TLV 256 holding TLV 516 c0 00 02 01 and TLV 512 00 00 fd e8 exactly once each, closing the unchecked Node NLRI. - TestRFC9086NativePeerRequiresLocalRouterID (local router-id 0.0.0.0 or absent: State errors, no label, no NLRI) beside HEAD TestRFC9086NativePeerRequiresNodeIdentities (local ASN 0). Judge break: the local-identifier check in epe_source.go State reduced to rejecting only a non-IPv4 address turns the new negative red. The confederation-identifier clause is conditional on confederations, which have no configuration surface in Ze (internal/component/bgp/reactor/forward_local_pref.go), so TLV 512 always carries the plain local ASN.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerRequiresLocalRouterID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_local_descriptors_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L186) | unit/verify | revert, verified |
| positive | [`TestRFC9086EveryBGPNLRICarriesLocalRouterIDAndASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_local_descriptors_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L94) | unit/verify | revert, verified |

### [`RFC9086-4.2-2`](#rfc9086-4.2-2)

The following Node Descriptor TLVs MUST be included in BGP-LS Link NLRI as Remote Node Descriptors when distributing BGP information: * BGP Router-ID (TLV 516), which contains the valid BGP Identifier of the peer BGP node. * Autonomous System Number (TLV 512) [RFC7752], which contains the ASN or the AS Confederation Identifier (an ASN) [RFC5065], if confederations are used, of the peer BGP node. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a BGP-LS Link NLRI whose Remote Node Descriptors lack TLV 516 or TLV 512 of the peer. (b) TestRFC9086NativePeerIndexAdvertisesSRGB asserts remote 516=192.0.2.2 and remote 512=65001 on the Link NLRI; each goes red if that TLV is dropped or carries the local value. Negative: TestRFC9086NativePeerRequiresNodeIdentities, a remote Router-ID of 0.0.0.0 refuses origination, installs no label and emits no command.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerRequiresNodeIdentities`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L95) | unit/verify | revert, verified |

### [`RFC9086-5-1`](#rfc9086-5-1)

When enabled for Egress Peer Engineering, the BGP router MUST include the PeerNode SID TLV in the BGP-LS Attribute for the BGP-LS Link NLRI corresponding to its BGP peering sessions. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an EPE-enabled BGP session advertised as a Link NLRI whose BGP-LS Attribute lacks the PeerNode SID TLV 1101. (b) TestRFC9086NativePeerIndexAdvertisesSRGB asserts exportTLVValues(linkAttrs, 1101) equals exactly the assigned SID; TestRFC9086NativeLinkWithoutPeerNodeRefused asserts a BGP Link snapshot with no 1101 is refused (replace errors, no command emitted).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativeLinkWithoutPeerNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L308) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L93) | unit/verify | revert, verified |

### [`RFC9086-5.2-1`](#rfc9086-5.2-1)

Link Descriptors MUST include the following TLV, as defined in [RFC7752]: - Link Local/Remote Identifiers (TLV 258) contains the 4-octet Link Local Identifier followed by the 4-octet Link Remote Identifier. (S5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9086-5.2-1, so there is no linked test.

### [`RFC9086-5-2`](#rfc9086-5-2)

A 3-octet local label where the 20 rightmost bits are used for encoding the label value. In this case, the V- and L-Flags MUST be SET. (S5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9086-5-2, so there is no linked test.

### [`RFC9086-5-3`](#rfc9086-5-3)

- Rsvd bits: Reserved for future use and MUST be zero when originated and ignored when received. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. D2 rejudge 2026-10-16 (independent judge, BGP-LS codec moved to internal/core/bgp/nlri/bgpls). Tagged units changed only by the package move: identifiers re-qualified with bgpls., comments re-pointed; producer bodies byte-identical modulo the writeTLV->WriteTLV and field-export renames (TLV code constants verified unchanged). RFC 9086 Section 5: "Rsvd bits: Reserved for future use and MUST be zero when originated and ignored when received." Originate: ls_export 0xc0/0xcf pair and the native PeerNode reserved-flags unit (unchanged). Receive: TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived iterates bgpls.TLVPeerNodeSID/AdjSID/SetSID (same codes 1101-1103) over the registered decoders with reserved set and clear, requiring equal flags, weight and SID; decodePeerSID unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L369) | unit/verify | revert, verified |
| negative | [`TestRFC9086SourceRsvdBitsNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_export_sentence_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerSIDReservedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/export_rfc9552_polarity_test.go#L366) | unit/verify | revert, verified |
| positive | [`TestRFC9086OriginatedPeerSIDRsvdBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_export_sentence_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC9086PeerSIDIgnoresReservedFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/attr_test.go#L857) | unit/verify | revert, verified |
| positive | [`TestRFC9086PeerSIDRsvdBitsIgnoredWhenReceived`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9086_rsvd_receive_test.go#L28) | unit/verify | revert, verified |

### [`RFC9086-5-10`](#rfc9086-5-10)

A 4-octet index defining the offset in the Segment Routing Global Block (SRGB) [RFC8402] advertised by this router. In this case, the SRGB MUST be advertised using the extensions defined in [RFC9085]. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: advertising an index-encoded PeerNode SID without advertising the SRGB through the RFC 9085 SR Capabilities TLV. (b) TestRFC9086NativePeerIndexAdvertisesSRGB asserts the Node NLRI attribute carries TLV 1034 with range 100 and base label 16000 alongside the index SID 7; TestRFC9086NativePeerIndexRequiresSRGB asserts an index SID with no SRGB is refused and no command is emitted.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativePeerIndexRequiresSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativePeerIndexAdvertisesSRGB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L90) | unit/verify | revert, verified |

### [`RFC9086-7-1`](#rfc9086-7-1)

The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7)

Records cover only part of this requirement: tested "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID,"; gap "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer."; Owner R3 preserves the tested PeerNode configuration scope. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig provides PeerNode assignments only. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig provides no per-recipient EPE information controls. PeerAdj and PeerSet controls remain absent, as recorded by RFC9086-7-2. No whole-requirement conformance is claimed.; they do not verify the whole requirement.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent EPEReconcileNow judgment against RFC9086 Section 7 and owner R3. Tested: "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID,". Gap: "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer." TestRFC9086NativeConfigurationEnableDisable parses PeerNode configuration, asserts label 16007 and exact PeerNode TLV bytes, then removes configuration and requires both collector withdrawals and label removal while the session stays up. The registered producer/exporter path connects these controls to native advertisements. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig provides only PeerNode assignments. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig exposes no per-recipient EPE control, and exporter reconciliation supplies the same desired topology to attached collectors. Existing publishLocked breaks establish producer reachability for both retained claims, not complete Section 7 conformance. The complete sentence remains unmet; no absent PeerAdj, PeerSet, or per-recipient capability is implemented by this correction.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L213) | unit/verify | revert, verified |
| positive | [`TestRFC9086NativeConfigurationEnableDisable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go#L212) | unit/verify | revert, verified |

### [`RFC9086-7-2`](#rfc9086-7-2)

PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer. (S7)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent EPEReconcileNow first judgment against the complete RFC9086 Section 7 operator-control sentence and owner R3. The retained split-gap clause requires PeerAdj SID and PeerSet SID advertisement controls and selection of information for each internal or external recipient. internal/component/bgp/plugins/epe/epe_config.go::ParseConfig accepts PeerNode assignments only; it has no PeerAdj or PeerSet assignment. internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig configures exporter enablement and domain mappings, not per-recipient EPE information controls. The exporter supplies the same desired topology to its attached collectors. These absent capabilities remain disclosed in Support remaining and the gap annotation. Existing PeerNode enable/disable tests prove none of this absent scope; recording it does not authorize implementation or imply another distinct RFC sentence.

No test carries RFC9086-7-2, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 2 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 4 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 2 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Simplified BSD License for extracted Code Components; it binds republication of the document, not any protocol behaviour. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A comparative remark closing the operator-control paragraph: 'what is required by a BGP speaker' is lowercase and states that this control is nothing new, imposing no behaviour of its own. | This is not different from what is required by a BGP speaker in terms of information origination and advertisement. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations prose: 'is also required to ensure' is lowercase and describes a deployment expectation about session isolation that RFC 7752 Section 8 defines, not an obligation this document places on the encoder or decoder. | The isolation of BGP-LS peering sessions is also required to ensure that BGP-LS topology information (including the newly added BGP peering topology) is not advertised to an external BGP peering session outside an administrative domain. |

## Superseded

No document obsoletes RFC 9086, so its obligations are stated where they were written.
