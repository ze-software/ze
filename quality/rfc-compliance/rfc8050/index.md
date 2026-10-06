# RFC 8050 - Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions

Partial. Every requirement this repository extracted from RFC 8050, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 83.3% | 5 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 1 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 77.8% | 28 of 36 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 6 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tagged units | 36 |
| Recorded audit verdicts | 6 |
| Discrimination records | 28 |
| Summary | `rfc/short/rfc8050.md` |
| Requirement shard | `rfc/requirements/rfc8050.md` |
| RFC text | `rfc/full/rfc8050.txt` |

## Enrolment

Enrolled: MRT Routing Information Export Format with BGP Additional Path Extensions: six MUST-level requirements. Tagged tests cover RIB entry layout (4.1-1), raw RIB_GENERIC NLRI (4.2-1), big-endian Path Identifiers (x-2), unchanged complete BGP4MP messages (x-3), subtype and actual directional OPEN context (x-4), and negotiated directional subtype selection (x-1). The x-2 numeric encoding row is {single-polarity: positive}; other rows carry positive and negative claims.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- BGP4MP/BGP4MP_ET recording observes complete original messages separately from semantic route delivery and selects direction/family ADD-PATH subtypes. Offline readers retain both actual OPENs per file/session epoch and use their negotiated per-family directional modes. Ordinary subtypes and unambiguous single-family ADD-PATH messages also decode without OPENs. TABLE_DUMP_V2 subtypes 8-11 store Path Identifiers in RIB entries
- subtype 12 stores them in raw NLRI. Context-aware repair and new proof carriers await consolidated verification and independent rejudgment.


**What the ledger says remains**

Multiple-family ADD-PATH records without both directional OPENs are explicitly undecodable: updates-only, mid-session and independently rotated captures cannot invent missing negotiation. Replay/inject/serve do not negotiate ADD-PATH and refuse Path-ID-bearing or ambiguous UPDATEs. TABLE_DUMP_V2 snapshots still select ADD-PATH through the static operator toggle; their RIB callback does not expose stored Path Identifiers.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC8050-x-1`](#rfc8050-x-1), [`RFC8050-4.1-1`](#rfc8050-4.1-1), [`RFC8050-4.2-1`](#rfc8050-4.2-1), [`RFC8050-x-3`](#rfc8050-x-3), [`RFC8050-x-4`](#rfc8050-x-4)

**Annotated (including scoped evidence) (1):** [`RFC8050-x-2`](#rfc8050-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8050-x-1` | The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L31). **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L23). **negative:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L32). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) |
| `RFC8050-4.1-1` | the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Attribute Length \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L167). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L264). **negative:** `unit/verify` [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L216) |
| `RFC8050-4.2-1` | These fields continue to encapsulate the raw and additional-path- enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries. For clarity, the RIB Entries in this subtype are not redefined. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L515). **negative:** `unit/verify` [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L312) |
| `RFC8050-x-2` | 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L173). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L268). **negative:** no negative test. **{single-polarity}:** the Path ID is written and read big-endian (internal/mrt/encode.go:107, decode.go:295); there is no alternate-endianness path |
| `RFC8050-x-3` | The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L367). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L99). **negative:** `unit/verify` [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L427). **negative:** `unit/verify` [`TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc8050_addpath_record_test.go#L30) |
| `RFC8050-x-4` | MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665). **positive:** `unit/verify` [`TestMRTCollisionWinnerReservation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L42). **positive:** `unit/verify` [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L19). **positive:** `unit/verify` [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L26). **positive:** `unit/verify` [`TestMRTMigrationEpochUsesActualLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_migration_epoch_test.go#L26). **positive:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L33). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22). **positive:** `unit/verify` [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17). **positive:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87). **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22). **positive:** `unit/verify` [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L21). **positive:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L72). **positive:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16). **negative:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674). **negative:** `unit/verify` [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L20). **negative:** `unit/verify` [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L27). **negative:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25). **negative:** `unit/verify` [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L22). **negative:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73). **negative:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) |

## Gaps and untested MUSTs

RFC 8050 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8050-x-1`](#rfc8050-x-1)

The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent reread of both complete tagged carriers and their helper/producer/consumer paths. RFC 8050 Section 2: The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. Section 3: These enhancements continue to encapsulate the entire BGP message in the BGP message field. The observer matrix requires exact ordinary/extended type, received/sent subtype 9/11 versus 4/7, complete original message bytes and exact record count for isolated classic/MP announcement/withdrawal locations. The actual TCP collision winner retains the same socket and byte-identical directional OPENs, then records eight isolated UPDATEs exactly once with literal received classic/MP subtypes 4/9 and sent classic/MP 11/7; mixed withdrawals additionally require exact prefixes and Path Identifiers. Q1-Q4 pass: isolated ordinary-family negatives cannot be masked by other-family or opposite-direction AP. setSendCtxID publishes the context into the epoch writer; observeReceivedWire and observedBGPWriter.observe dispatch to OnBGPMessage, updateAddPath and bgp4mpTypeSubtype. Stored panic records are producer-reachability evidence only, not a new semantic run. No checks or foreign MRT acceptance were performed; current execution/discrimination freshness remains pending. No TABLE_DUMP_V2 or whole-RFC claim is made.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L23) | unit/verify | revert, verified |

