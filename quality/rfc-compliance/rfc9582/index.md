# RFC 9582 - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2

Unsupported. Every requirement this repository extracted from RFC 9582, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 85.7% | 6 of 7 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.3% | 1 of 7 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 7 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 7 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 10 |
| Obligations that bind Ze | 7 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9582.md` |
| Requirement shard | `rfc/requirements/rfc9582.md` |
| RFC text | `rfc/full/rfc9582.txt` |

## Enrolment

Enrolled: RPKI-to-Router Protocol v2 (ASPA PDU parsing + version negotiation): 6 MET + 1 single-polarity (7-1) + 3 not-applicable (cache-side roles ze does not play)

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered**

Nothing. RFC 9582 profiles the ROA signed object itself: a CMS envelope carrying a `RouteOriginAttestation`, an EE certificate bearing an RFC 3779 IP address delegation extension, and a certification path to a trust anchor. No ROA object exists anywhere in the ze process. `parsePrefixPDU` reads an already-validated payload off an RTR PDU at fixed byte offsets, so the decode, the extension reading and the path validation this RFC specifies all happen in the RPKI cache ze dials. <!-- source: [`internal/component/bgp/plugins/rpki/rtr_pdu.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu.go) -- parsePrefixPDU -->

**What the ledger says remains**

