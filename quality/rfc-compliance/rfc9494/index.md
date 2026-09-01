# RFC 9494 - Long-Lived Graceful Restart for BGP

Partial. Every requirement this repository extracted from RFC 9494, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 78.3% | 18 of 23 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 23 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 42 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 36 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 21.7% | 5 of 23 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 23 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 36 |
| Gated MUST-level | 25 |
| Obligations that bind Ze | 23 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 5 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 42 |
| Tagged units | 42 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (18):** [`RFC9494-3.1-1`](#rfc9494-3.1-1), [`RFC9494-4.1-1`](#rfc9494-4.1-1), [`RFC9494-3.1-2`](#rfc9494-3.1-2), [`RFC9494-4-1`](#rfc9494-4-1), [`RFC9494-4.2-1`](#rfc9494-4.2-1), [`RFC9494-4.2-2`](#rfc9494-4.2-2), [`RFC9494-4.2-3`](#rfc9494-4.2-3), [`RFC9494-4.2-4`](#rfc9494-4.2-4), [`RFC9494-4.2-5`](#rfc9494-4.2-5), [`RFC9494-4.2-6`](#rfc9494-4.2-6), [`RFC9494-4.2-8`](#rfc9494-4.2-8), [`RFC9494-4.3-1`](#rfc9494-4.3-1), [`RFC9494-4.4-1`](#rfc9494-4.4-1), [`RFC9494-4.5-1`](#rfc9494-4.5-1), [`RFC9494-4.6-1`](#rfc9494-4.6-1), [`RFC9494-4.6-2`](#rfc9494-4.6-2), [`RFC9494-4.6-3`](#rfc9494-4.6-3), [`RFC9494-5-1`](#rfc9494-5-1)

**Annotated instead of tested (7):** [`RFC9494-4.2-7`](#rfc9494-4.2-7), [`RFC9494-4.2-9`](#rfc9494-4.2-9), [`RFC9494-4.3-2`](#rfc9494-4.3-2), [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1), [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2), [`RFC9494-5-2`](#rfc9494-5-2), [`RFC9494-4.2-10`](#rfc9494-4.2-10)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9494-3.1-1` | If the LLGR Capability is advertised, the Graceful Restart capability MUST also be advertised (§3.1, §4.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L33). **negative:** `unit/verify` [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L57) |
| `RFC9494-4.1-1` | If GR capability is not advertised alongside LLGR, the LLGR Capability MUST be disregarded (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L658). **negative:** `unit/verify` [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L745) |
| `RFC9494-3.1-2` | Reserved bits in the Flags field MUST be set to zero by the sender and MUST be ignored by the receiver (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L75). **negative:** `unit/verify` [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L95) |
| `RFC9494-4-1` | If configured to support LLGR procedures, a BGP speaker MUST use BGP Capabilities Advertisement to advertise the LLGR Capability (§4) | MUST | 4 | **positive:** `unit/verify` [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L582). **negative:** `unit/verify` [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L609) |
| `RFC9494-4.2-1` | After session goes down, stale routes for an AFI/SAFI MUST be retained for the sum of Restart Time and Long-Lived Stale Time (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L592). **negative:** `unit/verify` [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L490) |
| `RFC9494-4.2-2` | Once the LLGR period begins, for each AFI/SAFI with nonzero LLST, the helper router MUST start a timer for that Long-Lived Stale Time (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L455). **negative:** `unit/verify` [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L654) |
| `RFC9494-4.2-3` | If the LLST timer expires before session re-establishment, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L548). **positive:** `unit/verify` [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L163). **negative:** `unit/verify` [`TestRFC9494_NoLLSTExpiryAfterReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L172) |
| `RFC9494-4.2-4` | The helper router MUST attach the LLGR_STALE community to stale routes being retained (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_gr_test.go#L618). **negative:** `unit/verify` [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) |
| `RFC9494-4.2-5` | Routes marked with NO_LLGR community MUST NOT be retained and MUST be removed per normal RFC 4271 operation (§4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_gr_test.go#L659). **negative:** `unit/verify` [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63) |
| `RFC9494-4.2-6` | The helper router MUST perform the LLGR_STALE route processing procedures (§4.2, §4.3) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137). **negative:** `unit/verify` [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) |
| `RFC9494-4.2-7` | In case of consecutive restarts, previously marked stale routes MUST NOT be deleted before the LLST timer expires (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| `RFC9494-4.2-8` | Once LLGR period begins, helper MUST immediately remove all stale routes if F bit is not set, AFI/SAFI is not listed, or LLGR+GR capabilities are not received in re-established session (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L732). **negative:** `unit/verify` [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L682) |
| `RFC9494-4.2-9` | A running LLST timer MUST NOT be updated (other than by manual intervention) until the peer has established and synchronized a new session (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| `RFC9494-4.3-1` | A BGP speaker that advertised LLGR Capability MUST treat LLGR_STALE routes as least preferred in route selection (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L812). **negative:** `unit/verify` [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L844) |
| `RFC9494-4.3-2` | The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| `RFC9494-4.4-1` | A least preferred route MUST be treated as less preferred than any route that is not also least preferred (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L743). **negative:** `unit/verify` [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L771) |
| `RFC9494-4.5-1` | If LLGR Capability is received without accompanying GR Capability, the LLGR Capability MUST be ignored (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L327). **negative:** `unit/verify` [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L288) |
| `RFC9494-4.6-1` | For partial deployment, neighbors receiving stale routes MUST be internal (IBGP or Confederation) neighbors (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L430). **negative:** `unit/verify` [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L400) |
| `RFC9494-4.6-2` | For partial deployment, the NO_EXPORT community MUST be attached to the stale routes (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L434). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L183). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L370) |
| `RFC9494-4.6-3` | For partial deployment, stale routes MUST have their LOCAL_PREF set to zero (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L437). **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L187). **negative:** `unit/verify` [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L374) |
| `RFC9494-4.7.2-1` | When using ATTR_SET (IBGP PE-CE), PE MUST include LLGR_STALE community in CE advertisement if present in imported VPN route, even if not in ATTR_SET (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no ATTR_SET attribute and no RFC 6368 iBGP PE-CE model -- the path attribute code table stops at code 40 plus the provisional 252 and contains no code 128 (internal/core/bgp/attribute/attribute.go:46-66), `grep -rni "attr_set\\\|attrset" internal/core/` returns nothing, and `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, so there is no PE that imports a VPN route into a CE session |
| `RFC9494-4.7.2-2` | When exporting CE route to VPN address family, PE MUST include LLGR_STALE community in VPN route if present in path attributes received from CE (§4.7.2) | MUST | 4.7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no PE export of a CE route into a VPN address family -- `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, and the only route-target handling is parsing of the extended community and the RTC NLRI codec (internal/component/bgp/route/route_community.go:275, internal/component/bgp/plugins/nlri/rtc/), with no VRF import/export engine that could carry a CE route's LLGR_STALE into a VPN route |
| `RFC9494-5-1` | Implementations MUST NOT enable LLGR procedures by default (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L123). **negative:** `unit/verify` [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L148) |
| `RFC9494-5-2` | Implementations MUST require affirmative configuration per AFI/SAFI to enable LLGR (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| `RFC9494-4.2-10` | If LLST timer expires during synchronization and session subsequently resets, remaining routes for that AFI/SAFI MUST be removed immediately (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |
| `RFC9494-4.2-11` | The LLST timers received SHOULD be modifiable by local configuration (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.3-3` | LLGR_STALE routes SHOULD NOT be advertised to any neighbor from which the LLGR Capability has not been received (§4.3) | SHOULD NOT | 4.3 | **positive:** `unit/verify` [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L157). **negative:** `unit/verify` [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L341). **positive:** `functional/verify` [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L18) |
| `RFC9494-4.7.1-1` | When advertising stale routes over PE-CE EBGP session, implementation SHOULD by default attach NO_EXPORT community (§4.7.1) | SHOULD | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.2-1` | An implementation MAY allow users to configure policies for LLGR_STALE community (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-3.3-1` | An implementation MAY allow users to configure policies for NO_LLGR community (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.2-12` | The value of LLST received from a neighbor MAY be reduced by local configuration (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.6-4` | For partial deployment, stale routes MAY be advertised to IBGP neighbors without LLGR Capability (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-2` | An implementation MAY advertise stale routes over a PE-CE session when explicitly configured (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-3` | The second rule of Section 4.3 MAY be disregarded for PE-CE VPN sessions (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.1-4` | Attachment of NO_EXPORT community MAY be disabled by explicit configuration for PE-CE (§4.7.1) | MAY | 4.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9494-4.7.2-3` | For IBGP PE-CE when CE does not support LLGR, optional procedures of Section 4.6 MAY be followed, overriding LOCAL_PREF from ATTR_SET (§4.7.2) | MAY | 4.7.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9494-4.2-7`](#rfc9494-4.2-7) In case of consecutive restarts, previously marked stale routes MUST NOT be deleted before the LLST timer expires (§4.2) | {gap}, no test | ze applies the RFC 4724 Section 4.2 consecutive-restart rule unconditionally, including during an LLGR period -- every activation of a session-down dispatches "request bgp rib purge-stale <peer>" as its first step (internal/component/bgp/plugins/gr/gr.go:362, internal/component/bgp/plugins/gr/gr.go:532), and onSessionDown clears the prior peer state through clearPeerLocked (internal/component/bgp/plugins/gr/gr_state.go:125), so routes marked stale in the previous cycle are deleted at the new session drop rather than kept until their LLST timer expires |
| [`RFC9494-4.2-9`](#rfc9494-4.2-9) A running LLST timer MUST NOT be updated (other than by manual intervention) until the peer has established and synchronized a new session (§4.2) | {gap}, no test | a consecutive session drop while the peer is already in LLGR replaces the running timer -- onSessionDown calls clearPeerLocked, which stops every LLST timer via stopLLSTTimersLocked (internal/component/bgp/plugins/gr/gr_state.go:125, :457-465, :476-481), and enterLLGRLocked then arms a fresh time.AfterFunc for the family's full LLST (internal/component/bgp/plugins/gr/gr_state.go:384-387), so the remaining Long-Lived Stale Time is reset before the peer has established and synchronized a new session |
| [`RFC9494-4.3-2`](#rfc9494-4.3-2) The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3) | {gap}, no test | two egress paths strip the whole COMMUNITIES attribute with no LLGR_STALE (0xFFFF0006) exemption. applyFactsSendCommunity emits a whole-attribute suppress for code 8 whenever the peer's `send-community` is `none` or omits `standard` (internal/component/bgp/reactor/peer_forward_facts.go:249, mask set at :195-223), and it runs on both readvertise rails (internal/component/bgp/reactor/reactor_api_forward.go:516, internal/component/bgp/reactor/forward_rs.go:343). A `community-remove ffff0006` filter reaches the same attribute through AttrModRemove (internal/component/bgp/reactor/filter_delta.go:270), and removeValues drops every matching 4-octet value without checking which community it is (internal/component/bgp/plugins/filter_community/handler.go:120-131). The RIB-side attach path only appends (internal/component/bgp/plugins/rib/rib_commands_community.go:222-236), but that is not the path on which the community is lost |
| [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1) When using ATTR_SET (IBGP PE-CE), PE MUST include LLGR_STALE community in CE advertisement if present in imported VPN route, even if not in ATTR_SET (§4.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no ATTR_SET attribute and no RFC 6368 iBGP PE-CE model -- the path attribute code table stops at code 40 plus the provisional 252 and contains no code 128 (internal/core/bgp/attribute/attribute.go:46-66), `grep -rni "attr_set\\\|attrset" internal/core/` returns nothing, and `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, so there is no PE that imports a VPN route into a CE session |
| [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2) When exporting CE route to VPN address family, PE MUST include LLGR_STALE community in VPN route if present in path attributes received from CE (§4.7.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no PE export of a CE route into a VPN address family -- `grep -rln "vrf" --include=*.go internal/component/bgp/` returns no file, and the only route-target handling is parsing of the extended community and the RTC NLRI codec (internal/component/bgp/route/route_community.go:275, internal/component/bgp/plugins/nlri/rtc/), with no VRF import/export engine that could carry a CE route's LLGR_STALE into a VPN route |
| [`RFC9494-5-2`](#rfc9494-5-2) Implementations MUST require affirmative configuration per AFI/SAFI to enable LLGR (§5) | {gap}, no test | ze requires affirmative configuration, but its granularity is the peer, not the AFI/SAFI -- parseLLGRCapValue reads a single long-lived-stale-time from the peer's (or group's) graceful-restart container and stamps that same LLST onto every negotiated family (internal/component/bgp/plugins/gr/gr_llgr.go:130-171), and the YANG leaf sits in the per-peer capability container with the description "Applied to all negotiated address families for the peer" (internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang), so an operator cannot enable LLGR for one address family and leave another off |
| [`RFC9494-4.2-10`](#rfc9494-4.2-10) If LLST timer expires during synchronization and session subsequently resets, remaining routes for that AFI/SAFI MUST be removed immediately (§4.2) | {gap}, no test | ze holds no record of an LLST that elapsed during synchronization, because no LLST timer runs then -- onSessionReestablished stops every LLST timer and clears state.inLLGR the moment the session comes back (internal/component/bgp/plugins/gr/gr_state.go:194, :231, :476-481), so the expiry this requirement keys on cannot be observed and the next session reset starts an ordinary GR cycle with a full restart timer (internal/component/bgp/plugins/gr/gr_state.go:153-156) instead of removing the family's remaining routes immediately |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9494-3.1-1`](#rfc9494-3.1-1)

If the LLGR Capability is advertised, the Graceful Restart capability MUST also be advertised (§3.1, §4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_NoLLGRCapWithoutGRContainer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L57) | unit/verify | unproven |
| positive | [`TestRFC9494_LLGRCapDeclaredWithGRCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L33) | unit/verify | unproven |

### [`RFC9494-4.1-1`](#rfc9494-4.1-1)

If GR capability is not advertised alongside LLGR, the LLGR Capability MUST be disregarded (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleStructuredOpenGRPlusLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L745) | unit/verify | unproven |
| positive | [`TestHandleStructuredOpenLLGRNoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L658) | unit/verify | unproven |

### [`RFC9494-3.1-2`](#rfc9494-3.1-2)

Reserved bits in the Flags field MUST be set to zero by the sender and MUST be ignored by the receiver (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_FlagsReservedBitsIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC9494_FlagsReservedBitsZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L75) | unit/verify | unproven |

### [`RFC9494-4-1`](#rfc9494-4-1)

If configured to support LLGR procedures, a BGP speaker MUST use BGP Capabilities Advertisement to advertise the LLGR Capability (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractLLGRCapabilities_NoLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L609) | unit/verify | unproven |
| positive | [`TestExtractLLGRCapabilities_Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_test.go#L582) | unit/verify | unproven |

### [`RFC9494-4.2-1`](#rfc9494-4.2-1)

After session goes down, stale routes for an AFI/SAFI MUST be retained for the sum of Restart Time and Long-Lived Stale Time (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnTimerExpired_WithoutLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L490) | unit/verify | unproven |
| positive | [`TestLLSTTimerExpiry_LastFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L592) | unit/verify | unproven |

### [`RFC9494-4.2-2`](#rfc9494-4.2-2)

Once the LLGR period begins, for each AFI/SAFI with nonzero LLST, the helper router MUST start a timer for that Long-Lived Stale Time (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnSessionDown_ZeroGR_ZeroLLST`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L654) | unit/verify | unproven |
| positive | [`TestOnTimerExpired_WithLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L455) | unit/verify | unproven |

### [`RFC9494-4.2-3`](#rfc9494-4.2-3)

If the LLST timer expires before session re-establishment, the helper MUST delete all stale routes of that AFI/SAFI from the neighbor (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_NoLLSTExpiryAfterReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L172) | unit/verify | unproven |
| positive | [`TestLLSTTimerExpiry_SingleFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L548) | unit/verify | unproven |
| positive | [`TestRFC9494LLSTExpiryDeletesTheFamilysStaleRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc4724_retention_test.go#L163) | unit/verify | unproven |

### [`RFC9494-4.2-4`](#rfc9494-4.2-4)

The helper router MUST attach the LLGR_STALE community to stale routes being retained (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_FreshRoutesDoNotGetLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L34) | unit/verify | unproven |
| positive | [`TestAttachCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_gr_test.go#L618) | unit/verify | unproven |

### [`RFC9494-4.2-5`](#rfc9494-4.2-5)

Routes marked with NO_LLGR community MUST NOT be retained and MUST be removed per normal RFC 4271 operation (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_StaleRouteWithoutNoLLGRRetained`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L63) | unit/verify | unproven |
| positive | [`TestDeleteWithCommunity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_gr_test.go#L659) | unit/verify | unproven |

### [`RFC9494-4.2-6`](#rfc9494-4.2-6)

The helper router MUST perform the LLGR_STALE route processing procedures (§4.2, §4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_RouteWithoutLLGRStaleNotLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L175) | unit/verify | unproven |
| positive | [`TestRFC9494_LLGRStaleRouteIsLeastPreferred`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9494_test.go#L137) | unit/verify | unproven |

### [`RFC9494-4.2-7`](#rfc9494-4.2-7)

In case of consecutive restarts, previously marked stale routes MUST NOT be deleted before the LLST timer expires (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-7, so no unit is bound to it.

### [`RFC9494-4.2-8`](#rfc9494-4.2-8)

Once LLGR period begins, helper MUST immediately remove all stale routes if F bit is not set, AFI/SAFI is not listed, or LLGR+GR capabilities are not received in re-established session (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOnSessionReestablished_DuringLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L682) | unit/verify | unproven |
| positive | [`TestOnSessionReestablished_DuringLLGR_NoCaps`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_state_test.go#L732) | unit/verify | unproven |

### [`RFC9494-4.2-9`](#rfc9494-4.2-9)

A running LLST timer MUST NOT be updated (other than by manual intervention) until the peer has established and synchronized a new session (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-9, so no unit is bound to it.

### [`RFC9494-4.3-1`](#rfc9494-4.3-1)

A BGP speaker that advertised LLGR Capability MUST treat LLGR_STALE routes as least preferred in route selection (§4.3, §4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestComparePair_GRStaleCompetesNormally`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L844) | unit/verify | unproven |
| positive | [`TestComparePair_LLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L812) | unit/verify | unproven |

### [`RFC9494-4.3-2`](#rfc9494-4.3-2)

The LLGR_STALE community MUST NOT be removed when the route is further advertised (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.3-2, so no unit is bound to it.

### [`RFC9494-4.4-1`](#rfc9494-4.4-1)

A least preferred route MUST be treated as less preferred than any route that is not also least preferred (§4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectBest_BothLLGRStale`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L771) | unit/verify | unproven |
| positive | [`TestSelectBest_LLGRStaleDepreference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L743) | unit/verify | unproven |

### [`RFC9494-4.5-1`](#rfc9494-4.5-1)

If LLGR Capability is received without accompanying GR Capability, the LLGR Capability MUST be ignored (§4.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandleEventOpenLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L288) | unit/verify | unproven |
| positive | [`TestHandleEventOpenLLGR_NoGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_event_test.go#L327) | unit/verify | unproven |

### [`RFC9494-4.6-1`](#rfc9494-4.6-1)

For partial deployment, neighbors receiving stale routes MUST be internal (IBGP or Confederation) neighbors (§4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_EBGPNonLLGR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L400) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L430) | unit/verify | unproven |

### [`RFC9494-4.6-2`](#rfc9494-4.6-2)

For partial deployment, the NO_EXPORT community MUST be attached to the stale routes (§4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L370) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L434) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L183) | unit/verify | unproven |

### [`RFC9494-4.6-3`](#rfc9494-4.6-3)

For partial deployment, stale routes MUST have their LOCAL_PREF set to zero (§4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_LLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L374) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_IBGPPartial`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L437) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateDepreferencesIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L187) | unit/verify | unproven |

### [`RFC9494-4.7.2-1`](#rfc9494-4.7.2-1)

When using ATTR_SET (IBGP PE-CE), PE MUST include LLGR_STALE community in CE advertisement if present in imported VPN route, even if not in ATTR_SET (§4.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.7.2-1, so no unit is bound to it.

### [`RFC9494-4.7.2-2`](#rfc9494-4.7.2-2)

When exporting CE route to VPN address family, PE MUST include LLGR_STALE community in VPN route if present in path attributes received from CE (§4.7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.7.2-2, so no unit is bound to it.

### [`RFC9494-5-1`](#rfc9494-5-1)

Implementations MUST NOT enable LLGR procedures by default (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9494_LLGREnabledByExplicitConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC9494_LLGRNotEnabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/rfc9494_test.go#L123) | unit/verify | unproven |

### [`RFC9494-5-2`](#rfc9494-5-2)

Implementations MUST require affirmative configuration per AFI/SAFI to enable LLGR (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-5-2, so no unit is bound to it.

### [`RFC9494-4.2-10`](#rfc9494-4.2-10)

If LLST timer expires during synchronization and session subsequently resets, remaining routes for that AFI/SAFI MUST be removed immediately (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9494-4.2-10, so no unit is bound to it.

### [`RFC9494-4.3-3`](#rfc9494-4.3-3)

LLGR_STALE routes SHOULD NOT be advertised to any neighbor from which the LLGR Capability has not been received (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLLGREgressFilter_StateLoadedStillAdvertisesToLLGRPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L341) | unit/verify | unproven |
| positive | [`TestLLGREgressFilter_NilStateWithdrawsEBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/gr_egress_test.go#L157) | unit/verify | unproven |
| positive | [`llgr-egress-state-unloaded.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/llgr-egress-state-unloaded.ci#L18) | functional/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9494, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9494, so its obligations are stated where they were written.
