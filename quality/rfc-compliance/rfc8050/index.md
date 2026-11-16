# RFC 8050 - Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions

MRT ADD-PATH. Every requirement listed for RFC 8050, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Declared gap or no test | 0 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 31 | 31 of 36 tests linked to requirements, 0 without a fault test and 1 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial test coverage | 1 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Audit verdicts | 6 | of 6 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 6 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | bad | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | MRT ADD-PATH |
| Automatic checks | Included |
| Requirements | 6 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tests linked to requirements | 36 |
| Recorded audit verdicts | 6 |
| Fault test records | 32 |
| Summary | `rfc/short/rfc8050.md` |
| Requirement table | `rfc/requirements/rfc8050.md` |
| RFC text | `rfc/full/rfc8050.txt` |

## Automatic checks

Included in automatic checks: MRT Routing Information Export Format with BGP Additional Path Extensions: six MUST-level requirements. Tagged tests cover RIB entry layout (4.1-1), raw RIB_GENERIC NLRI (4.2-1), big-endian Path Identifiers (x-2), unchanged complete BGP4MP messages (x-3), subtype and actual directional OPEN context (x-4), and negotiated directional subtype selection (x-1). The x-2 numeric encoding row is {single-polarity: positive}; other rows carry positive and negative claims.

## Published support status

**Area:** MRT ADD-PATH

**Test verification:** 5 of 6 MUST proven

**Evidence:** unit 5

