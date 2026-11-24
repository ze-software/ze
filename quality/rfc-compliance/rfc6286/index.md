# RFC 6286 - Autonomous-System-Wide Unique BGP Identifier for BGP-4

AS-wide unique BGP Identifier (default configuration). Every requirement listed for RFC 6286, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

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
| Tests shown to detect a fault | 18 | 18 of 18 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | AS-wide unique BGP Identifier (default configuration) |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tests linked to requirements | 18 |
| Recorded audit verdicts | 5 |
| Fault test records | 18 |
| Summary | `rfc/short/rfc6286.md` |
| Requirement table | `rfc/requirements/rfc6286.md` |
| RFC text | `rfc/full/rfc6286.txt` |

## Automatic checks

Included in automatic checks: Autonomous-System-Wide Unique BGP Identifier for BGP-4: three short protocol revisions to RFC 4271 that ze now implements end to end -- the Section 2.2 reception check on both OPEN rails, the Section 2.1 AS-wide identifier claim (and its operator opt-out), and the Section 2.3 collision tie-break.

## Published support status

**Area:** AS-wide unique BGP Identifier (default configuration)

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

**Positive and negative tests (4):** [`RFC6286-2.1-1`](#rfc6286-2.1-1), [`RFC6286-2.2-1`](#rfc6286-2.2-1), [`RFC6286-2.2-2`](#rfc6286-2.2-2), [`RFC6286-2.3-1`](#rfc6286-2.3-1)

## Figures

- **MUST bar**: Ze verified 4
- **Overview bar**: Ze MUST verified 4, SHOULD verified 1

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
| SHOULD verified | 1 |
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

Source: [data/rfc/rfc6286.json](../../../data/rfc/rfc6286.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6286-2.1-1` | The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer. (Section 2.1) | MUST | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L128). **positive:** `unit/verify` [`TestRFC6286DefaultIdentifierSharedOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_default_identifier_test.go#L30). **positive:** `unit/verify` [`TestRFC6286DefaultIdentifierSurvivesUnchangedReload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_default_reload_test.go#L24). **positive:** `unit/verify` [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L35). **negative:** `unit/verify` [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L39). **negative:** `unit/verify` [`TestRFC6286NonZeroRouterIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L10) |
| `RFC6286-2.1-2` | The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. (Section 2.1) | SHOULD | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestRFC6286LocalIdentifierUniqueWithinItsAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_unique_within_as_test.go#L24). **positive:** `unit/verify` [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_routerid_unique_test.go#L468). **negative:** no negative test. **{single-polarity}:** the negative case -- ze accepting a duplicate -- is the operator-selected `bgp/session/allow-shared-router-id true` path, which is conformant precisely because the requirement is a lowercase "should", so there is no violation for a negative test to catch. Enforcement is proven by TestRouterIDClaimConcurrentOnlyOneWins and the TestRouterIDConflict* family; the opt-out by TestValidateOpenAllowSharedRouterID |
| `RFC6286-2.2-1` | If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L105). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L453). **positive:** `unit/verify` [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L166). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L455) |
| `RFC6286-2.2-2` | If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L107). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L457). **negative:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L109). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L459) |
| `RFC6286-2.3-1` | If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved. (Section 2.3) | MUST | 2.3 - Connection Collision Resolution | **positive:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L585). **negative:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L589) |

## Gaps and untested MUSTs

RFC 6286 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6286-2.1-1`](#rfc6286-2.1-1)

The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer. (Section 2.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/config.go::parsePeerSettings moved. RFC 6286 Section 2.1 defines a 4-octet, unsigned, non-zero identifier determined at startup and shared across interfaces and peers. This judgment covers the default configuration stated in Conformance Configuration: one nonzero global bgp/router-id, no session override, and no operator-requested identifier change. TestRFC6286DefaultIdentifierSharedOnWire drives ReadConfig, group resolution, PeersFromTree and Session.Accept, then decodes the actual OPEN for three peers across configured IPv4 and IPv6 local addresses, repeated session construction, and identifiers 1, 0x80000000 and 0xffffffff. Its recorded wrong-identifier break changes each expected wire value and fails this unit. TestRFC6286DefaultIdentifierSurvivesUnchangedReload asserts startup identity after initial apply, identity and both Peer objects after unchanged reload, and the OPEN identifier both peer settings emit. Its recorded unsolicited +1 change in the unchanged-global branch fails the initial identity assertion. Existing config tests reject explicit global and peer zero, accept identifier 1 and require the exact inherited value. These separate assertions cover the whole default-path definition. Explicit peer/group/peer-only identifiers and operator-requested reload changes remain supported controls, separately tested and outside this default-path claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestRFC6286NonZeroRouterIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L10) | unit/verify | revert, verified |
| positive | [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC6286DefaultIdentifierSharedOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_default_identifier_test.go#L30) | unit/verify | mutant, verified |
| positive | [`TestRFC6286DefaultIdentifierSurvivesUnchangedReload`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_default_reload_test.go#L24) | unit/verify | mutant, verified |
| positive | [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L35) | unit/verify | revert, verified |

### [`RFC6286-2.1-2`](#rfc6286-2.1-2)

The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. (Section 2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Section 2.1 uses lowercase should for AS-wide uniqueness. The single-positive annotation is appropriate: intentional duplication remains accepted through allow-shared-router-id, while Section 2.2 still rejects zero and internal self identifiers. TestRouterIDClaimConcurrentOnlyOneWins prepares sixteen peers of one AS, validates one shared identifier concurrently, requires exactly one acceptance, requires a routerIDConflictError for every other peer, and verifies the holder and absence of an unrelated claim. Peer.validateOpen and routerIDClaims.claim use the judged AS and serialize claim insertion. TestRFC6286LocalIdentifierUniqueWithinItsAS separately requires exact NOTIFICATION 2/3 and no OpenConfirm for the local identifier from an internal peer. Blanket acceptance and blanket refusal cannot satisfy these assertions. This row credits default advisory uniqueness, not an assertion that explicit operator options have been removed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_routerid_unique_test.go#L468) | unit/verify | mutant, verified |
| positive | [`TestRFC6286LocalIdentifierUniqueWithinItsAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_unique_within_as_test.go#L24) | unit/verify | revert, verified |

