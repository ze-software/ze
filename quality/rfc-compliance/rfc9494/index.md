# RFC 9494 - Long-Lived Graceful Restart for BGP

Partial. Every requirement this repository extracted from RFC 9494, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 72.0% | 18 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 25 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 41.5% | 27 of 65 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 36 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 8.0% | 2 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 5 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 19 | of 25 gated MUSTs judged | 4 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 36 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 65 |
| Tagged units | 65 |
| Recorded audit verdicts | 19 |
| Discrimination records | 27 |
| Summary | `rfc/short/rfc9494.md` |
| Requirement shard | `rfc/requirements/rfc9494.md` |
| RFC text | `rfc/full/rfc9494.txt` |

## Enrolment

Enrolled: Long-Lived Graceful Restart for BGP

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Helper-side LLGR: capability code 71 declared only alongside GR and disregarded when GR is absent, per-family Long-Lived Stale Time decode and advertisement, GR-to-LLGR handover with per-family LLST timers, NO_LLGR deletion and LLGR_STALE attachment on LLGR entry, LLGR_STALE routes treated as least preferred in best-path, and partial deployment toward non-LLGR neighbors (NO_EXPORT plus LOCAL_PREF=0 for iBGP, withdrawal for eBGP) (internal/component/bgp/plugins/gr, internal/component/bgp/plugins/rib). Requirements bound per line in [`rfc/short/rfc9494.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9494.md).

**What the ledger says remains**

Five MUST gaps annotated in [`rfc/short/rfc9494.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9494.md): LLGR_STALE is not protected on further advertisement -- a peer whose `send-community` omits `standard` (or is `none`) has the whole COMMUNITIES attribute suppressed on the readvertise rails ([`internal/component/bgp/reactor/peer_forward_facts.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts.go), reactor_api_forward.go, forward_rs.go), and a `community-remove ffff0006` filter strips it through removeValues, which exempts no value ([`internal/component/bgp/plugins/filter_community/handler.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/handler.go)) ([`RFC9494-4.3-2`](#rfc9494-4.3-2)); on a consecutive session drop ze applies the RFC 4724 purge of previously stale routes and re-arms a full LLST timer, so LLGR-marked routes are deleted early and the running timer is reset ([`RFC9494-4.2-7`](#rfc9494-4.2-7), 4.2-9); LLST timers are stopped at re-establishment, so an LLST elapsing during synchronization is not recorded and a subsequent reset removes nothing immediately ([`RFC9494-4.2-10`](#rfc9494-4.2-10)); and long-lived-stale-time is configured per peer and applied to every negotiated family rather than per AFI/SAFI ([`RFC9494-5-2`](#rfc9494-5-2)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 18 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (18):** [`RFC9494-3.1-1`](#rfc9494-3.1-1), [`RFC9494-4.1-1`](#rfc9494-4.1-1), [`RFC9494-3.1-2`](#rfc9494-3.1-2), [`RFC9494-4-1`](#rfc9494-4-1), [`RFC9494-4.2-1`](#rfc9494-4.2-1), [`RFC9494-4.2-2`](#rfc9494-4.2-2), [`RFC9494-4.2-3`](#rfc9494-4.2-3), [`RFC9494-4.2-4`](#rfc9494-4.2-4), [`RFC9494-4.2-5`](#rfc9494-4.2-5), [`RFC9494-4.2-6`](#rfc9494-4.2-6), [`RFC9494-4.2-8`](#rfc9494-4.2-8), [`RFC9494-4.3-1`](#rfc9494-4.3-1), [`RFC9494-4.4-1`](#rfc9494-4.4-1), [`RFC9494-4.5-1`](#rfc9494-4.5-1), [`RFC9494-4.6-1`](#rfc9494-4.6-1), [`RFC9494-4.6-2`](#rfc9494-4.6-2), [`RFC9494-4.6-3`](#rfc9494-4.6-3), [`RFC9494-5-1`](#rfc9494-5-1)

**Annotated (including scoped evidence) (7):** [`RFC9494-4.2-7`](#rfc9494-4.2-7), [`RFC9494-4.2-9`](#rfc9494-4.2-9), [`RFC9494-4.3-2`](#rfc9494-4.3-2), [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1), [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2), [`RFC9494-5-2`](#rfc9494-5-2), [`RFC9494-4.2-10`](#rfc9494-4.2-10)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9494-3.1-1` | If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1. (§3.1, §4.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L29). **negative:** `unit/verify` [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L53) |
| `RFC9494-4.1-1` | If it is not so advertised, the LLGR Capability MUST be disregarded. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L658). **negative:** `unit/verify` [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L745) |
| `RFC9494-3.1-2` | The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L71). **negative:** `unit/verify` [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L91) |
| `RFC9494-4-1` | If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L583). **negative:** `unit/verify` [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L610) |
| `RFC9494-4.2-1` | After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L211). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L27). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L110). **positive:** `unit/verify` [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L706). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L173). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L134). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L64). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L164). **negative:** `unit/verify` [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L604). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L98) |
| `RFC9494-4.2-2` | * For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long- Lived Stale Time. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L212). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L28). **positive:** `unit/verify` [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L111). **positive:** `unit/verify` [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L569). **positive:** `unit/verify` [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L135). **negative:** `unit/verify` [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L165). **negative:** `unit/verify` [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L768). **negative:** `unit/verify` [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L99) |
| `RFC9494-4.2-3` | If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L662). **positive:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L174). **positive:** `unit/verify` [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L208). **negative:** `unit/verify` [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L175) |
| `RFC9494-4.2-4` | * The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L618). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L65). **negative:** `unit/verify` [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) |
| `RFC9494-4.2-5` | * If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L659). **positive:** `unit/verify` [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L79). **positive:** `unit/verify` [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L66). **negative:** `unit/verify` [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L99). **negative:** `unit/verify` [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63). **positive:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6). **negative:** `functional/verify` [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) |
| `RFC9494-4.2-6` | * The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137). **negative:** `unit/verify` [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) |
| `RFC9494-4.2-7` | However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| `RFC9494-4.2-8` | Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L846). **negative:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L796) |
| `RFC9494-4.2-9` | If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| `RFC9494-4.3-1` | A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820). **negative:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852) |
| `RFC9494-4.3-2` | The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| `RFC9494-4.4-1` | A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755). **negative:** `unit/verify` [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) |
| `RFC9494-4.5-1` | If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L327). **negative:** `unit/verify` [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L288) |
| `RFC9494-4.6-1` | The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L430). **negative:** `unit/verify` [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L400) |
| `RFC9494-4.6-2` | * The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L434). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L183). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L370) |
| `RFC9494-4.6-3` | * The stale routes MUST have their LOCAL_PREF set to zero. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L437). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L187). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L374) |
| `RFC9494-4.7.2-1` | In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no ATTR_SET attribute and no RFC 6368 iBGP PE-CE model -- the path attribute code table stops at code 40 plus the provisional 252 and contains no code 128 (internal/core/bgp/attribute/attribute.go:46-66), `grep -rni "attr_set\\\|attrset" internal/core/` returns nothing, and `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, so there is no PE that imports a VPN route into a CE session |
| `RFC9494-4.7.2-2` | In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no PE export of a CE route into a VPN address family -- `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, and the only route-target handling is parsing of the extended community and the RTC NLRI codec (internal/component/bgp/route/route_community.go:275, internal/component/bgp/plugins/nlri/rtc/), with no VRF import/export engine that could carry a CE route's LLGR_STALE into a VPN route |
| `RFC9494-5-1` | Implementations MUST NOT enable these procedures by default. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L115). **positive:** `unit/verify` [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119). **negative:** `unit/verify` [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L80). **negative:** `unit/verify` [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) |
| `RFC9494-5-2` | They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| `RFC9494-4.2-10` | If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |
| `RFC9494-4.2-11` | The timers received in the Long-Lived Graceful Restart Capability SHOULD be modifiable by local configuration, which may impose an upper bound, a lower bound, or both on their respective values. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.3-3` | The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received. The exception is described in Section 4.6. (§4.3) | SHOULD NOT | 4.3 | **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L157). **negative:** `unit/verify` [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L341). **positive:** `functional/verify` [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L18) |
| `RFC9494-4.7.1-1` | Finally, if this exception is used, the implementation SHOULD, by default, attach the NO_EXPORT community to the routes in question, as an additional protection against stale routes spreading without limit. (§4.7.1) | SHOULD | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.2-1` | An implementation MAY allow users to configure policies that accept, reject, or modify routes based on the presence or absence of this community. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.3-1` | An implementation MAY allow users to configure policies that accept, reject, or modify routes based on the presence or absence of this community. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.2-12` | The value of a Long-Lived Stale Time in the capability received from a neighbor MAY be reduced by local configuration. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.6-4` | However, to facilitate incremental deployment, stale routes MAY be advertised to neighbors that have not advertised the Long-Lived Graceful Restart Capability under the following conditions: * The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-2` | For this reason, an implementation MAY advertise stale routes over a PE-CE session, when explicitly configured to do so. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-3` | an implementation MAY advertise stale routes over a PE-CE session, when explicitly configured to do so. That is, the second rule listed in Section 4.3 MAY be disregarded in such cases. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-4` | Attachment of the NO_EXPORT community MAY be disabled by explicit configuration in order to accommodate exceptional cases. (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.2-3` | If the CE router does not support the procedures of this document: Then the optional procedures of Section 4.6 MAY be followed, attaching the NO_EXPORT community and setting the value of LOCAL_PREF to zero, overriding the value found in the ATTR_SET. (§4.7.2) | MAY | 4.7.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9494-4.2-7`](#rfc9494-4.2-7) However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2) | {gap}, no test | ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| [`RFC9494-4.2-9`](#rfc9494-4.2-9) If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2) | {gap}, no test | a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| [`RFC9494-4.3-2`](#rfc9494-4.3-2) The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | {gap}, no test | two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1) In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no ATTR_SET attribute and no RFC 6368 iBGP PE-CE model -- the path attribute code table stops at code 40 plus the provisional 252 and contains no code 128 (internal/core/bgp/attribute/attribute.go:46-66), `grep -rni "attr_set\\\|attrset" internal/core/` returns nothing, and `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, so there is no PE that imports a VPN route into a CE session |
| [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2) In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no PE export of a CE route into a VPN address family -- `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, and the only route-target handling is parsing of the extended community and the RTC NLRI codec (internal/component/bgp/route/route_community.go:275, internal/component/bgp/plugins/nlri/rtc/), with no VRF import/export engine that could carry a CE route's LLGR_STALE into a VPN route |
| [`RFC9494-5-2`](#rfc9494-5-2) They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5) | {gap}, no test | ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| [`RFC9494-4.2-10`](#rfc9494-4.2-10) If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2) | {gap}, no test | ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9494-3.1-1`](#rfc9494-3.1-1)

If the LLGR capability is advertised, the Graceful Restart capability [RFC4724] MUST also be advertised; see Section 4.1. (§3.1, §4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: declaring code 71 without code 64. TestRFC9494_LLGRCapDeclaredWithGRCap fails unless extractGRCapabilities returns one code-64 declaration for the same peer as the code-71 one (require.Len grCaps 1, Code 64, same Peers). Negative: TestRFC9494_NoLLGRCapWithoutGRContainer places long-lived-stale-time outside the graceful-restart container and fails if either code 71 or code 64 is declared, so no config shape yields 71 alone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L53) | unit/verify | unproven |
| positive | [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L29) | unit/verify | unproven |

### [`RFC9494-4.1-1`](#rfc9494-4.1-1)

If it is not so advertised, the LLGR Capability MUST be disregarded. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: honouring code 71 received without code 64. TestHandleStructuredOpenLLGRNoGR sends an OPEN with only code 71 and fails if peerLLGRCaps holds the peer (assert.False llgrOK), which is the map onSessionDown reads. Negative: TestHandleStructuredOpenGRPlusLLGR keeps both capabilities when code 64 is present, with families, F bits and LLST 7200/3600 asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L745) | unit/verify | unproven |
| positive | [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L658) | unit/verify | unproven |