### [`RFC8050-4.1-1`](#rfc8050-4.1-1)

the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Peer Index | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Originated Time | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Path Identifier | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Attribute Length | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-10-02 (c33 judge): + TestRFC8050AddPathRIBEntryFollowsFigure1 compares WriteRIBEntries add-path output with the literal Figure 1 octets (Peer Index 00 03, Originated Time 5f 5e 10 00, Path Identifier 01 02 03 04, Attribute Length 00 04, attributes) and reads the same octets through DecodeRIBRecord under all four AFI/SAFI-specific add-path subtypes 8-11; - TestRIBRecordRoundTrip (base subtype 2 reads no Path Identifier). Literal octets, not a symmetric round trip, so a misplaced or mis-sized field on either side goes red. Judge break: decodeRIBEntries reading only the low 16 bits of the Path Identifier turns the new unit red in all four subtypes while HEAD TestRIBRecordAddPathRoundTrip stays green. Does not depend on add-path NLRI parsing (the RIB prefix sits in the record header, no Path ID).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L216) | unit/verify | unproven |
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L264) | unit/verify | unproven |
| positive | [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L167) | unit/verify | revert, verified |

### [`RFC8050-4.2-1`](#rfc8050-4.2-1)

These fields continue to encapsulate the raw and additional-path- enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries. For clarity, the RIB Entries in this subtype are not redefined. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: reading RIB_GENERIC_ADDPATH (subtype 12) RIB Entries as redefined (a Path Identifier in each entry), or losing the Path Identifier that belongs in the raw NLRI. TestRIBGenericAddPathRoundTrip hand-builds the bytes the RFC describes (Path ID 42 at the head of the NLRI blob, then an entry with Peer Index, Originated Time, Attribute Length, attributes and no Path ID) and requires rec.NLRI == nlri and Entries[0].Attributes == attrs; a decoder that read a 4-octet Path ID from the entry would take 0x0004 plus attribute bytes as the path id and misparse the attribute length, so the attribute comparison or the decode goes red. The negative TestRIBGenericRoundTrip requires the base subtype 6 NLRI to decode verbatim. Ze writes no subtype 12 record (internal/mrt/encode.go writes none; internal/mrt/decode.go is the only producer), so the decode path is the whole surface.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L312) | unit/verify | unproven |
| positive | [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L515) | unit/verify | unproven |

### [`RFC8050-x-2`](#rfc8050-x-2)

0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Peer Index | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Originated Time | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Path Identifier | (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-10-02 (c33 judge): {single-polarity: positive} (Figure 1 has no alternative encoding to refuse). TestRFC8050AddPathRIBEntryFollowsFigure1 pins the 32-bit field in network order by literal octets 01 02 03 04 on write and value 0x01020304 on read; four distinct octets and a value above 0xFFFF expose a 16-bit width or a reversed order. Judge break: a 16-bit read of the field turns it red, HEAD TestRIBRecordAddPathRoundTrip (PathID 1 and 2) stays green, which was the earlier gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L268) | unit/verify | unproven |
| positive | [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L173) | unit/verify | revert, verified |

