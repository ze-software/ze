# RFC 4486 - Subcodes for BGP Cease NOTIFICATION Message

BGP Cease subcodes and prefix maximum. Every requirement listed for RFC 4486, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 1 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 1 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 1 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 1 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 3 | 3 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 1 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 1 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 1 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 1 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 1 | of 11 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 1 mandatory requirements: they add up to 1. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Cease subcodes and prefix maximum |
| Automatic checks | Included |
| Requirements | 11 |
| Mandatory requirements (MUST level) | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 0 |
| Fault test records | 3 |
| Summary | `rfc/short/rfc4486.md` |
| Requirement table | `rfc/requirements/rfc4486.md` |
| RFC text | `rfc/full/rfc4486.txt` |

## Automatic checks

Included in automatic checks: Subcodes for BGP Cease NOTIFICATION Message: one MUST-level requirement, RFC4486-4-1 -- terminating a peering because the prefix count exceeded a locally configured upper bound MUST send a NOTIFICATION with Error Code Cease and Error Subcode "Maximum Number of Prefixes Reached". Both polarities are proven over the reactor's prefix-limit path (internal/component/bgp/reactor/session_prefix.go:399 decides, :448 builds the NOTIFICATION with message.NotifyCease and message.NotifyCeaseMaxPrefixes): positive in TestPrefixExceedTeardown, and negative in TestPrefixWarningThreshold, where a peer at the warning threshold but below the maximum receives no NOTIFICATION at all -- so the Cease is bound to the termination and not merely to a counter moving. test/plugin/prefix-maximum-enforce.ci pins the same bytes on the wire (error code 06, subcode 01, plus the AFI/SAFI/count Data field of RFC4486-4-10). The other ten rows in section 4 are SHOULD, RECOMMENDED or MAY and are not gated.

## Published support status

**Area:** BGP Cease subcodes and prefix maximum

**Test verification:** 1 of 1 MUST proven

**Evidence:** functional 1

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **1** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC4486-4-1`](#rfc4486-4-1)

## Figures

- **MUST bar**: Ze verified 1
- **Overview bar**: Ze MUST verified 1, SHOULD verified 1, Not assessed 9

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze verified | 1 |
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
| SHOULD / MAY rows | 10 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 9 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4486.json](../../../data/rfc/rfc4486.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4486-4-1` | If a BGP speaker decides to terminate its peering with a neighbor because the number of address prefixes received from the neighbor exceeds a locally configured upper bound (as described in [BGP-4]), then the speaker MUST send to the neighbor a NOTIFICATION message with the Error Code Cease and the Error Subcode "Maximum Number of Prefixes Reached" (§4) | MUST | 4 - Subcode Usage | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L114). **negative:** `unit/verify` [`TestPrefixWarningThreshold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L89). **positive:** `functional/verify` [`prefix-maximum-enforce.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefix-maximum-enforce.ci#L6) |
| `RFC4486-4-2` | If a BGP speaker decides to administratively shut down its peering with a neighbor, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Administrative Shutdown" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-3` | If a BGP speaker decides to de-configure a peer, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Peer De-configured" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-4` | If a BGP speaker decides to administratively reset the peering with a neighbor, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Administrative Reset" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-5` | If a BGP speaker decides to disallow a BGP connection (e.g., the peer is not configured locally) after the speaker accepts a transport protocol connection, then the BGP speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Connection Rejected" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-6` | If a BGP speaker decides to administratively reset the peering with a neighbor due to a configuration change other than the ones described above, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Other Configuration Change" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-7` | If a BGP speaker decides to send a NOTIFICATION message with the Error Code Cease as a result of the collision resolution procedure (as described in [BGP-4]), then the subcode SHOULD be set to "Connection Collision Resolution" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-8` | An implementation SHOULD impose an upper bound on the number of consecutive automatic retries (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-9` | It is RECOMMENDED that a BGP speaker behave as though the DampPeerOscillations attribute [BGP-4] were true for this peer when re-trying a BGP connection after the speaker receives a Cease NOTIFICATION message with a subcode of "Administrative Shutdown", "Peer De-configured", "Connection Rejected", or "Out of Resources" (§4) | RECOMMENDED | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-10` | The message MAY optionally include the Address Family information [BGP-MP] and the upper bound in the "Data" field, as shown in Figure 1 (§4) | MAY | 4 - Subcode Usage | **positive:** `unit/verify` [`TestPrefixNotificationDataCarriesTheConfiguredUpperBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L240). **negative:** no negative test |
| `RFC4486-4-11` | If a BGP speaker runs out of resources (e.g., memory) and decides to reset a session, then the speaker MAY send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Out of Resources" (§4) | MAY | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4486 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4486-4-1`](#rfc4486-4-1)

If a BGP speaker decides to terminate its peering with a neighbor because the number of address prefixes received from the neighbor exceeds a locally configured upper bound (as described in [BGP-4]), then the speaker MUST send to the neighbor a NOTIFICATION message with the Error Code Cease and the Error Subcode "Maximum Number of Prefixes Reached" (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPrefixWarningThreshold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L114) | unit/verify | revert, verified |
| positive | [`prefix-maximum-enforce.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefix-maximum-enforce.ci#L6) | functional/verify | not verified |

### [`RFC4486-4-10`](#rfc4486-4-10)

The message MAY optionally include the Address Family information [BGP-MP] and the upper bound in the "Data" field, as shown in Figure 1 (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestPrefixNotificationDataCarriesTheConfiguredUpperBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L240) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 |
| Signed off | 2026-07-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc4486.txt |
| Source fingerprint | ee862bf55c82959b |
| Record | rfc/extraction/rfc4486.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract restates section 1 word for word and states no obligation. |
| `1` | Introduction | 0 | reviewed | Introduction. Says what the document defines and that it 'also recommends' a backoff mechanism. The recommendation itself is stated normatively in section 4 and is captured there as RFC4486-4-9. |
| `2` | not stated | 0 | reviewed | Specification of Requirements: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Subcode Definition | 0 | reviewed | Subcode Definition. The registry table assigning subcodes 1 to 8 to their symbolic names. A value assignment, not a directive: the obligation to USE each value is in section 4. |
| `4` | Subcode Usage | 1 | reviewed | Subcode Usage. The only normative section. Its one capitalised MUST is the site below; the remaining ten sentences are SHOULD, RECOMMENDED or MAY and are captured as RFC4486-4-2 through RFC4486-4-11. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Defines subcodes 1 to 8 in the registry and names the process for future assignments. Binds IANA, not a speaker. |
| `6` | Security Considerations | 0 | reviewed | Security Considerations. One sentence: the extension does not change BGP's underlying security issues. No countermeasure is directed at a speaker. |
| `7` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | Normative References: RFC 2119, RFC 4271, RFC 4760 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 4760. |
| `8.2` | Informative References: RFC 2434, RFC 4020 | 0 | skipped (references) | Informative References: RFC 2434, RFC 4020. |

### Excluded sentences

The review of RFC 4486 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4486, so its obligations are stated where they were written.