### [`RFC9494-3.1-2`](#rfc9494-3.1-2)

The remaining bits are reserved and MUST be set to zero by the sender and ignored by the receiver. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Two clauses, both asserted. Sender: TestRFC9494_FlagsReservedBitsZeroOnSend fails unless the encoded Flags octet is exactly 0x80 (any reserved bit set goes red). Receiver: TestRFC9494_FlagsReservedBitsIgnoredOnReceive fails if a 0xFF Flags octet errors or decodes differently from 0x80, and if 0x7F leaks into the F bit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L91) | unit/verify | unproven |
| positive | [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L71) | unit/verify | unproven |

### [`RFC9494-4-1`](#rfc9494-4-1)

If a BGP speaker is configured to support the procedures of this document, it MUST use BGP Capabilities Advertisement [RFC5492] to advertise the Long-Lived Graceful Restart Capability. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a peer configured for LLGR whose capability declaration lacks code 71. TestExtractLLGRCapabilities_Basic fails unless extractLLGRCapabilities emits one Code 71 hex declaration for the peer whose payload decodes to LLST 3600. Negative: TestExtractLLGRCapabilities_NoLLGR asserts no declaration without long-lived-stale-time. Proven at the plugin's capability-declaration boundary; the hand-off to the OPEN is not asserted in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L610) | unit/verify | unproven |
| positive | [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L583) | unit/verify | unproven |