**Unmet:** RFC8050-4.1-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 1 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC8050-x-1`](#rfc8050-x-1), [`RFC8050-4.2-1`](#rfc8050-4.2-1), [`RFC8050-x-3`](#rfc8050-x-3), [`RFC8050-x-4`](#rfc8050-x-4)

**Annotated (including scoped evidence) (2):** [`RFC8050-4.1-1`](#rfc8050-4.1-1), [`RFC8050-x-2`](#rfc8050-x-2)

**Partial test coverage (subset of annotated; zero whole-requirement credit) (1):** [`RFC8050-4.1-1`](#rfc8050-4.1-1)

## Figures

- **MUST bar**: Ze verified 5, Not built 1
- **Overview bar**: Ze MUST verified 5, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 5 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8050.json](../../../data/rfc/rfc8050.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8050-x-1` | The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L34). **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L23). **negative:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L35). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) |
| `RFC8050-4.1-1` | the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Attribute Length \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L167). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L264). **negative:** `unit/verify` [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L216). **{partial}:** tested "the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows"; gap "This adds a field to the RIB Entries record to store the Path Identifier"; the record layout is met on both sides, proven by the tags: the reader parses the Path Identifier field and WriteRIBEntries places it. The TABLE_DUMP_V2 snapshot does not store the route's Path Identifier in it: internal/plugins/mrt/dump.go::writeTableDumpV2 writes an ADDPATH subtype when the operator toggle is set, but builds each RIBEntry with no PathID, because the RIB dump callback carries none, so every entry's field holds 0 and two paths of one prefix from one peer cannot be told apart. **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Rejudged 2026-10-30 (independent auditor) after the row gained a {partial} scope. RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes." Tested: the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows. TestRFC8050AddPathRIBEntryFollowsFigure1 compares WriteRIBEntries output with the literal Figure 1 octets and decodes them under subtypes 8-11; TestRIBRecordAddPathRoundTrip round-trips PathID 1 and 2 under subtype 8; TestRIBRecordRoundTrip (negative) reads no Path Identifier under base subtype 2. Gap: This adds a field to the RIB Entries record to store the Path Identifier. Producer internal/plugins/mrt/dump.go::writeTableDumpV2 builds each mrtfmt.RIBEntry from PeerIndex, OrigTime and Attributes only, because registry.RIBDumpVisitor.OnRoute carries no path id, so under the add-path toggle every entry's field is written as 0 and the route's Path Identifier is not stored. Revert records cover all three tagged units. |
| `RFC8050-4.2-1` | These fields continue to encapsulate the raw and additional-path- enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries. For clarity, the RIB Entries in this subtype are not redefined. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L515). **negative:** `unit/verify` [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L312) |
| `RFC8050-x-2` | 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L173). **positive:** `unit/verify` [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L268). **negative:** no negative test. **{single-polarity}:** the Path ID is written and read big-endian (internal/mrt/encode.go:107, decode.go:295); there is no alternate-endianness path |
| `RFC8050-x-3` | The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L367). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L99). **negative:** `unit/verify` [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L427). **negative:** `unit/verify` [`TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc8050_addpath_record_test.go#L31) |
| `RFC8050-x-4` | MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665). **positive:** `unit/verify` [`TestMRTCollisionWinnerReservation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L54). **positive:** `unit/verify` [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L19). **positive:** `unit/verify` [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L27). **positive:** `unit/verify` [`TestMRTMigrationEpochUsesActualLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_migration_epoch_test.go#L27). **positive:** `unit/verify` [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L36). **positive:** `unit/verify` [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22). **positive:** `unit/verify` [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17). **positive:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87). **positive:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22). **positive:** `unit/verify` [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L21). **positive:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L72). **positive:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16). **negative:** `unit/verify` [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674). **negative:** `unit/verify` [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L20). **negative:** `unit/verify` [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L28). **negative:** `unit/verify` [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88). **negative:** `unit/verify` [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25). **negative:** `unit/verify` [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L22). **negative:** `unit/verify` [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73). **negative:** `unit/verify` [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8050-4.1-1`](#rfc8050-4.1-1) the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Peer Index \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Originated Time \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Path Identifier \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| Attribute Length \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1) | {partial}, Partial test coverage | tested "the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows"; gap "This adds a field to the RIB Entries record to store the Path Identifier"; the record layout is met on both sides, proven by the tags: the reader parses the Path Identifier field and WriteRIBEntries places it. The TABLE_DUMP_V2 snapshot does not store the route's Path Identifier in it: internal/plugins/mrt/dump.go::writeTableDumpV2 writes an ADDPATH subtype when the operator toggle is set, but builds each RIBEntry with no PathID, because the RIB dump callback carries none, so every entry's field holds 0 and two paths of one prefix from one peer cannot be told apart |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8050-x-1`](#rfc8050-x-1)

The Advertisement of Multiple Paths [RFC7911] extension for BGP alters the encoding of the BGP Network Layer Reachability Information (NLRI) format for withdraws and announcements. Therefore, new BGP4MP/BGP4MP_ET subtypes as defined in [RFC6396] are required to signal to an MRT parser how to parse the NLRI. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudgment against the complete RFC 8050, Sections 2 and 3. Section 3 says, "These enhancements continue to encapsulate the entire BGP message in the BGP message field." The live collision test retains the winning TCP connection and exact directional OPEN occurrences, then compares complete isolated classic and MP announcements and withdrawals against received subtypes 9/4 and sent subtypes 11/7. Its ordinary-family controls distinguish per-family and per-direction ADD-PATH from a global flag. The observer matrix separately covers both BGP4MP and BGP4MP_ET, every NLRI location, exact record count, subtype and unchanged complete message bytes. The changed collision helper now reports full-stream context ambiguity explicitly; its separately retained known-connection context uses only the actual winning OPEN occurrences, never synthesized settings. This change does not remove the subtype or byte-preservation assertions. observeReceivedWire reads the receive context; the retained writer carries its own send context and transport. OnBGPMessage selects updateAddPath from the actual UPDATE's family fields and preserves the original message. No TABLE_DUMP_V2 or new foreign-parser execution is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L23) | unit/verify | revert, verified |

### [`RFC8050-4.1-1`](#rfc8050-4.1-1)

the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows: 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Peer Index | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Originated Time | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Path Identifier | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Attribute Length | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | BGP Attributes... (variable) +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ Figure 1: RIB Entries for AFI/SAFI-Specific RIB Subtypes with Support for Additional Paths This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes. (§4.1)

Records cover only part of this requirement: tested "the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows"; gap "This adds a field to the RIB Entries record to store the Path Identifier"; the record layout is met on both sides, proven by the tags: the reader parses the Path Identifier field and WriteRIBEntries places it. The TABLE_DUMP_V2 snapshot does not store the route's Path Identifier in it: internal/plugins/mrt/dump.go::writeTableDumpV2 writes an ADDPATH subtype when the operator toggle is set, but builds each RIBEntry with no PathID, because the RIB dump callback carries none, so every entry's field holds 0 and two paths of one prefix from one peer cannot be told apart; they do not verify the whole requirement.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Rejudged 2026-10-30 (independent auditor) after the row gained a {partial} scope. RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes." Tested: the existing RIB Entries field is redefined for use within the new AFI/SAFI-specific RIB subtypes defined by this document as follows. TestRFC8050AddPathRIBEntryFollowsFigure1 compares WriteRIBEntries output with the literal Figure 1 octets and decodes them under subtypes 8-11; TestRIBRecordAddPathRoundTrip round-trips PathID 1 and 2 under subtype 8; TestRIBRecordRoundTrip (negative) reads no Path Identifier under base subtype 2. Gap: This adds a field to the RIB Entries record to store the Path Identifier. Producer internal/plugins/mrt/dump.go::writeTableDumpV2 builds each mrtfmt.RIBEntry from PeerIndex, OrigTime and Attributes only, because registry.RIBDumpVisitor.OnRoute carries no path id, so under the add-path toggle every entry's field is written as 0 and the route's Path Identifier is not stored. Revert records cover all three tagged units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRIBRecordRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L264) | unit/verify | revert, verified |
| positive | [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L167) | unit/verify | revert, verified |

### [`RFC8050-4.2-1`](#rfc8050-4.2-1)

These fields continue to encapsulate the raw and additional-path- enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries. For clarity, the RIB Entries in this subtype are not redefined. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: reading RIB_GENERIC_ADDPATH (subtype 12) RIB Entries as redefined (a Path Identifier in each entry), or losing the Path Identifier that belongs in the raw NLRI. TestRIBGenericAddPathRoundTrip hand-builds the bytes the RFC describes (Path ID 42 at the head of the NLRI blob, then an entry with Peer Index, Originated Time, Attribute Length, attributes and no Path ID) and requires rec.NLRI == nlri and Entries[0].Attributes == attrs; a decoder that read a 4-octet Path ID from the entry would take 0x0004 plus attribute bytes as the path id and misparse the attribute length, so the attribute comparison or the decode goes red. The negative TestRIBGenericRoundTrip requires the base subtype 6 NLRI to decode verbatim. Ze writes no subtype 12 record (internal/mrt/encode.go writes none; internal/mrt/decode.go is the only producer), so the decode path is the whole surface.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRIBGenericRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L312) | unit/verify | revert, verified |
| positive | [`TestRIBGenericAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L515) | unit/verify | revert, verified |

### [`RFC8050-x-2`](#rfc8050-x-2)

0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Peer Index | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Originated Time | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | Path Identifier | (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rejudge 2026-10-02 (c33 judge): {single-polarity: positive} (Figure 1 has no alternative encoding to refuse). TestRFC8050AddPathRIBEntryFollowsFigure1 pins the 32-bit field in network order by literal octets 01 02 03 04 on write and value 0x01020304 on read; four distinct octets and a value above 0xFFFF expose a 16-bit width or a reversed order. Judge break: a 16-bit read of the field turns it red, HEAD TestRIBRecordAddPathRoundTrip (PathID 1 and 2) stays green, which was the earlier gap.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRIBRecordAddPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L268) | unit/verify | not verified |
| positive | [`TestRFC8050AddPathRIBEntryFollowsFigure1`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L173) | unit/verify | revert, verified |

### [`RFC8050-x-3`](#rfc8050-x-3)

The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 8050 Section 3: "The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field." 238bdb62e9 (spec-refuse-instead-of-defaulting) changed PeerInfo/StructuredEvent PeerAS from uint32 to rpc.RemoteAS; the unit now writes rpc.RemoteASOf(N) where it wrote N, the same known peer AS, and no assertion, input byte or expected value changed (TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout). Verdict unchanged: enforced. Earlier judgement follows. Independent final source rejudgment after formatting. RFC 8050 section 3: "The fields of these message types are identical to the equivalent non-additional-path versions specified in Section 4.4 of [RFC6396]. These enhancements continue to encapsulate the entire BGP message in the BGP message field." Read the full section and RFC 6396 section 4.4 base layouts, all four tagged carriers and actual producers. Q1 yes collectively: internal/mrt/rfc8050_layout_test.go::TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields feeds independently literal records through ReadFrom for subtypes 8/9/10/11, checks every base header field and all 45 UPDATE octets, and compares base-subtype readings. internal/mrt/mrt_test.go::TestBGP4MPMessageRoundTrip checks preserved fields and the complete literal Path-ID-bearing UPDATE. internal/plugins/mrt/rfc8050_addpath_record_test.go::TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout calls the actual recorder in both directions and asserts exactly two records, each byte after its timestamp, subtype 9/11, length 65, exactly 20 base-header octets and the complete 45-octet UPDATE. Q2 yes for those exact independent reader/writer assertions: lifting the Path Identifier into an extra MRT field, shifting fields or dropping/reconstructing message octets fails. The supplementary internal/mrt/mrt_test.go::TestBGP4MPStateChangeRoundTrip is only a roundtrip control over the shared header and cannot alone prove absence of a newly inserted field; its negative tag does not establish an independent adversarial ADDPATH case. Q3 yes for the effective carriers: a complete framed UPDATE containing a real Path Identifier isolates the temptation to add an MRT field; expected record length and all expected octets are independent of production encoders. The writer negative asserts forbidden output absence, not malformed-input rejection. Q4 yes collectively for unchanged fields and complete encapsulation; the weak state-change control is not used to confer that credit. internal/mrt/encode.go::WriteBGP4MPMessage/writeBGP4MPCommon and decode.go::DecodeBGP4MPMessage/decodeBGP4MPHeader implement the common layout, and internal/plugins/mrt/component.go::OnBGPMessage copies the complete original frame. Formatting the plugin carrier changes no assertion or expected octet. This is source judgment only; no discrimination renewal or test run is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBGP4MPStateChangeRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L427) | unit/verify | not verified |
| negative | [`TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc8050_addpath_record_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestBGP4MPMessageRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L367) | unit/verify | revert, verified |
| positive | [`TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_layout_test.go#L99) | unit/verify | revert, verified |

### [`RFC8050-x-4`](#rfc8050-x-4)

MRT parsers are usually stateless. In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized. (§2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/incoming_candidate.go::adoptIncoming moved. Independent rejudgment against RFC 8050 Section 2: "In order to parse BGP messages that contain data structures that depend on the capabilities negotiated during the BGP session setup, the MRT subtypes are utilized." Read all current tagged units and their assertion-producing helpers. Subtype-only controls decode all four ADD-PATH and four ordinary BGP4MP forms under both timestamp types without OPENs, preserving exact prefixes, identifiers including zero, and complete bytes. Mixed-family/direction controls require actual matching directional OPENs and refuse missing, stale, repeated, unrelated, identity-mismatched or independently opened context rather than guessing a mode. Real command handlers preserve that refusal and decode the exact mixed route counts with valid context. The changed live collision carrier preserves its original winner socket and OPEN occurrences at both handoff barriers. BGP4MP has no TCP ports: the full overlapping stream must report context unavailable; a separate known-winner decoder is explicitly seeded only from that TCP connection's two captured OPEN occurrences and excludes the observed old-socket Cease. It still asserts exact directional mixed prefixes and Path Identifiers, isolated subtype controls and record counts. Adoption retains the original Session rather than taking the old acceptWithOpen path. The migration carrier also pins the actual fallback OPEN identity and retained writer epoch. These are data/connection evidence assertions, not a claim that ambiguous overlapping TCP epochs can be inferred from one MRT endpoint tuple.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L674) | unit/verify | not verified |
| negative | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L17) | unit/verify | revert, verified |
| negative | [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L22) | unit/verify | revert, verified |
| negative | [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L20) | unit/verify | revert, verified |
| negative | [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestMRTEmptyMPFamilyRequiresContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_boundary_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsReadSubtypeContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_commands_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC8050CommandsRequireMixedContext`](https://github.com/ze-software/ze/blob/main/internal/analyze/rfc8050_session_commands_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestMRTCollisionWinnerReservation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L54) | unit/verify | mutant, verified |
| positive | [`TestMRTWinningCollisionPreservesOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_collision_epoch_test.go#L36) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestMRTMigrationEpochUsesActualLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_migration_epoch_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC8050MessageCallbackPreservesEncodingAndFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8050_mrt_observer_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestIsAddPathHelpers`](https://github.com/ze-software/ze/blob/main/internal/mrt/mrt_test.go#L665) | unit/verify | not verified |
| positive | [`TestRFC8050AddPathBGP4MPUpdateNLRIReadBySubtype`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_addpath_nlri_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC8050SubtypeControlsEveryNLRILocation`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_context_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestRFC8050NoOPENSubtypeDecodesExactNLRI`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_no_open_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestMRTContextCapabilityASNBoundaries`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_boundaries_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC8050SessionContextDecodesMixedDirections`](https://github.com/ze-software/ze/blob/main/internal/mrt/rfc8050_session_context_test.go#L72) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the IETF Trust Copyright Notice and constrains how code extracted from the document is licensed, not the behavior of an MRT writer or parser. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Document roadmap: 'The following two sections define the required subtypes' names what Sections 3 and 4 contain. 'required' is an adjective on the subtypes the previous paragraphs called for, and the sentence imposes nothing on an implementation. The obligation it points at is carried by site 2:1, mapped to RFC8050-x-1. | The following two sections define the required subtypes. |

## Superseded

No document obsoletes RFC 8050, so its obligations are stated where they were written.
