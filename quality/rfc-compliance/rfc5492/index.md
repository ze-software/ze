# RFC 5492 - Capabilities Advertisement with BGP-4

Supported. Every requirement this repository extracted from RFC 5492, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 44.4% | 4 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 44.4% | 4 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 54.5% | 12 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.1% | 1 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 8 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc5492.md` |
| Requirement shard | `rfc/requirements/rfc5492.md` |
| RFC text | `rfc/full/rfc5492.txt` |

## Enrolment

Enrolled: BGP Capabilities Advertisement: nine MUST-level requirements. 3-1 (Unsupported-Capability NOTIFICATION carries the offending capabilities), 5-1 (each capability encoded code+length+value as in OPEN), 4-2 (every Type-2 Optional Parameter processed), and 3-2 (unknown capability preserved without error) carry positive+negative tags in internal/core/bgp/capability and internal/component/bgp/reactor. The MUST NOTs 3-3/3-4/5-2 have justified single-polarity positive coverage in TestOpenIgnoresUnknownCapability: valid unknown capabilities provide no conforming rejection case under these prohibitions. The required-capability rejection test retains separate policy and payload assertions without claiming negative coverage of those prohibitions. 4-1 (accept multiple identical capability instances) is {single-polarity: positive} bound to a parser test. 4-3 is {not-applicable}: it obliges the author of a capability-defining specification to describe its error handling, not a runtime behavior ze implements.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Capability TLV parser, encoder, unknown-capability ignore behavior, negotiated session view.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5492-3-1`](#rfc5492-3-1), [`RFC5492-5-1`](#rfc5492-5-1), [`RFC5492-4-2`](#rfc5492-4-2), [`RFC5492-3-2`](#rfc5492-3-2)

**Annotated (including scoped evidence) (5):** [`RFC5492-4-1`](#rfc5492-4-1), [`RFC5492-4-3`](#rfc5492-4-3), [`RFC5492-3-3`](#rfc5492-3-3), [`RFC5492-3-4`](#rfc5492-3-4), [`RFC5492-5-2`](#rfc5492-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5492-3-1` | The message MUST contain the capability or capabilities that cause the speaker to send the message. (§3) | MUST | 3 - Overview of Operations | **positive:** `unit/verify` [`TestBuildUnsupportedCapabilityData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L359). **positive:** `unit/verify` [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L61). **negative:** `unit/verify` [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L62). **negative:** `unit/verify` [`TestSessionAcceptsRequiredCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1326) |
| `RFC5492-5-1` | The Data field in the NOTIFICATION message MUST list the set of capabilities that causes the speaker to send the message. (§5) | MUST | 5 - Extensions to Error Handling | **positive:** `unit/verify` [`TestBuildUnsupportedCapabilityDataCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1174). **positive:** `unit/verify` [`TestBuildUnsupportedCapabilityDataCodes_MultipleCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L389). **positive:** `unit/verify` [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L63). **negative:** `unit/verify` [`TestBuildUnsupportedCapabilityDataCodes_Empty`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L411). **negative:** `unit/verify` [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L64) |
| `RFC5492-4-1` | Note, however, that processing of multiple instances of such capability does not require special handling, as additional instances do not change the meaning of the announced capability; thus, a BGP speaker MUST be prepared to accept such multiple instances. (§4) | MUST | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** `unit/verify` [`TestParseAcceptsMultipleIdenticalCapabilityInstances`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L310). **positive:** `unit/verify` [`TestRFC5492RepeatedCapabilityInstancesAcceptedAtTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L143). **negative:** no negative test. **{single-polarity}:** ze's capability parser appends every capability TLV without dedup or reject (internal/core/bgp/capability/capability.go:177), so multiple identical instances are all accepted; there is no reject path, so no negative case exists |
| `RFC5492-4-2` | However, for backward compatibility, a BGP speaker MUST be prepared to receive an OPEN message that contains multiple Capabilities Optional Parameters, each of which contains one or more capabilities TLVs. (§4) | MUST | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** `unit/verify` [`TestParseFromOptionalParamsMultipleCapabilitiesParameters`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L341). **positive:** `unit/verify` [`TestRFC5492CapabilitiesSpreadOverSeveralParameters`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L173). **negative:** `unit/verify` [`TestOptionalParamRejectsTruncatedCapabilityTLV`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L267). **negative:** `unit/verify` [`TestRFC5492EveryCapabilitiesParameterReadBeforeRefusing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L201) |
| `RFC5492-3-2` | If a BGP speaker receives from its peer a capability that it does not itself support or recognize, it MUST ignore that capability. (§3) | MUST | 3 - Overview of Operations | **positive:** `unit/verify` [`TestParseUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L184). **positive:** `unit/verify` [`TestRFC5492UnrecognizedCapabilitiesIgnoredAtTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L90). **negative:** `unit/verify` [`TestParseRejectsMalformedKnownCapabilityLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L233). **negative:** `unit/verify` [`TestRFC5492UnrecognizedCapabilityNeverReadAsKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L117) |
| `RFC5492-4-3` | Processing of these capability instances is specific to the Capability Code and MUST be described in the document introducing the new capability. (§4) | MUST | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds the author of a specification that introduces a new capability to describe its multiple-instance error handling; it is not a runtime behavior ze implements |
| `RFC5492-3-3` | the BGP session MUST NOT be terminated in response to reception of a capability that is not supported by the local speaker. (§3) | MUST NOT | 3 - Overview of Operations | **positive:** `unit/verify` [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L109). **negative:** no negative test. **{single-polarity}:** a valid unknown capability has no conforming termination case under this prohibition; TestOpenIgnoresUnknownCapability exercises handleOpen and completes establishment, while rejection for a missing required capability is separate local policy |
| `RFC5492-3-4` | In particular, the Unsupported Capability NOTIFICATION message MUST NOT be generated and the BGP session MUST NOT be terminated in response to reception of a capability that is not supported by the local speaker. (§3) | MUST NOT | 3 - Overview of Operations | **positive:** `unit/verify` [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L112). **negative:** no negative test. **{single-polarity}:** this prohibition defines no input for which an unknown capability warrants Unsupported Capability; TestOpenIgnoresUnknownCapability observes the outgoing KEEPALIVE and absence of NOTIFICATION, while a missing required capability invokes a separate permitted response |
| `RFC5492-5-2` | It MUST NOT be used when a BGP speaker receives a capability that it does not understand; such capabilities MUST be ignored. (§5) | MUST NOT | 5 - Extensions to Error Handling | **positive:** `unit/verify` [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L114). **negative:** no negative test. **{single-polarity}:** valid unknown capabilities must all be ignored, so this rule has no conforming reject case; TestOpenIgnoresUnknownCapability checks known-capability negotiation and the session response, while malformed encodings and absent required capabilities exercise different rules |
| `RFC5492-3-5` | A BGP speaker determines that its peer doesn't support capabilities advertisement if, in response to an OPEN message that carries the Capabilities Optional Parameter, the speaker receives a NOTIFICATION message with the Error Subcode set to Unsupported Optional Parameter. (This is a consequence of the base BGP-4 specification [RFC4271] and not a new requirement.) In this case, the speaker SHOULD attempt to re-establish a BGP connection with the peer without sending to the peer the Capabilities Optional Parameter. (§3) | SHOULD | 3 - Overview of Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-4-4` | BGP speakers SHOULD NOT include more than one instance of a capability with the same Capability Code, Capability Length, and Capability Value. (§4) | SHOULD NOT | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-4-5` | The Capabilities Optional Parameter (OPEN Optional Parameter Type 2) SHOULD only be included in the OPEN message once. (§4) | SHOULD | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-4-6` | If the BGP speaker wishes to include multiple capabilities in the OPEN message, it SHOULD do so as discussed above -- by listing all those capabilities as TLVs within a single Capabilities Optional Parameter. (§4) | SHOULD | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-3-6` | The decision to send the message and terminate the peering is local to the speaker. If terminated, such peering SHOULD NOT be re-established automatically. (§3) | SHOULD NOT | 3 - Overview of Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-3-7` | If a BGP speaker that supports a certain capability determines that its peer doesn't support this capability, the speaker MAY send a NOTIFICATION message to the peer and terminate peering (§3) | MAY | 3 - Overview of Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC5492-4-7` | BGP speakers MAY include more than one instance of a capability (as identified by the Capability Code) with non-zero Capability Length field, but with different Capability Value and either the same or different Capability Length. (§4) | MAY | 4 - Capabilities Optional Parameter (Parameter Type 2) | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5492-4-3`](#rfc5492-4-3) Processing of these capability instances is specific to the Capability Code and MUST be described in the document introducing the new capability. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds the author of a specification that introduces a new capability to describe its multiple-instance error handling; it is not a runtime behavior ze implements |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5492-3-1`](#rfc5492-3-1)

The message MUST contain the capability or capabilities that cause the speaker to send the message. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Unsupported Capability NOTIFICATION that omits a causing capability or lists one that did not cause it. TestRFC5492UnsupportedCapabilityDataListsTheCausingSet drives handleOpen on a session requiring Route Refresh and Extended Message and reads the wire: none sent -> one 2/7 NOTIFICATION with Data exactly 02 00 06 00; Route Refresh plus unknown 254 sent -> Data exactly 06 00 (sent and unknown capabilities excluded); both sent -> no NOTIFICATION, OpenConfirm.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestSessionAcceptsRequiredCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1326) | unit/verify | unproven |
| positive | [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestBuildUnsupportedCapabilityData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L359) | unit/verify | unproven |

### [`RFC5492-5-1`](#rfc5492-5-1)

The Data field in the NOTIFICATION message MUST list the set of capabilities that causes the speaker to send the message. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: Data that does not list the set of capabilities that caused the NOTIFICATION. TestRFC5492UnsupportedCapabilityDataListsTheCausingSet asserts the sent NOTIFICATION's Data octets exactly: both missing -> 02 00 06 00 (the whole set), one missing -> 06 00 only (the causing set, not the required set). Builder-level units remain as supporting evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestBuildUnsupportedCapabilityDataCodes_Empty`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L411) | unit/verify | unproven |
| positive | [`TestRFC5492UnsupportedCapabilityDataListsTheCausingSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_reactor_b_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestBuildUnsupportedCapabilityDataCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1174) | unit/verify | unproven |
| positive | [`TestBuildUnsupportedCapabilityDataCodes_MultipleCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L389) | unit/verify | unproven |

### [`RFC5492-4-1`](#rfc5492-4-1)

Note, however, that processing of multiple instances of such capability does not require special handling, as additional instances do not change the meaning of the announced capability; thus, a BGP speaker MUST be prepared to accept such multiple instances. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row single-polarity positive. TestRFC5492RepeatedCapabilityInstancesAcceptedAtTheSession sends Route Refresh x3 and each MP TLV twice through handleOpen -> OpenConfirm, one KEEPALIVE, all negotiated; closes the session-level gap. Parser-level tag kept as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5492RepeatedCapabilityInstancesAcceptedAtTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestParseAcceptsMultipleIdenticalCapabilityInstances`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L310) | unit/verify | unproven |

### [`RFC5492-4-2`](#rfc5492-4-2)

However, for backward compatibility, a BGP speaker MUST be prepared to receive an OPEN message that contains multiple Capabilities Optional Parameters, each of which contains one or more capabilities TLVs. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 5492 §4, rfc/full/rfc5492.txt:201-211: 'However, for backward compatibility, a BGP speaker MUST be prepared to receive an OPEN message that contains multiple Capabilities Optional Parameters, each of which contains one or more capabilities TLVs.' The next sentence requires processing the capability set the same way whether combined or split. Read all four units. internal/core/bgp/capability/capability_test.go::TestParseFromOptionalParamsMultipleCapabilitiesParameters requires exactly two decoded capabilities, first Multiprotocol IPv4/unicast and second ASN4=65002. ::TestOptionalParamRejectsTruncatedCapabilityTLV requires ErrShortRead, but its outer parameter length is itself truncated: this supplementary unit does NOT independently isolate inner-TLV validation and its prose must not be credited as doing so. internal/component/bgp/reactor/rfc5492_open_session_test.go::TestRFC5492CapabilitiesSpreadOverSeveralParameters sends RR, Extended Message and unknown 254 in three parameters and requires OpenConfirm, exactly one KEEPALIVE, RR and both ExtendedMessageRecv/Send flags. ::TestRFC5492EveryCapabilitiesParameterReadBeforeRefusing varies which required capability is present across two parameters and requires Idle and exact NOTIFICATION 2/7 Data [6,0] or [2,0], naming only the capability absent from the entire parameter population.
Producer path read: capability.go::ParseFromOptionalParams bounds-checks and loops over every parameter, appending Parse results; session_open_as.go::validateOpenPeerAS returns the full list; session_handlers.go::handleOpen passes it to session_negotiate.go::negotiateWith, then validates required modes and advances. Unknown code 254 remains external protocol input, not an impossible closed-enum value. Q1 yes collectively; Q2 yes: returning after the first parameter changes negotiated flags and exact missing-capability Data; Q3 yes for the session pair, with the confounded supplemental malformed parser fixture disclosed; Q4 yes for split-parameter acceptance. The missing-capability negatives are policy-boundary controls, not a claim that multiple parameters themselves warrant rejection. No current semantic overlay was executed.
Pending note: retain enforced with the exact session assertions and parser-fixture limitation above; no claim of newly observed discrimination or live-peer acceptance.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5492EveryCapabilitiesParameterReadBeforeRefusing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L201) | unit/verify | revert, verified |
| negative | [`TestOptionalParamRejectsTruncatedCapabilityTLV`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC5492CapabilitiesSpreadOverSeveralParameters`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestParseFromOptionalParamsMultipleCapabilitiesParameters`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L341) | unit/verify | unproven |

### [`RFC5492-3-2`](#rfc5492-3-2)

If a BGP speaker receives from its peer a capability that it does not itself support or recognize, it MUST ignore that capability. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Session level via handleOpen. Positive: unrecognized codes 253/254 interleaved with MP IPv4 and required Route Refresh -> OpenConfirm, one KEEPALIVE, known capabilities negotiated. Negative (R1(b)): code 254 carrying the MP IPv6 value -> accepted but IPv6 not negotiated. Judge breaks: unrecognized 4-octet value parsed as Multiprotocol turned the negative red; unrecognized capability refused turned the positive red. HEAD negative TestParseRejectsMalformedKnownCapabilityLength concerns known capabilities and is supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5492UnrecognizedCapabilityNeverReadAsKnown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestParseRejectsMalformedKnownCapabilityLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L233) | unit/verify | unproven |
| positive | [`TestRFC5492UnrecognizedCapabilitiesIgnoredAtTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5492_open_session_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestParseUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L184) | unit/verify | unproven |