### [`RFC9494-4.2-1`](#rfc9494-4.2-1)

After the session goes down, and before the session is re- established, the stale routes for an AFI/SAFI MUST be retained. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 Section 4.2: 'After the session goes down, and before the session is re-established, the stale routes for an AFI/SAFI MUST be retained.' Read in its bounded GR-plus-LLST, zero-period and omitted-family context. Re-read all ten tagged units: gr_state_test.go TestLLSTTimerExpiry_LastFamily and TestOnTimerExpired_WithoutLLGR; rfc9494_llgr_entry_test.go TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod, TestRFC9494BothTimesZeroRetainsNothing, TestRFC9494RestartTimeThenLongLivedStaleTime and TestRFC9494DelayedDownPreservesLLSTDeadlines; rfc9494_omitted_gr_family_test.go TestLLGROmittedGRFamilyRetainsRealRIB, TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry, TestLLGROmittedGRFamilyWithoutLLSTIsRemoved and TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry, all under internal/component/bgp/plugins/gr/. The legacy callback-only units are not sufficient alone. The real registered-RIB units assert received-route existence before DOWN, retention at GR level 1 and LLGR level 2, both-zero deletion, and distinct before/at absolute deadlines on JSON and structured paths. Empty/mixed GR lists retain omitted exchanged LLGR families immediately; zero or unexchanged omitted families disappear. Virtual-time checks require presence one nanosecond before expiry and absence at expiry, preserve sibling retention, and prevent delayed DOWN dispatch or blocked readvertisement from extending the bound. This closes the earlier family-omission and timing evidence gaps rather than relying on timer-map presence. Producing symbols: gr.go::handleStateEvent, handleStructuredState, retainPeerFamilies, wireStateCallbacks; gr_state.go::onSessionDownDeferred, startRestartTimer, handleTimerExpired, enterLLGRLocked, handleLLSTExpired. Native panic records establish producer reachability only; semantic discrimination comes from those exact inventory/state/deadline assertions. This verdict covers this row's pre-reestablishment retention obligation, not separately recorded consecutive-restart, resynchronization or egress-community findings. No new check or interop run is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L604) | unit/verify | unproven |
| negative | [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L98) | unit/verify | revert, verified |
| negative | [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L164) | unit/verify | revert, verified |
| positive | [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L706) | unit/verify | unproven |
| positive | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L110) | unit/verify | revert, verified |

