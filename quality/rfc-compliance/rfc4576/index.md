# RFC 4576 - Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs)

No row in the public ledger. Every requirement this repository extracted from RFC 4576, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 4 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4576.md` |
| Requirement shard | `rfc/requirements/rfc4576.md` |
| RFC text | `rfc/full/rfc4576.txt` |

## Enrolment

Enrolled: Using an LSA Options Bit (DN bit) to Prevent Looping in BGP/MPLS IP VPNs: four MUST-level requirements, all {not-applicable} to Ze. RFC 4576 governs OSPF running as a BGP/MPLS-VPN PE-CE protocol (the DN bit prevents PE-CE loops). Ze does NOT run OSPF as an MPLS-VPN PE-CE protocol -- it has no L3VPN OSPF code path (no VPN OSPF route redistribution, no sham links, no PE-CE OSPF; grep across internal/plugins/ospf/ for sham-link/PE-CE/vpn-ospf/domain-id finds nothing), running OSPF only as a plain IGP. RFC4576-4-1 (PE sets DN bit on type 3/5/7 LSA to CE) and RFC4576-4-2 (DN bit clear on all other LSA types) have no PE origination code path; RFC4576-4-3 (ignore DN bit on other LSA types) -- Ze decodes the DN bit as a field (internal/plugins/ospf/v3/types/prefix.go:68) but performs no VPN loop-prevention acting on it; RFC4576-4-4 (PE MUST NOT use a CE DN-bit-set LSA in route calc) has no PE route-calc code path. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4576.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (4):** [`RFC4576-4-1`](#rfc4576-4-1), [`RFC4576-4-2`](#rfc4576-4-2), [`RFC4576-4-3`](#rfc4576-4-3), [`RFC4576-4-4`](#rfc4576-4-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4576-4-1` | "When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set." (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not run OSPF as a BGP/MPLS-VPN PE-CE protocol -- it has no L3VPN OSPF code path (no VPN OSPF route redistribution, no sham links, no PE-CE OSPF; grep for sham-link / PE-CE / vpn-ospf / domain-id across internal/plugins/ospf/ finds nothing). Ze runs OSPF only as a plain IGP, so it never originates a type 3/5/7 LSA "from a PE to a CE" and has no code path that would set the RFC 4576 DN bit. |
| `RFC4576-4-2` | "The DN bit MUST be clear in all other LSA types." (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze is not an MPLS-VPN PE and originates no LSAs in a VPN PE-CE context, so it never sets the DN bit on any LSA type; the "DN bit clear in all other LSA types" transmission rule governs a PE role Ze does not implement. |
| `RFC4576-4-3` | "The DN bit MUST be ignored in all other LSA types." (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This governs a VPN PE's receive processing of the DN bit. Ze runs plain (non-VPN) OSPF with no PE-CE role: its OSPFv3 prefix codec merely decodes the DN bit as a field (internal/plugins/ospf/v3/types/prefix.go:68 Down()) and Ze performs no VPN loop-prevention that acts on it, so the PE-CE DN-bit handling this requirement defines has no applicable code path. |
| `RFC4576-4-4` | "When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation." (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This governs a VPN PE ignoring a CE's DN-bit-set LSA during OSPF route calculation. Ze is not an MPLS-VPN PE and runs no OSPF PE-CE / VPN OSPF route calculation, so there is no code path in which a PE would consume a CE's DN-bit-set LSA. |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4576-4-1`](#rfc4576-4-1) "When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set." (§4) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not run OSPF as a BGP/MPLS-VPN PE-CE protocol -- it has no L3VPN OSPF code path (no VPN OSPF route redistribution, no sham links, no PE-CE OSPF; grep for sham-link / PE-CE / vpn-ospf / domain-id across internal/plugins/ospf/ finds nothing). Ze runs OSPF only as a plain IGP, so it never originates a type 3/5/7 LSA "from a PE to a CE" and has no code path that would set the RFC 4576 DN bit. |
| [`RFC4576-4-2`](#rfc4576-4-2) "The DN bit MUST be clear in all other LSA types." (§4) | no test | no test carries this requirement id; annotated {not-applicable}: Ze is not an MPLS-VPN PE and originates no LSAs in a VPN PE-CE context, so it never sets the DN bit on any LSA type; the "DN bit clear in all other LSA types" transmission rule governs a PE role Ze does not implement. |
| [`RFC4576-4-3`](#rfc4576-4-3) "The DN bit MUST be ignored in all other LSA types." (§4) | no test | no test carries this requirement id; annotated {not-applicable}: This governs a VPN PE's receive processing of the DN bit. Ze runs plain (non-VPN) OSPF with no PE-CE role: its OSPFv3 prefix codec merely decodes the DN bit as a field (internal/plugins/ospf/v3/types/prefix.go:68 Down()) and Ze performs no VPN loop-prevention that acts on it, so the PE-CE DN-bit handling this requirement defines has no applicable code path. |
| [`RFC4576-4-4`](#rfc4576-4-4) "When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation." (§4) | no test | no test carries this requirement id; annotated {not-applicable}: This governs a VPN PE ignoring a CE's DN-bit-set LSA during OSPF route calculation. Ze is not an MPLS-VPN PE and runs no OSPF PE-CE / VPN OSPF route calculation, so there is no code path in which a PE would consume a CE's DN-bit-set LSA. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4576-4-1`](#rfc4576-4-1)

"When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set." (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4576-4-1, so no unit is bound to it.

### [`RFC4576-4-2`](#rfc4576-4-2)

"The DN bit MUST be clear in all other LSA types." (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4576-4-2, so no unit is bound to it.

### [`RFC4576-4-3`](#rfc4576-4-3)

"The DN bit MUST be ignored in all other LSA types." (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4576-4-3, so no unit is bound to it.

### [`RFC4576-4-4`](#rfc4576-4-4)

"When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation." (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4576-4-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4576, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4576, so its obligations are stated where they were written.
