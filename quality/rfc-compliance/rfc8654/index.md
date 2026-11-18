# RFC 8654 - Extended Message Support for BGP

BGP Extended Message. Every requirement listed for RFC 8654, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 8 | of 12 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 12 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 12 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Tests shown to detect a fault | 42 | 42 of 62 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial test coverage | 1 | of 12 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 1 | of 12 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 8 | of 12 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 12 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 1 | of 12 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 12 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 12 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 12 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 12 | of 16 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 12 mandatory requirements: they add up to 12. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | bad | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | BGP Extended Message |
| Automatic checks | Included |
| Requirements | 16 |
| Mandatory requirements (MUST level) | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 62 |
| Tests linked to requirements | 62 |
| Recorded audit verdicts | 8 |
| Fault test records | 42 |
| Summary | `rfc/short/rfc8654.md` |
| Requirement table | `rfc/requirements/rfc8654.md` |
| RFC text | `rfc/full/rfc8654.txt` |

## Automatic checks

Included in automatic checks: Extended Message capability and framing are implemented in internal/component/bgp. Receive permission follows local advertisement; send permission follows peer advertisement. Tests exercise accumulation, RFC7606 discard and route removal, fatal-error cleanup and replacement-best delivery. These proofs do not establish every incorporated RFC obligation or the exact final route-owner release/admission ordering.

## Published support status

**Area:** BGP Extended Message

**Test verification:** 8 of 12 MUST proven

**Evidence:** interop 2, unit 6