### [`RFC9494-4.2-2`](#rfc9494-4.2-2)

* For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long- Lived Stale Time. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 Section 4.2: 'For each AFI/SAFI for which it has received a nonzero Long-Lived Stale Time, the helper router MUST start a timer for that Long-Lived Stale Time.' Its chapeau requires LLGR entry after the family's Restart Time, including zero. Re-read all eight tagged units: gr_state_test.go TestOnTimerExpired_WithLLGR and TestOnSessionDown_ZeroGR_ZeroLLST; rfc9494_llgr_entry_test.go TestRFC9494RestartTimeThenLongLivedStaleTime and TestRFC9494BothTimesZeroRetainsNothing; all four TestLLGROmittedGRFamily tests in rfc9494_omitted_gr_family_test.go, under internal/component/bgp/plugins/gr/. Callback occurrence and eventual deletion alone remain weak evidence, but the added real-RIB virtual-time tests pin received durations: omitted IPv4 LLST 2 expires at DOWN+2 while IPv6 GR 6 transitions at DOWN+6 and LLST 4 expires at DOWN+10; the omitted LLST 8 case survives sibling GR entry and expires at DOWN+8 rather than restarting. Each principal boundary asserts presence immediately before and absence at the boundary. A blocked immediate-family readvertisement still permits sibling GR 1 then LLST 2 to expire at DOWN+3. Both production event paths are exercised. Zero/unexchanged omitted families explicitly have no timer and no retained route; both-zero state is inactive. These assertions distinguish wrong timer duration, delayed start, shared-family deadlines, missing timer and timer replacement. Producing symbols: gr.go::handleStateEvent, handleStructuredState, retainPeerFamilies, wireStateCallbacks; gr_state.go::onSessionDownDeferred, startRestartTimer, handleTimerExpired, enterLLGRLocked, handleLLSTExpired. Native producer-panic records prove reachability, not duration correctness. No new check or interop run is claimed, and neighboring restart/resynchronization findings remain unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L768) | unit/verify | unproven |
| negative | [`TestRFC9494BothTimesZeroRetainsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L99) | unit/verify | revert, verified |
| negative | [`TestLLGROmittedGRFamilyWithoutLLSTIsRemoved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L569) | unit/verify | unproven |
| positive | [`TestRFC9494RestartTimeThenLongLivedStaleTime`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyReadvertisementDoesNotDelaySiblingExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyRetainsRealRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestLLGROmittedGRFamilyTimerSurvivesConventionalEntry`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_omitted_gr_family_test.go#L111) | unit/verify | revert, verified |

### [`RFC9494-4.2-3`](#rfc9494-4.2-3)

If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 Section 4.2: If the timer for the Long-Lived Stale Time for a given AFI/SAFI expires before the session is re-established, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor that it is retaining. Independently reread all current covers: TestLLSTTimerExpiry_SingleFamily observes IPv4 expiry and continued peer retention; TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes requires the production IPv4 purge command and forbids the live IPv6 purge; TestRFC9494DelayedDownPreservesLLSTDeadlines exercises registered real RIB commands on JSON and structured DOWN, with original GR+LLST deadlines, both-live, one-expired, both-expired and zero-GR cases. Its distinct negative assertions require exactly one level-2 route, literal LLGR_STALE and active family retention before each deadline, including one nanosecond before; positives require actual route absence and no family retention at expiry, and no active peer after the last deadline. IPv6 is seeded through injection, not wire decode. Q1-Q4 pass for the pre-reestablishment deletion clause; callback-only evidence is completed by the real RIB consumer. enterLLGRLocked and handleLLSTExpired dispatch through wireStateCallbacks to purgeStaleCommand/PurgeFamilyStale and best-change propagation. Native handleTimerExpired HALTs establish reachability only. No checks ran; current execution/discrimination maintenance and live peer/forwarding acceptance remain pending. This does not rejudge reconnect, consecutive-restart, missing-family activation, other partial/gap rows or destination FIB withdrawal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L662) | unit/verify | unproven |
| positive | [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC9494DelayedDownPreservesLLSTDeadlines`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L174) | unit/verify | revert, verified |

