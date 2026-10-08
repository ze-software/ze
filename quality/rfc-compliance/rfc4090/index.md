# RFC 4090 - Fast Reroute Extensions to RSVP-TE for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 4090, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 96.9% | 31 of 32 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 32 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 32 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 32 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 90.9% | 80 of 88 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 32 | of 40 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 32 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 32 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 32 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 32 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3.1% | 1 of 32 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 37 | of 32 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 32 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 40 |
| Gated MUST-level | 32 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 88 |
| Tagged units | 88 |
| Recorded audit verdicts | 37 |
| Discrimination records | 80 |
| Summary | `rfc/short/rfc4090.md` |
| Requirement shard | `rfc/requirements/rfc4090.md` |
| RFC text | `rfc/full/rfc4090.txt` |

## Enrolment

Enrolled: Fast Reroute Extensions to RSVP-TE for LSP Tunnels. The checklist was rewritten against the document's own text on 2026-09-21: the facility-backup path Ze implements (FAST_REROUTE/SESSION_ATTRIBUTE flags, RRO protection flags, PLR local repair, label-stacking bypass, merge-point selection, and the Section 4.2 rejection of a PATH carrying a DETOUR object) is tagged, and the Section 5, 6, 6.1 to 6.4 and 7 MUSTs the earlier row set omitted are now listed and untested.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Facility backup behavior. A PATH carrying a DETOUR object is rejected with a PathErr, as Section 4.2 requires of an LSR without one-to-one backup.

**What the ledger says remains**

