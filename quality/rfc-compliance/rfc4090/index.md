# RFC 4090 - Fast Reroute Extensions to RSVP-TE for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 4090, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 97.1% | 33 of 34 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 34 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 34 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 56.6% | 43 of 76 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 34 | of 39 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 34 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 34 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 34 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 34 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 2.9% | 1 of 34 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 34 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 39 |
| Gated MUST-level | 34 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 76 |
| Tagged units | 76 |
| Recorded audit verdicts | 0 |
| Discrimination records | 43 |
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

PLRs now signal and refresh the protected PATH through the bypass, and an MP merges it into matching protected state (Sections 6.4.3, 6.4.4, 7.1.1). A head-end PLR uses a distinct assigned local sender address and a two-label ingress push (Section 6.1.1). Runtime and discrimination evidence must be regenerated for these producers. One-to-one detours remain absent; Section 6 states "A PLR MAY support the DETOUR object". A bandwidth-guaranteed bypass is also absent ([`plan/spec-rsvpte-bypass-bandwidth-protection.md`](https://github.com/ze-software/ze/blob/main/plan/spec-rsvpte-bypass-bandwidth-protection.md)). Those optional feature records require a scope decision and do not authorize implementation.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 33 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **34** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (33):** [`RFC4090-4.1-1`](#rfc4090-4.1-1), [`RFC4090-4.3-1`](#rfc4090-4.3-1), [`RFC4090-4.4-1`](#rfc4090-4.4-1), [`RFC4090-4.4-2`](#rfc4090-4.4-2), [`RFC4090-4.4-3`](#rfc4090-4.4-3), [`RFC4090-6.5-2`](#rfc4090-6.5-2), [`RFC4090-3.2-1`](#rfc4090-3.2-1), [`RFC4090-3.2-2`](#rfc4090-3.2-2), [`RFC4090-4.2-1`](#rfc4090-4.2-1), [`RFC4090-4.1-3`](#rfc4090-4.1-3), [`RFC4090-4.4-6`](#rfc4090-4.4-6), [`RFC4090-4.4-7`](#rfc4090-4.4-7), [`RFC4090-5-1`](#rfc4090-5-1), [`RFC4090-5-2`](#rfc4090-5-2), [`RFC4090-5-3`](#rfc4090-5-3), [`RFC4090-5-4`](#rfc4090-5-4), [`RFC4090-6-1`](#rfc4090-6-1), [`RFC4090-6-2`](#rfc4090-6-2), [`RFC4090-6-3`](#rfc4090-6-3), [`RFC4090-6-4`](#rfc4090-6-4), [`RFC4090-6-5`](#rfc4090-6-5), [`RFC4090-6-6`](#rfc4090-6-6), [`RFC4090-6-7`](#rfc4090-6-7), [`RFC4090-6.1-1`](#rfc4090-6.1-1), [`RFC4090-6.2-1`](#rfc4090-6.2-1), [`RFC4090-6.3-1`](#rfc4090-6.3-1), [`RFC4090-6.4-1`](#rfc4090-6.4-1), [`RFC4090-6.4-2`](#rfc4090-6.4-2), [`RFC4090-6.4-3`](#rfc4090-6.4-3), [`RFC4090-7.1-1`](#rfc4090-7.1-1), [`RFC4090-7.1-2`](#rfc4090-7.1-2), [`RFC4090-7.2-1`](#rfc4090-7.2-1), [`RFC4090-7.2-2`](#rfc4090-7.2-2)

**Annotated instead of tested (1):** [`RFC4090-4.4-5`](#rfc4090-4.4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4090-4.1-1` | FAST_REROUTE object uses Class-Num 205, C-Type 1, object Length 24 (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestEncodeDecodeFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L35). **negative:** `unit/verify` [`TestFastRerouteShortBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L49) |
| `RFC4090-4.3-1` | To indicate that an LSP should be locally protected, the head-end LSR MUST either set the "local protection desired" flag (0x01) in the SESSION_ATTRIBUTE object or include a FAST_REROUTE object in the PATH message, or both (S4.3, S5) | MUST | 4.3 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168). **negative:** `unit/verify` [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L183) |
| `RFC4090-4.3-2` | "If node protection is desired, the head-end LSR should set the 'node protection desired' flag in the SESSION_ATTRIBUTE object; otherwise, this flag should be cleared" (0x10) (S4.3, S5) | SHOULD | 4.3 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L170). **positive:** `unit/verify` [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63). **negative:** `unit/verify` [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) |
| `RFC4090-4.1-2` | The head-end LSR "should include a FAST_REROUTE object and set the 'one-to-one backup desired' flag" (0x01), or for facility backup "should include a FAST_REROUTE object and set the 'facility backup desired' flag" (0x02) (S4.1, S5) | SHOULD | 4.1 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L163). **negative:** `unit/verify` [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L852) |
| `RFC4090-4.4-1` | "Whenever the PLR has a backup path available, the PLR MUST set the 'local protection available' flag" (0x01) (S4.4, S6) | MUST | 4.4 | **positive:** `unit/verify` [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L333). **negative:** `unit/verify` [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L351) |
| `RFC4090-4.4-2` | During fast reroute "the PLR MUST update the IPv4 or IPv6 sub-object it inserted into the RRO by setting the 'Local protection in use' and 'Local Protection Available' flags" (S4.4, S6.5) | MUST | 4.4 | **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L876). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L868) |
| `RFC4090-4.4-3` | "the PLR MUST set this flag when the node protection is provided and the 'node protection desired' flag was set in the SESSION_ATTRIBUTE object" (node protection, 0x08) (S4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L881). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L870) |
| `RFC4090-6.5-1` | "the PLR SHOULD send a Path Error message with error code of 'Notify' (Error code = 25) and an error value field of ss00 cccc cccc cccc, where ss=00 and the sub-code = 3 ('Tunnel locally repaired')" (S6.5, S6.5.1) | SHOULD | 6.5 | **positive:** `unit/verify` [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L457). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L479) |
| `RFC4090-6.5-2` | On local repair the PLR MUST NOT tear down the protected LSP (S6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L431). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L475) |
| `RFC4090-3.2-1` | Facility backup pushes the bypass label on top of the protected LSP label (label stacking) (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L427). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L472) |
| `RFC4090-3.2-2` | The merge point is the NHOP for link protection and the NNHOP for node protection (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestNodeProtectionLocalRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L769). **positive:** `unit/verify` [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L312). **negative:** `unit/verify` [`TestNodeProtectionNeedsNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L810) |
| `RFC4090-4.2-1` | An LSR that does not support the DETOUR object MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR, generated as [RSVP] specifies for unknown objects with a Class-Num of the form "0bbbbbbb" (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L904). **negative:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L923) |
| `RFC4090-6.5-3` | Global revertive mode: "The head-end LSR of each tunnel is responsible for reoptimizing the TE LSPs that used the failed resource" after a Notify (S6.5, S6.5.2) | SHOULD | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4090-4.4-4` | "A PLR MAY support the DETOUR object" (Class-Num 63), which identifies one-to-one backup detour LSPs (S4.4, S4.2, S6) | MAY | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4090-4.1-3` | The FAST_REROUTE object "MUST only be inserted into the PATH message by the head-end LER and MUST NOT be changed by downstream LSRs" (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4090TransitRelaysFastRerouteUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L81). **negative:** `unit/verify` [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L105) |
| `RFC4090-4.4-5` | "the PLR MUST set this flag when the desired bandwidth is guaranteed and the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object" (bandwidth protection, 0x04) (S4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a bypass reserves no bandwidth, so the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |
| `RFC4090-4.4-6` | "If the requested bandwidth is not guaranteed, the PLR MUST NOT set this flag" (bandwidth protection, 0x04) (S4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L214). **negative:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L213) |
| `RFC4090-4.4-7` | "If node protection is not provided, the PLR MUST NOT set this flag" (node protection, 0x08) (S4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L284). **negative:** `unit/verify` [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L301) |
| `RFC4090-5-1` | If a head-end LSR signals a FAST_REROUTE object, it MUST be stored for Path refreshes. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndStoresFastRerouteForRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L388). **negative:** `unit/verify` [`TestRFC4090HeadEndRefreshWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L409) |
| `RFC4090-5-2` | The head-end LSR of a protected LSP MUST set the "label recording desired" flag in the SESSION_ATTRIBUTE object. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndSetsLabelRecordingDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L423). **negative:** `unit/verify` [`TestRFC4090UnprotectedPathRequestsNoLabelRecording`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L434) |
| `RFC4090-5-3` | The head-end LSR of a protected LSP MUST support the additional flags defined in Section 4.4 being set or clear in the RRO IPv4 and IPv6 sub-objects. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L490). **negative:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L491) |
| `RFC4090-5-4` | The head-end LSR of a protected LSP MUST support the RRO Label sub-object. (S5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L506). **negative:** `unit/verify` [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L507) |
| `RFC4090-6-1` | Every LSR along a protected LSP (except the egress) MUST follow the PLR behavior described in this document. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L134). **negative:** `unit/verify` [`TestRFC4090EgressDoesNotActAsPLR`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L176) |
| `RFC4090-6-2` | A PLR MUST consider an LSP to have asked for local protection if the "local protection desired" flag is set in the SESSION_ATTRIBUTE object and/or the FAST_REROUTE object is included. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L133). **positive:** `unit/verify` [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L106). **negative:** `unit/verify` [`TestRFC4090NoProtectionRequestArmsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L154) |
| `RFC4090-6-3` | - Until a PLR has a backup path available, the PLR MUST clear the relevant four flags in the corresponding RRO IPv4 or IPv6 sub- object. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L55). **negative:** `unit/verify` [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L47) |
| `RFC4090-6-4` | "If no established one-to-one backup LSP or bypass tunnel exists, or if the one-to-one LSP and the bypass tunnel is in 'DOWN' state, the PLR MUST clear the 'local protection available' flag in its IPv4 (or IPv6) address sub-object of the RRO" (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L56). **negative:** `unit/verify` [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L48). **negative:** `unit/verify` [`TestRFC4090RefreshTracksBypassState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L64) |
| `RFC4090-6-5` | - The PLR MUST clear the "local protection in use" flag unless it is actively redirecting traffic into the backup path instead of along the protected LSP. (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L318). **negative:** `unit/verify` [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L319) |
| `RFC4090-6-6` | Setting or clearing the "node protection" flag to match the backup path "MUST be done if the 'node protection desired' flag was set in the SESSION_ATTRIBUTE object" (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L285). **negative:** `unit/verify` [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L302) |
| `RFC4090-6-7` | Setting or clearing the "bandwidth protection" flag to match the backup path "MUST be done if the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object" (S6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L215). **negative:** `unit/verify` [`TestRFC4090BandwidthDesiredWithoutBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L235) |
| `RFC4090-6.1-1` | If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel. (S6.1, S6.1.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC4090HeadEndUsesDistinctSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L348). **negative:** `unit/verify` [`TestRFC4090HeadEndRequiresAlternateSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L387) |
| `RFC4090-6.2-1` | "For bypass tunnels (Section 7), the destination MUST be the address of the MP" (S6.2) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L364). **negative:** `unit/verify` [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L365) |
| `RFC4090-6.3-1` | When the PLR detects a failure on the protected LSP, the PLR MUST rapidly switch packets to the protected LSP's backup LSP instead of to the protected LSP's normal out-segment. (S6.3, S6.3.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L342). **negative:** `unit/verify` [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L343) |
| `RFC4090-6.4-1` | On the backup Path message "the RSVP_HOP object MUST contain an IP source address belonging to the PLR" (S6.4, S6.4.3) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L144). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L281) |
| `RFC4090-6.4-2` | On the backup Path message "the PLR MUST generate an EXPLICIT_ROUTE object toward the egress" (S6.4, S6.4.3) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L145). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L282) |
| `RFC4090-6.4-3` | Before sending a Path message via a bypass tunnel the PLR MUST "remove all the sub-objects proceeding the first address belonging to the MP" and "replace this first MP address with an IP address of the MP" (S6.4, S6.4.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L146). **negative:** `unit/verify` [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L283) |
| `RFC4090-7.1-1` | If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP. (S7.1, S7.1.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L147). **negative:** `unit/verify` [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L298) |
| `RFC4090-7.1-2` | Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. (S7.1, S7.1.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L148). **negative:** `unit/verify` [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L299) |
| `RFC4090-7.2-1` | When a downstream LSR detects a local link failure, for any protected LSPs routed over the failed link, Path and Resv state MUST NOT be cleared, and PathTear and ResvErr messages MUST NOT be sent immediately. (S7.2) | MUST NOT | 7.2 | **positive:** `unit/verify` [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L553). **negative:** `unit/verify` [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L554) |
| `RFC4090-7.2-2` | State MUST be removed if it has not been refreshed before the refresh timer expires. (S7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L593). **negative:** `unit/verify` [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L594) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4090-4.4-5`](#rfc4090-4.4-5) "the PLR MUST set this flag when the desired bandwidth is guaranteed and the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object" (bandwidth protection, 0x04) (S4.4) | {gap}, no test | a bypass reserves no bandwidth, so the flag has no positive input; plan/spec-rsvpte-bypass-bandwidth-protection.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4090-4.1-1`](#rfc4090-4.1-1)

FAST_REROUTE object uses Class-Num 205, C-Type 1, object Length 24 (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFastRerouteShortBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L49) | unit/verify | unproven |
| positive | [`TestEncodeDecodeFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L35) | unit/verify | unproven |

### [`RFC4090-4.3-1`](#rfc4090-4.3-1)

To indicate that an LSP should be locally protected, the head-end LSR MUST either set the "local protection desired" flag (0x01) in the SESSION_ATTRIBUTE object or include a FAST_REROUTE object in the PATH message, or both (S4.3, S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L183) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168) | unit/verify | unproven |

### [`RFC4090-4.3-2`](#rfc4090-4.3-2)

"If node protection is desired, the head-end LSR should set the 'node protection desired' flag in the SESSION_ATTRIBUTE object; otherwise, this flag should be cleared" (0x10) (S4.3, S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L170) | unit/verify | unproven |
| positive | [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63) | unit/verify | unproven |

### [`RFC4090-4.1-2`](#rfc4090-4.1-2)

The head-end LSR "should include a FAST_REROUTE object and set the 'one-to-one backup desired' flag" (0x01), or for facility backup "should include a FAST_REROUTE object and set the 'facility backup desired' flag" (0x02) (S4.1, S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L852) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L163) | unit/verify | unproven |

### [`RFC4090-4.4-1`](#rfc4090-4.4-1)

"Whenever the PLR has a backup path available, the PLR MUST set the 'local protection available' flag" (0x01) (S4.4, S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L351) | unit/verify | unproven |
| positive | [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L333) | unit/verify | revert, verified |

### [`RFC4090-4.4-2`](#rfc4090-4.4-2)

During fast reroute "the PLR MUST update the IPv4 or IPv6 sub-object it inserted into the RRO by setting the 'Local protection in use' and 'Local Protection Available' flags" (S4.4, S6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L868) | unit/verify | unproven |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L876) | unit/verify | unproven |

### [`RFC4090-4.4-3`](#rfc4090-4.4-3)

"the PLR MUST set this flag when the node protection is provided and the 'node protection desired' flag was set in the SESSION_ATTRIBUTE object" (node protection, 0x08) (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L870) | unit/verify | unproven |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L881) | unit/verify | unproven |

### [`RFC4090-6.5-1`](#rfc4090-6.5-1)

"the PLR SHOULD send a Path Error message with error code of 'Notify' (Error code = 25) and an error value field of ss00 cccc cccc cccc, where ss=00 and the sub-code = 3 ('Tunnel locally repaired')" (S6.5, S6.5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L479) | unit/verify | unproven |
| positive | [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L457) | unit/verify | unproven |

### [`RFC4090-6.5-2`](#rfc4090-6.5-2)

On local repair the PLR MUST NOT tear down the protected LSP (S6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L475) | unit/verify | unproven |
| positive | [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L431) | unit/verify | unproven |

### [`RFC4090-3.2-1`](#rfc4090-3.2-1)

Facility backup pushes the bypass label on top of the protected LSP label (label stacking) (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L472) | unit/verify | unproven |
| positive | [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L427) | unit/verify | unproven |

### [`RFC4090-3.2-2`](#rfc4090-3.2-2)

The merge point is the NHOP for link protection and the NNHOP for node protection (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNodeProtectionNeedsNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L810) | unit/verify | unproven |
| positive | [`TestNodeProtectionLocalRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L769) | unit/verify | unproven |
| positive | [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L312) | unit/verify | unproven |

### [`RFC4090-4.2-1`](#rfc4090-4.2-1)

An LSR that does not support the DETOUR object MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR, generated as [RSVP] specifies for unknown objects with a Class-Num of the form "0bbbbbbb" (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L923) | unit/verify | unproven |
| positive | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L904) | unit/verify | unproven |

