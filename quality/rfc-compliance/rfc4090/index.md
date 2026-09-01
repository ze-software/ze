# RFC 4090 - Fast Reroute Extensions to RSVP-TE for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 4090, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 12 of 12 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 12 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 12 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 26 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 12 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 26 |
| Tagged units | 26 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4090.md` |
| Requirement shard | `rfc/requirements/rfc4090.md` |
| RFC text | `rfc/full/rfc4090.txt` |

## Enrolment

Enrolled: Fast Reroute Extensions to RSVP-TE for LSP Tunnels: 12 MUSTs all MET (FAST_REROUTE/SESSION_ATTRIBUTE flags, RRO protection flags, PLR local repair, label-stacking bypass, merge-point selection, PathErr Notify, and the Section 4.2 rejection of a PATH carrying a DETOUR object at an LSR without one-to-one backup support), positive+negative tags

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Facility backup behavior. A PATH carrying a DETOUR object is rejected with a PathErr, as Section 4.2 requires of an LSR without one-to-one backup.

**What the ledger says remains:**

One-to-one detour backup was explicitly split to later work.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC4090-4.1-1`](#rfc4090-4.1-1), [`RFC4090-4.3-1`](#rfc4090-4.3-1), [`RFC4090-4.3-2`](#rfc4090-4.3-2), [`RFC4090-4.1-2`](#rfc4090-4.1-2), [`RFC4090-4.4-1`](#rfc4090-4.4-1), [`RFC4090-4.4-2`](#rfc4090-4.4-2), [`RFC4090-4.4-3`](#rfc4090-4.4-3), [`RFC4090-6.5-1`](#rfc4090-6.5-1), [`RFC4090-6.5-2`](#rfc4090-6.5-2), [`RFC4090-3.2-1`](#rfc4090-3.2-1), [`RFC4090-3.2-2`](#rfc4090-3.2-2), [`RFC4090-4.2-1`](#rfc4090-4.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4090-4.1-1` | FAST_REROUTE object uses Class-Num 205, C-Type 1, object Length 24 (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestEncodeDecodeFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L35). **negative:** `unit/verify` [`TestFastRerouteShortBody`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L49) |
| `RFC4090-4.3-1` | "Local protection desired" (0x01) set in SESSION_ATTRIBUTE when protection is requested (S4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L166). **negative:** `unit/verify` [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L181) |
| `RFC4090-4.3-2` | "Node protection desired" (0x10) set in SESSION_ATTRIBUTE for node protection (S4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168). **positive:** `unit/verify` [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63). **negative:** `unit/verify` [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) |
| `RFC4090-4.1-2` | FAST_REROUTE Flags: facility backup (0x02) or one-to-one backup (0x01) set per the requested method (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L161). **negative:** `unit/verify` [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L833) |
| `RFC4090-4.4-1` | PLR sets RRO "local protection available" (0x01) once a backup is armed (S4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L323). **negative:** `unit/verify` [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L341) |
| `RFC4090-4.4-2` | PLR sets RRO "local protection in use" (0x02) once traffic is on the backup (S4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L857). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L849) |
| `RFC4090-4.4-3` | PLR sets RRO "node protection" (0x08) when the backup protects the next node (S4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L862). **negative:** `unit/verify` [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L851) |
| `RFC4090-6.5-1` | On local repair the PLR sends PathErr Error Code 25 (Notify), Error Value sub-code 3 (Tunnel locally repaired) toward the head-end (S6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L447). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L469) |
| `RFC4090-6.5-2` | On local repair the PLR MUST NOT tear down the protected LSP (S6.5) | MUST | 6.5 | **positive:** `unit/verify` [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L421). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L465) |
| `RFC4090-3.2-1` | Facility backup pushes the bypass label on top of the protected LSP label (label stacking) (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L417). **negative:** `unit/verify` [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L462) |
| `RFC4090-3.2-2` | The merge point is the NHOP for link protection and the NNHOP for node protection (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestNodeProtectionLocalRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L750). **positive:** `unit/verify` [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L307). **negative:** `unit/verify` [`TestNodeProtectionNeedsNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L791) |
| `RFC4090-4.2-1` | An LSR that does not support the DETOUR object MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR, generated as [RSVP] specifies for unknown objects with a Class-Num of the form "0bbbbbbb" (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L885). **negative:** `unit/verify` [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L904) |
| `RFC4090-6.5-3` | The head-end re-optimizes (make-before-break) onto a fresh path after a Notify and tears the repaired LSP (S6.5) | SHOULD | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4090-4.4-4` | DETOUR object (Class-Num 63) signals one-to-one backup detour LSPs (S4.4) | MAY | 4.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4090 declares no gap, and every gated MUST it carries has a test bound to it.

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

"Local protection desired" (0x01) set in SESSION_ATTRIBUTE when protection is requested (S4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPathNoProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L181) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L166) | unit/verify | unproven |