PLRs now signal and refresh the protected PATH through the bypass, and an MP merges it into matching protected state (Sections 6.4.3, 6.4.4, 7.1.1). A head-end PLR uses a distinct assigned local sender address and a two-label ingress push (Section 6.1.1). Each of those six requirements carries a discrimination record in both polarities ([`rfc/discrimination/rfc4090.json`](https://github.com/ze-software/ze/blob/main/rfc/discrimination/rfc4090.json), completed 2026-10-08). One-to-one detours remain absent; Section 6 states "A PLR MAY support the DETOUR object". A bandwidth-guaranteed bypass is also absent ([`plan/spec-rsvpte-bypass-bandwidth-protection.md`](https://github.com/ze-software/ze/blob/main/plan/spec-rsvpte-bypass-bandwidth-protection.md)). Those optional feature records require a scope decision and do not authorize implementation.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 31 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **32** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (31):** [`RFC4090-4.1-1`](#rfc4090-4.1-1), [`RFC4090-4.3-1`](#rfc4090-4.3-1), [`RFC4090-4.4-1`](#rfc4090-4.4-1), [`RFC4090-4.4-2`](#rfc4090-4.4-2), [`RFC4090-4.4-3`](#rfc4090-4.4-3), [`RFC4090-3.2-1`](#rfc4090-3.2-1), [`RFC4090-4.2-1`](#rfc4090-4.2-1), [`RFC4090-4.1-3`](#rfc4090-4.1-3), [`RFC4090-4.4-6`](#rfc4090-4.4-6), [`RFC4090-4.4-7`](#rfc4090-4.4-7), [`RFC4090-5-1`](#rfc4090-5-1), [`RFC4090-5-2`](#rfc4090-5-2), [`RFC4090-5-3`](#rfc4090-5-3), [`RFC4090-5-4`](#rfc4090-5-4), [`RFC4090-6-1`](#rfc4090-6-1), [`RFC4090-6-2`](#rfc4090-6-2), [`RFC4090-6-3`](#rfc4090-6-3), [`RFC4090-6-4`](#rfc4090-6-4), [`RFC4090-6-5`](#rfc4090-6-5), [`RFC4090-6-6`](#rfc4090-6-6), [`RFC4090-6-7`](#rfc4090-6-7), [`RFC4090-6.1-1`](#rfc4090-6.1-1), [`RFC4090-6.2-1`](#rfc4090-6.2-1), [`RFC4090-6.4-1`](#rfc4090-6.4-1), [`RFC4090-6.4-2`](#rfc4090-6.4-2), [`RFC4090-6.4.3-1`](#rfc4090-6.4.3-1), [`RFC4090-6.4-3`](#rfc4090-6.4-3), [`RFC4090-7.1-1`](#rfc4090-7.1-1), [`RFC4090-7.1-2`](#rfc4090-7.1-2), [`RFC4090-7.2-1`](#rfc4090-7.2-1), [`RFC4090-7.2-2`](#rfc4090-7.2-2)

**Annotated (including scoped evidence) (1):** [`RFC4090-4.4-5`](#rfc4090-4.4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4090-4.1-1` | Class-Num = 205 C-Type = 1 (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestEncodeDecodeFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L35). **positive:** `unit/verify` [`TestRFC4090FastRerouteClassAndCTypeLiterals`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L73). **negative:** `unit/verify` [`TestFastRerouteShortBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L49). **negative:** `unit/verify` [`TestRFC4090FastRerouteOtherCTypeNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L90) |
| `RFC4090-4.3-1` | To indicate that an LSP should be locally protected, the head-end LSR MUST either set the "local protection desired" flag in the SESSION_ATTRIBUTE object or include a FAST_REROUTE object in the PATH message, or both. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168). **negative:** `unit/verify` [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L183) |
| `RFC4090-4.3-2` | If node protection is desired, the head-end LSR should set the "node protection desired" flag in the SESSION_ATTRIBUTE object; otherwise, this flag should be cleared. (§5) | SHOULD | 5 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L170). **positive:** `unit/verify` [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63). **negative:** `unit/verify` [`TestRFC4090NodeDesiredClearedWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L47). **negative:** `unit/verify` [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) |
| `RFC4090-4.1-2` | If the head-end LSR desires that the one-to-one backup method be used for the protected LSP, then the head-end LSR should include a FAST_REROUTE object and set the "one-to-one backup desired" flag. If the head-end LSR desires that the protected LSP be protected via the facility backup method, then the head-end LSR should include a FAST_REROUTE object and set the "facility backup desired" flag. (§5) | SHOULD | 5 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L163). **positive:** `unit/verify` [`TestRFC4090OneToOneRequestInPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L28). **negative:** `unit/verify` [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L847). **negative:** `unit/verify` [`TestRFC4090RequestNeverNamesOtherMethod`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L35) |
| `RFC4090-4.4-1` | Whenever the PLR has a backup path available, the PLR MUST set the "local protection available" flag. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L332). **negative:** `unit/verify` [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L350) |
| `RFC4090-4.4-2` | During fast reroute, for each protected LSP containing an RRO object, the PLR obtains the RRO from the protected LSP's stored RESV. The PLR MUST update the IPv4 or IPv6 sub-object it inserted into the RRO by setting the "Local protection in use" and "Local Protection Available" flags. (§6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestRFC4090RepairSetsInUseAndAvailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L68). **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L871). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L863) |
| `RFC4090-4.4-3` | the PLR MUST set this flag when the node protection is provided and the "node protection desired" flag was set in the SESSION_ATTRIBUTE object. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC4090NodeBitSetWhenNodeProtectionProvided`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L93). **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L876). **negative:** `unit/verify` [`TestRFC4090NodeBitClearWhenNodeProtectionNotProvided`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L109). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L865) |
| `RFC4090-6.5-1` | To provide this notification, the PLR SHOULD send a Path Error message with error code of "Notify" (Error code = 25) and an error value field of ss00 cccc cccc cccc, where ss=00 and the sub-code = 3 ("Tunnel locally repaired") (see [RSVP-TE]). (§6.5.1) | SHOULD | 6.5.1 | **positive:** `unit/verify` [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L455). **positive:** `unit/verify` [`TestRFC4090LocalRepairNotifyLiterals`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L116). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L476) |
| `RFC4090-3.2-1` | The label will be switched for one which will be understood by R4 to indicate the protected LSP, and the bypass tunnel's label will then be pushed onto the label- stack of the redirected packets. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L426). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L470) |
| `RFC4090-4.2-1` | LSRs that do not support the DETOUR objects MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L899). **negative:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L918) |
| `RFC4090-4.2-2` | This PathErr SHOULD be generated as specified in [RSVP] for unknown objects with a Class-Num of the form "0bbbbbbb". (§4.2) | SHOULD | 4.2 | **positive:** `unit/verify` [`TestRFC4090DetourPathErrAsUnknownClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L628). **negative:** `unit/verify` [`TestRFC4090DetourOtherCTypeStillUnknownClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L648) |
| `RFC4090-6.5-3` | The globally revertive mode SHOULD always be used. (§6.5.2) | SHOULD | 6.5.2 | **positive:** `unit/verify` [`TestRFC4090HeadEndRevertsGlobally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L666). **negative:** `unit/verify` [`TestRFC4090TransitDoesNotReoptimize`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L689) |
| `RFC4090-6.5.2-1` | - Global revertive mode: The head-end LSR of each tunnel is responsible for reoptimizing the TE LSPs that used the failed resource. (§6.5.2) | SHOULD | 6.5.2 | **positive:** `unit/verify` [`TestRFC4090HeadEndRevertsGlobally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L667). **negative:** `unit/verify` [`TestRFC4090TransitDoesNotReoptimize`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L690) |
| `RFC4090-4.4-4` | A PLR MAY support the DETOUR object. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4090-4.1-3` | This object MUST only be inserted into the PATH message by the head-end LER and MUST NOT be changed by downstream LSRs. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4090TransitRelaysFastRerouteUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L81). **negative:** `unit/verify` [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L105) |
| `RFC4090-4.4-5` | the PLR MUST set this flag when the desired bandwidth is guaranteed and the "bandwidth protection desired" flag was set in the SESSION_ATTRIBUTE object. (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a bypass reserves no bandwidth, so the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |
| `RFC4090-4.4-6` | If the requested bandwidth is not guaranteed, the PLR MUST NOT set this flag. (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L214). **negative:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L213). **negative:** `unit/verify` [`TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L127) |
| `RFC4090-4.4-7` | If node protection is not provided, the PLR MUST NOT set this flag. Thus, if a PLR could only set up a link-protection backup path, the "Local protection available" bit will be set, but the "Node protection" bit will be cleared. (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L284). **negative:** `unit/verify` [`TestRFC4090LinkProtectionLeavesNodeBitClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L55) |
| `RFC4090-5-1` | If a head-end LSR signals a FAST_REROUTE object, it MUST be stored for Path refreshes. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndStoresFastRerouteForRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L387). **negative:** `unit/verify` [`TestRFC4090HeadEndRefreshWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L408) |
| `RFC4090-5-2` | The head-end LSR of a protected LSP MUST set the "label recording desired" flag in the SESSION_ATTRIBUTE object. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndSetsLabelRecordingDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L422). **negative:** `unit/verify` [`TestRFC4090UnprotectedPathRequestsNoLabelRecording`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L433) |
| `RFC4090-5-3` | The head-end LSR of a protected LSP MUST support the additional flags defined in Section 4.4 being set or clear in the RRO IPv4 and IPv6 sub-objects. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L489). **negative:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L490) |
| `RFC4090-5-4` | The head-end LSR of a protected LSP MUST support the RRO Label sub-object. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L505). **negative:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L506) |
| `RFC4090-6-1` | Every LSR along a protected LSP (except the egress) MUST follow the PLR behavior described in this document. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L134). **negative:** `unit/verify` [`TestRFC4090EgressDoesNotActAsPLR`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L176) |
| `RFC4090-6-2` | A PLR MUST consider an LSP to have asked for local protection if the "local protection desired" flag is set in the SESSION_ATTRIBUTE object and/or the FAST_REROUTE object is included. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L133). **positive:** `unit/verify` [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L106). **negative:** `unit/verify` [`TestRFC4090NoProtectionRequestArmsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L154) |
| `RFC4090-6-3` | - Until a PLR has a backup path available, the PLR MUST clear the relevant four flags in the corresponding RRO IPv4 or IPv6 sub- object. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L55). **negative:** `unit/verify` [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L47) |
| `RFC4090-6-4` | If no established one-to-one backup LSP or bypass tunnel exists, or if the one-to-one LSP and the bypass tunnel is in "DOWN" state, the PLR MUST clear the "local protection available" flag in its IPv4 (or IPv6) address sub-object of the RRO (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L56). **negative:** `unit/verify` [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L48). **negative:** `unit/verify` [`TestRFC4090RefreshTracksBypassState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L64) |
| `RFC4090-6-5` | - The PLR MUST clear the "local protection in use" flag unless it is actively redirecting traffic into the backup path instead of along the protected LSP. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L317). **negative:** `unit/verify` [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L318) |
| `RFC4090-6-6` | The PLR SHOULD also set the "node protection" flag if the backup path protects against the failure of the immediate downstream node, and, if the path does not, the PLR SHOULD clear the "node protection" flag. This MUST be done if the "node protection desired" flag was set in the SESSION_ATTRIBUTE object. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L285). **negative:** `unit/verify` [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L301) |
| `RFC4090-6-7` | if the path does not, the PLR SHOULD clear the "bandwidth protection" flag. This MUST be done if the "bandwidth protection desired" flag was set in the SESSION_ATTRIBUTE object. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L215). **negative:** `unit/verify` [`TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L128). **negative:** `unit/verify` [`TestRFC4090BandwidthDesiredWithoutBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L235) |
| `RFC4090-6-8` | The PLR SHOULD set the "bandwidth protection" flag if the backup path offers a bandwidth guarantee (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a bypass reserves no bandwidth, so no backup path offers a bandwidth guarantee and the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |
| `RFC4090-6.1-1` | If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel. (S6.1, S6.1.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4090HeadEndUsesDistinctSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L348). **negative:** `unit/verify` [`TestRFC4090HeadEndRequiresAlternateSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L387) |
| `RFC4090-6.2-1` | For bypass tunnels (Section 7), the destination MUST be the address of the MP. (§6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L363). **negative:** `unit/verify` [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L364) |
| `RFC4090-6.4-1` | The RSVP_HOP object MUST contain an IP source address belonging to the PLR. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L144). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L281) |
| `RFC4090-6.4-2` | The PLR MUST generate an EXPLICIT_ROUTE object toward the egress. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L145). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L282) |
| `RFC4090-6.4.3-1` | When the PLR detects a link or/and node failure condition, it has to reroute the data traffic onto the bypass tunnel (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L341). **negative:** `unit/verify` [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L342) |
| `RFC4090-6.4-3` | More specifically, the PLR MUST: - remove all the sub-objects proceeding the first address belonging to the MP, and - replace this first MP address with an IP address of the MP. (§6.4.4) | MUST | 6.4.4 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L146). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L283) |
| `RFC4090-7.1-1` | If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP. (§7.1.1) | MUST | 7.1.1 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L147). **negative:** `unit/verify` [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L298) |
| `RFC4090-7.1-2` | Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. (§7.1.1) | MUST | 7.1.1 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L148). **negative:** `unit/verify` [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L299) |
| `RFC4090-7.2-1` | When a downstream LSR detects a local link failure, for any protected LSPs routed over the failed link, Path and Resv state MUST NOT be cleared, and PathTear and ResvErr messages MUST NOT be sent immediately. (S7.2) | MUST NOT | 7.2 | **positive:** `unit/verify` [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L552). **negative:** `unit/verify` [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L553) |
| `RFC4090-7.2-2` | State MUST be removed if it has not been refreshed before the refresh timer expires. (S7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L592). **negative:** `unit/verify` [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L593) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4090-4.4-5`](#rfc4090-4.4-5) the PLR MUST set this flag when the desired bandwidth is guaranteed and the "bandwidth protection desired" flag was set in the SESSION_ATTRIBUTE object. (§4.4) | {gap}, no test | a bypass reserves no bandwidth, so the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |
| [`RFC4090-6-8`](#rfc4090-6-8) The PLR SHOULD set the "bandwidth protection" flag if the backup path offers a bandwidth guarantee (§6) | {gap} | a bypass reserves no bandwidth, so no backup path offers a bandwidth guarantee and the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4090-4.1-1`](#rfc4090-4.1-1)

Class-Num = 205 C-Type = 1 (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). TestRFC4090FastRerouteClassAndCTypeLiterals compares the encoded header with the literals 24/205/1 over a 0xFF buffer and decodes a raw 205/1 object as FAST_REROUTE; TestRFC4090FastRerouteOtherCTypeNotRecognized reads 205/2 as an unknown C-Type, not FAST_REROUTE. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.1 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFastRerouteShortBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC4090FastRerouteOtherCTypeNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestEncodeDecodeFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC4090FastRerouteClassAndCTypeLiterals`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L73) | unit/verify | revert, verified |

### [`RFC4090-4.3-1`](#rfc4090-4.3-1)

To indicate that an LSP should be locally protected, the head-end LSR MUST either set the "local protection desired" flag in the SESSION_ATTRIBUTE object or include a FAST_REROUTE object in the PATH message, or both. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. a protection-desired head-end PATH carries SESSION_ATTRIBUTE with 0x01 and FAST_REROUTE; negative is an unprotected PSB emitting neither (a conditioned-off input, not a violation) Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L183) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168) | unit/verify | unproven |

### [`RFC4090-4.3-2`](#rfc4090-4.3-2)

If node protection is desired, the head-end LSR should set the "node protection desired" flag in the SESSION_ATTRIBUTE object; otherwise, this flag should be cleared. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). The 'otherwise cleared' clause is now driven through buildPath: TestRFC4090NodeDesiredClearedWhenNotDesired builds a protected PATH without node protection and reads SESSION_ATTRIBUTE 0x01 set, 0x10 clear. The set clause stays proven on the built PATH. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestRFC4090NodeDesiredClearedWhenNotDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63) | unit/verify | revert, verified |

### [`RFC4090-4.1-2`](#rfc4090-4.1-2)

If the head-end LSR desires that the one-to-one backup method be used for the protected LSP, then the head-end LSR should include a FAST_REROUTE object and set the "one-to-one backup desired" flag. If the head-end LSR desires that the protected LSP be protected via the facility backup method, then the head-end LSR should include a FAST_REROUTE object and set the "facility backup desired" flag. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). Both clauses on the built PATH: TestRFC4090OneToOneRequestInPath (FAST_REROUTE present, 0x01 set) and TestBuildPathIncludesFastReroute for facility; TestRFC4090RequestNeverNamesOtherMethod shows each request leaves the other method's flag clear and the facility request sets 0x02. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L847) | unit/verify | revert, verified |
| negative | [`TestRFC4090RequestNeverNamesOtherMethod`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestRFC4090OneToOneRequestInPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L28) | unit/verify | revert, verified |

### [`RFC4090-4.4-1`](#rfc4090-4.4-1)

Whenever the PLR has a backup path available, the PLR MUST set the "local protection available" flag. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. full engine: armed bypass brought up, relayed RESV RRO subobject carries 0x01; negative is an unprotected LSP with zero flags Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L350) | unit/verify | unproven |
| positive | [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L332) | unit/verify | revert, verified |

### [`RFC4090-4.4-2`](#rfc4090-4.4-2)

During fast reroute, for each protected LSP containing an RRO object, the PLR obtains the RRO from the protected LSP's stored RESV. The PLR MUST update the IPv4 or IPv6 sub-object it inserted into the RRO by setting the "Local protection in use" and "Local Protection Available" flags. (§6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). TestRFC4090RepairSetsInUseAndAvailable drives handleLinkDown onto the bypass, then sendResv from the stored state, and reads the PLR's own RRO subobject with both 0x01 and 0x02 set; the fixture asserts in-use was clear before repair. Existing negative keeps in-use clear while no repair happened. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6.5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L863) | unit/verify | revert, verified |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L871) | unit/verify | revert, verified |
| positive | [`TestRFC4090RepairSetsInUseAndAvailable`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L68) | unit/verify | revert, verified |

### [`RFC4090-4.4-3`](#rfc4090-4.4-3)

the PLR MUST set this flag when the node protection is provided and the "node protection desired" flag was set in the SESSION_ATTRIBUTE object. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive TestRFC4090NodeBitSetWhenNodeProtectionProvided: node request with an NNHOP bypass armed relays 0x08. Negative TestRFC4090NodeBitClearWhenNodeProtectionNotProvided: node request with only an NHOP bypass up relays 0x08 clear; its record breaks selectBypass, the guard that keeps rroProtectionFlags (which reads the request bit) from claiming node protection over a link bypass. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.4 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L865) | unit/verify | revert, verified |
| negative | [`TestRFC4090NodeBitClearWhenNodeProtectionNotProvided`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L876) | unit/verify | revert, verified |
| positive | [`TestRFC4090NodeBitSetWhenNodeProtectionProvided`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L93) | unit/verify | revert, verified |

### [`RFC4090-6.5-1`](#rfc4090-6.5-1)

To provide this notification, the PLR SHOULD send a Path Error message with error code of "Notify" (Error code = 25) and an error value field of ss00 cccc cccc cccc, where ss=00 and the sub-code = 3 ("Tunnel locally repaired") (see [RSVP-TE]). (§6.5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). TestRFC4090LocalRepairNotifyLiterals compares the PathErr sent toward the head-end after handleLinkDown with the literals 25 and 0x0003, not the constants. Negative: an unrepairable failure sends code 24, not a Notify. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6.5.1 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L476) | unit/verify | revert, verified |
| positive | [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L455) | unit/verify | revert, verified |
| positive | [`TestRFC4090LocalRepairNotifyLiterals`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L116) | unit/verify | revert, verified |

### [`RFC4090-3.2-1`](#rfc4090-3.2-1)

The label will be switched for one which will be understood by R4 to indicate the protected LSP, and the bypass tunnel's label will then be pushed onto the label- stack of the redirected packets. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestLocalRepairSwitchesFIB asserts the exact backup stack [5000 bypass, 18000 MP label]; negative TestLocalRepairFallsBackToTeardown programs no stacked swap without a usable bypass. The quoted Section 3.2 sentence is descriptive ('will be switched') and carries no RFC 2119 keyword, although the row is [MUST] Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 3.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L470) | unit/verify | unproven |
| positive | [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L426) | unit/verify | unproven |

### [`RFC4090-4.2-1`](#rfc4090-4.2-1)

LSRs that do not support the DETOUR objects MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. a PATH with a DETOUR object is rejected: PathErr to the PLR, no RESV, no LSP state; the same PATH without DETOUR is accepted Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L918) | unit/verify | unproven |
| positive | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L899) | unit/verify | unproven |

### [`RFC4090-4.2-2`](#rfc4090-4.2-2)

This PathErr SHOULD be generated as specified in [RSVP] for unknown objects with a Class-Num of the form "0bbbbbbb". (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive: a PATH with DETOUR (Class-Num 63, C-Type 7) draws a PathErr to the PLR with code 13 and value 0x3f07 and no path state. Negative: C-Type 8 is still answered as unknown class (13, 0x3f08), not unknown C-Type (14). Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DetourOtherCTypeStillUnknownClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L648) | unit/verify | revert, verified |
| positive | [`TestRFC4090DetourPathErrAsUnknownClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L628) | unit/verify | revert, verified |