### [`RFC5492-4-3`](#rfc5492-4-3)

Processing of these capability instances is specific to the Capability Code and MUST be described in the document introducing the new capability. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5492-4-3, so no unit is bound to it.

### [`RFC5492-3-3`](#rfc5492-3-3)

the BGP session MUST NOT be terminated in response to reception of a capability that is not supported by the local speaker. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: terminating the session because a received capability is not supported. TestOpenIgnoresUnknownCapability sends an OPEN with code 254 and asserts handleOpen returns nil, state OpenConfirm then Established after KEEPALIVE, and s.Conn() is the same connection. {single-polarity: positive} is justified: a prohibition with no conforming termination input.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L109) | unit/verify | revert, verified |

### [`RFC5492-3-4`](#rfc5492-3-4)

In particular, the Unsupported Capability NOTIFICATION message MUST NOT be generated and the BGP session MUST NOT be terminated in response to reception of a capability that is not supported by the local speaker. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Quote now covers both halves: no Unsupported Capability NOTIFICATION and no termination for an unsupported capability. TestOpenIgnoresUnknownCapability asserts the bytes written after the OPEN are exactly one KEEPALIVE (Len HeaderLen, type KEEPALIVE) and unchanged after establishment (no NOTIFICATION), and that the connection survives to Established. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L112) | unit/verify | revert, verified |

