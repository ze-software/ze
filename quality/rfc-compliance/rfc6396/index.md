# RFC 6396 - Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format

Supported. Every requirement this repository extracted from RFC 6396, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 12.5% | 1 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 75.0% | 6 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 9 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.5% | 1 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 9 |
| Tagged units | 9 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC6396-4.3.4-1`](#rfc6396-4.3.4-1)

**Annotated instead of tested (12):** [`RFC6396-4.2-1`](#rfc6396-4.2-1), [`RFC6396-4.2-2`](#rfc6396-4.2-2), [`RFC6396-4.3.1-1`](#rfc6396-4.3.1-1), [`RFC6396-4.3.1-2`](#rfc6396-4.3.1-2), [`RFC6396-4.3.1-3`](#rfc6396-4.3.1-3), [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2), [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1), [`RFC6396-4.4.2-2`](#rfc6396-4.4.2-2), [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1), [`RFC6396-1-1`](#rfc6396-1-1), [`RFC6396-5.1-1`](#rfc6396-5.1-1), [`RFC6396-B.1-1`](#rfc6396-b.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6396-4.2-1` | TABLE_DUMP: AS_PATH attribute must only consist of 2-byte AS numbers (§4.2) | MUST | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's RIB export always emits TABLE_DUMP_V2 (the Section 4.2 mandate) and never operationally writes TABLE_DUMP, so the 2-byte-AS_PATH-in-TABLE_DUMP obligation binds a writer role ze does not play (internal/mrt/encode.go:185 has no production caller) |
| `RFC6396-4.2-2` | TABLE_DUMP_V2 must be used when 4-byte AS numbers are needed or when peer/prefix AFI differ (§4.2) | MUST | 4.2 - TABLE_DUMP Type | **positive:** `unit/verify` [`TestRibSubtype`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L17). **negative:** no negative test. **{single-polarity}:** ze's RIB dumper unconditionally emits TABLE_DUMP_V2 with AS4 peer entries, so it always satisfies the use-V2 mandate and there is no path that emits TABLE_DUMP to reject (internal/plugins/mrt/dump.go:113, :206) |
| `RFC6396-4.3.1-1` | TABLE_DUMP_V2 PEER_INDEX_TABLE: View Name Length must be set to 0 if no view name is present (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L162). **negative:** no negative test. **{single-polarity}:** the encoder writes View Name Length as len(viewName) and the producer always passes an empty name, so the length is 0 by construction (internal/mrt/encode.go:48, internal/plugins/mrt/dump.go:199) |
| `RFC6396-4.3.1-2` | TABLE_DUMP_V2 PEER_INDEX_TABLE: View Name encoding must follow UTF-8 (RFC 3629) (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L167). **negative:** no negative test. **{single-polarity}:** ze's PEER_INDEX_TABLE writer always emits an empty view name, which is trivially valid UTF-8, and no code path produces a non-UTF-8 view name (internal/plugins/mrt/dump.go:199, internal/mrt/encode.go:48-50) |
| `RFC6396-4.3.1-3` | TABLE_DUMP_V2: RIB entry MRT records must immediately follow the PEER_INDEX_TABLE MRT record (§4.3.1) | MUST | 4.3.1 - PEER_INDEX_TABLE Subtype | **positive:** `unit/verify` [`TestDumpV2PeerIndexBeforeFirstRIBEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L213). **negative:** no negative test. **{single-polarity}:** the RIB dump writes the PEER_INDEX_TABLE on the first OnRoute callback, before that route's RIB entry and before any other RIB record, guaranteeing the ordering (internal/plugins/mrt/dump.go:149-152) |
| `RFC6396-4.3.4-1` | TABLE_DUMP_V2 RIB entries: all AS numbers in the AS_PATH attribute must be encoded as 4-byte AS numbers (§4.3.4) | MUST | 4.3.4 - RIB Entries | **positive:** `unit/verify` [`TestDumpV2RIBEntryASPathIs4Byte`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L244). **positive:** `unit/verify` [`TestRFC6396RIBEntryASPathStoredFourByte`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6396_mrt_aspath_test.go#L45). **negative:** `unit/verify` [`TestRFC6396RIBEntryASPathFourByteSessionUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6396_mrt_aspath_test.go#L80) |
| `RFC6396-4.3.4-2` | TABLE_DUMP_V2 RIB entries: MP_REACH_NLRI attribute must only include Next Hop Address Length and Next Hop Address fields (AFI, SAFI, NLRI, Reserved omitted) (§4.3.4) | MUST | 4.3.4 - RIB Entries | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TABLE_DUMP_V2 RIB writer reconstructs a NEXT_HOP (type 3) attribute and never emits an MP_REACH_NLRI attribute in RIB entries, so the abbreviation obligation never binds ze's producer (internal/component/bgp/plugins/rib/rib_mrt.go:127-153) |
| `RFC6396-4.4.2-1` | BGP4MP_MESSAGE: AS_PATH must only consist of 2-byte AS numbers (§4.4.2) | MUST | 4.4.2 - BGP4MP_MESSAGE Subtype | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's live capture unconditionally emits the AS4 BGP4MP message subtype and never writes the 2-byte BGP4MP_MESSAGE (subtype 1), so the 2-byte-AS_PATH obligation binds a writer variant ze does not produce (internal/plugins/mrt/dump.go:240-250) |
| `RFC6396-4.4.2-2` | BGP4MP_MESSAGE: only one BGP message shall be encoded per record (§4.4.2) | MUST | 4.4.2 - BGP4MP_MESSAGE Subtype | **positive:** `unit/verify` [`TestOneBGPMessagePerBGP4MPRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/component_test.go#L84). **negative:** no negative test. **{single-polarity}:** OnBGPMessage is invoked once per BGP message and writes exactly one message into each BGP4MP record, so records always carry a single message (internal/plugins/mrt/component.go:99-142) |
| `RFC6396-4.4.3-1` | BGP4MP_MESSAGE_AS4: AS_PATH must only consist of 4-byte AS numbers (§4.4.3) | MUST | 4.4.3 - BGP4MP_MESSAGE_AS4 Subtype | **positive:** no positive test. **negative:** no negative test. **{gap}:** the live writer hardcodes the AS4 subtype and copies the on-wire message verbatim without checking negotiated AS4 capability, so a 2-byte (OLD-peer) session's 2-byte AS_PATH is mislabeled as AS4 (internal/plugins/mrt/dump.go:240-250, component.go:123; ze supports 2-byte sessions per internal/component/bgp/plugins/rib/storage/attrparse.go:18-24) |
| `RFC6396-1-1` | All multi-octet numeric values must be encoded in network byte order (big-endian) (§1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestCommonHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L24). **negative:** no negative test. **{single-polarity}:** all MRT encode and decode use binary.BigEndian throughout and round-trip byte-for-byte, with no alternate-endianness path (internal/mrt/encode.go:7, decode.go) |
| `RFC6396-5.1-1` | New Type Codes must be allocated starting at 65 (§5.1) | MUST | 5.1 - Type Codes | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an IANA registry allocation policy binding specification and registry authors; ze does not allocate MRT type codes |
| `RFC6396-B.1-1` | Deprecated informational types: message string encoding must follow UTF-8 (§B.1) | MUST | B.1 - Deprecated MRT Informational Types | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not define or produce the deprecated informational types (codes 0-4); its type table starts at OSPFv2 (11) (internal/mrt/types.go:6-16) |
| `RFC6396-4.2-3` | New implementations should use TABLE_DUMP_V2 instead of TABLE_DUMP (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-4` | TABLE_DUMP: Sequence Number should wrap back to zero when exceeding 16-bit bounds (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-5` | TABLE_DUMP: Status field should be set to 1 (§4.2) | SHOULD | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.3.3-1` | RIB_GENERIC: an implementation that does not recognize particular AFI and SAFI values should discard the remainder of the MRT record (§4.3.3) | SHOULD | 4.3.3 - RIB_GENERIC Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.1-2` | Deprecated informational types: Subtype field should be set to 0 (§B.1) | SHOULD | B.1 - Deprecated MRT Informational Types | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.1.3-1` | DIE Type: remote MRT repository should stop accepting messages upon receiving DIE (§B.1.3) | SHOULD | B.1.3 - DIE Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.2.1.5-1` | BGP_SYNC Subtype should be ignored (no known implementations) (§B.2.1.5) | SHOULD | B.2.1.5 - BGP_SYNC Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-B.2.2-1` | Deprecated routing types (RIP, RIPNG): Subtype field should be set to 0 (§B.2.2, §B.2.4) | SHOULD | B.2.2 - RIP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-6` | TABLE_DUMP: MRT decoding applications may wish to support this type (§4.2) | MAY | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.2-7` | TABLE_DUMP: View Number may be used to distinguish multiple RIB views (§4.2) | MAY | 4.2 - TABLE_DUMP Type | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.4.1-1` | BGP4MP_STATE_CHANGE: Peer AS Number may be set to zero if undefined (§4.4.1) | MAY | 4.4.1 - BGP4MP_STATE_CHANGE Subtype | **positive:** no positive test. **negative:** no negative test |
| `RFC6396-4.4.1-2` | BGP4MP: Interface Index value may be zero if unknown or unsupported (§4.4.1, §4.4.2) | MAY | 4.4.1 - BGP4MP_STATE_CHANGE Subtype | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6396-4.2-1`](#rfc6396-4.2-1) TABLE_DUMP: AS_PATH attribute must only consist of 2-byte AS numbers (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's RIB export always emits TABLE_DUMP_V2 (the Section 4.2 mandate) and never operationally writes TABLE_DUMP, so the 2-byte-AS_PATH-in-TABLE_DUMP obligation binds a writer role ze does not play (internal/mrt/encode.go:185 has no production caller) |
| [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2) TABLE_DUMP_V2 RIB entries: MP_REACH_NLRI attribute must only include Next Hop Address Length and Next Hop Address fields (AFI, SAFI, NLRI, Reserved omitted) (§4.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TABLE_DUMP_V2 RIB writer reconstructs a NEXT_HOP (type 3) attribute and never emits an MP_REACH_NLRI attribute in RIB entries, so the abbreviation obligation never binds ze's producer (internal/component/bgp/plugins/rib/rib_mrt.go:127-153) |
| [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1) BGP4MP_MESSAGE: AS_PATH must only consist of 2-byte AS numbers (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's live capture unconditionally emits the AS4 BGP4MP message subtype and never writes the 2-byte BGP4MP_MESSAGE (subtype 1), so the 2-byte-AS_PATH obligation binds a writer variant ze does not produce (internal/plugins/mrt/dump.go:240-250) |
| [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1) BGP4MP_MESSAGE_AS4: AS_PATH must only consist of 4-byte AS numbers (§4.4.3) | {gap}, no test | the live writer hardcodes the AS4 subtype and copies the on-wire message verbatim without checking negotiated AS4 capability, so a 2-byte (OLD-peer) session's 2-byte AS_PATH is mislabeled as AS4 (internal/plugins/mrt/dump.go:240-250, component.go:123; ze supports 2-byte sessions per internal/component/bgp/plugins/rib/storage/attrparse.go:18-24) |
| [`RFC6396-5.1-1`](#rfc6396-5.1-1) New Type Codes must be allocated starting at 65 (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: this is an IANA registry allocation policy binding specification and registry authors; ze does not allocate MRT type codes |
| [`RFC6396-B.1-1`](#rfc6396-b.1-1) Deprecated informational types: message string encoding must follow UTF-8 (§B.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not define or produce the deprecated informational types (codes 0-4); its type table starts at OSPFv2 (11) (internal/mrt/types.go:6-16) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6396-4.2-1`](#rfc6396-4.2-1)

TABLE_DUMP: AS_PATH attribute must only consist of 2-byte AS numbers (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.2-1, so no unit is bound to it.

### [`RFC6396-4.2-2`](#rfc6396-4.2-2)

TABLE_DUMP_V2 must be used when 4-byte AS numbers are needed or when peer/prefix AFI differ (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRibSubtype`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L17) | unit/verify | unproven |

### [`RFC6396-4.3.1-1`](#rfc6396-4.3.1-1)

TABLE_DUMP_V2 PEER_INDEX_TABLE: View Name Length must be set to 0 if no view name is present (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L162) | unit/verify | unproven |

### [`RFC6396-4.3.1-2`](#rfc6396-4.3.1-2)

TABLE_DUMP_V2 PEER_INDEX_TABLE: View Name encoding must follow UTF-8 (RFC 3629) (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPeerIndexTableEmptyViewName`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L167) | unit/verify | unproven |

### [`RFC6396-4.3.1-3`](#rfc6396-4.3.1-3)

TABLE_DUMP_V2: RIB entry MRT records must immediately follow the PEER_INDEX_TABLE MRT record (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDumpV2PeerIndexBeforeFirstRIBEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L213) | unit/verify | unproven |

### [`RFC6396-4.3.4-1`](#rfc6396-4.3.4-1)

TABLE_DUMP_V2 RIB entries: all AS numbers in the AS_PATH attribute must be encoded as 4-byte AS numbers (§4.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6396RIBEntryASPathFourByteSessionUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6396_mrt_aspath_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC6396RIBEntryASPathStoredFourByte`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/rfc6396_mrt_aspath_test.go#L45) | unit/verify | unproven |
| positive | [`TestDumpV2RIBEntryASPathIs4Byte`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/dump_test.go#L244) | unit/verify | unproven |

### [`RFC6396-4.3.4-2`](#rfc6396-4.3.4-2)

TABLE_DUMP_V2 RIB entries: MP_REACH_NLRI attribute must only include Next Hop Address Length and Next Hop Address fields (AFI, SAFI, NLRI, Reserved omitted) (§4.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.3.4-2, so no unit is bound to it.

### [`RFC6396-4.4.2-1`](#rfc6396-4.4.2-1)

BGP4MP_MESSAGE: AS_PATH must only consist of 2-byte AS numbers (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.4.2-1, so no unit is bound to it.

### [`RFC6396-4.4.2-2`](#rfc6396-4.4.2-2)

BGP4MP_MESSAGE: only one BGP message shall be encoded per record (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOneBGPMessagePerBGP4MPRecord`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/component_test.go#L84) | unit/verify | unproven |

### [`RFC6396-4.4.3-1`](#rfc6396-4.4.3-1)

BGP4MP_MESSAGE_AS4: AS_PATH must only consist of 4-byte AS numbers (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-4.4.3-1, so no unit is bound to it.

### [`RFC6396-1-1`](#rfc6396-1-1)

All multi-octet numeric values must be encoded in network byte order (big-endian) (§1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCommonHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L24) | unit/verify | unproven |

### [`RFC6396-5.1-1`](#rfc6396-5.1-1)

New Type Codes must be allocated starting at 65 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6396-5.1-1, so no unit is bound to it.

### [`RFC6396-B.1-1`](#rfc6396-b.1-1)

Deprecated informational types: message string encoding must follow UTF-8 (§B.1)

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