### [`RFC4090-6.5-3`](#rfc4090-6.5-3)

The globally revertive mode SHOULD always be used. (§6.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: both units gained only the RFC4090-6.5.2-1 tag comment; bodies byte-identical. Positive TestRFC4090HeadEndRevertsGlobally: on Notify 25/3 the head-end signals a make-before-break replacement (next LSP_ID PATH) and keeps the repaired LSP, the global revertive behavior. Negative TestRFC4090TransitDoesNotReoptimize: a transit starts no replacement and relays the Notify to the head-end. Re-judged 2026-10-08 against rfc4090.txt Section 6.5.2: TestRFC4090TransitDoesNotReoptimize changed only by passing nil for buildPathErr's new adspec argument (commit 9b8bfe250c); the inbound Notify is the same bytes as before (no ADSPEC), the assertions are unchanged, and handlePathErr changed only by threading the received ADSPEC into the relayed PathErr. Still red with handlePathErr disabled.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090TransitDoesNotReoptimize`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L689) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndRevertsGlobally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L666) | unit/verify | revert, verified |

### [`RFC4090-6.5.2-1`](#rfc4090-6.5.2-1)

- Global revertive mode: The head-end LSR of each tunnel is responsible for reoptimizing the TE LSPs that used the failed resource. (§6.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30. Positive TestRFC4090HeadEndRevertsGlobally: the head-end receiving Notify 25/3 creates the replacement LSP (LSPID 2) itself, keeps the repaired LSP, and sends a PATH with SenderTemplate LSPID 2. Negative TestRFC4090TransitDoesNotReoptimize: a transit receiving the same Notify creates no replacement, sends no PATH (count unchanged) and relays the PathErr 25/3 to the ingress. Level SHOULD over 'is responsible for' (no keyword) follows the section's SHOULD on global mode, accepted under D-3. Recorded red on frr.go::reoptimizeOnNotify (+) and engine.go::handlePathErr (-). Re-judged 2026-10-08 against rfc4090.txt Section 6.5.2: the negative unit changed only by a nil adspec argument to buildPathErr (commit 9b8bfe250c); input bytes and assertions unchanged, still red with handlePathErr disabled.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090TransitDoesNotReoptimize`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L690) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndRevertsGlobally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L667) | unit/verify | revert, verified |

### [`RFC4090-4.1-3`](#rfc4090-4.1-3)

This object MUST only be inserted into the PATH message by the head-end LER and MUST NOT be changed by downstream LSRs. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. a transit relays the received FAST_REROUTE unchanged field for field, and inserts none when the head-end sent only the SESSION_ATTRIBUTE flag Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.1 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC4090TransitRelaysFastRerouteUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L81) | unit/verify | revert, verified |

### [`RFC4090-4.4-5`](#rfc4090-4.4-5)

the PLR MUST set this flag when the desired bandwidth is guaranteed and the "bandwidth protection desired" flag was set in the SESSION_ATTRIBUTE object. (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4090-4.4-5, so no unit is bound to it.

### [`RFC4090-4.4-6`](#rfc4090-4.4-6)

If the requested bandwidth is not guaranteed, the PLR MUST NOT set this flag. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Genuine negative TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair: bandwidth desired, FAST_REROUTE bandwidth, a downstream subobject claiming 0x04 and an armed unguaranteed bypass; the PLR's own subobject keeps 0x04 clear before and during local repair. The positive TestRFC4090BandwidthBitNeverClaimed is the conforming report (0x01 only). No input can guarantee bandwidth (RFC4090-4.4-5 gap). Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.4 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L213) | unit/verify | revert, verified |
| negative | [`TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L214) | unit/verify | revert, verified |

### [`RFC4090-4.4-7`](#rfc4090-4.4-7)

If node protection is not provided, the PLR MUST NOT set this flag. Thus, if a PLR could only set up a link-protection backup path, the "Local protection available" bit will be set, but the "Node protection" bit will be cleared. (§4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive TestRFC4090NodeBitSetWhenNodeProtected; negative TestRFC4090LinkProtectionLeavesNodeBitClear (link request, NHOP bypass up: 0x01 set, 0x08 clear), which is the row's link-only case. The vacuous tag on TestRFC4090NodeBitClearWithoutNodeBypass is removed. A node request never falls back to a link bypass in Ze (spec-rsvpte-frr-link-protection-fallback). Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 4.4 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090LinkProtectionLeavesNodeBitClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L284) | unit/verify | revert, verified |

### [`RFC4090-5-1`](#rfc4090-5-1)

If a head-end LSR signals a FAST_REROUTE object, it MUST be stored for Path refreshes. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. head-end refresh PATH carries the stored FAST_REROUTE field for field; a tunnel with no protection refreshes none Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndRefreshWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L408) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndStoresFastRerouteForRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L387) | unit/verify | revert, verified |

### [`RFC4090-5-2`](#rfc4090-5-2)

The head-end LSR of a protected LSP MUST set the "label recording desired" flag in the SESSION_ATTRIBUTE object. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. protected tunnel's PATH SESSION_ATTRIBUTE has label recording desired set (SessAttrLabelRecording = 0x02); negative is an unprotected tunnel emitting no SESSION_ATTRIBUTE, a conditioned-off input. Tag prose on TestRFC4090HeadEndSetsLabelRecordingDesired says 0x04, which is the SE Style flag Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090UnprotectedPathRequestsNoLabelRecording`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L433) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndSetsLabelRecordingDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L422) | unit/verify | revert, verified |

### [`RFC4090-5-3`](#rfc4090-5-3)

The head-end LSR of a protected LSP MUST support the additional flags defined in Section 4.4 being set or clear in the RRO IPv4 and IPv6 sub-objects. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. head-end accepts a RESV whose RRO subobject carries all four flags (0x0F) or none, LSP goes Up and flags stored as received Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L490) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L489) | unit/verify | revert, verified |

