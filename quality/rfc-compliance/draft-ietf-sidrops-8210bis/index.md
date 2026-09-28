# DRAFT-IETF-SIDROPS-8210BIS - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2

No row in the public ledger. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-8210BIS, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 37.5% | 3 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.5% | 1 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 8 | of 8 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 50.0% | 4 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 4 |
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
| No test and no annotation | 4 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3), [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2)

**Annotated instead of tested (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

**No test and no annotation (4):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5), [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3), [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-1` | For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L230). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L252) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-3` | There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L231). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L283) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-4` | The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-5` | If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-7-1` | Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31). **negative:** no negative test. **{single-polarity}:** ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively |
| `DRAFT-IETF-SIDROPS-8210BIS-7-2` | If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU with Error Code 4 ("Unsupported Protocol Version") unless the received PDU is itself an Error Report PDU. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222). **negative:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) |
| `DRAFT-IETF-SIDROPS-8210BIS-7-3` | If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-7-4` | Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4) The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5) If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3) If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7) | no test | no test carries this requirement id |
| [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4) Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1)

For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L252) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L230) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3)

There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L283) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L231) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4)

The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-4, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5)

If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-5.12-5, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2)

If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU with Error Code 4 ("Unsupported Protocol Version") unless the received PDU is itself an Error Report PDU. (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) | unit/verify | unproven |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-3, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for DRAFT-IETF-SIDROPS-8210BIS, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-8210BIS, so its obligations are stated where they were written.
