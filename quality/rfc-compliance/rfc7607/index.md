# RFC 7607 - Codification of AS 0 Processing

Supported. Every requirement this repository extracted from RFC 7607, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 5 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 29 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 29 |
| Tagged units | 29 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7607.md` |
| Requirement shard | `rfc/requirements/rfc7607.md` |
| RFC text | `rfc/full/rfc7607.txt` |

## Enrolment

Enrolled: Codification of AS 0 Processing: five MUST-level requirements, all in Section 2, all met and all proven in both polarities. RFC7607-2-2 (AS 0 in AS_PATH or AGGREGATOR is malformed, handled per RFC 7606) is enforced by validateASPath and validateAggregatorAttr (internal/component/bgp/message/rfc7606.go), which land on treat-as-withdraw and attribute discard as RFC 7606 sections 7.2 and 7.7 each require. RFC7607-2-3 (AS 0 in AS4_PATH or AS4_AGGREGATOR, handled per RFC 6793) is enforced by validateAS4PathAttr and validateAS4AggregatorAttr (internal/component/bgp/message/rfc7607.go), which register the first validators codes 17 and 18 ever had and land on the attribute discard RFC 6793 section 6 chooses. RFC7607-2-1 (never originate or propagate AS 0) is proven over the real receive rail in internal/component/bgp/reactor/rfc7607_update_test.go, driving Session.enforceRFC7606 rather than a leaf validator: an AS 0 route is stopped before any plugin sees it, so nothing can relay one, and the paired negatives accept the same UPDATE with one AS number changed. RFC7607-2-4 (a peer AS of zero aborts the connection with NOTIFICATION 2/2 Bad Peer AS) is enforced by Session.validateOpenPeerAS (internal/component/bgp/reactor/session_open_as.go) on BOTH OPEN rails, handleOpen and the collision-winner processOpen, and reads the Four-octet AS capability as well as the two-octet My AS because a four-octet speaker carries its real AS there. RFC7607-2-5 (never initiate a connection claiming to be AS 0) is enforced by Session.sendOpen, paired with the zt:asn YANG range "1..4294967295" that refuses AS 0 at config load. Ze uses AS 0 internally as a not-known-yet sentinel for a dynamic peer, for observation grouping and for BGP-LS descriptors; every check above reads the WIRE field instead, and rfc/short/rfc7607.md tabulates the five sentinel sites against the check that leaves each one alone. Enrolled 2026-08-30.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

[`internal/component/bgp/message/rfc7607.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607.go) (AS4_PATH and AS4_AGGREGATOR validators), [`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go) (`validateASPath`, `validateAggregatorAttr`), [`internal/component/bgp/reactor/session_open_as.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as.go) (`validateOpenPeerAS`, both OPEN rails), [`internal/component/bgp/reactor/session_negotiate.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate.go) (`sendOpen`)

**What the ledger says remains**

All 5 MUST-level requirements implemented and extracted, each with a positive and a negative test tag. AS 0 in AS_PATH is treat-as-withdrawn and in AGGREGATOR, AS4_PATH and AS4_AGGREGATOR it is discarded, which is what RFC 7606 sections 7.2 and 7.7 and RFC 6793 section 6 each prescribe. An OPEN whose peer AS is zero, in either the two-octet My AS field or the Four-octet AS capability, draws NOTIFICATION 2/2 Bad Peer AS on both OPEN rails. This row read `Unsupported` from 2026-08-30 until the implementation landed the same day; before that it claimed `Supported` on a description belonging to RFC 4486.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC7607-2-1`](#rfc7607-2-1), [`RFC7607-2-2`](#rfc7607-2-2), [`RFC7607-2-3`](#rfc7607-2-3), [`RFC7607-2-4`](#rfc7607-2-4), [`RFC7607-2-5`](#rfc7607-2-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7607-2-1` | A BGP speaker MUST NOT originate or propagate a route with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or AS4_AGGREGATOR attributes (§2) | MUST NOT | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L88). **positive:** `unit/verify` [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L48). **negative:** `unit/verify` [`TestRFC7607AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L104). **negative:** `unit/verify` [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L68) |
| `RFC7607-2-2` | An UPDATE message that contains the AS number of zero in the AS_PATH or AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC7606] (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607ASPathZeroTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L29). **positive:** `unit/verify` [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L90). **positive:** `unit/verify` [`TestRFC7607AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L63). **positive:** `unit/verify` [`TestRFC7607CompleteUpdateASPathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L179). **positive:** `unit/verify` [`TestRFC7607ReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L244). **positive:** `unit/verify` [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L50). **negative:** `unit/verify` [`TestRFC7607ASPathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L49). **negative:** `unit/verify` [`TestRFC7607AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L90). **negative:** `unit/verify` [`TestRFC7607CompleteUpdateAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L192). **negative:** `unit/verify` [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L70) |
| `RFC7607-2-3` | An UPDATE message that contains the AS number of zero in the AS4_PATH or AS4_AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC6793] (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestRFC7607AS4AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L124). **positive:** `unit/verify` [`TestRFC7607AS4AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L138). **positive:** `unit/verify` [`TestRFC7607AS4PathReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L266). **positive:** `unit/verify` [`TestRFC7607AS4PathZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L111). **positive:** `unit/verify` [`TestRFC7607CompleteUpdateAS4PathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L208). **negative:** `unit/verify` [`TestRFC7607AS4AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L154). **negative:** `unit/verify` [`TestRFC7607AS4AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L138). **negative:** `unit/verify` [`TestRFC7607AS4PathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L128). **negative:** `unit/verify` [`TestRFC7607CompleteUpdateAS4PathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L224) |
| `RFC7607-2-4` | If a BGP speaker receives zero as the peer AS in an OPEN message, it MUST abort the connection and send a NOTIFICATION with Error Code "OPEN Message Error" and subcode "Bad Peer AS" (§2) | MUST | 2 - Behavior | **positive:** `unit/verify` [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L48). **positive:** `unit/verify` [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L101). **negative:** `unit/verify` [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L51). **negative:** `unit/verify` [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L103) |
| `RFC7607-2-5` | A router MUST NOT initiate a connection claiming to be AS 0 (§2) | MUST NOT | 2 - Behavior | **positive:** `unit/verify` [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L199). **negative:** `unit/verify` [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L202) |

