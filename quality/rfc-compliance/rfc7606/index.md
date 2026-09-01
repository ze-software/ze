# RFC 7606 - Revised Error Handling for BGP UPDATE Messages

Partial. Every requirement this repository extracted from RFC 7606, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 88.2% | 45 of 51 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.8% | 5 of 51 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 51 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 189 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 52 | of 56 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 52 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 2.0% | 1 of 51 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 52 | of 52 gated MUSTs judged | 1 weak, wrong or unimplemented, 2 no longer current. Each is named below under its own requirement id |

The 4 shares marked as a part above are the whole of the 51 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 56 |
| Gated MUST-level | 52 |
| Obligations that bind Ze | 51 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 191 |
| Tagged units | 189 |
| Recorded audit verdicts | 52 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7606.md` |
| Requirement shard | `rfc/requirements/rfc7606.md` |
| RFC text | `rfc/full/rfc7606.txt` |

## Enrolment

Enrolled: Revised UPDATE error handling: the error-handling contract for every UPDATE Ze parses. Pilot for this system -- ~60 near-1:1 tests already exist and the one real divergence (5.1 MP ordering, docs/architecture/wire/mp-nlri-ordering.md) proves the {gap} path is honest rather than an escape hatch.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Structural UPDATE validation, treat-as-withdraw (routes are synthesized into withdrawals and removed from the Adj-RIB-In), attribute-discard decisions, session-reset with NOTIFICATION UPDATE Message Error, per-attribute validation for 15 attribute codes, inner MP_REACH/MP_UNREACH NLRI overrun and RFC 4760 flag-consistency validation (§5.3, session reset via §3.j) for IPv4/IPv6 unicast and multicast (ADD-PATH aware, RFC 7911 path-ids skipped when negotiated), tests bound per requirement in [`rfc/requirements/rfc7606.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7606.md).

**What the ledger says remains**

One MUST-level gap, annotated in [`rfc/short/rfc7606.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7606.md) and gated by `./le rfc check`: §5.1 first bullet: Ze intentionally emits MP_UNREACH first and MP_REACH last ([`docs/architecture/wire/mp-nlri-ordering.md`](https://github.com/ze-software/ze/blob/main/docs/architecture/wire/mp-nlri-ordering.md)).

- **Closed 2026-08-01:** §5.4. A route whose NLRI type Ze does not implement is now discarded at ingress, in `enforceRFC7606` ([`internal/component/bgp/reactor/session_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation.go)), so it reaches neither the RIB nor the forward path. The RIB alone would not have sufficed, because `reactorForwardRS` relays the received wire without consulting it. The ruling is per family, because §5.4 discards "unless the relevant specification for that address family specifies otherwise" and RFC 9552 §5.2 uses that clause to REQUIRE preserve-and-propagate for BGP-LS. Each NLRI plugin registers its own recognizer (`internal/core/bgp/nlri/nlritype`), so the three families §5.4 binds each carry their own ruling: EVPN discards route types outside 1..5 ([`internal/component/bgp/plugins/nlri/evpn/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7606.go)), MCAST-VPN outside 1..7 (`.../nlri/mvpn/rfc7606.go`, RFC 6514 §4), and BGP-MUP anything but Architecture Type 1 with Route Type 1..4 (`.../nlri/mup/rfc7606.go`, draft-ietf-bess-mup-safi §3.1). BGP-LS registers no recognizer and propagates unchanged, and a family nobody has ruled on keeps its previous behavior. Judging a route type needs the section carved into single NLRIs, so MCAST-VPN and BGP-MUP gained `nlrisplit` splitters, which also made `nlrisplit.Supported` true for them: ze now installs both as opaque Adj-RIB-In entries the way it already installed EVPN (`insertPoolNLRIs`, [`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)). This reverses the divergence disclosed here until 2026-08-01.
- **Closed 2026-07-20:** §7.13, §7.15 and §7.16 gained attribute validators for codes 24, 25 and 128, §7.15's unrecognized-type tolerance is now met by design rather than by omission (the validator reads length only), and §6's debugging facility now logs the NLRI involved and the entire malformed UPDATE.
- **Also closed 2026-07-20:** §5.1's second bullet. Ze already originated only compliant UPDATEs; the relay now splits a received mixed shape as well, in both the zero-copy same-context branch and the re-encode branch, so every UPDATE ze sends carries at most one NLRI-bearing field. A compliant single-field UPDATE keeps the zero-copy forward: the shape verdict is cached per received message and the received bytes are handed on unchanged. Receive-side tolerance of any position or combination (§5.1 third bullet) is unchanged.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 45 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **52** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (45):** [`RFC7606-2-1`](#rfc7606-2-1), [`RFC7606-2-2`](#rfc7606-2-2), [`RFC7606-2-3`](#rfc7606-2-3), [`RFC7606-3.b-1`](#rfc7606-3.b-1), [`RFC7606-3.c-1`](#rfc7606-3.c-1), [`RFC7606-3.d-1`](#rfc7606-3.d-1), [`RFC7606-3.e-1`](#rfc7606-3.e-1), [`RFC7606-3.f-1`](#rfc7606-3.f-1), [`RFC7606-3.g-1`](#rfc7606-3.g-1), [`RFC7606-3.i-1`](#rfc7606-3.i-1), [`RFC7606-3.j-1`](#rfc7606-3.j-1), [`RFC7606-4-1`](#rfc7606-4-1), [`RFC7606-5.1-2`](#rfc7606-5.1-2), [`RFC7606-5.2-1`](#rfc7606-5.2-1), [`RFC7606-5.4-1`](#rfc7606-5.4-1), [`RFC7606-7.1-1`](#rfc7606-7.1-1), [`RFC7606-7.2-1`](#rfc7606-7.2-1), [`RFC7606-7.3-1`](#rfc7606-7.3-1), [`RFC7606-7.4-1`](#rfc7606-7.4-1), [`RFC7606-7.5-1`](#rfc7606-7.5-1), [`RFC7606-7.5-2`](#rfc7606-7.5-2), [`RFC7606-7.6-1`](#rfc7606-7.6-1), [`RFC7606-7.7-1`](#rfc7606-7.7-1), [`RFC7606-7.8-1`](#rfc7606-7.8-1), [`RFC7606-7.9-1`](#rfc7606-7.9-1), [`RFC7606-7.9-2`](#rfc7606-7.9-2), [`RFC7606-7.10-1`](#rfc7606-7.10-1), [`RFC7606-7.10-2`](#rfc7606-7.10-2), [`RFC7606-7.11-1`](#rfc7606-7.11-1), [`RFC7606-7.13-1`](#rfc7606-7.13-1), [`RFC7606-7.14-1`](#rfc7606-7.14-1), [`RFC7606-7.15-1`](#rfc7606-7.15-1), [`RFC7606-7.16-1`](#rfc7606-7.16-1), [`RFC7606-4-2`](#rfc7606-4-2), [`RFC7606-3.g-2`](#rfc7606-3.g-2), [`RFC7606-6-1`](#rfc7606-6-1), [`RFC7606-3.a-1`](#rfc7606-3.a-1), [`RFC7606-2-5`](#rfc7606-2-5), [`RFC7606-5.3-1`](#rfc7606-5.3-1), [`RFC7606-5.3-2`](#rfc7606-5.3-2), [`RFC7606-5.3-3`](#rfc7606-5.3-3), [`RFC7606-5.3-4`](#rfc7606-5.3-4), [`RFC7606-5.3-5`](#rfc7606-5.3-5), [`RFC7606-5.3-6`](#rfc7606-5.3-6), [`RFC7606-2-6`](#rfc7606-2-6)