### [`RFC9494-4.2-4`](#rfc9494-4.2-4)

* The helper router MUST attach the LLGR_STALE community to the stale routes being retained. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 §4.2: "The helper router MUST attach the LLGR_STALE community to the stale routes being retained." Read all three tagged carriers: internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go::TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod exercises production wireStateCallbacks on JSON and structured DOWN, real registered RIB commands, and requires retained received route 198.51.100.0/24 at level 2 with community 0xffff0006. internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go::TestAttachCommunity checks the actual stored bundle contains that exact value; internal/component/bgp/plugins/rib/rfc9494_test.go::TestRFC9494_FreshRoutesDoNotGetLLGRStale requires zero attachments, unchanged fresh level and no community for a fresh route. Two positive and one negative tags. gr.go::wireStateCallbacks dispatches deletion of NO_LLGR before attach-community, then raises stale level and triggers per-family readvertisement; rib_commands_community.go::attachCommunityCommand filters fresh entries and attachCommunity updates the bundle and clears the original-wire fingerprint. Removing marker attachment or attaching it to fresh routes breaks different assertions. The RFC note "Note that this requirement implies that the routes would need to be readvertised in order to disseminate the modified community" is traced to wireStateCallbacks::onLLGREntryDone; tagged units establish the attachment obligation at storage, not a destination-wire guarantee. Supplementary untagged test/plugin/llgr-peer-stale-time-drives-timer.ci waits for a destination LLGR_STALE UPDATE before acknowledging the phase. This verdict does not erase RFC9494-4.3-2 further-advertisement protection gap or claim every LLGR activation case is implemented (see 4.2-1/-2). No runtime renewal performed. Post-lint independent source rejudgment: the changed zero-Restart-Time tagged unit still compares the retained route community to literal 0xffff0006 on both event rails after real RIB DOWN; removing testPeer helper arguments leaves bytes and peer identity identical. Re-read TestAttachCommunity and the fresh-route negative as well as wireStateCallbacks: delete NO_LLGR, attach LLGR_STALE, raise level, then readvertise. The distinct fresh-route input must remain unmarked. Enforced attachment judgment is unchanged; cleanup now reports mux.Close failures. Destination-wire dissemination and missing-GR-family activation are not newly proven by this lint repair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) | unit/verify | unproven |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L618) | unit/verify | unproven |

### [`RFC9494-4.2-5`](#rfc9494-4.2-5)

