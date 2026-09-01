# RFC 5250 - The OSPF Opaque LSA Option

Experimental. Every requirement this repository extracted from RFC 5250, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 77.8% | 7 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 19 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 11 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5250.md` |
| Requirement shard | `rfc/requirements/rfc5250.md` |
| RFC text | `rfc/full/rfc5250.txt` |

## Enrolment

Enrolled: OSPF Opaque LSA Option (opaque LSA types 9/10/11): nine MUST-level requirements, all met (six with positive+negative tags, three single-polarity) in internal/plugins/ospf. 3-1 (opaque types 9/10/11 map to link/area/AS flooding scope), 3.1-1 (a type-9 link-local opaque LSA is stored and flooded only on its arrival interface), 3.1-3 (a type-11 AS-scope opaque LSA is not flooded into a stub area), 3.1-4 (opaque LSAs are flooded only to opaque-capable neighbors), 3.1-5 (opaque capability is learned from the DD Options O-bit, not from Hello), and 5-1 (an AS-scope opaque LSA from an unreachable originator is not usable) carry positive+negative tags. 3.1-2 (a type-10 area-scope opaque LSA is bound to and confined to its receiving area), 3-2 (the Link State ID splits into an Opaque Type octet and a 24-bit Opaque ID), and 5-2 (reachability is recomputed live, never cached) are {single-polarity: positive}.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Opaque LSA framework and retention.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5250-3-1`](#rfc5250-3-1), [`RFC5250-3.1-1`](#rfc5250-3.1-1), [`RFC5250-3.1-2`](#rfc5250-3.1-2), [`RFC5250-3.1-3`](#rfc5250-3.1-3), [`RFC5250-3.1-4`](#rfc5250-3.1-4), [`RFC5250-3.1-5`](#rfc5250-3.1-5), [`RFC5250-5-1`](#rfc5250-5-1)

**Annotated instead of tested (2):** [`RFC5250-3-2`](#rfc5250-3-2), [`RFC5250-5-2`](#rfc5250-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5250-3-1` | Recognize LS types 9, 10, 11 as Opaque LSAs and apply scope-specific flooding (§3, §3.1) | MUST | 3 | **positive:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L38). **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L73). **negative:** `unit/verify` [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L25). **negative:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L88) |
| `RFC5250-3.1-1` | Type-9 LSA received on an interface other than the target interface MUST be discarded and not acknowledged (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L116). **negative:** `unit/verify` [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L120) |
| `RFC5250-3.1-2` | Type-10 LSA whose area differs from the target interface's area MUST be discarded (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L63). **positive:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L203). **negative:** `unit/verify` [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L207) |
| `RFC5250-3.1-3` | Type-11 LSA MUST NOT be flooded into stub areas or NSSAs; received on such an interface it MUST be discarded (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L161). **negative:** `unit/verify` [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L152) |
| `RFC5250-3.1-4` | Flood Opaque LSAs only to opaque-capable neighbors (O-bit set in DD) (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L38). **negative:** `unit/verify` [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L42) |
| `RFC5250-3-2` | Split the Link State ID into a 1-byte Opaque Type + 3-byte Opaque ID for the LSDB key (§3, Appendix A.2) | MUST | 3 | **positive:** `unit/verify` [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/lsa_opaque_test.go#L19). **negative:** no negative test. **{single-polarity}:** the Link State ID split is a pure bit operation (high octet is the Opaque Type, low 24 bits the Opaque ID) with no validation or reject path, so there is no negative behavior to drive |
| `RFC5250-3.1-5` | Ignore the O-bit when received in packets other than Database Description packets (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L74). **negative:** `unit/verify` [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L63) |
| `RFC5250-5-1` | For type-11 LSAs, look up the originating ASBR's routing-table entry; if unreachable, do nothing with the LSA (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L51). **negative:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L44) |
| `RFC5250-5-2` | Discontinue using all Opaque LSAs from an originator detected as unreachable (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L52). **negative:** no negative test. **{single-polarity}:** reachability is recomputed from a live SPF seam on every opaque delivery (internal/plugins/ospf/opaque.go:126,243) and never cached, so a now-unreachable originator yields not-usable on the next evaluation; there is no stale-cache code path to drive a negative |
| `RFC5250-3.1-6` | Set the O-bit in DD packets to advertise opaque capability; SHOULD NOT set it in non-DD packets (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5250-8-1` | Rate-limit Opaque LSA origination (>= 5 s) and acceptance (>= 1 s) (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5250 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5250-3-1`](#rfc5250-3-1)

Recognize LS types 9, 10, 11 as Opaque LSAs and apply scope-specific flooding (§3, §3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L88) | unit/verify | unproven |
| negative | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L25) | unit/verify | unproven |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L73) | unit/verify | unproven |
| positive | [`TestLSTypeKnownValues`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/lstype_test.go#L38) | unit/verify | unproven |

### [`RFC5250-3.1-1`](#rfc5250-3.1-1)

Type-9 LSA received on an interface other than the target interface MUST be discarded and not acknowledged (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L120) | unit/verify | unproven |
| positive | [`TestOpaqueType9WrongInterfaceDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L116) | unit/verify | unproven |

### [`RFC5250-3.1-2`](#rfc5250-3.1-2)

Type-10 LSA whose area differs from the target interface's area MUST be discarded (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L207) | unit/verify | unproven |
| positive | [`TestOpaqueScopeRouting`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L63) | unit/verify | unproven |
| positive | [`TestOpaqueType10ConfinedToItsArea`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L203) | unit/verify | unproven |

### [`RFC5250-3.1-3`](#rfc5250-3.1-3)

Type-11 LSA MUST NOT be flooded into stub areas or NSSAs; received on such an interface it MUST be discarded (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L152) | unit/verify | unproven |
| positive | [`TestOpaqueType11StubDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_scope_test.go#L161) | unit/verify | unproven |

### [`RFC5250-3.1-4`](#rfc5250-3.1-4)

Flood Opaque LSAs only to opaque-capable neighbors (O-bit set in DD) (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L42) | unit/verify | unproven |
| positive | [`TestOpaqueFloodOnlyToOpaqueNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_flood_test.go#L38) | unit/verify | unproven |

### [`RFC5250-3-2`](#rfc5250-3-2)

Split the Link State ID into a 1-byte Opaque Type + 3-byte Opaque ID for the LSDB key (§3, Appendix A.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpaqueLinkStateIDSplit`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/lsa_opaque_test.go#L19) | unit/verify | unproven |

### [`RFC5250-3.1-5`](#rfc5250-3.1-5)

Ignore the O-bit when received in packets other than Database Description packets (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L63) | unit/verify | unproven |
| positive | [`TestOpaqueBitIgnoredOutsideDD`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/neighbor/dd_opaque_test.go#L74) | unit/verify | unproven |

### [`RFC5250-5-1`](#rfc5250-5-1)

For type-11 LSAs, look up the originating ASBR's routing-table entry; if unreachable, do nothing with the LSA (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L44) | unit/verify | unproven |
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L51) | unit/verify | unproven |

### [`RFC5250-5-2`](#rfc5250-5-2)

Discontinue using all Opaque LSAs from an originator detected as unreachable (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOpaqueType11UnreachableOriginatorNotUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/opaque_reachability_test.go#L52) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5250, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5250, so its obligations are stated where they were written.
