# RFC 8654 - Extended Message Support for BGP

Supported. Every requirement this repository extracted from RFC 8654, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 8 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 2 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 25 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 16 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 25 |
| Tagged units | 25 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8654.md` |
| Requirement shard | `rfc/requirements/rfc8654.md` |
| RFC text | `rfc/full/rfc8654.txt` |

## Enrolment

Enrolled: Extended Message Support for BGP: twelve MUST-level requirements. Ten are met in internal/component/bgp: 3-1 (extended-message users get RFC 7606 revised error handling), 4-1 (a negotiated UPDATE or ROUTE-REFRESH may be up to 65535 octets), 4-2 (a built message never exceeds the negotiated maximum), 4-3 (OPEN and KEEPALIVE stay within 4096 even when extended), 6-1 (message length is validated against the per-type maximum), 5-1 and 5-2 (an over-4096 UPDATE is rejected when extended is not negotiated, with no liberal bypass), and 5-4 (a bad length yields a NOTIFICATION Message Header Error / Bad Message Length) carry positive+negative tags; 3-2 (the Extended Message capability is code 6) and 3-3 (its value is zero-length) are {single-polarity: positive}. Two are {not-applicable}: 5-3 (ze never emits an over-4096 NOTIFICATION -- its data is truncated to 128 octets) and 5-5 (a process obligation on the authors of specifications that define new BGP message types).

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Extended message capability and 65535-byte message limit when negotiated.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC8654-3-1`](#rfc8654-3-1), [`RFC8654-4-1`](#rfc8654-4-1), [`RFC8654-4-2`](#rfc8654-4-2), [`RFC8654-4-3`](#rfc8654-4-3), [`RFC8654-6-1`](#rfc8654-6-1), [`RFC8654-5-1`](#rfc8654-5-1), [`RFC8654-5-2`](#rfc8654-5-2), [`RFC8654-5-4`](#rfc8654-5-4)

**Annotated instead of tested (4):** [`RFC8654-3-2`](#rfc8654-3-2), [`RFC8654-3-3`](#rfc8654-3-3), [`RFC8654-5-3`](#rfc8654-5-3), [`RFC8654-5-5`](#rfc8654-5-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8654-3-1` | Peers using BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per RFC 7606 (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L124). **negative:** `unit/verify` [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L238) |
| `RFC8654-3-2` | Capability Code MUST be 6 (§3, Wire Format) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29). **negative:** no negative test. **{single-polarity}:** the capability-code assignment is a single fixed value, so the only falsifiable check is that CodeExtendedMessage encodes as 6; there is no distinct rejection behavior for a negative case to exercise |
| `RFC8654-3-3` | Capability Length MUST be 0 (§3, Wire Format) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L467). **negative:** no negative test. **{single-polarity}:** the capability's value is fixed at zero length, so the enforceable behavior is that WriteTo emits Cap Len 0 and Parse round-trips it; there is no separate malformed form of this fixed-zero-length capability for a negative to drive |
| `RFC8654-4-1` | An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256) |
| `RFC8654-4-2` | Applications generating information encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1613). **positive:** `unit/verify` [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366). **positive:** `unit/verify` [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394). **positive:** `unit/verify` [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187). **positive:** `unit/verify` [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224). **positive:** `unit/verify` [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_test.go#L1416). **negative:** `unit/verify` [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1585). **negative:** `unit/verify` [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330). **negative:** `unit/verify` [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) |
| `RFC8654-4-3` | OPEN and KEEPALIVE messages MUST NOT exceed 4,096 octets regardless of capability (§4, §6) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) |
| `RFC8654-6-1` | The value of the Length field MUST always be at least 19 and no greater than 65,535 for UPDATE/NOTIFICATION/ROUTE-REFRESH when extended, or 4,096 otherwise (§6) | MUST | 6 - Changes to RFC 4271 | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) |
| `RFC8654-5-1` | A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the capability MUST NOT accept a BGP Extended Message (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259) |
| `RFC8654-5-2` | A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) |
| `RFC8654-5-3` | If a NOTIFICATION is to be sent to a peer that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets (§5) | MUST NOT | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never generates a NOTIFICATION near 4096 octets -- notification.go:191 sizes it as 19 + 2 + len(Data) and the Administrative Shutdown Communication is truncated to 128 octets (internal/component/bgp/message/notification.go:311-313), so there is no over-4096 NOTIFICATION code path to cap |
| `RFC8654-5-4` | Any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of RFC 4271 (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290) |
| `RFC8654-5-5` | A protocol specification that defines new BGP message types MUST describe how to handle peers that can only accommodate 4,096 octet messages (§5) | MUST | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds the author of a specification that defines a new BGP message type to state its extended-message eligibility; it is not a runtime behavior ze implements |
| `RFC8654-4-4` | A BGP speaker capable of receiving BGP Extended Messages SHOULD advertise the BGP Extended Message Capability to its peers (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-5` | When propagating an UPDATE to a neighbor that has not advertised the BGP Extended Message Capability, the speaker SHOULD try to reduce the outgoing message size by removing attributes eligible under the "attribute discard" approach of RFC 7606 (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-5-6` | BGP protocol developers and implementers are conservative in their application and use of BGP Extended Messages (§5) | RECOMMENDED | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-6` | A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP Extended Message Capability was received from that peer (§4) | MAY | 4 - Operation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8654-5-3`](#rfc8654-5-3) If a NOTIFICATION is to be sent to a peer that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze never generates a NOTIFICATION near 4096 octets -- notification.go:191 sizes it as 19 + 2 + len(Data) and the Administrative Shutdown Communication is truncated to 128 octets (internal/component/bgp/message/notification.go:311-313), so there is no over-4096 NOTIFICATION code path to cap |
| [`RFC8654-5-5`](#rfc8654-5-5) A protocol specification that defines new BGP message types MUST describe how to handle peers that can only accommodate 4,096 octet messages (§5) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds the author of a specification that defines a new BGP message type to state its extended-message eligibility; it is not a runtime behavior ze implements |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8654-3-1`](#rfc8654-3-1)

Peers using BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per RFC 7606 (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L238) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L124) | unit/verify | unproven |

### [`RFC8654-3-2`](#rfc8654-3-2)

Capability Code MUST be 6 (§3, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29) | unit/verify | unproven |

### [`RFC8654-3-3`](#rfc8654-3-3)

Capability Length MUST be 0 (§3, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L467) | unit/verify | unproven |

### [`RFC8654-4-1`](#rfc8654-4-1)

An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265) | unit/verify | unproven |