### [`RFC5492-5-2`](#rfc5492-5-2)

It MUST NOT be used when a BGP speaker receives a capability that it does not understand; such capabilities MUST be ignored. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 (Unsupported Capability MUST NOT be used for a capability not understood): TestOpenIgnoresUnknownCapability asserts only a KEEPALIVE is written for an OPEN with code 254. Clause 2 (such capabilities MUST be ignored): the same unit asserts the following Route Refresh TLV is negotiated and the session reaches Established. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpenIgnoresUnknownCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_handlers_test.go#L114) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 3, rfc5492 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc5492.txt |
| Source fingerprint | d97da179f6d6ed92 |
| Record | rfc/extraction/rfc5492.json |
| Mapped sentences | 8 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice and Abstract. The Abstract states what the document defines and that it obsoletes RFC 3392. It directs no speaker. |
| `1` | Introduction | 2 | walked | Introduction. Two sentences, both indicative, and both excluded below. The first recounts what the base BGP-4 specification requires of a speaker that meets an unrecognized Optional Parameter. The second states what a pair of speakers supporting this document can do. Section 3 says of the same fact that it 'is a consequence of the base BGP-4 specification [RFC4271] and not a new requirement'. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The RFC 2119 key-words paragraph. It tells a reader how to read sections 3 to 5 and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Overview of Operations | 3 | walked | Overview of Operations. The document's main normative section: three sites, mapped below to RFC5492-3-1, RFC5492-3-2 and RFC5492-3-3. Site 3:3 states two MUST NOT obligations in one sentence, so RFC5492-3-4 is declared unsourced here rather than mapped. Its remaining directives carry no MUST-level keyword, so the site scan cannot see them: the SHOULD to re-establish without the Capabilities Optional Parameter after an Unsupported Optional Parameter NOTIFICATION (RFC5492-3-5), the SHOULD NOT to re-establish a peering terminated for a missing required capability (RFC5492-3-6), and the MAY to send a NOTIFICATION and terminate when the peer does not support a required capability (RFC5492-3-7). The section's opening MAY, that an OPEN message may include the Capabilities Optional Parameter, and its three indicative paragraphs on how a speaker learns a peer's capabilities, state no obligation. |
| `4` | Capabilities Optional Parameter (Parameter Type 2) | 3 | walked | Capabilities Optional Parameter (Parameter Type 2). Assigns parameter type 2 and defines the <Capability Code, Capability Length, Capability Value> triple, which are value definitions carried by the Wire Formats and Constants tables of rfc/short/rfc5492.md. Three sites, mapped below to RFC5492-4-1, RFC5492-4-3 and RFC5492-4-2. Its three advisory sentences carry no MUST-level keyword and are the unsourced ids below: the SHOULD NOT against duplicate identical instances (RFC5492-4-4), the SHOULD to include the Capabilities Optional Parameter once (RFC5492-4-5), the SHOULD to list every capability as a TLV inside that one parameter (RFC5492-4-6), and the MAY to include several instances of one code with different values (RFC5492-4-7). The closing sentence, that the set of capabilities should be processed the same way whether it arrives in one parameter or several, is the lowercase 'should' the document's own section 2 gives no normative level; it restates the MUST at site 4:3 and the summary captures it under RFC5492-4-2 in its Decoding Rules. |
| `5` | Extensions to Error Handling | 3 | walked | Extensions to Error Handling. Assigns Error Subcode 7, Unsupported Capability, as a value definition. Three sites: 5:1 and 5:3 are mapped to RFC5492-5-1 and RFC5492-5-2, and 5:2 is a recapitulation of section 3. The sentence 'Each such capability is encoded in the same way as it would be encoded in the OPEN message' carries no MUST-level keyword, so the scan cannot raise it; the summary folds it into RFC5492-5-1, whose row names it. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the Capability Code registry and its three assignment policies over codes 1-63, 64-127 and 128-255, the reserved code 0, and the 'BGP OPEN Optional Parameter Types' registry with parameter types 1 and 2. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | walked | Security Considerations. States that this extension does not change the security issues inherent in existing BGP. No countermeasure is directed at a speaker. |
| `8` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `9` | References | 0 | skipped (references) | References. The heading over sections 9.1 and 9.2. |
| `9.1` | Normative References: RFC 2119, RFC 4271 and RFC 5226 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271 and RFC 5226. |
| `9.2` | Informative References: RFC 4272 and RFC 4760 | 0 | skipped (references) | Informative References: RFC 4272 and RFC 4760. |
| `A` | Appendix A | 0 | skipped (appendix-non-normative) | Appendix A. Comparison between RFC 2842 and RFC 3392. Two lines of editorial history about a document this one obsoletes. No obligation. |
| `B` | Appendix B | 0 | skipped (appendix-non-normative) | Appendix B. Comparison between RFC 3392 and This Document. Editorial history, including that this document 'clarifies requirements by changing a number of SHOULDs to MUSTs'. It names no obligation of its own; the changed levels are the MUSTs of sections 3, 4 and 5. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 4271, which the sentence cites: a speaker that receives an OPEN with an unrecognized Optional Parameter terminates the peering. This document recounts it as the problem it solves, and section 3 says of the same fact that it 'is a consequence of the base BGP-4 specification [RFC4271] and not a new requirement'. The lowercase 'must' is what the prose-register scan raised. | The base BGP-4 specification [RFC4271] requires that when a BGP speaker receives an OPEN message with one or more unrecognized Optional Parameters, the speaker must terminate the BGP peering. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative: it states what a pair of speakers supporting this document can do, not what either of them owes. The word the scan raised is 'required' in 'all capabilities required to support the peering', which is descriptive English rather than the RFC 2119 keyword. Section 2 binds the key words only in upper case, and the prose-register scan is case-insensitive. The behavior the sentence describes is stated normatively at site 3:2 and is captured as RFC5492-3-2. | A pair of BGP speakers that supports this specification can establish the peering even when presented with unrecognized capabilities, so long as all capabilities required to support the peering are supported. |
| `5:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative: it recapitulates what section 3 already established, that the Unsupported Capability NOTIFICATION is how a speaker complains about a missing capability the peering requires. The sentence says so itself: 'As explained in the "Overview of Operations" section'. The word the scan raised is 'required' in 'a required capability', which is descriptive English rather than the RFC 2119 keyword; section 2 binds the key words only in upper case. The same sentence's lowercase 'cannot', in 'without which the peering cannot proceed', describes the peer's situation rather than stating an obligation. The obligation of this paragraph is the next sentence, site 5:3. | As explained in the "Overview of Operations" section, the Unsupported Capability NOTIFICATION is a way for a BGP speaker to complain that its peer does not support a required capability without which the peering cannot proceed. |

## Superseded

No document obsoletes RFC 5492, so its obligations are stated where they were written.
