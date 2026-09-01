# RFC 6397 - Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions

No row in the public ledger. Every requirement this repository extracted from RFC 6397, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6397.md` |
| Requirement shard | `rfc/requirements/rfc6397.md` |
| RFC text | `rfc/full/rfc6397.txt` |

## Enrolment

Enrolled: MRT BGP Routing Information Export Format with Geo-Location Extensions: four MUST-level requirements, all {not-applicable} to Ze. Ze implements standard MRT (RFC 6396) only -- its writer emits TABLE_DUMP_V2 with a PEER_INDEX_TABLE and RIB entries (internal/plugins/mrt/dump.go:187 writePeerIndexTable, dump.go:200 TypeTableDumpV2/TDV2PeerIndexTable) -- and has no RFC 6397 geo-location extension (no GEO_PEER_TABLE, Collector/Peer Latitude/Longitude, WGS84, or location-object code path anywhere in internal/plugins/mrt/). RFC6397-4.1-1 (Collector coords not mixed WGS84/NAN) and RFC6397-4.1-2 (Peer coords not mixed) have no coordinate fields to constrain; RFC6397-4.1-3 (GEO_PEER_TABLE order matches PEER_INDEX_TABLE) has no GEO_PEER_TABLE; RFC6397-6-1 (privacy filtering of location info) has no location records to filter. The 4.1-4 MAY (NAN coordinates for privacy) is not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6397.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC6397-4.1-1`](#rfc6397-4.1-1), [`RFC6397-4.1-2`](#rfc6397-4.1-2), [`RFC6397-4.1-3`](#rfc6397-4.1-3), [`RFC6397-6-1`](#rfc6397-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6397-4.1-1` | Collector Latitude and Collector Longitude must not be a mix of WGS84 coordinates and NAN values (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 6397 MRT geo-location extension. Its MRT writer emits only standard RFC 6396 records -- TABLE_DUMP_V2 with a PEER_INDEX_TABLE and RIB entries (internal/plugins/mrt/dump.go:187 writePeerIndexTable, dump.go:200 TypeTableDumpV2/TDV2PeerIndexTable) -- and has no GEO_PEER_TABLE / Collector-Latitude / Collector-Longitude / WGS84 code path (grep for geo-location/latitude/longitude/WGS84 across internal/plugins/mrt/ finds nothing), so Ze never emits Collector coordinates that could be mixed with NAN. |
| `RFC6397-4.1-2` | Peer Latitude and Peer Longitude must not be a mix of WGS84 coordinates and NAN values for a single peer (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze emits no GEO_PEER_TABLE and no Peer-Latitude / Peer-Longitude fields (it writes only the standard RFC 6396 PEER_INDEX_TABLE, internal/plugins/mrt/dump.go:187), so there are no per-peer coordinates that could be mixed with NAN. |
| `RFC6397-4.1-3` | Order of Peer Entries in GEO_PEER_TABLE must match the order and number as existing in the PEER_INDEX_TABLE (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze emits no GEO_PEER_TABLE record at all (only the standard RFC 6396 PEER_INDEX_TABLE and RIB entries, internal/plugins/mrt/dump.go:187,200), so there is no GEO_PEER_TABLE ordering to keep consistent with the PEER_INDEX_TABLE. |
| `RFC6397-6-1` | Location information from a location object with more restrictive privacy rules must not be included in an MRT geo-location record unless non-technical measures are in place (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze generates no MRT geo-location records (it has no RFC 6397 GEO_PEER_TABLE / location-object code path in internal/plugins/mrt/), so there is no location information for it to include or filter on privacy grounds. |
| `RFC6397-4.1-4` | Coordinates may be NAN (IEEE 754) when geo-location is considered private (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6397-4.1-1`](#rfc6397-4.1-1) Collector Latitude and Collector Longitude must not be a mix of WGS84 coordinates and NAN values (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 6397 MRT geo-location extension. Its MRT writer emits only standard RFC 6396 records -- TABLE_DUMP_V2 with a PEER_INDEX_TABLE and RIB entries (internal/plugins/mrt/dump.go:187 writePeerIndexTable, dump.go:200 TypeTableDumpV2/TDV2PeerIndexTable) -- and has no GEO_PEER_TABLE / Collector-Latitude / Collector-Longitude / WGS84 code path (grep for geo-location/latitude/longitude/WGS84 across internal/plugins/mrt/ finds nothing), so Ze never emits Collector coordinates that could be mixed with NAN. |
| [`RFC6397-4.1-2`](#rfc6397-4.1-2) Peer Latitude and Peer Longitude must not be a mix of WGS84 coordinates and NAN values for a single peer (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze emits no GEO_PEER_TABLE and no Peer-Latitude / Peer-Longitude fields (it writes only the standard RFC 6396 PEER_INDEX_TABLE, internal/plugins/mrt/dump.go:187), so there are no per-peer coordinates that could be mixed with NAN. |
| [`RFC6397-4.1-3`](#rfc6397-4.1-3) Order of Peer Entries in GEO_PEER_TABLE must match the order and number as existing in the PEER_INDEX_TABLE (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze emits no GEO_PEER_TABLE record at all (only the standard RFC 6396 PEER_INDEX_TABLE and RIB entries, internal/plugins/mrt/dump.go:187,200), so there is no GEO_PEER_TABLE ordering to keep consistent with the PEER_INDEX_TABLE. |
| [`RFC6397-6-1`](#rfc6397-6-1) Location information from a location object with more restrictive privacy rules must not be included in an MRT geo-location record unless non-technical measures are in place (§6) | no test | no test carries this requirement id; annotated {not-applicable}: Ze generates no MRT geo-location records (it has no RFC 6397 GEO_PEER_TABLE / location-object code path in internal/plugins/mrt/), so there is no location information for it to include or filter on privacy grounds. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6397-4.1-1`](#rfc6397-4.1-1)

Collector Latitude and Collector Longitude must not be a mix of WGS84 coordinates and NAN values (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6397-4.1-1, so no unit is bound to it.

### [`RFC6397-4.1-2`](#rfc6397-4.1-2)

Peer Latitude and Peer Longitude must not be a mix of WGS84 coordinates and NAN values for a single peer (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6397-4.1-2, so no unit is bound to it.

### [`RFC6397-4.1-3`](#rfc6397-4.1-3)

Order of Peer Entries in GEO_PEER_TABLE must match the order and number as existing in the PEER_INDEX_TABLE (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6397-4.1-3, so no unit is bound to it.

### [`RFC6397-6-1`](#rfc6397-6-1)

Location information from a location object with more restrictive privacy rules must not be included in an MRT geo-location record unless non-technical measures are in place (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6397-6-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6397, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6397, so its obligations are stated where they were written.
