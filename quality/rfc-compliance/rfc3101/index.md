# RFC 3101 - The OSPF Not-So-Stubby Area (NSSA) Option

Experimental. Every requirement this repository extracted from RFC 3101, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 68.0% | 17 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 20.0% | 13 of 65 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 32.0% | 8 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 30 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 8 |
| Nightly-only evidence | 0 |
| Test tags | 65 |
| Tagged units | 65 |
| Recorded audit verdicts | 0 |
| Discrimination records | 13 |
| Summary | `rfc/short/rfc3101.md` |
| Requirement shard | `rfc/requirements/rfc3101.md` |
| RFC text | `rfc/full/rfc3101.txt` |

## Enrolment

Enrolled: OSPF NSSA (RFC 3101): 13 MET (N/E-bit Hello negotiation, Type-7 origination/flood-scope, P-bit boundary policy, ASBR E-bit, Type-3 import, translator election, Type-7->Type-5 translation, highest-RID duplicate suppression) + 2 gap (install-side default P-gate, unconditional default into every NSSA). The 2026-09-21 extraction walk read the document against this checklist and added 9 rows it did not carry: RFC3101-2.3-3 (re-originate the Type-7 LSA with a new forwarding address when the forwarding address's interface goes Down), RFC3101-2.5-2 through -5 (the four Section 2.5 rules on which routing table entry an ASBR match may use, and when the calculation runs), RFC3101-3.1-3 (translation by several border routers is not recommended), and RFC3101-x-3 through -x-5 (the Appendix D configuration vehicles for the P-bit, for ExternalRoutingCapability and for the default LSA's metric). None of the 9 carries a tagged test, so each is an open gap.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Type-7 origination/flooding, redistribution into NSSA, N/E-bit Hello negotiation, Router-LSA Nt/E/B flags, translator election (Nt-bit candidates, highest-RID, always/never roles, stability grace), Type-7 to Type-5 translation with FA/metric/tag preservation and highest-RID duplicate suppression, source preference, Type-3 summary import policy. For both address families: mandatory border-router defaults with no operator gate, no-summary defaults through the summary path (Type-3 for OSPFv2, Inter-Area-Prefix for OSPFv3), no summary-LSA default where summary routes ARE imported (Section 2.7 MUST NOT), an internal router's `default-originate` held inert in a no-summary NSSA (Section 1.3 mutual exclusivity), and the P-bit and suppressed-summary-import gates on installing a received Type-7 default, each gate proven permissive on a router that is not an NSSA border router.

**What the ledger says remains**