### [`RFC4090-5-4`](#rfc4090-5-4)

The head-end LSR of a protected LSP MUST support the RRO Label sub-object. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. head-end accepts an RRO with Label subobjects and resolves the recorded label; no label recorded resolves none Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 5 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L506) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L505) | unit/verify | revert, verified |

### [`RFC4090-6-1`](#rfc4090-6-1)

Every LSR along a protected LSP (except the egress) MUST follow the PLR behavior described in this document. (S6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. transit on a protected LSP arms the NHOP bypass; the egress arms none and reports no flag. Umbrella row: the specific PLR behaviours carry their own rows Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090EgressDoesNotActAsPLR`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L134) | unit/verify | revert, verified |

### [`RFC4090-6-2`](#rfc4090-6-2)

A PLR MUST consider an LSP to have asked for local protection if the "local protection desired" flag is set in the SESSION_ATTRIBUTE object and/or the FAST_REROUTE object is included. (S6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. flag-only and FAST_REROUTE-only PATHs are both protection requests that arm a bypass; flag clear and no FAST_REROUTE stores nothing Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoProtectionRequestArmsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L106) | unit/verify | revert, verified |

### [`RFC4090-6-3`](#rfc4090-6-3)

- Until a PLR has a backup path available, the PLR MUST clear the relevant four flags in the corresponding RRO IPv4 or IPv6 sub- object. (S6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. armed bypass not yet up: relayed RRO subobject flags are 0; once up, 0x01 set Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L55) | unit/verify | revert, verified |

### [`RFC4090-6-4`](#rfc4090-6-4)

If no established one-to-one backup LSP or bypass tunnel exists, or if the one-to-one LSP and the bypass tunnel is in "DOWN" state, the PLR MUST clear the "local protection available" flag in its IPv4 (or IPv6) address sub-object of the RRO (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. bypass not established: 0x01 clear; established: set; bypass goes Down: next RESV refresh clears it Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L48) | unit/verify | revert, verified |
| negative | [`TestRFC4090RefreshTracksBypassState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L56) | unit/verify | revert, verified |