**Unmet:** RFC8654-5-3, RFC8654-5-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 1 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **12** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (8):** [`RFC8654-3-1`](#rfc8654-3-1), [`RFC8654-3-3`](#rfc8654-3-3), [`RFC8654-4-1`](#rfc8654-4-1), [`RFC8654-4-2`](#rfc8654-4-2), [`RFC8654-4-3`](#rfc8654-4-3), [`RFC8654-6-1`](#rfc8654-6-1), [`RFC8654-5-1`](#rfc8654-5-1), [`RFC8654-5-2`](#rfc8654-5-2)

**Annotated (including scoped evidence) (4):** [`RFC8654-3-2`](#rfc8654-3-2), [`RFC8654-5-3`](#rfc8654-5-3), [`RFC8654-5-4`](#rfc8654-5-4), [`RFC8654-5-5`](#rfc8654-5-5)

**Partial test coverage (subset of annotated; zero whole-requirement credit) (1):** [`RFC8654-5-4`](#rfc8654-5-4)

## Figures

- **MUST bar**: Ze verified 8, Tests awaiting verification 1, Not built 2
- **Overview bar**: Ze MUST verified 8, SHOULD verified 1, Not owed 1, Tests awaiting verification 1, Not assessed 3, Not built 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 12 |
| MUST rows kept (the MUST bar) | 11 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 2 |
| MUST rows not owed | 1 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 1 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8654.json](../../../data/rfc/rfc8654.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8654-3-1` | Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]. (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L336). **positive:** `unit/verify` [`TestRFC8654ExtendedDuplicateAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L411). **positive:** `unit/verify` [`TestRFC8654ExtendedDuplicateMPResets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L479). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L252). **positive:** `unit/verify` [`TestRFC8654ExtendedTwoFamilyWithdrawalRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L498). **positive:** `unit/verify` [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L35). **negative:** `unit/verify` [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L337). **negative:** `unit/verify` [`TestRFC8654ExtendedDuplicateAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L412). **negative:** `unit/verify` [`TestRFC8654ExtendedDuplicateMPResets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L480). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L255). **negative:** `unit/verify` [`TestRFC8654ExtendedTwoFamilyWithdrawalRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L499). **negative:** `unit/verify` [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L36) |
| `RFC8654-3-2` | The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L26). **negative:** no negative test. **{single-polarity}:** the capability-code assignment is a single fixed value, so the only falsifiable check is that CodeExtendedMessage encodes as 6; there is no distinct rejection behavior for a negative case to exercise |
| `RFC8654-3-3` | The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 and Capability Length 0. (§3) | MUST | 3 - BGP Extended Message Capability | **positive:** `unit/verify` [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L523). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L27). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageNonZeroLengthRefused`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L52) |
| `RFC8654-4-1` | An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L28). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256). **positive:** `interop/nightly` [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L13) |
| `RFC8654-4-2` | Applications generating information that might be encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1632). **positive:** `unit/verify` [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366). **positive:** `unit/verify` [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394). **positive:** `unit/verify` [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187). **positive:** `unit/verify` [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224). **positive:** `unit/verify` [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_update_split_test.go#L1417). **negative:** `unit/verify` [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1604). **negative:** `unit/verify` [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330). **negative:** `unit/verify` [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) |
| `RFC8654-4-3` | The BGP Extended Message Capability applies to all messages except for OPEN and KEEPALIVE messages. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321). **negative:** `unit/verify` [`TestRFC8654AdvertisementDoesNotExtendControlBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L259). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) |
| `RFC8654-6-1` | [RFC4271] states "The value of the Length field MUST always be at least 19 and no greater than 4096." This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages. Section 6.1 of [RFC4271] specifies raising an error if the length of a message is over 4,096 octets. For all messages except for OPEN and KEEPALIVE messages, if the receiver has advertised the BGP Extended Message Capability, this document raises that limit to 65,535. (§6) | MUST | 6 - Changes to RFC 4271 | **positive:** `unit/verify` [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L108). **positive:** `unit/verify` [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L69). **positive:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L35). **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L26). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253). **negative:** `unit/verify` [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L109). **negative:** `unit/verify` [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L70). **negative:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L38). **negative:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L27). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) |
| `RFC8654-5-1` | A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message. (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L29). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267). **negative:** `unit/verify` [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L30). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259). **negative:** `interop/nightly` [`checkExtendedNeitherRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L76). **negative:** `interop/nightly` [`checkExtendedRemoteOnlyRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L58) |
| `RFC8654-5-2` | A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5) | MUST NOT | 5 - Error Handling | **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) |
| `RFC8654-5-3` | However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5) | MUST NOT | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/component/bgp/message/notification.go::Len meets it, since a NOTIFICATION is 19 + 2 + len(Data) octets, its Data copies at most a field of a message the peer sent within its own 4096-octet limit, and BuildCeaseData cuts the Administrative Shutdown Communication to 128 octets; no test asserts it |
| `RFC8654-5-4` | Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]. (§5) | MUST | 5 - Error Handling | **positive:** `unit/verify` [`TestRFC8654FatalLengthReelectsAlternateBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L175). **positive:** `unit/verify` [`TestRFC8654FatalLengthReleasesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L53). **positive:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298). **negative:** `unit/verify` [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L41). **negative:** `unit/verify` [`TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_bad_length_session_test.go#L27). **negative:** `unit/verify` [`TestRFC8654ValidLengthRetainsInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L130). **negative:** `unit/verify` [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290). **{partial}:** tested "treats an improper BGP Extended Message as a fatal error"; gap "follow the error-handling procedures of [RFC4271]"; live TCP tests prove exact fatal-length notification, completed session/resource/RIB cleanup and downstream withdrawal or alternate replacement, with valid-length retention controls. They do not observe the last invalid-route or advertisement owner's lifetime through outbound admission; internal/component/bgp/reactor/relay_recovery.go::admitRecovery is the inspected admission boundary. Header helpers assert returned Notification values or no error, not socket cleanup or owner lifetime. The whole row remains unproven. **Partial test coverage:** Tests and fault test records cover only the part marked Tested; they do not verify the whole requirement. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Tested "treats an improper BGP Extended Message as a fatal error"; Gap "follow the error-handling procedures of [RFC4271]"; Producer internal/component/bgp/reactor/relay_recovery.go::admitRecovery. Independent rejudgment from complete RFC 8654 and RFC 4271 Sections 6 and 6.1, every tagged unit, and the current reader/publication and recovery boundaries. RFC 8654 Section 5 says, "Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]." Extracting readFrame preserves exact Bad Message Length notification, Event21, transport closure and the pre-body limit check. Header helpers assert returned notification values or no error; they do not observe socket or route cleanup. Actual TCP tests first establish real received routes, Loc-RIB state and downstream advertisements, then assert exact 1/2 Data 1001, EOF, Idle, timer/context/session cleanup, source storage removal and both downstream withdrawals. The alternate-best carrier checks both replacement attributes together on recipient TCP through a causal post-recovery marker, a single Loc-RIB replacement and withdrawal without a survivor. The adjacent 4096-octet control retains the original session/resources/routes and forwards its changed MED without notification or withdrawal. These assertions remain useful after the shared-reader extraction. RFC 4271 Section 6 also requires withdrawal or replacement before final invalid-route deletion. None of these end-state observations measures the last invalid-route or advertisement owner's lifetime through outbound admission; the tests explicitly disclaim that proof. admitRecovery stages owned references and dispatches completion-bound work, but reading that implementation cannot substitute for a lifecycle observation or a discriminating fault at its final-owner boundary. The existing partial annotation remains accurate and earns zero whole-requirement credit. No new execution or imported ordering conformance is claimed. Further independent rejudgment after receivedFrame value transfer: reread the full RFC, exact partial annotation, every tagged unit and assertion-producing helper, prior judgment and complete changed reader/candidate paths. The frame wrapper now transfers by value with unchanged error, notification, closure and buffer-handle ownership. Main renewed this row's existing misclassified-fatal-length break under native receipt 70610: the exact tagged unit passes clean and fails broken when expected notification code 1 becomes 6. Tagged bodies/claims and the Tested/Gap/Producer scope are unchanged. Verdict remains partial, with the same unproven final-owner lifetime ordering and every existing producer key retained. Final closure rejudgment: independently reread the full RFC 8654 and RFC 4271 Sections 6 and 6.1, the complete partial annotation, all seven tagged entries and their assertion-producing helpers, and the changed receive/adoption boundaries. Both rails retain pre-body Bad Message Length rejection and exact fatal teardown; the coalesced rail now admits UPDATE semantics only in Established and retains the shared buffer-ownership finalizer. The actual TCP cleanup, valid 4096-octet retention and alternate-best replacement assertions remain discriminating for their declared Tested scope. Retaining and replaying decoded OPEN observations on selection leaves original wire observation and fatal-header handling separate. The unchanged Gap is still the final invalid-route or advertisement owner lifetime through outbound admission at relay_recovery.go::admitRecovery: none of the tagged tests observes that lifetime or the final-owner boundary under a targeted break. Reading correct staging code cannot replace that proof. Verdict remains partial, with zero whole-requirement credit; every existing producer key and the full Tested/Gap/Producer explanation are retained. Companion-fixture re-read also covers the final extendedRecoveryPeers repair: independent terminal host routes and actual next-hop resolution restore the alternate-best scenario's prerequisite, while all fatal-length, exact replacement, causal fence and no-transient-withdrawal assertions remain. This does not supply the missing final-owner lifetime observation, so the whole row remains partial. |
| `RFC8654-5-5` | Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5) | MUST | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages."; binds future protocol specifications; ze writes no specification: it is a BGP speaker that implements the Extended Message capability as RFC 8654 defines it |
| `RFC8654-4-4` | A BGP speaker that is capable of receiving BGP Extended Messages SHOULD advertise the BGP Extended Message Capability to its peers using BGP Capabilities Advertisement [RFC5492]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-5` | When propagating that UPDATE onward to a neighbor that has not advertised the BGP Extended Message Capability, the speaker SHOULD try to reduce the outgoing message size by removing attributes eligible under the "attribute discard" approach of [RFC7606]. (§4) | SHOULD | 4 - Operation | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-5-6` | BGP protocol developers and implementers are conservative in their application and use of BGP Extended Messages (§5) | RECOMMENDED | 5 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC8654-4-6` | A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP Extended Message Capability was received from that peer (§4) | MAY | 4 - Operation | **positive:** `unit/verify` [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L196). **negative:** `unit/verify` [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L197). **positive:** `interop/nightly` [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L17). **negative:** `interop/nightly` [`checkExtendedSendDeniedFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L40) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8654-5-3`](#rfc8654-5-3) However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5) | {gap}, no test | gap in proof: internal/component/bgp/message/notification.go::Len meets it, since a NOTIFICATION is 19 + 2 + len(Data) octets, its Data copies at most a field of a message the peer sent within its own 4096-octet limit, and BuildCeaseData cuts the Administrative Shutdown Communication to 128 octets; no test asserts it |
| [`RFC8654-5-4`](#rfc8654-5-4) Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]. (§5) | {partial}, Partial test coverage | tested "treats an improper BGP Extended Message as a fatal error"; gap "follow the error-handling procedures of [RFC4271]"; live TCP tests prove exact fatal-length notification, completed session/resource/RIB cleanup and downstream withdrawal or alternate replacement, with valid-length retention controls. They do not observe the last invalid-route or advertisement owner's lifetime through outbound admission; internal/component/bgp/reactor/relay_recovery.go::admitRecovery is the inspected admission boundary. Header helpers assert returned Notification values or no error, not socket cleanup or owner lifetime. The whole row remains unproven |
| [`RFC8654-5-5`](#rfc8654-5-5) Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: "Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages."; binds future protocol specifications; ze writes no specification: it is a BGP speaker that implements the Extended Message capability as RFC 8654 defines it |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8654-3-1`](#rfc8654-3-1)

Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-24 (independent auditor, authored none of 9c3697d0d1 or 439a413ffd). RFC 8654 Section 3: "Peers that wish to use the BGP Extended Message Capability MUST support error handling for BGP UPDATE messages per [RFC7606]." 439a413ffd added a well-formed subtest to TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling and moved the positive tag onto it: on a session negotiating Extended Message both ways, a >4096-octet UPDATE (1100 /24s, valid ORIGIN, AS_PATH, NEXT_HOP) leaves ReadAndProcess error-free, the session Established, exactly one dispatch whose withdrawn section is empty, attribute section byte-equal and NLRI byte-equal to what was sent, and no NOTIFICATION. That is a genuinely conforming input, which closes the previous finding (positive tag on an error input). The negative now names both error subtests: malformed ORIGIN (length 2) dispatches every route as withdrawn with no attributes, session up, no NOTIFICATION; attribute section overrunning the message gives an error, Idle, nothing dispatched and NOTIFICATION 3/1 with empty data. Positive and negative are distinct subtests with distinct inputs. Other carriers unchanged. Discrimination records observed red: positive under session_read.go extendedMessage forced false, negative under the treat-as-withdraw branch forced to None. Units ran green at HEAD. Independent companion-fixture rejudgment against RFC 8654 Section 3 and its referenced RFC 7606 handling, all twelve tagged entries and their assertion-producing helpers: the shared extendedRecoveryPeers repair installs separately owned terminal interior host routes for the next hops and checks actual Loc-RIB resolution with cleanup. It does not seed subject BGP routes; both affected RIB-recovery carriers still install their exact subject routes through live BGP first. The ORIGIN-only invalid input still requires actual Adj-RIB-In and Loc-RIB removal plus downstream withdrawals without resetting, and post-error controls require useful restoration on the same session. The two-family carrier still preserves its unrelated survivor, reinstalls the previously withdrawn family before the malformed input and observes both withdrawals and recovery. The supplementary session matrix still distinguishes conforming extended UPDATEs, exact treat-as-withdraw, attribute discard, keep-first duplicates and duplicate-MP/session-reset outcomes through its stated readers with unchanged exact payload and notification assertions. Existing twelve-entry scope and enforced verdict remain; this source rejudgment invents no new runtime receipt.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8654ExtendedTwoFamilyWithdrawalRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L499) | unit/verify | revert, verified |
| negative | [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L36) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L337) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedDuplicateAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L412) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedDuplicateMPResets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L480) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L255) | unit/verify | mutant, verified |
| positive | [`TestRFC8654ExtendedTwoFamilyWithdrawalRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L498) | unit/verify | revert, verified |
| positive | [`TestRFC8654TreatAsWithdrawRemovesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedAttributeDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L336) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedDuplicateAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L411) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedDuplicateMPResets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L479) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_receive_session_dispatch_test.go#L252) | unit/verify | mutant, verified |

