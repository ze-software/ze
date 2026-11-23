# RFC 8210 - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1

RPKI RTR v1. Every requirement listed for RFC 8210, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 30 | of 59 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 5 | of 59 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 59 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 59 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 88 | 88 of 103 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 20 | of 59 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 35 | of 59 mandatory requirements reviewed | 2 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 59 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 4 | of 59 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 59 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 59 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 59 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 59 | of 86 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 59 mandatory requirements: they add up to 59. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | RPKI RTR v1 |
| Automatic checks | Included |
| Requirements | 86 |
| Mandatory requirements (MUST level) | 59 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 22 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 108 |
| Tests linked to requirements | 103 |
| Recorded audit verdicts | 35 |
| Fault test records | 88 |
| Summary | `rfc/short/rfc8210.md` |
| Requirement table | `rfc/requirements/rfc8210.md` |
| RFC text | `rfc/full/rfc8210.txt` |

## Automatic checks

Included in automatic checks: RPKI to Router Protocol, Version 1

## Published support status

**Area:** RPKI RTR v1

**Test verification:** 35 of 59 MUST proven

**Evidence:** unit 35

**Unmet:** RFC8210-2-1, RFC8210-4-2, RFC8210-4-3, RFC8210-5.10-1, RFC8210-5.11-3, RFC8210-5.3-1, RFC8210-5.3-2, RFC8210-5.5-1, RFC8210-5.6-1, RFC8210-5.8-1, RFC8210-7-2, RFC8210-9.1-2, RFC8210-9.1-5, RFC8210-9.2-12, RFC8210-9.2-3, RFC8210-9.2-6, RFC8210-9.2-9, RFC8210-9.3-1, RFC8210-9.3-2, RFC8210-9.4-1, RFC8210-9.4-2, RFC8210-9.4-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 30 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 29 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **59** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (30):** [`RFC8210-5-1`](#rfc8210-5-1), [`RFC8210-5.1-1`](#rfc8210-5.1-1), [`RFC8210-5.1-2`](#rfc8210-5.1-2), [`RFC8210-5.1-3`](#rfc8210-5.1-3), [`RFC8210-5.1-4`](#rfc8210-5.1-4), [`RFC8210-5.1-5`](#rfc8210-5.1-5), [`RFC8210-5.10-2`](#rfc8210-5.10-2), [`RFC8210-5.11-1`](#rfc8210-5.11-1), [`RFC8210-5.11-2`](#rfc8210-5.11-2), [`RFC8210-6-2`](#rfc8210-6-2), [`RFC8210-7-3`](#rfc8210-7-3), [`RFC8210-5.2-1`](#rfc8210-5.2-1), [`RFC8210-7-4`](#rfc8210-7-4), [`RFC8210-4-1`](#rfc8210-4-1), [`RFC8210-8.3-1`](#rfc8210-8.3-1), [`RFC8210-12-1`](#rfc8210-12-1), [`RFC8210-9-3`](#rfc8210-9-3), [`RFC8210-9.2-1`](#rfc8210-9.2-1), [`RFC8210-9.2-2`](#rfc8210-9.2-2), [`RFC8210-9.2-4`](#rfc8210-9.2-4), [`RFC8210-9.2-5`](#rfc8210-9.2-5), [`RFC8210-10-1`](#rfc8210-10-1), [`RFC8210-7-7`](#rfc8210-7-7), [`RFC8210-9.2-8`](#rfc8210-9.2-8), [`RFC8210-10-2`](#rfc8210-10-2), [`RFC8210-9.2-10`](#rfc8210-9.2-10), [`RFC8210-8.4-1`](#rfc8210-8.4-1), [`RFC8210-3-1`](#rfc8210-3-1), [`RFC8210-8.1-3`](#rfc8210-8.1-3), [`RFC8210-9.2-11`](#rfc8210-9.2-11)

**Annotated (including scoped evidence) (29):** [`RFC8210-5.3-1`](#rfc8210-5.3-1), [`RFC8210-5.3-2`](#rfc8210-5.3-2), [`RFC8210-5.5-1`](#rfc8210-5.5-1), [`RFC8210-5.6-1`](#rfc8210-5.6-1), [`RFC8210-5.10-1`](#rfc8210-5.10-1), [`RFC8210-5.8-1`](#rfc8210-5.8-1), [`RFC8210-5.11-3`](#rfc8210-5.11-3), [`RFC8210-5.11-4`](#rfc8210-5.11-4), [`RFC8210-6-1`](#rfc8210-6-1), [`RFC8210-7-1`](#rfc8210-7-1), [`RFC8210-7-2`](#rfc8210-7-2), [`RFC8210-8.1-1`](#rfc8210-8.1-1), [`RFC8210-8.2-1`](#rfc8210-8.2-1), [`RFC8210-9-1`](#rfc8210-9-1), [`RFC8210-9-2`](#rfc8210-9-2), [`RFC8210-9.1-1`](#rfc8210-9.1-1), [`RFC8210-9.1-2`](#rfc8210-9.1-2), [`RFC8210-9.2-3`](#rfc8210-9.2-3), [`RFC8210-9.2-6`](#rfc8210-9.2-6), [`RFC8210-9.3-1`](#rfc8210-9.3-1), [`RFC8210-9.4-1`](#rfc8210-9.4-1), [`RFC8210-4-2`](#rfc8210-4-2), [`RFC8210-4-3`](#rfc8210-4-3), [`RFC8210-9.3-2`](#rfc8210-9.3-2), [`RFC8210-9.4-2`](#rfc8210-9.4-2), [`RFC8210-9.4-3`](#rfc8210-9.4-3), [`RFC8210-9.2-9`](#rfc8210-9.2-9), [`RFC8210-8.1-2`](#rfc8210-8.1-2), [`RFC8210-2-1`](#rfc8210-2-1)

## Figures

- **MUST bar**: Ze verified 35, Not built 20
- **Overview bar**: Ze MUST verified 35, SHOULD verified 6, Not owed 4, Not assessed 19, Not built 22

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 59 |
| MUST rows kept (the MUST bar) | 55 |
| Ze verified | 35 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 20 |
| MUST rows not owed | 4 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 4 |
| SHOULD / MAY rows | 27 |
| SHOULD verified | 6 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 19 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8210.json](../../../data/rfc/rfc8210.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8210-5-1` | Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRTRReservedOctetsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L471). **positive:** `unit/verify` [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L77). **positive:** `unit/verify` [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L16). **positive:** `unit/verify` [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L52). **negative:** `unit/verify` [`TestRTRReservedOctetsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L492). **negative:** `unit/verify` [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L32) |
| `RFC8210-5.1-1` | The remaining bits in the Flags field are reserved for future use. In protocol version 1, they MUST be zero on transmission and MUST be ignored on receipt. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L322). **negative:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L331). **negative:** `unit/verify` [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L78) |
| `RFC8210-5.1-2` | Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the Prefix element. This MUST NOT be less than the Prefix Length element. (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L50). **negative:** `unit/verify` [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L121) |
| `RFC8210-5.1-3` | If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data") (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L33). **negative:** `unit/verify` [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L34) |
| `RFC8210-5.1-4` | If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L35). **negative:** `unit/verify` [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L36) |
| `RFC8210-5.1-5` | routers MUST treat sessions with different Protocol Version fields as separate sessions even if they do happen to have the same Session ID. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC8210DifferentVersionSameSessionIDIsSeparateSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L163). **negative:** `unit/verify` [`TestRFC8210DifferentVersionSameSessionIDIsSeparateSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L164) |
| `RFC8210-5.3-1` | When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache's reply to a Serial Query, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| `RFC8210-5.3-2` | That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache's merge of changes, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| `RFC8210-5.5-1` | When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache's payload PDUs, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| `RFC8210-5.6-1` | The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received Prefix PDUs (parseIPv4Prefix, parseIPv6Prefix) and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| `RFC8210-5.10-1` | The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received PDUs and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| `RFC8210-5.10-2` | For this reason, implementations MUST compare Subject Public Key values as well as SKIs when detecting duplicate PDUs. (§5.10) | MUST | 5.10 | **positive:** `unit/verify` [`TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L40). **negative:** `unit/verify` [`TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L41) |
| `RFC8210-5.8-1` | The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Session ID and Protocol Version rule binds the cache's End of Data PDU, but its sentence names fields, not a role, and takes no exclusion shape the gate accepts; ze writes no End of Data or Cache Response PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| `RFC8210-5.11-1` | An Error Report PDU MUST NOT be sent for an Error Report PDU. (§5.11) | MUST NOT | 5.11 | **positive:** `unit/verify` [`TestRTRErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L186). **negative:** `unit/verify` [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L148) |
| `RFC8210-5.11-2` | If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (§5.11) | MUST | 5.11 | **positive:** `unit/verify` [`TestRFC8210GenericErrorReportHasEmptyErroneousPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L210). **negative:** `unit/verify` [`TestRFC8210GenericErrorReportHasEmptyErroneousPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L211) |
| `RFC8210-5.11-3` | If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no exclusion shape fits this conditional sentence; writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go takes no text argument, so ze never sends error text and the condition never arises, and no test carries this id |
| `RFC8210-5.11-4` | The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (§5.11) | MUST | 5.11 | **positive:** `unit/verify` [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L149). **negative:** no negative test. **{single-polarity}:** ze never emits diagnostic text: internal/component/bgp/plugins/rpki/rtr_pdu.go::writeErrorReport takes no text argument and always writes a zero Length of Error Text, so no input makes ze send a report with text to contrast against |
| `RFC8210-6-1` | Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval."; binds caches; ze fills only the router role: parseEndOfData in internal/component/bgp/plugins/rpki/rtr_pdu.go reads the intervals a cache sends, and ze writes no End of Data PDU (rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)) |
| `RFC8210-6-2` | Expire Interval: This parameter tells the router how long it can continue to use the current version of the data while unable to perform a successful subsequent query. The router MUST NOT retain the data past the time indicated by this parameter. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L13). **negative:** `unit/verify` [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L14). **negative:** `unit/verify` [`TestRTRSerialDeltaCannotRenewAnElapsedLease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L93) |
| `RFC8210-7-1` | A router MUST start each transport connection by issuing either a Reset Query or a Serial Query. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L505). **negative:** no negative test. **{single-polarity}:** connectAndSync writes a Reset Query or a Serial Query as the first bytes of every connection (internal/component/bgp/plugins/rpki/rtr_session.go:165-176) before entering readLoop, and no received input can make the router open a connection without a query, so there is no negative case |
| `RFC8210-7-2` | If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds a cache, but its subject carries a which clause, so no exclusion shape the gate accepts fits; ze is the router, sends its queries at version 1 or 2 and receives none (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out), so the obligation has no producer and no test |
| `RFC8210-7-3` | The cache may reply with a version 0 response. In this case, the router MUST either downgrade to version 0 or terminate the connection. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L226). **negative:** `unit/verify` [`TestRFC8210V1CacheResponseIsNotTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_version_test.go#L23) |
| `RFC8210-5.2-1` | If the router receives a Serial Notify PDU during the initial startup period where the router and cache are still negotiating to agree on a protocol version, the router MUST simply ignore the Serial Notify PDU, even if the Serial Notify PDU is for an unexpected protocol version. (§5.2, §7) | MUST | 5.2 | **positive:** `unit/verify` [`TestRTRStartupSerialNotifyOfAnyVersionIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L263). **positive:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L420). **negative:** `unit/verify` [`TestRTRStartupSerialNotifyOfAnyVersionIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L292). **negative:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L444) |
| `RFC8210-7-4` | If either party receives a PDU for a different Protocol Version once the above negotiation completes, that party MUST drop the session (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRTLSSupportedVersionCannotChangeWithinResponse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L120). **negative:** `unit/verify` [`TestRTRTLSSupportedVersionCannotChangeWithinResponse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L121) |
| `RFC8210-4-1` | The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L442). **positive:** `unit/verify` [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L32). **negative:** `unit/verify` [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L443). **negative:** `unit/verify` [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L33) |
| `RFC8210-8.1-1` | To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query periodically. (§8.1, §8.2) | MUST | 8.1 | **positive:** `unit/verify` [`TestRunPollsOnRefreshAfterSuccessRetryAfterFailure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_poll_test.go#L25). **negative:** no negative test. **{single-polarity}:** this is a periodic sender obligation. cacheGroup.Run selects the cache-supplied refresh interval after success and retry interval after failure; TestRunPollsOnRefreshAfterSuccessRetryAfterFailure observes actual Reset and Serial Queries rather than timer fields |
| `RFC8210-8.3-1` | Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L440). **positive:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L302). **positive:** `unit/verify` [`TestRTRCacheResetReloadsTheWholeSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L326). **negative:** `unit/verify` [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L441). **negative:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L316). **negative:** `unit/verify` [`TestRTRCacheResetReloadsTheWholeSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L338) |
| `RFC8210-12-1` | Errors which are considered fatal MUST cause the session to be dropped. (§12) | MUST | 12 | **positive:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L83). **positive:** `unit/verify` [`TestRTRFatalErrorReportDropsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L143). **negative:** `unit/verify` [`TestIsFatalError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L200). **negative:** `unit/verify` [`TestRTRFatalErrorReportDropsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L172) |
| `RFC8210-8.2-1` | The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "The cache MUST rate-limit Serial Notifies to no more frequently than one per minute."; binds cache; ze fills only the router role: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go reads a received Serial Notify, and rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), none of them a Serial Notify |
| `RFC8210-9-1` | Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 14. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go#L88). **positive:** `unit/verify` [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L25). **negative:** no negative test. **{single-polarity}:** Ze offers unprotected TCP with explicit trusted-network true and defaults that transport to port 323 (internal/component/bgp/plugins/rpki/rpki_config.go; internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-9-2` | If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network."; binds a network operator; network topology and trust boundary are deployment obligations Ze cannot observe: internal/component/bgp/plugins/rpki/rpki_config.go::parseRPKIConfig refuses unprotected TCP unless trusted-network true is configured |
| `RFC8210-9-3` | If available to the operator, caches and routers MUST use one of the following more protected protocols: (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L34). **negative:** `unit/verify` [`TestRTRTLSNoPlaintextFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L182) |
| `RFC8210-9.1-1` | Cache servers supporting SSH transport MUST accept RSA authentication (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Cache servers supporting SSH transport MUST accept RSA authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication."; binds cache servers supporting SSH transport; ze fills only the router role and accepts no RTR connection on any transport: connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out to each configured cache |
| `RFC8210-9.1-2` | User authentication MUST be supported (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSHv2 is a MAY transport in Section 9 and the owner declined no SSH feature, so the obligation has no producer and no test |
| `RFC8210-9.2-1` | Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L35). **negative:** `unit/verify` [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L136) |
| `RFC8210-9.2-2` | Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L36). **negative:** `unit/verify` [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L137) |
| `RFC8210-9.2-3` | when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the check binds the cache, but the sentence's first MUST binds the router's certificate, so no exclusion shape the gate accepts fits the row; ze is the TLS client and authenticates no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| `RFC8210-9.2-4` | Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid MITM attacks. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L37). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L94) |
| `RFC8210-9.2-5` | rpki-rtr implementations which use TLS MUST NOT use Common Name (CN-ID) identifiers (§9.2) | MUST NOT | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L38). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L95) |
| `RFC8210-9.2-6` | the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache's server certificate, but the sentence's first MUST binds certification authorities, so no exclusion shape the gate accepts fits the row; ze's router refuses a cache certificate without a matching dNSName (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go, proven under RFC8210-9.2-4), which enforces the rule from the client side but is not this obligation, and no test carries this id |
| `RFC8210-9.3-1` | If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 9 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| `RFC8210-9.4-1` | Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC8210-10-1` | If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation of BGP announcements. (§10) | MUST NOT | 10 | **positive:** `unit/verify` [`TestRTRValidationIgnoresWhichCacheSupplied`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L555). **positive:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_roa_cache_test.go#L152). **negative:** `unit/verify` [`TestRTRValidationIgnoresWhichCacheSupplied`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L558). **negative:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_roa_cache_test.go#L157) |
| `RFC8210-4-2` | As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the cache server's clock, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and evaluates no certificate or ROA, so the obligation has no producer and no test |
| `RFC8210-4-3` | Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the comparison is the cache's choice of the changes since the router's serial, but the sentence names no role and takes no exclusion shape the gate accepts; ze's router sends its serial in a Serial Query (writeSerialQuery in internal/component/bgp/plugins/rpki/rtr_pdu.go) and orders no serials, so the obligation has no producer and no test |
| `RFC8210-5.1-6` | To reduce the risk of confusion, cache servers SHOULD NOT use the same Session ID across multiple protocol versions (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.6-2` | Should the router client receive an IPvX PDU with a {Prefix, Len, Max-Len, ASN} identical to one it already has active, it SHOULD raise a Duplicate Announcement Received error. (§5.6, §5.10) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-5` | If an erroneous Error Report PDU is received, the session SHOULD be dropped. (§5.11) | SHOULD | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-3` | Refresh Interval: This parameter tells the router how long to wait before next attempting to poll the cache and between subsequent attempts, using a Serial Query or Reset Query PDU. The router SHOULD NOT poll the cache sooner than indicated by this parameter. (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-4` | Retry Interval: This parameter tells the router how long to wait before retrying a failed Serial Query or Reset Query. The router SHOULD NOT retry sooner than indicated by this parameter. (§6) | SHOULD NOT | 6 | **positive:** `unit/verify` [`TestCacheGroupFullRoundWaitsForRetryInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L564). **negative:** `unit/verify` [`TestCacheGroupFullRoundWaitsForRetryInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L565) |
| `RFC8210-7-5` | Caches SHOULD NOT send Serial Notify PDUs before version negotiation completes. (§7) | SHOULD NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.3-2` | When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. (§8.3) | SHOULD | 8.3 | **positive:** `unit/verify` [`TestRTRCacheResetReturnsToPreferredCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L219). **negative:** `unit/verify` [`TestRTRCacheResetReturnsToPreferredCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L220) |
| `RFC8210-9-4` | To reduce exposure to dropped but non-terminated sessions, both caches and routers SHOULD enable keep-alives when available in the chosen transport protocol. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-5` | Caches and routers SHOULD use TCP-AO transport [RFC5925] over the rpki-rtr port. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-3` | Cache servers supporting SSH transport MUST accept RSA authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication. (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-4` | Client routers SHOULD verify the public key of the cache to avoid MITM attacks. (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-6` | If the router has never issued a successful query against a particular cache, it SHOULD retry periodically using the default Retry Interval, above. (§6) | SHOULD | 6 | **positive:** `unit/verify` [`TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L683). **negative:** `unit/verify` [`TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L690) |
| `RFC8210-9.2-12` | when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection. (§9.2) | SHOULD | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the rejection binds the cache, but the sentence's first keyword is a MUST binding the router's certificate, not this row's SHOULD, so no exclusion shape the gate accepts fits; ze is the TLS client and rejects no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| `RFC8210-9.2-7` | DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*". (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.2-2` | Upon receipt of a Serial Notify PDU, the router MAY issue an immediate Serial Query (Section 5.3) or Reset Query (Section 5.4) without waiting for the Refresh Interval timer (see Section 6) to expire. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-6` | If the error is associated with a PDU of excessive length, i.e., too long to be any legal PDU other than another Error Report, or a possibly corrupt length, the Erroneous PDU field MAY be truncated. (§5.11) | MAY | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-6` | The cache may terminate the connection, perhaps with a version 0 Error Report PDU. In this case, the router MAY retry the connection using protocol version 0. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-5` | if the router needs to downgrade to a lower protocol version number, it MAY send the first Serial Query or Reset Query immediately. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-5` | host authentication MAY be supported. Implementations MAY support password authentication. (§9.1) | MAY | 9.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSHv2 is a MAY transport in Section 9 and the owner declined no SSH feature, so the obligation has no producer and no test |
| `RFC8210-9-6` | Caches and routers MAY use Secure Shell version 2 (SSHv2) transport [RFC4252] using the normal SSH port. For an example, see Section 9.1. o Caches and routers MAY use TCP MD5 transport [RFC2385] using the rpki-rtr port. Note that TCP MD5 has been obsoleted by TCP-AO [RFC5925]. o Caches and routers MAY use TCP over IPsec transport [RFC4301] using the rpki-rtr port. o Caches and routers MAY use Transport Layer Security (TLS) transport [RFC5246] using port rpki-rtr-tls (324); see Section 14. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-7` | If either party receives a PDU containing an unrecognized Protocol Version (neither 0 nor 1) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU unless the received PDU is itself an Error Report PDU. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L224). **negative:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L225). **negative:** `unit/verify` [`TestRTRUnknownVersionErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L204) |
| `RFC8210-9.2-8` | o The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSDNSAddressIsTheReference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L221). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L97) |
| `RFC8210-10-2` | A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L632). **positive:** `unit/verify` [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L126). **negative:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L633). **negative:** `unit/verify` [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L127) |
| `RFC8210-9.3-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 9 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| `RFC8210-9.4-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC8210-9.4-3` | The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC8210-9.2-9` | Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds certification authorities, but its subject carries a which clause, so no exclusion shape the gate accepts fits; ze serves no rpki-rtr endpoint and its own CA (internal/component/pki/ca.go) issues certificates only for Ze's internal listeners, never an rpki-rtr server certificate, so the obligation has no producer and no test |
| `RFC8210-9.2-10` | a CN field may be present in the server certificate's subject name but MUST NOT be used for authentication within the rules described in [RFC6125]. (§9.2) | MUST NOT | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L39). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L96) |
| `RFC8210-8.1-2` | When a transport connection is first established, the router MUST send either a Reset Query or a Serial Query. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L508). **negative:** no negative test. **{single-polarity}:** the first bytes connectAndSync writes on a freshly established transport are a Reset Query when the serial is 0 and a Serial Query otherwise (internal/component/bgp/plugins/rpki/rtr_session.go:165-176); no received input can produce a connection that carries no opening query, so there is no negative case |
| `RFC8210-8.4-1` | When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (§8.4) | MUST | 8.4 | **positive:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L333). **negative:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L334) |
| `RFC8210-2-1` | While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds a cache's transmission, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router, sends no payload data to any party, and stages a received delta until End of Data (pendingVRPs in internal/component/bgp/plugins/rpki/rtr_session.go), so the obligation has no producer and no test |
| `RFC8210-3-1` | A Relying Party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any cache(s) it uses. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L33). **positive:** `unit/verify` [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L26). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L93). **negative:** `unit/verify` [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L61) |
| `RFC8210-8.1-3` | In all other cases, the router lacks the necessary data for fast resynchronization and therefore MUST fall back to a Reset Query. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_resync_test.go#L67). **positive:** `unit/verify` [`TestRTRResetQueryWithoutFastResyncData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L386). **negative:** `unit/verify` [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_resync_test.go#L81). **negative:** `unit/verify` [`TestRTRResetQueryWithoutFastResyncData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L448) |
| `RFC8210-9.2-11` | o Support for the DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations which use TLS. (§9.2) | REQUIRED | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L32). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L92) |
| `RFC8210-7-9` | unless the PDU containing the unexpected Protocol Version was itself an Error Report PDU, the party dropping the session SHOULD send an Error Report with an error code of 8 ("Unexpected Protocol Version"). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-10-3` | The client SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a different cache or established a new connection to the previous cache. (§10) | SHOULD | 10 | **positive:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L634). **negative:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L635) |
| `RFC8210-10-4` | If a client loses connectivity to a cache it is using or otherwise decides to switch to a new cache, it SHOULD retain the data from the previous cache until it has a full set of data from one or more other caches. (§10) | SHOULD | 10 | **positive:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L636). **negative:** `unit/verify` [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L637) |
| `RFC8210-9.3-3` | Key rollover with TCP MD5 is problematic. Cache servers SHOULD support [RFC4808]. (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.4-2` | When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. (§8.4) | SHOULD | 8.4 | **positive:** `unit/verify` [`TestNoDataAvailableMovesToNextCacheInPreferenceOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L720). **negative:** `unit/verify` [`TestNoDataAvailableMovesToNextCacheInPreferenceOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L721) |
| `RFC8210-11-1` | The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa, before any data are exchanged. (§13) | SHOULD | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.2-2` | The cache server SHOULD send a Notify PDU with its current Serial Number when the cache's serial changes (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8210-5.3-1`](#rfc8210-5.3-1) When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3) | {gap}, no test | the obligation binds the cache's reply to a Serial Query, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| [`RFC8210-5.3-2`](#rfc8210-5.3-2) That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3) | {gap}, no test | the obligation binds the cache's merge of changes, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| [`RFC8210-5.5-1`](#rfc8210-5.5-1) When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5) | {gap}, no test | the obligation binds the cache's payload PDUs, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| [`RFC8210-5.6-1`](#rfc8210-5.6-1) The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6) | {gap}, no test | the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received Prefix PDUs (parseIPv4Prefix, parseIPv6Prefix) and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| [`RFC8210-5.10-1`](#rfc8210-5.10-1) The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10) | {gap}, no test | the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received PDUs and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| [`RFC8210-5.8-1`](#rfc8210-5.8-1) The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8) | {gap}, no test | the Session ID and Protocol Version rule binds the cache's End of Data PDU, but its sentence names fields, not a role, and takes no exclusion shape the gate accepts; ze writes no End of Data or Cache Response PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)), so the obligation has no producer and no test |
| [`RFC8210-5.11-3`](#rfc8210-5.11-3) If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11) | {gap}, no test | no exclusion shape fits this conditional sentence; writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go takes no text argument, so ze never sends error text and the condition never arises, and no test carries this id |
| [`RFC8210-6-1`](#rfc8210-6-1) Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: "Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval."; binds caches; ze fills only the router role: parseEndOfData in internal/component/bgp/plugins/rpki/rtr_pdu.go reads the intervals a cache sends, and ze writes no End of Data PDU (rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport)) |
| [`RFC8210-7-2`](#rfc8210-7-2) If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7) | {gap}, no test | the obligation binds a cache, but its subject carries a which clause, so no exclusion shape the gate accepts fits; ze is the router, sends its queries at version 1 or 2 and receives none (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out), so the obligation has no producer and no test |
| [`RFC8210-8.2-1`](#rfc8210-8.2-1) The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2) | no test | no test carries this requirement id; annotated {not-applicable}: "The cache MUST rate-limit Serial Notifies to no more frequently than one per minute."; binds cache; ze fills only the router role: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go reads a received Serial Notify, and rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), none of them a Serial Notify |
| [`RFC8210-9-2`](#rfc8210-9-2) If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: "If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network."; binds a network operator; network topology and trust boundary are deployment obligations Ze cannot observe: internal/component/bgp/plugins/rpki/rpki_config.go::parseRPKIConfig refuses unprotected TCP unless trusted-network true is configured |
| [`RFC8210-9.1-1`](#rfc8210-9.1-1) Cache servers supporting SSH transport MUST accept RSA authentication (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: "Cache servers supporting SSH transport MUST accept RSA authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication."; binds cache servers supporting SSH transport; ze fills only the router role and accepts no RTR connection on any transport: connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out to each configured cache |
| [`RFC8210-9.1-2`](#rfc8210-9.1-2) User authentication MUST be supported (§9.1) | {gap}, no test | ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSHv2 is a MAY transport in Section 9 and the owner declined no SSH feature, so the obligation has no producer and no test |
| [`RFC8210-9.2-3`](#rfc8210-9.2-3) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2) | {gap}, no test | the check binds the cache, but the sentence's first MUST binds the router's certificate, so no exclusion shape the gate accepts fits the row; ze is the TLS client and authenticates no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| [`RFC8210-9.2-6`](#rfc8210-9.2-6) the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2) | {gap}, no test | the obligation binds the cache's server certificate, but the sentence's first MUST binds certification authorities, so no exclusion shape the gate accepts fits the row; ze's router refuses a cache certificate without a matching dNSName (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go, proven under RFC8210-9.2-4), which enforces the rule from the client side but is not this obligation, and no test carries this id |
| [`RFC8210-9.3-1`](#rfc8210-9.3-1) If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3) | {gap}, no test | ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 9 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| [`RFC8210-9.4-1`](#rfc8210-9.4-1) Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4) | {gap}, no test | ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC8210-4-2`](#rfc8210-4-2) As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4) | {gap}, no test | the obligation binds the cache server's clock, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and evaluates no certificate or ROA, so the obligation has no producer and no test |
| [`RFC8210-4-3`](#rfc8210-4-3) Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4) | {gap}, no test | the comparison is the cache's choice of the changes since the router's serial, but the sentence names no role and takes no exclusion shape the gate accepts; ze's router sends its serial in a Serial Query (writeSerialQuery in internal/component/bgp/plugins/rpki/rtr_pdu.go) and orders no serials, so the obligation has no producer and no test |
| [`RFC8210-9.2-12`](#rfc8210-9.2-12) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection. (§9.2) | {gap} | the rejection binds the cache, but the sentence's first keyword is a MUST binding the router's certificate, not this row's SHOULD, so no exclusion shape the gate accepts fits; ze is the TLS client and rejects no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| [`RFC8210-9.1-5`](#rfc8210-9.1-5) host authentication MAY be supported. Implementations MAY support password authentication. (§9.1) | {gap} | ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSHv2 is a MAY transport in Section 9 and the owner declined no SSH feature, so the obligation has no producer and no test |
| [`RFC8210-9.3-2`](#rfc8210-9.3-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3) | {gap}, no test | ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 9 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| [`RFC8210-9.4-2`](#rfc8210-9.4-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4) | {gap}, no test | ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC8210-9.4-3`](#rfc8210-9.4-3) The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4) | {gap}, no test | ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 9 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC8210-9.2-9`](#rfc8210-9.2-9) Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | {gap}, no test | the obligation binds certification authorities, but its subject carries a which clause, so no exclusion shape the gate accepts fits; ze serves no rpki-rtr endpoint and its own CA (internal/component/pki/ca.go) issues certificates only for Ze's internal listeners, never an rpki-rtr server certificate, so the obligation has no producer and no test |
| [`RFC8210-2-1`](#rfc8210-2-1) While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2) | {gap}, no test | the obligation binds a cache's transmission, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router, sends no payload data to any party, and stages a received delta until End of Data (pendingVRPs in internal/component/bgp/plugins/rpki/rtr_session.go), so the obligation has no producer and no test |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8210-5-1`](#rfc8210-5-1)

Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-30 (independent judge) after commit 60e9406387 changed only comments in these units (RFC6810 tags removed, VALIDATES prose rewritten; no assertion line changed). RFC 8210 Section 5 reads 'Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt.' Transmit: TestWriteResetQueryZeroesReservedOverGarbage writes a Reset Query over a 0xFF buffer and asserts octets 2-3 are zero, the specified fields are real and octet 8 is untouched, so a writer that inherits residue goes red (TestWriteResetQuery alone writes over a zero buffer and could not); TestWriteSerialQuery is the negative, octets 2-3 of a Serial Query carry the Session ID 0x1234 and are not zeroed. writeResetQuery takes the version as a parameter and zeroes regardless of it, so the rtrVersionMax buffers prove the same write a version 1 session makes. Receipt: TestReservedPrefixFieldsPreserveValidation runs a version 1 session whose IPv4 and IPv6 Prefix PDUs carry 0xFF in every reserved octet and asserts the route validates Valid; TestRTRReservedOctetsIgnoredOnReceipt sends a Cache Reset with 0xFFFF in its reserved octets 2-3 over TCP and asserts the sync still ends with errRtrCacheResetReceivedWillDo and serial 0, while its negative shows the same 0xFFFF in a Cache Response is adopted as the Session ID, so the receiver ignores only reserved fields. Both clauses and both polarities hold.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L32) | unit/verify | not verified |
| negative | [`TestRTRReservedOctetsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L492) | unit/verify | revert, verified |
| positive | [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L77) | unit/verify | not verified |
| positive | [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L52) | unit/verify | not verified |
| positive | [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L16) | unit/verify | not verified |
| positive | [`TestRTRReservedOctetsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L471) | unit/verify | revert, verified |

### [`RFC8210-5.1-1`](#rfc8210-5.1-1)

The remaining bits in the Flags field are reserved for future use. In protocol version 1, they MUST be zero on transmission and MUST be ignored on receipt. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden on receipt: letting reserved Flags bits affect acceptance or the announce/withdraw decision. TestParsePrefixPDUFlagsHighBitsIgnored (rtr_pdu_test.go) asserts 0xFF is accepted (require.NoError) and read as announce, and 0xFE is accepted and read as withdraw (assert.False announce), so a parser that validated or read the high bits goes red; negative TestReservedPrefixFieldsPreserveValidation repeats the withdrawal case through handlePDU for IPv4 and IPv6. Transmit clause: Ze writes only Reset Query, Serial Query and Error Report PDUs (rtr_pdu.go), none of which carries a Flags field, so there is no producer that could violate it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_pdu_reserved_test.go#L78) | unit/verify | not verified |
| negative | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L331) | unit/verify | revert, verified |
| positive | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L322) | unit/verify | revert, verified |

### [`RFC8210-5.1-2`](#rfc8210-5.1-2)

Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the Prefix element. This MUST NOT be less than the Prefix Length element. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-30 (independent judge) after commit 60e9406387 removed the RFC6810-5.1-1 tags from these units; no assertion line changed. RFC 8210 Section 5.1 reads 'Max Length:  An 8-bit unsigned integer denoting the longest prefix allowed by the Prefix element.  This MUST NOT be less than the Prefix Length element.' Negative: TestParseIPv4PrefixMaxLenLessThanPrefixLen builds a version 1 (rtrVersionMin) IPv4 Prefix PDU with prefix length 24, a valid value, and max length 16, and asserts parseIPv4Prefix errors with 'invalid', so the buffer isolates the Max Length rule. Positive: TestParseIPv4Prefix accepts a version 1 PDU with max length 24 over prefix length 8 and pins the VRP's ASN, prefix and max length. The check sits in parsePrefixPDU, which the IPv6 parser shares; removing the maxLen < prefixLen test turns the negative red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L50) | unit/verify | revert, verified |

### [`RFC8210-5.1-3`](#rfc8210-5.1-3)

If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data") (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L33) | unit/verify | revert, verified |

### [`RFC8210-5.1-4`](#rfc8210-5.1-4)

If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRTRTLSSerialResponseSessionIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L35) | unit/verify | revert, verified |

### [`RFC8210-5.1-5`](#rfc8210-5.1-5)

routers MUST treat sessions with different Protocol Version fields as separate sessions even if they do happen to have the same Session ID. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8210DifferentVersionSameSessionIDIsSeparateSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L164) | unit/verify | revert, verified |
| positive | [`TestRFC8210DifferentVersionSameSessionIDIsSeparateSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L163) | unit/verify | revert, verified |

### [`RFC8210-5.3-1`](#rfc8210-5.3-1)

When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.3-1, so there is no linked test.

### [`RFC8210-5.3-2`](#rfc8210-5.3-2)

That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.3-2, so there is no linked test.

### [`RFC8210-5.5-1`](#rfc8210-5.5-1)

When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.5-1, so there is no linked test.

### [`RFC8210-5.6-1`](#rfc8210-5.6-1)

The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.6-1, so there is no linked test.

### [`RFC8210-5.10-1`](#rfc8210-5.10-1)

The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.10-1, so there is no linked test.

### [`RFC8210-5.10-2`](#rfc8210-5.10-2)

For this reason, implementations MUST compare Subject Public Key values as well as SKIs when detecting duplicate PDUs. (§5.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8210 Section 5.10 requires implementations to compare Subject Public Key values as well as SKIs when detecting duplicate Router Key PDUs. TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI exercises real TCP RTR sessions over RTRSession.syncOnce -> handlePDU -> parseRouterKeyPDU -> applyStagedRouterKey (which keys on {SKI [20]byte, ASN uint32, SPKI string}). Positive covers both v1 and v2: announces two Router Keys sharing the same SKI and ASN with distinct SPKIs, a third with a different SKI and identical SPKI/ASN, and a fourth with a different ASN and identical SKI/SPKI, asserting clean sync, io.EOF (no Error Report), all 4 keys retained, and an incremental withdrawal removing only the matching {SKI, ASN, SPKI} while preserving its colliding-SKI sibling. Negative proves true duplicate detection on identical {SKI, ASN, SPKI} both within one sync and across a subsequent incremental Serial Query sync, asserting errRtrDuplicateRouterKey, exact Error Report code 7 encapsulating the offending Router Key PDU, and session close. Both polarities carry verified revert discrimination records against applyStagedRouterKey.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC8210RouterKeyDuplicateComparesSPKIAndSKI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L40) | unit/verify | revert, verified |

### [`RFC8210-5.8-1`](#rfc8210-5.8-1)

The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.8-1, so there is no linked test.

### [`RFC8210-5.11-1`](#rfc8210-5.11-1)

An Error Report PDU MUST NOT be sent for an Error Report PDU. (§5.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering an Error Report with an Error Report. TestRTRErrorReportIsNotAnswered writes an Error Report with version 99 (which would otherwise draw Unsupported Protocol Version) into readLoop and asserts the cache side's one-byte io.ReadFull ends in io.EOF, so any reply byte goes red. Negative TestASPAProviderListErrorOnWire: a malformed ASPA PDU does draw an Error Report (report[1] == pduErrorRpt, code 9, the encapsulated PDU echoed), so a router that never sent Error Reports goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRTRErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L186) | unit/verify | revert, verified |

### [`RFC8210-5.11-2`](#rfc8210-5.11-2)

If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (§5.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8210 Section 5.11 requires that when an error is generic (e.g., Internal Error) and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. TestRFC8210GenericErrorReportHasEmptyErroneousPDU tests both polarities over live TCP RTR sessions. Positive triggers a genuine generic Internal Error (panic in onROAChange after valid Cache Response, IPv4 Prefix, and End of Data) through readLoop -> reportInternalError -> writeErrorReport(..., errInternalError, nil) and asserts exact 16-byte wire length (empty Erroneous PDU), PDU type 10, error code 1, header length 16, Length of Encapsulated PDU == 0, and Length of Error Text == 0. Negative triggers a PDU-associated protocol error (unrecognized version 99 in Cache Response) through readLoop -> handleResponseError -> writeErrorReport(..., errUnsupportedVersion, pduBuf) and asserts total length 16+len(badResp), Length of Encapsulated PDU == len(badResp), and exact encapsulation of badResp, ruling out a writer that vacuously drops encapsulated PDUs. reportInternalError is the sole generic Error Report emitter in rpki; both polarities carry verified revert discrimination records.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8210GenericErrorReportHasEmptyErroneousPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC8210GenericErrorReportHasEmptyErroneousPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_router_key_error_report_test.go#L210) | unit/verify | revert, verified |

### [`RFC8210-5.11-3`](#rfc8210-5.11-3)

If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-5.11-3, so there is no linked test.

### [`RFC8210-5.11-4`](#rfc8210-5.11-4)

The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (§5.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a nonzero Length of Error Text on a report carrying no diagnostic text. TestASPAProviderListErrorOnWire reads the transmitted Error Report and asserts the uint32 after the encapsulated PDU is 0 (assert.Equal uint32(0)). Row carries {single-polarity: positive}: writeErrorReport takes no text, so no input produces a report with text to contrast.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L149) | unit/verify | revert, verified |

### [`RFC8210-6-1`](#rfc8210-6-1)

Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-6-1, so there is no linked test.

### [`RFC8210-6-2`](#rfc8210-6-2)

Expire Interval: This parameter tells the router how long it can continue to use the current version of the data while unable to perform a successful subsequent query. The router MUST NOT retain the data past the time indicated by this parameter. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using VRPs or ASPAs after the Expire Interval measured from the last completed End of Data. TestRTRDataExpiryAndRenewal (synctest) asserts the payloads still validate at expire-1s (assertLive) and, after a renewal and an incomplete response, that at the lease deadline cache.Validate is NotFound and aspaCache.isProvider false ("expired payloads still authorize routes"), with both consumers notified; the negatives assert a completed EOD renews (assertLive after renewal), an incomplete response neither renews nor publishes, and TestRTRSerialDeltaCannotRenewAnElapsedLease asserts a serial EOD after the deadline returns errRtrDataExpired and publishes nothing even with the expiry timer stopped. The quote now carries the Expire Interval definition sentence, which names the parameter the MUST NOT refers to and adds no obligation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L14) | unit/verify | revert, verified |
| negative | [`TestRTRSerialDeltaCannotRenewAnElapsedLease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L93) | unit/verify | not verified |
| positive | [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_expire_test.go#L13) | unit/verify | revert, verified |

### [`RFC8210-7-1`](#rfc8210-7-1)

A router MUST start each transport connection by issuing either a Reset Query or a Serial Query. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a transport connection on which the router does not speak first with a Reset or Serial Query. TestFirstPDUOnConnectionIsAQuery accepts the connection at a real listener and require.NoError on io.ReadFull of the first PDU (a silent router times out), then asserts buf[1] == pduResetQuery for a fresh session and pduSerialQuery for a resuming one. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L505) | unit/verify | revert, verified |

### [`RFC8210-7-2`](#rfc8210-7-2)

If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-7-2, so there is no linked test.

### [`RFC8210-7-3`](#rfc8210-7-3)

The cache may reply with a version 0 response. In this case, the router MUST either downgrade to version 0 or terminate the connection. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after a version 0 reply to the router's query, continuing at the router's own version (neither downgrading nor terminating). TestRTRUnknownNegotiationVersion "unsupported v0": the cache sends a version 0 Cache Response, and the test asserts syncOnce returns errRtrUnsupportedProtocol and the cache's trailing read reaches io.EOF (require.ErrorIs got.err io.EOF), so a router that kept the stream open or loaded the v0 reply goes red. Ze supports no version 0, so terminating is the branch it takes. Negative TestRFC8210V1CacheResponseIsNotTerminated: a reply at the router's own version keeps s.version, publishes its VRP (Validate Valid), and draws no trailing bytes (assert.Empty), so a check that terminated on every Cache Response goes red. The quote now carries the conditioning sentence (the cache replies with a version 0 response).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8210V1CacheResponseIsNotTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_version_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L226) | unit/verify | not verified |

### [`RFC8210-5.2-1`](#rfc8210-5.2-1)

If the router receives a Serial Notify PDU during the initial startup period where the router and cache are still negotiating to agree on a protocol version, the router MUST simply ignore the Serial Notify PDU, even if the Serial Notify PDU is for an unexpected protocol version. (§5.2, §7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent same-scope rejudgment after reading RFC 8210 Sections 5.2 and 7, the Section 5.8 prerequisite, both complete tagged test units and the current readLoop, checkStreamSequence, checkResponseIdentity and handlePDU paths. Section 5.2 requires initial-negotiation Serial Notify to be ignored even for an unexpected protocol version; Section 7 explains that neither its Session ID nor serial is useful before negotiation completes. The complete requirement quote, MUST level, existing enforced verdict and all four original covers (the two polarities on each of two test units) remain unchanged. TestRTRStartupSerialNotifyOfAnyVersionIsIgnored sends an actual version-99 Serial Notify carrying Session ID 0xBEEF and serial 999 through net.Pipe/readLoop before a valid Cache Response and End of Data. It requires readLoop success, EOF rather than any response byte, final Session ID 7 from the Cache Response and final serial 5 from End of Data. Its paired control sends version-99 Cache Response in the same startup window and requires errRtrUnsupportedProtocol plus an Error Report of type 10/code 4; the exemption cannot be widened to every startup PDU. The fixture correction changes only the End of Data Session ID from zero to 7, matching the opening Cache Response as Section 5.8 requires; it does not change the EOD serial 5, notify bytes, unsupported-version control or assertions. This removes a different identity error that would otherwise confound the intended successful sequence. The bounded cache script and joined net.Pipe cleanup are retained. TestSerialNotifyIgnoredDuringStartup separately calls handlePDU with the supported version and requires no error, no completion, no adopted Session ID or serial, and unchanged idle state/version. Its Cache Response control must adopt the offered Session ID and advance the exchange, rejecting a blanket ignore-all implementation. The handler delegates identity checks, then exempts Serial Notify from initial unsupported-version refusal and returns without modifying session data; the stream sequence check does not reject that control PDU before it reaches the handler. The current four discrimination records retain their original requirement, units, polarities, revert route and RTRSession.handlePDU producer. Main receipts c37ecb67 and c2575cdd for the stream unit, and 733c8d68 and fa2111e5 for the direct-handler unit, each show the named clean baseline PASS followed by the intentional producer-disable panic red and the exact recorded polarity. These observations establish that the named units reach the producer; they are not normal-return semantic mutations and do not individually isolate the Serial Notify exemption. No new discrimination credit or broader protocol claim is inferred. The two independent handwritten 8210bis observations and the six existing RFC6810 version-0 unimplemented judgments remain separate; this rejudgment changes neither their scope nor their verdicts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRStartupSerialNotifyOfAnyVersionIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L292) | unit/verify | revert, verified |
| negative | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L444) | unit/verify | revert, verified |
| positive | [`TestRTRStartupSerialNotifyOfAnyVersionIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L263) | unit/verify | revert, verified |
| positive | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L420) | unit/verify | revert, verified |

### [`RFC8210-7-4`](#rfc8210-7-4)

If either party receives a PDU for a different Protocol Version once the above negotiation completes, that party MUST drop the session (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSSupportedVersionCannotChangeWithinResponse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRTRTLSSupportedVersionCannotChangeWithinResponse`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_identity_lifecycle_test.go#L120) | unit/verify | revert, verified |

### [`RFC8210-4-1`](#rfc8210-4-1)

The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 8210 Section 4: 'The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI.' Forbidden: selecting a cache other than the most preferred one that answers. TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers (unit unchanged) lists the standby (preference 200) first and the preferred (10) second, waits for the preferred accept and fails if the standby accepts within 500ms; its failover subtest has the next cache in preference order carry the load when the preferred refuses. New unit TestCacheGroupCacheResetFromPreferredReloadsIt goes through the real config path, rpkiPlugin.startSessions, with the preference 20 cache written first: positive, the preference 10 cache receives Reset, Serial, Reset and its reload (192.0.3.0/24 AS64501) becomes Valid; negative, requireNoQuery on the preference 20 cache for 500ms after that reload and its 203.0.113.0/24 stays NotFound. A config-order pass would query the preference 20 cache first and never reach the three preferred queries, and a Cache Reset handed to the less-preferred cache would query it, so both go red. Positive and negative are distinct assertions. The 'or set of caches' alternative is a permission, not a forbidden behaviour.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L443) | unit/verify | revert, verified |
| negative | [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L442) | unit/verify | revert, verified |
| positive | [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L32) | unit/verify | revert, verified |

### [`RFC8210-8.1-1`](#rfc8210-8.1-1)

To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query periodically. (§8.1, §8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a router that never re-polls. TestRunPollsOnRefreshAfterSuccessRetryAfterFailure (cacheGroup.Run against a real listener, refresh 1s, retry 7200s) waits for a second accept (waitAccept fails the test if none arrives) and asserts it comes within 30s, and that the queries are a Reset Query then a Serial Query. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRunPollsOnRefreshAfterSuccessRetryAfterFailure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_poll_test.go#L25) | unit/verify | revert, verified |

### [`RFC8210-8.3-1`](#rfc8210-8.3-1)

Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (§8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-30 (independent judge) after commit 60e9406387 removed the RFC6810-6.3-1 tags and rewrote the VALIDATES line of TestCacheResetTriggersResetQuery; no assertion line changed. Judges the MUST: RFC 8210 Section 8.3, 'If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache.' TestRTRCacheResetReloadsTheWholeSet runs one configured cache over TCP: queries on the wire are Reset, Serial, Reset after the Cache Reset, the reload's 192.0.2.0/24 becomes Valid and the old 10.0.0.0/24 NotFound, so the load replaces rather than merges; its negative answers the Serial Query with data and sees Serial Queries and a merged delta. TestCacheGroupCacheResetFromPreferredReloadsIt drives two parsed caches through startSessions where the Cache Reset comes from the most preferred: the same Reset, Serial, Reset sequence in the same round, the new set replaces the old, and the preference 20 cache gets no query and its VRP never validates. TestCacheResetTriggersResetQuery pins the session state: a Cache Reset ends the sync with errRtrCacheResetReceivedWillDo and clears the serial, a Serial Notify leaves it. Both the Reset Query and the entire-new-load halves are asserted.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L441) | unit/verify | revert, verified |
| negative | [`TestRTRCacheResetReloadsTheWholeSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L338) | unit/verify | revert, verified |
| negative | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L316) | unit/verify | revert, verified |
| positive | [`TestCacheGroupCacheResetFromPreferredReloadsIt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L440) | unit/verify | revert, verified |
| positive | [`TestRTRCacheResetReloadsTheWholeSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L326) | unit/verify | revert, verified |
| positive | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L302) | unit/verify | revert, verified |

### [`RFC8210-12-1`](#rfc8210-12-1)

Errors which are considered fatal MUST cause the session to be dropped. (§12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping the session after a fatal Error Report. TestRTRFatalErrorReportDropsTheSession drives codes 0,1,3,5,6,7,8 over TCP: each fails syncOnce and the cache's Read returns io.EOF (red if the router keeps reading). Negative: a fatal code 0 followed by a full load on the same connection publishes nothing (VRP NotFound, Synced false), red for a router that logs and continues. Code 4 is dispatched by handlePDU's downgrade branch, which also closes the transport; its EOF is asserted under TestRTRUnknownVersionErrorReportIsNotAnswered (code 4) and the downgrade tests, not in this unit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIsFatalError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L200) | unit/verify | not verified |
| negative | [`TestRTRFatalErrorReportDropsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L172) | unit/verify | revert, verified |
| positive | [`TestRTRFatalErrorReportDropsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L83) | unit/verify | not verified |

### [`RFC8210-8.2-1`](#rfc8210-8.2-1)

The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-8.2-1, so there is no linked test.

### [`RFC8210-9-1`](#rfc8210-9-1)

Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 14. (§9)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go::TestParseRPKIConfigDefaults moved. Re-read 2026-10-30 (independent judge) after commit 60e9406387 removed the RFC6810-7-1 tags from these units; no assertion line changed. RFC 8210 Section 9 reads 'Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 14.' Single-polarity positive, as the row's annotation argues: there is no non-conforming input that a router implementing the transport refuses. TestParseRPKIConfigDefaults asserts a trusted-network cache-server with no port parses to 323. TestRTRUnprotectedTCPFromConfig parses the same entry, asserts port 323, moves only the port to an unprivileged listener, starts the plugin's sessions, and asserts the cache reads a clear-text Reset Query (version byte and PDU type at offsets 0 and 1, so a TLS ClientHello would fail), the plugin syncs and the served VRP validates Valid. A config path that dropped plain TCP or another default port goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rpki_config_test.go#L88) | unit/verify | mutant, verified |
| positive | [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L25) | unit/verify | revert, verified |

### [`RFC8210-9-2`](#rfc8210-9-2)

If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9-2, so there is no linked test.

### [`RFC8210-9-3`](#rfc8210-9-3)

If available to the operator, caches and routers MUST use one of the following more protected protocols: (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using an unprotected stream when the operator has configured a protected transport. TestRTRTLSAuthenticatedCache asserts the RTR exchange completes over TLS 1.2 and 1.3 (observed state.Version, queryType, published VRP and ASPA); negative TestRTRTLSNoPlaintextFallback offers a plaintext RTR server at the TLS address and assertRTRTLSRefused asserts syncOnce errors, zero RTR query bytes reach the peer on any connection, and nothing is published, so a plaintext fallback goes red. Availability is the operator's configuration; the refusal of a cache with neither tls nor trusted-network sits in untagged TestRTRTLSConfigRejectsUnsafeTransport and is RFC8210-3-1's concern.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSNoPlaintextFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L182) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L34) | unit/verify | revert, verified |

### [`RFC8210-9.1-1`](#rfc8210-9.1-1)

Cache servers supporting SSH transport MUST accept RSA authentication (§9.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.1-1, so there is no linked test.

### [`RFC8210-9.1-2`](#rfc8210-9.1-2)

User authentication MUST be supported (§9.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.1-2, so there is no linked test.

### [`RFC8210-9.2-1`](#rfc8210-9.2-1)

Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a TLS session in which the router presents no client certificate, or not the configured one. The fixture server uses RequireAndVerifyClientCert, and TestRTRTLSAuthenticatedCache asserts the cache received the configured leaf (bytes.Equal chain[0].Raw) and both intermediates, so a router sending no certificate fails the handshake (t.Fatalf on syncOnce) and one sending another goes red. Negative TestRTRTLSRejectsUnusableClientIdentity: missing CA, missing certificate, missing or mismatched key, wrong EKU and missing IP SAN all go through assertRTRTLSRefused, with a working alternate identity present so a silent substitution goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L35) | unit/verify | revert, verified |

### [`RFC8210-9.2-2`](#rfc8210-9.2-2)

Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating to the cache with a router certificate that carries no iPAddress SAN. Negative TestRTRTLSRejectsUnusableClientIdentity "missing IP SAN" installs a client-auth certificate without an IP SAN and asserts assertRTRTLSRefused (syncOnce error, zero query bytes); the fixture server does not check IP SANs, so the refusal is Ze's and a Ze that sent the certificate would sync and go red. Positive TestRTRTLSAuthenticatedCache presents the fixture router certificate (issued with an iPAddress SAN) and asserts it is the one the cache received; it does not assert the SAN content itself.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L36) | unit/verify | revert, verified |

### [`RFC8210-9.2-3`](#rfc8210-9.2-3)

when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.2-3, so there is no linked test.

### [`RFC8210-9.2-4`](#rfc8210-9.2-4)

Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid MITM attacks. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a cache certificate without verifying it against the dNSName reference. Negative TestRTRTLSRejectsUnauthenticatedCaches "wrong DNS" (server-name other.rtr.test) and "CN only" (no dNSName SAN) both go through assertRTRTLSRefused: syncOnce error, zero query bytes, nothing published, so a router that skipped name verification goes red. Positive TestRTRTLSAuthenticatedCache: a trusted certificate whose dNSName matches syncs and publishes VRP and ASPA.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L37) | unit/verify | revert, verified |

### [`RFC8210-9.2-5`](#rfc8210-9.2-5)

rpki-rtr implementations which use TLS MUST NOT use Common Name (CN-ID) identifiers (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating a cache by its Common Name. Negative TestRTRTLSRejectsUnauthenticatedCaches "CN only" issues a server certificate whose CN matches the reference and has no DNS SAN, and assertRTRTLSRefused asserts it is refused, so a CN fallback goes red. Positive TestRTRTLSAuthenticatedCache: a certificate whose CN differs from the reference but whose DNS-ID matches authenticates, so a CN check that overrode DNS-ID goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L38) | unit/verify | revert, verified |

### [`RFC8210-9.2-6`](#rfc8210-9.2-6)

the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.2-6, so there is no linked test.

### [`RFC8210-9.3-1`](#rfc8210-9.3-1)

If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.3-1, so there is no linked test.

### [`RFC8210-9.4-1`](#rfc8210-9.4-1)

Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.4-1, so there is no linked test.

### [`RFC8210-10-1`](#rfc8210-10-1)

If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation of BGP announcements. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-30 (independent judge) after commit 60e9406387 removed the RFC6810-8-2 tags and rewrote the VALIDATES line of TestValidationDoesNotDistinguishCacheSource; no assertion line changed. RFC 8210 Section 10 reads 'If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation of BGP announcements.' Ze holds one cache's set at a time (the group's End of Data replaces it), so the reachable form of the rule is that validation is blind to which configured cache supplied the data. TestRTRValidationIgnoresWhichCacheSupplied starts the plugin's sessions twice, the VRP served by the preferred cache and then by the standby with the preferred refused, and asserts the Valid, Invalid and NotFound triple is identical and pins each member, so weighting, marking or wildcarding by source goes red. TestValidationDoesNotDistinguishCacheSource adds the held-together case on the shared ROACache: two batches standing for two caches each validate Valid and an origin mismatch is Invalid on content alone.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_roa_cache_test.go#L157) | unit/verify | not verified |
| negative | [`TestRTRValidationIgnoresWhichCacheSupplied`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L558) | unit/verify | revert, verified |
| positive | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_roa_cache_test.go#L152) | unit/verify | not verified |
| positive | [`TestRTRValidationIgnoresWhichCacheSupplied`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L555) | unit/verify | revert, verified |

### [`RFC8210-4-2`](#rfc8210-4-2)

As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-4-2, so there is no linked test.

### [`RFC8210-4-3`](#rfc8210-4-3)

Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-4-3, so there is no linked test.

### [`RFC8210-6-4`](#rfc8210-6-4)

Retry Interval: This parameter tells the router how long to wait before retrying a failed Serial Query or Reset Query. The router SHOULD NOT retry sooner than indicated by this parameter. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheGroupFullRoundWaitsForRetryInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L565) | unit/verify | revert, verified |
| positive | [`TestCacheGroupFullRoundWaitsForRetryInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L564) | unit/verify | revert, verified |

### [`RFC8210-8.3-2`](#rfc8210-8.3-2)

When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. (§8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 8210 Section 8.3, 'When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list.', together with Section 6, 'The router SHOULD NOT retry sooner than indicated by this parameter.' TestRTRCacheResetReturnsToPreferredCache changed under a ze-test-judge approval: the old round 2 re-asked a cache that had just failed, which Section 6 rules out and poll now refuses. Rounds now: (1) the preferred cache loads and announces a 2s Retry Interval; (2) it answers its Serial Query with No Data Available, and the standby loads by Reset Query, replacing the preferred set (held states checked); (3) the standby answers Serial Query with Cache Reset, the preferred cache is inside its Retry Interval and receives no query, the standby receives the 8.3 Reset Query, the held set is retained, and poll's returned delay must be at most 2s, so the attempt on the more-preferred cache is scheduled for the first moment Section 6 allows; (4) the next round sends the preferred cache a Reset Query, a partial preferred load does not replace the held set, and after End of Data the old-only prefix is NotFound, the shared prefix rejects the old origin and accepts the new, the new-only prefix is Valid, with no further standby query. Starting rounds at the holder, or returning the standby's Refresh or Retry Interval after the Cache Reset, fails the test. Positive (preferred Reset Query and its set used) and negative (standby authorization not retained, standby not queried) are distinct assertions. Revert records on poll re-recorded.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRCacheResetReturnsToPreferredCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L220) | unit/verify | revert, verified |
| positive | [`TestRTRCacheResetReturnsToPreferredCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L219) | unit/verify | revert, verified |

### [`RFC8210-6-6`](#rfc8210-6-6)

If the router has never issued a successful query against a particular cache, it SHOULD retry periodically using the default Retry Interval, above. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess: a cacheGroup whose only cache refuses every connection returns 600s (rtrIntervalRetryDefault) from poll; negative: after one successful sync whose EOD sets Retry 30, a failed poll waits 30s, so the default governs only before the first success. Run repeats poll, which is the periodic retry.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L690) | unit/verify | revert, verified |
| positive | [`TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L683) | unit/verify | revert, verified |

### [`RFC8210-9.2-12`](#rfc8210-9.2-12)

when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection. (§9.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-30 (independent judge) after the row's annotation moved from {not-applicable} to {gap}. RFC 8210 Section 9.2 reads 'Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities; when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection.' The SHOULD binds the cache, which Ze never is, but the sentence's first keyword is the MUST on the router certificate, so no not-applicable shape the gate accepts fits this SHOULD row, and the audit record cannot reclassify the row on its own: it follows the summary's {gap}. Recorded as unimplemented because no Ze code checks a connecting router's IP address or rejects its connection: Ze runs no RTR TLS server, internal/component/bgp/plugins/rpki/rtr_tls.go::RTRSession.startTLS is the client side of a connection internal/component/bgp/plugins/rpki/rtr_session.go::RTRSession.connectAndSync dialled, and internal/component/bgp/plugins/rpki/rtr_tls.go::buildRTRTLSConfig only refuses Ze's own client certificate when it lacks an iPAddress subjectAltName. The obligation binds a role Ze does not fill; the verdict marks it unmet only because the gate admits no exclusion for this sentence.

No test carries RFC8210-9.2-12, so there is no linked test.

### [`RFC8210-9.1-5`](#rfc8210-9.1-5)

host authentication MAY be supported. Implementations MAY support password authentication. (§9.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-30 (independent judge) after the row's annotation moved from {not-applicable} to {gap}. RFC 8210 Section 9.1 reads 'User authentication MUST be supported; host authentication MAY be supported.  Implementations MAY support password authentication.' The sentence binds both ends of an SSH transport, and the router end is Ze's role. Section 9 makes SSHv2 a transport Ze may offer, and no owner decision declines it, so the absent SSH transport is an implementation gap, not an exclusion: the earlier not-applicable verdict, which read 'no SSH transport code exists' as no code path, recorded an unimplemented feature as an exclusion. internal/component/bgp/plugins/rpki/rtr_session.go::RTRSession.connectAndSync dials TCP and, for a cache configured with it, hands the connection to internal/component/bgp/plugins/rpki/rtr_tls.go::RTRSession.startTLS; no code offers an SSH transport, so neither host nor password authentication exists.

No test carries RFC8210-9.1-5, so there is no linked test.

### [`RFC8210-7-7`](#rfc8210-7-7)

If either party receives a PDU containing an unrecognized Protocol Version (neither 0 nor 1) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU unless the received PDU is itself an Error Report PDU. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an unrecognized-version PDU, terminating without a report, or answering an unknown-version Error Report. TestRTRUnknownNegotiationVersion: v99 PDU yields errRtrUnsupportedProtocol plus Error Report code 4 then EOF; v2 completes. New negative TestRTRUnknownVersionErrorReportIsNotAnswered: a v99 Error Report with code 0 and code 4 fails the sync and the cache's io.ReadFull of one byte ends in io.EOF, so any reply byte goes red (the exception clause, guarded by handlePDU's Error Report exemption and readLoop's hdr.Type != pduErrorRpt).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRUnknownVersionErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L204) | unit/verify | revert, verified |
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L225) | unit/verify | not verified |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L224) | unit/verify | not verified |

### [`RFC8210-9.2-8`](#rfc8210-9.2-8)

o The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using a reference identifier other than the cache DNS name (the connected IP, or any name the certificate carries). Negative TestRTRTLSRejectsUnauthenticatedCaches: 'wrong DNS' (server-name other.rtr.test vs cert cache.rtr.test) and 'IP SAN only' (connect to 127.0.0.1, cert has only that IP SAN) both go red via assertRTRTLSRefused (syncOnce error, zero query bytes, nothing published). Positive TestRTRTLSDNSAddressIsTheReference: omitted server-name uses the configured cache DNS name, asserting state.ServerName == localhost and a published VRP.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRTRTLSDNSAddressIsTheReference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L221) | unit/verify | revert, verified |

### [`RFC8210-10-2`](#rfc8210-10-2)

A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden by RFC 8210 Section 10, 'A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data.': an update from one cache applied to another cache's data. Two units. TestRTRCacheSwitchKeepsSerialBasesSeparate: both caches share Session ID and serial; after switching 2 to 3 to 2 the query sequence must be Reset, Serial, Reset (cache 3), Reset (cache 2), so cache 2 is never Serial-queried against cache 3's base, and each round only the selected cache's prefix and ASPA provider validate, so a merge or reuse goes red. TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData (re-judged 2026-10-27 by the closer, who wrote none of its tests): feeds two sessions of one cacheGroup through handlePDU; the new cache's staged withdraw of the old cache's VRP and ASPA leaves the old set intact (require after every PDU, ROA count 2), its End of Data leaves exactly its own VRP and ASPA (count 1, both old ASPAs gone), and a later delta withdraw from the old cache ends in errRtrDataExpired with the new cache's VRP still Valid. The source mark is the shared rtrDataLease generation; a stale-generation delta reaching the store, a merge at End of Data, or a staged PDU reaching the store each fail an assertion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L633) | unit/verify | revert, verified |
| negative | [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L632) | unit/verify | revert, verified |
| positive | [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L126) | unit/verify | revert, verified |

### [`RFC8210-9.3-2`](#rfc8210-9.3-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.3-2, so there is no linked test.

### [`RFC8210-9.4-2`](#rfc8210-9.4-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.4-2, so there is no linked test.

### [`RFC8210-9.4-3`](#rfc8210-9.4-3)

The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.4-3, so there is no linked test.

### [`RFC8210-9.2-9`](#rfc8210-9.2-9)

Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-9.2-9, so there is no linked test.

### [`RFC8210-9.2-10`](#rfc8210-9.2-10)

a CN field may be present in the server certificate's subject name but MUST NOT be used for authentication within the rules described in [RFC6125]. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating a cache by its CN. Negative TestRTRTLSRejectsUnauthenticatedCaches 'CN only' (CN cache.rtr.test, no DNS SAN) is refused via assertRTRTLSRefused. Positive TestRTRTLSAuthenticatedCache: a cert whose CN is not-the-cache-reference.invalid but whose DNS-ID matches authenticates, so a CN check is not used either way.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L39) | unit/verify | revert, verified |

### [`RFC8210-8.1-2`](#rfc8210-8.1-2)

When a transport connection is first established, the router MUST send either a Reset Query or a Serial Query. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a new transport on which the router sends nothing first, or something other than a Reset/Serial Query. TestFirstPDUOnConnectionIsAQuery requires io.ReadFull of the first PDU to succeed and asserts buf[1] == pduResetQuery (fresh) and == pduSerialQuery (resuming). Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L508) | unit/verify | revert, verified |

### [`RFC8210-8.4-1`](#rfc8210-8.4-1)

When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (§8.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after No Data Available with no other cache, stopping, sending Serial Queries, or not repeating the Reset Query until a usable load arrives. TestNoDataAvailableKeepsResetQueryMode runs a single-session cacheGroup (no other cache), the cache answers two queries with Error Report code 2, and the test asserts three consecutive queries are pduResetQuery each within 4s at a 1s retry (a router that stopped hits t.Fatal, one that sent a Serial Query fails assert.Equal); the negative asserts that once a complete load is published the next query is pduSerialQuery, which pins the 'until it gets a new usable load' bound. The widened quote adds the SHOULD failover sentence only as the condition's context; this row judges its MUST.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L334) | unit/verify | revert, verified |
| positive | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L333) | unit/verify | revert, verified |

### [`RFC8210-2-1`](#rfc8210-2-1)

While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8210-2-1, so there is no linked test.

### [`RFC8210-3-1`](#rfc8210-3-1)

A Relying Party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any cache(s) it uses. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-30 (independent judge) after commit 60e9406387 removed the RFC6810-7-1 tag from TestRTRUnprotectedTCPFromConfig; no assertion line changed, and the TLS units are unchanged. RFC 8210 Section 3 reads 'A Relying Party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any cache(s) it uses.' TLS path: TestRTRTLSAuthenticatedCache completes a mutually authenticated handshake under the named CA over TLS 1.2 and 1.3 and only then publishes the cache's VRP and ASPA; TestRTRTLSRejectsUnauthenticatedCaches shows an issuer outside the named CA, wrong DNS-ID, CN-only, IP-SAN-only and one-way TLS each refused before any record is published. Unprotected path: TestRTRUnprotectedTCPFromConfig syncs and uses data from a cache declared trusted-network, and its negative shows cache-server {} and trusted-network false refused by parseRPKIConfig, the declaration being the only difference from the accepted entry. Both the trust relationship and the trusted channel are asserted on both transports.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L93) | unit/verify | not verified |
| negative | [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L33) | unit/verify | not verified |
| positive | [`TestRTRUnprotectedTCPFromConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_transport_test.go#L26) | unit/verify | revert, verified |

### [`RFC8210-8.1-3`](#rfc8210-8.1-3)

In all other cases, the router lacks the necessary data for fast resynchronization and therefore MUST fall back to a Reset Query. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRTRResetQueryWithoutFastResyncData records the first query on the wire in each lacking-data case: no remembered session, expired lease (serial 42 held), cache switch (standby holds serial 42 over a current lease, so only startFullSync forces the Reset), and version downgrade (v2 Serial answered with code 4 at v1, retried as v1 Reset). Negative: same cache with a current lease opens with a Serial Query carrying Session ID 7 and serial 42. Each case isolated from the others.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_resync_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRTRResetQueryWithoutFastResyncData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L448) | unit/verify | revert, verified |
| positive | [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_resync_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRTRResetQueryWithoutFastResyncData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go#L386) | unit/verify | revert, verified |

### [`RFC8210-9.2-11`](#rfc8210-9.2-11)

o Support for the DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations which use TLS. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a TLS client that cannot authenticate the cache by DNS-ID (dNSName SAN). Positive TestRTRTLSAuthenticatedCache: a DNS-ID cert with a differing CN syncs and publishes VRP and ASPA. Negative TestRTRTLSRejectsUnauthenticatedCaches: wrong DNS, CN only, IP SAN only are refused (assertRTRTLSRefused). The server-implementation clause binds a role Ze does not fill: Ze has no RTR cache server (only rtr_session.go and rtr_pdu.go, both client-side).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_tls_test.go#L32) | unit/verify | revert, verified |

### [`RFC8210-10-3`](#rfc8210-10-3)

The client SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a different cache or established a new connection to the previous cache. (§10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L635) | unit/verify | revert, verified |
| positive | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L634) | unit/verify | revert, verified |

### [`RFC8210-10-4`](#rfc8210-10-4)

If a client loses connectivity to a cache it is using or otherwise decides to switch to a new cache, it SHOULD retain the data from the previous cache until it has a full set of data from one or more other caches. (§10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L637) | unit/verify | revert, verified |
| positive | [`TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L636) | unit/verify | revert, verified |

### [`RFC8210-8.4-2`](#rfc8210-8.4-2)

When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. (§8.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNoDataAvailableMovesToNextCacheInPreferenceOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L721) | unit/verify | revert, verified |
| positive | [`TestNoDataAvailableMovesToNextCacheInPreferenceOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go#L720) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-28 |
| Register | rfc2119 |
| Source | rfc/full/rfc8210.txt |
| Source fingerprint | d55f8b529fb430bf |
| Record | rfc/extraction/rfc8210.json |
| Mapped sentences | 55 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 3 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `5.1` | not stated | 4 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `5.3` | not stated | 2 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `5.5` | not stated | 1 | reviewed | not stated |
| `5.6` | not stated | 1 | reviewed | not stated |
| `5.7` | not stated | 0 | reviewed | not stated |
| `5.8` | not stated | 1 | reviewed | not stated |
| `5.9` | not stated | 0 | reviewed | not stated |
| `5.10` | not stated | 2 | reviewed | not stated |
| `5.11` | not stated | 4 | reviewed | not stated |
| `6` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 7 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 3 | reviewed | not stated |
| `8.2` | not stated | 2 | reviewed | not stated |
| `8.3` | not stated | 1 | reviewed | not stated |
| `8.4` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 3 | reviewed | not stated |
| `9.1` | not stated | 2 | reviewed | not stated |
| `9.2` | not stated | 7 | reviewed | not stated |
| `9.3` | not stated | 2 | reviewed | not stated |
| `9.4` | not stated | 4 | reviewed | not stated |
| `10` | not stated | 2 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 1 | reviewed | not stated |
| `13` | not stated | 1 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `15` | not stated | 0 | reviewed | not stated |
| `15.1` | not stated | 0 | reviewed | not stated |
| `15.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 5.2 obligation mapped at site 5.2:1: a router ignores a Serial Notify received during the initial startup period, even one for an unexpected protocol version | The router MUST ignore any Serial Notify PDUs it might receive from the cache during this initial startup period, regardless of the Protocol Version field in the Serial Notify PDU. |
| `7:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the ignore-Serial-Notify-before-negotiation obligation mapped at site 5.2:1, adding only the backwards-compatibility reason | Routers, however, MUST handle such notifications (by ignoring them) for backwards compatibility with caches serving protocol version 0. |
| `8.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the periodic Serial-or-Reset Query obligation already mapped at site 8.1:3, here for the old-withdraw case | To limit the length of time a cache must keep old withdraws, a router MUST send either a Serial Query or a Reset Query periodically. |
| `9.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the MAC-length half of the TCP-AO support obligation the row already states: "key lengths of at least 80 printable ASCII bytes and MAC lengths of at least 96 bits" | Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. |
| `13:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Security Considerations restatement of the section 9 rule that an unprotected TCP transport puts the router and cache on the same trusted, controlled network | Protocols which provide integrity and authenticity SHOULD be used, and if they cannot, i.e., TCP is used as the transport, the router and cache MUST be on the same trusted, controlled network. |

## Superseded

No document obsoletes RFC 8210, so its obligations are stated where they were written.
