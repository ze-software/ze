# RFC 6397 - Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions

MRT geo-location. Every requirement listed for RFC 6397, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 8 | 8 of 8 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 4 | of 4 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | MRT geo-location |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tests linked to requirements | 8 |
| Recorded audit verdicts | 4 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc6397.md` |
| Requirement table | `rfc/requirements/rfc6397.md` |
| RFC text | `rfc/full/rfc6397.txt` |

## Automatic checks

Included in automatic checks: MRT BGP Routing Information Export Format with Geo-Location Extensions: four MUST-level requirements, all implemented and proven in both polarities. Ze implements TABLE_DUMP_V2 GEO_PEER_TABLE (subtype 7) in internal/mrt/geo.go, internal/mrt/decode.go, and internal/plugins/mrt/{dump.go,register.go,config.go,yang/ze-mrt-conf.yang}: ValidateGeoCoordPair, WriteGeoPeerTable, DecodeGeoPeerTable, and ParseConfig enforce no WGS84/NaN mixing for Collector (RFC6397-4.1-1) and Peer (RFC6397-4.1-2); BuildGeoPeerTableForIndex, writeGeoPeerTable, and ValidateGeoPeerTableAgainstIndex preserve and verify exact peer order and count against PEER_INDEX_TABLE (RFC6397-4.1-3); GeoPrivacyRules.AllowInclude excludes location information with more restrictive GEOPRIV privacy rules unless non-technical measures are confirmed (RFC6397-6-1). The 4.1-4 MAY (NAN coordinates for privacy) is not gated.

## Published support status

**Area:** MRT geo-location

**Test verification:** 4 of 4 MUST proven

**Evidence:** unit 4

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC6397-4.1-1`](#rfc6397-4.1-1), [`RFC6397-4.1-2`](#rfc6397-4.1-2), [`RFC6397-4.1-3`](#rfc6397-4.1-3), [`RFC6397-6-1`](#rfc6397-6-1)

## Figures

- **MUST bar**: Ze verified 4
- **Overview bar**: Ze MUST verified 4, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 4 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6397.json](../../../data/rfc/rfc6397.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6397-4.1-1` | The Collector Latitude and Collector Longitude MUST NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L90). **negative:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L91) |
| `RFC6397-4.1-2` | The Peer Latitude and Peer Longitude MUST NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values for a single peer. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L92). **negative:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L93) |
| `RFC6397-4.1-3` | The order of the Peer Entries in the GEO_PEER_TABLE MUST match the order and number as existing in the PEER_INDEX_TABLE. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L94). **negative:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L95) |
| `RFC6397-6-1` | Location information derived from a location object with more restrictive privacy rules MUST NOT be included in an MRT geo-location record unless there are non-technical measures in place that enforce and communicate the constraints on the use of the location information in the MRT geo-location format (e.g., contractual agreements between peers). (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L96). **negative:** `unit/verify` [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L97) |
| `RFC6397-4.1-4` | The latitude and longitude MAY be a Not A Number (NAN) [IEEE754] for situations where the geo-location of the collector is considered private. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6397 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6397-4.1-1`](#rfc6397-4.1-1)

The Collector Latitude and Collector Longitude MUST NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Production code (ValidateGeoCoordPair, WriteGeoPeerTable, DecodeGeoPeerTable, BuildGeoPeerTableForIndex, parseGeoEntity) rejects mixed collector WGS84/NaN with ErrGeoMixedCoords while accepting both paired WGS84 and paired private (NaN, NaN). Positive tests reject both mixed directions across ParseConfig, WriteGeoPeerTable, and DecodeGeoPeerTable with isolated 14-byte wire frames; negative tests prove both paired WGS84 (48.8566, 2.3522) and paired private (NaN, NaN) collectors round-trip end-to-end through ParseConfig -> DumpRIB -> mrt.ReadFile. Discrimination proofs cite real producers (ValidateGeoCoordPair, WriteGeoPeerTable).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L90) | unit/verify | revert, verified |

