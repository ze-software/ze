# DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE - BGP BFD Strict-Mode

BFD strict mode for BGP, capability code 74. Every requirement listed for DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 4 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 15 | 15 of 15 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |
| Audit verdicts | 4 | of 4 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 4 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BFD strict mode for BGP, capability code 74 |
| Automatic checks | Included |
| Requirements | 4 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tests linked to requirements | 15 |
| Recorded audit verdicts | 4 |
| Fault test records | 15 |
| Summary | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` |
| Requirement table | `rfc/requirements/draft-ietf-idr-bgp-bfd-strict-mode.md` |
| RFC text | `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` |

## Automatic checks

Included in automatic checks: BFD Strict-Mode for BGP (capability code 74): four MUST-level requirements with tagged tests. DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 now has nonzero-hold-down entrypoint proof in TestDraftBFDNonzeroHoldDownInitialFSM: initial FSM and OPEN exchange proceed with Down or newly Up BFD, while establishment waits for the configured interval. The three cases passed, independent semantic review accepted the whole condition, and current native discrimination records and the renewed audit verdict are recorded. Ze advertises the capability from the peer's own bfd block (parsePeerFromTree, internal/component/bgp/reactor/config.go), negotiates it as BfdStrictNegotiated (Negotiate, internal/core/bgp/capability/negotiated.go), and runs the Section 8 FSM procedures in internal/component/bgp/fsm/fsm.go with their wire half in internal/component/bgp/reactor/session_bfd_strict.go. Event 20 sections 8.3.5 and 8.4.5 use the configured RFC 4271 DelayOpenTimer through FSM.EnterDelayOpenBfdUpPending (internal/component/bgp/fsm/delay_open.go). These clauses carry no MUST-level keyword site; their behavioral tests add no requirement rows or RFC tags.

## Published support status

**Area:** BFD strict mode for BGP, capability code 74

**Proof:** 4 of 4 MUST proven

**Evidence:** unit 4

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1`](#draft-ietf-idr-bgp-bfd-strict-mode-4-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1`](#draft-ietf-idr-bgp-bfd-strict-mode-6-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1`](#draft-ietf-idr-bgp-bfd-strict-mode-10-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2`](#draft-ietf-idr-bgp-bfd-strict-mode-10-2)

## Figures

- **MUST bar**: Ze proven 4
- **Overview bar**: Ze MUST proven 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 4 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/draft-ietf-idr-bgp-bfd-strict-mode.json](../../../data/rfc/draft-ietf-idr-bgp-bfd-strict-mode.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1` | If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestSessionBFDStrictConfigChangedUsesConfigSubcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L413). **negative:** `unit/verify` [`TestDraftBFDDisabledConfigChangeReleasesPendingSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L58). **negative:** `unit/verify` [`TestSessionBFDStrictConfigChangedWithBFDDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L495) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1` | A BGP speaker which supports capabilities advertisement and has BFD strict-mode enabled MUST include the BFD Strict-Mode Capability in its OPEN message. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestBFDSettingsStrictParse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L58). **positive:** `unit/verify` [`TestBFDStrictCapabilityInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L115). **negative:** `unit/verify` [`TestBFDSettingsStrictDisabledAdvertisesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L109). **negative:** `unit/verify` [`TestBFDStrictCapabilityInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L116) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1` | To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestDraftBFDNonzeroHoldDownInitialFSM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_initial_fsm_test.go#L35). **positive:** `unit/verify` [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L185). **negative:** `unit/verify` [`TestDraftBFDNonzeroHoldDownInitialFSM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_initial_fsm_test.go#L36). **negative:** `unit/verify` [`TestSessionBFDStrictSendsKeepaliveOnBFDUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L221) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2` | That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker. (§10) | MUST NOT | 10 | **positive:** `unit/verify` [`TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L16). **positive:** `unit/verify` [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L190). **negative:** `unit/verify` [`TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L17). **negative:** `unit/verify` [`TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L289) |

## Gaps and untested MUSTs

DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1`](#draft-ietf-idr-bgp-bfd-strict-mode-4-1)

If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Draft Section 4 Event 35: "If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead." session_bfd_strict.go raiseBFDStrictConfigChanged chooses AdminDown when disabled. All three carriers were read: session_bfd_strict_test.go TestSessionBFDStrictConfigChangedUsesConfigSubcode positively pins enabled Cease/Other Configuration Change and Idle; TestSessionBFDStrictConfigChangedWithBFDDisabled pins no event-side effect outside a pending substate; draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go TestDraftBFDDisabledConfigChangeReleasesPendingSession now checks BOTH pending substates, exact OpenConfirm/Established state, cleared substate and an actual KEEPALIVE. Thus suppressing Event 35 without substituting AdminDown fails rather than satisfying only the first sentence. The enabled/disabled inputs differ in the governing condition, and no unrelated malformed UPDATE confounds the outcome. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDraftBFDDisabledConfigChangeReleasesPendingSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L58) | unit/verify | revert, verified |
| negative | [`TestSessionBFDStrictConfigChangedWithBFDDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L495) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictConfigChangedUsesConfigSubcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L413) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1`](#draft-ietf-idr-bgp-bfd-strict-mode-6-1)

A BGP speaker which supports capabilities advertisement and has BFD strict-mode enabled MUST include the BFD Strict-Mode Capability in its OPEN message. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a strict-mode speaker sending an OPEN without capability 74. TestBFDStrictCapabilityInSentOpen builds peers from configuration and counts code-74 TLVs in the octets sendOpen writes: exactly one with bfd strict, zero with strict under a disabled bfd block and with a bfd block without strict. Red if sendOpen drops or duplicates 74 or emits it without strict-mode enabled. The settings-level units remain as supporting evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBFDSettingsStrictDisabledAdvertisesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L109) | unit/verify | revert, verified |
| negative | [`TestBFDStrictCapabilityInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestBFDSettingsStrictParse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestBFDStrictCapabilityInSentOpen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_b_open_caps_test.go#L115) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1`](#draft-ietf-idr-bgp-bfd-strict-mode-10-1)