### [`RFC4090-6-5`](#rfc4090-6-5)

- The PLR MUST clear the "local protection in use" flag unless it is actively redirecting traffic into the backup path instead of along the protected LSP. (S6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. in-use bit clear on the relayed RESV while armed and up, set only after link failure redirected traffic Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L318) | unit/verify | revert, verified |
| positive | [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L317) | unit/verify | revert, verified |

### [`RFC4090-6-6`](#rfc4090-6-6)

The PLR SHOULD also set the "node protection" flag if the backup path protects against the failure of the immediate downstream node, and, if the path does not, the PLR SHOULD clear the "node protection" flag. This MUST be done if the "node protection desired" flag was set in the SESSION_ATTRIBUTE object. (§6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Judged 2026-09-29 (routing child, continuation judge). Unchanged judgement; TestRFC4090NodeBitClearWithoutNodeBypass lost only its RFC4090-4.4-7 tag. The set half is proven; the clear half still runs with no backup path armed, because selectBypass refuses an NHOP bypass for a node request, so no unit shows a backup path that does not protect the next node with the node bit cleared. Blocked by spec-rsvpte-frr-link-protection-fallback. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the weak verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L301) | unit/verify | revert, verified |
| positive | [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L285) | unit/verify | revert, verified |

### [`RFC4090-6-7`](#rfc4090-6-7)

