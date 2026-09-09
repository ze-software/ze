# DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE - BGP BFD Strict-Mode

Supported. Every requirement this repository extracted from DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 8 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 4 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 8 |
| Summary | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` |
| Requirement shard | `rfc/requirements/draft-ietf-idr-bgp-bfd-strict-mode.md` |
| RFC text | `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` |

## Enrolment

Enrolled: BFD Strict-Mode for BGP (capability code 74): four MUST-level requirements, all four implemented and each proven by a tagged test. Ze advertises the capability from the peer's own bfd block (parsePeerFromTree, internal/component/bgp/reactor/config.go), negotiates it as BfdStrictNegotiated (Negotiate, internal/core/bgp/capability/negotiated.go), and runs the Section 8 FSM procedures in internal/component/bgp/fsm/fsm.go with their wire half in internal/component/bgp/reactor/session_bfd_strict.go. The two Event 20 sections, 8.3.5 and 8.4.5, are conditional on the RFC 4271 DelayOpenTimer, which Ze does not implement (permitted by RFC 4271 Section 8.2.1.3), and they carry no MUST-level keyword site.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- Capability code 74, length 0, advertised in the OPEN for a peer whose `connection bfd { strict true }` is enabled, and negotiated to `Negotiated.BFDStrictMode` when both speakers send it. FSM events 30 to 35 and the two OpenSent sub-states of Section 8.1 are implemented in `internal/component/bgp/fsm/`. The KEEPALIVE is withheld and the session held in OpenSent by `Session.advanceAfterOpen`, released by `Session.handleBFDEvent`, and closed with Cease / BFD Down or Cease / Other Configuration Change by the same function ([`internal/component/bgp/reactor/session_bfd_strict.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict.go)). The BFD session opens before the BGP FSM starts and outlives a transition to Idle (`Peer.run`, `Peer.cleanup`, Section 7). The BfdHoldTimer of Section 3 attribute 18 lives on `fsm.Timers`, defaults to 30 seconds, and is armed only when the negotiated BGP hold time is zero
- where it is non-zero the ordinary RFC 4271 HoldTimer bounds the wait, re-armed to the negotiated value by `advanceAfterOpen`. The Section 10 BFD hold-down interval is the `hold-down` leaf, in milliseconds, zero by default. Both halves are proven against a second implementation: `test/interop/scenarios/bgp-bfd-strict-speaker` (the lab speaker, which advertises capability 74 and answers BFD built from RFC 5880) and `test/interop/scenarios/bgp-bfd-strict-frr` (FRR 10.3.1, which implements neither). Sections 8.3.5 and 8.4.5 revise Event 20, an OPEN received while the DelayOpenTimer runs
- Ze implements no DelayOpenTimer, an RFC 4271 optional session attribute its Section 8.2.1.3 permits omitting, so `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are unreachable and undeclared. Neither section carries a MUST-level obligation, so that is an implementation gap in RFC 4271's optional feature and not a conformance gap in this draft.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1`](#draft-ietf-idr-bgp-bfd-strict-mode-4-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1`](#draft-ietf-idr-bgp-bfd-strict-mode-6-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1`](#draft-ietf-idr-bgp-bfd-strict-mode-10-1), [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2`](#draft-ietf-idr-bgp-bfd-strict-mode-10-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1` | "If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead" (§4, Event 35) | MUST NOT | 4 | **positive:** `unit/verify` [`TestSessionBFDStrictConfigChangedUsesConfigSubcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L413). **negative:** `unit/verify` [`TestSessionBFDStrictConfigChangedWithBFDDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L495) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1` | "A BGP speaker which supports capabilities advertisement and has BFD strict-mode enabled MUST include the BFD Strict-Mode Capability in its OPEN message" (§6) | MUST | 6 | **positive:** `unit/verify` [`TestBFDSettingsStrictParse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L57). **negative:** `unit/verify` [`TestBFDSettingsStrictDisabledAdvertisesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L108) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1` | "To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled" (§10) | MUST | 10 | **positive:** `unit/verify` [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L185). **negative:** `unit/verify` [`TestSessionBFDStrictSendsKeepaliveOnBFDUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L221) |
| `DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2` | "That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker" (§10) | MUST NOT | 10 | **positive:** `unit/verify` [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L190). **negative:** `unit/verify` [`TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L289) |

## Gaps and untested MUSTs

DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1`](#draft-ietf-idr-bgp-bfd-strict-mode-4-1)

"If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead" (§4, Event 35)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionBFDStrictConfigChangedWithBFDDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L495) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictConfigChangedUsesConfigSubcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L413) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1`](#draft-ietf-idr-bgp-bfd-strict-mode-6-1)

"A BGP speaker which supports capabilities advertisement and has BFD strict-mode enabled MUST include the BFD Strict-Mode Capability in its OPEN message" (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBFDSettingsStrictDisabledAdvertisesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestBFDSettingsStrictParse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_bfd_strict_test.go#L57) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1`](#draft-ietf-idr-bgp-bfd-strict-mode-10-1)

"To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled" (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionBFDStrictSendsKeepaliveOnBFDUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L185) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2`](#draft-ietf-idr-bgp-bfd-strict-mode-10-2)

"That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker" (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L289) | unit/verify | revert, verified |
| positive | [`TestSessionBFDStrictWithholdsKeepalive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_bfd_strict_test.go#L190) | unit/verify | revert, verified |

## Extraction sign-off

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
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `8.3.2` | not stated | 0 | walked | not stated |
| `8.3.3` | not stated | 0 | walked | not stated |
| `8.3.4` | not stated | 0 | walked | not stated |
| `8.3.5` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.4.1` | not stated | 0 | walked | not stated |
| `8.4.2` | not stated | 0 | walked | not stated |
| `8.4.3` | not stated | 0 | walked | not stated |
| `8.4.4` | not stated | 0 | walked | not stated |
| `8.4.5` | not stated | 0 | walked | not stated |
| `8.5` | not stated | 0 | walked | not stated |
| `8.5.1` | not stated | 0 | walked | not stated |
| `8.5.2` | not stated | 0 | walked | not stated |
| `8.5.3` | not stated | 0 | walked | not stated |
| `8.5.4` | not stated | 0 | walked | not stated |
| `8.5.5` | not stated | 0 | walked | not stated |
| `8.5.6` | not stated | 0 | walked | not stated |
| `8.6` | not stated | 0 | walked | not stated |
| `8.6.1` | not stated | 0 | walked | not stated |
| `8.6.2` | not stated | 0 | walked | not stated |
| `8.6.3` | not stated | 0 | walked | not stated |
| `8.7` | not stated | 0 | walked | not stated |
| `8.7.1` | not stated | 0 | walked | not stated |
| `8.7.2` | not stated | 0 | walked | not stated |
| `8.7.3` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
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

The walk over DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE, so its obligations are stated where they were written.