### [`RFC8654-3-2`](#rfc8654-3-2)

The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Code 6 is the whole obligation; row single-polarity positive (a fixed code has no refusal form). TestRFC8654ExtendedMessageWireIsCode6Length0 reads the octet WriteTo puts at a non-zero offset (06), Code()==6, and Parse(06 00) yields *ExtendedMessage: a code drift in WriteTo or in Parse's dispatch goes red. The old TestCapabilityCodeConstants tag (constant only) is supplementary. Judge 2026-09-30.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L29) | unit/verify | not verified |
| positive | [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L26) | unit/verify | revert, verified |

### [`RFC8654-3-3`](#rfc8654-3-3)

The BGP Extended Message Capability is a new BGP capability [RFC5492] defined with Capability Code 6 and Capability Length 0. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: WriteTo writes exactly 06 00 with Len 2 and nothing beyond, Parse(06 00) accepted. Negative: TestRFC8654ExtendedMessageNonZeroLengthRefused sends code 6 with length 1 and 4 and requires ErrInvalidLength and no capability. Judge break: parseZeroLengthCapability accepting a non-zero length (len>8 guard) turned the negative red. Single-polarity marker removal is correct: a refusal path exists.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8654ExtendedMessageNonZeroLengthRefused`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestCapabilityRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L523) | unit/verify | not verified |
| positive | [`TestRFC8654ExtendedMessageWireIsCode6Length0`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L27) | unit/verify | revert, verified |

### [`RFC8654-4-1`](#rfc8654-4-1)

An implementation that advertises the BGP Extended Message Capability MUST be capable of receiving a message with a length up to and including 65,535 octets (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L256) | unit/verify | not verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L265) | unit/verify | not verified |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L28) | unit/verify | revert, verified |
| positive | [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L13) | interop/nightly | revert, verified |

### [`RFC8654-4-2`](#rfc8654-4-2)

Applications generating information that might be encapsulated within BGP messages MUST limit the size of their payload to take the maximum message size into account. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant: an UPDATE on the wire larger than the negotiated maximum. TestSendUpdateWithSplitBoundsAnOversizePayload drives Peer.sendUpdateWithSplit with an over-4096 payload to a non-extended peer and asserts every wire frame length <= 4096 with the NLRI preserved; TestSendUpdateWithSplitTracksTheNegotiatedMaximum asserts the bound is 65535 when negotiated; BuildUnicastWithMaxSize refuses an oversize build (ErrUpdateTooLarge). Both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildUnicast_MaxSize_TooLarge`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1604) | unit/verify | not verified |
| negative | [`TestSendPluginRoutesBoundsAnOversizeGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L330) | unit/verify | not verified |
| negative | [`TestSendUpdateWithSplitBoundsAnOversizePayload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L143) | unit/verify | not verified |
| positive | [`TestSplitUpdate_VPNChunksFitMaxMessageSize`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_update_split_test.go#L1417) | unit/verify | revert, verified |
| positive | [`TestBuildUnicast_MaxSize_Fits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L1632) | unit/verify | not verified |
| positive | [`TestSendPluginRoutesLeavesAFittingGroupWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L366) | unit/verify | not verified |
| positive | [`TestSendPluginRoutesTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L394) | unit/verify | not verified |
| positive | [`TestSendUpdateWithSplitLeavesAFittingPayloadWhole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L187) | unit/verify | not verified |
| positive | [`TestSendUpdateWithSplitTracksTheNegotiatedMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_max_message_size_test.go#L224) | unit/verify | not verified |

### [`RFC8654-4-3`](#rfc8654-4-3)

The BGP Extended Message Capability applies to all messages except for OPEN and KEEPALIVE messages. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Prior independent judgment retained after scope comparison, not gratuitously re-audited. The complete Section 4 exception sentence and tagged units are unchanged: TestMaxMessageLength, TestValidateLengthWithMax, and TestRFC8654AdvertisementDoesNotExtendControlBounds. The directional test file gained an import and sibling accumulation unit; its control-bound unit is unchanged in the scoped diff. OPEN 4097 and KEEPALIVE 20 still require exact NOTIFICATION 1/2 in both readers, while valid controls and all supported type maxima are pinned. No new foreign execution or canonical fingerprint computation is claimed. The parent stamping tool must compute current unit/file hashes.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L241) | unit/verify | not verified |
| negative | [`TestRFC8654AdvertisementDoesNotExtendControlBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestMaxMessageLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L321) | unit/verify | not verified |

### [`RFC8654-6-1`](#rfc8654-6-1)

[RFC4271] states "The value of the Length field MUST always be at least 19 and no greater than 4096." This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages. Section 6.1 of [RFC4271] specifies raising an error if the length of a message is over 4,096 octets. For all messages except for OPEN and KEEPALIVE messages, if the receiver has advertised the BGP Extended Message Capability, this document raises that limit to 65,535. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read both RFC 8654 Section 6 paragraphs with Sections 3-5: the common minimum remains, the 65535 maximum applies only after receiver advertisement, and OPEN/KEEPALIVE are excluded. All five tagged functions were re-read. TestValidateLengthWithMax and TestRFC8654LengthBoundsPerTypeAndCapability accept per-type minima and 4096 boundaries, accept 65535 for extended UPDATE/NOTIFICATION/ROUTE-REFRESH, reject 0/18 and standard 4097, reject OPEN 4097/65535 even with extension and KEEPALIVE 20, and identify Bad Message Length, including erroneous Length in the boundary helper. TestRFC8654ExtendedMessageDirections tests all four advertisement combinations and independently pins receive/send permission in negotiated and encoding views. TestRFC8654ReceiveLimitFollowsLocalOPEN drives actual OPENs and both session readers at 4096/4097/65535 with complete-payload equality or exact 1/2 plus offending Length and no dispatch. Its rejected sender receive only joins completion; the wire and dispatch predicates remain the proof. TestRFC8654AnnouncementAccumulation uses individually standard-sized announcements to isolate batching rather than header admission, requiring exact consumer batches through 65535, overflow splitting, KEEPALIVE flush, empty-flush nonduplication and continued Established state. The producers are directional capability.Negotiate, Session.negotiateWith, Header.ValidateLengthWithMax and the single/coalesced readers. The comment-only receive change loses no proof or boundary. No native tests were executed in this audit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L248) | unit/verify | not verified |
| negative | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L109) | unit/verify | revert, verified |
| negative | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L253) | unit/verify | not verified |
| positive | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC8654AnnouncementAccumulation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC8654ExtendedMessageDirections`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8654_extended_message_test.go#L69) | unit/verify | revert, verified |

### [`RFC8654-5-1`](#rfc8654-5-1)

A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message. (§5)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/session_negotiate.go::publishNegotiation moved. Independent rejudgment against the complete RFC 8654 Section 5: "A BGP speaker that has the ability to use BGP Extended Messages but has not advertised the BGP Extended Message Capability, presumably due to configuration, MUST NOT accept a BGP Extended Message." The immutable prepared OPEN pair now feeds prepareNegotiation and publishNegotiation; the receive permission still comes only from capability.Negotiate's local advertisement, while writer resizing follows the independent peer advertisement. readFrame and the coalesced reader both call ValidateLengthWithMax before reading the declared body. The tagged live OPEN-exchange matrix covers all four local/peer advertisement combinations through both readers at 4096, 4097 and 65535 octets. Local absence above 4096 requires an error, zero dispatch and exact NOTIFICATION 1/2 with the offending Length; local presence requires one intact dispatch, Established and no answer bytes. The header table independently distinguishes accepted and rejected limits. Read both tagged FRR checker bodies and their shared receive-rejection predicate: they inspect original Ze OPEN advertisement separately from the relay-delivered copy, require FRR's actual large UPDATE, exact rejection and teardown of the original session. No new test or FRR execution is claimed by this source audit. Further independent rejudgment after receivedFrame value transfer: reread the full RFC, this row, all tagged units and their assertion-producing helpers, both prior judgments, and the complete changed readFrame/dispatchFrame/admitFrame paths. Only the frame wrapper and error returns changed representation; advertised receive permission, per-type length validation before body read, exact notification and borrowed buffer ownership are preserved. Tagged bodies and claims are unchanged. Main renewed the existing readFrame permission break under native receipt 52883: clean PASS for all three RFC7606 handling variants and own named broken failures when extended receive permission is forced false. That receipt belongs to RFC8654-3-1 and supports reader preservation; it does not replace this row's own unchanged tagged evidence. Verdict remains enforced and all existing producer keys are retained. Final closure rejudgment: independently reread RFC 8654, the precise checklist clause, all six tagged entries and assertion-producing helpers, both receive rails, immutable negotiation and selected incoming adoption. The changed coalesced admission now rejects an UPDATE outside Established before semantic delivery. It still validates the same advertised receive length before the declared body and observes each complete original frame before admission; this does not weaken the established-session directional limit matrix. The new private decoded OPEN retention changes only learned-event publication, not the actual OPEN advertisement, physical wire observer, local-advertisement receive permission or per-type length gate. Existing claims, six entries and five producer keys remain intact. No new interop execution is claimed by this source judgment.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L259) | unit/verify | not verified |
| negative | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L30) | unit/verify | revert, verified |
| negative | [`checkExtendedNeitherRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L76) | interop/nightly | revert, verified |
| negative | [`checkExtendedRemoteOnlyRejectFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L58) | interop/nightly | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L267) | unit/verify | not verified |
| positive | [`TestRFC8654ReceiveLimitFollowsLocalOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L29) | unit/verify | revert, verified |

