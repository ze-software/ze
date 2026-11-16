# DRAFT-IETF-SIDROPS-8210BIS - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2

No entry in the public support list. Every requirement listed for DRAFT-IETF-SIDROPS-8210BIS, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6 | of 8 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 8 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 8 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 8 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 11 | 11 of 18 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 8 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 8 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 8 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 8 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 8 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 8 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (backlog), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 8 mandatory requirements: they add up to 8. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Excluded (backlog) |
| Requirements | 8 |
| Mandatory requirements (MUST level), not checked | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 1 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tests linked to requirements | 18 |
| Recorded audit verdicts | 0 |
| Fault test records | 11 |
| Summary | `rfc/short/draft-ietf-sidrops-8210bis.md` |
| Requirement table | `rfc/requirements/draft-ietf-sidrops-8210bis.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-8210bis.txt` |

## Automatic checks

Excluded from automatic checks (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): Existing RTR v2 ledger awaiting requirement reattribution from the historical RFC 9582 identifiers. The cached authority is revision 27; proof relocation and enrolment must preserve the source requirements and their tests.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for DRAFT-IETF-SIDROPS-8210BIS.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 1 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **8** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (6):** [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4), [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5), [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2), [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

**Annotated (including scoped evidence) (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

**No test and no annotation (1):** [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

## Figures

- **MUST bar**: requirements not reviewed
- **Overview bar**: requirements not reviewed

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | requirements not reviewed |
| MUST rows kept (the MUST bar) | requirements not reviewed |
| Ze proven | requirements not reviewed |
| OS verified | requirements not reviewed |
| Proof pending | requirements not reviewed |
| OS testing gap | requirements not reviewed |
| Exclusion unchecked | requirements not reviewed |
| Not assessed | requirements not reviewed |
| Not built | requirements not reviewed |
| MUST rows not owed | requirements not reviewed |
| MUST not owed: feature-declined | requirements not reviewed |
| MUST not owed: linux-limit | requirements not reviewed |
| MUST not owed: not-applicable | requirements not reviewed |
| SHOULD / MAY rows | requirements not reviewed |
| SHOULD proven | requirements not reviewed |
| SHOULD OS verified | requirements not reviewed |
| SHOULD OS testing gap | requirements not reviewed |
| SHOULD not owed | requirements not reviewed |
| SHOULD not owed: feature-declined | requirements not reviewed |
| SHOULD not owed: linux-limit | requirements not reviewed |
| SHOULD not owed: not-applicable | requirements not reviewed |
| SHOULD proof pending | requirements not reviewed |
| SHOULD exclusion unchecked | requirements not reviewed |
| SHOULD not assessed | requirements not reviewed |
| SHOULD not built | requirements not reviewed |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/draft-ietf-sidrops-8210bis.json](../../../data/rfc/draft-ietf-sidrops-8210bis.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-1` | For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L572). **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L219). **negative:** `unit/verify` [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L599). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L241) |
| `DRAFT-IETF-SIDROPS-8210BIS-5.12-3` | There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L642). **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L220). **negative:** `unit/verify` [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L658). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L272) |
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

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-1`](#draft-ietf-sidrops-8210bis-5.12-1)

For an announcement, the PDU MUST contain at least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code 9 ("ASPA Provider List Error") MUST be returned by the router. (§5.12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L241) | unit/verify | unproven |
| negative | [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L599) | unit/verify | revert, verified |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L219) | unit/verify | unproven |
| positive | [`TestASPAEmptyAnnouncementDrawsErrorReport9`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L572) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-3`](#draft-ietf-sidrops-8210bis-5.12-3)

There are zero or more 32-bit Provider Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System Number in a given ASPA PDU MUST be unique. (§5.12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L272) | unit/verify | unproven |
| negative | [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L658) | unit/verify | revert, verified |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L220) | unit/verify | unproven |
| positive | [`TestASPAProviderListOfZeroOrMoreUniqueAscending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L642) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-4`](#draft-ietf-sidrops-8210bis-5.12-4)

The router MUST see at most one ASPA from a particular cache for a particular Customer Autonomous System Number active at any time. As a number of conditions in the global RPKI may present multiple valid ASPA RPKI records for a single customer to a particular RP cache, this places a burden on the cache to form the union of multiple ASPA records it has received from the global RPKI into one ASPA PDU. Receipt of an ASPA PDU announcement with the announce/withdraw flag set to 1 when the router already has an ASPA PDU with the same Customer Autonomous System Number from that cache replaces the previous one. The cache MUST deliver the complete data of the ASPA record(s) of a CAS in a single ASPA PDU. (§5.12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L86) | unit/verify | mutant, verified |
| positive | [`TestASPAAnnouncementReplacesTheSameCustomer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L73) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-5.12-5`](#draft-ietf-sidrops-8210bis-5.12-5)

If the announce/withdraw flag is set to 0 in an ASPA PDU, the customer AS of the ASPA record MUST be provided, there MUST be no Provider list, and the PDU Length MUST be 12. A router receiving this type of ASPA PDU (i.e., a withdrawal) MUST remove the entire ASPA record from that cache for that Customer AS. (§5.12)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L124) | unit/verify | mutant, verified |
| positive | [`TestASPAWithdrawalRemovesTheWholeRecord`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_aspa_record_test.go#L109) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-1`](#draft-ietf-sidrops-8210bis-7-1)

Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L31) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-2`](#draft-ietf-sidrops-8210bis-7-2)

If either party receives a PDU containing an unrecognized Protocol Version (neither 0, 1, nor 2) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU with Error Code 4 ("Unsupported Protocol Version") unless the received PDU is itself an Error Report PDU. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRUnknownVersionErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L205) | unit/verify | revert, verified |
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L223) | unit/verify | unproven |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L222) | unit/verify | unproven |

### [`DRAFT-IETF-SIDROPS-8210BIS-7-3`](#draft-ietf-sidrops-8210bis-7-3)

If a cache which supports version C receives a query with Protocol Version Q < C, and the cache does not support versions <= Q, the cache MUST send an Error Report PDU (Section 5.8) with Protocol Version C and Error Code 4 ("Unsupported Protocol Version") and disconnect the transport, as negotiation is hopeless. If a cache which supports version C receives a query with Protocol Version Q < C, and the cache can support version Q, the cache MUST establish the session at protocol version Q, [RFC6810] or [RFC8210], and respond with a Cache Response (Section 5.5) of that Protocol Version, Q. The RPKI-Router session is then considered open. If the cache which supports C as its highest version receives a query of version Q > C, the cache MUST send an Error Report PDU with Protocol Version C and Error Code 4. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries DRAFT-IETF-SIDROPS-8210BIS-7-3, so there is no linked test.

### [`DRAFT-IETF-SIDROPS-8210BIS-7-4`](#draft-ietf-sidrops-8210bis-7-4)

Once a router has established a transport connection to a cache, it MUST attempt to open an RPKI-Router 'session' by issuing either a Reset Query Section 5.4) or a Serial Query (Section 5.3) with the highest version of this protocol the router implements in the Protocol Version field. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L95) | unit/verify | mutant, verified |
| positive | [`TestRTRSessionOpensAtTheHighestVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_8210bis_version_open_test.go#L78) | unit/verify | revert, verified |

## Requirement list review

No requirement list review is recorded for DRAFT-IETF-SIDROPS-8210BIS, so there is no recorded check of its requirements against the original RFC, sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-8210BIS, so its obligations are stated where they were written.