### [`RFC8050-x-3`](#rfc8050-x-3)

The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent final source rejudgment after formatting. RFC 8050 section 3: "The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field." Read the full section and RFC 6396 section 4.4 base layouts, all four tagged carriers and actual producers. Q1 yes collectively: internal/mrt/rfc8050_layout_test.go::TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields feeds independently literal records through ReadFrom for subtypes 8/9/10/11, checks every base header field and all 45 UPDATE octets, and compares base-subtype readings. internal/mrt/mrt_test.go::TestBGP4MPMessageRoundTrip checks preserved fields and the complete literal Path-ID-bearing UPDATE. internal/plugins/mrt/rfc8050_addpath_record_test.go::TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout calls the actual recorder in both directions and asserts exactly two records, each byte after its timestamp, subtype 9/11, length 65, exactly 20 base-header octets and the complete 45-octet UPDATE. Q2 yes for those exact independent reader/writer assertions: lifting the Path Identifier into an extra MRT field, shifting fields or dropping/reconstructing message octets fails. The supplementary internal/mrt/mrt_test.go::TestBGP4MPStateChangeRoundTrip is only a roundtrip control over the shared header and cannot alone prove absence of a newly inserted field; its negative tag does not establish an independent adversarial ADDPATH case. Q3 yes for the effective carriers: a complete framed UPDATE containing a real Path Identifier isolates the temptation to add an MRT field; expected record length and all expected octets are independent of production encoders. The writer negative asserts forbidden output absence, not malformed-input rejection. Q4 yes collectively for unchanged fields and complete encapsulation; the weak state-change control is not used to confer that credit. internal/mrt/encode.go::WriteBGP4MPMessage/writeBGP4MPCommon and decode.go::DecodeBGP4MPMessage/decodeBGP4MPHeader implement the common layout, and internal/plugins/mrt/component.go::OnBGPMessage copies the complete original frame. Formatting the plugin carrier changes no assertion or expected octet. This is source judgment only; no discrimination renewal or test run is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L427) | unit/verify | unproven |
| negative | [`TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc8050_addpath_record_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L367) | unit/verify | revert, verified |
| positive | [`TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L99) | unit/verify | revert, verified |

### [`RFC8050-x-4`](#rfc8050-x-4)

MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent stale-unit rejudgment of all 21 current covers, comprising 13 distinct test functions, from the supplied native Collect/NewRenderInput map. RFC 8050 Section 2 (rfc/full/rfc8050.txt:95-104): 'MRT parsers are usually stateless.' 'In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized.' Section 3 (lines 132-135): 'These enhancements continue to encapsulate the entire BGP message in the BGP message field.' These sentences require subtype-directed decoding, not invention of an unavailable per-family negotiation map; RFC 7911 Section 5 (rfc/full/rfc7911.txt:235-250) makes ADD-PATH directional and AFI/SAFI-specific. Retained evidence: internal/mrt/rfc8050_no_open_test.go::TestRFC8050NoOPENSubtypeDecodesExactNLRI exercises 96 independently framed no-OPEN records across both MRT types, all eight ordinary/ADD-PATH subtypes and six isolated NLRI locations, requiring one delivered record, unchanged complete bytes, exactly two specified prefixes, IDs [0,0x01020304] for ADD-PATH and no IDs for ordinary encoding. internal/mrt/rfc8050_addpath_nlri_test.go::TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype independently requires exactly 10.0.0.0/24 without an OPEN. internal/mrt/mrt_test.go::TestIsAddPathHelpers distinguishes base and ADD-PATH RIB/message subtypes. internal/mrt/rfc8050_context_test.go::TestRFC8050SubtypeControlsEveryNLRILocation retains exact classic/MP prefix checks and location-specific truncated-ID errors with actual OPENs for multiple families. internal/mrt/rfc8050_session_context_test.go::TestRFC8050SessionContextDecodesMixedDirections and internal/mrt/rfc8050_session_context_boundaries_test.go::TestMRTContextCapabilityASNBoundaries retain exact four-prefix/directional-ID checks, absence of IDs in ordinary families, and ErrContextUnavailable for missing/repeated/one-sided OPENs, terminated/reused epochs, independent streams and mismatched identities. internal/analyze/rfc8050_commands_test.go::TestRFC8050CommandsReadSubtypeContext and internal/analyze/rfc8050_session_commands_test.go::TestRFC8050CommandsRequireMixedContext retain W=2 A=2, exact density counts, successful content filtering with retained OPEN context and explicit semantic-command failures without context. internal/analyze/rfc8050_boundary_test.go::TestMRTEmptyMPFamilyRequiresContext retains explicit missing-context refusal and the empty-MP-family control decoding [10.0.0.0/24,0.0.0.0/0] without IDs. internal/component/bgp/reactor/rfc8050_mrt_observer_test.go::TestRFC8050MessageCallbackPreservesEncodingAndFraming retains exact type/subtype/full-byte/count assertions for both directions, both MRT types and isolated classic/MP announcements/withdrawals. Both TestMRTWinningCollisionPreservesOPEN and TestMRTCollisionWinnerReservation in internal/component/bgp/reactor/rfc8050_collision_epoch_test.go retain original directional OPEN bytes, the surviving TCP socket, exact mixed-withdrawal prefixes/IDs and one record per supplied packet, including both handoff barriers. The sole migration-unit change is t.Cleanup(s.stopSendHoldTimer) at internal/component/bgp/reactor/rfc8050_migration_epoch_test.go:51; no consumer-visible assertion changed. TestMRTMigrationEpochUsesActualLocalOPEN still requires real Bad Peer AS fallback, actual fallback LocalAS on records, the original three OPENs, exactly two UPDATEs, byte-identical incoming/outgoing packets, incoming classic 10.1.0.0/24 without IDs plus MP 2001:db9::/32 with 0x01020304, and outgoing classic 10.2.0.0/24 with 0x05060708 plus MP 2001:dba::/32 without IDs, even through the retained writer after transport replacement (lines 80-179). Producer/consumer tracing confirms notifyWireMessage forwards to OnBGPMessage, recorder subtype selection reaches dispatchBGP4MP, and ParseBGPMessage/ParseMPReach/ParseMPUnreach consume AddPathFor and preserve identifiers. The polarity pair uses distinct ordinary/ADD-PATH layouts; the no-OPEN matrix isolates decoding rather than depending on another validation error. [INFERENCE] Forcing ordinary/ADD-PATH mode, truncating identifiers, inventing prefixes or losing epoch evidence would fail these exact assertions. Stored native producer-panic records establish producer dependence only, not those targeted semantic mutations. No execution was performed for this rejudgment, and the old note's mutation/race-run claims are not independently renewed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674) | unit/verify | unproven |
| negative | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) | unit/verify | revert, verified |
| negative | [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L22) | unit/verify | revert, verified |
| negative | [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L20) | unit/verify | revert, verified |
| negative | [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestMRTCollisionWinnerReservation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestMRTMigrationEpochUsesActualLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_migration_epoch_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665) | unit/verify | unproven |
| positive | [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L72) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8050.txt |
| Source fingerprint | 8bd154a2aa1b4659 |
| Record | rfc/extraction/rfc8050.json |
| Mapped sentences | 1 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the IETF Trust Copyright Notice and constrains how code extracted from the document is licensed, not the behavior of an MRT writer or parser. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Document roadmap: 'The following two sections define the required subtypes' names what Sections 3 and 4 contain. 'required' is an adjective on the subtypes the previous paragraphs called for, and the sentence imposes nothing on an implementation. The obligation it points at is carried by site 2:1, mapped to RFC8050-x-1. | The following two sections define the required subtypes. |

## Superseded

No document obsoletes RFC 8050, so its obligations are stated where they were written.