### [`RFC8654-4-2`](#rfc8654-4-2)

Applications generating information encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1585) | unit/verify | unproven |
| negative | [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330) | unit/verify | unproven |
| negative | [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) | unit/verify | unproven |
| positive | [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1613) | unit/verify | unproven |
| positive | [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_split_test.go#L1416) | unit/verify | unproven |
| positive | [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366) | unit/verify | unproven |
| positive | [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394) | unit/verify | unproven |
| positive | [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187) | unit/verify | unproven |
| positive | [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224) | unit/verify | unproven |

### [`RFC8654-4-3`](#rfc8654-4-3)

OPEN and KEEPALIVE messages MUST NOT exceed 4,096 octets regardless of capability (§4, §6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) | unit/verify | unproven |
| positive | [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321) | unit/verify | unproven |

### [`RFC8654-6-1`](#rfc8654-6-1)

The value of the Length field MUST always be at least 19 and no greater than 65,535 for UPDATE/NOTIFICATION/ROUTE-REFRESH when extended, or 4,096 otherwise (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253) | unit/verify | unproven |

### [`RFC8654-5-1`](#rfc8654-5-1)

A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the capability MUST NOT accept a BGP Extended Message (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267) | unit/verify | unproven |

### [`RFC8654-5-2`](#rfc8654-5-2)

A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269) | unit/verify | unproven |

### [`RFC8654-5-3`](#rfc8654-5-3)

If a NOTIFICATION is to be sent to a peer that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8654-5-3, so no unit is bound to it.

### [`RFC8654-5-4`](#rfc8654-5-4)

Any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of RFC 4271 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290) | unit/verify | unproven |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298) | unit/verify | unproven |

### [`RFC8654-5-5`](#rfc8654-5-5)