### [`RFC6286-2.2-1`](#rfc6286-2.2-1)

If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Section 2.2's zero-identifier condition is isolated by TestOpenValidateBGPIdentifier: zero from internal and external peers requires OPEN Message Error / Bad BGP Identifier with empty data, while valid boundary identifiers 1 and 0xffffffff are accepted. The current recorded clause breaks disable only zero rejection or reject every nonzero identifier and demonstrate the corresponding exact assertions. TestHandleOpenRejectsBadBGPIdentifier repeats refusal through handleOpen and asserts the error, exact transmitted NOTIFICATION and lack of OpenConfirm; its valid cases retain nil error and OpenConfirm and now close/drain the recorded connection to assert no NOTIFICATION. TestProcessOpenRejectsBadBGPIdentifier verifies exact zero refusal through the collision-winner rail. The two OPEN rails call validateOpenIdentifier and cannot bypass this condition.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L455) | unit/verify | mutant, verified |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L453) | unit/verify | mutant, verified |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L105) | unit/verify | mutant, verified |
| positive | [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L166) | unit/verify | mutant, verified |

### [`RFC6286-2.2-2`](#rfc6286-2.2-2)

If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Section 2.2's self-identifier condition is isolated by TestOpenValidateBGPIdentifier: an internal peer presenting the local nonzero identifier requires exact error 2/3, an external peer presenting that identifier is accepted, and internal peers with localID-1 or localID+1 are accepted. Current clause-break records remove internal-self refusal or remove the internal-peer guard and expose the corresponding wrong outcomes. TestHandleOpenRejectsBadBGPIdentifier repeats both sides through handleOpen; its strengthened valid arm additionally closes/drains the captured connection and asserts no NOTIFICATION. validateOpenIdentifier judges the peer AS through the prepared proposal and isIBGPWith; the selected ASN4 reader tests cover the AS_TRANS/capability distinction. Zero-input cases separately cover the sentence's first condition and do not substitute for the nonzero self-identifier cases.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L459) | unit/verify | mutant, verified |
| negative | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L109) | unit/verify | mutant, verified |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L457) | unit/verify | mutant, verified |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L107) | unit/verify | mutant, verified |

### [`RFC6286-2.3-1`](#rfc6286-2.3-1)

