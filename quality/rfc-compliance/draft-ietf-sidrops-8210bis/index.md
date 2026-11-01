# DRAFT-IETF-SIDROPS-8210BIS - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2

No row in the public ledger. Every requirement this repository extracted from DRAFT-IETF-SIDROPS-8210BIS, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 11 | 11 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 8 | of 8 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every count below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 8 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1 | of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 8 gated MUSTs: they add up to 8. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | No row in the public ledger |
| Enrolment | Not enrolled (backlog) |
| Requirements | 8 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 1 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 11 |
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
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 1 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5), [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2), [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

**Annotated (including scoped evidence) (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

**No test and no annotation (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

## Figures

- **MUST bar**: not walked
- **Overview bar**: not walked

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 1 |
| Mandatory register | not complete |
| MUST rows | not walked |
| MUST rows kept (the MUST bar) | not walked |
| Ze proven | not walked |
| OS verified | not walked |
| OS testing gap | not walked |
| Ze correctness gap | not walked |
| MUST rows not owed | not walked |
| MUST not owed: feature-declined | not walked |
| MUST not owed: linux-limit | not walked |
| MUST not owed: not-applicable | not walked |
| SHOULD / MAY rows | not walked |
| SHOULD proven | not walked |
| SHOULD OS verified | not walked |
| SHOULD OS testing gap | not walked |
| SHOULD not owed | not walked |
| SHOULD not owed: feature-declined | not walked |
| SHOULD not owed: linux-limit | not walked |
| SHOULD not owed: not-applicable | not walked |
| SHOULD not ever | not walked |
| SHOULD not yet | not walked |
| SHOULD not decided | not walked |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/draft-ietf-sidrops-8210bis.json](../../../data/rfc/draft-ietf-sidrops-8210bis.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-1` | For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L570). **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L219). **negative:** `unit/verify` [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L597). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L241) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-3` | There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L640). **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L220). **negative:** `unit/verify` [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L656). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L272) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-4` | The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L73). **negative:** `unit/verify` [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L86) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-5` | If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L109). **negative:** `unit/verify` [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L124) |
| `DRAFT-IETF-SIDROPS-8210BIS-7-1` | Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31). **negative:** no negative test. **{single-polarity}:** ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively |
| `DRAFT-IETF-SIDROPS-8210BIS-7-2` | If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU with Error Code 4 ("Unsupported Protocol Version") unless the received PDU is itself an Error Report PDU. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222). **negative:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223). **negative:** `unit/verify` [`TestRTRUnknownVersionErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L205) |
| `DRAFT-IETF-SIDROPS-8210BIS-7-3` | If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-8210BIS-7-4` | Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L78). **negative:** `unit/verify` [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L95) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3) If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1)

For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L241) | unit/verify | unproven |
| negative | [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L597) | unit/verify | revert, verified |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L219) | unit/verify | unproven |
| positive | [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L570) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3)

There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L272) | unit/verify | unproven |
| negative | [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L656) | unit/verify | revert, verified |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L220) | unit/verify | unproven |
| positive | [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L640) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4)

The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L86) | unit/verify | mutant, verified |
| positive | [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L73) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5)

If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L124) | unit/verify | mutant, verified |
| positive | [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L109) | unit/verify | revert, verified |

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
| negative | [`TestRTRUnknownVersionErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L205) | unit/verify | revert, verified |
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) | unit/verify | unproven |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-3, so no unit is bound to it.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L95) | unit/verify | mutant, verified |
| positive | [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L78) | unit/verify | revert, verified |

## Extraction sign-off

No extraction sign-off exists for DRAFT-IETF-SIDROPS-8210BIS, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-8210BIS, so its obligations are stated where they were written.
