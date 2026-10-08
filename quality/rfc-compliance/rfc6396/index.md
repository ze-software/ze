# RFC 6396 - Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format

Supported. Every requirement this repository extracted from RFC 6396, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 15.4% | 2 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 38.5% | 5 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 68.4% | 13 of 19 tagged units, 0 escaped and 1 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 38.5% | 5 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.7% | 1 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 7 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc6396.md` |
| Requirement shard | `rfc/requirements/rfc6396.md` |
| RFC text | `rfc/full/rfc6396.txt` |

## Enrolment

Enrolled: MRT Routing Information Export Format (RFC 6396): both-way encoder/decoder; 7 single-polarity positive + 1 gap (BGP4MP_MESSAGE_AS4 mislabels 2-byte-peer AS_PATH) + 5 not-applicable

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Daemon-side MRT recording, TABLE_DUMP_V2 snapshots, BGP4MP messages, analysis tools.

**What the ledger says remains**

One MUST gap gated in [`rfc/short/rfc6396.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6396.md) [[`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1)]: the live BGP4MP writer always emits the BGP4MP_MESSAGE_AS4 subtype and records the on-wire message verbatim without checking the session's negotiated 4-byte-AS capability, so a message from an OLD (2-byte) peer carries a 2-byte AS_PATH mislabeled as AS4. RIB-path AS_PATH is unaffected (canonicalized to 4-byte).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC6396-4.3.1-2`](#rfc6396-4.3.1-2), [`RFC6396-4.3.4-1`](#rfc6396-4.3.4-1)

**Annotated (including scoped evidence) (11):** [`RFC6396-4.2-1`](#rfc6396-4.2-1), [`RFC6396-4.2-2`](#rfc6396-4.2-2), [`RFC6396-4.3.1-1`](#rfc6396-4.3.1-1), [`RFC6396-4.3.1-3`](#rfc6396-4.3.1-3), [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2), [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1), [`RFC6396-4.4.2-2`](#rfc6396-4.4.2-2), [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1), [`RFC6396-1-1`](#rfc6396-1-1), [`RFC6396-5.1-1`](#rfc6396-5.1-1), [`RFC6396-B.1-1`](#rfc6396-b.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6396-4.2-1` | The AS_PATH attribute MUST only consist of 2-byte AS numbers. (§4.2) | MUST | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's RIB export always emits TABLE_DUMP_V2 (the Section 4.2 mandate) and never operationally writes TABLE_DUMP, so the 2-byte-AS_PATH-in-TABLE_DUMP obligation binds a writer role ze does not play (internal/mrt/encode.go:185 has no production caller) |
| `RFC6396-4.2-2` | The TABLE_DUMP Type does not permit 4-byte Peer AS numbers, nor does it allow the AFI of the peer IP to differ from the AFI of the Prefix field.  The TABLE_DUMP_V2 Type MUST be used in these situations. (§4.2) | MUST | 4.2 - TABLE_DUMP Type | **positive:** `unit/verify` [`TestDumpV2PeerWidthAndPrefixFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_width_test.go#L20). **positive:** `unit/verify` [`TestRibSubtype`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L17). **negative:** no negative test. **{single-polarity}:** ze's RIB dumper unconditionally emits TABLE_DUMP_V2 with AS4 peer entries, so it always satisfies the use-V2 mandate and there is no path that emits TABLE_DUMP to reject (internal/plugins/mrt/dump.go:113, :206) |
| `RFC6396-4.3.1-1` | The View Name is OPTIONAL and, if not present, the View Name Length MUST be set to 0. (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L165). **negative:** no negative test. **{single-polarity}:** the encoder writes View Name Length as len(viewName) and the producer always passes an empty name, so the length is 0 by construction (internal/mrt/encode.go:48, internal/plugins/mrt/dump.go:199) |
| `RFC6396-4.3.1-2` | The View Name encoding MUST follow the UTF-8 transformation format [RFC3629]. (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L170). **positive:** `unit/verify` [`TestRFC6396DumpProducerEmitsNoViewName`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_view_name_test.go#L18). **positive:** `unit/verify` [`TestRFC6396ViewNameUTF8`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L98). **negative:** `unit/verify` [`TestRFC6396ViewNameUTF8`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L99) |
| `RFC6396-4.3.1-3` | The RIB entry MRT records MUST immediately follow the PEER_INDEX_TABLE MRT record. (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestDumpV2PeerIndexBeforeFirstRIBEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L213). **negative:** no negative test. **{single-polarity}:** the RIB dump writes the PEER_INDEX_TABLE on the first OnRoute callback, before that route's RIB entry and before any other RIB record, guaranteeing the ordering (internal/plugins/mrt/dump.go:149-152) |
| `RFC6396-4.3.4-1` | All AS numbers in the AS_PATH attribute MUST be encoded as 4-byte AS numbers. (§4.3.4) | MUST | 4.3.4 - RIB Entries | **positive:** `unit/verify` [`TestDumpV2RIBEntryASPathIs4Byte`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L244). **positive:** `unit/verify` [`TestRFC6396RIBEntryASPathStoredFourByte`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L596). **positive:** `unit/verify` [`TestRFC6396TwoByteSessionRouteDumpsFourByteASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc6396_mrt_aspath_test.go#L99). **negative:** `unit/verify` [`TestRFC6396FourByteSessionRouteDumpsASPathUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc6396_mrt_aspath_test.go#L115). **negative:** `unit/verify` [`TestRFC6396RIBEntryASPathFourByteSessionUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L624) |
| `RFC6396-4.3.4-2` | There is one exception to the encoding of BGP attributes for the BGP MP_REACH_NLRI attribute (BGP Type Code 14) [RFC4760].  Since the AFI, SAFI, and NLRI information is already encoded in the RIB Entry Header or RIB_GENERIC Entry Header, only the Next Hop Address Length and Next Hop Address fields are included.  The Reserved field is omitted. (§4.3.4) | MUST | 4.3.4 - RIB Entries | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TABLE_DUMP_V2 RIB writer reconstructs a NEXT_HOP (type 3) attribute and never emits an MP_REACH_NLRI attribute in RIB entries, so the abbreviation obligation never binds ze's producer (internal/component/bgp/plugins/rib/rib_mrt.go:127-153) |
| `RFC6396-4.4.2-1` | The BGP4MP_MESSAGE Subtype does not support 4-byte AS numbers.  The AS_PATH contained in these messages MUST only consist of 2-byte AS numbers. (§4.4.2) | MUST | 4.4.2 - BGP4MP_MESSAGE Subtype | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's live capture unconditionally emits the AS4 BGP4MP message subtype and never writes the 2-byte BGP4MP_MESSAGE (subtype 1), so the 2-byte-AS_PATH obligation binds a writer variant ze does not produce (internal/plugins/mrt/dump.go:240-250) |
| `RFC6396-4.4.2-2` | Only one BGP message SHALL be encoded in the BGP4MP_MESSAGE Subtype. (§4.4.2) | MUST | 4.4.2 - BGP4MP_MESSAGE Subtype | **positive:** `unit/verify` [`TestOneBGPMessagePerBGP4MPRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_component_test.go#L84). **positive:** `unit/verify` [`TestRFC6396WireCaptureIgnoresSemanticSynthesis`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6396_wire_capture_test.go#L48). **positive:** `unit/verify` [`TestRFC6396WireMessagesNeverCoalesce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6396_wire_capture_test.go#L91). **negative:** no negative test. **{single-polarity}:** OnBGPMessage is invoked once per BGP message and writes exactly one message into each BGP4MP record, so records always carry a single message (internal/plugins/mrt/component.go:99-142) |
| `RFC6396-4.4.3-1` | The AS_PATH in these messages MUST only consist of 4-byte AS numbers. (§4.4.3) | MUST | 4.4.3 - BGP4MP_MESSAGE_AS4 Subtype | **positive:** no positive test. **negative:** no negative test. **{gap}:** the live writer hardcodes the AS4 subtype and copies the on-wire message verbatim without checking negotiated AS4 capability, so a 2-byte (OLD-peer) session's 2-byte AS_PATH is mislabeled as AS4 (internal/plugins/mrt/dump.go:240-250, component.go:123; ze supports 2-byte sessions per internal/component/bgp/plugins/rib/storage/attrparse.go:18-24) |
| `RFC6396-1-1` | Fields which contain multi-octet numeric values are encoded in network octet order from most significant octet to least significant octet. (§1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestCommonHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L24). **positive:** `unit/verify` [`TestRFC6396NumericFieldsUseExternalNetworkOrder`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L14). **positive:** `unit/verify` [`TestRFC6396RemainingNumericFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_remaining_fields_test.go#L13). **negative:** no negative test. **{single-polarity}:** encoders have no alternate-endianness mode to refuse. TestRFC6396NumericFieldsUseExternalNetworkOrder asserts exact external octets for PEER_INDEX_TABLE, RIB and BGP4MP fields and decodes independent literal input; the common-header test separately pins its four numeric fields |
| `RFC6396-5.1-1` | New Type Codes MUST be allocated starting at 65. (§5.1) | MUST | 5.1 - Type Codes | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA registry allocation policy binding specification and registry authors; ze does not allocate MRT type codes |
| `RFC6396-B.1-1` | The message string encoding MUST follow the UTF-8 transformation format [RFC3629]. (§B.1) | MUST | B.1 - Deprecated MRT Informational Types | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not define or produce the deprecated informational types (codes 0-4); its type table starts at OSPFv2 (11) (internal/mrt/types.go:6-16) |
| `RFC6396-4.2-3` | It is RECOMMENDED that new MRT encoding implementations use the TABLE_DUMP_V2 Type (see below) instead of the TABLE_DUMP Type due to limitations in this type. (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-4` | A typical RIB dump will exceed the 16-bit bounds of this counter, and an implementation SHOULD simply wrap back to zero and continue incrementing the counter in such cases. (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-5` | The Status octet is unused in the TABLE_DUMP Type and SHOULD be set to 1. (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.3.3-1` | An implementation that does not recognize particular AFI and SAFI values SHOULD discard the remainder of the MRT record. (§4.3.3) | SHOULD | 4.3.3 - RIB_GENERIC Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.1-2` | The Subtype field is unused for these Types and SHOULD be set to 0. (§B.1) | SHOULD | B.1 - Deprecated MRT Informational Types | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.1.3-1` | The DIE Type signals a remote MRT repository that it SHOULD stop accepting messages. (§B.1.3) | SHOULD | B.1.3 - DIE Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.2.1.5-1` | There are no known implementations of this subtype, and it SHOULD be ignored. (§B.2.1.5) | SHOULD | B.2.1.5 - BGP_SYNC Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.2.2-1` | The Subtype field is currently reserved for this type and SHOULD be set to 0. (§B.2) | SHOULD | B.2 - Other Deprecated MRT Types | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-6` | However, due to the significant volume of historical data encoded with this type, MRT decoding applications MAY wish to support this type. (§4.2) | MAY | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-7` | In cases where multiple RIB views are present, an implementation MAY use the View Number field to distinguish entries from each view. (§4.2) | MAY | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.4.1-1` | In some cases, the Peer AS Number may be undefined.  In such cases, the value of this field MAY be set to zero. (§4.4.1) | MAY | 4.4.1 - BGP4MP_STATE_CHANGE Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.4.1-2` | The index value is OPTIONAL and MAY be zero if unknown or unsupported. (§4.4) | MAY | 4.4 - BGP4MP Type | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6396-4.2-1`](#rfc6396-4.2-1) The AS_PATH attribute MUST only consist of 2-byte AS numbers. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's RIB export always emits TABLE_DUMP_V2 (the Section 4.2 mandate) and never operationally writes TABLE_DUMP, so the 2-byte-AS_PATH-in-TABLE_DUMP obligation binds a writer role ze does not play (internal/mrt/encode.go:185 has no production caller) |
| [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2) There is one exception to the encoding of BGP attributes for the BGP MP_REACH_NLRI attribute (BGP Type Code 14) [RFC4760].  Since the AFI, SAFI, and NLRI information is already encoded in the RIB Entry Header or RIB_GENERIC Entry Header, only the Next Hop Address Length and Next Hop Address fields are included.  The Reserved field is omitted. (§4.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TABLE_DUMP_V2 RIB writer reconstructs a NEXT_HOP (type 3) attribute and never emits an MP_REACH_NLRI attribute in RIB entries, so the abbreviation obligation never binds ze's producer (internal/component/bgp/plugins/rib/rib_mrt.go:127-153) |
| [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1) The BGP4MP_MESSAGE Subtype does not support 4-byte AS numbers.  The AS_PATH contained in these messages MUST only consist of 2-byte AS numbers. (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's live capture unconditionally emits the AS4 BGP4MP message subtype and never writes the 2-byte BGP4MP_MESSAGE (subtype 1), so the 2-byte-AS_PATH obligation binds a writer variant ze does not produce (internal/plugins/mrt/dump.go:240-250) |
| [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1) The AS_PATH in these messages MUST only consist of 4-byte AS numbers. (§4.4.3) | {gap}, no test | the live writer hardcodes the AS4 subtype and copies the on-wire message verbatim without checking negotiated AS4 capability, so a 2-byte (OLD-peer) session's 2-byte AS_PATH is mislabeled as AS4 (internal/plugins/mrt/dump.go:240-250, component.go:123; ze supports 2-byte sessions per internal/component/bgp/plugins/rib/storage/attrparse.go:18-24) |
| [`RFC6396-5.1-1`](#rfc6396-5.1-1) New Type Codes MUST be allocated starting at 65. (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA registry allocation policy binding specification and registry authors; ze does not allocate MRT type codes |
| [`RFC6396-B.1-1`](#rfc6396-b.1-1) The message string encoding MUST follow the UTF-8 transformation format [RFC3629]. (§B.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not define or produce the deprecated informational types (codes 0-4); its type table starts at OSPFv2 (11) (internal/mrt/types.go:6-16) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6396-4.2-1`](#rfc6396-4.2-1)

The AS_PATH attribute MUST only consist of 2-byte AS numbers. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.2-1, so no unit is bound to it.

### [`RFC6396-4.2-2`](#rfc6396-4.2-2)

The TABLE_DUMP Type does not permit 4-byte Peer AS numbers, nor does it allow the AFI of the peer IP to differ from the AFI of the Prefix field.  The TABLE_DUMP_V2 Type MUST be used in these situations. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent supplemental source rejudgment, 2026-10-06. RFC6396 Section 4.2: "The TABLE_DUMP Type does not permit 4-byte Peer AS numbers, nor does it allow the AFI of the peer IP to differ from the AFI of the Prefix field. The TABLE_DUMP_V2 Type MUST be used in these situations." Read both current carriers, TestRibSubtype and TestDumpV2PeerWidthAndPrefixFamily. The latter drives actual Component.writeTableDumpV2/writePeerIndexTable and Writer/ReadFile using eight isolated combinations of peer IPv4/IPv6, prefix IPv4/IPv6 and ASN 65001/0x01020304. Every record header must be TypeTableDumpV2 before reader dispatch; both PIT and RIB counts must be one, with exact subtype, PIT width/address/ASN/BGP ID and RIB sequence/prefix/peer index/attributes. Thus subtype-only TestRibSubtype is supplementary, not mistaken for proving the Type. Ordinary same-family cases are genuine controls, and each mandatory V2 trigger is also isolated. The single-positive annotation is justified by the operational writer unconditionally emitting V2, not by inventing a legacy rejection obligation. Traced dumpRIB -> writeTableDumpV2 -> PIT/RIB writeHeader and reader readRecords -> dispatch TypeTableDumpV2 -> dispatchTDV2 -> non-nil PIT/RIB callbacks. TestDumpV2PeerWidthAndPrefixFamily has no current diff against HEAD; history includes 67a74ef4d4 and the integrated checkpoint, so its stale-unit is preexisting judgment debt rather than this mechanical edit. Prior producer keys retained with empty values for main to stamp. Existing prior note reports race, mutation and CLI observations; this source rejudgment neither reruns nor independently attests them. No new defect found for this requirement; no tests or canonical writes executed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRibSubtype`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L17) | unit/verify | unproven |
| positive | [`TestDumpV2PeerWidthAndPrefixFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_width_test.go#L20) | unit/verify | revert, verified |

### [`RFC6396-4.3.1-1`](#rfc6396-4.3.1-1)

The View Name is OPTIONAL and, if not present, the View Name Length MUST be set to 0. (§4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent MRTJudge 2026-10-02, rejudged changed checked-encoder call. RFC6396 section 4.3.1: The View Name is OPTIONAL and, if not present, the View Name Length MUST be set to 0. TestPeerIndexTableEmptyViewName asserts the actual encoded two octets at offset 4 are zero and decoded name empty; errors from the migrated encoder are checked. The production writePeerIndexTable passes an empty string; the additional dump-producer test inspected under 4.3.1-2 pins the entire emitted empty-name PIT body. Single-positive annotation is valid because no independently supplied absent-name length exists. Native record disables WritePeerIndexTable; enforcement rests on the zero-field assertion, not the panic.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L165) | unit/verify | revert, verified |

### [`RFC6396-4.3.1-2`](#rfc6396-4.3.1-2)

The View Name encoding MUST follow the UTF-8 transformation format [RFC3629]. (§4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent MRTJudge 2026-10-02. RFC6396 section 4.3.1: The View Name encoding MUST follow the UTF-8 transformation format [RFC3629]. TestRFC6396ViewNameUTF8 positively pins nonempty ASCII and multibyte UTF-8 literal octets and byte length on encode and independent decode; negative inputs isolate illegal leading byte, overlong form, surrogate, out-of-range code point and truncated sequence, requiring encoder error/zero count/untouched buffer and decoder error. Removing either utf8 validation fails the relevant negative assertions. TestRFC6396DumpProducerEmitsNoViewName reaches writeTableDumpV2/writePeerIndexTable and compares exact full PIT body, so invalid production name insertion cannot hide behind the old vacuous empty-string test. Retained empty-name tagged test is supplementary. Native records for both codec polarities and actual producer are present; their panic breaks prove reachability only, while the inspected assertions establish this verdict.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6396ViewNameUTF8`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestRFC6396ViewNameUTF8`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC6396DumpProducerEmitsNoViewName`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_view_name_test.go#L18) | unit/verify | revert, verified |

### [`RFC6396-4.3.1-3`](#rfc6396-4.3.1-3)

The RIB entry MRT records MUST immediately follow the PEER_INDEX_TABLE MRT record. (§4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a RIB entry record written before the PEER_INDEX_TABLE, or another record between them. internal/plugins/mrt/dump_test.go::TestDumpV2PeerIndexBeforeFirstRIBEntry drives the real writeTableDumpV2 path and asserts order[0] is (TABLE_DUMP_V2, PEER_INDEX_TABLE) and order[1] is (TABLE_DUMP_V2, RIB_IPV4_UNICAST); a late or missing PEER_INDEX_TABLE or an interleaved record turns it red. Single-polarity positive marker on the row. It drives one route only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDumpV2PeerIndexBeforeFirstRIBEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L213) | unit/verify | unproven |

### [`RFC6396-4.3.4-1`](#rfc6396-4.3.4-1)

All AS numbers in the AS_PATH attribute MUST be encoded as 4-byte AS numbers. (§4.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. rib TestRFC6396TwoByteSessionRouteDumpsFourByteASPath drives a 2-byte-session UPDATE (AS_SEQ 65001 65002, 2-octet) through wireu.CollapseAS4Family, the RIB received-UPDATE entry (context stays ASN4=false), dumpRIBForMRT, the internal/mrt RIB writers and DecodeRIBRecord: the entry AS_PATH is 4-octet. Negative TestRFC6396FourByteSessionRouteDumpsASPathUnchanged: a 4-byte session path with AS 200000 is written octet-equal (no second widening, no narrowing). mrt TestDumpV2RIBEntryASPathIs4Byte (claim reworded to what it asserts) proves writeTableDumpV2 keeps a 4-byte path. Records observed red on revert of CollapseAS4Family, reconstructWireAttrs and writeTableDumpV2. Limit: the rib test composes the record with the same internal/mrt writers the MRT plugin calls rather than the plugin itself; the plugin half is the mrt unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6396FourByteSessionRouteDumpsASPathUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc6396_mrt_aspath_test.go#L115) | unit/verify | revert, verified |
| negative | [`TestRFC6396RIBEntryASPathFourByteSessionUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L624) | unit/verify | unproven |
| positive | [`TestRFC6396TwoByteSessionRouteDumpsFourByteASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc6396_mrt_aspath_test.go#L99) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC6396RIBEntryASPathStoredFourByte`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc6793_reconcile_test.go#L596) | unit/verify | unproven |
| positive | [`TestDumpV2RIBEntryASPathIs4Byte`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_dump_test.go#L244) | unit/verify | revert, verified |

### [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2)

There is one exception to the encoding of BGP attributes for the BGP MP_REACH_NLRI attribute (BGP Type Code 14) [RFC4760].  Since the AFI, SAFI, and NLRI information is already encoded in the RIB Entry Header or RIB_GENERIC Entry Header, only the Next Hop Address Length and Next Hop Address fields are included.  The Reserved field is omitted. (§4.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.3.4-2, so no unit is bound to it.

### [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1)

The BGP4MP_MESSAGE Subtype does not support 4-byte AS numbers.  The AS_PATH contained in these messages MUST only consist of 2-byte AS numbers. (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.4.2-1, so no unit is bound to it.

### [`RFC6396-4.4.2-2`](#rfc6396-4.4.2-2)

Only one BGP message SHALL be encoded in the BGP4MP_MESSAGE Subtype. (§4.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 6396 §4.4.2: "Only one BGP message SHALL be encoded in the BGP4MP_MESSAGE Subtype." In the surrounding paragraph the subtype is transparent to the actual message; the earlier paragraph includes marker, length and type. All three current tagged carriers now discriminate original-message boundaries: internal/plugins/mrt/rfc6396_component_test.go::TestOneBGPMessagePerBGP4MPRecord sends KEEPALIVE/unequal-length UPDATE/KEEPALIVE and demands exactly three byte-identical records; internal/component/bgp/reactor/rfc6396_wire_capture_test.go::TestRFC6396WireCaptureIgnoresSemanticSynthesis requires exactly one original packet despite two synthetic withdrawals or session reset; TestRFC6396WireMessagesNeverCoalesce requires the two unequal original packets while semantic delivery coalesces. Producers session_wire.go::observeReceivedWire and internal/plugins/mrt/component.go::OnBGPMessage observe/copy the complete original frame, not reconstructed semantic callbacks; internal/mrt/encode.go::WriteBGP4MPMessage writes one frame. Exact counts and bytes would reject merging, splitting, reconstruction or loss. The declared single positive polarity is justified for a producer cardinality invariant; these are not malformed-input rejection claims. This source judgment closes the prior one-call-only proof gap, not the separate AS4-width gap, nor a claim that runtime renewal has run. Post-lint rejudgment 2026-10-04: TestRFC6396WireMessagesNeverCoalesce retains unequal original frames, exact two-element byte equality and one semantic delivery; checked pipe closes cannot weaken those assertions. dispatchObservedWire now borrows an immutable seven-field epoch identity from Peer.runOnce and constructs call-local metadata before applying context ASN and actual transport/local-AS overrides. The changed cardinality carriers use recordSessionWire -> notifyWireMessage directly, not dispatchObservedWire; they must not claim proof of epoch identity. The untagged directional OPEN, migration and collision tests remain separate evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC6396WireCaptureIgnoresSemanticSynthesis`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6396_wire_capture_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC6396WireMessagesNeverCoalesce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6396_wire_capture_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestOneBGPMessagePerBGP4MPRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6396_component_test.go#L84) | unit/verify | revert, verified |

### [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1)

The AS_PATH in these messages MUST only consist of 4-byte AS numbers. (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.4.3-1, so no unit is bound to it.

### [`RFC6396-1-1`](#rfc6396-1-1)

Fields which contain multi-octet numeric values are encoded in network octet order from most significant octet to least significant octet. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent final source rejudgment after formatting. RFC 6396 section 1: "Fields which contain multi-octet numeric values are encoded in network octet order from most significant octet to least significant octet." The following sentence separately preserves routing-message field order; this row is the numeric-order sentence, not an assertion of every encapsulated protocol's semantics. Q1 yes: all three tagged units assert external network order rather than accepting a mutually wrong encoder/decoder roundtrip. internal/mrt/mrt_test.go::TestCommonHeaderRoundTrip checks the exact timestamp/type/subtype/length octets before decoding them. internal/mrt/rfc6396_wire_fields_test.go::TestRFC6396NumericFieldsUseExternalNetworkOrder independently constructs PIT name-length/count/AS2/AS4, RIB sequence/count/index/time/attribute-length and BGP4MP AS2/AS4/interface/AFI octets, compares writer output, then decodes the literals. internal/mrt/rfc6396_remaining_fields_test.go::TestRFC6396RemainingNumericFields independently pins ET microseconds, both FSM state fields and AS widths, RIB_GENERIC sequence/AFI, and TABLE_DUMP view/sequence/time/ASN/attribute-length under both address widths. Q2 yes by source: asymmetric values and exact lengths/bytes/decoded values reject endian reversal, missing fields and offset shifts; formatted multi-line conditionals retain their failures. No runtime execution or renewed discrimination is claimed. Q3 yes for numeric layout: independent expected buffers and structural lengths reach the intended fields. Opaque repeated attribute bytes are not evidence of BGP attribute validity and are not parsed as such by these MRT envelope readers. Q4 yes across the implemented base-RFC numeric envelope fields; the three carriers are complementary, not each whole-scope alone. RFC 8050 Path Identifiers and RFC 6397 geolocation are separately specified extensions. Read internal/mrt/encode.go numeric writers and internal/mrt/decode.go readers: their 16/32-bit numeric operations are big-endian, while address and message octets are copied unchanged. The declared single positive polarity is appropriate to an encoder layout invariant without an alternate-endianness mode to reject. This judgment does not erase the separate RFC6396-4.4.3-1 AS_PATH-width gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCommonHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L24) | unit/verify | unproven |
| positive | [`TestRFC6396RemainingNumericFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_remaining_fields_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC6396NumericFieldsUseExternalNetworkOrder`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc6396_wire_fields_test.go#L14) | unit/verify | revert, verified |

### [`RFC6396-5.1-1`](#rfc6396-5.1-1)

New Type Codes MUST be allocated starting at 65. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-5.1-1, so no unit is bound to it.

### [`RFC6396-B.1-1`](#rfc6396-b.1-1)

The message string encoding MUST follow the UTF-8 transformation format [RFC3629]. (§B.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-B.1-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc6396 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc6396.txt |
| Source fingerprint | e2d04c91ecd5e7f4 |
| Record | rfc/extraction/rfc6396.json |
| Mapped sentences | 11 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Its one site is the IETF Trust Legal Provisions paragraph, excluded below. |
| `1` | Introduction | 0 | walked | Introduction. Indicative history of the MRT format, the projects that extended it, and the note that codes 0 through 10 are deprecated and documented in Appendix B. Its closing paragraph states the byte order, "Fields which contain multi-octet numeric values are encoded in network octet order from most significant octet to least significant octet", with no modal verb at all, so the site scan sees nothing to classify. The summary reads that sentence as a gated obligation, which is the unsourced id below. |
| `1.1` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph. It binds no writer or reader and the derivation excludes it from the site inventory. |
| `2` | MRT Common Header | 0 | walked | MRT Common Header. Field definitions for Timestamp, Type, Subtype, Length and Message, all indicative, plus the note that a post-2038 implementation will need an alternate epoch whose mechanism is out of scope. No modal verb in any case appears, so the prose scan derives no site. The field widths are carried by the Wire Formats table of rfc/short/rfc6396.md. |
| `3` | Extended Timestamp MRT Header | 0 | walked | Extended Timestamp MRT Header. Defines the Microsecond Timestamp field, where it sits, and that it counts toward the Length. Indicative throughout, with no modal verb in any case. |
| `4` | MRT Types | 0 | walked | MRT Types. The Type Code list (11 OSPFv2 through 49 OSPFv3_ET) and the sentence explaining the _ET suffix. A value table, not a directive. |
| `4.1` | OSPFv2 Type | 0 | walked | OSPFv2 Type. Wire format of the OSPFv2 message field and what Remote IP Address, Local IP Address and OSPF Message Contents hold. Indicative; ze does not write this type. |
| `4.2` | TABLE_DUMP Type | 2 | walked | TABLE_DUMP Type. Two capitalised MUST-level sites, mapped below to RFC6396-4.2-2 and RFC6396-4.2-1. Its remaining directives are advisory and carry no site: the RECOMMENDED to use TABLE_DUMP_V2 in new implementations, the MAY for decoding applications to support TABLE_DUMP, the MAY to use View Number to separate RIB views, the SHOULD to wrap the 16-bit Sequence Number, and the SHOULD to set the unused Status octet to 1. Those five are the unsourced ids below. |
| `4.3` | TABLE_DUMP_V2 Type | 0 | walked | TABLE_DUMP_V2 Type. States what V2 adds over TABLE_DUMP and lists subtypes 1 through 6. A value table with no directive. |
| `4.3.1` | PEER_INDEX_TABLE Subtype | 3 | walked | PEER_INDEX_TABLE Subtype. Three capitalised MUST-level sites, mapped below to RFC6396-4.3.1-3, -1 and -2. The rest is field definition: the Collector BGP ID, the Peer Count, and the Peer Entry fields Peer Type, Peer BGP ID, Peer IP Address and Peer AS, with the A and I bits of Peer Type and the rule that the peer index begins at 0. Every one of those is stated indicatively, so no site of this section obligates a WRITER about the VALUE it puts in a peer entry field, the Peer BGP ID included. They are carried by the Wire Formats table of rfc/short/rfc6396.md. |
| `4.3.2` | AFI/SAFI-Specific RIB Subtypes | 0 | walked | AFI/SAFI-Specific RIB Subtypes. Defines the RIB Entry Header and states that Prefix Length and Prefix follow the BGP NLRI encoding with irrelevant trailing bits. Indicative. |
| `4.3.3` | RIB_GENERIC Subtype | 0 | walked | RIB_GENERIC Subtype. Defines the RIB_GENERIC Entry Header. Its one directive is the SHOULD to discard the remainder of an MRT record whose AFI and SAFI the implementation does not recognize, which is advisory and carries no site: the unsourced id below. |
| `4.3.4` | RIB Entries | 1 | walked | RIB Entries. One capitalised MUST-level site, mapped below to RFC6396-4.3.4-1. The MP_REACH_NLRI abbreviation is stated indicatively, "only the Next Hop Address Length and Next Hop Address fields are included. The Reserved field is omitted", with no modal verb, so the scan derives no site for it. The summary reads it as a gated obligation, which is the unsourced id below. |
| `4.4` | BGP4MP Type | 0 | walked | BGP4MP Type. Names the six BGP4MP subtypes and their codes. A value table with no directive. |
| `4.4.1` | BGP4MP_STATE_CHANGE Subtype | 0 | walked | BGP4MP_STATE_CHANGE Subtype. Wire format, the six FSM state values from RFC 4271 Section 8.2.2, and the AFI values. Its two directives are advisory and carry no site: the MAY to set an undefined Peer AS Number to zero and the MAY for an unknown or unsupported Interface Index to be zero. Those are the unsourced ids below. |
| `4.4.2` | BGP4MP_MESSAGE Subtype | 2 | walked | BGP4MP_MESSAGE Subtype. Two capitalised MUST-level sites, mapped below to RFC6396-4.4.2-1 and RFC6396-4.4.2-2. The Interface Index MAY is a repeat of the section 4.4.1 statement and is carried by RFC6396-4.4.1-2, which section 4.4.1 lists as unsourced. |
| `4.4.3` | BGP4MP_MESSAGE_AS4 Subtype | 1 | walked | BGP4MP_MESSAGE_AS4 Subtype. One capitalised MUST-level site, mapped below to RFC6396-4.4.3-1. The rest states that the subtype is otherwise identical to BGP4MP_MESSAGE and shows the fields. |
| `4.4.4` | BGP4MP_STATE_CHANGE_AS4 Subtype | 0 | walked | BGP4MP_STATE_CHANGE_AS4 Subtype. States that it is BGP4MP_STATE_CHANGE with 4-byte Peer and Local AS fields, and shows the format. Indicative. |
| `4.4.5` | BGP4MP_MESSAGE_LOCAL Subtype | 0 | walked | BGP4MP_MESSAGE_LOCAL Subtype. States that the subtype marks a locally generated BGP message and that the Local fields name the collector while the Peer fields name the recipient. Indicative. |
| `4.4.6` | BGP4MP_MESSAGE_AS4_LOCAL Subtype | 0 | walked | BGP4MP_MESSAGE_AS4_LOCAL Subtype. States that the fields are identical to BGP4MP_MESSAGE_AS4 and that the record marks a locally generated message. Indicative. |
| `4.5` | ISIS Type | 0 | walked | ISIS Type. States that the IS-IS PDU follows the MRT Common Header directly, that there is no type-specific header, and that the Subtype code is undefined. Indicative. |
| `4.6` | OSPFv3 Type | 0 | walked | OSPFv3 Type. Wire format of the OSPFv3 message field, extending OSPFv2 with variable-length addresses. Indicative. |
| `5` | IANA Considerations | 1 | walked | IANA Considerations. Registration guidance under BCP 26 for the Type Code and Subtype Code name spaces. Binds IANA and the authors of future MRT specifications, not an MRT writer or reader. Its one site is the BCP 26 policy-name sentence, excluded below. |
| `5.1` | Type Codes | 2 | walked | Type Codes. The allocation policy for the Type Code registry: 0-64 reserved, new codes from 65, IETF Review to 511, Specification Required to 2047, First Come First Served to 64511, Experimental Use to 65534, and 65535 reserved. Site 5.1:1 carries the allocation floor and maps to RFC6396-5.1-1, which rfc/short/rfc6396.md declares; site 5.1:2 is a policy-name match the case-insensitive scan made and is excluded below. |
| `5.2` | Subtype Codes | 1 | walked | Subtype Codes. States that Subtype definitions are specific to a Type Code and that Subtype assignments follow the rules of their Type Code. Its one site binds the author of a future MRT Subtype definition and is excluded below. |
| `5.3` | Defined Type Codes | 0 | skipped (iana) | Defined Type Codes. The registry table of the twenty Type Codes this document defines, each pointing at the section that specifies it. |
| `5.4` | Defined BGP, BGP4PLUS, and BGP4PLUS_01 Subtype Codes | 0 | skipped (iana) | Defined BGP, BGP4PLUS, and BGP4PLUS_01 Subtype Codes. A registry table for the deprecated BGP Type's eight subtypes. |
| `5.5` | Defined TABLE_DUMP Subtype Codes | 0 | skipped (iana) | Defined TABLE_DUMP Subtype Codes. A registry table: AFI_IPv4 is 1 and AFI_IPv6 is 2. |
| `5.6` | Defined TABLE_DUMP_V2 Subtype Codes | 0 | skipped (iana) | Defined TABLE_DUMP_V2 Subtype Codes. A registry table: PEER_INDEX_TABLE 1 through RIB_GENERIC 6. |
| `5.7` | Defined BGP4MP and BGP4MP_ET Subtype Codes | 0 | skipped (iana) | Defined BGP4MP and BGP4MP_ET Subtype Codes. A registry table for the six BGP4MP subtypes, and the closing note that BGP4MP_ET shares them. |
| `6` | Security Considerations | 0 | walked | Security Considerations. States that the MRT fields are descriptive and induce no behavior in the recipient application, that peer IP addresses, next hops and path attributes can be sensitive, and that an organization publishing MRT dumps beyond its domain should check with the peers whose information is included. That last direction binds the operator publishing an archive rather than the MRT writer, it carries no RFC 2119 keyword, and the summary declares no requirement for it. |
| `7` | References | 0 | skipped (references) | References. The heading itself, with the two subsections below carrying the entries. |
| `7.1` | not stated | 0 | skipped (references) | Normative References: IANA-AF, RFC 791, RFC 1195, RFC 2119, RFC 2328, RFC 2460, RFC 3629, RFC 4271, RFC 4760, RFC 5226, RFC 5340. |
| `7.2` | not stated | 0 | skipped (references) | Informative References: GEOMRT, MRT_PROG_GUIDE, POSIX, RFC 4272. |
| `A` | MRT Encoding Examples | 0 | skipped (appendix-non-normative) | MRT Encoding Examples. The appendix says so in its own first sentence, "This appendix, which is not normative, contains MRT encoding examples". It shows one BGP4MP_MESSAGE_AS4 record and one TABLE_DUMP_V2 pair in hexadecimal. |
| `B` | Deprecated MRT Types | 0 | walked | Deprecated MRT Types. Two indicative sentences: the appendix lists deprecated types, and they are documented for informational purposes. The subsections below carry what text there is. |
| `B.1` | Deprecated MRT Informational Types | 1 | walked | Deprecated MRT Informational Types. Codes 0 through 4, which the section itself says are not known to be implemented. One capitalised MUST-level site, mapped below to RFC6396-B.1-1. Its one advisory directive, the SHOULD to set the unused Subtype field to 0, carries no site and is the unsourced id below. |
| `B.1.1` | NULL Type | 0 | walked | NULL Type. One sentence: the NULL Type message causes no operation. |
| `B.1.2` | START Type | 0 | walked | START Type. One sentence: the record indicates that a collector is about to begin generating MRT records. |
| `B.1.3` | DIE Type | 0 | walked | DIE Type. One sentence, carrying the advisory SHOULD that a remote MRT repository stop accepting messages. It is advisory, so the scan derives no site; the summary declares it as the unsourced id below. |
| `B.1.4` | I_AM_DEAD Type | 0 | walked | I_AM_DEAD Type. One sentence: the record indicates that a collector has shut down and stopped generating MRT records. |
| `B.1.5` | PEER_DOWN Type | 0 | walked | PEER_DOWN Type. States what the record was intended for and that the BGP state change types duplicate the function. Indicative. |
| `B.2` | Other Deprecated MRT Types | 0 | walked | Other Deprecated MRT Types. The code list 5 through 10: BGP, RIP, IDRP, RIPNG, BGP4PLUS and BGP4PLUS_01. A value table. |
| `B.2.1` | BGP Type | 0 | walked | BGP Type. States that the Message field carries BGP routing information, that the content depends on the Subtype, and that the type and all its subtypes are deprecated by BGP4MP. Indicative. |
| `B.2.1.1` | BGP_NULL Subtype | 0 | walked | BGP_NULL Subtype. One sentence: the subtype is unused and deprecated. |
| `B.2.1.2` | BGP_UPDATE Subtype | 0 | walked | BGP_UPDATE Subtype. Wire format of the deprecated BGP_UPDATE record and its Peer AS, Peer IP, Local AS, Local IP and BGP UPDATE fields. Indicative. |
| `B.2.1.3` | BGP_PREF_UPDATE Subtype | 0 | walked | BGP_PREF_UPDATE Subtype. States that the format was never fully specified and is not known to be implemented. Indicative. |
| `B.2.1.4` | BGP_STATE_CHANGE Subtype | 0 | walked | BGP_STATE_CHANGE Subtype. Wire format of the deprecated state-change record and the note that the AS and IP fields are 2-byte and IPv4. Indicative. |
| `B.2.1.5` | BGP_SYNC Subtype | 0 | walked | BGP_SYNC Subtype. Describes a record whose use is unclear with no known implementations. Its one advisory statement, that the subtype should be ignored, carries no capitalised keyword site and is the unsourced id below. |
| `B.2.1.6` | BGP_OPEN Subtype | 0 | walked | BGP_OPEN Subtype. States that the record encodes a received BGP OPEN message in the BGP_UPDATE format. Indicative. |
| `B.2.1.7` | BGP_NOTIFY Subtype | 0 | walked | BGP_NOTIFY Subtype. States that the record encodes a received BGP NOTIFICATION message in the BGP_UPDATE format. Indicative. |
| `B.2.1.8` | BGP_KEEPALIVE Subtype | 0 | walked | BGP_KEEPALIVE Subtype. States that the record encodes a received BGP KEEPALIVE message in the BGP_UPDATE format. Indicative. |
| `B.2.2` | RIP Type | 0 | walked | RIP Type. States that the Message field carries a RIP packet and that the type is deprecated. Its advisory statement that the unused Subtype field should be set to 0 covers this section and B.2.4, carries no capitalised keyword site, and is the unsourced id below. |
| `B.2.3` | IDRP Type | 0 | walked | IDRP Type. States that the type was intended to carry IDRP information, that the format was never fully specified, and that no implementation is known. Indicative. |
| `B.2.4` | RIPNG Type | 0 | walked | RIPNG Type. States that the Message field carries a RIPng packet and that the type is deprecated. Its unused-Subtype statement is the one RFC6396-B.2.2-1 covers, listed as unsourced on section B.2.2. |
| `B.2.5` | BGP4PLUS and BGP4PLUS_01 Types | 0 | walked | BGP4PLUS and BGP4PLUS_01 Types. States that the two types encode BGP with multiprotocol extensions in early Zebra releases and are deprecated by BGP4MP. Indicative. |
| `B.2.6` | Deprecated BGP4MP Subtypes | 0 | walked | Deprecated BGP4MP Subtypes. Names subtypes 2 and 3, BGP4MP_ENTRY and BGP4MP_SNAPSHOT, as deprecated. A value table. |
| `B.2.6.1` | BGP4MP_ENTRY Subtype | 0 | walked | BGP4MP_ENTRY Subtype. Wire format of the deprecated RIB entry record, deprecated by TABLE_DUMP_V2. Indicative. |
| `B.2.6.2` | BGP4MP_SNAPSHOT Subtype | 0 | walked | BGP4MP_SNAPSHOT Subtype. Describes a record pointing at an external dump file, with the note that it is not known to be implemented. Indicative. |
| `C` | not stated | 0 | skipped (acknowledgements) | Acknowledgements, and with it the unnumbered Authors' Addresses block, which carries no section number and so stays in this body under the heading derivation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions paragraph of the Copyright Notice, which the boilerplate filter does not strip because it names the Simplified BSD License rather than RFC 2119. Its "must" binds a person who extracts a code component from the document, and it states a licensing condition rather than a protocol obligation. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A sentence naming the four BCP 26 registration policies this document uses. The prose scan matches it on "Required" inside the policy name "Specification Required", which is a proper name quoted from BCP 26 and not a directive to anyone. | The following policies are used here with the meanings defined in BCP 26: "Specification Required", "IETF Consensus", "Experimental Use", "First Come First Served". |
| `5.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A registry allocation statement: Type Codes 512 to 2047 are handed out under the BCP 26 "Specification Required" policy. The prose scan matches it on "Required" inside that policy name. The sentence is indicative and directs nobody. | Type Codes 512-2047 are assigned based on Specification Required. |
| `5.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the author of a future MRT Subtype specification, and behind them the IANA registry that will refuse a Subtype naming no Type Code. Ze neither allocates MRT Subtype codes nor writes MRT specifications, so no MRT writer or reader in ze is bound by it. The role is a specification author and the IANA registry behind them, so no producer could act as it. Ze only WRITES records under codes already assigned: `asyncWriter.Write` (`internal/plugins/mrt/async_writer.go`). | New Subtype Code definitions must reference an existing Type Code to which the Subtype belongs. |

## Superseded

No document obsoletes RFC 6396, so its obligations are stated where they were written.