### [`RFC4090-4.1-3`](#rfc4090-4.1-3)

The FAST_REROUTE object "MUST only be inserted into the PATH message by the head-end LER and MUST NOT be changed by downstream LSRs" (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC4090TransitRelaysFastRerouteUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L81) | unit/verify | revert, verified |

### [`RFC4090-4.4-5`](#rfc4090-4.4-5)

"the PLR MUST set this flag when the desired bandwidth is guaranteed and the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object" (bandwidth protection, 0x04) (S4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4090-4.4-5, so no unit is bound to it.

### [`RFC4090-4.4-6`](#rfc4090-4.4-6)

"If the requested bandwidth is not guaranteed, the PLR MUST NOT set this flag" (bandwidth protection, 0x04) (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L213) | unit/verify | revert, verified |
| positive | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L214) | unit/verify | revert, verified |

### [`RFC4090-4.4-7`](#rfc4090-4.4-7)

"If node protection is not provided, the PLR MUST NOT set this flag" (node protection, 0x08) (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L301) | unit/verify | revert, verified |
| positive | [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L284) | unit/verify | revert, verified |

### [`RFC4090-5-1`](#rfc4090-5-1)

If a head-end LSR signals a FAST_REROUTE object, it MUST be stored for Path refreshes. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndRefreshWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L409) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndStoresFastRerouteForRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L388) | unit/verify | revert, verified |