The Section 2.4 default-route origination dispatches on address family in `applyNSSADefaults` ([`internal/plugins/ospf/nssa.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa.go)), so an OSPFv3 NSSA border router originates the 0x2007 NSSA-LSA that RFC 5340 Section 4.4.3.7 defines rather than the OSPFv2 0x0007. Both halves are unit-proven in each family, both families reach the operator's `show ospf database nssa-external` subview under [`test/ospf/ospf-nssa-abr-default.ci`](https://github.com/ze-software/ze/blob/main/test/ospf/ospf-nssa-abr-default.ci) and [`test/ospfv3/ospfv3-nssa-abr-default.ci`](https://github.com/ze-software/ze/blob/main/test/ospfv3/ospfv3-nssa-abr-default.ci), and OSPFv2 origination is additionally proven against FRR by `test/interop/scenarios/ospf-stub-nssa-frr`. Two things are still owed, tracked by [`plan/immediate/spec-ospf-rfc3101-nssa-defaults.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-ospf-rfc3101-nssa-defaults.md): OSPFv3 has no interop scenario, so no peer daemon has read a Ze OSPFv3 NSSA default, and neither family has an interop scenario for the two install-side gates; and three sites compute NSSA border-router status independently, so the advertised Router-LSA B-bit and the originated default can disagree across a backbone transition. Section 2.2 Type-7 address-range aggregation into one Type-5 stays unimplemented; it is a MAY. Note that the [`rfc/short/rfc3101.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3101.md) checklist cannot record an address-family difference: its requirement ids carry no address-family dimension, so a tagged test on either path satisfies it for both. Same OSPF experimental status. Nine further rows were added by the 2026-09-21 extraction walk and none is proven: [`RFC3101-2.3-3`](#rfc3101-2.3-3), [`RFC3101-2.5-2`](#rfc3101-2.5-2), [`RFC3101-2.5-3`](#rfc3101-2.5-3), [`RFC3101-2.5-4`](#rfc3101-2.5-4), [`RFC3101-2.5-5`](#rfc3101-2.5-5), [`RFC3101-3.1-3`](#rfc3101-3.1-3) and RFC3101-x-3 through RFC3101-x-5.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 17 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 8 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (17):** [`RFC3101-2.1-1`](#rfc3101-2.1-1), [`RFC3101-2.1-2`](#rfc3101-2.1-2), [`RFC3101-x-1`](#rfc3101-x-1), [`RFC3101-2.3-1`](#rfc3101-2.3-1), [`RFC3101-2.3-2`](#rfc3101-2.3-2), [`RFC3101-2.4-1`](#rfc3101-2.4-1), [`RFC3101-2.4-2`](#rfc3101-2.4-2), [`RFC3101-2.4-3`](#rfc3101-2.4-3), [`RFC3101-2.4-4`](#rfc3101-2.4-4), [`RFC3101-2.4-5`](#rfc3101-2.4-5), [`RFC3101-2.5-1`](#rfc3101-2.5-1), [`RFC3101-3.1-1`](#rfc3101-3.1-1), [`RFC3101-2.7-1`](#rfc3101-2.7-1), [`RFC3101-3.1-2`](#rfc3101-3.1-2), [`RFC3101-3.2-1`](#rfc3101-3.2-1), [`RFC3101-3.2-2`](#rfc3101-3.2-2), [`RFC3101-2.7-3`](#rfc3101-2.7-3)

**No test and no annotation (8):** [`RFC3101-2.3-3`](#rfc3101-2.3-3), [`RFC3101-2.5-2`](#rfc3101-2.5-2), [`RFC3101-2.5-3`](#rfc3101-2.5-3), [`RFC3101-2.5-4`](#rfc3101-2.5-4), [`RFC3101-2.5-5`](#rfc3101-2.5-5), [`RFC3101-x-3`](#rfc3101-x-3), [`RFC3101-x-4`](#rfc3101-x-4), [`RFC3101-x-5`](#rfc3101-x-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3101-2.1-1` | Verify N-bit and E-bit in received Hellos match the area type before adjacency (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L46). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L68) |
| `RFC3101-2.1-2` | Refuse adjacency unless both routers agree on the N-bit (Section 2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L48). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L59) |
| `RFC3101-x-1` | Keep the E-bit clear whenever the N-bit is set (Appendix A) | MUST | x | **positive:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L50). **negative:** `unit/verify` [`TestOSPFNSSANbitMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/iface/hello_nssa_test.go#L70) |
| `RFC3101-2.3-1` | Originate Type-7 NSSA-LSAs with LS Type value 7 (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L33). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L49) |
| `RFC3101-2.3-2` | Flood Type-7 LSAs only within the originating NSSA (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L86). **negative:** `unit/verify` [`TestOSPFType7FloodScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L92) |
| `RFC3101-2.4-1` | Set the P-bit on Type-7 LSAs an NSSA internal ASBR wants in the transit topology (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L35). **negative:** `unit/verify` [`TestOSPFType7Origination`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_test.go#L57) |
| `RFC3101-2.4-2` | Ensure a non-zero forwarding address whenever the P-bit is set; otherwise do not originate the Type-7 LSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L32). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L24). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L171) |
| `RFC3101-2.4-3` | Clear the P-bit on a Type-7 LSA when the same network is also originated as a Type-5 LSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L34). **negative:** `unit/verify` [`TestOSPFNSSAPBitBoundaryPolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_pbit_test.go#L42) |
| `RFC3101-2.4-4` | Clear the P-bit on a Type-7 default LSA originated by an NSSA border router; install a Type-7 default only if its P-bit is set (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L78). **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L37). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L81). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L71). **negative:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L111). **negative:** `unit/verify` [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L138). **negative:** `unit/verify` [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L90). **negative:** `unit/verify` [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L191). **negative:** `unit/verify` [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L112) |
| `RFC3101-2.4-5` | Originate a default-destination LSA into every directly attached NSSA (Section 2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L31). **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L128). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L103). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L62). **negative:** `unit/verify` [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L52). **negative:** `unit/verify` [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L173). **positive:** `interop/nightly` [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1263) |
| `RFC3101-2.5-1` | Ignore Type-7 default LSAs on an NSSA border router that suppresses Type-3 summary import (Section 2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L94). **positive:** `unit/verify` [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L96). **negative:** `unit/verify` [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L80). **negative:** `unit/verify` [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L140). **negative:** `unit/verify` [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L114) |
| `RFC3101-2.3-3` | "When the interface whose IP address is the LSA's forwarding address transitions to a Down state (see [OSPF] Section 9.3), the router must select a new forwarding address for the LSA and then re-originate it" (Section 2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.5-2` | "Since the flooding scope of a Type-7 LSA is restricted to the originating NSSA, the routing table entry of its ASBR must be found in the originating NSSA" (Section 2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.5-3` | "For a Type-5 LSA the matching routing table entry must specify an intra-area or inter-area path through a Type-5 capable area" (Section 2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.5-4` | "For a Type-7 LSA the matching routing table entry must specify an intra-area path through the LSA's originating NSSA" (Section 2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.5-5` | The NSSA ASBR routing table calculation "must be run when Type-7 LSAs are processed during the AS external route calculation" (Section 2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-3.1-1` | Set the E-bit in Type-1 router-LSAs of directly attached non-stub areas (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L90). **negative:** `unit/verify` [`TestOSPFASBRBitFromNSSAType7`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/origination_external_test.go#L76) |
| `RFC3101-2.7-1` | Support optional import of summary routes into NSSAs as Type-3 summary-LSAs (Section 2.7) | MUST | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L85). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L96) |
| `RFC3101-3.1-2` | Elect the translator as the reachable NSSA border router with Nt set or the highest Router ID (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFNSSANonCandidateDoesNotWedge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L235). **positive:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L34). **negative:** `unit/verify` [`TestOSPFNSSANoTranslateWhenNotElected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L215). **negative:** `unit/verify` [`TestOSPFNSSATranslatorElection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L37) |
| `RFC3101-3.2-1` | In translation, set the advertising router to the translator's Router ID and preserve mask, path type, metric, forwarding address, and route tag (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L83). **negative:** `unit/verify` [`TestOSPFNSSAPbitNotTranslated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L143). **negative:** `unit/verify` [`TestOSPFNSSATranslation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_test.go#L97) |
| `RFC3101-3.2-2` | Suppress duplicate translation: translate only if this router has the highest Router ID among translators advertising a functionally equivalent Type-5 LSA (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L36). **positive:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L118). **negative:** `unit/verify` [`TestOSPFHigherRIDType5Exists`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/nssa_higher_rid_test.go#L28). **negative:** `unit/verify` [`TestOSPFNSSAHigherRIDType5Suppresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L128) |
| `RFC3101-x-3` | "Implementations must provide a vehicle for setting the P-bit when external routes are imported into the NSSA as Type-7 LSAs" (Appendix D) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-x-4` | "For NSSAs the ExternalRoutingCapability area configuration parameter must be set to accept Type-7 external routes" (Appendix D) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-x-5` | "Additionally there must be a way of configuring the metric of the default LSA that a border router advertises into its directly attached NSSAs" (Appendix D) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-x-2` | Honor the TranslatorStabilityInterval (default 40 s) before relinquishing translator duties (Appendix D) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-3.1-3` | "It is not recommended that multiple NSSA border routers perform Type-7 to Type-5 translation unless it is required to route packets efficiently through Area 0 to an NSSA partitioned by Type-7 address ranges" (Section 3.1) | NOT RECOMMENDED | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.4-6` | Originate Type-4 summary-LSAs into an NSSA (Section 2.4) | SHOULD NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3101-2.7-2` | Originate a Type-3 summary-LSA as the NSSA default when summary import is disabled (no-summary NSSA) (Section 2.7) | SHOULD | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L126). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L101). **positive:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L105). **negative:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L149). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L90). **negative:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L99) |
| `RFC3101-2.7-3` | Originate the NSSA default as a Type-3 summary-LSA when summary routes ARE imported (Section 2.7) | MUST NOT | 2.7 | **positive:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L151). **positive:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L92). **positive:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L117). **negative:** `unit/verify` [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L130). **negative:** `unit/verify` [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L105). **negative:** `unit/verify` [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L108) |
| `RFC3101-2.2-1` | Aggregate Type-7 routes into one Type-5 LSA per configured Type-7 address range, with a 0.0.0.0 forwarding address (Section 2.2, Section 3.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3101-2.3-3`](#rfc3101-2.3-3) "When the interface whose IP address is the LSA's forwarding address transitions to a Down state (see [OSPF] Section 9.3), the router must select a new forwarding address for the LSA and then re-originate it" (Section 2.3) | no test | no test carries this requirement id |
| [`RFC3101-2.5-2`](#rfc3101-2.5-2) "Since the flooding scope of a Type-7 LSA is restricted to the originating NSSA, the routing table entry of its ASBR must be found in the originating NSSA" (Section 2.5) | no test | no test carries this requirement id |
| [`RFC3101-2.5-3`](#rfc3101-2.5-3) "For a Type-5 LSA the matching routing table entry must specify an intra-area or inter-area path through a Type-5 capable area" (Section 2.5) | no test | no test carries this requirement id |
| [`RFC3101-2.5-4`](#rfc3101-2.5-4) "For a Type-7 LSA the matching routing table entry must specify an intra-area path through the LSA's originating NSSA" (Section 2.5) | no test | no test carries this requirement id |
| [`RFC3101-2.5-5`](#rfc3101-2.5-5) The NSSA ASBR routing table calculation "must be run when Type-7 LSAs are processed during the AS external route calculation" (Section 2.5) | no test | no test carries this requirement id |
| [`RFC3101-x-3`](#rfc3101-x-3) "Implementations must provide a vehicle for setting the P-bit when external routes are imported into the NSSA as Type-7 LSAs" (Appendix D) | no test | no test carries this requirement id |
| [`RFC3101-x-4`](#rfc3101-x-4) "For NSSAs the ExternalRoutingCapability area configuration parameter must be set to accept Type-7 external routes" (Appendix D) | no test | no test carries this requirement id |
| [`RFC3101-x-5`](#rfc3101-x-5) "Additionally there must be a way of configuring the metric of the default LSA that a border router advertises into its directly attached NSSAs" (Appendix D) | no test | no test carries this requirement id |

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
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L171) | unit/verify | unproven |
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
| negative | [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L90) | unit/verify | revert, verified |
| negative | [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestOSPFv3NSSADefaultPBitFollowsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L191) | unit/verify | unproven |
| negative | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L111) | unit/verify | unproven |
| negative | [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L138) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L37) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L71) | unit/verify | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L78) | unit/verify | unproven |

### [`RFC3101-2.4-5`](#rfc3101-2.4-5)

Originate a default-destination LSA into every directly attached NSSA (Section 2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFNSSAInternalRouterOriginatesNoBorderDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L52) | unit/verify | unproven |
| negative | [`TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L173) | unit/verify | unproven |
| positive | [`checkNSSADefault`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1263) | interop/nightly | unproven |
| positive | [`TestOSPFNSSABorderRouterDefaultsEveryArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_ac14_16_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSABorderRouterOriginatesDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L62) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L128) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L103) | unit/verify | unproven |

### [`RFC3101-2.5-1`](#rfc3101-2.5-1)

Ignore Type-7 default LSAs on an NSSA border router that suppresses Type-3 summary import (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L80) | unit/verify | unproven |
| negative | [`TestOSPFNSSANonBorderRouterInstallsPClearDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestOSPFv3NSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/nssa_install_gate_v6_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSABorderRouterDefaultPBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/external_nssa_test.go#L94) | unit/verify | unproven |

### [`RFC3101-2.3-3`](#rfc3101-2.3-3)

"When the interface whose IP address is the LSA's forwarding address transitions to a Down state (see [OSPF] Section 9.3), the router must select a new forwarding address for the LSA and then re-originate it" (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-2.3-3, so no unit is bound to it.

### [`RFC3101-2.5-2`](#rfc3101-2.5-2)

"Since the flooding scope of a Type-7 LSA is restricted to the originating NSSA, the routing table entry of its ASBR must be found in the originating NSSA" (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-2.5-2, so no unit is bound to it.

### [`RFC3101-2.5-3`](#rfc3101-2.5-3)

"For a Type-5 LSA the matching routing table entry must specify an intra-area or inter-area path through a Type-5 capable area" (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-2.5-3, so no unit is bound to it.

### [`RFC3101-2.5-4`](#rfc3101-2.5-4)

"For a Type-7 LSA the matching routing table entry must specify an intra-area path through the LSA's originating NSSA" (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-2.5-4, so no unit is bound to it.

### [`RFC3101-2.5-5`](#rfc3101-2.5-5)

The NSSA ASBR routing table calculation "must be run when Type-7 LSAs are processed during the AS external route calculation" (Section 2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-2.5-5, so no unit is bound to it.

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
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L96) | unit/verify | unproven |
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

### [`RFC3101-x-3`](#rfc3101-x-3)

"Implementations must provide a vehicle for setting the P-bit when external routes are imported into the NSSA as Type-7 LSAs" (Appendix D)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-x-3, so no unit is bound to it.

### [`RFC3101-x-4`](#rfc3101-x-4)

"For NSSAs the ExternalRoutingCapability area configuration parameter must be set to accept Type-7 external routes" (Appendix D)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-x-4, so no unit is bound to it.

### [`RFC3101-x-5`](#rfc3101-x-5)

"Additionally there must be a way of configuring the metric of the default LSA that a border router advertises into its directly attached NSSAs" (Appendix D)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3101-x-5, so no unit is bound to it.

### [`RFC3101-2.7-2`](#rfc3101-2.7-2)

Originate a Type-3 summary-LSA as the NSSA default when summary import is disabled (no-summary NSSA) (Section 2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L99) | unit/verify | unproven |
| negative | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L149) | unit/verify | unproven |
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L90) | unit/verify | unproven |
| positive | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L105) | unit/verify | unproven |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L126) | unit/verify | unproven |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L101) | unit/verify | unproven |