## Gaps and untested MUSTs

RFC 7607 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7607-2-1`](#rfc7607-2-1)

A BGP speaker MUST NOT originate or propagate a route with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or AS4_AGGREGATOR attributes (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L104) | unit/verify | unproven |
| negative | [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L68) | unit/verify | unproven |
| positive | [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L48) | unit/verify | unproven |

### [`RFC7607-2-2`](#rfc7607-2-2)

An UPDATE message that contains the AS number of zero in the AS_PATH or AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC7606] (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607ASPathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L49) | unit/verify | unproven |
| negative | [`TestRFC7607AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L90) | unit/verify | unproven |
| negative | [`TestRFC7607CompleteUpdateAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L192) | unit/verify | unproven |
| negative | [`TestRFC7607UpdateWithRealASIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC7607ASPathZeroTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L29) | unit/verify | unproven |
| positive | [`TestRFC7607AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC7607CompleteUpdateASPathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L179) | unit/verify | unproven |
| positive | [`TestRFC7607ReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L244) | unit/verify | unproven |
| positive | [`TestRFC7607AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC7607UpdateWithASZeroIsNotAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L50) | unit/verify | unproven |

### [`RFC7607-2-3`](#rfc7607-2-3)

An UPDATE message that contains the AS number of zero in the AS4_PATH or AS4_AGGREGATOR attribute MUST be considered as malformed and be handled by the procedures specified in [RFC6793] (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7607AS4AggregatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L154) | unit/verify | unproven |
| negative | [`TestRFC7607AS4PathNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L128) | unit/verify | unproven |
| negative | [`TestRFC7607CompleteUpdateAS4PathAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L224) | unit/verify | unproven |
| negative | [`TestRFC7607AS4AggregatorRealASIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC7607AS4AggregatorZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC7607AS4PathReachesTheAttributeWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L266) | unit/verify | unproven |
| positive | [`TestRFC7607AS4PathZeroDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L111) | unit/verify | unproven |
| positive | [`TestRFC7607CompleteUpdateAS4PathZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7607_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC7607AS4AggregatorASZeroIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7607_update_test.go#L124) | unit/verify | unproven |

### [`RFC7607-2-4`](#rfc7607-2-4)

If a BGP speaker receives zero as the peer AS in an OPEN message, it MUST abort the connection and send a NOTIFICATION with Error Code "OPEN Message Error" and subcode "Bad Peer AS" (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L51) | unit/verify | unproven |
| negative | [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L103) | unit/verify | unproven |
| positive | [`TestHandleOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L48) | unit/verify | unproven |
| positive | [`TestProcessOpenRejectsPeerASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L101) | unit/verify | unproven |

### [`RFC7607-2-5`](#rfc7607-2-5)

A router MUST NOT initiate a connection claiming to be AS 0 (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L202) | unit/verify | unproven |
| positive | [`TestSendOpenRefusesLocalASZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as_test.go#L199) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-6-supported-extraction-signoff |
| Signed off | 2026-08-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc7607.txt |
| Source fingerprint | 1d694dc81b42f157 |
| Record | rfc/extraction/rfc7607.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, copyright notice, Abstract and Table of Contents. The Abstract states the document's scope and binds nobody. |
| `1` | Introduction | 0 | walked | Introduction. Explains why AS 0 is proscribed, citing the RFC 6491 ROA signal, and states that this document updates the error handling of RFC 4271 sections 6.2 and 6.3. Its one obligation-shaped sentence, 'requires that BGP implementations not accept or propagate routes containing AS 0', is a rationale for section 2 and carries no RFC 2119 keyword; section 2 states it normatively as 2:1. |
| `1.1` | Requirements Notation | 0 | walked | Requirements Notation. The RFC 2119 key-words paragraph. It tells a reader how to read section 2 and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Behavior | 5 | walked | Behavior. The whole normative content of the document: five sentences, all five mapped, none excluded. Its closing paragraph, encouraging authors of future protocol extensions to keep AS 0 in mind, binds a document author rather than an implementation and carries no keyword, so the derivation raises no site for it. |
| `3` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA changed the AS 0 entry in the 16-bit Autonomous System Numbers registry to read 'Reserved'. A completed registry action, not an obligation on a speaker. |
| `4` | Security Considerations | 0 | walked | Security Considerations. Explains what the section 2 rules buy: a resource holder can declare an address resource unused, and a malicious party cannot hijack it by announcing AS 0. Rationale for the obligations already captured, with no keyword and no obligation of its own. |
| `5` | References | 0 | skipped (references) | References. The section heading only; its entries are in 5.1 and 5.2. |
| `5.1` | Normative References | 0 | skipped (references) | Normative References. Bibliography entries for IANA.AS_Numbers, RFC 2119, RFC 4271, RFC 6793 and RFC 7606. |
| `5.2` | Informative References | 0 | skipped (references) | Informative References. The bibliography entry for RFC 6491, plus the Acknowledgements and Authors' Addresses that follow it. |

### Excluded sentences

The walk over RFC 7607 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7607, so its obligations are stated where they were written.