### [`RFC4090-5-2`](#rfc4090-5-2)

The head-end LSR of a protected LSP MUST set the "label recording desired" flag in the SESSION_ATTRIBUTE object. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090UnprotectedPathRequestsNoLabelRecording`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L434) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndSetsLabelRecordingDesired`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L423) | unit/verify | revert, verified |

### [`RFC4090-5-3`](#rfc4090-5-3)

The head-end LSR of a protected LSP MUST support the additional flags defined in Section 4.4 being set or clear in the RRO IPv4 and IPv6 sub-objects. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L491) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndAcceptsRROProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L490) | unit/verify | revert, verified |

### [`RFC4090-5-4`](#rfc4090-5-4)

The head-end LSR of a protected LSP MUST support the RRO Label sub-object. (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L507) | unit/verify | revert, verified |
| positive | [`TestRFC4090HeadEndAcceptsRROLabelSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L506) | unit/verify | revert, verified |

### [`RFC4090-6-1`](#rfc4090-6-1)

Every LSR along a protected LSP (except the egress) MUST follow the PLR behavior described in this document. (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090EgressDoesNotActAsPLR`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L134) | unit/verify | revert, verified |

### [`RFC4090-6-2`](#rfc4090-6-2)

A PLR MUST consider an LSP to have asked for local protection if the "local protection desired" flag is set in the SESSION_ATTRIBUTE object and/or the FAST_REROUTE object is included. (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoProtectionRequestArmsNothing`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC4090FastRerouteAloneRequestsProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC4090TransitInsertsNoFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L106) | unit/verify | revert, verified |

### [`RFC4090-6-3`](#rfc4090-6-3)

- Until a PLR has a backup path available, the PLR MUST clear the relevant four flags in the corresponding RRO IPv4 or IPv6 sub- object. (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L55) | unit/verify | revert, verified |

### [`RFC4090-6-4`](#rfc4090-6-4)

"If no established one-to-one backup LSP or bypass tunnel exists, or if the one-to-one LSP and the bypass tunnel is in 'DOWN' state, the PLR MUST clear the 'local protection available' flag in its IPv4 (or IPv6) address sub-object of the RRO" (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FlagsClearUntilBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L48) | unit/verify | revert, verified |
| negative | [`TestRFC4090RefreshTracksBypassState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC4090FlagsSetOnceBypassEstablished`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L56) | unit/verify | revert, verified |

### [`RFC4090-6-5`](#rfc4090-6-5)

- The PLR MUST clear the "local protection in use" flag unless it is actively redirecting traffic into the backup path instead of along the protected LSP. (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L319) | unit/verify | revert, verified |
| positive | [`TestRFC4090InUseOnlyWhileRedirecting`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L318) | unit/verify | revert, verified |

### [`RFC4090-6-6`](#rfc4090-6-6)

Setting or clearing the "node protection" flag to match the backup path "MUST be done if the 'node protection desired' flag was set in the SESSION_ATTRIBUTE object" (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NodeBitClearWithoutNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L302) | unit/verify | revert, verified |
| positive | [`TestRFC4090NodeBitSetWhenNodeProtected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L285) | unit/verify | revert, verified |