### [`RFC6397-4.1-2`](#rfc6397-4.1-2)

The Peer Latitude and Peer Longitude MUST NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values for a single peer. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Production code (ValidateGeoCoordPair, WriteGeoPeerTable, DecodeGeoPeerTable, BuildGeoPeerTableForIndex, parseGeoEntity) rejects mixed single-peer WGS84/NaN while permitting paired WGS84, paired NaN, and independent WGS84 vs NaN across different peers. Positive polarity rejects both mixed directions via ParseConfig and mixed single-peer inputs in WriteGeoPeerTable and DecodeGeoPeerTable with exact ErrGeoMixedCoords (isolated by valid (NaN, NaN) collector and valid 27-byte frame); negative polarity proves paired WGS84 (192.0.2.1) and paired private NaN (2001:db8::2) peers coexist and round-trip end-to-end through ParseConfig -> DumpRIB -> ReadFile. Discrimination proofs cite real producers (ValidateGeoCoordPair, WriteGeoPeerTable).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L92) | unit/verify | revert, verified |

### [`RFC6397-4.1-3`](#rfc6397-4.1-3)

The order of the Peer Entries in the GEO_PEER_TABLE MUST match the order and number as existing in the PEER_INDEX_TABLE. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Tx path (writePeerIndexTable -> writeGeoPeerTable -> BuildGeoPeerTableForIndex) emits GEO_PEER_TABLE matching PEER_INDEX_TABLE peer count, order, Type, and BGP ID end-to-end through ParseConfig -> DumpRIB -> mrt.ReadFile (including 7 mixed IPv4/IPv6 peers in non-alphabetical order and unconfigured peers defaulting to NaN, NaN). Rx path wires ValidateGeoPeerTableAgainstIndex directly into internal/mrt/reader.go (readRecords tracks *PeerIndexTable and dispatchTDV2 validates TDV2GeoPeerTable against it on read). Negative tests prove mrt.ReadFrom rejects swapped Peer Type (peer 0 IPv4 vs peer 1 IPv6), swapped same-Type BGP ID (peer 2 IPv4 vs peer 3 IPv4), and truncated peer count with ErrGeoPeerTableMismatch. Discrimination proofs cite real live producers (BuildGeoPeerTableForIndex, ValidateGeoPeerTableAgainstIndex).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L94) | unit/verify | revert, verified |

### [`RFC6397-6-1`](#rfc6397-6-1)

Location information derived from a location object with more restrictive privacy rules MUST NOT be included in an MRT geo-location record unless there are non-technical measures in place that enforce and communicate the constraints on the use of the location information in the MRT geo-location format (e.g., contractual agreements between peers). (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Production code (GeoPrivacyRules.IsMoreRestrictive/AllowInclude, BuildGeoPeerTableForIndex, parseGeoEntity, ze-mrt-conf.yang) enforces all four RFC 6397 Section 6 GEOPRIV default rules and the non-technical-measures override. Positive polarity proves each restrictive rule independently excludes coordinates (emitted as NaN, NaN) end-to-end through ParseConfig -> DumpRIB -> mrt.ReadFile with non-technical-measures=false: retransmission-allowed=false (peer 2), retention-years=99 boundary <100 (peer 3), non-empty ruleset-reference (peer 4), and non-empty note-well (peer 5 and collector). Negative polarity proves default privacy (peer 0) and restrictive rules with non-technical-measures=true (peer 6) include WGS84 coordinates end-to-end. Discrimination proofs cite GeoPrivacyRules.AllowInclude.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC6397GeoPeerTableCoordsOrderAndPrivacy`](https://github.com/ze-software/ze/blob/main/internal/plugins/mrt/rfc6397_geo_test.go#L96) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6397.txt |
| Source fingerprint | ac8d364817918651 |
| Record | rfc/extraction/rfc6397.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 3 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 0 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 6397 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 6397, so its obligations are stated where they were written.