If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Section 2.3 preserves the connection initiated by the larger-AS speaker when identifiers are identical. TestDetectCollisionEqualIdentifierPrefersLargerAS requires accept-and-close-existing when the remote AS is larger and reject-and-keep-existing when the local AS is larger, both with equal identifiers. Its negative rows use the same AS ordering but unequal identifiers and require the RFC 4271 identifier comparison in both directions. Thus blanket local or remote preference and applying the AS rule to unequal identifiers fail exact assertions. Session.detectCollision reads the frozen local proposal and judged peer AS. TestASN4DisabledSessionReadersJudgeMyAS additionally drives this consumer with AS_TRANS below the local AS and the four-octet capability ASN above it, requiring opposite collision winners, so four-octet AS support is not inferred solely from a uint32 field.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L589) | unit/verify | mutant, verified |
| positive | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L585) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc6286 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc6286.txt |
| Source fingerprint | c09b7508ad62e214 |
| Record | rfc/extraction/rfc6286.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | reviewed | Title block, Abstract, Status of This Memo and Copyright Notice. Walked rather than skipped because the site scan attributes two sites here, both excluded below: the Abstract's restatement of what the document does to RFC 4271, and the IETF Trust Legal Provisions boilerplate. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 1 | reviewed | Introduction. Reports that RFC 4271 specifies the BGP Identifier as a valid IPv4 host address assigned to the speaker, that deployed code requires two speakers to carry distinct Identifiers, and that this document relaxes both so an IPv6-only network can comply. Its one site, 1:1, is that report and is excluded below. The relaxed definition itself is stated in section 2.1. |
| `2` | Protocol Revisions, the parent heading | 0 | reviewed | Protocol Revisions, the parent heading. One sentence naming what the revisions cover: the definition of the BGP Identifier and the procedures for a speaker that supports the AS-wide Unique BGP Identifier. It directs nobody; 2.1, 2.2 and 2.3 carry the obligations. |
| `2.1` | Definition of the BGP Identifier | 0 | reviewed | Definition of the BGP Identifier. The obligation is the indented replacement text: 'The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer.' It is written in the indicative with a lowercase 'should', so neither the capitalised scan nor the modal scan attributes a site to this section, and both ids read from it are declared unsourced here. RFC6286-2.1-1 is the 4-octet, unsigned, NON-ZERO definition, whose non-zero half ze enforces on reception through Open.ValidateBGPIdentifier and refuses to originate through parseRouterID (internal/component/bgp/reactor/config.go). RFC6286-2.1-2 is the lowercase 'should' of AS-wide uniqueness, enforced by default through the claim registry in internal/component/bgp/reactor/routerid_unique.go, which keys routerIDKey on (peer AS, identifier), with bgp/session/allow-shared-router-id as the operator opt-out the advisory level permits. |
| `2.2` | Open Message Error Handling | 0 | reviewed | Open Message Error Handling. The obligation is the indented replacement text for RFC 4271 section 6.2: 'If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier".' Written in the indicative with no modal at all, so no scan attributes a site to this section. It states two rejection conditions and the second is gated on the peer being internal, which is why the summary declares two ids and both are declared unsourced here. Session.validateOpenIdentifier (internal/component/bgp/reactor/session_open_validation.go) is the producer for both, on the handleOpen and processOpen rails. |
| `2.3` | Connection Collision Resolution | 0 | reviewed | Connection Collision Resolution. The obligation is the indented extension to RFC 4271 section 6.8: 'If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved.' Indicative, no modal, no site. Declared unsourced here as RFC6286-2.3-1. The lead-in scopes it to an external peer, because an internal peer presenting this speaker's identifier is already rejected by 2.2, and the closing sentence says the extension covers 4-octet AS numbers [RFC4893]. Both facts qualify the same single obligation and neither adds one. |
| `3` | Remarks | 0 | reviewed | Remarks. Five indicative observations, none of which directs a speaker: an Identifier allocated per RFC 4271 already fits the revised definition; a confederation counts as one AS for the purpose of AS-wide uniqueness; a speaker supporting this cannot share an Identifier with an external neighbor until that neighbor is upgraded, which is a consequence of the un-upgraded neighbor still applying RFC 4271's stricter test; and the Identifier is also used in AGGREGATOR, in the route-reflection attributes and in route selection. The section's own closing sentence states its purpose: to conclude that the revisions introduce no backward compatibility issue. The confederation observation scopes the lowercase 'should' of 2.1 and adds no obligation of its own, so it earns no id; rfc/short/rfc6286.md carries all five observations under Remarks. |
| `4` | Security Considerations | 0 | reviewed | Security Considerations. Two sentences: this extension introduces no new security considerations, and BGP's are discussed in RFC 4271. No countermeasure is directed at a speaker. |
| `5` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `6` | Normative References: RFC 4271 and RFC 4893 | 0 | skipped (references) | Normative References: RFC 4271 and RFC 4893. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Abstract, non-normative by RFC convention and word-for-word the second paragraph of section 1. Its lowercase 'required' is inside a report of the document's own editorial effect on RFC 4271 -- it 'relaxes the "uniqueness" requirement so that only Autonomous-System-wide (AS-wide) uniqueness of the BGP Identifiers is required' -- which states what scope the revised requirement has, not an obligation on a speaker. The obligation itself is the indented replacement text of section 2.1, invisible to both scans and declared as unsourced RFC6286-2.1-1 and RFC6286-2.1-2 there. | To accommodate situations where the current requirements for the BGP Identifier are not met, this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only Autonomous-System- wide (AS-wide) uniqueness of the BGP Identifiers is required. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no protocol behavior at a BGP speaker. RFC 6286 contains no code component: it revises three prose paragraphs of RFC 4271 and defines no syntax. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's second paragraph, the same report as front:1 with the IPv6-only network named as the motivating case. It says what this document does to RFC 4271's text, so its lowercase 'required' describes the new scope of the uniqueness requirement rather than imposing one. Classified not-a-requirement rather than duplicate-of because a duplicate must name an id another site MAPS, and no site of this document maps anything: every obligation is stated in the indicative and is declared unsourced on 2.1, 2.2 and 2.3. | To accommodate situations where the current requirements for the BGP Identifier are not met (such as in the case of an IPv6-only network), this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only AS-wide uniqueness of the BGP Identifiers is required. |

## Superseded

No document obsoletes RFC 6286, so its obligations are stated where they were written.
