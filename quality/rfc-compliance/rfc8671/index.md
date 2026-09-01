# RFC 8671 - Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP)

Supported within BMP sender scope. Every requirement this repository extracted from RFC 8671, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.0% | 7 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.0% | 1 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 2 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported within BMP sender scope |
| Enrolment | Enrolled |
| Requirements | 12 |
| Gated MUST-level | 10 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 1 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8671.md` |
| Requirement shard | `rfc/requirements/rfc8671.md` |
| RFC text | `rfc/full/rfc8671.txt` |

## Enrolment

Enrolled: Support for Adj-RIB-Out in BMP: three MUSTs over the BMP Per-Peer Header (internal/component/bgp/plugins/bmp). RFC8671-x-1 (O flag is bit 4) both polarities: PeerFlagO == 1<<4 and a flags byte with bit 4 set decodes as Adj-RIB-Out while bit 3 does not (TestRFC8671OFlagBit4). RFC8671-x-2 (when O=1 the L flag is also set) both polarities: the sent direction sets O and L (TestPeerHeaderFromEventAdjRIBOut), the received direction sets neither (TestPeerHeaderFromEventAdjRIBIn). RFC8671-x-3 (Adj-RIB-Out Peer Up carries the same OPENs as Adj-RIB-In) is {single-polarity: positive}: ze sources both OPENs from the per-peer openCache (bmp.go:757-772) independent of the O flag, proven round-tripping in TestBMPPeerUpRoundTrip. Ledger row unchanged (no gap).

## What the public ledger says

**Status:** Supported within BMP sender scope

**What the ledger says is covered:**

Adj-RIB-Out direction flag handling and sent-route monitoring.

**What the ledger says remains**

One MUST is a gap. Section 6.2 requires the O flag to be zero on a Statistics Report, and ze sends no Statistics Report at all: `senderSession.writeStatisticsReport` clears the flag correctly and has no non-test caller, and the `statistics-timeout` leaf drives no timer, so the obligation is never exercised ([`RFC8671-6.2-1`](#rfc8671-6.2-1)). One feature is absent by decision, and it is not a conformance gap: ze exports the post-policy Adj-RIB-Out view only, and the pre-policy Adj-RIB-Out view is not built. RFC 7854 Section 5 leaves that choice to the implementation, so the obligations that hang on it do not bind ze ([`RFC8671-5.2-1`](#rfc8671-5.2-1)). A later scope decision can revisit it and build the pre-policy view.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 1 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC8671-x-1`](#rfc8671-x-1), [`RFC8671-x-2`](#rfc8671-x-2), [`RFC8671-4-1`](#rfc8671-4-1), [`RFC8671-5.1-1`](#rfc8671-5.1-1), [`RFC8671-6.1-1`](#rfc8671-6.1-1), [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1), [`RFC8671-7.2-1`](#rfc8671-7.2-1)

**Annotated instead of tested (2):** [`RFC8671-x-3`](#rfc8671-x-3), [`RFC8671-6.2-1`](#rfc8671-6.2-1)

**No test and no annotation (1):** [`RFC8671-5.2-1`](#rfc8671-5.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8671-x-1` | O flag uses bit 4 of the Per-Peer Header Flags field (Key Constraints) | MUST | x | **positive:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219). **negative:** `unit/verify` [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) |
| `RFC8671-x-2` | When O=1 (Adj-RIB-Out), the L flag (bit 6, post-policy) is also set (Peer Up Behavior) | MUST | x | **positive:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L57). **negative:** `unit/verify` [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L80) |
| `RFC8671-x-3` | Peer Up message for Adj-RIB-Out (O=1) carries the same sent and received OPEN messages as for Adj-RIB-In (Peer Up Behavior) | MUST | x | **positive:** `unit/verify` [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/msg_test.go#L103). **negative:** no negative test. **{single-polarity}:** ze builds every Peer Up from the peer's cached sent and received OPEN messages (internal/component/bgp/plugins/bmp/bmp.go:757-772, pair.sent/pair.received) regardless of the O flag, so an Adj-RIB-Out Peer Up carries the same OPENs as an Adj-RIB-In one by construction; there is no "different OPENs for Adj-RIB-Out" case to assert as a negative. The positive (a Peer Up round-trips its sent/received OPENs) is proven in TestBMPPeerUpRoundTrip |
| `RFC8671-x-4` | A single BGP session MAY produce two Peer Up messages: one for Adj-RIB-In (O=0) and one for Adj-RIB-Out (O=1) (Peer Up Behavior) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-x-5` | Pre-policy Adj-RIB-Out (L=0, O=1) is valid but uncommon (Key Constraints) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-4-1` | The Per-Peer Header Flags bits reserved for future use MUST be transmitted as 0, and their values MUST be ignored on receipt (§4) | MUST | 4 - Per-Peer Header | **positive:** `unit/verify` [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66). **negative:** `unit/verify` [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) |
| `RFC8671-5.1-1` | Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1) | MUST | 5.1 - Post-policy | **positive:** `unit/verify` [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164). **negative:** `unit/verify` [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229). **negative:** `unit/verify` [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) |
| `RFC8671-5.2-1` | The L flag MUST be set to 0 to indicate pre-policy (§5.2) | MUST | 5.2 - Pre-policy | **positive:** no positive test. **negative:** no negative test |
| `RFC8671-6.1-1` | The O flag MUST be set accordingly to indicate if the Route Monitoring or Route Mirroring message conveys Adj-RIB-In or Adj-RIB-Out (§6.1) | MUST | 6.1 - Route Monitoring and Route Mirroring | **positive:** `unit/verify` [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268). **negative:** `unit/verify` [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) |
| `RFC8671-6.2-1` | Statistics Report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero (§6.2) | MUST | 6.2 - Statistics Report | **positive:** no positive test. **negative:** no negative test. **{gap}:** the requirement implies a Statistics Report is created, and ze creates none. `senderSession.writeStatisticsReport` (internal/component/bgp/plugins/bmp/sender.go) is the only production encoder of one and it clears the O flag correctly, but it has no non-test caller, so no ze BMP session puts a Statistics Report on the wire and the obligation is never exercised. The missing piece is the timer: the `statistics-timeout` leaf (internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang) is parsed into `senderConfig.StatisticsTimeout`, carried into `senderBehavior.statistics` by `behaviorOf` (sender_config.go) so a later timer bounces the session per §7.2, and read by no ticker. Owner ruling, 2026-08-31: an emission path added later that produced a non-zero O flag would redden nothing, so the encoder test is not proof of conformance |
| `RFC8671-6.3.1-1` | When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1) | MUST | 6.3.1 - Peer Up Information | **positive:** `unit/verify` [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L458). **negative:** `unit/verify` [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L475) |
| `RFC8671-7.2-1` | A change that alters the behavior of an existing BMP session MUST bounce that session with a Peer Down/Peer Up sequence; ze keeps the BMP session up and sends a Peer Down (reason 5, configuration reasons) then a Peer Up for every established peer reported on it. The bounce is owed to a CHANGE, so ze compares the parsed sender configuration against the one in force and acts only when a leaf deciding what the session carries has moved. The change arrives on the plugin config-apply callback, and the tests drive that callback rather than the function behind it (§7.2) | MUST | 7.2 - Changes to Existing BMP Session | **positive:** `unit/verify` [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L804). **negative:** `unit/verify` [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L963). **negative:** `unit/verify` [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L913) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8671-5.2-1`](#rfc8671-5.2-1) The L flag MUST be set to 0 to indicate pre-policy (§5.2) | no test | no test carries this requirement id |
| [`RFC8671-6.2-1`](#rfc8671-6.2-1) Statistics Report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero (§6.2) | {gap}, no test | the requirement implies a Statistics Report is created, and ze creates none. `senderSession.writeStatisticsReport` (internal/component/bgp/plugins/bmp/sender.go) is the only production encoder of one and it clears the O flag correctly, but it has no non-test caller, so no ze BMP session puts a Statistics Report on the wire and the obligation is never exercised. The missing piece is the timer: the `statistics-timeout` leaf (internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang) is parsed into `senderConfig.StatisticsTimeout`, carried into `senderBehavior.statistics` by `behaviorOf` (sender_config.go) so a later timer bounces the session per §7.2, and read by no ticker. Owner ruling, 2026-08-31: an emission path added later that produced a non-zero O flag would redden nothing, so the encoder test is not proof of conformance |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8671-x-1`](#rfc8671-x-1)

O flag uses bit 4 of the Per-Peer Header Flags field (Key Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L222) | unit/verify | unproven |
| positive | [`TestRFC8671OFlagBit4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L219) | unit/verify | unproven |

### [`RFC8671-x-2`](#rfc8671-x-2)

When O=1 (Adj-RIB-Out), the L flag (bit 6, post-policy) is also set (Peer Up Behavior)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerHeaderFromEventAdjRIBIn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L80) | unit/verify | unproven |
| positive | [`TestPeerHeaderFromEventAdjRIBOut`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L57) | unit/verify | unproven |

### [`RFC8671-x-3`](#rfc8671-x-3)

Peer Up message for Adj-RIB-Out (O=1) carries the same sent and received OPEN messages as for Adj-RIB-In (Peer Up Behavior)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPPeerUpRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/msg_test.go#L103) | unit/verify | unproven |

### [`RFC8671-4-1`](#rfc8671-4-1)

The Per-Peer Header Flags bits reserved for future use MUST be transmitted as 0, and their values MUST be ignored on receipt (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671ReservedPeerFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC8671ReservedPeerFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L66) | unit/verify | unproven |

### [`RFC8671-5.1-1`](#rfc8671-5.1-1)

Post-policy Adj-RIB-Out MUST convey to the BMP receiver what is actually transmitted to the peer (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L229) | unit/verify | unproven |
| negative | [`TestRFC8671PostPolicyConveysUnknownAttributeUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC8671PostPolicyConveysTransmittedBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L164) | unit/verify | unproven |

### [`RFC8671-5.2-1`](#rfc8671-5.2-1)

The L flag MUST be set to 0 to indicate pre-policy (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8671-5.2-1, so no unit is bound to it.

### [`RFC8671-6.1-1`](#rfc8671-6.1-1)

The O flag MUST be set accordingly to indicate if the Route Monitoring or Route Mirroring message conveys Adj-RIB-In or Adj-RIB-Out (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671OFlagClearOnAdjRIBInMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L306) | unit/verify | unproven |
| positive | [`TestRFC8671OFlagSetOnAdjRIBOutMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L268) | unit/verify | unproven |

### [`RFC8671-6.2-1`](#rfc8671-6.2-1)

Statistics Report messages are not specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8671-6.2-1, so no unit is bound to it.

### [`RFC8671-6.3.1-1`](#rfc8671-6.3.1-1)

When multiple Admin Labels are included, the BMP receiver MUST preserve their order (§6.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671AdminLabelReversedOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L475) | unit/verify | unproven |
| positive | [`TestRFC8671AdminLabelOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L458) | unit/verify | unproven |

### [`RFC8671-7.2-1`](#rfc8671-7.2-1)

A change that alters the behavior of an existing BMP session MUST bounce that session with a Peer Down/Peer Up sequence; ze keeps the BMP session up and sends a Peer Down (reason 5, configuration reasons) then a Peer Up for every established peer reported on it. The bounce is owed to a CHANGE, so ze compares the parsed sender configuration against the one in force and acts only when a leaf deciding what the session carries has moved. The change arrives on the plugin config-apply callback, and the tests drive that callback rather than the function behind it (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8671RemovingEveryCollectorBouncesTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L963) | unit/verify | unproven |
| negative | [`TestRFC8671UnrelatedBGPChangeBouncesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L913) | unit/verify | unproven |
| positive | [`TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc8671_test.go#L804) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 4, rfc8671 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc8671.txt |
| Source fingerprint | aed5ce37b74c3a4d |
| Record | rfc/extraction/rfc8671.json |
| Mapped sentences | 7 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: what BMP monitors today, why an operator cannot see what it advertises, and that this document updates the per-peer header of RFC 7854 Section 4.2 with a new flag. No sentence directs a speaker. |
| `2` | Terminology | 0 | walked | Terminology. The BCP 14 key-words paragraph, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Definitions | 1 | walked | Definitions. Three terms: Adj-RIB-Out quoted from RFC 4271, pre-policy Adj-RIB-Out and post-policy Adj-RIB-Out. The one capitalised keyword sits in the post-policy definition and repeats section 5.1 word for word, so it is excluded below as a duplicate. |
| `4` | Per-Peer Header | 1 | walked | Per-Peer Header. Adds the O flag to the RFC 7854 Section 4.2 flags field and redefines Peer Address, Peer AS, Peer BGP ID and Timestamp for O=1. The bit position and the meaning of the flag are stated indicatively ('The O flag indicates Adj-RIB-In if set to 0 and Adj-RIB-Out if set to 1') and in the bit diagram, so section 2 gives them no RFC 2119 level and the site scan cannot see them; RFC8671-x-1 is read from there and is listed as unsourced. The one site is the reserved-bits sentence, mapped below. The four redefined fields are value definitions carried by the O Flag table of rfc/short/rfc8671.md, not directives. |
| `5` | Adj-RIB-Out | 0 | walked | Adj-RIB-Out. A heading with no body text: 5.1 and 5.2 carry the whole section. |
| `5.1` | Post-policy | 2 | walked | Post-policy. States the primary use case, then two capitalised MUSTs, both mapped below. The rest is indicative: which attributes are set at transmission time, and what post-policy reflects. |
| `5.2` | Pre-policy | 2 | walked | Pre-policy. Two capitalised MUSTs, both excluded below as binding a role ze does not implement. The remaining prose is indicative: what the candidate route set holds for each peering session type, and that a null or loopback next hop is common before transmission. RFC8671-x-5 records that pre-policy Adj-RIB-Out is permitted and uncommon; that is read from this section's indicative prose, so it is listed as unsourced. |
| `6` | BMP Messages | 0 | walked | BMP Messages. Two sentences. The first says some messages carrying a per-peer header are not applicable to the Adj-RIB-In or Adj-RIB-Out distinction, and names Peer Up and Peer Down. The second, 'Unless otherwise defined, the O flag should be set to 0 in the per-peer header in BMP messages', uses a lowercase 'should', so under section 2 it carries no RFC 2119 level and the site scan does not see it. ze meets it anyway: peerHeaderFromEvent sets the O flag only for a sent-direction event, so a Peer Up, a Peer Down and a Loc-RIB message all carry O=0. |
| `6.1` | Route Monitoring and Route Mirroring | 1 | walked | Route Monitoring and Route Mirroring. One sentence, one capitalised MUST, mapped below. |
| `6.2` | Statistics Report | 1 | walked | Statistics Report. One capitalised MUST, excluded below, then the four new Stat Types 14 to 17 as value definitions. Its second sentence, 'The O flag SHOULD be ignored by the BMP receiver', is a SHOULD and never gates; ze's receiver meets it, because decodeStatisticsReport reads the per-peer header and processStatisticsReport stores the entries without consulting the O flag. |
| `6.3` | Peer Up and Down Notifications | 0 | walked | Peer Up and Down Notifications. No capitalised MUST-level keyword. Two indicative sentences and one SHOULD: the peering state a Peer Up or Peer Down conveys 'is independent of whether or not route monitoring or route mirroring messages will be sent for Adj-RIB-In, Adj-RIB-Out, or both', and a receiver SHOULD ignore the O flag on these two messages. RFC8671-x-3, which records that an Adj-RIB-Out Peer Up carries the same sent and received OPEN messages as an Adj-RIB-In one, is read from that independence sentence, so it is listed as unsourced. RFC8671-x-4 is NOT listed: it records that one BGP session MAY produce two Peer Up messages, and no sentence of RFC 8671 says that. It is a MAY, so it gates nothing and the reverse arithmetic asks no home for it. |
| `6.3.1` | Peer Up Information | 1 | walked | Peer Up Information. Defines Peer Up Information TLV type 4, Admin Label: a free-form UTF-8 string, administratively assigned, with no terminator required. One capitalised MUST on the order of multiple labels, mapped below. The type number is a value definition and the last sentence, 'The Admin Label is optional', removes any obligation to send one. |
| `7` | Other Considerations | 0 | walked | Other Considerations. A heading with no body text. |
| `7.1` | Peer and Update Groups | 0 | walked | Peer and Update Groups. Indicative prose on why a BMP sender cannot publish a simple peer group name, and why the Admin Label of section 6.3.1 answers the same need. Its last sentence puts label configuration and assignment outside the document. No directive. |
| `7.2` | Changes to Existing BMP Session | 1 | walked | Changes to Existing BMP Session. One sentence, one capitalised MUST, mapped below to RFC8671-7.2-1. |
| `8` | Security Considerations | 0 | walked | Security Considerations. Imports Section 11 of RFC 7854, states that an implementation SHOULD require sessions with authorized and trusted monitoring devices, and states that this document adds no further consideration. The SHOULD is advisory and never gates. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Names the registry the three subsections write to. |
| `9.1` | Addition to the BMP Peer Flags registry: flag 3, the O flag | 0 | skipped (iana) | Addition to the BMP Peer Flags registry: flag 3, the O flag. Binds IANA, not a speaker. |
| `9.2` | not stated | 0 | skipped (iana) | Additions to the BMP Statistics Types registry: types 14 to 17. Binds IANA, not a speaker. |
| `9.3` | not stated | 0 | skipped (iana) | Addition to the BMP Initiation Message TLVs registry: type 4, Admin Label. Binds IANA, not a speaker. |
| `10` | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 7854, RFC 8174. The section also absorbs the Acknowledgements, Contributors and Authors' Addresses blocks, none of which states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Definitions entry for post-policy Adj-RIB-Out closes with the same sentence section 5.1 states: 'This MUST convey to the BMP receiver what is actually transmitted to the peer.' Site 5.1:1 maps that obligation to RFC8671-5.1-1. One obligation, written twice. | This MUST convey to the BMP receiver what is actually transmitted to the peer. |
| `5.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The feature is OPTIONAL and ze decided not to offer it. RFC 7854 Section 5: 'A BMP speaker may send pre-policy routes, post-policy routes, or both.' ze offers the post-policy Adj-RIB-Out view only: peerHeaderFromEvent (internal/component/bgp/plugins/bmp/bmp_events.go) sets PeerFlagO and PeerFlagL in one statement for a sent-direction event, so O=1 with L=0 is unreachable, and the route-monitoring-policy leaf offers pre-policy (Adj-RIB-In), post-policy (Adj-RIB-Out) and all, with no pre-policy Adj-RIB-Out choice. This obligation is conditional on offering that feature: ze never reaches a pre-policy phase completion at which a mandatory attribute could be unknown, and holds no route for which it would have to zero the next hop. The permission it hangs on is RFC8671-x-5 [MAY], which ze does not exercise. This is a SCOPE DECISION and not outstanding work. The absent feature is recorded as an implementation gap in docs/features/rfc-status.md, which a later scope decision can revisit; it is not a conformance gap. | All mandatory attributes, such as next hop, MUST be either zero or have an empty length if they are unknown at the pre-policy phase completion. |
| `5.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The same optional feature as site 5.2:1. RFC 7854 Section 5: 'A BMP speaker may send pre-policy routes, post-policy routes, or both.' The pre-policy L flag rule binds a speaker that sends the pre-policy view, and ze does not send it: peerHeaderFromEvent (internal/component/bgp/plugins/bmp/bmp_events.go) sets PeerFlagO and PeerFlagL together for a sent-direction event and never sets O alone. The post-policy half of the same rule, which ze does exercise, is site 5.1:2 mapping RFC8671-x-2. This is a SCOPE DECISION and not outstanding work; the absent feature is recorded as an implementation gap in docs/features/rfc-status.md, not as a conformance gap. | The L flag MUST be set to 0 to indicate pre-policy. |

## Superseded

No document obsoletes RFC 8671, so its obligations are stated where they were written.
