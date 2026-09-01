# RFC 3101 - The OSPF Not-So-Stubby Area (NSSA) Option

Experimental. Every requirement this repository extracted from RFC 3101, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 16 of 16 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 16 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 16 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 52 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 16 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 20 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 52 |
| Tagged units | 52 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3101.md` |
| Requirement shard | `rfc/requirements/rfc3101.md` |
| RFC text | `rfc/full/rfc3101.txt` |

## Enrolment

Enrolled: OSPF NSSA (RFC 3101): 13 MET (N/E-bit Hello negotiation, Type-7 origination/flood-scope, P-bit boundary policy, ASBR E-bit, Type-3 import, translator election, Type-7->Type-5 translation, highest-RID duplicate suppression) + 2 gap (install-side default P-gate, unconditional default into every NSSA)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Type-7 origination/flooding, redistribution into NSSA, N/E-bit Hello negotiation, Router-LSA Nt/E/B flags, translator election (Nt-bit candidates, highest-RID, always/never roles, stability grace), Type-7 to Type-5 translation with FA/metric/tag preservation and highest-RID duplicate suppression, source preference, Type-3 summary import policy. For both address families: mandatory border-router defaults with no operator gate, no-summary defaults through the summary path (Type-3 for OSPFv2, Inter-Area-Prefix for OSPFv3), and the P-bit and suppressed-summary-import gates on installing a received Type-7 default.

**What the ledger says remains**

The Section 2.4 default-route origination now dispatches on address family in `applyNSSADefaults` ([`internal/plugins/ospf/nssa.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa.go)), so an OSPFv3 NSSA border router originates the 0x2007 NSSA-LSA that RFC 5340 Section 4.4.3.7 defines rather than the OSPFv2 0x0007. Both halves are unit-proven in each family, and OSPFv2 origination is additionally proven against FRR by `test/interop/scenarios/ospf-stub-nssa-frr`. Two things are still owed, tracked by [`plan/spec-ospf-rfc3101-nssa-defaults.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ospf-rfc3101-nssa-defaults.md): OSPFv3 has unit evidence only, with no interop scenario and no functional `.ci` driving a running daemon; and three sites compute NSSA border-router status independently, so the advertised Router-LSA B-bit and the originated default can disagree across a backbone transition. Note that the [`rfc/short/rfc3101.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3101.md) checklist cannot record the difference: its requirement ids carry no address-family dimension, so a tagged test on either path satisfies it for both. Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC3101-2.1-1`](#rfc3101-2.1-1), [`RFC3101-2.1-2`](#rfc3101-2.1-2), [`RFC3101-x-1`](#rfc3101-x-1), [`RFC3101-2.3-1`](#rfc3101-2.3-1), [`RFC3101-2.3-2`](#rfc3101-2.3-2), [`RFC3101-2.4-1`](#rfc3101-2.4-1), [`RFC3101-2.4-2`](#rfc3101-2.4-2), [`RFC3101-2.4-3`](#rfc3101-2.4-3), [`RFC3101-2.4-4`](#rfc3101-2.4-4), [`RFC3101-2.4-5`](#rfc3101-2.4-5), [`RFC3101-2.5-1`](#rfc3101-2.5-1), [`RFC3101-3.1-1`](#rfc3101-3.1-1), [`RFC3101-2.7-1`](#rfc3101-2.7-1), [`RFC3101-3.1-2`](#rfc3101-3.1-2), [`RFC3101-3.2-1`](#rfc3101-3.2-1), [`RFC3101-3.2-2`](#rfc3101-3.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3101-2.1-1` | Verify N-bit and E-bit in received Hellos match the area type before adjacency (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L46). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L68) |
| `RFC3101-2.1-2` | Refuse adjacency unless both routers agree on the N-bit (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L48). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L59) |
| `RFC3101-x-1` | Keep the E-bit clear whenever the N-bit is set (Appendix A) | MUST | x | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L50). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L70) |
| `RFC3101-2.3-1` | Originate Type-7 NSSA-LSAs with LS Type value 7 (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L33). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L49) |
| `RFC3101-2.3-2` | Flood Type-7 LSAs only within the originating NSSA (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L86). **negative:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L92) |
| `RFC3101-2.4-1` | Set the P-bit on Type-7 LSAs an NSSA internal ASBR wants in the transit topology (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L35). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L57) |
| `RFC3101-2.4-2` | Ensure a non-zero forwarding address whenever the P-bit is set; otherwise do not originate the Type-7 LSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L32). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L24). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L167) |
| `RFC3101-2.4-3` | Clear the P-bit on a Type-7 LSA when the same network is also originated as a Type-5 LSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L34). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L42) |
| `RFC3101-2.4-4` | Clear the P-bit on a Type-7 default LSA originated by an NSSA border router; install a Type-7 default only if its P-bit is set (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L78). **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L37). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L71). **negative:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L111). **negative:** `unit/verify` [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L187) |
| `RFC3101-2.4-5` | Originate a default-destination LSA into every directly attached NSSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L31). **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L124). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L101). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L62). **negative:** `unit/verify` [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L52). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L169). **positive:** `interop/nightly` [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1263) |
| `RFC3101-2.5-1` | Ignore Type-7 default LSAs on an NSSA border router that suppresses Type-3 summary import (Section 2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L94). **negative:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L80) |
| `RFC3101-3.1-1` | Set the E-bit in Type-1 router-LSAs of directly attached non-stub areas (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L90). **negative:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L76) |
| `RFC3101-2.7-1` | Support optional import of summary routes into NSSAs as Type-3 summary-LSAs (Section 2.7) | MUST | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L85). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L94) |
| `RFC3101-3.1-2` | Elect the translator as the reachable NSSA border router with Nt set or the highest Router ID (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFNSSANonCandidateDoesNotWedge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L235). **positive:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L34). **negative:** `unit/verify` [`TestOSPFNSSANoTranslateWhenNotElected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L215). **negative:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L37) |
| `RFC3101-3.2-1` | In translation, set the advertising router to the translator's Router ID and preserve mask, path type, metric, forwarding address, and route tag (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L83). **negative:** `unit/verify` [`TestOSPFNSSAPbitNotTranslated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L143). **negative:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L97) |
| `RFC3101-3.2-2` | Suppress duplicate translation: translate only if this router has the highest Router ID among translators advertising a functionally equivalent Type-5 LSA (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L36). **positive:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L118). **negative:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L28). **negative:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L128) |
| `RFC3101-x-2` | Honor the TranslatorStabilityInterval (default 40 s) before relinquishing translator duties (Appendix D) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.4-6` | Originate Type-4 summary-LSAs into an NSSA (Section 2.4) | SHOULD NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.7-2` | Originate a Type-3 summary-LSA as the NSSA default when summary import is disabled (no-summary NSSA) (Section 2.7) | SHOULD | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L122). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L99). **positive:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L105). **negative:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L143). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L90). **negative:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L99) |
| `RFC3101-2.2-1` | Aggregate Type-7 routes into one Type-5 LSA per configured Type-7 address range, with a 0.0.0.0 forwarding address (Section 2.2, Section 3.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 3101 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3101-2.1-1`](#rfc3101-2.1-1)

Verify N-bit and E-bit in received Hellos match the area type before adjacency (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L68) | unit/verify | unproven |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L46) | unit/verify | unproven |

### [`RFC3101-2.1-2`](#rfc3101-2.1-2)

Refuse adjacency unless both routers agree on the N-bit (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L59) | unit/verify | unproven |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L48) | unit/verify | unproven |

### [`RFC3101-x-1`](#rfc3101-x-1)

Keep the E-bit clear whenever the N-bit is set (Appendix A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L70) | unit/verify | unproven |
| positive | [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L50) | unit/verify | unproven |

### [`RFC3101-2.3-1`](#rfc3101-2.3-1)

Originate Type-7 NSSA-LSAs with LS Type value 7 (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L49) | unit/verify | unproven |
| positive | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L33) | unit/verify | unproven |

### [`RFC3101-2.3-2`](#rfc3101-2.3-2)

Flood Type-7 LSAs only within the originating NSSA (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L92) | unit/verify | unproven |
| positive | [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L86) | unit/verify | unproven |

### [`RFC3101-2.4-1`](#rfc3101-2.4-1)

Set the P-bit on Type-7 LSAs an NSSA internal ASBR wants in the transit topology (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L57) | unit/verify | unproven |
| positive | [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L35) | unit/verify | unproven |

### [`RFC3101-2.4-2`](#rfc3101-2.4-2)

Ensure a non-zero forwarding address whenever the P-bit is set; otherwise do not originate the Type-7 LSA (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L24) | unit/verify | unproven |
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L167) | unit/verify | unproven |
| positive | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L32) | unit/verify | unproven |

### [`RFC3101-2.4-3`](#rfc3101-2.4-3)

Clear the P-bit on a Type-7 LSA when the same network is also originated as a Type-5 LSA (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L42) | unit/verify | unproven |
| positive | [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L34) | unit/verify | unproven |

### [`RFC3101-2.4-4`](#rfc3101-2.4-4)

Clear the P-bit on a Type-7 default LSA originated by an NSSA border router; install a Type-7 default only if its P-bit is set (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L187) | unit/verify | unproven |
| negative | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L111) | unit/verify | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L37) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L71) | unit/verify | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L78) | unit/verify | unproven |

### [`RFC3101-2.4-5`](#rfc3101-2.4-5)

Originate a default-destination LSA into every directly attached NSSA (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L52) | unit/verify | unproven |
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L169) | unit/verify | unproven |
| positive | [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1263) | interop/nightly | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L62) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L124) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L101) | unit/verify | unproven |

### [`RFC3101-2.5-1`](#rfc3101-2.5-1)

Ignore Type-7 default LSAs on an NSSA border router that suppresses Type-3 summary import (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L80) | unit/verify | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L94) | unit/verify | unproven |

### [`RFC3101-3.1-1`](#rfc3101-3.1-1)

Set the E-bit in Type-1 router-LSAs of directly attached non-stub areas (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L76) | unit/verify | unproven |
| positive | [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L90) | unit/verify | unproven |

### [`RFC3101-2.7-1`](#rfc3101-2.7-1)

Support optional import of summary routes into NSSAs as Type-3 summary-LSAs (Section 2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L94) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L85) | unit/verify | unproven |

### [`RFC3101-3.1-2`](#rfc3101-3.1-2)

Elect the translator as the reachable NSSA border router with Nt set or the highest Router ID (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSANoTranslateWhenNotElected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L215) | unit/verify | unproven |
| negative | [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L37) | unit/verify | unproven |
| positive | [`TestOSPFNSSANonCandidateDoesNotWedge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L235) | unit/verify | unproven |
| positive | [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L34) | unit/verify | unproven |

### [`RFC3101-3.2-1`](#rfc3101-3.2-1)

In translation, set the advertising router to the translator's Router ID and preserve mask, path type, metric, forwarding address, and route tag (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAPbitNotTranslated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L143) | unit/verify | unproven |
| negative | [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L97) | unit/verify | unproven |
| positive | [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L83) | unit/verify | unproven |

### [`RFC3101-3.2-2`](#rfc3101-3.2-2)

Suppress duplicate translation: translate only if this router has the highest Router ID among translators advertising a functionally equivalent Type-5 LSA (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L28) | unit/verify | unproven |
| negative | [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L128) | unit/verify | unproven |
| positive | [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L36) | unit/verify | unproven |
| positive | [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L118) | unit/verify | unproven |

### [`RFC3101-2.7-2`](#rfc3101-2.7-2)

Originate a Type-3 summary-LSA as the NSSA default when summary import is disabled (no-summary NSSA) (Section 2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L99) | unit/verify | unproven |
| negative | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L143) | unit/verify | unproven |
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L90) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L105) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L122) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L99) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 3101, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3101, so its obligations are stated where they were written.