**Annotated instead of tested (7):** [`RFC7606-3.h-1`](#rfc7606-3.h-1), [`RFC7606-3.h-2`](#rfc7606-3.h-2), [`RFC7606-5.1-1`](#rfc7606-5.1-1), [`RFC7606-5.1-3`](#rfc7606-5.1-3), [`RFC7606-7.14-2`](#rfc7606-7.14-2), [`RFC7606-7.15-2`](#rfc7606-7.15-2), [`RFC7606-8-1`](#rfc7606-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7606-2-1` | When treat-as-withdraw is used, the UPDATE message MUST be treated as though all contained routes had been withdrawn (§2) | MUST | 2 | **positive:** `unit/verify` [`TestSessionRFC7606ValidUpdateDispatchesAnnouncement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L277). **negative:** `unit/verify` [`TestRIBTreatAsWithdrawAddPathPreservesPathID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_structured_test.go#L166). **negative:** `unit/verify` [`TestRIBTreatAsWithdrawRemovesInstalledRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_structured_test.go#L92). **negative:** `unit/verify` [`TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1964). **negative:** `unit/verify` [`TestSynthesizeWithdrawConvertsMPReachToMPUnreach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L57). **negative:** `unit/verify` [`TestSynthesizeWithdrawMovesNLRIToWithdrawn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L21) |
| `RFC7606-2-2` | Attribute discard: malformed attribute MUST be discarded and UPDATE processing continues (§2) | MUST | 2 | **positive:** `unit/verify` [`TestApplyAttrDiscardEmptyEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L336). **negative:** `unit/verify` [`TestApplyAttrDiscardMultipleEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L255). **negative:** `unit/verify` [`TestSessionRFC7606AttributeDiscardContinues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2045) |
| `RFC7606-2-3` | Attribute discard MUST NOT be used for attributes affecting route selection or installation (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestRFC7606NonRouteSelectionAttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L489). **negative:** `unit/verify` [`TestRFC7606RouteSelectionAttributesNeverDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L418) |
| `RFC7606-3.b-1` | If Withdrawn Routes Length + Total Attribute Length + 23 exceeds Message Length, Error Subcode MUST be Malformed Attribute List (§3.b) | MUST | 3.b | **positive:** `unit/verify` [`TestEnforceRFC7606_SectionLengthsExactlyFitAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L115). **negative:** `unit/verify` [`TestSessionRFC7606SectionLengthConflictNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L154) |
| `RFC7606-3.c-1` | If Optional or Transitive flag bits conflict with specification, attribute MUST be treated as malformed with treat-as-withdraw, unless the specification for the attribute mandates different handling for incorrect Attribute Flags (§3.c) | MUST | 3.c | **positive:** `unit/verify` [`TestRFC7606FlagsOptionalAttributeValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1220). **negative:** `unit/verify` [`TestRFC7606FlagsWellKnownMarkedOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1179). **negative:** `unit/verify` [`TestRFC7606FlagsWellKnownNotTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1200) |
| `RFC7606-3.d-1` | If any well-known mandatory attributes are missing, treat-as-withdraw MUST be used (§3.d) | MUST | 3.d | **positive:** `unit/verify` [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L183). **negative:** `unit/verify` [`TestRFC7606MissingASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L146). **negative:** `unit/verify` [`TestRFC7606MissingOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L128). **negative:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2308) |
| `RFC7606-3.e-1` | Treat-as-withdraw MUST be used for malformed ORIGIN, AS_PATH, NEXT_HOP, MULTI_EXIT_DISC, or LOCAL_PREF (§3.e) | MUST | 3.e | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1618). **negative:** `unit/verify` [`TestRFC7606ASPathUnrecognizedSegmentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L447). **negative:** `unit/verify` [`TestRFC7606LocalPrefIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1016). **negative:** `unit/verify` [`TestRFC7606MalformedOriginLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L12) |
| `RFC7606-3.f-1` | Attribute discard MUST be used for malformed ATOMIC_AGGREGATE or AGGREGATOR (§3.f) | MUST | 3.f | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1619). **negative:** `unit/verify` [`TestRFC7606AggregatorLen8NoASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1336). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L421). **negative:** `unit/verify` [`TestRFC7606MalformedAtomicAggregate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L165) |
| `RFC7606-3.g-1` | If MP_REACH_NLRI or MP_UNREACH_NLRI appears more than once, a NOTIFICATION MUST be sent with Malformed Attribute List (§3.g) | MUST | 3.g | **positive:** `unit/verify` [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L696). **negative:** `unit/verify` [`TestRFC7606MultipleMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L210). **negative:** `unit/verify` [`TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_test.go#L48). **negative:** `unit/verify` [`TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_test.go#L79). **negative:** `unit/verify` [`TestRFC7606Section3gDuplicateMPUnreachResetsWithRoutesPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_unreach_test.go#L51). **negative:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2247). **negative:** `unit/verify` [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L30). **negative:** `unit/verify` [`TestSessionRFC7606SessionResetNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1812) |
| `RFC7606-3.h-1` | When multiple attribute errors exist with same approach, the specified approach MUST be used (§3.h) | MUST | 3.h | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC7606CollectAllErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1520). **{single-polarity}:** "multiple attribute errors exist" has no conforming instance -- an UPDATE with zero errors is not a case of this rule, it is the absence of the rule |
| `RFC7606-3.h-2` | When multiple attribute errors exist with different approaches, the strongest action MUST be used (§3.h) | MUST | 3.h | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC7606MultipleErrorsStrongest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1496). **{single-polarity}:** as 3.h-1: the premise of the rule is that errors exist, so no positive case can be constructed |
| `RFC7606-3.i-1` | The Withdrawn Routes field MUST be checked for syntactic correctness in the same manner as the NLRI field (§3.i) | MUST | 3.i | **positive:** `unit/verify` [`TestEnforceRFC7606_ValidWithdrawnNLRIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L73). **negative:** `unit/verify` [`TestEnforceRFC7606_InvalidWithdrawnNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L172) |
| `RFC7606-3.j-1` | If NLRI/MP_REACH/MP_UNREACH cannot be successfully parsed, session reset (or AFI/SAFI disable) MUST be followed (§3.j) | MUST | 3.j | **positive:** `unit/verify` [`TestRFC7606NLRIPrefixLengthValidIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L824). **positive:** `unit/verify` [`TestRFC7606NLRIPrefixLengthValidIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L868). **negative:** `unit/verify` [`TestEnforceRFC7606_InvalidTrailingNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L198). **negative:** `unit/verify` [`TestRFC7606MPReachTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L798). **negative:** `unit/verify` [`TestRFC7606MPUnreachTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1411). **negative:** `unit/verify` [`TestRFC7606NLRIOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L928). **negative:** `unit/verify` [`TestRFC7606NLRIPrefixLengthTooLongIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L844). **negative:** `unit/verify` [`TestRFC7606NLRIPrefixLengthTooLongIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L897). **negative:** `functional/verify` [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L9) |
| `RFC7606-4-1` | In attribute length conflicts, treat-as-withdraw MUST be used, unless some other more severe error dictates a stronger approach (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1620). **negative:** `unit/verify` [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L237) |
| `RFC7606-5.1-1` | MP_REACH_NLRI or MP_UNREACH_NLRI (if present) SHALL be encoded as the very first path attribute (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze intentionally emits MP_UNREACH first and MP_REACH last, treating them as NLRI-carrying wire structures rather than descriptive attributes; deliberate design decision recorded in docs/architecture/wire/mp-nlri-ordering.md and disclosed publicly in the docs/features/rfc-status.md RFC 7606 row |
| `RFC7606-5.1-2` | An UPDATE message MUST NOT contain more than one of: non-empty Withdrawn Routes, non-empty NLRI, MP_REACH_NLRI, MP_UNREACH_NLRI (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestForwardSplitsMixedShapeAcrossContextsThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L156). **positive:** `unit/verify` [`TestForwardSplitsMixedShapeSameContextThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L51). **positive:** `unit/verify` [`TestNLRIBearingFieldCountEveryCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L55). **positive:** `unit/verify` [`TestSplitCompliantSplitsMixedUpdateThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L119). **positive:** `unit/verify` [`TestSplitWireUpdateDoesNotManufactureEoR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/split_eor_test.go#L32). **positive:** `unit/verify` [`TestSplitWireUpdateSplitsMixedShapeThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc7606_shape_test.go#L23). **negative:** `unit/verify` [`TestForwardCompliantShapeKeepsZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L116). **negative:** `unit/verify` [`TestSplitCompliantPassesThroughCompliantUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L151). **negative:** `unit/verify` [`TestSplitWireUpdateCompliantShapeUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc7606_shape_test.go#L58). **positive:** `functional/verify` [`rfc7606-relay-one-field.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-relay-one-field.ci#L4) |
| `RFC7606-5.1-3` | Implementation MUST still be prepared to receive MP_REACH/MP_UNREACH in any position or combination from older speakers (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L697). **positive:** `unit/verify` [`TestRFC7606Section51AcceptsMPReachWithLegacyNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L62). **positive:** `unit/verify` [`TestRFC7606Section51AcceptsMPUnreachAfterOtherAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L41). **positive:** `unit/verify` [`TestRFC7606Section51AcceptsMPUnreachWithLegacyNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L72). **positive:** `unit/verify` [`TestRFC7606Section51AcceptsReachAndUnreachTogether`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L51). **negative:** no negative test. **positive:** `functional/verify` [`rfc7606-receive-combinations.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-receive-combinations.ci#L3). **positive:** `functional/verify` [`rfc7606-relay-one-field.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-relay-one-field.ci#L8). **positive:** `interop/nightly` [`checkRFC7606MixedUpdate`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L607). **{single-polarity}:** the obligation is to ACCEPT MP attributes in any position, so the only conforming assertion is acceptance -- a negative would assert a rejection the RFC forbids |
| `RFC7606-5.2-1` | For an UPDATE containing path attributes OTHER THAN MP_UNREACH_NLRI that encodes no reachable NLRI, if any path attribute error specifies an approach other than attribute discard, session reset MUST be used. The "other than MP_UNREACH_NLRI" clause exempts End-of-RIB, which §5.2 defines as an UPDATE carrying only an MP_UNREACH_NLRI encoding no NLRI (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC7606MPUnreachMinValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1437). **positive:** `unit/verify` [`TestRFC7606NoNLRIAttributeDiscardNoEscalation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1477). **positive:** `unit/verify` [`TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_noreach_test.go#L40). **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2283). **negative:** `unit/verify` [`TestRFC7606NoNLRIEscalation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1457). **negative:** `unit/verify` [`TestRFC7606Section52LeavesAnUpdateWithNLRIAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_noreach_test.go#L69). **negative:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2269) |
| `RFC7606-5.4-1` | A BGP speaker advertising typed address family support MUST discard routes with unrecognized NLRI types, unless the relevant specification for that address family specifies otherwise (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC7606Section54DiscardsUnrecognizedEVPNType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L149). **positive:** `unit/verify` [`TestRFC7606Section54DiscardsUnrecognizedMUPType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L126). **positive:** `unit/verify` [`TestRFC7606Section54DiscardsUnrecognizedMVPNType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L97). **positive:** `unit/verify` [`TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L254). **positive:** `unit/verify` [`TestRFC7606Section54FiltersTreatAsWithdrawSynthesis`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L159). **positive:** `unit/verify` [`TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L44). **positive:** `unit/verify` [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L89). **positive:** `unit/verify` [`TestRFC7606Section54SessionResetsUnparseableTypedNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L336). **positive:** `unit/verify` [`mup/TestRecognizeNLRIRejectsUnimplementedTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606_test.go#L50). **positive:** `unit/verify` [`mvpn/TestRecognizeNLRIRejectsUnimplementedTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mvpn/rfc7606_test.go#L45). **negative:** `unit/verify` [`TestRFC7606Section54PropagatesUnknownBGPLSType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L187). **positive:** `functional/verify` [`rfc7606-54-discard-unrecognized-mup-nlri.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci#L4). **positive:** `functional/verify` [`rfc7606-54-discard-unrecognized-nlri.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-discard-unrecognized-nlri.ci#L4). **negative:** `functional/verify` [`rfc7606-54-bgpls-override-propagates.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-bgpls-override-propagates.ci#L4). **positive:** `interop/nightly` [`checkRFC7606TypedNLRIDiscard`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L711) |
| `RFC7606-7.1-1` | ORIGIN: malformed if length != 1 or undefined value; treat-as-withdraw (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC7606OriginValueEGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L560). **positive:** `unit/verify` [`TestRFC7606OriginValueIGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L543). **positive:** `unit/verify` [`TestRFC7606OriginValueIncomplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L577). **negative:** `unit/verify` [`TestRFC7606MalformedOriginLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L11). **negative:** `unit/verify` [`TestRFC7606OriginValueInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L594). **negative:** `functional/verify` [`rfc7606-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci#L8) |
| `RFC7606-7.2-1` | AS_PATH: malformed if unrecognized segment type, overrun, underrun, or zero-length segment; treat-as-withdraw (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC7606ASPath4ByteASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1384). **positive:** `unit/verify` [`TestRFC7606ASPathValidSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L403). **positive:** `unit/verify` [`TestRFC7606ASPathValidSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L425). **negative:** `unit/verify` [`TestRFC7606ASPath4ByteASNOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2377). **negative:** `unit/verify` [`TestRFC7606ASPathSegmentOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L469). **negative:** `unit/verify` [`TestRFC7606ASPathSegmentUnderrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L491). **negative:** `unit/verify` [`TestRFC7606ASPathUnrecognizedSegmentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L446). **negative:** `unit/verify` [`TestRFC7606ASPathZeroSegmentLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L518) |
| `RFC7606-7.3-1` | NEXT_HOP: malformed if length != 4; treat-as-withdraw (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L184). **negative:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1769) |
| `RFC7606-7.4-1` | MULTI_EXIT_DISC: malformed if length != 4; treat-as-withdraw (§7.4) | MUST | 7.4 | **positive:** `unit/verify` [`TestRFC7606FlagsOptionalAttributeValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1221). **negative:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1783) |
| `RFC7606-7.5-1` | LOCAL_PREF from eBGP: attribute discard (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestRFC7606LocalPrefIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L995). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L407). **negative:** `unit/verify` [`TestRFC7606LocalPrefEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L974). **negative:** `unit/verify` [`TestSessionRFC7606AttributeDiscardContinues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2046) |
| `RFC7606-7.5-2` | LOCAL_PREF from iBGP: malformed if length != 4; treat-as-withdraw (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestRFC7606LocalPrefIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L996). **negative:** `unit/verify` [`TestRFC7606LocalPrefIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1015) |
| `RFC7606-7.6-1` | ATOMIC_AGGREGATE: malformed if length != 0; attribute discard (§7.6) | MUST | 7.6 | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1622). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L420). **negative:** `unit/verify` [`TestRFC7606MalformedAtomicAggregate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L164) |
| `RFC7606-7.7-1` | AGGREGATOR: malformed if length != 6 (without 4-byte AS) or != 8 (with 4-byte AS); attribute discard (§7.7) | MUST | 7.7 | **positive:** `unit/verify` [`TestRFC7606AggregatorLen6NoASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1291). **positive:** `unit/verify` [`TestRFC7606AggregatorLen8WithASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1313). **negative:** `unit/verify` [`TestRFC7606AggregatorLen6WithASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1360). **negative:** `unit/verify` [`TestRFC7606AggregatorLen8NoASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1335). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L434) |
| `RFC7606-7.8-1` | Community: malformed if length is zero or not a multiple of 4; treat-as-withdraw (§7.8) | MUST | 7.8 | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1623). **negative:** `unit/verify` [`TestRFC7606CommunityZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L62). **negative:** `unit/verify` [`TestRFC7606MalformedCommunityLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L31) |
| `RFC7606-7.9-1` | ORIGINATOR_ID from eBGP: attribute discard (§7.9) | MUST | 7.9 | **positive:** `unit/verify` [`TestRFC7606OriginatorIDIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1062). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L449). **negative:** `unit/verify` [`TestRFC7606OriginatorIDEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1041) |
| `RFC7606-7.9-2` | ORIGINATOR_ID from iBGP: malformed if length != 4; treat-as-withdraw (§7.9) | MUST | 7.9 | **positive:** `unit/verify` [`TestRFC7606OriginatorIDIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1063). **negative:** `unit/verify` [`TestRFC7606OriginatorIDIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1082) |
| `RFC7606-7.10-1` | CLUSTER_LIST from eBGP: attribute discard (§7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC7606ClusterListIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1128). **negative:** `unit/verify` [`TestRFC7606ClusterListEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1107). **negative:** `unit/verify` [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L462) |
| `RFC7606-7.10-2` | CLUSTER_LIST from iBGP: malformed if length is zero or not a multiple of 4; treat-as-withdraw (§7.10) | MUST | 7.10 | **positive:** `unit/verify` [`TestRFC7606ClusterListIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1129). **negative:** `unit/verify` [`TestRFC7606ClusterListIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1154) |
| `RFC7606-7.11-1` | MP_REACH_NLRI with inconsistent Next Hop length: session reset or AFI/SAFI disable (§7.11) | MUST | 7.11 | **positive:** `unit/verify` [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L695). **positive:** `unit/verify` [`TestRFC7606MPReachIPv6NextHopDualValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L642). **positive:** `unit/verify` [`TestRFC7606MPReachIPv6NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L617). **positive:** `unit/verify` [`TestRFC7606MPReachVPNv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L747). **negative:** `unit/verify` [`TestRFC7606MPReachIPv4NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L722). **negative:** `unit/verify` [`TestRFC7606MPReachIPv6NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L670). **negative:** `unit/verify` [`TestRFC7606MPReachVPNv4NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L773) |
| `RFC7606-7.13-1` | Traffic Engineering path attribute if malformed: treat-as-withdraw (§7.13) | MUST | 7.13 | **positive:** `unit/verify` [`TestRFC7606TrafficEngineeringValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L83). **negative:** `unit/verify` [`TestRFC7606TrafficEngineeringTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L56) |
| `RFC7606-7.14-1` | Extended Community: malformed if length is zero or not a multiple of 8; treat-as-withdraw (§7.14) | MUST | 7.14 | **positive:** `unit/verify` [`TestRFC7606ExtendedCommunityValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L300). **negative:** `unit/verify` [`TestRFC7606ExtendedCommunityZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L331) |
| `RFC7606-7.14-2` | Unrecognized Extended Community Type or Sub-Type MUST NOT be treated as error (§7.14) | MUST NOT | 7.14 | **positive:** `unit/verify` [`TestRFC7606ExtendedCommunityUnrecognizedType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L360). **negative:** no negative test. **{single-polarity}:** the rule is a prohibition (MUST NOT treat an unrecognized type as an error), not a detection duty -- the only conforming observation is acceptance, and a negative would assert the rejection the RFC forbids |
| `RFC7606-7.15-1` | IPv6 Extended Community: malformed if length is zero or not a multiple of 20; treat-as-withdraw (§7.15) | MUST | 7.15 | **positive:** `unit/verify` [`TestRFC7606IPv6ExtCommunityValidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L142). **negative:** `unit/verify` [`TestRFC7606IPv6ExtCommunityBadLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L115) |
| `RFC7606-7.15-2` | Unrecognized IPv6 Extended Community Type or Sub-Type MUST NOT be treated as error (§7.15) | MUST NOT | 7.15 | **positive:** `unit/verify` [`TestRFC7606IPv6ExtCommunityUnrecognizedType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L172). **negative:** no negative test. **{single-polarity}:** the rule is a prohibition (MUST NOT treat an unrecognized type as an error), not a detection duty -- the only conforming observation is acceptance, and a negative would assert the rejection the RFC forbids. Now met BY DESIGN rather than by omission: validateIPv6ExtCommunityAttr takes the attribute value as `_` and tests length alone, so no Type or Sub-Type can reach a rejection |
| `RFC7606-7.16-1` | ATTR_SET if malformed: treat-as-withdraw (§7.16) | MUST | 7.16 | **positive:** `unit/verify` [`TestRFC7606AttrSetInnerASPathAlwaysFourOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L31). **positive:** `unit/verify` [`TestRFC7606AttrSetInnerDiscardDoesNotWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_discard_test.go#L25). **positive:** `unit/verify` [`TestRFC7606AttrSetInnerIBGPAttributesOnEBGPSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L55). **positive:** `unit/verify` [`TestRFC7606AttrSetValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L252). **negative:** `unit/verify` [`TestRFC7606AttrSetInnerMalformedStillWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L83). **negative:** `unit/verify` [`TestRFC7606AttrSetMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L207). **negative:** `unit/verify` [`TestRFC7606AttrSetNestingCapBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L102) |
| `RFC7606-4-2` | For all path attributes other than those specified as having an attribute length that may be zero, a zero attribute length SHALL be a syntax error handled as a malformed attribute. Of the attributes considered in RFC 7606, only AS_PATH and ATOMIC_AGGREGATE may validly have zero length; the RFC leaves this open for future attributes (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1621). **positive:** `unit/verify` [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L185). **negative:** `unit/verify` [`TestRFC7606ZeroLengthAttributeMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L273) |
| `RFC7606-3.g-2` | Duplicate non-MP attributes: discard all but first occurrence (§3.g) | MUST | 3.g | **positive:** `unit/verify` [`TestRFC7606DuplicateAttributeFirstOccurrenceWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L542). **negative:** `unit/verify` [`TestRFC7606DuplicateAttributeFirstOccurrenceIsValidated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L563) |
| `RFC7606-8-1` | A new BGP attribute specification MUST define what constitutes malformation and how to handle it (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an obligation on the authors of FUTURE attribute specifications, not on an implementation; Ze has no code path that could satisfy or violate it |
| `RFC7606-6-1` | Implementation must provide debugging facilities, at minimum logging an error listing the NLRI involved and containing the entire malformed UPDATE. NOTE: §6 uses lowercase "must", outside the RFC 2119 keyword set scoped by §1.1; kept at MUST level as a Ze policy choice, not an RFC 2119 obligation (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC7606DiagnosticsCoverSessionReset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L108). **positive:** `unit/verify` [`TestRFC7606DiagnosticsListNLRIAndUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L64). **negative:** `unit/verify` [`TestRFC7606DiagnosticsCostsNothingWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diag_cost_test.go#L26). **negative:** `unit/verify` [`TestRFC7606DiagnosticsSilentWhenDebugDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L90) |
| `RFC7606-7.2-2` | Leftmost-AS check violations SHOULD use treat-as-withdraw (§7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7606-8-2` | New attribute specifications SHOULD provide consideration of debugging facilities (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7606-7.2-3` | Leftmost-AS check violations MAY use session reset if configured (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7606-2-4` | Where session reset is specified, a speaker MAY instead use the RFC 4760 §7 "AFI/SAFI disable" approach: ignore all subsequent routes with that AFI/SAFI. §7.11 mandates one of the two approaches; the permission to choose is in §2 (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7606-3.a-1` | An error detected while processing an UPDATE for which a session reset is specified MUST be indicated by sending a NOTIFICATION with Error Code "UPDATE Message Error"; the subcode elaborates on the specific nature (§3.a) | MUST | 3.a | **positive:** `unit/verify` [`TestSessionRFC7606ValidUpdateSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L219). **negative:** `unit/verify` [`TestEnforceRFC7606_ShortBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L148). **negative:** `functional/verify` [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L7) |
| `RFC7606-2-5` | When treat-as-withdraw is used, the affected routes MUST be removed from the Adj-RIB-In per the procedures of RFC 4271 (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_test.go#L258). **negative:** `unit/verify` [`TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_test.go#L259) |
| `RFC7606-5.3-1` | The NLRI or Withdrawn Routes field SHALL be considered syntactically incorrect if the length of any included NLRI is greater than 32 (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606NLRIMaxPrefixLengthAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L54). **negative:** `unit/verify` [`TestEnforceRFC7606_InvalidTrailingNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L196). **negative:** `unit/verify` [`TestRFC7606NLRIPrefixLengthTooLongIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L842) |
| `RFC7606-5.3-2` | The NLRI or Withdrawn Routes field SHALL be considered syntactically incorrect if, when parsing NLRI, the length of the last NLRI found exceeds the unconsumed data remaining in the field (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606NLRILastPrefixExactlyFitsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L75). **negative:** `unit/verify` [`TestEnforceRFC7606_InvalidWithdrawnNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L170) |
| `RFC7606-5.3-3` | MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the length of any included NLRI is inconsistent with the given AFI/SAFI (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606MPNLRILengthConsistentWithAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L97). **positive:** `unit/verify` [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L285). **negative:** `unit/verify` [`TestRFC7606NLRIPrefixLengthTooLongIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L895) |
| `RFC7606-5.3-4` | MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if, when parsing NLRI in the attribute, the length of the last NLRI found exceeds the unconsumed data remaining in the attribute (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L286). **negative:** `unit/verify` [`TestRFC7606MPReachNLRIOverrunsAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L225) |
| `RFC7606-5.3-5` | MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the attribute flags are inconsistent with those specified in RFC 4760. NOTE: this routes via §3.j to session reset / AFI-SAFI disable, a STRONGER action than the generic flag-conflict treat-as-withdraw of R005 (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L287). **negative:** `unit/verify` [`TestRFC7606MPReachFlagsInconsistentWithRFC4760`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L257) |
| `RFC7606-5.3-6` | MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the MP_UNREACH_NLRI length is less than 3, or the MP_REACH_NLRI length is less than 5 (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC7606MPMinimumLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L157). **negative:** `unit/verify` [`TestRFC7606MPReachLengthFourIsIncorrect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_minimum_length_test.go#L9). **negative:** `functional/verify` [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L6) |
| `RFC7606-2-6` | When multiple errors dictate different approaches, the strongest action MUST be used; the strength ordering is session reset > AFI/SAFI disable > treat-as-withdraw > attribute discard, as listed in §2 (§2, §3.h) | MUST | 2 | **positive:** `unit/verify` [`TestRFC7606EqualStrengthErrorsDoNotEscalate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L370). **negative:** `unit/verify` [`TestRFC7606StrongestActionWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L316) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7606-5.1-1`](#rfc7606-5.1-1) MP_REACH_NLRI or MP_UNREACH_NLRI (if present) SHALL be encoded as the very first path attribute (§5.1) | {gap}, no test | Ze intentionally emits MP_UNREACH first and MP_REACH last, treating them as NLRI-carrying wire structures rather than descriptive attributes; deliberate design decision recorded in docs/architecture/wire/mp-nlri-ordering.md and disclosed publicly in the docs/features/rfc-status.md RFC 7606 row |
| [`RFC7606-8-1`](#rfc7606-8-1) A new BGP attribute specification MUST define what constitutes malformation and how to handle it (§8) | no test | no test carries this requirement id; annotated {not-applicable}: an obligation on the authors of FUTURE attribute specifications, not on an implementation; Ze has no code path that could satisfy or violate it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7606-2-1`](#rfc7606-2-1)

When treat-as-withdraw is used, the UPDATE message MUST be treated as though all contained routes had been withdrawn (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal (session_test.go) sends a malformed-ORIGIN UPDATE announcing 10.0.0.0/8 and asserts the DISPATCHED body has withdrawn=={0x08,0x0a}, nlri empty, attrLen zero; the message-level TestSynthesizeWithdrawMovesNLRIToWithdrawn and ...ConvertsMPReachToMPUnreach pin the rewrite exactly. Positive TestSessionRFC7606ValidUpdateDispatchesAnnouncement requires a valid UPDATE to stay byte-for-byte an announcement. 2026-07-20 re-audit: coverage STRENGTHENED beyond message synthesis. TestRIBTreatAsWithdrawRemovesInstalledRoute (rib_structured_test.go) now proves the route actually LEAVES the RIB -- Adj-RIB-In Len()==0 and a best-change Withdraw for 10.0.0.0/8 published to the Loc-RIB -- and that a treat-as-withdraw for a never-installed prefix publishes no spurious event. TestRIBTreatAsWithdrawAddPathPreservesPathID proves exactly the re-advertised ADD-PATH path (ID 42) is withdrawn while its sibling (ID 43) for the same prefix survives. Section 2 says the routes MUST be withdrawn; until these, the evidence stopped at 'a withdrawal-shaped message was dispatched'. Assertions are exact and fail on the old suppression bug. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSynthesizeWithdrawConvertsMPReachToMPUnreach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L57) | unit/verify | unproven |
| negative | [`TestSynthesizeWithdrawMovesNLRIToWithdrawn`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L21) | unit/verify | unproven |
| negative | [`TestRIBTreatAsWithdrawAddPathPreservesPathID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_structured_test.go#L166) | unit/verify | unproven |
| negative | [`TestRIBTreatAsWithdrawRemovesInstalledRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_structured_test.go#L92) | unit/verify | unproven |
| negative | [`TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1964) | unit/verify | unproven |
| positive | [`TestSessionRFC7606ValidUpdateDispatchesAnnouncement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L277) | unit/verify | unproven |

### [`RFC7606-2-2`](#rfc7606-2-2)

Attribute discard: malformed attribute MUST be discarded and UPDATE processing continues (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestApplyAttrDiscardMultipleEntries (attr_discard_test.go:256) asserts findAttrByCode(6)/(7) are nil (discarded) while ORIGIN/AS_PATH/NEXT_HOP survive (continues processing); TestSessionRFC7606AttributeDiscardContinues (session_test.go:1990) drives eBGP LOCAL_PREF and requires callbackCount==1 with session Established (UPDATE still dispatched). Positive TestApplyAttrDiscardEmptyEntries (:337) guards against unconditional discard. Both halves observed exactly. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestApplyAttrDiscardMultipleEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L255) | unit/verify | unproven |
| negative | [`TestSessionRFC7606AttributeDiscardContinues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2045) | unit/verify | unproven |
| positive | [`TestApplyAttrDiscardEmptyEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L336) | unit/verify | unproven |

### [`RFC7606-2-3`](#rfc7606-2-3)

Attribute discard MUST NOT be used for attributes affecting route selection or installation (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606RouteSelectionAttributesNeverDiscarded (rfc7606_structural_test.go:426) enumerates ORIGIN/AS_PATH/NEXT_HOP/MED/LOCAL_PREF(iBGP) and asserts NotEqual(AttributeDiscard) AND NotEqual(None) AND DiscardEntries empty with AttrCode pinned per case — a two-sided exclusion that fails if any route-selection attr is discarded or silently accepted. Positive TestRFC7606NonRouteSelectionAttributesAreDiscarded (:497) shows ATOMIC_AGGREGATE/AGGREGATOR ARE discarded, so the MUST NOT is not trivially satisfied by never discarding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606RouteSelectionAttributesNeverDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L418) | unit/verify | unproven |
| positive | [`TestRFC7606NonRouteSelectionAttributesAreDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L489) | unit/verify | unproven |

### [`RFC7606-3.b-1`](#rfc7606-3.b-1)

If Withdrawn Routes Length + Total Attribute Length + 23 exceeds Message Length, Error Subcode MUST be Malformed Attribute List (§3.b)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSessionRFC7606SectionLengthConflictNotification (rfc7606_session_structural_test.go:153) declares Total Attr Length=0xFF over a shorter body and reads the NOTIFICATION off the wire: require.Equal notifBody[1]==NotifyUpdateMalformedAttr (subcode 1) plus callbackCount==0. Positive TestEnforceRFC7606_SectionLengthsExactlyFitAccepted (:114) pins the exact-fit boundary with explicit arithmetic asserts so an off-by-one is caught. Exact subcode assertion fails on non-compliance. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSessionRFC7606SectionLengthConflictNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L154) | unit/verify | unproven |
| positive | [`TestEnforceRFC7606_SectionLengthsExactlyFitAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L115) | unit/verify | unproven |

### [`RFC7606-3.c-1`](#rfc7606-3.c-1)

If Optional or Transitive flag bits conflict with specification, attribute MUST be treated as malformed with treat-as-withdraw, unless the specification for the attribute mandates different handling for incorrect Attribute Flags (§3.c)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606FlagsWellKnownMarkedOptional (rfc7606_test.go:1051) sets ORIGIN Optional bit (0xc0) => require.Equal(TreatAsWithdraw) AttrCode 1 Description '3.c'; TestRFC7606FlagsWellKnownNotTransitive (:1072) clears AS_PATH Transitive => TreatAsWithdraw AttrCode 2. Positive TestRFC7606FlagsOptionalAttributeValid (:1093) accepts correctly-flagged MED (0x80). Exact TAW assertions fail on over-reaction to SessionReset; other attrs valid so the flag error is isolated.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606FlagsWellKnownMarkedOptional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1179) | unit/verify | unproven |
| negative | [`TestRFC7606FlagsWellKnownNotTransitive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1200) | unit/verify | unproven |
| positive | [`TestRFC7606FlagsOptionalAttributeValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1220) | unit/verify | unproven |

### [`RFC7606-3.d-1`](#rfc7606-3.d-1)

If any well-known mandatory attributes are missing, treat-as-withdraw MUST be used (§3.d)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606MissingOrigin (rfc7606_test.go:54) and TestRFC7606MissingASPath (:72) assert require.Equal(TreatAsWithdraw) with AttrCode 1/2; mp_reach/missing_ORIGIN (:2180) and missing_AS_PATH (:2190) do the same on the MP_REACH path. NEXT_HOP correctly not required (discretionary per §3.d note). Positive TestRFC7606ValidUpdate (:111) accepts a full mandatory set. Exact assertions, isolated buffers.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MissingASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L146) | unit/verify | unproven |
| negative | [`TestRFC7606MissingOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L128) | unit/verify | unproven |
| negative | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2308) | unit/verify | unproven |
| positive | [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L183) | unit/verify | unproven |

### [`RFC7606-3.e-1`](#rfc7606-3.e-1)

Treat-as-withdraw MUST be used for malformed ORIGIN, AS_PATH, NEXT_HOP, MULTI_EXIT_DISC, or LOCAL_PREF (§3.e)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Umbrella pinned by exact require.Equal(TreatAsWithdraw) negatives for ORIGIN (rfc7606_test.go:13, len 2), AS_PATH (:334, bad segment type) and LOCAL_PREF iBGP (:888, len 3), plus the Baseline_EBGP/IBGP positive (:1495) asserting well-formed ORIGIN/AS_PATH/NEXT_HOP/MED are ActionNone. Covers 3 of the 5 named members directly; NEXT_HOP/MED malformation is enforced under 7.3-1/7.4-1 with the same exact TAW assertion. Fails on downgrade or over-reaction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606ASPathUnrecognizedSegmentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L447) | unit/verify | unproven |
| negative | [`TestRFC7606LocalPrefIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1016) | unit/verify | unproven |
| negative | [`TestRFC7606MalformedOriginLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L12) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1618) | unit/verify | unproven |

### [`RFC7606-3.f-1`](#rfc7606-3.f-1)

Attribute discard MUST be used for malformed ATOMIC_AGGREGATE or AGGREGATOR (§3.f)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606MalformedAtomicAggregate (rfc7606_test.go:91, len 1) and TestRFC7606AggregatorLen8NoASN4Invalid (:1208) assert require.Equal(AttributeDiscard) AttrCode 6/7; TestRFC7606DiscardEntryReasonCodes cases (attr_discard_test.go:421/435) re-pin AttributeDiscard with the exact discard entry. Positive baseline (:1495) keeps well-formed ATOMIC_AGGREGATE/AGGREGATOR at ActionNone. Exact assertions fail if TAW or None substituted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L421) | unit/verify | unproven |
| negative | [`TestRFC7606AggregatorLen8NoASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1336) | unit/verify | unproven |
| negative | [`TestRFC7606MalformedAtomicAggregate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L165) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1619) | unit/verify | unproven |

### [`RFC7606-3.g-1`](#rfc7606-3.g-1)

If MP_REACH_NLRI or MP_UNREACH_NLRI appears more than once, a NOTIFICATION MUST be sent with Malformed Attribute List (§3.g)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSessionRFC7606SessionResetNotification (session_test.go:1759) sends duplicate MP_REACH_NLRI, reads the NOTIFICATION off the wire and asserts notifBody[0]==code 3 and notifBody[1]==NotifyUpdateMalformedAttr (subcode 1) with callbackCount==0. Message-level TestRFC7606MultipleMPReach (rfc7606_test.go:136) and dup/MP_UNREACH (:2120) pin SessionReset for both attrs; positive TestRFC7606MPReachIPv4NextHopValid (:586) shows a single MP_REACH is not a multiplicity error. Wire subcode observed directly. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged. 2026-08-04 RE-JUDGED over 6 units, two of them new, by a session that did not write them. The checklist text did not move (requirement_sha f0e79a44d92324d0) and the four earlier units are byte-identical. The additions cover a shape none of the four reached: ValidateUpdateRFC7606AddPath counted the MP attributes and judged the count AFTER its walk, so one attribute whose declared length overruns the section returned treat-as-withdraw from inside the loop and the duplicate was never reported. The MUST was skippable with three octets. The check now returns the moment the second MP attribute is seen (message/rfc7606.go, inside the attribute loop). The verdict stays enforced, and the set discriminates: moving the check back after the loop through a `go test -overlay` mutation turns TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk RED with NO error returned at all -- no NOTIFICATION, session kept -- while the wire subcode the requirement names stays proven by TestSessionRFC7606SessionResetNotification (code 3, subcode 1), which both paths reach through the one producer Session.rfc7606SessionReset. ONE CITED UNIT IS WEAK AND IS NOT SILENTLY TAGGED. TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk stays GREEN under that same mutation, so it does not prove the withdrawal half of Section 3.g, which its own doc comment says is why it exists. Its fixture carries two MP_UNREACH attributes, no NLRI and no MP_REACH, and that is exactly the RFC 7606 Section 5.2 shape: the structuralError helper escalates it to session reset whether or not the duplicate was ever judged, so the assertion passes for a neighbouring rule's reason. Question 3 of the audit, a cascade-confounded buffer. The MUST itself is not left unproven -- the message-level duplicate MP_UNREACH case in rfc7606_test.go pins SessionReset on a completed walk -- so what is missing is the intersection the new unit claims. Giving the fixture reachable NLRI removes the Section 5.2 escape. RESOLVED THE SAME DAY, and re-judged here over 7 units. The confounded fixture could not be corrected in place: the native hook in internal/le/hookruntime/writeedit.go refuses an edit to an RFC-tagged test without the owner's approval, which is the right guard. A companion was added instead, TestRFC7606Section3gDuplicateMPUnreachResetsWithRoutesPresent (session_validation_dupmp_unreach_test.go), whose UPDATE announces 10.0.0.0/24 with a NEXT_HOP, so structuralError takes its treat-as-withdraw branch and Section 5.2 cannot produce the verdict. Measured, not taken on report: moving the Section 3.g check back after the walk turns the companion RED with NO error returned at all -- no NOTIFICATION, session kept -- alongside its MP_REACH twin, while the confounded unit stays GREEN. That single run proves the companion discriminates AND reproduces the weakness, in the same fixture set. The confounded unit stays cited and stays recorded here: it is now redundant rather than load-bearing, and deleting the finding would erase why the companion exists. Its own unit is byte-identical to what this audit judged, so nothing was weakened to reach green. Folding the companion back into the twin, with the corrected fixture, still needs the owner's approval. 2026-08-04, SECOND RE-JUDGEMENT of the same day, by a session that wrote none of these units and did not take the earlier measurement on trust. The checklist text still has not moved (requirement_sha f0e79a44d92324d0); two units did. TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk had its FIXTURE corrected in place under an rfc-test-change-approved: 2026-08-04 marker -- it gained a NEXT_HOP and an announced 10.0.0.0/24, and no assertion changed -- and the companion's header comment was re-worded to record the twin as fixed. THE WEAK FINDING RECORDED ABOVE IS RESOLVED, and it is kept above rather than deleted because it is why the companion exists. Measured here with go test -overlay, two mutants over ValidateUpdateRFC7606AddPath (message/rfc7606.go). MUTANT A, the Section 3.g verdict moved back out of the attribute loop to just after it: the three abandoned-walk units go RED (DuplicateMPBeatsAnAbandonedWalk, DuplicateMPUnreachBeatsAnAbandonedWalk, DuplicateMPUnreachResetsWithRoutesPresent), which reproduces the correction -- the corrected twin now discriminates where it previously survived. TestSessionRFC7606SessionResetNotification and TestRFC4271UpdateErrorReportedAsUpdateMessageError stay GREEN under mutant A and that is correct: their bodies carry no framing error, so a post-loop verdict still reaches them. They are the proof of the MUST itself, not of where it is judged. MUTANT B, the check deleted outright: those two go RED as well, along with TestRFC7606MultipleMPReach and the dup/MP_REACH and dup/MP_UNREACH subtests of TestRFC7606SystematicLengthCorruption. The positive is not vacuous either: a third mutant widening the test to mpReachCount > 0 turns TestRFC7606MPReachIPv4NextHopValid RED. Unmutated, both packages are green for these units. THREE RESIDUAL FINDINGS, none of which makes a cited unit unable to fail. (1) POLARITY MISLABEL. All three abandoned-walk tags read 'positive' while their input is a duplicate MP attribute, which VIOLATES this requirement: by the convention in ai/skills/ze-rfc.md they are negatives. The pair still holds on a genuine positive (TestRFC7606MPReachIPv4NextHopValid asserts ActionNone for a single MP_REACH), so the ledger's positive side is overstated by three rather than empty. Fixing it edits an RFC-tagged test and is the owner's call. (2) REDUNDANCY. With the twin's fixture corrected, DuplicateMPUnreachBeatsAnAbandonedWalk and DuplicateMPUnreachResetsWithRoutesPresent now carry byte-identical fixtures and identical assertions. Two units, one proof. The fold-back the companion's comment proposes is the right end state. (3) THE WIRE SUBCODE IS PROVEN ON THE MP_REACH LEG ONLY. TestSessionRFC7606SessionResetNotification reads code 3 / subcode 1 off the wire for a duplicate MP_REACH; no test reads the wire for a duplicate MP_UNREACH. Both legs return the one verdict from message/rfc7606.go and both reach Session.rfc7606SessionReset (session_validation.go), which passes NotifyUpdateMessage and NotifyUpdateMalformedAttr unconditionally, so the second half of the MUST holds by construction on that leg rather than by observation. 2026-08-04, THIRD RE-JUDGEMENT of the same day, and the shortest of them, because the fingerprints did the reading. FINDING (1), THE POLARITY MISLABEL, IS RESOLVED. The three abandoned-walk tags now read `negative`. That is the correct label: `ai/skills/ze-rfc.md` defines the polarity over the INPUT, not over the verdict -- its stated reason is that "a negative-only test passes if the code rejects everything; a positive-only test passes if it accepts everything", which only discriminates when the positive carries a CONFORMING input. A duplicate MP attribute is the violation RFC 7606 Section 3.g names, so a test asserting Ze rejects it is a negative, exactly as the worked example tags "ORIGIN length 2 is treated as withdraw" and as the sibling tag at message/rfc7606_test.go:214 already read. THE `no assertion changed` CLAIM WAS VERIFIED RATHER THAN ACCEPTED. Reconstructing each of the three units by deleting only the POLARITY-CORRECTED approval comment blocks and reverting `negative` to `positive` reproduces 62d48a87d1b0bf06, dd9ccddf1d385643 and 896b8ad0bd0112d8 -- the exact unit shas the second re-judgement recorded. Under the whitespace-and-blank-line normalisation those hashes use, that is proof no executable line moved: fixture bytes, inputs and assertions are byte-identical to what was mutation-tested earlier today, so MUTANT A and MUTANT B above carry over unchanged and were not re-run. The four untouched units hash to their recorded values as well, the positive TestRFC7606MPReachIPv4NextHopValid among them (e8d83289676a1ed9). BOTH POLARITIES SURVIVE: the row is 1 positive / 6 negative, and `./le rfc check` reports the stale verdict as its only violation -- no missing-polarity finding and no coverage-ratchet fire, the positive side being overstated before rather than the negative side empty now. Both packages are green unmutated. TWO RESIDUAL FINDINGS STAND, unchanged and re-stated. REDUNDANCY: DuplicateMPUnreachBeatsAnAbandonedWalk and DuplicateMPUnreachResetsWithRoutesPresent still carry byte-identical fixtures and assertions, so the ledger counts one proof twice; the fold-back is the owner's call because it deletes a tracked test. WIRE SUBCODE ON ONE LEG: no test reads code 3 / subcode 1 off the wire for a duplicate MP_UNREACH, so that half of the MUST holds by construction through Session.rfc7606SessionReset rather than by observation. 2026-08-04, FOURTH RE-JUDGEMENT of the same day, by a session that wrote none of these units. The checklist text still has not moved (requirement_sha f0e79a44d92324d0) and NO existing unit moved: all seven previously recorded unit shas recompute to their recorded values, the positive TestRFC7606MPReachIPv4NextHopValid among them (e8d83289676a1ed9). One unit is NEW, session_dupmp_unreach_wire_test.go::TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire, and it closes RESIDUAL FINDING (3) recorded above -- the wire subcode proven on the MP_REACH leg only. IT OBSERVES THE WIRE, verified through the producer chain rather than inferred: rfc7606SessionReset -> logNotifyErr -> sendNotificationWithin -> writeMessageWithin, which encodes the Notification with msg.WriteTo into the session write buffer and flushes it to the session conn (session_write.go:113-141). The harness setupEstablishedSessionEBGP builds a net.Pipe, hands the server end to the session and the client end to the test, so the asserted bytes are the ones a peer receives; the test parses them with message.ParseHeader and reads notifBody[0] and notifBody[1] off the encoded body. No shortcut and no callback stands in for the connection. MEASURED, NOT TAKEN ON REPORT. Two mutants over unmodified HEAD producers (git diff on message/rfc7606.go and reactor/session_validation.go is empty). MUTANT S, the subcode in rfc7606SessionReset swapped to NotifyUpdateMissingAttr: exactly three units go RED -- the new one, TestSessionRFC7606SectionLengthConflictNotification and TestSessionRFC7606SessionResetNotification -- reproducing the author's measurement independently. MUTANT B, the Section 3.g verdict disabled outright: the new unit goes RED with `An error is expected but got nil` -- no session reset, no NOTIFICATION, session kept -- alongside the three abandoned-walk units, TestSessionRFC7606SessionResetNotification, TestRFC7606MultipleMPReach and both dup subtests of TestRFC7606SystematicLengthCorruption. THAT SECOND RESULT IS THE ONE THAT MATTERS. rfc7606SessionReset passes code 3 / subcode 1 for EVERY session reset, so a wire test whose fixture reaches a reset by a neighbouring rule would assert the same two bytes and prove nothing about 3.g -- the cascade-confound that made DuplicateMPUnreachBeatsAnAbandonedWalk weak earlier today. Its fixture has no NLRI and no MP_REACH, the Section 5.2 shape, so the risk was live. It does not fire: Section 5.2's escalation is gated on `strongest > RFC7606ActionAttributeDiscard` (message/rfc7606.go:503) and this UPDATE's ORIGIN, empty AS_PATH and two well-formed MP_UNREACH attributes record no error at all, so with 3.g removed the walk returns ActionNone. The duplicate check is the only thing that turns this input into a NOTIFICATION. POLARITY: `negative` is correct. ai/skills/ze-rfc.md fixes polarity on the INPUT, and a duplicate MP_UNREACH is the violation Section 3.g names. BOTH POLARITIES HOLD: the row is 1 positive / 7 negative. ONE RESIDUAL FINDING STANDS, unchanged. REDUNDANCY: DuplicateMPUnreachBeatsAnAbandonedWalk and DuplicateMPUnreachResetsWithRoutesPresent still carry byte-identical fixtures and assertions, and mutant B reddens both together, which is the ledger counting one proof twice. The fold-back stays the owner's call because it deletes a tracked test. The wire-subcode finding is now CLOSED by observation rather than by construction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MultipleMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L210) | unit/verify | unproven |
| negative | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2247) | unit/verify | unproven |
| negative | [`TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_dupmp_unreach_wire_test.go#L30) | unit/verify | unproven |
| negative | [`TestSessionRFC7606SessionResetNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L1812) | unit/verify | unproven |
| negative | [`TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_test.go#L48) | unit/verify | unproven |
| negative | [`TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_test.go#L79) | unit/verify | unproven |
| negative | [`TestRFC7606Section3gDuplicateMPUnreachResetsWithRoutesPresent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_dupmp_unreach_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L696) | unit/verify | unproven |

### [`RFC7606-3.h-1`](#rfc7606-3.h-1)

When multiple attribute errors exist with same approach, the specified approach MUST be used (§3.h)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606CollectAllErrors (rfc7606_test.go:1393) uses two attribute-discard errors (ATOMIC_AGG + AGGREGATOR) and asserts require.Equal(AttributeDiscard) with DiscardEntries len 2 covering codes 6 and 7 — fails on escalation or on dropping either entry. The {single-polarity,negative} annotation is sound: 'multiple attribute errors exist' has no error-free instance, so no positive can be constructed. Exact assertion, isolated buffer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606CollectAllErrors`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1520) | unit/verify | unproven |

### [`RFC7606-3.h-2`](#rfc7606-3.h-2)

When multiple attribute errors exist with different approaches, the strongest action MUST be used (§3.h)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606MultipleErrorsStrongest (rfc7606_test.go:1369) combines ATOMIC_AGG discard (weaker) with COMMUNITY TAW (stronger) and asserts require.Equal(TreatAsWithdraw) AttrCode 8 — fails if the weaker action wins or if it over-escalates to SessionReset. The {single-polarity,negative} annotation holds (premise is that errors exist). The stronger boundary (TAW vs SessionReset) is separately pinned by 2-6.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MultipleErrorsStrongest`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1496) | unit/verify | unproven |

### [`RFC7606-3.i-1`](#rfc7606-3.i-1)

The Withdrawn Routes field MUST be checked for syntactic correctness in the same manner as the NLRI field (§3.i)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative TestEnforceRFC7606_InvalidWithdrawnNLRI (session_validate_test.go:87) puts prefix-length 33 in the Withdrawn Routes field (withdrawn-only body, isolated) and asserts require.Equal(SessionReset)+err — proving the withdrawn field IS syntax-checked like NLRI (mirror of TestEnforceRFC7606_InvalidTrailingNLRI for the NLRI field). Positive TestEnforceRFC7606_ValidWithdrawnNLRIAccepted (rfc7606_session_structural_test.go:72) drives a deliberately non-empty valid withdrawn field through enforceRFC7606 => ActionNone, unrewritten. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnforceRFC7606_InvalidWithdrawnNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L172) | unit/verify | unproven |
| positive | [`TestEnforceRFC7606_ValidWithdrawnNLRIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L73) | unit/verify | unproven |

### [`RFC7606-3.j-1`](#rfc7606-3.j-1)

If NLRI/MP_REACH/MP_UNREACH cannot be successfully parsed, session reset (or AFI/SAFI disable) MUST be followed (§3.j)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-audited 2026-08-27. Exact SessionReset negatives cover MP_REACH length 4, MP_UNREACH length 2, IPv4 /33, IPv6 /129, and an NLRI overrun; descriptions or AttrCode isolate the failing parser rule. Parseable IPv4 and IPv6 NLRIs require no error. TestEnforceRFC7606_InvalidTrailingNLRI drives the production Session.enforceRFC7606 path, and test/plugin/rfc7606-reset.ci observes the resulting code-3/subcode-1 NOTIFICATION on the wire. The compiled passivePlugin13 replacement only runs the plugin lifecycle and cannot satisfy that ze-peer expectation. Any downgrade to TreatAsWithdraw, acceptance of an unparseable field, or reset of a parseable field fails an exact assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MPReachTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L798) | unit/verify | unproven |
| negative | [`TestRFC7606MPUnreachTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1411) | unit/verify | unproven |
| negative | [`TestRFC7606NLRIOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L928) | unit/verify | unproven |
| negative | [`TestRFC7606NLRIPrefixLengthTooLongIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L844) | unit/verify | unproven |
| negative | [`TestRFC7606NLRIPrefixLengthTooLongIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L897) | unit/verify | unproven |
| negative | [`TestEnforceRFC7606_InvalidTrailingNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L198) | unit/verify | unproven |
| negative | [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L9) | functional/verify | unproven |
| positive | [`TestRFC7606NLRIPrefixLengthValidIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L824) | unit/verify | unproven |
| positive | [`TestRFC7606NLRIPrefixLengthValidIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L868) | unit/verify | unproven |

### [`RFC7606-4-1`](#rfc7606-4-1)

In attribute length conflicts, treat-as-withdraw MUST be used, unless some other more severe error dictates a stronger approach (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_structural_test.go:248 (TestRFC7606AttributeLengthConflictTreatAsWithdraw) drives a COMMUNITY declaring len 8 with only 4 octets present and asserts require.Equal(TreatAsWithdraw) + require.Equal(AttrCode 8) + Contains('Section 4'); the branch it pins is rfc7606.go:222-231 (pos+attrLen>len). Valid ORIGIN/AS_PATH/NEXT_HOP filler isolates the conflict, and the exact-action assertion would fail on the session-reset over-reaction. Positive rfc7606_test.go:1495 (Baseline_EBGP) pins a conflict-free set to ActionNone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AttributeLengthConflictTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L237) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1620) | unit/verify | unproven |

### [`RFC7606-5.1-1`](#rfc7606-5.1-1)

MP_REACH_NLRI or MP_UNREACH_NLRI (if present) SHALL be encoded as the very first path attribute (§5.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} is honest and confirmed: buildUpdatePayload (wireu/split.go:440-476) writes the attribute section as baseAttrs, then mpUnreach, then mpReach (:463-470), so MP_REACH is emitted LAST, not 'the very first path attribute' as §5.1 SHALL requires. Disclosed in docs/architecture/wire/mp-nlri-ordering.md ('MP_REACH last: Intentionally non-compliant') and rfc-status.md gap (1). 2026-07-29 (plan/spec-rfcgate-3-audit-teeth.md): the `code` map below transcribes the producing code this note already names, so the claim is falsifiable. Nothing was re-judged: with neither tests nor code fingerprinted this verdict's freshness test was `{} == {}` and it could never go stale, i.e. nobody was ever asked to look again if the gap closed. The fingerprint is the enclosing FUNCTION's span, not the file, because producer files churn far more than test files and a file-level hash here would just manufacture a new false-stale class (spec R-5).

No test carries RFC7606-5.1-1, so no unit is bound to it.

### [`RFC7606-5.1-2`](#rfc7606-5.1-2)

An UPDATE message MUST NOT contain more than one of: non-empty Withdrawn Routes, non-empty NLRI, MP_REACH_NLRI, MP_UNREACH_NLRI (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-audited 2026-08-27. NLRIBearingFieldCount identifies each of the four Section 5.1 fields, both message and wire splitters start from an all-four-fields UPDATE and require every output to carry at most one, and buildFwdBody tests cover same-context and re-encoded relay branches. Compliant one-field inputs must remain one unchanged object. test/plugin/rfc7606-relay-one-field.ci sends Withdrawn Routes plus legacy NLRI and requires an exact withdraw-only frame before a separate announcement. Its compiled routeServerReplay13 observer waits for both EoRs and the forwarded fence prefix; it does not inspect or synthesize the two load-bearing peer frames. Removing either split path, dropping a field, or splitting a compliant update fails.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSplitCompliantPassesThroughCompliantUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L151) | unit/verify | unproven |
| negative | [`TestForwardCompliantShapeKeepsZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L116) | unit/verify | unproven |
| negative | [`TestSplitWireUpdateCompliantShapeUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc7606_shape_test.go#L58) | unit/verify | unproven |
| positive | [`TestNLRIBearingFieldCountEveryCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L55) | unit/verify | unproven |
| positive | [`TestSplitCompliantSplitsMixedUpdateThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_shape_test.go#L119) | unit/verify | unproven |
| positive | [`TestForwardSplitsMixedShapeAcrossContextsThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L156) | unit/verify | unproven |
| positive | [`TestForwardSplitsMixedShapeSameContextThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_forward_body_test.go#L51) | unit/verify | unproven |
| positive | [`TestSplitWireUpdateSplitsMixedShapeThatFits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc7606_shape_test.go#L23) | unit/verify | unproven |
| positive | [`TestSplitWireUpdateDoesNotManufactureEoR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/split_eor_test.go#L32) | unit/verify | unproven |
| positive | [`rfc7606-relay-one-field.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-relay-one-field.ci#L4) | functional/verify | unproven |

### [`RFC7606-5.1-3`](#rfc7606-5.1-3)

Implementation MUST still be prepared to receive MP_REACH/MP_UNREACH in any position or combination from older speakers (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/le/interoplab/bgp/check_rfc.go::checkRFC7606MixedUpdate moved. Re-audited 2026-08-27. {single-polarity} positive remains correct because rejection cannot conform. Four message tests require ActionNone for late MP_UNREACH, both distinct MP attributes, MP_REACH plus legacy NLRI, and MP_UNREACH plus legacy NLRI; the earlier late-MP_REACH unit covers the other position. test/plugin/rfc7606-receive-combinations.ci drives those four shapes through the daemon and requires distinct relayed components, while rfc7606-relay-one-field requires the legacy Withdrawn Routes plus NLRI combination and rejects an UPDATE-error NOTIFICATION. routeServerReplay13 only waits for EoR and a final forwarded prefix, so the ze-peer expectations remain the discriminating evidence. The native interop check runs scenarioOperations plus withdrawal extras: FRR must keep the session, install the announcement, observe the withdrawal, and not retain the withdrawn route. A receive-side position or combination restriction prevents one of these exact outcomes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7606Section51AcceptsMPReachWithLegacyNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L62) | unit/verify | unproven |
| positive | [`TestRFC7606Section51AcceptsMPUnreachAfterOtherAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L41) | unit/verify | unproven |
| positive | [`TestRFC7606Section51AcceptsMPUnreachWithLegacyNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L72) | unit/verify | unproven |
| positive | [`TestRFC7606Section51AcceptsReachAndUnreachTogether`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_section51_receive_test.go#L51) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L697) | unit/verify | unproven |
| positive | [`checkRFC7606MixedUpdate`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L607) | interop/nightly | unproven |
| positive | [`rfc7606-receive-combinations.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-receive-combinations.ci#L3) | functional/verify | unproven |
| positive | [`rfc7606-relay-one-field.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-relay-one-field.ci#L8) | functional/verify | unproven |

### [`RFC7606-5.2-1`](#rfc7606-5.2-1)

For an UPDATE containing path attributes OTHER THAN MP_UNREACH_NLRI that encodes no reachable NLRI, if any path attribute error specifies an approach other than attribute discard, session reset MUST be used. The "other than MP_UNREACH_NLRI" clause exempts End-of-RIB, which §5.2 defines as an UPDATE carrying only an MP_UNREACH_NLRI encoding no NLRI (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negatives pin the escalation exactly: rfc7606_test.go:1328 (malformed ORIGIN len=2) and :2142 (ORIGIN len=5) both assert require.Equal(SessionReset) + Contains('5.2'), driving rfc7606.go:336-342 (!hasNLRI && mpReachCount==0 && strongest>AttributeDiscard). Positives pin both exemptions: :1348/:2155 assert attribute-discard-only stays AttributeDiscard (NOT escalated), and :1308 (MP_UNREACH-only, no NLRI, no error) asserts ActionNone. The 'other than MP_UNREACH' EoR clause is only exercised in its no-error form. 2026-08-04 RE-JUDGED over 7 units, two of them new, by a session that did not write them. The checklist text did not move (requirement_sha ca4c9ceb1871b67f) and the five earlier units are byte-identical. The additions close the same abandoned-walk class as the Section 3.g and Section 5.4 ones: the escalation was judged after the attribute walk, so an UPDATE with attributes, no NLRI and a truncated attribute header returned treat-as-withdraw, and SynthesizeWithdrawFamilies produces no body at all for such an UPDATE -- ze consumed it and told the peer nothing where Section 5.2 requires a session reset. Both new units were measured, not read. Making the structuralError helper skip the escalation turns exactly TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned RED and nothing else. Making it escalate unconditionally turns exactly TestRFC7606Section52LeavesAnUpdateWithNLRIAlone RED, which is what proves the negative is a real boundary rather than an absence: an UPDATE that does carry reachable NLRI must stay treat-as-withdraw, or any peer would hold a one-octet way to drop the session. The pair is two inputs with opposite expected outcomes, and each pins the exact RFC7606Action rather than a floor.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606NoNLRIEscalation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1457) | unit/verify | unproven |
| negative | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2269) | unit/verify | unproven |
| negative | [`TestRFC7606Section52LeavesAnUpdateWithNLRIAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_noreach_test.go#L69) | unit/verify | unproven |
| positive | [`TestRFC7606MPUnreachMinValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1437) | unit/verify | unproven |
| positive | [`TestRFC7606NoNLRIAttributeDiscardNoEscalation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1477) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2283) | unit/verify | unproven |
| positive | [`TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_noreach_test.go#L40) | unit/verify | unproven |

### [`RFC7606-5.4-1`](#rfc7606-5.4-1)

A BGP speaker advertising typed address family support MUST discard routes with unrecognized NLRI types, unless the relevant specification for that address family specifies otherwise (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/le/interoplab/bgp/check_rfc.go::checkRFC7606TypedNLRIDiscard moved. Re-audited 2026-08-27 across all 15 tagged units and the ingress producer. Positive unit evidence proves EVPN, MCAST-VPN and BGP-MUP recognized routes survive while an unrecognized route between them is removed; ADD-PATH offsets, MP_UNREACH, treat-as-withdraw synthesis, abandoned attribute walks, empty results, and unparseable typed framing have distinct exact outcomes. The BGP-LS negative requires an overriding family to preserve the same unknown type. The three functional fixtures retain complementary contains/reject wire assertions; compiled routeServerReplay13 only waits for EoRs and a later IPv4 fence. The native interop speaker oracle parses every relayed EVPN NLRI, fails on any type outside 1..5, and the checker requires result=PASS, established=yes, route-bearing-updates>=1 and evpn-nlri>=1, so absence is non-vacuous. Removing discard fails reject/PASS, while blanket or over-discard fails the surviving-route contains checks and BGP-LS negative. Targeted re-audit 2026-08-28: rfc7606-54-bgpls-override-propagates.ci only replaced its Python readiness observer with compiled routeServerReplay13 and enabled ze-peer linger. Its source UPDATE and contiguous known-plus-unknown BGP-LS contains assertion did not change. The observer waits for EoRs and the later IPv4 fence only; it cannot make the BGP-LS assertion pass. If Ze drops, rewrites, or reorders unknown type 99, the receiver no longer contains the asserted bytes. Targeted re-audit 2026-08-29: TestRFC7606Section54PropagatesUnknownBGPLSType changed its KNOWN fixture only, from a two-octet Node stub to a well-formed Node NLRI carrying its Protocol-ID, Identifier and a Local Node Descriptors TLV. Its assertion is unchanged: the MP_REACH survives and its NLRI section is byte-identical to what arrived, unknown type 99 included. The edit STRENGTHENS the case rather than weakening it. The old stub could not hold a Protocol-ID and Identifier, so RFC 9552 Section 8.2.2's syntactic walk would drop it as malformed, and the surviving-bytes assertion could have been satisfied by a different rule than the one under test. Discrimination is unchanged and rests on the nlritype registry: BGP-LS registers no recognizer, so the Section 5.4 filter does not fire for it. Register one that rejects type 99 and the equality assertion fails on the first differing byte. Re-ran the fourteen TestRFC7606Section54 units: all pass.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606Section54PropagatesUnknownBGPLSType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L187) | unit/verify | unproven |
| negative | [`rfc7606-54-bgpls-override-propagates.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-bgpls-override-propagates.ci#L4) | functional/verify | unproven |
| positive | [`mup/TestRecognizeNLRIRejectsUnimplementedTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606_test.go#L50) | unit/verify | unproven |
| positive | [`mvpn/TestRecognizeNLRIRejectsUnimplementedTypes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mvpn/rfc7606_test.go#L45) | unit/verify | unproven |
| positive | [`TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC7606Section54ReadsTypedNLRIUnderAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go#L89) | unit/verify | unproven |
| positive | [`TestRFC7606Section54DiscardsUnrecognizedEVPNType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L149) | unit/verify | unproven |
| positive | [`TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L254) | unit/verify | unproven |
| positive | [`TestRFC7606Section54SessionResetsUnparseableTypedNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_test.go#L336) | unit/verify | unproven |
| positive | [`TestRFC7606Section54DiscardsUnrecognizedMUPType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC7606Section54DiscardsUnrecognizedMVPNType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC7606Section54FiltersTreatAsWithdrawSynthesis`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validation_nlritype_typed_test.go#L159) | unit/verify | unproven |
| positive | [`checkRFC7606TypedNLRIDiscard`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L711) | interop/nightly | unproven |
| positive | [`rfc7606-54-discard-unrecognized-mup-nlri.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci#L4) | functional/verify | unproven |
| positive | [`rfc7606-54-discard-unrecognized-nlri.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-54-discard-unrecognized-nlri.ci#L4) | functional/verify | unproven |

### [`RFC7606-7.1-1`](#rfc7606-7.1-1)

ORIGIN: malformed if length != 1 or undefined value; treat-as-withdraw (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-audited 2026-08-27. Isolated exact negatives cover both Section 7.1 clauses: ORIGIN length 2 and defined-length value 3 each require TreatAsWithdraw, AttrCode 1, and a Section 7.1 description. Three positives accept exactly values 0, 1 and 2 with ActionNone. test/plugin/rfc7606-withdraw.ci adds end-to-end session-survival proof: after the malformed ORIGIN, a second valid route must reach the peer. The compiled rfc7606Withdraw13 observer waits for establishment, sends the first route, gates on the post-malformation update counter, then sends the second route; it cannot make the peer expectation pass after a reset. Acceptance, SessionReset over-reaction, the wrong attribute, or rejecting a defined value each fail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MalformedOriginLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L11) | unit/verify | unproven |
| negative | [`TestRFC7606OriginValueInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L594) | unit/verify | unproven |
| negative | [`rfc7606-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-withdraw.ci#L8) | functional/verify | unproven |
| positive | [`TestRFC7606OriginValueEGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L560) | unit/verify | unproven |
| positive | [`TestRFC7606OriginValueIGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L543) | unit/verify | unproven |
| positive | [`TestRFC7606OriginValueIncomplete`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L577) | unit/verify | unproven |

### [`RFC7606-7.2-1`](#rfc7606-7.2-1)

AS_PATH: malformed if unrecognized segment type, overrun, underrun, or zero-length segment; treat-as-withdraw (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. All four §7.2 malformation types have isolated exact negatives, each asserting TreatAsWithdraw + AttrCode 2 plus a distinguishing Description substring: unrecognized segment type (rfc7606_test.go:344 'segment type'), overrun (:366 'overrun'), underrun (:389 'underrun'), zero-length segment (:414 'zero'). The substring assertions prevent passing via a neighbouring rule; buffers keep ORIGIN/NEXT_HOP valid so only AS_PATH is malformed. Positives :290/:312 accept valid AS_SEQUENCE/AS_SET (Action None).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606ASPath4ByteASNOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2377) | unit/verify | unproven |
| negative | [`TestRFC7606ASPathSegmentOverrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L469) | unit/verify | unproven |
| negative | [`TestRFC7606ASPathSegmentUnderrun`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L491) | unit/verify | unproven |
| negative | [`TestRFC7606ASPathUnrecognizedSegmentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L446) | unit/verify | unproven |
| negative | [`TestRFC7606ASPathZeroSegmentLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L518) | unit/verify | unproven |
| positive | [`TestRFC7606ASPath4ByteASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1384) | unit/verify | unproven |
| positive | [`TestRFC7606ASPathValidSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L403) | unit/verify | unproven |
| positive | [`TestRFC7606ASPathValidSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L425) | unit/verify | unproven |

### [`RFC7606-7.3-1`](#rfc7606-7.3-1)

NEXT_HOP: malformed if length != 4; treat-as-withdraw (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_test.go:1642 case 'padded/NEXT_HOP_len8_garbage' is in the exactAction map (:1772), so the loop at :1780 asserts require.Equal(TreatAsWithdraw) AND require.Equal(AttrCode 3); buffer is ORIGIN+AS_PATH valid then NEXT_HOP len 8, isolating a single error. Over-reaction to SessionReset would fail the exact Action equal. Positive :111 TestRFC7606ValidUpdate accepts NEXT_HOP len 4 (Action None).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1769) | unit/verify | unproven |
| positive | [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L184) | unit/verify | unproven |

### [`RFC7606-7.4-1`](#rfc7606-7.4-1)

MULTI_EXIT_DISC: malformed if length != 4; treat-as-withdraw (§7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_test.go:1656 case 'padded/MED_len8_garbage' is in the exactAction map (:1773), asserting require.Equal(TreatAsWithdraw) + AttrCode 4 at :1780/:1787; buffer keeps ORIGIN/AS_PATH/NEXT_HOP valid so MED len 8 is the only error. Positive :1093 TestRFC7606FlagsOptionalAttributeValid accepts MED len 4 (Action None). Both under- and over-compliance fail the exact equal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1783) | unit/verify | unproven |
| positive | [`TestRFC7606FlagsOptionalAttributeValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1221) | unit/verify | unproven |

### [`RFC7606-7.5-1`](#rfc7606-7.5-1)

LOCAL_PREF from eBGP: attribute discard (§7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_test.go:846 TestRFC7606LocalPrefEBGPDiscard sends a valid-LENGTH (4) LOCAL_PREF over eBGP and asserts require.Equal AttributeDiscard + AttrCode 5 + '7.5', so the discard is attributable purely to eBGP not to length. The positive at :868 sends the identical bytes over iBGP and gets Action None, isolating the eBGP rule. Reinforced by attr_discard_test.go:408 (exact AttributeDiscard + Code 5 + Reason EBGPInvalid) and reactor test session_test.go:1990 which proves the session stays Established (not reset) and the UPDATE is still dispatched. 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L407) | unit/verify | unproven |
| negative | [`TestRFC7606LocalPrefEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L974) | unit/verify | unproven |
| negative | [`TestSessionRFC7606AttributeDiscardContinues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_test.go#L2046) | unit/verify | unproven |
| positive | [`TestRFC7606LocalPrefIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L995) | unit/verify | unproven |

### [`RFC7606-7.5-2`](#rfc7606-7.5-2)

LOCAL_PREF from iBGP: malformed if length != 4; treat-as-withdraw (§7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_test.go:888 TestRFC7606LocalPrefIBGPInvalid sends iBGP LOCAL_PREF len 3 and asserts require.Equal TreatAsWithdraw + AttrCode 5 + '7.5'; buffer isolates the malformed LOCAL_PREF. Positive :868 accepts iBGP LOCAL_PREF len 4 (Action None). The exact Action equal fails on both non-detection and over-reaction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606LocalPrefIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1015) | unit/verify | unproven |
| positive | [`TestRFC7606LocalPrefIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L996) | unit/verify | unproven |

### [`RFC7606-7.6-1`](#rfc7606-7.6-1)

ATOMIC_AGGREGATE: malformed if length != 0; attribute discard (§7.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_test.go:91 TestRFC7606MalformedAtomicAggregate sends ATOMIC_AGGREGATE len 1 and asserts require.Equal AttributeDiscard + AttrCode 6 + '7.6', with the rest of the attribute set valid (isolated). Positive baseline at :1495 accepts ATOMIC_AGGREGATE len 0 (exact require.Equal Action None over the full set). Reinforced by attr_discard_test.go:422 (exact AttributeDiscard + Code 6 + Reason InvalidLength).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L420) | unit/verify | unproven |
| negative | [`TestRFC7606MalformedAtomicAggregate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1622) | unit/verify | unproven |

### [`RFC7606-7.7-1`](#rfc7606-7.7-1)

AGGREGATOR: malformed if length != 6 (without 4-byte AS) or != 8 (with 4-byte AS); attribute discard (§7.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both length variants are pinned in both directions: positives accept len 6 without asn4 (rfc7606_test.go:1163) and len 8 with asn4 (:1185), each Action None; negatives assert require.Equal AttributeDiscard + AttrCode 7 + '7.7' for len 8 without asn4 (:1208) and len 6 with asn4 (:1232), so the capability-dependent expected length is genuinely exercised. Reinforced by attr_discard_test.go:436 (exact AttributeDiscard + Code 7 + Reason InvalidLength).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L434) | unit/verify | unproven |
| negative | [`TestRFC7606AggregatorLen6WithASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1360) | unit/verify | unproven |
| negative | [`TestRFC7606AggregatorLen8NoASN4Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1335) | unit/verify | unproven |
| positive | [`TestRFC7606AggregatorLen6NoASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1291) | unit/verify | unproven |
| positive | [`TestRFC7606AggregatorLen8WithASN4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1313) | unit/verify | unproven |

### [`RFC7606-7.8-1`](#rfc7606-7.8-1)

Community: malformed if length is zero or not a multiple of 4; treat-as-withdraw (§7.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. both malformation clauses of 7.8 now proven. The length-5 negative (TestRFC7606MalformedCommunityLength) asserts exact TreatAsWithdraw + AttrCode 8 + 'not a multiple of 4'; the new zero-length negative (TestRFC7606CommunityZeroLength) asserts TreatAsWithdraw + AttrCode 8 + 'is zero' over an otherwise well-formed UPDATE. Positive is length 4. validateCommunityAttr (rfc7606.go) names which clause fired.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606CommunityZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L62) | unit/verify | unproven |
| negative | [`TestRFC7606MalformedCommunityLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L31) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1623) | unit/verify | unproven |

### [`RFC7606-7.9-1`](#rfc7606-7.9-1)

ORIGINATOR_ID from eBGP: attribute discard (§7.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative pins the exact eBGP-context discard: attr_discard_test.go:449 (originator_id_ebgp_reason_1, isIBGP=false, code 9) asserts Action==AttributeDiscard, DiscardEntries len 1, Code==9, Reason==DiscardReasonEBGPInvalid; rfc7606_test.go:913 repeats AttributeDiscard+AttrCode 9+"7.9". Positive rfc7606_test.go:935 sends the identical ORIGINATOR_ID from iBGP and asserts ActionNone, pinning the context dependence. Fails if the code stopped discarding eBGP ORIGINATOR_ID.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L449) | unit/verify | unproven |
| negative | [`TestRFC7606OriginatorIDEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1041) | unit/verify | unproven |
| positive | [`TestRFC7606OriginatorIDIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1062) | unit/verify | unproven |

### [`RFC7606-7.9-2`](#rfc7606-7.9-2)

ORIGINATOR_ID from iBGP: malformed if length != 4; treat-as-withdraw (§7.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive rfc7606_test.go:935 (len-4 iBGP ORIGINATOR_ID) asserts ActionNone; negative rfc7606_test.go:954 (len-5 iBGP) asserts the exact TreatAsWithdraw + AttrCode 9 + Description "7.9", matching validateOriginatorIDAttr (rfc7606.go:517). Fails if a malformed-length iBGP ORIGINATOR_ID were accepted. Minor: only len 5 exercises the !=4 predicate, so a hypothetical >4 bug would slip len 3, but the len-4-accept/len-5-reject boundary is pinned.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606OriginatorIDIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1082) | unit/verify | unproven |
| positive | [`TestRFC7606OriginatorIDIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1063) | unit/verify | unproven |

### [`RFC7606-7.10-1`](#rfc7606-7.10-1)

CLUSTER_LIST from eBGP: attribute discard (§7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative attr_discard_test.go:462 (cluster_list_ebgp_reason_1, isIBGP=false) asserts AttributeDiscard + Code 10 + Reason DiscardReasonEBGPInvalid; rfc7606_test.go:979 repeats AttributeDiscard+AttrCode 10+"7.10". Positive rfc7606_test.go:1001 sends the same CLUSTER_LIST from iBGP and asserts ActionNone. Pins the eBGP-context discard of validateClusterListAttr (rfc7606.go:530).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiscardEntryReasonCodes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/attr_discard_test.go#L462) | unit/verify | unproven |
| negative | [`TestRFC7606ClusterListEBGPDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1107) | unit/verify | unproven |
| positive | [`TestRFC7606ClusterListIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1128) | unit/verify | unproven |

### [`RFC7606-7.10-2`](#rfc7606-7.10-2)

CLUSTER_LIST from iBGP: malformed if length is zero or not a multiple of 4; treat-as-withdraw (§7.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive rfc7606_test.go:1001 (len-8 iBGP) asserts ActionNone; negative rfc7606_test.go:1026 (len-5 iBGP) asserts exact TreatAsWithdraw + AttrCode 10 + "7.10". Coverage gap acknowledged in the test comment (1022-1023): only the 'not a multiple of 4' clause is tagged; the zero-length clause is untagged (cascade-confounded), so removing the `length==0` check in validateClusterListAttr (rfc7606.go:538) would not be caught by the tagged tests, though the code does implement both clauses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606ClusterListIBGPInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1154) | unit/verify | unproven |
| positive | [`TestRFC7606ClusterListIBGPValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1129) | unit/verify | unproven |

### [`RFC7606-7.11-1`](#rfc7606-7.11-1)

MP_REACH_NLRI with inconsistent Next Hop length: session reset or AFI/SAFI disable (§7.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Three negatives assert the exact SessionReset + AttrCode 14 + "7.11" across families for an inconsistent next-hop length: IPv6 NH_LEN=5 (rfc7606_test.go:557), IPv4 NH_LEN=3 (:609), VPNv4 NH_LEN=4 (:660). Four positives assert ActionNone for consistent lengths (IPv6/16 :504, IPv6/32 :529, IPv4/4 :586, VPNv4/12 :634). validateMPReachNextHop (rfc7606.go:695-720) checks nhLen against the per-AFI/SAFI ValidNextHopLens table, so the reset fires for NH inconsistency specifically, not a structural cascade. Would fail if Ze weakened the action below session reset.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MPReachIPv4NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L722) | unit/verify | unproven |
| negative | [`TestRFC7606MPReachIPv6NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L670) | unit/verify | unproven |
| negative | [`TestRFC7606MPReachVPNv4NextHopInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L773) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachIPv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L695) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachIPv6NextHopDualValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L642) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachIPv6NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L617) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachVPNv4NextHopValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L747) | unit/verify | unproven |

### [`RFC7606-7.13-1`](#rfc7606-7.13-1)

Traffic Engineering path attribute if malformed: treat-as-withdraw (§7.13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IMPLEMENTED 2026-07-20. validateTrafficEngineeringAttr (message/rfc7606_optional_attrs.go) registers code 24 and rejects anything shorter than one RFC 5543 descriptor (36 octets: SwitchingCap+Encoding+Reserved+8x4 bandwidth). Negative TestRFC7606TrafficEngineeringTooShort pins lengths 0/1/35 to TreatAsWithdraw with AttrCode 24; positive TestRFC7606TrafficEngineeringValid pins 36/44/72 to Action None, so a validator that rejected everything would fail. Both drive ValidateUpdateRFC7606, not the validator directly. The check is deliberately minimal because 7.13 states RFC 5543 'does not detail what constitutes malformation'; over-validating would blackhole valid routes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606TrafficEngineeringTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L56) | unit/verify | unproven |
| positive | [`TestRFC7606TrafficEngineeringValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L83) | unit/verify | unproven |

### [`RFC7606-7.14-1`](#rfc7606-7.14-1)

Extended Community: malformed if length is zero or not a multiple of 8; treat-as-withdraw (§7.14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive rfc7606_test.go:187 (len-8 Extended Community) asserts ActionNone; negative rfc7606_test.go:218 (len-0) asserts exact TreatAsWithdraw + AttrCode 16 + "7.14", isolated as the last attribute with no trailing bytes (comment 212-215). Note: only the zero-length ('non-zero') clause is tagged; the 'not a multiple of 8' clause for nonzero lengths (e.g. 12) is untagged, so dropping the `length%8!=0` check while keeping `==0` in validateExtCommunityAttr (rfc7606.go:551) would pass the tagged tests. The code implements both clauses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606ExtendedCommunityZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L331) | unit/verify | unproven |
| positive | [`TestRFC7606ExtendedCommunityValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L300) | unit/verify | unproven |

### [`RFC7606-7.14-2`](#rfc7606-7.14-2)

Unrecognized Extended Community Type or Sub-Type MUST NOT be treated as error (§7.14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity (positive-only) argument holds: §7.14 is a MUST NOT prohibition ('MUST NOT treat an unrecognized Extended Community Type or Sub-Type as an error'), so acceptance is the only conforming observation. Positive rfc7606_test.go:247 uses valid length 8 with unrecognized Type 0x3f/Sub-Type 0xee and asserts ActionNone; length kept valid so the 7.14-1 length clause cannot mask the type question. validateExtCommunityAttr (rfc7606.go:550-560) inspects only length, never type, so introducing a type allowlist would flip this test to TreatAsWithdraw and fail it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7606ExtendedCommunityUnrecognizedType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L360) | unit/verify | unproven |

### [`RFC7606-7.15-1`](#rfc7606-7.15-1)

IPv6 Extended Community: malformed if length is zero or not a multiple of 20; treat-as-withdraw (§7.15)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IMPLEMENTED 2026-07-20. validateIPv6ExtCommunityAttr registers code 25 and applies 7.15's exact rule: malformed unless the length is a non-zero multiple of 20. Negative TestRFC7606IPv6ExtCommunityBadLength pins 0/19/21/30; positive TestRFC7606IPv6ExtCommunityValidLength pins 20/40. Zero is covered explicitly because 0 % 20 == 0 would pass a multiple-of-20 test alone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606IPv6ExtCommunityBadLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L115) | unit/verify | unproven |
| positive | [`TestRFC7606IPv6ExtCommunityValidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L142) | unit/verify | unproven |

### [`RFC7606-7.15-2`](#rfc7606-7.15-2)

Unrecognized IPv6 Extended Community Type or Sub-Type MUST NOT be treated as error (§7.15)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IMPLEMENTED 2026-07-20, and the basis CHANGED. Previously met by omission (nothing validated code 25, so nothing could error). Now met by design: validateIPv6ExtCommunityAttr takes the attribute value as `_` and tests length alone, so no Type or Sub-Type can reach a rejection. TestRFC7606IPv6ExtCommunityUnrecognizedType pins Type 0x3f / Sub-Type 0xee with a valid length 20 to Action None -- the length clause cannot fire and mask the type question. Positive-only by the same argument as 7.14-2: the rule is a prohibition, so a negative would assert the rejection the RFC forbids.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7606IPv6ExtCommunityUnrecognizedType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L172) | unit/verify | unproven |

### [`RFC7606-7.16-1`](#rfc7606-7.16-1)

ATTR_SET if malformed: treat-as-withdraw (§7.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IMPLEMENTED 2026-07-20. validateAttrSetAttr registers code 128 and applies all three RFC 6368 Section 5 malformed conditions: length < 4, contained MP_REACH/MP_UNREACH, and inner attributes malformed themselves (recursing through validateAttribute, so the definition is shared not duplicated). Negative TestRFC7606AttrSetMalformed covers all three plus truncated inner header/length/value and over-deep nesting; positive TestRFC7606AttrSetValid covers Origin-AS-only, valid inner attributes, and one legal nesting level. Recursion is depth-capped at 4 because a peer controls nesting and could otherwise exhaust the stack. 2026-07-20 (later): the redundant 'nested beyond the depth cap' case was removed from TestRFC7606AttrSetMalformed under owner approval; the cap is covered more strongly by TestRFC7606AttrSetNestingCapBoundary, which pins the deepest ACCEPTED nesting as well as the first rejected one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AttrSetInnerMalformedStillWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L83) | unit/verify | unproven |
| negative | [`TestRFC7606AttrSetNestingCapBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L102) | unit/verify | unproven |
| negative | [`TestRFC7606AttrSetMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC7606AttrSetInnerASPathAlwaysFourOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L31) | unit/verify | unproven |
| positive | [`TestRFC7606AttrSetInnerIBGPAttributesOnEBGPSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_context_test.go#L55) | unit/verify | unproven |
| positive | [`TestRFC7606AttrSetInnerDiscardDoesNotWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_attrset_discard_test.go#L25) | unit/verify | unproven |
| positive | [`TestRFC7606AttrSetValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_optional_attrs_test.go#L252) | unit/verify | unproven |

### [`RFC7606-4-2`](#rfc7606-4-2)

For all path attributes other than those specified as having an attribute length that may be zero, a zero attribute length SHALL be a syntax error handled as a malformed attribute. Of the attributes considered in RFC 7606, only AS_PATH and ATOMIC_AGGREGATE may validly have zero length; the RFC leaves this open for future attributes (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative rfc7606_structural_test.go:281 asserts exact TreatAsWithdraw + AttrCode for zero-length COMMUNITY(8)/MED(4)/LARGE_COMMUNITY(32). Positives cover the carve-out: rfc7606_test.go:111 accepts empty AS_PATH (len 0) to ActionNone and Baseline_EBGP:1467/1495 accepts ATOMIC_AGGREGATE len 0. Both the 'zero is a syntax error' clause and the 'only AS_PATH/ATOMIC_AGGREGATE may be zero' carve-out are pinned.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606ZeroLengthAttributeMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L273) | unit/verify | unproven |
| positive | [`TestRFC7606SystematicLengthCorruption`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L1621) | unit/verify | unproven |
| positive | [`TestRFC7606ValidUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L185) | unit/verify | unproven |

### [`RFC7606-3.g-2`](#rfc7606-3.g-2)

Duplicate non-MP attributes: discard all but first occurrence (§3.g)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The pair distinguishes first-wins from last-wins: TestRFC7606DuplicateAttributeFirstOccurrenceWins (rfc7606_structural_test.go:550) = valid ORIGIN + malformed duplicate => require.Equal(None) (last-wins would error); TestRFC7606DuplicateAttributeFirstOccurrenceIsValidated (:571) = malformed ORIGIN(value 3) first + valid duplicate => TreatAsWithdraw AttrCode 1 Description 'undefined value' (last-wins would return None). A last-wins implementation flips both, so both fail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DuplicateAttributeFirstOccurrenceIsValidated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L563) | unit/verify | unproven |
| positive | [`TestRFC7606DuplicateAttributeFirstOccurrenceWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L542) | unit/verify | unproven |

### [`RFC7606-8-1`](#rfc7606-8-1)

A new BGP attribute specification MUST define what constitutes malformation and how to handle it (§8)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. {not-applicable} annotation holds up: RFC §8 ('A document that specifies a new BGP attribute MUST provide specifics regarding what constitutes an error...') binds AUTHORS of future BGP attribute specifications, not a running implementation. There is genuinely no Ze code path that could satisfy or violate it, so it is not implementable (per the method, an implementable N/A would be 'wrong' -- this one is not). Recorded 'enforced' only in the sense that the N/A classification is honest and correct; there is no test to demand and no finding. 2026-07-29 (plan/spec-rfcgate-3-audit-teeth.md, owner ruling OR-1): the value was `enforced` with an empty `tests` map, which the schema now refuses -- `enforced` means the tests would fail if the code stopped complying, and this note itself says there is no test to demand. That was the honest reading of a vocabulary with no honest state for it, so Thomas added one rather than have the record re-judged: `not-applicable`, which requires an empty `tests` map, a mandatory `no_code_path` reason, and an independently committed {not-applicable} annotation on the checklist line. No judgement changed here; the same finding is now recorded in a word that means it. UNRESOLVED and deliberately left so: ai/rules/rfc-compliance.md voids {not-applicable} as AUTHORITY, so the annotation this verdict agrees with is itself a classification the owner has voided. This ruling makes the VERDICT honest about what the code does; it does not re-affirm the annotation, and re-deriving that is fleet-drain work under the rfcgate umbrella's D4. RFC7606-8-1 needs looking at again when the drain reaches it.

No test carries RFC7606-8-1, so no unit is bound to it.

### [`RFC7606-6-1`](#rfc7606-6-1)

Implementation must provide debugging facilities, at minimum logging an error listing the NLRI involved and containing the entire malformed UPDATE. NOTE: §6 uses lowercase "must", outside the RFC 2119 keyword set scoped by §1.1; kept at MUST level as a Ze policy choice, not an RFC 2119 obligation (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IMPLEMENTED 2026-07-20. Session.rfc7606Diagnostics (reactor/session_validation.go) logs the NLRI involved (withdrawn and announced IPv4 prefixes, decoded ADD-PATH-aware) and the entire UPDATE body as hex, wired into all three RFC 7606 outcomes: attribute-discard, treat-as-withdraw (logged BEFORE SynthesizeWithdraw rewrites the body, so the dump is what the peer sent) and session-reset. TestRFC7606DiagnosticsListNLRIAndUpdate asserts the prefix and the raw ORIGIN bytes appear; TestRFC7606DiagnosticsCoverSessionReset covers the reset path; TestIPv4PrefixListTolerance pins the decoder on malformed input. The negative is TestRFC7606DiagnosticsCostsNothingWhenDisabled, which asserts zero allocations when the level is off -- that is what pins the Enabled() guard, since slog evaluates arguments eagerly and without the guard a peer could make ze-build hex-encode every malformed UPDATE it sends. TestRFC7606DiagnosticsSilentWhenDebugDisabled sits alongside it but only shows no line is emitted; a level-filtering handler drops the line with or without the guard, so it does NOT pin it. Note the requirement's 'must' is lowercase in the RFC and so outside RFC 2119; kept at MUST level as a ze policy choice.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606DiagnosticsCostsNothingWhenDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diag_cost_test.go#L26) | unit/verify | unproven |
| negative | [`TestRFC7606DiagnosticsSilentWhenDebugDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC7606DiagnosticsCoverSessionReset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC7606DiagnosticsListNLRIAndUpdate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_diagnostics_test.go#L64) | unit/verify | unproven |

### [`RFC7606-3.a-1`](#rfc7606-3.a-1)

An error detected while processing an UPDATE for which a session reset is specified MUST be indicated by sending a NOTIFICATION with Error Code "UPDATE Message Error"; the subcode elaborates on the specific nature (§3.a)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-audited 2026-08-27 after the fixture observer moved to compiled Go. test/plugin/rfc7606-reset.ci still drives a real session and requires the exact NOTIFICATION frame ...0015030301 (Error Code 3 UPDATE Message Error, subcode 1). Its native passivePlugin13 observer only keeps the required plugin process alive and cannot make the wire expectation pass. TestSessionRFC7606ValidUpdateSendsNoNotification uses a net.Pipe read deadline to prove a valid UPDATE sends no NOTIFICATION and stays Established. TestEnforceRFC7606_ShortBody reinforces the exact SessionReset classification. Under-reaction, a wrong Error Code, and notifying a conforming UPDATE each fail a distinct assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnforceRFC7606_ShortBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L148) | unit/verify | unproven |
| negative | [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L7) | functional/verify | unproven |
| positive | [`TestSessionRFC7606ValidUpdateSendsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7606_session_structural_test.go#L219) | unit/verify | unproven |

### [`RFC7606-2-5`](#rfc7606-2-5)

When treat-as-withdraw is used, the affected routes MUST be removed from the Adj-RIB-In per the procedures of RFC 4271 (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. now proven at the Adj-RIB-In itself, not by a dispatch-shape proxy shared with 2-1. TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute (adj_rib_in/rib_test.go): a valid announce leaves 10.0.0.0/8 in ribIn (Len 1, the positive); feeding the message.SynthesizeWithdraw output for a malformed re-announce of the same prefix removes it (Len 0, the negative), observed directly on r.ribIn via removeStructuredNLRIs. The proxy 2-5 tags on the reactor session tests were removed; the reactor still proves 2-1 (dispatched as a withdrawal). 2026-07-22 re-stamp: the verdict went stale only because commit c4038def0 prepended an rfc-test-change-approved header and swapped package qualifiers (bgptypes.RouteAction*->routeaction.*, message.Type*->msgtype.Type*, rib/events->core/bgp/ribevents) in the tagged files; tagged_unit_shas fingerprints the whole enclosing file. Normalising the 254772452..HEAD diff under that rename cancels with ZERO unmatched assertion deletions, and rfc/short/rfc7606.md is byte-identical since the audit, so the judgement above still describes exactly what it judged. 2026-07-25 re-stamp: stale again for the same structural reason -- spec-fixit-bgp-egress-rail-divergence edited OTHER tests in rib_test.go (replay now carries the source peer; the update-hex command form is gone), and tagged_unit_shas fingerprints the whole enclosing file. TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute is byte-identical, and so is every line of the path it exercises: git diff on rib.go shows handleReceivedStructured, installStructuredNLRIs and removeStructuredNLRIs unmodified (the only diff match is a context line). rfc/short/rfc7606.md is byte-identical, so requirement_sha is unchanged. Re-judged against rfc/full/rfc7606.txt:243-254 rather than the summary: the obligation is that contained routes be removed from the Adj-RIB-In, and the test asserts ribIn Len 1->0 -- RIB state, not a dispatch-shape proxy -- with exact equality on both halves, and the two buffers differ only in the ORIGIN length octet with identical NLRI, so the removal is measured against the key that was installed. Narrowness noted and unchanged: it covers the IPv4-unicast body-NLRI encoding, not the MP_UNREACH_NLRI parenthetical, which is a separate branch. Verdict stands. 2026-07-25 re-stamp (2): stale again for the SAME structural reason -- tagged_unit_shas fingerprints the whole enclosing FILE, and this change added three subtests to TestReplayOwnerDedupe in rib_test.go for the declarative stage-2 replay claim. The sole diff hunk is @@ -958,6 +958,58 @@ inside TestReplayOwnerDedupe; TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute (line 269) is byte-identical and did not even shift line number, and requirement_sha is unchanged (verified: stored == recomputed). The judgement above still describes exactly what it judged. 2026-07-26 re-stamp: stale again, and again only from the file-level fingerprint. requirement_sha is byte-identical (a8534d7b2f2b4ae6) and git shows ZERO changed lines in TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute or its two tags; b8f64e345 added a maxMsgID argument to buildReplayRoutes call sites elsewhere in rib_test.go, and tagged_unit_shas hashes the whole enclosing file. Re-read both halves before re-stamping: the positive feeds a valid announce and asserts ribIn[peer].Len()==1, the negative feeds message.SynthesizeWithdraw of a malformed-ORIGIN re-announce and asserts Len()==0, so an implementation that stopped removing on treat-as-withdraw leaves Len()==1 and fails the negative. Verdict unchanged: enforced. This is the SECOND such false-stale for this requirement (see the 2026-07-22 entry above); both cost a re-audit of an untouched test, which is the documented over-trigger bias working as designed. 2026-07-26 re-stamp (3): stale again for the SAME structural reason -- tagged_unit_shas fingerprints the whole enclosing FILE, and the peer-up replay-cut fix (adj-rib-in: max-msg-id bounded by PRESENCE, not by the value 0) changed six buildReplayRoutes CALL SITES in rib_test.go at lines 351, 397, 428, 433, 442, 462 and 584 -- every one of them a `0` -> `unboundedReplay()` argument-form change, with no assertion added, removed, reworded or weakened anywhere in the file. TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute (rib_test.go:269-317) is byte-identical: every diff hunk is at line 351 or below. So is the path it exercises -- the rib.go diff touches handleStructuredState, handleState and buildReplayRoutes only, none of which that test calls; handleReceivedStructured, installStructuredNLRIs and removeStructuredNLRIs are unmodified. rfc/short/rfc7606.md is untouched and requirement_sha is unchanged (a8534d7b2f2b4ae6), and both tag lines are still 267/268. Verdict stands, with the same narrowness noted before: IPv4-unicast body NLRI, not the MP_UNREACH_NLRI parenthetical.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_test.go#L259) | unit/verify | unproven |
| positive | [`TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/rib_test.go#L258) | unit/verify | unproven |

### [`RFC7606-5.3-1`](#rfc7606-5.3-1)

The NLRI or Withdrawn Routes field SHALL be considered syntactically incorrect if the length of any included NLRI is greater than 32 (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Negative session_validate_test.go:107 (TestEnforceRFC7606_InvalidTrailingNLRI) runs the production path enforceRFC7606 -> ValidateNLRISyntax(nlri,false) on a trailing /33; asserts require.Error + require.Equal(SessionReset). Valid ORIGIN/AS_PATH/NEXT_HOP filler isolates it, and removing the >32 check (rfc7606.go:857) would let the /33 parse cleanly (5 bytes fit) -> ActionNone -> test fails. Positive structural_test.go:55 accepts /32; direct unit negative rfc7606_test.go:736 reinforces.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606NLRIPrefixLengthTooLongIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L842) | unit/verify | unproven |
| negative | [`TestEnforceRFC7606_InvalidTrailingNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L196) | unit/verify | unproven |
| positive | [`TestRFC7606NLRIMaxPrefixLengthAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L54) | unit/verify | unproven |

### [`RFC7606-5.3-2`](#rfc7606-5.3-2)

The NLRI or Withdrawn Routes field SHALL be considered syntactically incorrect if, when parsing NLRI, the length of the last NLRI found exceeds the unconsumed data remaining in the field (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative (session_validate_test.go TestEnforceRFC7606_InvalidWithdrawnNLRI) now uses a /24 Withdrawn NLRI with only 2 octets present (24<=32, so the >32 rule 5.3-1 cannot fire); enforceRFC7606 -> ValidateNLRISyntax on the Withdrawn field (session_validation.go:54) returns SessionReset via the overrun clause. Positive is the exact-fit /24. Isolated from 5.3-1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnforceRFC7606_InvalidWithdrawnNLRI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_validate_test.go#L170) | unit/verify | unproven |
| positive | [`TestRFC7606NLRILastPrefixExactlyFitsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L75) | unit/verify | unproven |

### [`RFC7606-5.3-3`](#rfc7606-5.3-3)

MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the length of any included NLRI is inconsistent with the given AFI/SAFI (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. MP NLRI length-vs-AFI/SAFI is enforced by validateMPNLRIField in the ValidateUpdateRFC7606 main loop (ADD-PATH aware: skips the RFC 7911 4-byte path id) -> ValidateNLRISyntaxAddPath. Negative (TestRFC7606NLRIPrefixLengthTooLongIPv6) drives ValidateUpdateRFC7606 with an IPv6 MP_REACH NLRI prefixLen 129>128 (17 octets follow, so a length not overrun violation); asserts '129 > 128'. Positive is a well-formed IPv6 MP_REACH. Isolated.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606NLRIPrefixLengthTooLongIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L895) | unit/verify | unproven |
| positive | [`TestRFC7606MPNLRILengthConsistentWithAFIAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L285) | unit/verify | unproven |

### [`RFC7606-5.3-4`](#rfc7606-5.3-4)

MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if, when parsing NLRI in the attribute, the length of the last NLRI found exceeds the unconsumed data remaining in the attribute (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. MP last-NLRI overrun is enforced by validateMPNLRIField in the main loop (ADD-PATH aware). Negative (TestRFC7606MPReachNLRIOverrunsAttribute) uses a valid 16-octet next hop so 7.11 passes and the only defect is a /128 NLRI with 2 octets; asserts 'overrun'. Positive fits exactly. Isolated from the 7.11 NHLen=0 path the old test passed through. TestRFC7606MPReachAddPathValidNotReset guards that a valid ADD-PATH UPDATE is not misread.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MPReachNLRIOverrunsAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L225) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L286) | unit/verify | unproven |

### [`RFC7606-5.3-5`](#rfc7606-5.3-5)

MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the attribute flags are inconsistent with those specified in RFC 4760. NOTE: this routes via §3.j to session reset / AFI-SAFI disable, a STRONGER action than the generic flag-conflict treat-as-withdraw of R005 (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. validateAttributeFlags now checks MP_REACH/MP_UNREACH flags per RFC 4760 (optional, non-transitive) and returns SessionReset. Negative (TestRFC7606MPReachFlagsInconsistentWithRFC4760) uses a valid next hop and NLRI with flags 0x40; flipping to 0x80 yields None, proving isolation; asserts 'RFC 4760'.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MPReachFlagsInconsistentWithRFC4760`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L257) | unit/verify | unproven |
| positive | [`TestRFC7606MPReachWellFormedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_withdraw_test.go#L287) | unit/verify | unproven |

### [`RFC7606-5.3-6`](#rfc7606-5.3-6)

MP_REACH_NLRI/MP_UNREACH_NLRI SHALL be considered incorrect if the MP_UNREACH_NLRI length is less than 3, or the MP_REACH_NLRI length is less than 5 (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-audited and strengthened 2026-08-27. TestRFC7606MPReachLengthFourIsIncorrect is the isolated negative at the exact boundary: AFI, SAFI and zero NextHopLen occupy four octets for FlowSpec, and it requires SessionReset, AttrCode 14, and description 'length 4 < 5'. TestRFC7606MPMinimumLengthsAccepted pins MP_REACH length 5 and MP_UNREACH length 3 to ActionNone, so both lower boundaries and off-by-one over-reaction are covered. test/plugin/rfc7606-reset.ci supplies end-to-end reinforcement with a shorter MP_REACH and exact code-3/subcode-1 NOTIFICATION; its compiled passive observer cannot satisfy the wire check. This closes the prior audit's admitted hole where a len=2 negative could pass through a neighbouring framing error while length 4 was wrongly accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606MPReachLengthFourIsIncorrect`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_minimum_length_test.go#L9) | unit/verify | unproven |
| negative | [`rfc7606-reset.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/rfc7606-reset.ci#L6) | functional/verify | unproven |
| positive | [`TestRFC7606MPMinimumLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L157) | unit/verify | unproven |

### [`RFC7606-2-6`](#rfc7606-2-6)

When multiple errors dictate different approaches, the strongest action MUST be used; the strength ordering is session reset > AFI/SAFI disable > treat-as-withdraw > attribute discard, as listed in §2 (§2, §3.h)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7606StrongestActionWins (rfc7606_structural_test.go:324) has two require.Equal cases: ATOMIC_AGG(discard)+COMMUNITY(TAW) => TreatAsWithdraw AttrCode 8, and COMMUNITY(TAW)+MP_REACH IPv6 5-byte-NH => SessionReset AttrCode 14. Positive TestRFC7606EqualStrengthErrorsDoNotEscalate (:378) requires two discard errors to yield EXACTLY AttributeDiscard (both DiscardEntries present), which only require.Equal can enforce against escalation. Buffers isolate the intended errors with controlled ordering.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606StrongestActionWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L316) | unit/verify | unproven |
| positive | [`TestRFC7606EqualStrengthErrorsDoNotEscalate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_structural_test.go#L370) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 7606, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7606, so its obligations are stated where they were written.