A protocol specification that defines new BGP message types MUST describe how to handle peers that can only accommodate 4,096 octet messages (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8654-5-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc8654 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc8654.txt |
| Source fingerprint | b14e6e2fbf6eafd8 |
| Record | rfc/extraction/rfc8654.json |
| Mapped sentences | 9 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates section 1. Its one site is the IETF Trust Legal Provisions sentence, excluded below. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: RFC 4271 mandates 4,096 octets, new AFIs, SAFIs and capabilities need more, and this document raises the limit to 65,535 octets for every message except OPEN and KEEPALIVE. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, which states that the key words bind when, and only when, they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. It is also what makes the lowercase modals in sections 4 and 8 non-normative in this document. |
| `2` | BGP Extended Message | 0 | walked | BGP Extended Message. Definitions only: a message over 4,096 octets is a BGP Extended Message, extended messages have a maximum size of 65,535 octets, and the smallest message is a 19-octet KEEPALIVE. Stated indicatively, so under section 1.1 it carries no RFC 2119 level. The 65,535 and 19 octet bounds are carried normatively by site 6:1, which maps RFC8654-6-1, and by the Wire Formats and Constants tables of rfc/short/rfc8654.md. |
| `3` | BGP Extended Message Capability | 1 | walked | BGP Extended Message Capability. Its one capitalised site is the RFC 7606 error-handling prerequisite, mapped below to RFC8654-3-1. The rest is value assignment and description: Capability Code 6, Capability Length 0, how a speaker advertises it with RFC 5492, and what advertising conveys. Those are indicative, so the two wire-value obligations the summary declares from them are the unsourced ids below. |
| `4` | Operation | 5 | walked | Operation. Two capitalised MUST-level sites, mapped below to RFC8654-4-1 and RFC8654-4-2. Its three remaining sites carry lowercase modals that section 1.1 gives no normative level, and each is excluded below. The section's other directives are the SHOULD to advertise the capability, the SHOULD to try attribute discard when propagating to a peer that did not advertise it, the MAY that permits sending an extended message only to a peer the capability was received from, and the scoping sentence that the capability applies to all messages except OPEN and KEEPALIVE; the summary declares those as the unsourced ids below. The sentence stating that a listener which has not advertised the capability will generate a NOTIFICATION with Bad Message Length is written in the indicative future and cites RFC 4271 Section 6.1 for the subcode, so the site scan cannot see it; rfc/short/rfc8654.md carries it in its Errors list and in the Error Handling table. |
| `5` | Error Handling | 5 | walked | Error Handling. The document's densest normative section: five capitalised MUST-level sites, all mapped below to RFC8654-5-1 through RFC8654-5-5. Its one advisory, the RECOMMENDED that developers and implementers are conservative in their use of extended messages, is the unsourced id below. The sentence stating that UPDATE error handling per RFC 4271 Section 6.3 is unchanged is indicative and adds no obligation. |
| `6` | Changes to RFC 4271 | 1 | walked | Changes to RFC 4271. Its one site quotes the RFC 4271 Length-field MUST and states the change to 65,535, mapped below to RFC8654-6-1. Its second paragraph restates the same change against RFC 4271 Section 6.1 in the indicative and adds no separate obligation. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the allocation of value 6, "BGP Extended Message", in the "Capability Codes" registry. Binds IANA, not a speaker; the wire value itself is RFC8654-3-2. |
| `8` | Security Considerations | 1 | walked | Security Considerations. States that the extension does not change BGP's underlying security issues, that buffering 65,535-octet messages increases exposure to resource exhaustion, and lists three consequences of reducing an outgoing message for a peer that does not support extended messages. Its one site is the RFC 7606 eligibility criterion for attribute discard, excluded below. No countermeasure is directed at a speaker. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 5492, RFC 7606, RFC 8174. |
| `9.2` | Informative References: RFC 4272, RFC 7752, RFC 8205 | 0 | skipped (references) | Informative References: RFC 4272, RFC 7752, RFC 8205. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. It states a licensing condition on code extracted from the document text and directs no protocol behavior at a BGP speaker. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `4:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4271 Section 9.2, which the sentence cites as its authority. RFC 8654 raises the Length ceiling only for a neighbor that advertised the capability, so for a neighbor that did not, RFC 4271's own 4,096-octet limit is what forbids the send. The modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | If the message is still too big, then it must not be sent to the neighbor ([RFC4271], Section 9.2). |
| `4:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4271 Section 9.1.3, which the sentence cites: withdrawing a route that can no longer be advertised is the Update-Send Process's own rule, not a new one this document states. The modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | Additionally, if the NLRI was previously advertised to that peer, it must be withdrawn from service ([RFC4271], Section 9.1.3). |
| `4:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AS operator, not a BGP speaker. The paragraph's own next sentences name that role: a consistent view is guaranteed only if all the iBGP speakers advertise the capability, and if that is not the case "the operator should consider whether or not" to advertise it to external peers. It is a deployment judgement about which speakers are configured to advertise, and no code path in a speaker can carry it. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | If an Autonomous System (AS) has multiple internal BGP speakers and also has multiple external BGP neighbors, care must be taken to ensure a consistent view within the AS in order to present a consistent external view. |
| `8:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Eligibility for attribute discard is defined by RFC 7606, which the sentence cites. This sentence restates that criterion inside a security note about the consequence of discarding, and its modal is lowercase, which section 1.1 gives no RFC 2119 level in this document. | The attributes eligible under the "attribute discard" approach must have no effect on route selection or installation [RFC7606]. |

## Superseded

No document obsoletes RFC 8654, so its obligations are stated where they were written.
