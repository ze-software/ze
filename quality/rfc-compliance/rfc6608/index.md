# RFC 6608 - Subcodes for BGP Finite State Machine Error

BGP FSM Error subcodes. Every requirement listed for RFC 6608, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 3 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 3 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 3 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 3 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 3 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 5 | 5 of 6 tests linked to requirements, 0 without a fault test and 1 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 3 | of 3 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 3 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 3 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 3 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 3 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 3 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 3 | of 3 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 3 mandatory requirements: they add up to 3. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | BGP FSM Error subcodes |
| Automatic checks | Included |
| Requirements | 3 |
| Mandatory requirements (MUST level) | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tests linked to requirements | 6 |
| Recorded audit verdicts | 3 |
| Fault test records | 6 |
| Summary | `rfc/short/rfc6608.md` |
| Requirement table | `rfc/requirements/rfc6608.md` |
| RFC text | `rfc/full/rfc6608.txt` |

## Automatic checks

Included in automatic checks: Three MUSTs requiring a code-5 FSM Error NOTIFICATION with state-specific subcode 1/2/3 and exactly one received-message type octet. The reactor retains the receiving state before FSM transition, diagnoses unexpected messages through notifyFSMMessageError (internal/component/bgp/reactor/session_fsm_notification.go), refuses early ROUTE-REFRESH before capability filtering, and closes the connection. The real TCP peer matrices and valid-message controls in rfc6608_fsm_notifications_test.go bind each row; verified discrimination records determine the proven share.

## Published support status

**Area:** BGP FSM Error subcodes

**Test verification:** 3 of 3 MUST proven

**Evidence:** unit 3

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **3** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC6608-4-1`](#rfc6608-4-1), [`RFC6608-4-2`](#rfc6608-4-2), [`RFC6608-4-3`](#rfc6608-4-3)

## Figures

- **MUST bar**: Ze verified 3
- **Overview bar**: Ze MUST verified 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 3 |
| MUST rows kept (the MUST bar) | 3 |
| Ze verified | 3 |
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

Source: [data/rfc/rfc6608.json](../../../data/rfc/rfc6608.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6608-4-1` | If a BGP speaker receives an unexpected message (e.g., KEEPALIVE/ UPDATE/ROUTE-REFRESH message) on a session in OpenSent state, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenSent State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6608OpenSentUnexpectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L23). **negative:** `unit/verify` [`TestRFC6608OpenSentExpectedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L79) |
| `RFC6608-4-2` | If a BGP speaker receives an unexpected message (e.g., OPEN/UPDATE/ ROUTE-REFRESH message) on a session in OpenConfirm state, it MUST send a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenConfirm State" to the neighbor. The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6608OpenConfirmUnexpectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L49). **negative:** `unit/verify` [`TestRFC6608OpenConfirmExpectedKeepaliveKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L90) |
| `RFC6608-4-3` | If a BGP speaker receives an unexpected message (e.g., OPEN message) on a session in Established State, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in Established State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC6608EstablishedUnexpectedOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L70). **negative:** `unit/verify` [`TestRFC6608EstablishedExpectedMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L98) |

## Gaps and untested MUSTs

RFC 6608 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6608-4-1`](#rfc6608-4-1)

If a BGP speaker receives an unexpected message (e.g., KEEPALIVE/ UPDATE/ROUTE-REFRESH message) on a session in OpenSent state, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenSent State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC 6608 Section 4 in full. The TCP matrix receives KEEPALIVE, well-formed and malformed UPDATE, ROUTE-REFRESH, and non-version NOTIFICATION in ordinary OpenSent. It compares the entire notification payload to code 5/subcode 1/the single received-type octet, requires exactly one packet and real EOF, then waits for session release. The valid OPEN control reaches OpenConfirm and checks no notification, close, session replacement or retry increment. This excludes both silent discard and unconditional refusal. The proposed producer substitution exchanges state subcode 1 for 2; its observed red remains the native record's responsibility. BFD strict-mode pending is outside this ordinary-state matrix and retains its separate tests.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6608OpenSentExpectedOpenKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L79) | unit/verify | mutant, verified |
| positive | [`TestRFC6608OpenSentUnexpectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L23) | unit/verify | mutant, verified |

### [`RFC6608-4-2`](#rfc6608-4-2)

If a BGP speaker receives an unexpected message (e.g., OPEN/UPDATE/ ROUTE-REFRESH message) on a session in OpenConfirm state, it MUST send a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenConfirm State" to the neighbor. The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC 6608 Section 4 in full. After the actual OPEN exchange and before peer KEEPALIVE, the TCP matrix receives another same-connection OPEN, well-formed and malformed UPDATE, and ROUTE-REFRESH. It compares the entire notification payload to code 5/subcode 2/the single received-type octet, requires exactly one packet and real EOF, then waits for session release. The expected KEEPALIVE control reaches Established and checks no notification, close, session replacement or retry increment. The proposed producer substitution exchanges state subcode 2 for 1; its observed red remains the native record's responsibility. A separate pending TCP connection is collision resolution, not this test's second OPEN on the same connection.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6608OpenConfirmExpectedKeepaliveKeepsTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L90) | unit/verify | mutant, verified |
| positive | [`TestRFC6608OpenConfirmUnexpectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L49) | unit/verify | mutant, verified |

### [`RFC6608-4-3`](#rfc6608-4-3)

If a BGP speaker receives an unexpected message (e.g., OPEN message) on a session in Established State, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in Established State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently reread RFC 6608 Section 4, the complete requirement, both tagged units and their producers after the approved control strengthening. The RFC requires a NOTIFICATION with Finite State Machine Error and Receive Unexpected Message in Established State; its Data sentence says: The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. TestRFC6608EstablishedUnexpectedOpen completes the actual OPEN/KEEPALIVE handshake before sending another same-connection OPEN. It requires exactly one packet, compares the entire NOTIFICATION payload to code 5/subcode 3/one OPEN-type octet, observes real TCP EOF rather than a timeout and waits for release of that Session. A separate pending TCP connection is collision resolution and is not this input. TestRFC6608EstablishedExpectedMessagesKeepTheConnection retains all three original valid KEEPALIVE, UPDATE and ROUTE-REFRESH frames and every per-frame no-notification, open-socket, Established-state, same-session and zero-retry assertion. It now follows them with a terminal same-connection OPEN and requires the sole exact 5/3/Data-1 notification, EOF, session release and retry count one. TCP FIFO makes that terminal reply a causal processing barrier for the preceding valid traffic: an earlier refusal has another message-type Data octet, an extra response, a prematurely closed socket or an excess retry count. The negative claim explicitly reserves error and closure for the terminal marker. Its ROUTE-REFRESH arm proves preservation of the established session, not negotiation or a refresh response. The named positive and strengthened negative both pass in the main thread's race run job-fsm-reviewed-controls-0091f069.log. Retain the refreshed native negative discrimination before stamping this rejudgment; the positive record's state-subcode substitution remains the separate exact-diagnostic proof. This verdict judges this requirement's session-bound behavior and does not claim an external-daemon scenario has run.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6608EstablishedExpectedMessagesKeepTheConnection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L98) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC6608EstablishedUnexpectedOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6608_fsm_notifications_test.go#L70) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6608.txt |
| Source fingerprint | a864ccc2fecb4951 |
| Record | rfc/extraction/rfc6608.json |
| Mapped sentences | 3 |
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
| `4` | not stated | 3 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 6608 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 6608, so its obligations are stated where they were written.
