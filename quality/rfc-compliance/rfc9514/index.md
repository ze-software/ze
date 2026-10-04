# RFC 9514 - Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing over IPv6 (SRv6)

Partial. Every requirement this repository extracted from RFC 9514, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 65.0% | 13 of 20 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.0% | 2 of 20 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 20 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 42 of 42 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 20 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 20 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 20 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 20 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 20 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial proof; remaining gap | 10.0% | 2 of 20 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 15.0% | 3 of 20 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 17 | of 20 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 20 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 23 |
| Gated MUST-level | 20 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 42 |
| Tagged units | 42 |
| Recorded audit verdicts | 17 |
| Discrimination records | 42 |
| Summary | `rfc/short/rfc9514.md` |
| Requirement shard | `rfc/requirements/rfc9514.md` |
| RFC text | `rfc/full/rfc9514.txt` |

## Enrolment

Enrolled: BGP-LS SRv6 extensions, including separate origination and reserved-field receive obligations. The offline attribute decoder handles SRv6 Capabilities and Locator TLVs through its registered consumer decoders.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The offline consumer decodes SRv6 Capabilities, Locator, End.X SID, Endpoint Behavior, BGP PeerNode SID, and SID Structure TLVs. The native IS-IS LSDB adapter maps SRv6 capabilities, locators, End/End.X SIDs, and SID structure into BGP-LS advertisements with source topology and locator association. The originator clears reserved words, requires Endpoint Behavior for SID NLRIs, and refuses oversized SID structures.

**What the ledger says remains**