if the path does not, the PLR SHOULD clear the "bandwidth protection" flag. This MUST be done if the "bandwidth protection desired" flag was set in the SESSION_ATTRIBUTE object. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Row narrowed to the clear clause and its MUST condition; the set clause is RFC4090-6-8 {gap}. With bandwidth desired and a backup that guarantees no bandwidth, the bit is clear: TestRFC4090BandwidthBitNeverClaimed, and TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair pushes every input toward setting it (request, downstream claim, repair). TestRFC4090BandwidthDesiredWithoutBackup is a second conforming case. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BandwidthDesiredWithoutBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L235) | unit/verify | revert, verified |
| negative | [`TestRFC4090BandwidthBitNotClaimedFromDownstreamOrRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_plr_flags_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L215) | unit/verify | revert, verified |

### [`RFC4090-6.1-1`](#rfc4090-6.1-1)

If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel. (S6.1, S6.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. head-end PLR signals the backup PATH from the distinct local address 10.0.1.2, not its SENDER_TEMPLATE address; with no alternate address no backup identity is signaled Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6.1, 6.1.1 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndRequiresAlternateSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L387) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndUsesDistinctSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L348) | unit/verify | revert, verified |

### [`RFC4090-6.2-1`](#rfc4090-6.2-1)

For bypass tunnels (Section 7), the destination MUST be the address of the MP. (§6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. bypass PATH SESSION tunnel endpoint and IP destination are the configured merge point; a bypass to another MP is not selected Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L364) | unit/verify | mutant, verified |
| positive | [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L363) | unit/verify | mutant, verified |

### [`RFC4090-6.4-1`](#rfc4090-6.4-1)

The RSVP_HOP object MUST contain an IP source address belonging to the PLR. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. backup PATH RSVP_HOP equals the PLR router id in link, node and egress cases; no backup PATH without an established bypass Re-judged 2026-10-08 against rfc4090.txt Sections 6.4.3, 6.4.4 and 7.1.1: TestRFC4090ProtectedPathSurvivesRepair changed only by passing nil for buildPathErr's new adspec argument in the late MP PathErr it injects (commit 9b8bfe250c); every assertion on the backup PATH, its ERO, the MP's forwarded sender and its refresh is unchanged, and the discrimination record was re-observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L281) | unit/verify | revert, verified |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L144) | unit/verify | revert, verified |

### [`RFC4090-6.4-2`](#rfc4090-6.4-2)

The PLR MUST generate an EXPLICIT_ROUTE object toward the egress. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. backup PATH carries the ERO from the MP to the egress (equals psb.ERO[index:]); no backup ERO before repair Re-judged 2026-10-08 against rfc4090.txt Sections 6.4.3, 6.4.4 and 7.1.1: TestRFC4090ProtectedPathSurvivesRepair changed only by passing nil for buildPathErr's new adspec argument in the late MP PathErr it injects (commit 9b8bfe250c); every assertion on the backup PATH, its ERO, the MP's forwarded sender and its refresh is unchanged, and the discrimination record was re-observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L282) | unit/verify | revert, verified |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L145) | unit/verify | revert, verified |

### [`RFC4090-6.4.3-1`](#rfc4090-6.4.3-1)

When the PLR detects a link or/and node failure condition, it has to reroute the data traffic onto the bypass tunnel (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). TestRFC4090FailureSwitchesToBackup: a failure on the protected link reprograms the protected in-label to the bypass next hop under the bypass label; a failure on an unrelated link programs no backup. Both polarities sit in one unit on separate assertions; the revert record reds through the positive half only. Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 6.4.3 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L342) | unit/verify | revert, verified |
| positive | [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L341) | unit/verify | revert, verified |

### [`RFC4090-6.4-3`](#rfc4090-6.4-3)

More specifically, the PLR MUST: - remove all the sub-objects proceeding the first address belonging to the MP, and - replace this first MP address with an IP address of the MP. (§6.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. backup ERO starts at the MP address with the preceding hops removed, including the failed node in the node case; before repair the normal ERO is kept Re-judged 2026-10-08 against rfc4090.txt Sections 6.4.3, 6.4.4 and 7.1.1: TestRFC4090ProtectedPathSurvivesRepair changed only by passing nil for buildPathErr's new adspec argument in the late MP PathErr it injects (commit 9b8bfe250c); every assertion on the backup PATH, its ERO, the MP's forwarded sender and its refresh is unchanged, and the discrimination record was re-observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L283) | unit/verify | revert, verified |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L146) | unit/verify | revert, verified |

### [`RFC4090-7.1-1`](#rfc4090-7.1-1)

If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP. (§7.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4090ProtectedPathSurvivesRepair (link and node cases): after the PLR's backup PATH reaches the MP, the MP forwards the protected LSP's SENDER_TEMPLATE downstream; negative TestRFC4090DifferentPathsDoNotMerge: a PATH whose remaining ERO differs is not merged and keeps its own sender Re-judged 2026-10-08 against rfc4090.txt Sections 6.4.3, 6.4.4 and 7.1.1: TestRFC4090ProtectedPathSurvivesRepair changed only by passing nil for buildPathErr's new adspec argument in the late MP PathErr it injects (commit 9b8bfe250c); every assertion on the backup PATH, its ERO, the MP's forwarded sender and its refresh is unchanged, and the discrimination record was re-observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L298) | unit/verify | revert, verified |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L147) | unit/verify | revert, verified |

### [`RFC4090-7.1-2`](#rfc4090-7.1-2)

Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. (§7.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4090ProtectedPathSurvivesRepair: refreshPaths on the MP emits exactly one more PATH with no new incoming PATH once the merged final PATH is identified; negative: an unmerged transit emits no local refresh Re-judged 2026-10-08 against rfc4090.txt Sections 6.4.3, 6.4.4 and 7.1.1: TestRFC4090ProtectedPathSurvivesRepair changed only by passing nil for buildPathErr's new adspec argument in the late MP PathErr it injects (commit 9b8bfe250c); every assertion on the backup PATH, its ERO, the MP's forwarded sender and its refresh is unchanged, and the discrimination record was re-observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L299) | unit/verify | revert, verified |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_bypass_test.go#L148) | unit/verify | revert, verified |

### [`RFC4090-7.2-1`](#rfc4090-7.2-1)

When a downstream LSR detects a local link failure, for any protected LSPs routed over the failed link, Path and Resv state MUST NOT be cleared, and PathTear and ResvErr messages MUST NOT be sent immediately. (S7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. downstream LSR keeps PSB and RSB on upstream link failure and sends no PathTear/PathErr/ResvErr; as PLR with no bypass it tears down Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 7.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L553) | unit/verify | revert, verified |
| positive | [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L552) | unit/verify | revert, verified |

### [`RFC4090-7.2-2`](#rfc4090-7.2-2)

State MUST be removed if it has not been refreshed before the refresh timer expires. (S7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. cleanupTick removes the LSP whose PATH is older than period x multiplier and keeps a refreshed one Re-stamped 2026-10-08: every tagged unit is byte-identical to the judged one (rfc check: SHIFTED; the stamp recomputed the same unit fingerprints), only its test file moved; the row was re-read verbatim against rfc4090.txt Section 7.2 and the enforced verdict holds unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L593) | unit/verify | revert, verified |
| positive | [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc4090_frr_test.go#L592) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-29 |
| Register | rfc2119 |
| Source | rfc/full/rfc4090.txt |
| Source fingerprint | d53e6ba082494f4e |
| Record | rfc/extraction/rfc4090.json |
| Mapped sentences | 29 |
| Declined as scope | 19 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 4 | walked | not stated |
| `5` | not stated | 5 | walked | not stated |
| `6` | not stated | 8 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 1 | walked | not stated |
| `6.1.2` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `6.3` | not stated | 5 | walked | not stated |
| `6.3.1` | not stated | 0 | walked | not stated |
| `6.3.2` | not stated | 6 | walked | not stated |
| `6.3.3` | not stated | 1 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.4.1` | not stated | 1 | walked | not stated |
| `6.4.2` | not stated | 0 | walked | not stated |
| `6.4.3` | not stated | 2 | walked | not stated |
| `6.4.4` | not stated | 1 | walked | not stated |
| `6.5` | not stated | 1 | walked | not stated |
| `6.5.1` | not stated | 0 | walked | not stated |
| `6.5.2` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.1.1` | not stated | 2 | walked | not stated |
| `7.1.2` | not stated | 4 | walked | not stated |
| `7.1.2.1` | not stated | 0 | walked | not stated |
| `7.1.3` | not stated | 1 | walked | not stated |
| `7.2` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is the detour LSP destination rule. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - For detour LSPs, the destination MUST be the tail-end of the protected LSP. |
| `6.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour SENDER_TEMPLATE rewriting. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - If the sender template-specific method is to be used, then the PLR MUST change the "IPv4 (or IPv6) tunnel sender address" of the SENDER_TEMPLATE to an address belonging to the PLR that is not the same as that used for the protected LSP. |
| `6.3:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is the DETOUR object and the detour SESSION_ATTRIBUTE flags. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - If the path-specific method is to be used, then the PLR MUST add a DETOUR object to the PATH message. - The SESSION_ATTRIBUTE flags "Local protection desired", "Bandwidth protection desired", and "Node protection desired" MUST be cleared. |
| `6.3:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is FAST_REROUTE removal from a detour PATH. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - If the protected LSP's Path message contained a FAST_REROUTE object, this object MUST be removed from the detour LSP's PATH message. |
| `6.3:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is the detour EXPLICIT_ROUTE object. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - The PLR MUST generate an EXPLICIT_ROUTE object toward the egress. |
| `6.3:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is the detour reservation style. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | - The detour LSPs MUST use the same reservation style as the protected LSP. |
| `6.3.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour message separation. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | The PLR MUST not mix the messages for the protected and the detour LSPs. |
| `6.3.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour message forwarding. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | When a PLR receives Resv, ResvTear, and PathErr messages from the downstream detour destination, the messages MUST not be forwarded upstream. |
| `6.3.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour ResvErr/ResvConf propagation. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | Similarly, when a PLR receives ResvErr and ResvConf messages from a protected LSP, it MUST not propagate them onto the associated detour LSP. |
| `6.3.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour PathTear handling. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | When a PLR node receives a PathTear message from upstream, it MUST delete both the protected and the detour LSPs. |
| `6.3.2:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour PathTear propagation. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | The PathTear messages MUST propagate to both protected and detour LSPs. |
| `6.3.2:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour ResvTear propagation. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | When a PLR node receives the ResvTear messages from downstream for a protected LSP, as long as a detour is up, the ResvTear messages MUST not be sent further upstream. |
| `6.3.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is the detour switch-over rule of Section 6.3.3, Local Reroute of Traffic onto Detour LSP. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." The facility-backup switch-over Ze performs is row RFC4090-6.4.3-1 (Section 6.4.3). | When the PLR detects a failure on the protected LSP, the PLR MUST rapidly switch packets to the protected LSP's backup LSP instead of to the protected LSP's normal out-segment. |
| `6.4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 5 head-end obligation to set the label recording desired flag, which RFC4090-5-2 already carries; this sentence says so itself ("As described in Section 6"). | As described in Section 6, the head-end LSR MUST set the "label recording requested" flag in the SESSION_ATTRIBUTE object for LSPs requesting local protection. |
| `7.1.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour merging by the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | In this case, Path state merging is REQUIRED. |
| `7.1.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour Path state recording under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | Otherwise, the MP MUST record the Path state and the incoming interface. |
| `7.1.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour merge eligibility under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | If the Path messages do not share an outgoing interface and a next-hop LSR, the MP MUST consider them to be independent LSPs and MUST NOT merge them. |
| `7.1.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour refresh after merging under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. |
| `7.1.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is ResvTear handling for merged detours. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | If the LSR does not have an alternate associated LSP, then the MP MUST propagate the ResvTear toward the LSP's ingress, and, for each backup LSP merged into that LSP at this LSR, the ResvTear SHOULD also be propagated along the backup LSP. |

## Superseded

No document obsoletes RFC 4090, so its obligations are stated where they were written.