* If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9494 §4.2: "If any of the routes from the peer have been marked with the NO_LLGR community, either as sent by the peer or as the result of a configured policy, they MUST NOT be retained and MUST be removed as per the normal operation of [RFC4271]." Read all seven tagged carriers (four positive, three negative): internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go::TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod deletes a real received NO_LLGR route while its otherwise eligible sibling survives on both DOWN rails; internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go::TestDeleteWithCommunity checks exact storage deletion; internal/component/bgp/plugins/rib/rfc9494_test.go::TestRFC9494_StaleRouteWithoutNoLLGRRetained retains an LLGR_STALE-only route. internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go::TestRFC9494NoLLGRSweepWithdrawsTheBestPath additionally requires exactly one best-change batch, IPv4/unicast, exactly one Withdraw for 10.0.0.0/24; TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR uses a nonmatching real community 65001:100 and requires unchanged route count and no event. Both tags in test/plugin/llgr-import-no-llgr.ci are distinct route outcomes: wire UPDATEs initially lack communities, only marked receives import modify:NO-LLGR; internal/test/fixture/plugin_fixture_llgr_import.go::llgrImportNoLLGR checks policy-added 65535:7 on 10.0.0.0/24 and its absence from 10.0.1.0/24 before releasing TCP closes, then requires marked absent and plain retained with 65535:6. The stderr success fence is emitted only after these checks. The previous configured-policy evidence hole is therefore closed in current source. Producer chain: filter_modify config.go::parseCommOps and modify.go::buildDynamicDelta, GR wireStateCallbacks, then rib_commands_community.go::deleteWithCommunityCommand removes matching stale routes, reconciles sent-source state and recomputes/publishes best-path changes. Peer-added and policy-added clauses plus normal best-path removal are covered compositionally; the import fixture itself reads received RIB, not a downstream socket, and no destination-wire observation or runtime green is claimed here. Distinct matching/nonmatching community inputs isolate the rule. Existing unrelated LLGR gaps remain. Native red records over changed unit/producer require parent renewal before stamping. Post-lint independent source rejudgment: the same changed zero-RT unit supplies a received 0xffff0007 route and an otherwise eligible sibling, requires the marked route absent and the sibling retained with 0xffff0006 after actual DOWN on both rails. Re-read all other carriers, including the .ci and its llgrImportNoLLGR observer: policy attachment is checked before a wire fence releases TCP closes; marked absence and unmarked LLGR_STALE precede the success fence. The RIB best-change pair requires one exact withdrawal versus no event for nonmatching community. Constant-peer removal and checked cleanup preserve these assertions. Preserve enforced and every prior planned producer-change renewal, including functional citations; no new functional run is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494SweepKeepsTheBestPathWithoutNoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L99) | unit/verify | revert, verified |
| negative | [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63) | unit/verify | unproven |
| negative | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L7) | functional/verify | revert, verified |
| positive | [`TestRFC9494ZeroRestartTimeRetainsThroughTheLLGRPeriod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_llgr_entry_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC9494NoLLGRSweepWithdrawsTheBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_no_llgr_withdraw_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_rib_gr_test.go#L659) | unit/verify | unproven |
| positive | [`llgr-import-no-llgr.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-import-no-llgr.ci#L6) | functional/verify | revert, verified |

### [`RFC9494-4.2-6`](#rfc9494-4.2-6)

* The helper router MUST perform the procedures listed in Section 4.3. (§4.2, §4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The row obliges every Section 4.3 procedure. The tagged units prove one: TestRFC9494_LLGRStaleRouteIsLeastPreferred fails unless attach-community raises StaleLevel to the depreference threshold and the stale LOCAL_PREF 500 candidate loses to a fresh LOCAL_PREF 100; the negative keeps an unmarked route competing normally. The other Section 4.3 procedures (LLGR_STALE on receipt from the peer, NO_LLGR handling, advertisement restrictions) have no assertion in these units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) | unit/verify | unproven |
| positive | [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137) | unit/verify | unproven |

### [`RFC9494-4.2-7`](#rfc9494-4.2-7)

However, in the case of consecutive restarts, the previously marked stale routes MUST NOT be deleted before the timer for the Long-Lived Stale Time expires. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-7, so no unit is bound to it.

### [`RFC9494-4.2-8`](#rfc9494-4.2-8)

Similar to [RFC4724], once the LLGR Period begins, the Helper MUST immediately remove all the stale routes from the peer that it is retaining for that address family if any of the following occur: * the F bit for a specific address family is not set in the newly received LLGR Capability, or * a specific address family is not included in the newly received LLGR Capability, or * the LLGR and accompanying GR Capability are not received in the re-established session at all. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Three listed cases. Only the third (neither GR nor LLGR received) is asserted: onSessionReestablished(nil,nil) returns IPv4Unicast for purge and the peer leaves LLGR. The negative (F=1 in both caps, nothing purged) is the control. Neither the F bit clear for a family in the new LLGR capability nor a family absent from it has an assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L796) | unit/verify | unproven |
| positive | [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L846) | unit/verify | unproven |

### [`RFC9494-4.2-9`](#rfc9494-4.2-9)

If a Long-Lived Stale Time timer is running for routes with a given AFI/SAFI received from a peer, it MUST NOT be updated (other than by manual operator intervention) until the peer has established and synchronized a new session. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-9, so no unit is bound to it.

### [`RFC9494-4.3-1`](#rfc9494-4.3-1)

A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2: * Treat the route as the least preferred in route selection (see below). (§4.3, §4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-10-02 (BGP c25 judge): TestComparePair_LLGRStale's body is unchanged (only the RFC4724-4.2-5 tag lines left its doc comment), so the finding stands. It does not isolate the rule: normal (10.0.0.1) and stale (10.0.0.2) have equal LOCAL_PREF, so the lower peer address wins on the final tie-break with or without the Step-0 stale comparison, and both ComparePair assertions stay green if it is removed (the third assertion in the same unit shows the lower-address tie-break). Neither the receive path (a route arriving with LLGR_STALE gets StaleLevel 2) nor the self-attached path is asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L852) | unit/verify | unproven |
| positive | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L820) | unit/verify | unproven |

### [`RFC9494-4.3-2`](#rfc9494-4.3-2)

The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.3-2, so no unit is bound to it.

### [`RFC9494-4.4-1`](#rfc9494-4.4-1)

A least preferred route MUST be treated as less preferred than any other route that is not also least preferred. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a least-preferred route beating one that is not. TestSelectBest_LLGRStaleDepreference gives the stale candidate LOCAL_PREF 300 against 100 and fails, in both input orders, unless the normal route wins, so removing Step 0 goes red. Negative: TestSelectBest_BothLLGRStale fails unless two stale candidates fall through to LOCAL_PREF, so the rule orders only across the least-preferred boundary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L783) | unit/verify | unproven |
| positive | [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L755) | unit/verify | unproven |