Native OSPFv3 SRv6 state and SRv6 BGP EPE segment assignment are absent. SID Structure sum-at-most-128 validation is absent from the offline consumer decoder. New native producer proofs still require centralized execution and discrimination.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 2 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **20** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9514-3.1-1`](#rfc9514-3.1-1), [`RFC9514-3.1-2`](#rfc9514-3.1-2), [`RFC9514-3.1-3`](#rfc9514-3.1-3), [`RFC9514-4.1-1`](#rfc9514-4.1-1), [`RFC9514-4.1-2`](#rfc9514-4.1-2), [`RFC9514-4.2-1`](#rfc9514-4.2-1), [`RFC9514-4.2-2`](#rfc9514-4.2-2), [`RFC9514-5.1-1`](#rfc9514-5.1-1), [`RFC9514-5.1-2`](#rfc9514-5.1-2), [`RFC9514-7.1-1`](#rfc9514-7.1-1), [`RFC9514-7.1-2`](#rfc9514-7.1-2), [`RFC9514-7.1-4`](#rfc9514-7.1-4), [`RFC9514-8-1`](#rfc9514-8-1)

**Annotated (including scoped evidence) (7):** [`RFC9514-6-1`](#rfc9514-6-1), [`RFC9514-7.1-3`](#rfc9514-7.1-3), [`RFC9514-7.2-1`](#rfc9514-7.2-1), [`RFC9514-7.2-2`](#rfc9514-7.2-2), [`RFC9514-7.2-5`](#rfc9514-7.2-5), [`RFC9514-7.2-3`](#rfc9514-7.2-3), [`RFC9514-7.2-6`](#rfc9514-7.2-6)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (2):** [`RFC9514-7.2-5`](#rfc9514-7.2-5), [`RFC9514-7.2-6`](#rfc9514-7.2-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9514-3.1-1` | A single instance of this TLV MUST be included in the BGP-LS Attribute for each SRv6-capable node. (§3.1) | MUST | 3.1 - SRv6 Capabilities TLV | **positive:** `unit/verify` [`TestRFC9514ISISSRv6CapabilitiesSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L82). **negative:** `unit/verify` [`TestRFC9514ISISSRv6CapabilitiesSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L91) |
| `RFC9514-3.1-2` | Reserved: 2-octet field that MUST be set to 0 when originated (§3.1) | MUST | 3.1 - SRv6 Capabilities TLV | **positive:** `unit/verify` [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L40). **negative:** `unit/verify` [`TestRFC9514NativeSRv6ReservedCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L58) |
| `RFC9514-3.1-3` | Reserved: 2-octet field that MUST be set to 0 when originated and ignored on receipt. (§3.1) | MUST | 3.1 - SRv6 Capabilities TLV | **positive:** `unit/verify` [`TestRFC9514CapabilitiesDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L131). **positive:** `unit/verify` [`TestRFC9514OriginatedReservedWordsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L34). **negative:** `unit/verify` [`TestRFC9514CapabilitiesReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L143). **negative:** `unit/verify` [`TestRFC9514PoisonedSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L44) |
| `RFC9514-4.1-1` | Reserved: 1-octet field that MUST be set to 0 when originated (§4.1) | MUST | 4.1 - SRv6 End.X SID TLV | **positive:** `unit/verify` [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L107). **negative:** `unit/verify` [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L110) |
| `RFC9514-4.1-2` | Reserved: 1-octet field that MUST be set to 0 when originated and ignored on receipt. (§4.1) | MUST | 4.1 - SRv6 End.X SID TLV | **positive:** `unit/verify` [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L43). **positive:** `unit/verify` [`TestRFC9514ISISEndXOriginatedReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L42). **negative:** `unit/verify` [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L44). **negative:** `unit/verify` [`TestRFC9514ISISEndXAllOnesNeverReachReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L52) |
| `RFC9514-4.2-1` | Reserved: 1-octet field that MUST be set to 0 when originated (§4.2) | MUST | 4.2 - SRv6 LAN End.X SID TLV | **positive:** `unit/verify` [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L108). **negative:** `unit/verify` [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L111) |
| `RFC9514-4.2-2` | Reserved: 1-octet field that MUST be set to 0 when originated and ignored on receipt. (§4.2) | MUST | 4.2 - SRv6 LAN End.X SID TLV | **positive:** `unit/verify` [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L45). **positive:** `unit/verify` [`TestRFC9514ISISEndXOriginatedReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L43). **negative:** `unit/verify` [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L46). **negative:** `unit/verify` [`TestRFC9514ISISEndXAllOnesNeverReachReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L53) |
| `RFC9514-5.1-1` | Reserved: 2-octet field. The value MUST be set to 0 when originated (§5.1) | MUST | 5.1 - SRv6 Locator TLV | **positive:** `unit/verify` [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L41). **negative:** `unit/verify` [`TestRFC9514NativeSRv6ReservedCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L59) |
| `RFC9514-5.1-2` | Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt. (§5.1) | MUST | 5.1 - SRv6 Locator TLV | **positive:** `unit/verify` [`TestRFC9514LocatorDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L155). **positive:** `unit/verify` [`TestRFC9514OriginatedReservedWordsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L35). **negative:** `unit/verify` [`TestRFC9514LocatorReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L183). **negative:** `unit/verify` [`TestRFC9514PoisonedSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L45) |
| `RFC9514-6-1` | SRv6 SID Descriptors: Set of SRv6 SID Descriptor TLVs. This field MUST contain a single SRv6 SID Information TLV (§6) | MUST | 6 - SRv6 SID NLRI | **positive:** `unit/verify` [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L42). **negative:** no negative test. **{single-polarity}:** internal/component/bgp/plugins/ls_export/export_encode.go::encodeTopology passes one SRv6SID to ls.NewBGPLSSRv6SID, which writes the one SRv6SID field of SRv6SIDDescriptor as exactly one TLV 518, so no input can yield a second one for a negative case |
| `RFC9514-7.1-1` | The SRv6 Endpoint Behavior TLV is a mandatory TLV that MUST be included in the BGP-LS Attribute associated with the BGP-LS SRv6 SID NLRI. (§7.1) | MUST | 7.1 - SRv6 Endpoint Behavior TLV | **positive:** `unit/verify` [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L43). **negative:** `unit/verify` [`TestRFC9514NativeSIDRequiresEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L68) |
| `RFC9514-7.1-2` | Undefined flags MUST be set to 0 when originating (§7.1) | MUST | 7.1 - SRv6 Endpoint Behavior TLV | **positive:** `unit/verify` [`TestRFC9514ISISEndpointBehaviorFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L136). **negative:** `unit/verify` [`TestRFC9514ISISEndpointBehaviorFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L137) |
| `RFC9514-7.1-4` | Undefined flags MUST be set to 0 when originating and ignored on receipt. (§7.1) | MUST | 7.1 - SRv6 Endpoint Behavior TLV | **positive:** `unit/verify` [`TestRFC9514EndpointBehaviorUndefinedFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L70). **positive:** `unit/verify` [`TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L76). **negative:** `unit/verify` [`TestRFC9514EndpointBehaviorUndefinedFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L71). **negative:** `unit/verify` [`TestRFC9514ISISEndpointBehaviorNativeFlagsNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L85) |
| `RFC9514-7.1-3` | The algorithm value MUST be 0 unless an algorithm is associated locally with the SRv6 Locator from which the SID is allocated. (§7.1) | MUST | 7.1 - SRv6 Endpoint Behavior TLV | **positive:** `unit/verify` [`TestRFC9514ISISEndpointBehaviorAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L154). **negative:** no negative test. **{single-polarity}:** the IS-IS source copies the algorithm of the locator the SID is allocated from into the TLV (internal/plugins/isis/bgpls_export.go::endSID) and reads no other algorithm, so both branches of the rule, 0 with no associated algorithm and the associated one otherwise, are conforming outputs the positive test asserts, and no input exists for a refusal |
| `RFC9514-7.2-1` | The SRv6 BGP PeerNode SID TLV is a mandatory TLV for use in the BGP-LS Attribute for an SRv6 SID NLRI advertised by BGP for the EPE functionality. This TLV MUST be included along with SRv6 SIDs that are associated with the BGP PeerNode or PeerSet functionality. (§7.2) | MUST | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** no positive test. **negative:** no negative test. **{gap}:** the LsSRv6BGPPeerNodeSID decoder and encoder exist but no EPE origination path instantiates or includes the TLV from a live session (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:78, :102, plugin.go:70-71) |
| `RFC9514-7.2-2` | Other bits are reserved for future use and MUST be set to 0 when originated (§7.2) | MUST | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** no positive test. **negative:** no negative test. **{gap}:** no production path originates an SRv6 BGP PeerNode SID TLV 1251, because Ze assigns no SRv6 BGP EPE segments (RFC9514-7.2-1), so no originated Flags octet exists to prove; the BGP-LS exporter masks the Flags of any TLV 1251 a producer hands it to the three defined bits (value[0] &= 0xe0 in internal/component/bgp/plugins/ls_export/export_encode.go clearOriginatedReserved), and the offline nlri/ls struct encoder, which is not on the origination path, writes Flags verbatim (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:170) |
| `RFC9514-7.2-5` | Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt. (§7.2) | MUST | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** `unit/verify` [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L108). **negative:** `unit/verify` [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L109). **{partial}:** tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID accepts reserved flag bits without interpreting them; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent BGPHighJudge 2026-10-02. RFC 9514 Section 7.2: "Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt." Tested selector "ignored on receipt." is supported by TestRFC9514PeerNodeSIDReservedIgnored: well-formed 12-byte TLV values differing only in low five Flags bits (0xe0 versus 0xff) must both decode, preserve Weight/PeerAS/PeerBGPID and retain exact AS 65000. The rejection negative and meaningful-field equality positive cover distinct failures; raw Flags display is not interpretation of reserved bits. Producer internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID is registered for 1251 and reached by AttrTLVsToJSON/offline UPDATE decoding; the propagator intentionally performs no semantic TLV interpretation. Gap selector "MUST be set to 0 when originated" remains unmet: native EPE publishLocked originates MPLS PeerNode 1101 only, no SRv6 EPE assignment/origination. Offline encoding and exporter masking cannot prove that clause. Existing two native receive revert records retained; partial earns zero whole-requirement proof credit. |
| `RFC9514-7.2-3` | Reserved: 2-octet field. The value MUST be set to 0 when originated (§7.2) | MUST | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** no positive test. **negative:** no negative test. **{gap}:** no production path originates an SRv6 BGP PeerNode SID TLV 1251, because Ze assigns no SRv6 BGP EPE segments (RFC9514-7.2-1); the BGP-LS exporter clears the Reserved field of any TLV 1251 a producer hands it (clear(value[2:4]) in internal/component/bgp/plugins/ls_export/export_encode.go clearOriginatedReserved), and the offline nlri/ls struct encoder writes it as 0 (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:172-173) |
| `RFC9514-7.2-6` | Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt. (§7.2) | MUST | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** `unit/verify` [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L121). **negative:** `unit/verify` [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L122). **{partial}:** tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID ignores the Reserved field; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent BGPHighJudge 2026-10-02. RFC 9514 Section 7.2: "Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt." Tested selector "ignored on receipt." is supported by TestRFC9514PeerNodeSIDReservedIgnored: the 12-byte value with Reserved 0xffff is accepted and equals the complete decoded zero-Reserved object, independently detecting rejection and field leakage. Producer internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID skips bytes 2:4; it is registered in the operator-facing AttrTLVsToJSON/offline decoder path. Gap selector "MUST be set to 0 when originated" remains unmet because no live native SRv6 BGP EPE assignment/origination produces TLV 1251. Neither offline WriteTo zeroes nor export masking establish a live originator. Full quote, existing receive tags and both native receive revert records remain; partial is a finding, not enforced conformance. |
| `RFC9514-8-1` | The sum of the LB Length, LN Length, Fun. Length, and Arg. Length MUST be less than or equal to 128. (§8) | MUST | 8 - SRv6 SID Structure TLV | **positive:** `unit/verify` [`TestRFC9514NativeSIDStructureBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L85). **negative:** `unit/verify` [`TestRFC9514NativeSIDStructureBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L86) |
| `RFC9514-11-1` | The isolation of BGP-LS peering sessions is RECOMMENDED to ensure that BGP-LS topology information (including the newly added SR information) is not advertised to an external BGP peering session outside the SR domain. (§11) | SHOULD | 11 - Security Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9514-6-2` | This field MUST contain a single SRv6 SID Information TLV (Section 6.1) and MAY contain the Multi-Topology Identifier TLV [RFC7752]. (§6) | MAY | 6 - SRv6 SID NLRI | **positive:** no positive test. **negative:** no negative test |
| `RFC9514-7.2-4` | When set, the S-Flag indicates that the SID refers to a set of BGP peering sessions (i.e., BGP Peer Set SID functionality) and therefore MAY be assigned to one or more End.X SIDs associated with BGP peering sessions. (§7.2) | MAY | 7.2 - SRv6 BGP Peer Node SID TLV | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9514-7.2-1`](#rfc9514-7.2-1) The SRv6 BGP PeerNode SID TLV is a mandatory TLV for use in the BGP-LS Attribute for an SRv6 SID NLRI advertised by BGP for the EPE functionality. This TLV MUST be included along with SRv6 SIDs that are associated with the BGP PeerNode or PeerSet functionality. (§7.2) | {gap}, no test | the LsSRv6BGPPeerNodeSID decoder and encoder exist but no EPE origination path instantiates or includes the TLV from a live session (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:78, :102, plugin.go:70-71) |
| [`RFC9514-7.2-2`](#rfc9514-7.2-2) Other bits are reserved for future use and MUST be set to 0 when originated (§7.2) | {gap}, no test | no production path originates an SRv6 BGP PeerNode SID TLV 1251, because Ze assigns no SRv6 BGP EPE segments (RFC9514-7.2-1), so no originated Flags octet exists to prove; the BGP-LS exporter masks the Flags of any TLV 1251 a producer hands it to the three defined bits (value[0] &= 0xe0 in internal/component/bgp/plugins/ls_export/export_encode.go clearOriginatedReserved), and the offline nlri/ls struct encoder, which is not on the origination path, writes Flags verbatim (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:170) |
| [`RFC9514-7.2-5`](#rfc9514-7.2-5) Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt. (§7.2) | {partial}, Partial proof; remaining gap | tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID accepts reserved flag bits without interpreting them; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause |
| [`RFC9514-7.2-3`](#rfc9514-7.2-3) Reserved: 2-octet field. The value MUST be set to 0 when originated (§7.2) | {gap}, no test | no production path originates an SRv6 BGP PeerNode SID TLV 1251, because Ze assigns no SRv6 BGP EPE segments (RFC9514-7.2-1); the BGP-LS exporter clears the Reserved field of any TLV 1251 a producer hands it (clear(value[2:4]) in internal/component/bgp/plugins/ls_export/export_encode.go clearOriginatedReserved), and the offline nlri/ls struct encoder writes it as 0 (internal/component/bgp/plugins/nlri/ls/attr_srv6.go:172-173) |
| [`RFC9514-7.2-6`](#rfc9514-7.2-6) Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt. (§7.2) | {partial}, Partial proof; remaining gap | tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID ignores the Reserved field; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9514-3.1-1`](#rfc9514-3.1-1)

A single instance of this TLV MUST be included in the BGP-LS Attribute for each SRv6-capable node. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an SRv6-capable node with no TLV 1038, or with two. TestRFC9514ISISSRv6CapabilitiesSingleInstance positive fails on len(got)!=1 for a node advertising SRv6 Capabilities (absence goes red); negative feeds two native sub-TLVs and fails on len(got)!=1 (a second instance goes red).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514ISISSRv6CapabilitiesSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC9514ISISSRv6CapabilitiesSingleInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L82) | unit/verify | revert, verified |

### [`RFC9514-3.1-2`](#rfc9514-3.1-2)

Reserved: 2-octet field that MUST be set to 0 when originated (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero Reserved word in an originated TLV 1038. TestRFC9514NativeSRv6ReservedCleared feeds source reserved 0xffff and require.Equal on the emitted value {0x40,0,0,0} goes red if any reserved byte leaks; the positive pins the clean encoding with the flags preserved.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514NativeSRv6ReservedCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L40) | unit/verify | revert, verified |

### [`RFC9514-3.1-3`](#rfc9514-3.1-3)

Reserved: 2-octet field that MUST be set to 0 when originated and ignored on receipt. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: receipt clause as before (TestRFC9514CapabilitiesReservedIgnored, exact map). Originate clause now tagged: ls_export TestRFC9514OriginatedReservedWordsZero (+) and TestRFC9514PoisonedSourceReservedNeverOriginated (-: source reserved 0xff, wire TLV 1038 = 40 00 00 00). Judge break (1038 clear removed in clearOriginatedReserved) turns the negative red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514PoisonedSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L44) | unit/verify | revert, verified |
| negative | [`TestRFC9514CapabilitiesReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestRFC9514OriginatedReservedWordsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC9514CapabilitiesDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L131) | unit/verify | revert, verified |

### [`RFC9514-4.1-1`](#rfc9514-4.1-1)

Reserved: 1-octet field that MUST be set to 0 when originated (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero Reserved octet in an originated End.X SID TLV 1106. TestRFC9514ISISEndXReservedZero fails on got[0][5]!=0 for both clean and all-ones native flags/weight.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L107) | unit/verify | revert, verified |

### [`RFC9514-4.1-2`](#rfc9514-4.1-2)

Reserved: 1-octet field that MUST be set to 0 when originated and ignored on receipt. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: receipt clause as before. Originate clause: isis TestRFC9514ISISEndXOriginatedReservedZero (+) and TestRFC9514ISISEndXAllOnesNeverReachReserved (-: native flags and weight 0xff) assert TLV 1106 Reserved octet 0 from adjacencySIDv6. Judge break (Reserved written from weight) turns both red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L44) | unit/verify | mutant, verified |
| negative | [`TestRFC9514ISISEndXAllOnesNeverReachReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L43) | unit/verify | mutant, verified |
| positive | [`TestRFC9514ISISEndXOriginatedReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L42) | unit/verify | revert, verified |

### [`RFC9514-4.2-1`](#rfc9514-4.2-1)

Reserved: 1-octet field that MUST be set to 0 when originated (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero Reserved octet in an originated LAN End.X SID TLV. TestRFC9514ISISEndXReservedZero fails on got[0][5]!=0 for TLV 1107 with clean and all-ones native values. Ze originates no OSPFv3 1108 (no SRv6 code in internal/plugins/ospf).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestRFC9514ISISEndXReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L108) | unit/verify | revert, verified |

### [`RFC9514-4.2-2`](#rfc9514-4.2-2)

Reserved: 1-octet field that MUST be set to 0 when originated and ignored on receipt. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: receipt clause as before. Originate clause: the same isis units assert TLV 1107 Reserved octet 0 from adjacencySIDv6 for ordinary and all-ones native flags and weight. Judge break (Reserved written from weight) turns both red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L46) | unit/verify | mutant, verified |
| negative | [`TestRFC9514ISISEndXAllOnesNeverReachReserved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC9514EndXSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L45) | unit/verify | mutant, verified |
| positive | [`TestRFC9514ISISEndXOriginatedReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L43) | unit/verify | revert, verified |

### [`RFC9514-5.1-1`](#rfc9514-5.1-1)

Reserved: 2-octet field. The value MUST be set to 0 when originated (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero Reserved word in an originated SRv6 Locator TLV 1162. TestRFC9514NativeSRv6ReservedCleared feeds reserved 0xffff and require.Equal on the whole emitted value goes red on a leak or on damage to following fields.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514NativeSRv6ReservedCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L41) | unit/verify | revert, verified |

### [`RFC9514-5.1-2`](#rfc9514-5.1-2)

Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: receipt clause as before (TestRFC9514LocatorReservedIgnored). Originate clause: ls_export units assert TLV 1162 Reserved word 0 with flags, algorithm, metric and sub-TLVs intact, from a zero and a 0xff-poisoned source. Judge break (1162 clear removed) turns the negative red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514PoisonedSourceReservedNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L45) | unit/verify | revert, verified |
| negative | [`TestRFC9514LocatorReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestRFC9514OriginatedReservedWordsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_sentence_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC9514LocatorDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L155) | unit/verify | revert, verified |

### [`RFC9514-6-1`](#rfc9514-6-1)

SRv6 SID Descriptors: Set of SRv6 SID Descriptor TLVs. This field MUST contain a single SRv6 SID Information TLV (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: SRv6 SID Descriptors with zero or several SID Information TLVs. TestRFC9514NativeSRv6Advertisements require.Equal(t, [][]byte{sid[:]}, exportTLVValues(..., 518)) goes red on none or two; the row carries a {single-polarity} marker because the encoder writes one field.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L42) | unit/verify | revert, verified |

### [`RFC9514-7.1-1`](#rfc9514-7.1-1)

The SRv6 Endpoint Behavior TLV is a mandatory TLV that MUST be included in the BGP-LS Attribute associated with the BGP-LS SRv6 SID NLRI. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an SRv6 SID NLRI whose attribute lacks TLV 1250. Positive require.Equal on the 1250 values goes red if absent; TestRFC9514NativeSIDRequiresEndpointBehavior require.Error on a replacement without 1250 and Len(commands,3) go red if it is advertised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514NativeSIDRequiresEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC9514NativeSRv6Advertisements`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L43) | unit/verify | revert, verified |

### [`RFC9514-7.1-2`](#rfc9514-7.1-2)

Undefined flags MUST be set to 0 when originating (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: undefined flag bits set in an originated TLV 1250. TestRFC9514ISISEndpointBehaviorFlagsZero feeds native End SID flags 0xff and bytes.Equal to {0,1,0,0} goes red on any flag bit; IS-IS End SID defines no flag (RFC 9352), so every bit is undefined.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514ISISEndpointBehaviorFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRFC9514ISISEndpointBehaviorFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L136) | unit/verify | revert, verified |

### [`RFC9514-7.1-4`](#rfc9514-7.1-4)

Undefined flags MUST be set to 0 when originating and ignored on receipt. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-09-30: receipt clause as before. Originate clause: isis TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero (+) and ...NativeFlagsNeverOriginated (-: native End flags 0xff) assert TLV 1250 exactly 00 01 00 00 (no flag is defined, so every flag is undefined). Judge break (native flags copied into the Flags octet in endSID) turns the negative red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514EndpointBehaviorUndefinedFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L71) | unit/verify | revert, verified |
| negative | [`TestRFC9514ISISEndpointBehaviorNativeFlagsNeverOriginated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC9514EndpointBehaviorUndefinedFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go#L76) | unit/verify | revert, verified |

### [`RFC9514-7.1-3`](#rfc9514-7.1-3)

The algorithm value MUST be 0 unless an algorithm is associated locally with the SRv6 Locator from which the SID is allocated. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a non-zero algorithm for a SID whose locator has no algorithm, or another algorithm than the locator's. TestRFC9514ISISEndpointBehaviorAlgorithm fails on got[0][3]!=algo for locator algorithm 0 and 1. {single-polarity} marker: both branches are conforming outputs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9514ISISEndpointBehaviorAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc9514_bgpls_export_test.go#L154) | unit/verify | revert, verified |

### [`RFC9514-7.2-1`](#rfc9514-7.2-1)

The SRv6 BGP PeerNode SID TLV is a mandatory TLV for use in the BGP-LS Attribute for an SRv6 SID NLRI advertised by BGP for the EPE functionality. This TLV MUST be included along with SRv6 SIDs that are associated with the BGP PeerNode or PeerSet functionality. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-1, so no unit is bound to it.

### [`RFC9514-7.2-2`](#rfc9514-7.2-2)

Other bits are reserved for future use and MUST be set to 0 when originated (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-2, so no unit is bound to it.

### [`RFC9514-7.2-5`](#rfc9514-7.2-5)

Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt. (§7.2)

Scoped tag-claim records only: tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID accepts reserved flag bits without interpreting them; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent BGPHighJudge 2026-10-02. RFC 9514 Section 7.2: "Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt." Tested selector "ignored on receipt." is supported by TestRFC9514PeerNodeSIDReservedIgnored: well-formed 12-byte TLV values differing only in low five Flags bits (0xe0 versus 0xff) must both decode, preserve Weight/PeerAS/PeerBGPID and retain exact AS 65000. The rejection negative and meaningful-field equality positive cover distinct failures; raw Flags display is not interpretation of reserved bits. Producer internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID is registered for 1251 and reached by AttrTLVsToJSON/offline UPDATE decoding; the propagator intentionally performs no semantic TLV interpretation. Gap selector "MUST be set to 0 when originated" remains unmet: native EPE publishLocked originates MPLS PeerNode 1101 only, no SRv6 EPE assignment/origination. Offline encoding and exporter masking cannot prove that clause. Existing two native receive revert records retained; partial earns zero whole-requirement proof credit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L108) | unit/verify | revert, verified |

### [`RFC9514-7.2-3`](#rfc9514-7.2-3)

Reserved: 2-octet field. The value MUST be set to 0 when originated (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9514-7.2-3, so no unit is bound to it.

### [`RFC9514-7.2-6`](#rfc9514-7.2-6)

Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt. (§7.2)

Scoped tag-claim records only: tested "ignored on receipt."; gap "MUST be set to 0 when originated"; internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID ignores the Reserved field; no live SRv6 BGP EPE segment-assignment path originates TLV 1251, so offline encoding and export masking do not prove the origination clause; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Independent BGPHighJudge 2026-10-02. RFC 9514 Section 7.2: "Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt." Tested selector "ignored on receipt." is supported by TestRFC9514PeerNodeSIDReservedIgnored: the 12-byte value with Reserved 0xffff is accepted and equals the complete decoded zero-Reserved object, independently detecting rejection and field leakage. Producer internal/component/bgp/plugins/nlri/ls/attr_srv6.go::decodeSRv6BGPPeerNodeSID skips bytes 2:4; it is registered in the operator-facing AttrTLVsToJSON/offline decoder path. Gap selector "MUST be set to 0 when originated" remains unmet because no live native SRv6 BGP EPE assignment/origination produces TLV 1251. Neither offline WriteTo zeroes nor export masking establish a live originator. Full quote, existing receive tags and both native receive revert records remain; partial is a finding, not enforced conformance.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC9514PeerNodeSIDReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/ls/rfc9514_reserved_test.go#L121) | unit/verify | revert, verified |

### [`RFC9514-8-1`](#rfc9514-8-1)

The sum of the LB Length, LN Length, Fun. Length, and Arg. Length MUST be less than or equal to 128. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising a SID Structure whose four lengths sum above 128. TestRFC9514NativeSIDStructureBoundary accepts 32+32+16+48=128 (require.Equal on the emitted 1252) and require.Error plus Len(commands,3) go red if 129 is accepted or advertised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9514NativeSIDStructureBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L86) | unit/verify | revert, verified |
| positive | [`TestRFC9514NativeSIDStructureBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/ls_export/rfc9514_export_test.go#L85) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9514.txt |
| Source fingerprint | 9d86d7c4371f0240 |
| Record | rfc/extraction/rfc9514.json |
| Mapped sentences | 13 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | SRv6 Capabilities TLV | 2 | walked | SRv6 Capabilities TLV. Two sites: the single-instance inclusion MUST (3.1-1) and the Reserved field (3.1-2). Each reserved-field sentence in this document states two obligations, "MUST be set to 0 when originated and ignored on receipt", and the MUST-level scan raises one site for the pair. The originate-as-zero half is mapped below. The ignore-on-receipt half shares that same site and is declared unsourced here, because it is the half a decode-only implementation is the entry point for. RFC9514-3.1-3 carries it. |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | SRv6 End.X SID TLV | 1 | walked | SRv6 End.X SID TLV. One site, the Reserved field, mapped to 4.1-1. Each reserved-field sentence in this document states two obligations, "MUST be set to 0 when originated and ignored on receipt", and the MUST-level scan raises one site for the pair. The originate-as-zero half is mapped below. The ignore-on-receipt half shares that same site and is declared unsourced here, because it is the half a decode-only implementation is the entry point for. RFC9514-4.1-2 carries it. |
| `4.2` | SRv6 LAN End.X SID TLV | 1 | walked | SRv6 LAN End.X SID TLV. One site, the Reserved field, mapped to 4.2-1. Each reserved-field sentence in this document states two obligations, "MUST be set to 0 when originated and ignored on receipt", and the MUST-level scan raises one site for the pair. The originate-as-zero half is mapped below. The ignore-on-receipt half shares that same site and is declared unsourced here, because it is the half a decode-only implementation is the entry point for. RFC9514-4.2-2 carries it. |
| `4.3` | SRv6 Locator TLV reference in the link attribute set | 0 | walked | SRv6 Locator TLV reference in the link attribute set. No 2119 keyword and no obligation of its own. |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | SRv6 Locator TLV | 1 | walked | SRv6 Locator TLV. One site, the Reserved field, mapped to 5.1-1. Each reserved-field sentence in this document states two obligations, "MUST be set to 0 when originated and ignored on receipt", and the MUST-level scan raises one site for the pair. The originate-as-zero half is mapped below. The ignore-on-receipt half shares that same site and is declared unsourced here, because it is the half a decode-only implementation is the entry point for. RFC9514-5.1-2 carries it. |
| `6` | SRv6 SID NLRI | 1 | walked | SRv6 SID NLRI. One site carrying two levels: the MUST that the SRv6 SID Descriptors field contain a single SRv6 SID Information TLV (mapped to 6-1) and the MAY that it contain the Multi-Topology Identifier TLV of RFC 7752, declared unsourced as 6-2. |
| `6.1` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | SRv6 Endpoint Behavior TLV | 3 | walked | SRv6 Endpoint Behavior TLV. Three sites: the mandatory inclusion of the TLV in the BGP-LS Attribute of the SRv6 SID NLRI (7.1-1), the algorithm value of 0 unless an algorithm is associated locally with the SRv6 Locator (7.1-3), and undefined flags (7.1-2). Each reserved-field sentence in this document states two obligations, "MUST be set to 0 when originated and ignored on receipt", and the MUST-level scan raises one site for the pair. The originate-as-zero half is mapped below. The ignore-on-receipt half shares that same site and is declared unsourced here, because it is the half a decode-only implementation is the entry point for. RFC9514-7.1-4 carries it for the undefined flags. |
| `7.2` | SRv6 BGP Peer Node SID TLV | 3 | walked | SRv6 BGP Peer Node SID TLV. Three sites: the inclusion MUST for SIDs associated with BGP PeerNode or PeerSet functionality (7.2-1), the bits reserved for future use in the Flags field (7.2-2), and the Reserved field (7.2-3). Each reserved-field sentence states origination and receipt obligations, and the MUST-level scan raises one site for the pair. Sites 7.2:2 and 7.2:3 remain mapped to the origination rows 7.2-2 and 7.2-3. Unsourced rows RFC9514-7.2-5 and RFC9514-7.2-6 retain the complete sentences, with proof scoped to receipt and an explicit origination gap; they do not quote only the receipt half. Owner decision 10 and its 2026-10-02 annotation-plus-partial-verdict approval authorize this representation. This explanation correction changes no site, mapping, exclusion or requirement identity and claims no new extraction walk. The MAY that a PeerSet SID be assigned to one or more End.X SIDs carries no MUST-level site and is declared unsourced as 7.2-4. |
| `8` | SRv6 SID Structure TLV | 1 | walked | SRv6 SID Structure TLV. One site: the sum of LB Length, LN Length, Function Length and Argument Length MUST be at most 128, mapped to 8-1. |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | Security Considerations | 1 | walked | Security Considerations. No MUST-level site. The RECOMMENDED isolation of BGP-LS peering sessions, so that topology information is not advertised outside the SR domain, is declared unsourced as 11-1. |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC boilerplate: the Trust Legal Provisions copyright notice that opens every RFC. Its 'must' addresses whoever extracts Code Components from the document and states a licensing condition, not a protocol behaviour. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose describing what the extension allows: consumer applications 'receive the SRv6 SIDs from nodes across an IGP domain or even across Autonomous Systems (ASes) as required'. The words 'as required' qualify the applications' own need and direct no speaker. | On similar lines, introducing the SRv6-related information in BGP-LS allows consumer applications that require topological visibility to also receive the SRv6 SIDs from nodes across an IGP domain or even across Autonomous Systems (ASes) as required. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Motivation prose in the subjunctive, explaining why a design that was NOT chosen would have been worse: 'If the SRv6 SIDs had been advertised within the BGP-LS Link Attribute ... the BGP-LS update would have grown rather large'. It describes a rejected alternative and states no obligation. | If the SRv6 SIDs had been advertised within the BGP- LS Link Attribute associated with the existing Node NLRI, the BGP-LS update would have grown rather large with the increase in SRv6 SIDs on the node and would have also required a large update message to be generated for any change, even a change to a single SRv6 SID. |
| `11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An assumption recorded in the Security Considerations, not an obligation: 'The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC9352] and [RFC9513]).' The mechanisms it names are obligations of RFC 9352 and RFC 9513 on the IGP, and this sentence assumes rather than imposes them. | The IGP instances originating these TLVs are assumed to support all the required security and authentication mechanisms (as described in [RFC9352] and [RFC9513]). |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix prose comparing this document's encoding with SR-MPLS: 'In the case of SR-MPLS, an additional Link NLRI is required to be advertised corresponding to each BGP peering session on the node.' It describes what RFC 9086 does, not what an implementation of this document owes. | In the case of SR-MPLS, an additional Link NLRI is required to be advertised corresponding to each BGP peering session on the node. |

## Superseded

No document obsoletes RFC 9514, so its obligations are stated where they were written.