Every obligation, and implementing them means a relying-party validator: a CMS/DER decoder (RFC 5652 and RFC 6488), an RFC 3779 extension reader, X.509 path validation to a trust anchor, and an RRDP or rsync fetcher. That is what Routinator and rpki-client are, and ze delegates it deliberately. **This row read `Supported` until 2026-09-01, and the claim was never about this RFC.** [`rfc/short/rfc9582.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9582.md) still declares thirteen ids describing ASPA PDUs and RTR version negotiation, which are `draft-ietf-sidrops-8210bis`: RFC 9582 has no Section 5.12 and its Section 7 is IANA Considerations, so each id cites a section its own document does not have. The obligations are real and ze meets them; only the attribution is wrong. Correcting it is [`plan/spec-rfc-requirement-reattribution.md`](https://github.com/ze-software/ze/blob/main/plan/spec-rfc-requirement-reattribution.md), which the ledger's own ratchets refuse until they learn to let proof follow an obligation to another document.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC9582-5.12-1`](#rfc9582-5.12-1), [`RFC9582-5.12-2`](#rfc9582-5.12-2), [`RFC9582-5.12-3`](#rfc9582-5.12-3), [`RFC9582-5.12-6`](#rfc9582-5.12-6), [`RFC9582-5.12-7`](#rfc9582-5.12-7), [`RFC9582-7-2`](#rfc9582-7-2)

**Annotated instead of tested (4):** [`RFC9582-5.12-4`](#rfc9582-5.12-4), [`RFC9582-5.12-5`](#rfc9582-5.12-5), [`RFC9582-7-1`](#rfc9582-7-1), [`RFC9582-7-3`](#rfc9582-7-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9582-5.12-1` | Provider AS set in ASPA PDU MUST contain at least one provider ASN (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L223). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L273) |
| `RFC9582-5.12-2` | Customer AS MUST NOT appear in its own provider set (§5.12) | MUST NOT | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L224). **negative:** `unit/verify` [`TestParseASPAPDUSelfRef`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L319) |
| `RFC9582-5.12-3` | Provider ASNs MUST be in ascending order within the PDU (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L225). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L340) |
| `RFC9582-5.12-4` | Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a cache-side emission/deduplication guarantee; ze is the RTR router that only consumes ASPA PDUs (no ASPA PDU writer exists, only query writers) and never enforces or emits this pairing (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, :103) |
| `RFC9582-5.12-5` | Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** exact-(Customer-AS,AFI) withdraw matching binds the cache's emission and a per-AFI record model; ze consumes withdraws keyed on Customer-AS alone (the §5.12 router option to ignore AFI) and maintains no AFI dimension to match (internal/component/bgp/plugins/rpki/rtr_session.go:283, aspa_cache.go:114) |
| `RFC9582-5.12-6` | Router MUST ignore ASPA PDUs with unknown AFI values (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L226). **negative:** `unit/verify` [`TestParseASPAPDUUnknownAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L305) |
| `RFC9582-5.12-7` | Customer AS 0 is reserved, MUST NOT appear (§5.12) | MUST NOT | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L227). **negative:** `unit/verify` [`TestParseASPAPDUReservedCustomerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L362) |
| `RFC9582-7-1` | Router starting a v2 session MUST send query with version=2 (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L21). **negative:** no negative test. **{single-polarity}:** ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively |
| `RFC9582-7-2` | On Unsupported Protocol Version error, router MUST downgrade or disconnect (§7) | MUST | 7 | **positive:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L50). **negative:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L64) |
| `RFC9582-7-3` | Cache receiving a version it does not support MUST send error code 4 (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the RTR cache/server role; ze runs only an RTR client that dials out and reads error reports, with no listener and no error-report writer, so it never receives queries or sends error code 4 (internal/component/bgp/plugins/rpki/rtr_session.go:125) |
| `RFC9582-7-4` | Router SHOULD start at highest supported version (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5.12-8` | Provider ASNs SHOULD be sorted ascending; cache MUST sort, router SHOULD verify (§5.12) | SHOULD | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5.12-9` | Router MAY ignore AFI field and apply ASPA to all address families (§5.12) | MAY | 5.12 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9582-5.12-4`](#rfc9582-5.12-4) Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12) | no test | no test carries this requirement id; annotated {not-applicable}: this is a cache-side emission/deduplication guarantee; ze is the RTR router that only consumes ASPA PDUs (no ASPA PDU writer exists, only query writers) and never enforces or emits this pairing (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, :103) |
| [`RFC9582-5.12-5`](#rfc9582-5.12-5) Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12) | no test | no test carries this requirement id; annotated {not-applicable}: exact-(Customer-AS,AFI) withdraw matching binds the cache's emission and a per-AFI record model; ze consumes withdraws keyed on Customer-AS alone (the §5.12 router option to ignore AFI) and maintains no AFI dimension to match (internal/component/bgp/plugins/rpki/rtr_session.go:283, aspa_cache.go:114) |
| [`RFC9582-7-3`](#rfc9582-7-3) Cache receiving a version it does not support MUST send error code 4 (§7) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the RTR cache/server role; ze runs only an RTR client that dials out and reads error reports, with no listener and no error-report writer, so it never receives queries or sends error code 4 (internal/component/bgp/plugins/rpki/rtr_session.go:125) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9582-5.12-1`](#rfc9582-5.12-1)

Provider AS set in ASPA PDU MUST contain at least one provider ASN (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L273) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L223) | unit/verify | unproven |

### [`RFC9582-5.12-2`](#rfc9582-5.12-2)

Customer AS MUST NOT appear in its own provider set (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUSelfRef`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L319) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L224) | unit/verify | unproven |

### [`RFC9582-5.12-3`](#rfc9582-5.12-3)

Provider ASNs MUST be in ascending order within the PDU (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L340) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L225) | unit/verify | unproven |

### [`RFC9582-5.12-4`](#rfc9582-5.12-4)

Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5.12-4, so no unit is bound to it.

### [`RFC9582-5.12-5`](#rfc9582-5.12-5)

Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5.12-5, so no unit is bound to it.

### [`RFC9582-5.12-6`](#rfc9582-5.12-6)

Router MUST ignore ASPA PDUs with unknown AFI values (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnknownAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L305) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L226) | unit/verify | unproven |

### [`RFC9582-5.12-7`](#rfc9582-5.12-7)

Customer AS 0 is reserved, MUST NOT appear (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUReservedCustomerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L362) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L227) | unit/verify | unproven |

### [`RFC9582-7-1`](#rfc9582-7-1)

Router starting a v2 session MUST send query with version=2 (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L21) | unit/verify | unproven |

### [`RFC9582-7-2`](#rfc9582-7-2)

On Unsupported Protocol Version error, router MUST downgrade or disconnect (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L64) | unit/verify | unproven |
| positive | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L50) | unit/verify | unproven |

### [`RFC9582-7-3`](#rfc9582-7-3)

Cache receiving a version it does not support MUST send error code 4 (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-7-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9582, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9582, so its obligations are stated where they were written.
