# RFC 5187 - OSPFv3 Graceful Restart

Experimental. Every requirement this repository extracted from RFC 5187, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 4 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5187.md` |
| Requirement shard | `rfc/requirements/rfc5187.md` |
| RFC text | `rfc/full/rfc5187.txt` |

## Enrolment

Enrolled: OSPFv3 Graceful Restart: four MUST-level requirements, all met and tested with both polarities. RFC5187-2.2-1 (Grace Period TLV always in a grace-LSA) and RFC5187-2.2-2 (Restart Reason TLV always present) via the shared grace-LSA body codec (internal/plugins/ospf/packet/grace_lsa.go EncodeGraceLSA always emits both, DecodeGraceLSA rejects a body missing either): TestGraceLSARoundTrip (both TLVs round-trip) and TestGraceLSADecodeMissingMandatory (a Reason-only or Period-only body is rejected). Ze originates the OSPFv3 link-scoped grace-LSA (LS Type 0x000B, gr_restarter.go:295-302). RFC5187-3.1-1 (preserve LSA-ID to prefix correspondence across restart) and RFC5187-3.2-1 (preserve OSPFv3 Interface ID across restart) via the NVS preservation maps PrefixLSIDs/InterfaceIDs (internal/plugins/ospf/gr_nvs.go:41-45): TestRestartFactPersistsAcrossRestart (maps read back intact after a restart) and TestStaleRestartFactIgnored (an expired/cleared restart fact is inactive, so stale IDs are not restored). No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Restarter and helper behavior for OSPFv3.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5187-2.2-1`](#rfc5187-2.2-1), [`RFC5187-2.2-2`](#rfc5187-2.2-2), [`RFC5187-3.1-1`](#rfc5187-3.1-1), [`RFC5187-3.2-1`](#rfc5187-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5187-2.2-1` | Grace Period TLV (Type=1, Length=4): "This TLV MUST always appear in a grace-LSA" (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L12). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L65) |
| `RFC5187-2.2-2` | Graceful Restart Reason TLV (Type=2, Length=1): "This TLV MUST always appear in a grace-LSA" (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L16). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L69) |
| `RFC5187-3.1-1` | "the restarting router MUST preserve the LSA ID to prefix correspondence across graceful restarts" (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L43). **negative:** `unit/verify` [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L90) |
| `RFC5187-3.2-1` | "the OSPFv3 Interface ID, as described in section 3.1.2 of [OSPFv3], MUST be preserved by the restarting router across restarts" (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L46). **negative:** `unit/verify` [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L94) |

## Gaps and untested MUSTs

RFC 5187 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5187-2.2-1`](#rfc5187-2.2-1)

Grace Period TLV (Type=1, Length=4): "This TLV MUST always appear in a grace-LSA" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L65) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L12) | unit/verify | unproven |

### [`RFC5187-2.2-2`](#rfc5187-2.2-2)

Graceful Restart Reason TLV (Type=2, Length=1): "This TLV MUST always appear in a grace-LSA" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L69) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L16) | unit/verify | unproven |

### [`RFC5187-3.1-1`](#rfc5187-3.1-1)

"the restarting router MUST preserve the LSA ID to prefix correspondence across graceful restarts" (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L90) | unit/verify | unproven |
| positive | [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L43) | unit/verify | unproven |

### [`RFC5187-3.2-1`](#rfc5187-3.2-1)

"the OSPFv3 Interface ID, as described in section 3.1.2 of [OSPFv3], MUST be preserved by the restarting router across restarts" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L94) | unit/verify | unproven |
| positive | [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_nvs_test.go#L46) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5187, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5187, so its obligations are stated where they were written.