### [`RFC4090-6-7`](#rfc4090-6-7)

Setting or clearing the "bandwidth protection" flag to match the backup path "MUST be done if the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object" (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BandwidthDesiredWithoutBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestRFC4090BandwidthBitNeverClaimed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L215) | unit/verify | revert, verified |

### [`RFC4090-6.1-1`](#rfc4090-6.1-1)

If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel. (S6.1, S6.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090HeadEndRequiresAlternateSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L387) | unit/verify | unproven |
| positive | [`TestRFC4090HeadEndUsesDistinctSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L348) | unit/verify | unproven |

### [`RFC4090-6.2-1`](#rfc4090-6.2-1)

"For bypass tunnels (Section 7), the destination MUST be the address of the MP" (S6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L365) | unit/verify | mutant, verified |
| positive | [`TestRFC4090BypassDestinationIsMergePoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L364) | unit/verify | mutant, verified |

### [`RFC4090-6.3-1`](#rfc4090-6.3-1)

When the PLR detects a failure on the protected LSP, the PLR MUST rapidly switch packets to the protected LSP's backup LSP instead of to the protected LSP's normal out-segment. (S6.3, S6.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L343) | unit/verify | revert, verified |
| positive | [`TestRFC4090FailureSwitchesToBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L342) | unit/verify | revert, verified |

### [`RFC4090-6.4-1`](#rfc4090-6.4-1)

On the backup Path message "the RSVP_HOP object MUST contain an IP source address belonging to the PLR" (S6.4, S6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L281) | unit/verify | unproven |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L144) | unit/verify | revert, verified |

### [`RFC4090-6.4-2`](#rfc4090-6.4-2)

On the backup Path message "the PLR MUST generate an EXPLICIT_ROUTE object toward the egress" (S6.4, S6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L282) | unit/verify | unproven |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L145) | unit/verify | revert, verified |

### [`RFC4090-6.4-3`](#rfc4090-6.4-3)

Before sending a Path message via a bypass tunnel the PLR MUST "remove all the sub-objects proceeding the first address belonging to the MP" and "replace this first MP address with an IP address of the MP" (S6.4, S6.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090NoBackupPathBeforeRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L283) | unit/verify | unproven |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L146) | unit/verify | revert, verified |

### [`RFC4090-7.1-1`](#rfc4090-7.1-1)

If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP. (S7.1, S7.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L298) | unit/verify | unproven |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L147) | unit/verify | revert, verified |

### [`RFC4090-7.1-2`](#rfc4090-7.1-2)

Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. (S7.1, S7.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DifferentPathsDoNotMerge`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L299) | unit/verify | unproven |
| positive | [`TestRFC4090ProtectedPathSurvivesRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_bypass_rfc4090_test.go#L148) | unit/verify | revert, verified |

### [`RFC4090-7.2-1`](#rfc4090-7.2-1)

When a downstream LSR detects a local link failure, for any protected LSPs routed over the failed link, Path and Resv state MUST NOT be cleared, and PathTear and ResvErr messages MUST NOT be sent immediately. (S7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L554) | unit/verify | revert, verified |
| positive | [`TestRFC4090DownstreamKeepsStateOnUpstreamLinkFailure`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L553) | unit/verify | revert, verified |

### [`RFC4090-7.2-2`](#rfc4090-7.2-2)

State MUST be removed if it has not been refreshed before the refresh timer expires. (S7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L594) | unit/verify | revert, verified |
| positive | [`TestRFC4090StateExpiresWithoutRefresh`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_rfc4090_test.go#L593) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4090.txt |
| Source fingerprint | d53e6ba082494f4e |
| Record | rfc/extraction/rfc4090.json |
| Mapped sentences | 30 |
| Declined as scope | 18 |
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
| `6.4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 5 head-end obligation to set the label recording desired flag, which RFC4090-5-2 already carries; this sentence says so itself ("As described in Section 6"). | As described in Section 6, the head-end LSR MUST set the "label recording requested" flag in the SESSION_ATTRIBUTE object for LSPs requesting local protection. |
| `7.1.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour merging by the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | In this case, Path state merging is REQUIRED. |
| `7.1.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour Path state recording under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | Otherwise, the MP MUST record the Path state and the incoming interface. |
| `7.1.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour merge eligibility under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | If the Path messages do not share an outgoing interface and a next-hop LSR, the MP MUST consider them to be independent LSPs and MUST NOT merge them. |
| `7.1.2:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is detour refresh after merging under the path-specific method. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically. |
| `7.1.3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | One-to-one detour backup is optional and ze declined it; this site is ResvTear handling for merged detours. RFC 4090 Section 6 makes the method optional: "A PLR MAY support the DETOUR object." | If the LSR does not have an alternate associated LSP, then the MP MUST propagate the ResvTear toward the LSP's ingress, and, for each backup LSP merged into that LSP at this LSR, the ResvTear SHOULD also be propagated along the backup LSP. |

## Superseded

No document obsoletes RFC 4090, so its obligations are stated where they were written.
