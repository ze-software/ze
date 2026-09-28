# DRAFT-IETF-SIDROPS-8210BIS - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2

No row in the public ledger. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-8210BIS, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 27.3% | 3 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9.1% | 1 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 11 | of 13 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 63.6% | 7 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (backlog) |
| Requirements | 13 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 7 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tagged units | 7 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-ietf-sidrops-8210bis.md` |
| Requirement shard | `rfc/requirements/draft-ietf-sidrops-8210bis.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-8210bis.txt` |

## Enrolment

Not enrolled (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): Existing RTR v2 ledger awaiting requirement reattribution from the historical RFC 9582 identifiers. The cached authority is revision 27; proof relocation and enrolment must preserve the source requirements and their tests.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for DRAFT-IETF-SIDROPS-8210BIS.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 7 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3), [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2)

**Annotated instead of tested (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

**No test and no annotation (7):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5), [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3), [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-2`](#draft-ietf-sidrops-8210bis-5.12-2), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-6`](#draft-ietf-sidrops-8210bis-5.12-6), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-7`](#draft-ietf-sidrops-8210bis-5.12-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-1` | An ASPA announcement MUST contain at least one Provider Autonomous System Number; otherwise the router MUST return Error Report 9 (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L230). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L252) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-3` | Each Provider Autonomous System Number in a given ASPA PDU MUST be unique; the provider fields are in increasing numeric order (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L231). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L283) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-4` | The router MUST see at most one active ASPA from a particular cache for a particular Customer Autonomous System Number; the cache MUST deliver the complete data for that customer in a single PDU (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-5` | An ASPA withdrawal MUST provide the Customer AS, contain no provider list, and have PDU Length 12; the router MUST remove that customer's entire ASPA record from that cache (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-7-1` | Router starting a v2 session MUST send query with version=2 (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31). **negative:** no negative test. **{single-polarity}:** ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively |
| `DRAFT-IETF-SIDROPS-8210BIS-7-2` | If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during negotiation, it MUST either downgrade to a known version or terminate the connection, with Error Report 4 unless the received PDU is itself an Error Report (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222). **negative:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) |
| `DRAFT-IETF-SIDROPS-8210BIS-7-3` | A cache receiving an unsupported query version MUST send Error Report 4 with its supported version C; when Q < C and the cache supports no version <= Q, it MUST also disconnect the transport (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-7-4` | The router MUST initiate the session with a Reset Query or Serial Query carrying the highest protocol version it implements (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-2` | Customer AS MUST NOT appear in its own provider set (§5.12) | MUST NOT | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-6` | Router MUST ignore ASPA PDUs with unknown AFI values (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-7` | Customer AS 0 is reserved, MUST NOT appear (§5.12) | MUST NOT | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-8` | Provider ASNs SHOULD be sorted ascending; cache MUST sort, router SHOULD verify (§5.12) | SHOULD | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-9` | Router MAY ignore AFI field and apply ASPA to all address families (§5.12) | MAY | 5.12 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4) The router MUST see at most one active ASPA from a particular cache for a particular Customer Autonomous System Number; the cache MUST deliver the complete data for that customer in a single PDU (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5) An ASPA withdrawal MUST provide the Customer AS, contain no provider list, and have PDU Length 12; the router MUST remove that customer's entire ASPA record from that cache (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3) A cache receiving an unsupported query version MUST send Error Report 4 with its supported version C; when Q < C and the cache supports no version <= Q, it MUST also disconnect the transport (§7) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4) The router MUST initiate the session with a Reset Query or Serial Query carrying the highest protocol version it implements (§7) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-2`](#draft-ietf-sidrops-8210bis-5.12-2) Customer AS MUST NOT appear in its own provider set (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-6`](#draft-ietf-sidrops-8210bis-5.12-6) Router MUST ignore ASPA PDUs with unknown AFI values (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-7`](#draft-ietf-sidrops-8210bis-5.12-7) Customer AS 0 is reserved, MUST NOT appear (§5.12) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1)

An ASPA announcement MUST contain at least one Provider Autonomous System Number; otherwise the router MUST return Error Report 9 (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L252) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L230) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3)

Each Provider Autonomous System Number in a given ASPA PDU MUST be unique; the provider fields are in increasing numeric order (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L283) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L231) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4)

The router MUST see at most one active ASPA from a particular cache for a particular Customer Autonomous System Number; the cache MUST deliver the complete data for that customer in a single PDU (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-4, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5)

An ASPA withdrawal MUST provide the Customer AS, contain no provider list, and have PDU Length 12; the router MUST remove that customer's entire ASPA record from that cache (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-5, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

Router starting a v2 session MUST send query with version=2 (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2)

If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during negotiation, it MUST either downgrade to a known version or terminate the connection, with Error Report 4 unless the received PDU is itself an Error Report (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) | unit/verify | unproven |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

A cache receiving an unsupported query version MUST send Error Report 4 with its supported version C; when Q < C and the cache supports no version <= Q, it MUST also disconnect the transport (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-3, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

The router MUST initiate the session with a Reset Query or Serial Query carrying the highest protocol version it implements (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-4, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-2`](#draft-ietf-sidrops-8210bis-5.12-2)

Customer AS MUST NOT appear in its own provider set (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-2, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-6`](#draft-ietf-sidrops-8210bis-5.12-6)

Router MUST ignore ASPA PDUs with unknown AFI values (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-6, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-7`](#draft-ietf-sidrops-8210bis-5.12-7)

Customer AS 0 is reserved, MUST NOT appear (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-7, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for DRAFT-IETF-SIDROPS-8210BIS, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-8210BIS, so its obligations are stated where they were written.