To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent reviewer StrictInitialFSMReview read draft Section 10: 'To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled.' TestDraftBFDNonzeroHoldDownInitialFSM starts real Idle sessions with strict mode and 400 ms hold-down for initially Down and newly Up BFD. Session.Start reaches Active, Session.Accept reaches OpenSent and emits the exact OPEN, and ReadAndProcess receives the peer OPEN and KEEPALIVE before clock advancement. The configured hold-down and negotiated strict capability are asserted. Down BFD withholds KEEPALIVE; newly Up BFD permits OpenConfirm but not premature Established. Both remain pending at 399 ms and establish at 400 ms; the zero-hold-down control establishes without advancing the clock. The exact accumulated output and far-end stream reject extra or premature messages. The original zero-interval carriers remain supplementary, not the combined-condition proof. Main observed all three cases pass in job-remaining-bgp-fixture-after-2acfc16f.log and independently read the new test and draft Section 10. This semantic judgment does not claim a producer-break run or independent-daemon interoperability; native discrimination remains separately owed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDraftBFDNonzeroHoldDownInitialFSM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_initial_fsm_test.go#L36) | unit/verify | revert, verified |
| negative | [`TestSessionBFDStrictSendsKeepaliveOnBFDUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestDraftBFDNonzeroHoldDownInitialFSM`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_initial_fsm_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L185) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2`](#draft-ietf-idr-bgp-bfd-strict-mode-10-2)

That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Draft Section 10: "That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker." The preceding sentence requires the FSM enabled, while the following paragraph permits Established to wait for hold-down. TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN configures 400 ms, reads the actual local OPEN from an accepted connection, processes the remote OPEN/KEEPALIVE with an unadvanced FakeClock, and distinguishes Down pending from Up running hold-down. session_bfd_strict_test.go TestSessionBFDStrictWithholdsKeepalive and TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise confine the gate to negotiated strict mode. Producers advanceAfterOpen and bfdStrictHolds gate KEEPALIVE/establishment, not the already observed TCP/OPEN exchange. Delaying connection/OPEN until hold-down elapsed or bypassing the hold-down assertion fails. This closes the old zero-hold-down fixture gap without demanding Established prematurely. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L17) | unit/verify | revert, verified |
| negative | [`TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L289) | unit/verify | revert, verified |
| positive | [`TestDraftBFDHoldDownDoesNotGateConnectionOrOPEN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_bgp_bfd_strict_mode_pending_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L190) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement (spec-bgp-bfd-strict) |
| Signed off | 2026-09-08 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt |
| Source fingerprint | d68744366556d25a |
| Record | rfc/extraction/draft-ietf-idr-bgp-bfd-strict-mode.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, abstract, status of this memo, copyright notice and table of contents. |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `8.3` | not stated | 0 | reviewed | not stated |
| `8.3.1` | not stated | 0 | reviewed | not stated |
| `8.3.2` | not stated | 0 | reviewed | not stated |
| `8.3.3` | not stated | 0 | reviewed | not stated |
| `8.3.4` | not stated | 0 | reviewed | not stated |
| `8.3.5` | not stated | 0 | reviewed | not stated |
| `8.4` | not stated | 0 | reviewed | not stated |
| `8.4.1` | not stated | 0 | reviewed | not stated |
| `8.4.2` | not stated | 0 | reviewed | not stated |
| `8.4.3` | not stated | 0 | reviewed | not stated |
| `8.4.4` | not stated | 0 | reviewed | not stated |
| `8.4.5` | not stated | 0 | reviewed | not stated |
| `8.5` | not stated | 0 | reviewed | not stated |
| `8.5.1` | not stated | 0 | reviewed | not stated |
| `8.5.2` | not stated | 0 | reviewed | not stated |
| `8.5.3` | not stated | 0 | reviewed | not stated |
| `8.5.4` | not stated | 0 | reviewed | not stated |
| `8.5.5` | not stated | 0 | reviewed | not stated |
| `8.5.6` | not stated | 0 | reviewed | not stated |
| `8.6` | not stated | 0 | reviewed | not stated |
| `8.6.1` | not stated | 0 | reviewed | not stated |
| `8.6.2` | not stated | 0 | reviewed | not stated |
| `8.6.3` | not stated | 0 | reviewed | not stated |
| `8.7` | not stated | 0 | reviewed | not stated |
| `8.7.1` | not stated | 0 | reviewed | not stated |
| `8.7.2` | not stated | 0 | reviewed | not stated |
| `8.7.3` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 2 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | skipped (iana) | IANA Considerations: the registrations this document requests. They bind IANA, not an implementation. |
| `13.1` | not stated | 0 | skipped (iana) | IANA Considerations: the registrations this document requests. They bind IANA, not an implementation. |
| `13.2` | not stated | 0 | skipped (iana) | IANA Considerations: the registrations this document requests. They bind IANA, not an implementation. |
| `13.3` | not stated | 0 | skipped (iana) | IANA Considerations: the registrations this document requests. They bind IANA, not an implementation. |
| `14` | Acknowledgement of reviewers | 0 | skipped (acknowledgements) | Acknowledgement of reviewers. |
| `15` | Reference list entries | 0 | skipped (references) | Reference list entries. |
| `16` | Reference list entries | 0 | skipped (references) | Reference list entries. |
| `A` | RFC 7942 implementation status of other vendors | 0 | skipped (appendix-non-normative) | RFC 7942 implementation status of other vendors. Removed on publication by the document's own note. |
| `A.1` | RFC 7942 implementation status of other vendors | 0 | skipped (appendix-non-normative) | RFC 7942 implementation status of other vendors. Removed on publication by the document's own note. |
| `A.2` | RFC 7942 implementation status of other vendors | 0 | skipped (appendix-non-normative) | RFC 7942 implementation status of other vendors. Removed on publication by the document's own note. |
| `A.3` | RFC 7942 implementation status of other vendors | 0 | skipped (appendix-non-normative) | RFC 7942 implementation status of other vendors. Removed on publication by the document's own note. |

### Excluded sentences

The review of DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE, so its obligations are stated where they were written.
