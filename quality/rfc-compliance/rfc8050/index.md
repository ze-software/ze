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
| Proven by a recorded break | 68.0% | 17 of 25 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 6 | of 6 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

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
| Test tags | 25 |
| Tagged units | 25 |
| Recorded audit verdicts | 6 |
| Discrimination records | 17 |
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
| `RFC8050-x-1` | The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L23). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) |
| `RFC8050-4.1-1` | the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Attribute Length \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L167). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L264). **negative:** `unit/verify` [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L216) |
| `RFC8050-4.2-1` | These fields continue to encapsulate the raw and additional-path- enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries. For clarity, the RIB Entries in this subtype are not redefined. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L515). **negative:** `unit/verify` [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L312) |
| `RFC8050-x-2` | 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L173). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L268). **negative:** no negative test. **{single-polarity}:** the Path ID is written and read big-endian (internal/mrt/encode.go:107, decode.go:295); there is no alternate-endianness path |
| `RFC8050-x-3` | The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L367). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L99). **negative:** `unit/verify` [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L427). **negative:** `unit/verify` [`TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc8050_addpath_record_test.go#L30) |
| `RFC8050-x-4` | MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22). **positive:** `unit/verify` [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17). **positive:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87). **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22). **positive:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L72). **positive:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16). **negative:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674). **negative:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25). **negative:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73). **negative:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) |

## Gaps and untested MUSTs

RFC 8050 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8050-x-1`](#rfc8050-x-1)

The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 8050 §2: "The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI." The sole tagged carrier internal/component/bgp/reactor/rfc8050_mrt_observer_test.go::TestRFC8050MessageCallbackPreservesEncodingAndFraming has genuinely distinct positive/negative cases: both record types and directions, each isolated classic/MP announcement/withdrawal, exact subtype 9/11 versus 4/7 and full original bytes. It no longer exercises reconstructed messages. Producers internal/plugins/mrt/component.go::OnBGPMessage and dump.go::updateAddPath/bgp4mpTypeSubtype select a subtype from supplied directional context, but updateAddPath ORs modes across families. The tagged carrier never combines families with unequal modes, and fabricates its ContextID rather than observing both OPENs/session epochs. A lost or borrowed negotiation context for mixed UPDATEs can leave this row green. internal/mrt/context.go::SessionContexts.ObserveMessage, ObserveState, BGPMessage.AddPathFor and updateContext now implement actual directional OPEN evidence, epoch invalidation and refusal when ambiguous; that implementation is not proof owned by this row. Preserve weak for the mixed/context end-to-end clause, not an assertion that context support is absent. No splitting of original packets or invented OPEN context is authorized. Post-lint rejudgment 2026-10-04: the epoch-pointer optimization does not close this finding. This carrier calls notifyWireMessage with a fabricated registered context and does not call dispatchObservedWire or capture session OPENs. updateAddPath still ORs observed family modes; the single-family positive/negative assertions retain their bounded meaning.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) | unit/verify | revert, verified |
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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent final source rejudgment of all seven tagged units, not adoption of the earlier pending note. RFC 8050 section 2: "MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized." Usually is not a prohibition on real OPEN context. Q1 substantially yes for bounded assertions, not for a universal subtype-only decoding claim: internal/mrt/mrt_test.go::TestIsAddPathHelpers asserts subtype classifiers only; its negative prose must not be read as proving the decoder never consults negotiation. internal/mrt/rfc8050_addpath_nlri_test.go::TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype genuinely proves one no-OPEN subtype-9 classic announcement. internal/mrt/rfc8050_context_test.go::TestRFC8050SubtypeControlsEveryNLRILocation checks the AddPath flag for eight subtypes and both MRT types, exact classic/MP prefixes and each truncated-ID location, but supplies matching OPENs for all decode cases. internal/analyze/rfc8050_commands_test.go::TestRFC8050CommandsReadSubtypeContext likewise supplies OPENs before exact show/density counts and AS_PATH filtering. internal/mrt/rfc8050_session_context_test.go::TestRFC8050SessionContextDecodesMixedDirections uses actual literal opposite-direction OPENs to assert four distinct prefixes, exact IDs 0x01020304/0x05060708 and absence of IDs in ordinary families; missing/one-sided/repeated OPEN, teardown, notification, ASN identity reuse, unrelated endpoint/interface, a new epoch and a new ReadFrom are explicitly unavailable. Dynamic OPEN identity, post-handshake state preservation and retained immutable context are also asserted. internal/analyze/rfc8050_session_commands_test.go::TestRFC8050CommandsRequireMixedContext checks both mixed arrangements through show/density/filter, keeps OPEN evidence through filtering, and requires context errors without the prelude, including AS-path/attribute/community commands. internal/component/bgp/reactor/rfc8050_mrt_observer_test.go::TestRFC8050MessageCallbackPreservesEncodingAndFraming checks each isolated NLRI location, both directions and MRT types, exact subtype and original bytes; its registered context is injected, not established by a real handshake. Q2 yes for these bounded exact values/errors; no for whole subtype-directed decoding coverage. context.go::AddPathFor consults negotiated modes when available and updateContext returns early with negotiation, so matching-OPEN fixtures can remain green if the no-OPEN mode selection is broken. Classifier tests cannot fill that decoder gap. Whole-body panic discrimination records establish reachability for their named break, not correctness of mode selection; none was rerun here. Q3 yes for mixed-direction context cases and individually damaged NLRI fields, with limits: the older homogeneous fixtures duplicate classic/MP prefixes and are layout tests, not full BGP semantic-validity proof. The mixed fixture avoids that ambiguity. The session carrier's claim of complete message preservation is wider than its own byte assertions: it checks decoded values/IDs but not full wire-byte equality; exact original-byte proof resides in other carriers. Q4 no: no-OPEN decoding is tagged only for one subtype and one classic announcement; the matching-OPEN matrix lacks a no-OPEN base negative and complete no-OPEN location/subtype coverage. Subtype/OPEN disagreement is untested, and the current producer silently prefers OPENs; RFC 8050 does not explicitly prescribe the conflict policy, so this is unspecified/unproven handling, not an invented requirement to reject every conflict. Formatting does not close these findings. Read reader.go::readRecords/dispatchBGP4MP, context.go::ObserveMessage/ObserveState/AddPathFor/updateContext, bgp.go::parseUpdate/parsePrefixesAFI, MP parsers, analyze count/display/filter producers and recorder subtype producers. Mixed-family/epoch support is implemented and meaningfully tested, not absent. session_context_boundaries_test.go has no RFC tags and does not enlarge this row's credited carriers or require a tagged-unit renewal. Preserve weak whole-clause coverage; no protocol expansion or runtime result is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674) | unit/verify | unproven |
| negative | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) | unit/verify | revert, verified |
| negative | [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665) | unit/verify | unproven |
| positive | [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16) | unit/verify | revert, verified |
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