### [`RFC9494-4.5-1`](#rfc9494-4.5-1)

If the LLGR Capability is received without an accompanying GR Capability, the LLGR Capability MUST be ignored, that is, the implementation MUST behave as though no LLGR Capability has been received. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: honouring an LLGR capability that arrives without GR. TestHandleEventOpenLLGR_NoGR sends an OPEN with code 71 and no code 64 and fails if peerLLGRCaps holds the peer (assert.False llgrOK). Negative: the same OPEN with code 64 keeps the decoded LLGR capability (Families, F bit, LLST 3600 asserted), so the removal is conditional on GR being absent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L288) | unit/verify | unproven |
| positive | [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_event_test.go#L327) | unit/verify | unproven |

### [`RFC9494-4.6-1`](#rfc9494-4.6-1)

The neighbors MUST be internal (Internal BGP (IBGP) or Confederation) neighbors. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: applying the partial-deployment delivery to a non-internal neighbor. TestLLGREgressFilter_EBGPNonLLGR (PeerAS!=LocalAS, no LLGR) fails unless mods.IsWithdraw(). Positive: the internal neighbor (PeerAS==LocalAS) keeps the announce with NO_EXPORT 0xFFFFFF01 added and LOCAL_PREF set to 0, each value asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L400) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L430) | unit/verify | unproven |

### [`RFC9494-4.6-2`](#rfc9494-4.6-2)

* The NO_EXPORT community [RFC1997] MUST be attached to the stale routes. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a stale route delivered on the partial-deployment branch without NO_EXPORT. TestLLGREgressFilter_IBGPPartial and TestLLGREgressFilter_NilStateDepreferencesIBGP fail unless a COMMUNITIES AttrModAdd with value 0xFFFFFF01 is emitted for an internal non-LLGR neighbor (hasCommunityAdd asserted). Negative: TestLLGREgressFilter_LLGRPeer fails if any modification is emitted for an LLGR-capable neighbor.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L370) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L434) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L183) | unit/verify | unproven |

### [`RFC9494-4.6-3`](#rfc9494-4.6-3)

* The stale routes MUST have their LOCAL_PREF set to zero. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a stale route delivered on the partial-deployment branch with its LOCAL_PREF kept. TestLLGREgressFilter_IBGPPartial and TestLLGREgressFilter_NilStateDepreferencesIBGP fail unless a LOCAL_PREF AttrModSet with value 0 is emitted (hasLocalPrefSet asserted). Negative: TestLLGREgressFilter_LLGRPeer fails if an LLGR-capable neighbor receives any modification.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L374) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L437) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L187) | unit/verify | unproven |

### [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1)