### [`RFC3101-2.7-3`](#rfc3101-2.7-3)

Originate the NSSA default as a Type-3 summary-LSA when summary routes ARE imported (Section 2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L108) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_nssa_default_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSANoSummaryDefaultInjection`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestOSPFNSSAType3SummaryImport`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/area_type_test.go#L92) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3101.txt |
| Source fingerprint | 7061642290964ece |
| Record | rfc/extraction/rfc3101.json |
| Mapped sentences | 25 |
| Declined as scope | 15 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1.0` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 1 | walked | not stated |
| `1.2` | not stated | 1 | walked | not stated |
| `1.3` | not stated | 3 | walked | not stated |
| `2.0` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 5 | walked | not stated |
| `2.4` | not stated | 5 | walked | not stated |
| `2.5` | not stated | 5 | walked | not stated |
| `2.6` | not stated | 0 | walked | not stated |
| `2.7` | not stated | 2 | walked | not stated |
| `3.0` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4.0` | not stated | 1 | walked | not stated |
| `5.0` | not stated | 0 | walked | not stated |
| `6.0` | not stated | 0 | walked | not stated |
| `7.0` | not stated | 9 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 is the transit-network motivation: the sentence describes the deployment problem NSSA exists to solve and binds no implementation. | In order to run OSPF out to BR18, BR18 must be a member of a non-stub area or the OSPF backbone before it can import routes other than its directly connected network(s). |
| `1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.2 is the corporate-network motivation: the sentence states what any solution has to achieve, as a design goal, not an obligation on an implementation. | Any solution to this dilemma must also honor Area 1's path of choice to 192.168.192/20 through A0 with redundancy through B0 while at the same time honoring Area 2's path of choice to 192.168.208/20 through B0 with redundancy through A0. |
| `1.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 1.3 is the overview; it restates the border-router default-origination obligation Section 2.4 states. | Default routes are necessary because NSSAs do not receive full routing information and must have a default route in order to route to AS-external destinations. |
| `2.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence quotes [OSPF] Section 10.5 to state the E-bit condition the Section 2.1 Hello check verifies. | As explained in [OSPF] Section 10.5, if Type-5 LSAs are not flooded into/throughout the area, the E-bit must be clear in the option field of the received Hello packets. |
| `2.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.1 restates the Appendix A rule that the E-bit is clear whenever the N-bit is set. | Therefore, if the N-bit is set in the options field, the E-bit must be clear. |
| `2.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence restates the non-zero forwarding address obligation Section 2.4 states for a P-bit Type-7 LSA. | Those Type-7 LSAs that are to be translated into Type-5 LSAs must have their forwarding address set. |
| `2.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explanatory aside: it says non-zero forwarding addresses ease translation because border routers are not required to compute them, and states no obligation. | Also the non-zero forwarding addresses of Type-7 LSAs ease the process of their translation into Type-5 LSAs, as NSSA border routers are not required to compute them. |
| `2.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "The LSAs of these networks must have a valid non-zero forwarding address" restates the same obligation in the same section. | The LSAs of these networks must have a valid non-zero forwarding address. |
| `2.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The zero-forwarding-address case restates the rule that the ASBR entry is chosen from the originating NSSA. | Since a Type-7 LSA only has area-wide flooding scope, when its forwarding address is set to 0.0.0.0, its ASBR's routing table entry must be chosen from the originating NSSA. |
| `4.0:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.0 describes what the [DIGI] document contains; it is a pointer, not an obligation of this document. | [DIGI] describes the extensions to OSPF required to add digital signature authentication to Link State data and to provide a certification mechanism for router data. |
| `7.0:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Appendix B describes the Type-1 router-LSA and defers outright: "For details concerning the construction of router-LSAs, see [OSPF] Section 12.4.1." The single-router-LSA rule is RFC 2328's. | All of the router's links to the area must be described in a single router-LSA. |
| `7.0:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix C's Type-7 packet format restates the Section 2.4 P-bit origination rule. | The Options field must have the N/P bit set as described in Appendix A when the originating router desires that the external route be propagated throughout the OSPF domain. |
| `7.0:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix C restates the Section 2.4 forwarding address rule word for word. | If the P-bit is set, the forwarding address must be non-zero, otherwise it may be 0.0.0.0. |
| `7.0:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix F's differences list restates the Section 2.7 no-summary default rule. | When summary routes are not imported into an NSSA, the default LSA originated by its border routers must be a Type-3 summary-LSA. |
| `7.0:9` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Internet Society copyright boilerplate. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3101, so its obligations are stated where they were written.