### [`RFC4090-4.3-2`](#rfc4090-4.3-2)

"Node protection desired" (0x10) set in SESSION_ATTRIBUTE for node protection (S4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionAttributeEmptyName`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L91) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L168) | unit/verify | unproven |
| positive | [`TestSessionAttributeProtectionFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L63) | unit/verify | unproven |

### [`RFC4090-4.1-2`](#rfc4090-4.1-2)

FAST_REROUTE Flags: facility backup (0x02) or one-to-one backup (0x01) set per the requested method (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFastRerouteOneToOneMethodFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L833) | unit/verify | unproven |
| positive | [`TestBuildPathIncludesFastReroute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L161) | unit/verify | unproven |

### [`RFC4090-4.4-1`](#rfc4090-4.4-1)

PLR sets RRO "local protection available" (0x01) once a backup is armed (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPLRNoBypassWithoutProtection`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L341) | unit/verify | unproven |
| positive | [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L323) | unit/verify | unproven |

### [`RFC4090-4.4-2`](#rfc4090-4.4-2)

PLR sets RRO "local protection in use" (0x02) once traffic is on the backup (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L849) | unit/verify | unproven |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L857) | unit/verify | unproven |

### [`RFC4090-4.4-3`](#rfc4090-4.4-3)

PLR sets RRO "node protection" (0x08) when the backup protects the next node (S4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L851) | unit/verify | unproven |
| positive | [`TestRROProtectionFlagsReflectState`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L862) | unit/verify | unproven |

### [`RFC4090-6.5-1`](#rfc4090-6.5-1)

On local repair the PLR sends PathErr Error Code 25 (Notify), Error Value sub-code 3 (Tunnel locally repaired) toward the head-end (S6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L469) | unit/verify | unproven |
| positive | [`TestLocalRepairSendsNotify`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L447) | unit/verify | unproven |

### [`RFC4090-6.5-2`](#rfc4090-6.5-2)

On local repair the PLR MUST NOT tear down the protected LSP (S6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L465) | unit/verify | unproven |
| positive | [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L421) | unit/verify | unproven |

### [`RFC4090-3.2-1`](#rfc4090-3.2-1)

Facility backup pushes the bypass label on top of the protected LSP label (label stacking) (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalRepairFallsBackToTeardown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L462) | unit/verify | unproven |
| positive | [`TestLocalRepairSwitchesFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L417) | unit/verify | unproven |

### [`RFC4090-3.2-2`](#rfc4090-3.2-2)

The merge point is the NHOP for link protection and the NNHOP for node protection (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNodeProtectionNeedsNodeBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L791) | unit/verify | unproven |
| positive | [`TestNodeProtectionLocalRepair`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L750) | unit/verify | unproven |
| positive | [`TestPLRArmsBypass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L307) | unit/verify | unproven |

### [`RFC4090-4.2-1`](#rfc4090-4.2-1)

An LSR that does not support the DETOUR object MUST reject any Path message containing a DETOUR object and send a PathErr to notify the PLR, generated as [RSVP] specifies for unknown objects with a Class-Num of the form "0bbbbbbb" (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L904) | unit/verify | unproven |
| positive | [`TestEnginePathWithDetourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L885) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 4090, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4090, so its obligations are stated where they were written.