In addition to including the path attributes derived from the ATTR_SET attribute in the advertised route as per [RFC6368], the PE router MUST also include the LLGR_STALE community if it is present in the path attributes of the imported route, even if it is not present in the ATTR_SET attribute. (§4.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.7.2-1, so no unit is bound to it.

### [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2)

In addition to including in the VPN route the ATTR_SET derived from the path attributes as per [RFC6368], the PE router MUST also include the LLGR_STALE community in the VPN route if it is present in the path attributes of the route as received from the CE. (§4.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.7.2-2, so no unit is bound to it.

### [`RFC9494-5-1`](#rfc9494-5-1)

Implementations MUST NOT enable these procedures by default. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge, c27 test defect fixed). Speaker side unchanged: TestRFC9494_LLGRNotEnabledByDefault fails if GR-only config declares code 71 or the YANG leaf long-lived-stale-time gains a default; TestRFC9494_LLGREnabledByExplicitConfig shows explicit config emits it (parseLLGRCapValue revert records +/-). Helper side: the gr plugin records the families of Ze's own sent code 71 and exchangedLLGRLocked hands session-down/up only received families Ze declared. The shared fixture rfc9494Opens now lists IPv4 AND IPv6 unicast in both GR Capabilities (TLV and JSON 00000001018000020180), so IPv6 reaches the LLGR decision. Negative TestRFC9494HelperProceduresOffWithoutLocalConfig (no code 71, or IPv6 only): no ffff0007/ffff0006 for ipv4. Positive TestRFC9494HelperProceduresOnPerConfiguredFamily (Ze declares IPv4, peer IPv4+IPv6): both LLGR commands for ipv4/unicast, exactly neither delete-with-community ffff0007 nor attach-community ffff0006 for ipv6/unicast, and purge-stale ipv6/unicast. Judge overlay (exchangedLLGRLocked returning every received family) now reds the positive on BOTH event paths with all three new assertions, and all four negative subcases; before the fixture change it left the positive green. Records: exchangedLLGRLocked revert +/-, re-recorded. Residual: the per-AFI/SAFI configuration clause is RFC9494-5-2, still a {gap} (LLGR configured per peer).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494HelperProceduresOffWithoutLocalConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L80) | unit/verify | revert, verified |
| negative | [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC9494HelperProceduresOnPerConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_helper_default_red_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L119) | unit/verify | revert, verified |

### [`RFC9494-5-2`](#rfc9494-5-2)

They MUST require affirmative configuration per AFI/SAFI in order to enable them. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-5-2, so no unit is bound to it.

### [`RFC9494-4.2-10`](#rfc9494-4.2-10)

If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-10, so no unit is bound to it.

### [`RFC9494-4.3-3`](#rfc9494-4.3-3)

The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received. The exception is described in Section 4.6. (§4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 9494 §4.3 states, "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received." It immediately adds, "The exception is described in Section 4.6." Section 4.6 permits the internal-neighbor exception while requiring both NO_EXPORT and LOCAL_PREF zero. Re-read all three current carriers: internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go::TestLLGREgressFilter_NilStateWithdrawsEBGP, the same file's TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer, and test/plugin/llgr-egress-state-unloaded.ci, including its driver internal/test/fixture/plugin_fixture_09_bgp.go::llgrReadvertise09. The RunGRPlugin-to-runGRPlugin change in the first carrier is explanatory prose only; neither its assertions nor its tag claim changed. The unloaded-state positive correctly requires mods.IsWithdraw for an external destination with stale metadata, and the loaded-capable negative correctly requires acceptance, no withdrawal and no modifications. The functional carrier runs GR externally, marks an existing route stale at level 2, triggers clear-rib-out and requires the internal neighbor's second UPDATE to contain NO_EXPORT (FFFFFF01). It does not assert LOCAL_PREF=0 or exercise an external destination.

The current producers were traced through gr_egress.go::LLGREgressFilter/hasLLGR, register.go's Egress and Readvertise registration, reactor_api_forward.go and forward_rs.go's filter loops and withdrawal conversion, and reactor_api_batch.go::decideStaleReadvertise/sendStaleReadvertiseUnit. These consumers explicitly realize SetWithdraw as withdrawal and attribute modifications as modified announcements; the source does not silently discard those decisions. Nevertheless, the requirement's tagged evidence leaves the ordinary loaded-state/non-LLGR-neighbor case untested: every tagged positive uses unloaded state, while the only loaded-state tagged unit gives the destination an LLGR capability. A change from hasLLGR = s.hasLLGR(dest.Address.String()) to hasLLGR = true whenever s != nil would advertise stale routes to ordinary non-LLGR external neighbors and leave all three tagged carriers' assertions satisfied [INFERENCE from source; no mutation executed]. Existing untagged-for-this-row tests, including TestLLGREgressFilter_StaleEBGPWithdrawsRegardlessOfRestartState and TestLLGREgressFilter_EBGPWithdraw, exercise that missing condition, but they are not fingerprinted evidence for this row. The functional carrier also checks only NO_EXPORT, not both §4.6 obligations, and none of these three carriers tests receipt of an actual LLGR_STALE-marked UPDATE: the units supply stale metadata directly and the functional driver uses mark-stale. Therefore the earlier enforced verdict overstates what this row's tags discriminate; record weak, not a new claim of a patch-introduced production bug. No partial annotation exists, so partial is unavailable. Existing LLGR-only-family, synchronization and other absent-feature scopes remain unchanged, with no implementation authorized.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L341) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go#L157) | unit/verify | revert, verified |
| positive | [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L18) | functional/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9494.txt |
| Source fingerprint | c6e4d6c4836c4b4e |
| Record | rfc/extraction/rfc9494.json |
| Mapped sentences | 25 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 2 | walked | not stated |
| `4.2` | not stated | 12 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `4.5` | not stated | 1 | walked | not stated |
| `4.6` | not stated | 3 | walked | not stated |
| `4.7` | not stated | 0 | walked | not stated |
| `4.7.1` | not stated | 0 | walked | not stated |
| `4.7.2` | not stated | 2 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4.1 restates the Section 3.1 obligation verbatim; site 3.1:1 carries it and cites Section 4.1 itself ("see Section 4.1"). | If the LLGR Capability is advertised, the Graceful Restart capability MUST also be advertised. |
| `4.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A quotation of superseded RFC 4724 text, introduced by "The following text in Section 4.2 of [RFC4724] no longer applies" and followed by "and the following procedures are specified instead". The block is set off with change bars and states no obligation of this document. | \| If the session does not get re-established within the "Restart \| Time" that the peer advertised previously, the Receiving Speaker \| MUST delete all the stale routes from the peer that it is \| retaining. |
| `4.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The chapeau introducing the LLGR-period bullet list. Each enumerated procedure carries its own MUST and is mapped separately (sites 4.2:4 through 4.2:8); the chapeau's own content, that the procedures begin once the Restart Time period ends, is carried in RFC9494-4.2-2. | Once the Restart Time period ends (including the case in which the Restart Time is zero), the LLGR period is said to have begun and the following procedures MUST be performed: |

## Superseded

No document obsoletes RFC 9494, so its obligations are stated where they were written.
