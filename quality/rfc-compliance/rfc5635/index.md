# RFC 5635 - Remote Triggered Black Hole Filtering with Unicast Reverse Path Forwarding (uRPF)

Partial. Every requirement this repository extracted from RFC 5635, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 0 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5635.md` |
| Requirement shard | `rfc/requirements/rfc5635.md` |
| RFC text | `rfc/full/rfc5635.txt` |

## Enrolment

Enrolled: One MUST-level sentence (§3.2, customer-triggered RTBH): an operator who lets customers trigger blackholing MUST only accept announcements for prefixes the customer is authorized to advertise. RFC5635-3.2-1 restates the first condition RFC 7999 Section 3.3 states for the same customer-triggered blackhole, and Ze answers both through one producer (internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, which honours the blackhole only when coveredByAuthorized finds the prefix inside the peer's `blackhole prefixes`). The row is therefore a `{rollup}` onto RFC7999-3.3-1 and gates nothing of its own. The one SHOULD NOT (§4.1, source-based RTBH from peers or customers) is not gated. The walk is recorded in rfc/extraction/rfc5635.json at register rfc2119, its one site mapped. Enrolled 2026-10-26.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Customer-triggered destination RTBH is authorized per BGP session: Ze turns a received route into a discard route only when the prefix is covered by one of the peer's `blackhole prefixes` and the session named the blackhole community ([`internal/component/bgp/plugins/rib/rib_blackhole.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole.go)::blackholeRouteType), which is the Section 3.2 MUST, carried as a rollup onto RFC7999-3.3-1 and proven there. The discard route is a Linux FIB blackhole or a VPP drop path. Per-interface reverse path filtering, the uRPF half of source-based RTBH, is configured by Ze and performed by Linux (`rp_filter` for IPv4, an nftables fib rule for IPv6) or VPP.

**What the ledger says remains**

No gated row of this document is open. The Section 4.1 SHOULD NOT (do not accept source-based RTBH announcements from peers or customers) has no dedicated guard: a blackhole is honoured on any session that named the community and authorized the prefix. Source-based RTBH as a whole (a discard route matched by the uRPF source lookup) has no end-to-end test.

## Coverage

RFC 5635 declares no MUST-level requirement, so the gate counts nothing here.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5635-3.2-1` | If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC7999-3.3-1; RFC 7999 Section 3.3 states the same condition for the same customer-triggered blackhole, that the announced prefix is covered by one the neighbour is authorized to advertise, and Ze enforces both in internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, so this sentence asserts nothing that row does not carry. **derived:** met |
| `RFC5635-4.1-1` | As a matter of policy, operators SHOULD NOT accept source-based RTBH announcements from their peers or customers, they should only be installed by local or attack management systems within their administrative domain. (§4.1) | SHOULD NOT | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5635-3.2-1`](#rfc5635-3.2-1) If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2) | no test | no test carries this requirement id; annotated {rollup}: RFC7999-3.3-1; RFC 7999 Section 3.3 states the same condition for the same customer-triggered blackhole, that the announced prefix is covered by one the neighbour is authorized to advertise, and Ze enforces both in internal/component/bgp/plugins/rib/rib_blackhole.go::blackholeRouteType, so this sentence asserts nothing that row does not carry |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5635-3.2-1`](#rfc5635-3.2-1)

If this is configured, an operator MUST only accept announcements from the customer for prefixes that the customer is authorized to advertise, in order to prevent the customer from accidentally (or intentionally) black- holing space that they are not allowed to advertise. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5635-3.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5635.txt |
| Source fingerprint | d5c6de8647891bec |
| Record | rfc/extraction/rfc5635.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, status, copyright and contents | 0 | skipped (front-matter) | Title page, status, copyright and contents |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments |
| `7` | References | 0 | skipped (references) | References |
| `7.1` | Normative references | 0 | skipped (references) | Normative references |
| `7.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5635 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5635, so its obligations are stated where they were written.