### [`RFC8654-5-2`](#rfc8654-5-2)

A speaker MUST NOT implement a more liberal policy accepting BGP Extended Messages (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L261) | unit/verify | not verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L269) | unit/verify | not verified |

### [`RFC8654-5-3`](#rfc8654-5-3)

However, if a NOTIFICATION is to be sent to a BGP speaker that has not advertised the BGP Extended Message Capability, the size of the message MUST NOT exceed 4,096 octets. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8654-5-3, so there is no linked test.

### [`RFC8654-5-4`](#rfc8654-5-4)

Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]. (§5)

Records cover only part of this requirement: tested "treats an improper BGP Extended Message as a fatal error"; gap "follow the error-handling procedures of [RFC4271]"; live TCP tests prove exact fatal-length notification, completed session/resource/RIB cleanup and downstream withdrawal or alternate replacement, with valid-length retention controls. They do not observe the last invalid-route or advertisement owner's lifetime through outbound admission; internal/component/bgp/reactor/relay_recovery.go::admitRecovery is the inspected admission boundary. Header helpers assert returned Notification values or no error, not socket cleanup or owner lifetime. The whole row remains unproven; they do not verify the whole requirement.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Tested "treats an improper BGP Extended Message as a fatal error"; Gap "follow the error-handling procedures of [RFC4271]"; Producer internal/component/bgp/reactor/relay_recovery.go::admitRecovery. Independent rejudgment from complete RFC 8654 and RFC 4271 Sections 6 and 6.1, every tagged unit, and the current reader/publication and recovery boundaries. RFC 8654 Section 5 says, "Similarly, any speaker that treats an improper BGP Extended Message as a fatal error MUST follow the error-handling procedures of [RFC4271]." Extracting readFrame preserves exact Bad Message Length notification, Event21, transport closure and the pre-body limit check. Header helpers assert returned notification values or no error; they do not observe socket or route cleanup. Actual TCP tests first establish real received routes, Loc-RIB state and downstream advertisements, then assert exact 1/2 Data 1001, EOF, Idle, timer/context/session cleanup, source storage removal and both downstream withdrawals. The alternate-best carrier checks both replacement attributes together on recipient TCP through a causal post-recovery marker, a single Loc-RIB replacement and withdrawal without a survivor. The adjacent 4096-octet control retains the original session/resources/routes and forwards its changed MED without notification or withdrawal. These assertions remain useful after the shared-reader extraction. RFC 4271 Section 6 also requires withdrawal or replacement before final invalid-route deletion. None of these end-state observations measures the last invalid-route or advertisement owner's lifetime through outbound admission; the tests explicitly disclaim that proof. admitRecovery stages owned references and dispatches completion-bound work, but reading that implementation cannot substitute for a lifecycle observation or a discriminating fault at its final-owner boundary. The existing partial annotation remains accurate and earns zero whole-requirement credit. No new execution or imported ordering conformance is claimed. Further independent rejudgment after receivedFrame value transfer: reread the full RFC, exact partial annotation, every tagged unit and assertion-producing helper, prior judgment and complete changed reader/candidate paths. The frame wrapper now transfers by value with unchanged error, notification, closure and buffer-handle ownership. Main renewed this row's existing misclassified-fatal-length break under native receipt 70610: the exact tagged unit passes clean and fails broken when expected notification code 1 becomes 6. Tagged bodies/claims and the Tested/Gap/Producer scope are unchanged. Verdict remains partial, with the same unproven final-owner lifetime ordering and every existing producer key retained. Final closure rejudgment: independently reread the full RFC 8654 and RFC 4271 Sections 6 and 6.1, the complete partial annotation, all seven tagged entries and their assertion-producing helpers, and the changed receive/adoption boundaries. Both rails retain pre-body Bad Message Length rejection and exact fatal teardown; the coalesced rail now admits UPDATE semantics only in Established and retains the shared buffer-ownership finalizer. The actual TCP cleanup, valid 4096-octet retention and alternate-best replacement assertions remain discriminating for their declared Tested scope. Retaining and replaying decoded OPEN observations on selection leaves original wire observation and fatal-header handling separate. The unchanged Gap is still the final invalid-route or advertisement owner lifetime through outbound admission at relay_recovery.go::admitRecovery: none of the tagged tests observes that lifetime or the final-owner boundary under a targeted break. Reading correct staging code cannot replace that proof. Verdict remains partial, with zero whole-requirement credit; every existing producer key and the full Tested/Gap/Producer explanation are retained. Companion-fixture re-read also covers the final extendedRecoveryPeers repair: independent terminal host routes and actual next-hop resolution restore the alternate-best scenario's prerequisite, while all fatal-length, exact replacement, causal fence and no-transient-withdrawal assertions remain. This does not supply the missing final-owner lifetime observation, so the whole row remains partial.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L290) | unit/verify | revert, verified |
| negative | [`TestRFC8654LengthBoundsPerTypeAndCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8654_length_bounds_test.go#L41) | unit/verify | revert, verified |
| negative | [`TestRFC8654OverLengthUpdateFollowsRFC4271ErrorHandling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_bad_length_session_test.go#L27) | unit/verify | mutant, verified |
| negative | [`TestRFC8654ValidLengthRetainsInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestValidateLengthWithMax`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/header_test.go#L298) | unit/verify | revert, verified |
| positive | [`TestRFC8654FatalLengthReleasesInstalledRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_fatal_length_cleanup_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC8654FatalLengthReelectsAlternateBest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_rib_recovery_test.go#L175) | unit/verify | revert, verified |

### [`RFC8654-5-5`](#rfc8654-5-5)

Future protocol specifications MUST describe how to handle peers that can only accommodate 4,096 octet messages. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8654-5-5, so there is no linked test.

### [`RFC8654-4-6`](#rfc8654-4-6)

A BGP speaker MAY send BGP Extended Messages to a peer only if the BGP Extended Message Capability was received from that peer (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L197) | unit/verify | revert, verified |
| negative | [`checkExtendedSendDeniedFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L40) | interop/nightly | revert, verified |
| positive | [`TestRFC8654SendLimitFollowsPeerOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8654_directional_session_test.go#L196) | unit/verify | revert, verified |
| positive | [`checkExtendedAsymmetricFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_extended_message_scenarios.go#L17) | interop/nightly | revert, verified |

## Requirement list review

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
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: RFC 4271 mandates 4,096 octets, new AFIs, SAFIs and capabilities need more, and this document raises the limit to 65,535 octets for every message except OPEN and KEEPALIVE. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | reviewed | Requirements Language. The BCP 14 key-words paragraph, which states that the key words bind when, and only when, they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. It is also what makes the lowercase modals in sections 4 and 8 non-normative in this document. |
| `2` | BGP Extended Message | 0 | reviewed | BGP Extended Message. Definitions only: a message over 4,096 octets is a BGP Extended Message, extended messages have a maximum size of 65,535 octets, and the smallest message is a 19-octet KEEPALIVE. Stated indicatively, so under section 1.1 it carries no RFC 2119 level. The 65,535 and 19 octet bounds are carried normatively by site 6:1, which maps RFC8654-6-1, and by the Wire Formats and Constants tables of rfc/short/rfc8654.md. |
| `3` | BGP Extended Message Capability | 1 | reviewed | BGP Extended Message Capability. Its one capitalised site is the RFC 7606 error-handling prerequisite, mapped below to RFC8654-3-1. The rest is value assignment and description: Capability Code 6, Capability Length 0, how a speaker advertises it with RFC 5492, and what advertising conveys. Those are indicative, so the two wire-value obligations the summary declares from them are the unsourced ids below. |
| `4` | Operation | 5 | reviewed | Operation. Two capitalised MUST-level sites, mapped below to RFC8654-4-1 and RFC8654-4-2. Its three remaining sites carry lowercase modals that section 1.1 gives no normative level, and each is excluded below. The section's other directives are the SHOULD to advertise the capability, the SHOULD to try attribute discard when propagating to a peer that did not advertise it, the MAY that permits sending an extended message only to a peer the capability was received from, and the scoping sentence that the capability applies to all messages except OPEN and KEEPALIVE; the summary declares those as the unsourced ids below. The sentence stating that a listener which has not advertised the capability will generate a NOTIFICATION with Bad Message Length is written in the indicative future and cites RFC 4271 Section 6.1 for the subcode, so the site scan cannot see it; rfc/short/rfc8654.md carries it in its Errors list and in the Error Handling table. |
| `5` | Error Handling | 5 | reviewed | Error Handling. The document's densest normative section: five capitalised MUST-level sites, all mapped below to RFC8654-5-1 through RFC8654-5-5. Its one advisory, the RECOMMENDED that developers and implementers are conservative in their use of extended messages, is the unsourced id below. The sentence stating that UPDATE error handling per RFC 4271 Section 6.3 is unchanged is indicative and adds no obligation. |
| `6` | Changes to RFC 4271 | 1 | reviewed | Changes to RFC 4271. Its one site quotes the RFC 4271 Length-field MUST and states the change to 65,535, mapped below to RFC8654-6-1. Its second paragraph restates the same change against RFC 4271 Section 6.1 in the indicative and adds no separate obligation. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the allocation of value 6, "BGP Extended Message", in the "Capability Codes" registry. Binds IANA, not a speaker; the wire value itself is RFC8654-3-2. |
| `8` | Security Considerations | 1 | reviewed | Security Considerations. States that the extension does not change BGP's underlying security issues, that buffering 65,535-octet messages increases exposure to resource exhaustion, and lists three consequences of reducing an outgoing message for a peer that does not support extended messages. Its one site is the RFC 7606 eligibility criterion for attribute discard, excluded below. No countermeasure is directed at a speaker. |
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
