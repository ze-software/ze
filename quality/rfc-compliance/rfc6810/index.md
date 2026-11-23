# RFC 6810 - The Resource Public Key Infrastructure (RPKI) to Router Protocol

RPKI-to-Router Protocol v0 (RPKI-RTR), not spoken: ze speaks versions 1 and 2 only (RFC 8210). Every requirement listed for RFC 6810, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 40 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 40 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 40 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 40 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 38 | of 40 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 6 | of 40 mandatory requirements reviewed | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 40 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 2 | of 40 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 40 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 40 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 40 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 40 | of 68 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 40 mandatory requirements: they add up to 40. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | RPKI-to-Router Protocol v0 (RPKI-RTR), not spoken: ze speaks versions 1 and 2 only (RFC 8210) |
| Automatic checks | Included |
| Requirements | 68 |
| Mandatory requirements (MUST level) | 40 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 38 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 6 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc6810.md` |
| Requirement table | `rfc/requirements/rfc6810.md` |
| RFC text | `rfc/full/rfc6810.txt` |

## Automatic checks

Included in automatic checks: RPKI to Router Protocol v0 / RPKI-RTR (RFC 6810): router (cache-client) role. Ze does not speak RTR protocol version 0, which this document defines: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, as RFC 8210 Section 7 permits in place of a downgrade. Version 0 is owed work (owner ruling 2026-10-30). Of the 40 MUST-level rows, 38 are gaps and 2 are not-applicable (cache obligations, each quoting the RFC sentence that binds the cache). No row counts as met: RFC6810-5-1, 5.1-1, 5.1-3, 6.1-1, 6.3-1, 6.4-1, 7-1 and 8-2 were tagged by tests that run version 1 sessions, which prove RFC 8210 behaviour and not RFC 6810, so under that ruling they are gaps and their RFC6810 tags leave those tests. RFC6810-5.1-3 was added 2026-09-21 from the Section 5.1 sentence the checklist had not carried

## Published support status

**Area:** RPKI-to-Router Protocol v0 (RPKI-RTR), not spoken: ze speaks versions 1 and 2 only (RFC 8210)

**Test verification:** 20 of 40 MUST proven

**Evidence:** superseded 20

**Unmet:** RFC6810-2-1, RFC6810-3-1, RFC6810-4-2, RFC6810-5.10-3, RFC6810-5.5-1, RFC6810-5.6-1, RFC6810-5.8-1, RFC6810-7.1-2, RFC6810-7.2-3, RFC6810-7.2-5, RFC6810-7.2-6, RFC6810-7.3-1, RFC6810-7.3-2, RFC6810-7.4-1, RFC6810-7.4-2, RFC6810-7.4-3, RFC6810-7.4-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 40 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **40** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (40):** [`RFC6810-5-1`](#rfc6810-5-1), [`RFC6810-5.1-1`](#rfc6810-5.1-1), [`RFC6810-5.1-3`](#rfc6810-5.1-3), [`RFC6810-5.6-1`](#rfc6810-5.6-1), [`RFC6810-5.5-1`](#rfc6810-5.5-1), [`RFC6810-5.8-1`](#rfc6810-5.8-1), [`RFC6810-5.10-1`](#rfc6810-5.10-1), [`RFC6810-5.10-2`](#rfc6810-5.10-2), [`RFC6810-5.10-3`](#rfc6810-5.10-3), [`RFC6810-5.10-4`](#rfc6810-5.10-4), [`RFC6810-2-1`](#rfc6810-2-1), [`RFC6810-5.1-2`](#rfc6810-5.1-2), [`RFC6810-4-1`](#rfc6810-4-1), [`RFC6810-6.1-1`](#rfc6810-6.1-1), [`RFC6810-6.2-1`](#rfc6810-6.2-1), [`RFC6810-6.3-1`](#rfc6810-6.3-1), [`RFC6810-6.4-1`](#rfc6810-6.4-1), [`RFC6810-3-1`](#rfc6810-3-1), [`RFC6810-4-2`](#rfc6810-4-2), [`RFC6810-7-1`](#rfc6810-7-1), [`RFC6810-7-2`](#rfc6810-7-2), [`RFC6810-7-3`](#rfc6810-7-3), [`RFC6810-7.1-1`](#rfc6810-7.1-1), [`RFC6810-7.1-2`](#rfc6810-7.1-2), [`RFC6810-8-1`](#rfc6810-8-1), [`RFC6810-8-2`](#rfc6810-8-2), [`RFC6810-7.2-1`](#rfc6810-7.2-1), [`RFC6810-7.2-2`](#rfc6810-7.2-2), [`RFC6810-7.2-3`](#rfc6810-7.2-3), [`RFC6810-7.2-4`](#rfc6810-7.2-4), [`RFC6810-7.2-5`](#rfc6810-7.2-5), [`RFC6810-7.2-6`](#rfc6810-7.2-6), [`RFC6810-7.2-7`](#rfc6810-7.2-7), [`RFC6810-7.2-8`](#rfc6810-7.2-8), [`RFC6810-7.3-1`](#rfc6810-7.3-1), [`RFC6810-7.3-2`](#rfc6810-7.3-2), [`RFC6810-7.4-1`](#rfc6810-7.4-1), [`RFC6810-7.4-2`](#rfc6810-7.4-2), [`RFC6810-7.4-3`](#rfc6810-7.4-3), [`RFC6810-7.4-4`](#rfc6810-7.4-4)

## Figures

- **MUST bar**: Ze verified 20, Not built 17
- **Overview bar**: Ze MUST verified 20, SHOULD verified 8, Not owed 3, Not assessed 20, Not built 17

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 40 |
| MUST rows kept (the MUST bar) | 37 |
| Ze verified | 20 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 17 |
| MUST rows not owed | 3 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 3 |
| SHOULD / MAY rows | 28 |
| SHOULD verified | 8 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 20 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6810.json](../../../data/rfc/rfc6810.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6810-5-1` | Fields with unspecified content MUST be zero on transmission (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code zeroes unspecified fields at versions 1 and 2 (writeResetQuery in internal/component/bgp/plugins/rpki/rtr_pdu.go), proven under RFC8210-5-1 by TestWriteResetQuery, TestWriteSerialQuery and TestWriteResetQueryZeroesReservedOverGarbage, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-5.1-1` | Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the prefix. This MUST NOT be less than the Prefix Length element. (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code refuses a Max Length below the Prefix Length at versions 1 and 2 (parsePrefixPDU in internal/component/bgp/plugins/rpki/rtr_pdu.go), proven under RFC8210-5.1-2 by TestParseIPv4Prefix and TestParseIPv4PrefixMaxLenLessThanPrefixLen, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-5.1-3` | The value of such a field MUST be ignored on receipt. (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code ignores reserved fields on receipt at versions 1 and 2 (parsePrefixPDU in internal/component/bgp/plugins/rpki/rtr_pdu.go, handlePDU), proven under RFC8210-5-1 and RFC8210-5.1-1 by TestReservedPrefixFieldsPreserveValidation, TestRTRReservedOctetsIgnoredOnReceipt and TestParsePrefixPDUFlagsHighBitsIgnored, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-5.6-1` | The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received Prefix PDUs (parseIPv4Prefix, parseIPv6Prefix) and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| `RFC6810-5.5-1` | When replying to a Reset Query, the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (Section 5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache's payload PDUs, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go holds only writeResetQuery, writeSerialQuery and writeErrorReport), so the obligation has no producer and no test |
| `RFC6810-5.8-1` | The Session ID MUST be the same as that of the corresponding Cache Response that began the, possibly null, sequence of data PDUs. (Section 5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the Session ID rule binds the cache's End of Data PDU, but its sentence names a field, not a role, and takes no exclusion shape the gate accepts; ze writes no End of Data or Cache Response PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go holds only writeResetQuery, writeSerialQuery and writeErrorReport), so the obligation has no producer and no test |
| `RFC6810-5.10-1` | If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; no exclusion shape fits this conditional sentence; ze's router does send Error Reports, always with the offending PDU encapsulated and never a generic one (readLoop in internal/component/bgp/plugins/rpki/rtr_session.go reports a malformed ASPA PDU or an unsupported version through writeErrorReport), and no test carries this id |
| `RFC6810-5.10-2` | An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | MUST NOT | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (readLoop in internal/component/bgp/plugins/rpki/rtr_session.go writes no Error Report when the failing PDU is itself an Error Report), proven under RFC8210-5.11-1 in rtr_session_test.go, and no test carries this id |
| `RFC6810-5.10-3` | If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3269]). (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; no exclusion shape fits this conditional sentence; writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go takes no text argument, so ze never sends error text, and no test carries this id |
| `RFC6810-5.10-4` | The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go always writes a zero Length of Error Text), proven under RFC8210-5.11-4 in rtr_session_test.go, and no test carries this id |
| `RFC6810-2-1` | As a cache is receiving, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds a cache's transmission, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router, sends no payload data to any party, and stages a received delta until End of Data (pendingVRPs in internal/component/bgp/plugins/rpki/rtr_session.go), so the obligation has no producer and no test |
| `RFC6810-5.1-2` | If, at any time, either the router or the cache finds the value of the session identifier is not the same as the other's, they MUST completely drop the session and the router MUST flush all data learned from that cache. (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze does not detect a Session ID change -- handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go adopts the Cache Response Session ID unconditionally (s.sessionID = hdr.SessionID) with no comparison to the prior value and no cache flush |
| `RFC6810-4-1` | The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (cacheGroup in internal/component/bgp/plugins/rpki/rtr_session.go polls the caches in preference order after sortCachesByPreference in rpki_config.go, and loads from the most preferred one that answers), proven under RFC8210-4-1 in rfc8210_rtr_preference_test.go, and no test carries this id |
| `RFC6810-6.1-1` | To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query no less frequently than once an hour. (Section 6.1, Section 6.2) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code re-queries on the Refresh and Retry Intervals at versions 1 and 2 (cacheGroup.Run and pollDelay in internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.1-1 by TestRunPollsOnRefreshAfterSuccessRetryAfterFailure, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-6.2-1` | The cache MUST rate limit Serial Notifies to no more frequently than one per minute. (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "The cache MUST rate limit Serial Notifies to no more frequently than one per minute."; binds cache; ze fills only the router role: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go reads a received Serial Notify and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only writeResetQuery, writeSerialQuery and writeErrorReport, none of them a Serial Notify |
| `RFC6810-6.3-1` | If there are no more preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (Section 6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code answers a Cache Reset with a Reset Query and a whole new load at versions 1 and 2 (handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.3-1 by TestCacheResetTriggersResetQuery and TestRTRCacheResetReloadsTheWholeSet, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-6.4-1` | If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (Section 6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code repeats Reset Queries after No Data Available at versions 1 and 2 (internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.4-1 by TestNoDataAvailableKeepsResetQueryMode, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-3-1` | A relying party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any authoritative cache(s) it uses. (Section 3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (parseRPKIConfig in internal/component/bgp/plugins/rpki/rpki_config.go refuses a cache with neither TLS nor trusted-network true, and buildRTRTLSConfig in rtr_tls.go trusts only the named CA; the subject holds a list, so no exclusion shape fits, and the router is ze's role), proven under RFC8210-3-1 in rfc8210_rtr_tls_test.go and rfc8210_rtr_transport_test.go, and no test carries this id |
| `RFC6810-4-2` | As a cache server must evaluate certificates and ROAs (Route Origin Attestations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache server's clock, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and evaluates no certificate or ROA, so the obligation has no producer and no test |
| `RFC6810-7-1` | Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 12. (Section 7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code dials unprotected TCP on rpki-rtr port 323 at versions 1 and 2 (internal/component/bgp/plugins/rpki/rtr_session.go, rpki_config.go), proven under RFC8210-9-1 by TestParseRPKIConfigDefaults and TestRTRUnprotectedTCPFromConfig, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-7-2` | If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (Section 7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the routers as well as the cache, so it is ze's; ze cannot see the topology, so parseRPKIConfig in internal/component/bgp/plugins/rpki/rpki_config.go only refuses unprotected TCP unless the cache carries trusted-network true, nothing checks that the network is trusted and controlled, and no test carries this id |
| `RFC6810-7-3` | If available to the operator, caches and routers MUST use one of the following more protected protocols. (Section 7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go starts TLS for a cache configured with it, and parseRPKIConfig in rpki_config.go refuses unprotected TCP unless the operator sets trusted-network true), proven under RFC8210-9-3 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.1-1` | Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication (Section 7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication."; binds cache servers supporting SSH transport; ze fills only the router role and accepts no RTR connection on any transport: connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out to each configured cache |
| `RFC6810-7.1-2` | User authentication MUST be supported (Section 7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; user authentication binds both ends of an SSH transport, and ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSH is a MAY transport in Section 7 and the owner declined no SSH feature, so the obligation has no producer and no test |
| `RFC6810-8-1` | A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (Section 8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (ze loads from one cache at a time and each cache switch replaces the published VRP and ASPA sets with that cache's own (cacheGroup in internal/component/bgp/plugins/rpki/rtr_session.go), so no update from one cache can touch another cache's data), proven under RFC8210-10-2 in rfc8210_rtr_preference_test.go, and no test carries this id |
| `RFC6810-8-2` | If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code validates against the merged cache content at versions 1 and 2 (internal/component/bgp/plugins/rpki/roa_cache.go), proven under RFC8210-10-1 by TestValidationDoesNotDistinguishCacheSource and TestRTRValidationIgnoresWhichCacheSupplied, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| `RFC6810-7.2-1` | Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go presents the named PKI certificate through GetClientCertificate), proven under RFC8210-9.2-1 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.2-2` | Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go refuses a router certificate with no iPAddress subjectAltName), proven under RFC8210-9.2-2 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.2-3` | when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the check binds the cache, but the sentence's first MUST binds the router's certificate, so no exclusion shape the gate accepts fits the row; ze is the TLS client and authenticates no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| `RFC6810-7.2-4` | Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid monkey-in-the-middle attacks. (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go sets ServerName to the DNS reference and RootCAs to the named CA, so Go crypto/tls verifies the cache certificate's dNSName), proven under RFC8210-9.2-4 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.2-5` | Support for DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations that use TLS. (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (rtrServerName in internal/component/bgp/plugins/rpki/rtr_tls.go accepts only a DNS reference name, so the client side supports DNS-ID), proven under RFC8210-9.2-11 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.2-6` | DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache's server certificate, but the sentence's first MUST binds certification authorities, so no exclusion shape the gate accepts fits the row; ze's router refuses a cache certificate without a matching dNSName (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go, proven under RFC8210-9.2-4), which enforces the rule from the client side but is not the issuer's obligation, and no test carries this id |
| `RFC6810-7.2-7` | rpki-rtr implementations that use TLS MUST NOT use CN-ID identifiers; a CN field may be present in the server certificate's subject name, but MUST NOT be used for authentication within the rules described in [RFC6125]. (Section 7.2) | MUST NOT | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go verifies the DNS reference through Go crypto/tls, which matches subjectAltName names only and never falls back to the Common Name), proven under RFC8210-9.2-5 and RFC8210-9.2-10 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.2-8` | The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (rtrServerName in internal/component/bgp/plugins/rpki/rtr_tls.go makes the cache's DNS name the TLS ServerName), proven under RFC8210-9.2-8 in rfc8210_rtr_tls_test.go, and no test carries this id |
| `RFC6810-7.3-1` | If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (Section 7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 7 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| `RFC6810-7.3-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 7 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| `RFC6810-7.4-1` | Implementations MUST support key lengths of at least 80 printable ASCII bytes. (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC6810-7.4-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC6810-7.4-3` | MAC (Message Authentication Code) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC6810-7.4-4` | The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| `RFC6810-6.2-2` | The cache server SHOULD send a notify PDU with its current Serial Number when the cache's serial changes (Section 6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.6-2` | Should the router client receive an IPvX PDU with a {Prefix, Len, Max-Len, ASN} identical to one it already has active, it SHOULD raise a Duplicate Announcement Received error. (Section 5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-4` | Caches and routers SHOULD use TCP-AO, SSHv2, TCP MD5, or IPsec transport (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-5` | To reduce exposure to dropped but non-terminated sessions, both caches and routers SHOULD enable keep-alives when available in the chosen transport protocol. (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.3-2` | Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more preferred caches in its cache list. (Section 6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.4-2` | When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-3` | If a client loses connectivity to a cache it is using, or otherwise decides to switch to a new cache, it SHOULD retain the data from the previous cache until it has a full set of data from one or more other caches. (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-4` | A client SHOULD delete the data from a cache when it has been unable to refresh from that cache for a configurable timer value. The default for that value is twice the polling period for that cache. (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-1` | It also SHOULD be topologically close so that a minimum of validated routing data are needed to bootstrap a router's access to a cache. (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.10-5` | If an erroneous Error Report PDU is received, the session SHOULD be dropped. (Section 5.10) | SHOULD | 5.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-3` | Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication. (Section 7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-4` | Client routers SHOULD verify the public key of the cache to avoid monkey-in-the-middle attacks. (Section 7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.2-9` | when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection. (Section 7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.2-10` | DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*" (Section 7.2) | SHOULD NOT | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-6` | Operators SHOULD use procedural means, e.g., access control lists (ACLs), to reduce the exposure to authentication issues. (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.3-3` | Key rollover with TCP MD5 is problematic. Cache servers SHOULD support [RFC4808]. (Section 7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-5` | The client SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a different cache or established a new connection to the previous cache. (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-2` | The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-3` | Protocols that provide integrity and authenticity SHOULD be used (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-10-1` | Errors that are considered fatal SHOULD cause the session to be dropped. (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.1-2` | Alternatively, if the router has significant unexpired data from a broken session with the same cache, it MAY start with a Serial Query containing the Session ID from the previous session to ensure the Serial Numbers are commensurate. (Section 6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.10-6` | If the error is associated with a PDU of excessive length, i.e., too long to be any legal PDU other than another Error Report, or a possibly corrupt length, the Erroneous PDU field MAY be truncated. (Section 5.10) | MAY | 5.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5-2` | Fields with unspecified content MUST be zero on transmission and MAY be ignored on receipt. (Section 5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-7` | Caches and routers MAY use SSHv2 transport [RFC4252] using a the normal SSH port. For an example, see Section 7.1. Caches and routers MAY use TCP MD5 transport [RFC2385] using the rpki-rtr port. Note that TCP MD5 has been obsoleted by TCP-AO [RFC5925]. Caches and routers MAY use IPsec transport [RFC4301] using the rpki- rtr port. Caches and routers MAY use TLS transport [RFC5246] using a port, rpki-rtr-tls (324); see Section 12. (Section 7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-5` | host authentication MAY be supported (Section 7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-6` | Implementations MAY support password authentication. (Section 7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-6` | A client MAY drop the data from a particular cache when it is fully in sync with one or more other caches. (Section 8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-3-2` | There MAY be mechanisms for the router to assure itself of the authenticity of the cache and to authenticate itself to the cache. (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6810-5-1`](#rfc6810-5-1) Fields with unspecified content MUST be zero on transmission (Section 5) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code zeroes unspecified fields at versions 1 and 2 (writeResetQuery in internal/component/bgp/plugins/rpki/rtr_pdu.go), proven under RFC8210-5-1 by TestWriteResetQuery, TestWriteSerialQuery and TestWriteResetQueryZeroesReservedOverGarbage, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-5.1-1`](#rfc6810-5.1-1) Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the prefix. This MUST NOT be less than the Prefix Length element. (Section 5.1) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code refuses a Max Length below the Prefix Length at versions 1 and 2 (parsePrefixPDU in internal/component/bgp/plugins/rpki/rtr_pdu.go), proven under RFC8210-5.1-2 by TestParseIPv4Prefix and TestParseIPv4PrefixMaxLenLessThanPrefixLen, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-5.1-3`](#rfc6810-5.1-3) The value of such a field MUST be ignored on receipt. (Section 5.1) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code ignores reserved fields on receipt at versions 1 and 2 (parsePrefixPDU in internal/component/bgp/plugins/rpki/rtr_pdu.go, handlePDU), proven under RFC8210-5-1 and RFC8210-5.1-1 by TestReservedPrefixFieldsPreserveValidation, TestRTRReservedOctetsIgnoredOnReceipt and TestParsePrefixPDUFlagsHighBitsIgnored, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-5.6-1`](#rfc6810-5.6-1) The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (Section 5.6) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache server, but its sentence holds braces, which the annotation's own braces cannot carry in a quote, so no exclusion shape the gate accepts fits; ze fills only the router role: it parses received Prefix PDUs (parseIPv4Prefix, parseIPv6Prefix) and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only Reset Query, Serial Query and Error Report PDUs (writeResetQuery, writeSerialQuery, writeErrorReport), so it tells no router client what to hold, and the obligation has no producer and no test |
| [`RFC6810-5.5-1`](#rfc6810-5.5-1) When replying to a Reset Query, the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (Section 5.5) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache's payload PDUs, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and writes no payload PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go holds only writeResetQuery, writeSerialQuery and writeErrorReport), so the obligation has no producer and no test |
| [`RFC6810-5.8-1`](#rfc6810-5.8-1) The Session ID MUST be the same as that of the corresponding Cache Response that began the, possibly null, sequence of data PDUs. (Section 5.8) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the Session ID rule binds the cache's End of Data PDU, but its sentence names a field, not a role, and takes no exclusion shape the gate accepts; ze writes no End of Data or Cache Response PDU (internal/component/bgp/plugins/rpki/rtr_pdu.go holds only writeResetQuery, writeSerialQuery and writeErrorReport), so the obligation has no producer and no test |
| [`RFC6810-5.10-1`](#rfc6810-5.10-1) If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (Section 5.10) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; no exclusion shape fits this conditional sentence; ze's router does send Error Reports, always with the offending PDU encapsulated and never a generic one (readLoop in internal/component/bgp/plugins/rpki/rtr_session.go reports a malformed ASPA PDU or an unsupported version through writeErrorReport), and no test carries this id |
| [`RFC6810-5.10-2`](#rfc6810-5.10-2) An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (readLoop in internal/component/bgp/plugins/rpki/rtr_session.go writes no Error Report when the failing PDU is itself an Error Report), proven under RFC8210-5.11-1 in rtr_session_test.go, and no test carries this id |
| [`RFC6810-5.10-3`](#rfc6810-5.10-3) If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3269]). (Section 5.10) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; no exclusion shape fits this conditional sentence; writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go takes no text argument, so ze never sends error text, and no test carries this id |
| [`RFC6810-5.10-4`](#rfc6810-5.10-4) The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (Section 5.10) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (writeErrorReport in internal/component/bgp/plugins/rpki/rtr_pdu.go always writes a zero Length of Error Text), proven under RFC8210-5.11-4 in rtr_session_test.go, and no test carries this id |
| [`RFC6810-2-1`](#rfc6810-2-1) As a cache is receiving, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. (§2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds a cache's transmission, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router, sends no payload data to any party, and stages a received delta until End of Data (pendingVRPs in internal/component/bgp/plugins/rpki/rtr_session.go), so the obligation has no producer and no test |
| [`RFC6810-5.1-2`](#rfc6810-5.1-2) If, at any time, either the router or the cache finds the value of the session identifier is not the same as the other's, they MUST completely drop the session and the router MUST flush all data learned from that cache. (Section 5.1) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze does not detect a Session ID change -- handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go adopts the Cache Response Session ID unconditionally (s.sessionID = hdr.SessionID) with no comparison to the prior value and no cache flush |
| [`RFC6810-4-1`](#rfc6810-4-1) The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (Section 4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (cacheGroup in internal/component/bgp/plugins/rpki/rtr_session.go polls the caches in preference order after sortCachesByPreference in rpki_config.go, and loads from the most preferred one that answers), proven under RFC8210-4-1 in rfc8210_rtr_preference_test.go, and no test carries this id |
| [`RFC6810-6.1-1`](#rfc6810-6.1-1) To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query no less frequently than once an hour. (Section 6.1, Section 6.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code re-queries on the Refresh and Retry Intervals at versions 1 and 2 (cacheGroup.Run and pollDelay in internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.1-1 by TestRunPollsOnRefreshAfterSuccessRetryAfterFailure, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-6.2-1`](#rfc6810-6.2-1) The cache MUST rate limit Serial Notifies to no more frequently than one per minute. (Section 6.2) | no test | no test carries this requirement id; annotated {not-applicable}: "The cache MUST rate limit Serial Notifies to no more frequently than one per minute."; binds cache; ze fills only the router role: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go reads a received Serial Notify and internal/component/bgp/plugins/rpki/rtr_pdu.go writes only writeResetQuery, writeSerialQuery and writeErrorReport, none of them a Serial Notify |
| [`RFC6810-6.3-1`](#rfc6810-6.3-1) If there are no more preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (Section 6.3) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code answers a Cache Reset with a Reset Query and a whole new load at versions 1 and 2 (handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.3-1 by TestCacheResetTriggersResetQuery and TestRTRCacheResetReloadsTheWholeSet, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-6.4-1`](#rfc6810-6.4-1) If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (Section 6.4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code repeats Reset Queries after No Data Available at versions 1 and 2 (internal/component/bgp/plugins/rpki/rtr_session.go), proven under RFC8210-8.4-1 by TestNoDataAvailableKeepsResetQueryMode, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-3-1`](#rfc6810-3-1) A relying party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any authoritative cache(s) it uses. (Section 3) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (parseRPKIConfig in internal/component/bgp/plugins/rpki/rpki_config.go refuses a cache with neither TLS nor trusted-network true, and buildRTRTLSConfig in rtr_tls.go trusts only the named CA; the subject holds a list, so no exclusion shape fits, and the router is ze's role), proven under RFC8210-3-1 in rfc8210_rtr_tls_test.go and rfc8210_rtr_transport_test.go, and no test carries this id |
| [`RFC6810-4-2`](#rfc6810-4-2) As a cache server must evaluate certificates and ROAs (Route Origin Attestations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (Section 4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache server's clock, but its sentence opens with a clause and takes no exclusion shape the gate accepts; ze is the router and evaluates no certificate or ROA, so the obligation has no producer and no test |
| [`RFC6810-7-1`](#rfc6810-7-1) Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 12. (Section 7) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses an initial data PDU whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the attempt; after Cache Response establishes a response version, checkResponseIdentity in internal/component/bgp/plugins/rpki/rtr_response_identity.go rejects a different version with Error Report code 8 and termination, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code dials unprotected TCP on rpki-rtr port 323 at versions 1 and 2 (internal/component/bgp/plugins/rpki/rtr_session.go, rpki_config.go), proven under RFC8210-9-1 by TestParseRPKIConfigDefaults and TestRTRUnprotectedTCPFromConfig, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-7-2`](#rfc6810-7-2) If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (Section 7) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the routers as well as the cache, so it is ze's; ze cannot see the topology, so parseRPKIConfig in internal/component/bgp/plugins/rpki/rpki_config.go only refuses unprotected TCP unless the cache carries trusted-network true, nothing checks that the network is trusted and controlled, and no test carries this id |
| [`RFC6810-7-3`](#rfc6810-7-3) If available to the operator, caches and routers MUST use one of the following more protected protocols. (Section 7) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go starts TLS for a cache configured with it, and parseRPKIConfig in rpki_config.go refuses unprotected TCP unless the operator sets trusted-network true), proven under RFC8210-9-3 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.1-1`](#rfc6810-7.1-1) Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication (Section 7.1) | no test | no test carries this requirement id; annotated {not-applicable}: "Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication."; binds cache servers supporting SSH transport; ze fills only the router role and accepts no RTR connection on any transport: connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials out to each configured cache |
| [`RFC6810-7.1-2`](#rfc6810-7.1-2) User authentication MUST be supported (Section 7.1) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; user authentication binds both ends of an SSH transport, and ze implements no RTR SSH transport (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS); SSH is a MAY transport in Section 7 and the owner declined no SSH feature, so the obligation has no producer and no test |
| [`RFC6810-8-1`](#rfc6810-8-1) A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (Section 8) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (ze loads from one cache at a time and each cache switch replaces the published VRP and ASPA sets with that cache's own (cacheGroup in internal/component/bgp/plugins/rpki/rtr_session.go), so no update from one cache can touch another cache's data), proven under RFC8210-10-2 in rfc8210_rtr_preference_test.go, and no test carries this id |
| [`RFC6810-8-2`](#rfc6810-8-2) If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented, and version 0 is owed work (owner ruling 2026-10-30); the shared router code validates against the merged cache content at versions 1 and 2 (internal/component/bgp/plugins/rpki/roa_cache.go), proven under RFC8210-10-1 by TestValidationDoesNotDistinguishCacheSource and TestRTRValidationIgnoresWhichCacheSupplied, which run version 1 sessions and so prove RFC 8210 behaviour, not this row; no test carries this id |
| [`RFC6810-7.2-1`](#rfc6810-7.2-1) Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go presents the named PKI certificate through GetClientCertificate), proven under RFC8210-9.2-1 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.2-2`](#rfc6810-7.2-2) Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go refuses a router certificate with no iPAddress subjectAltName), proven under RFC8210-9.2-2 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.2-3`](#rfc6810-7.2-3) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the check binds the cache, but the sentence's first MUST binds the router's certificate, so no exclusion shape the gate accepts fits the row; ze is the TLS client and authenticates no connecting router (startTLS in internal/component/bgp/plugins/rpki/rtr_tls.go dials out), so the obligation has no producer and no test |
| [`RFC6810-7.2-4`](#rfc6810-7.2-4) Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid monkey-in-the-middle attacks. (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go sets ServerName to the DNS reference and RootCAs to the named CA, so Go crypto/tls verifies the cache certificate's dNSName), proven under RFC8210-9.2-4 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.2-5`](#rfc6810-7.2-5) Support for DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations that use TLS. (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (rtrServerName in internal/component/bgp/plugins/rpki/rtr_tls.go accepts only a DNS reference name, so the client side supports DNS-ID), proven under RFC8210-9.2-11 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.2-6`](#rfc6810-7.2-6) DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the obligation binds the cache's server certificate, but the sentence's first MUST binds certification authorities, so no exclusion shape the gate accepts fits the row; ze's router refuses a cache certificate without a matching dNSName (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go, proven under RFC8210-9.2-4), which enforces the rule from the client side but is not the issuer's obligation, and no test carries this id |
| [`RFC6810-7.2-7`](#rfc6810-7.2-7) rpki-rtr implementations that use TLS MUST NOT use CN-ID identifiers; a CN field may be present in the server certificate's subject name, but MUST NOT be used for authentication within the rules described in [RFC6125]. (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (buildRTRTLSConfig in internal/component/bgp/plugins/rpki/rtr_tls.go verifies the DNS reference through Go crypto/tls, which matches subjectAltName names only and never falls back to the Common Name), proven under RFC8210-9.2-5 and RFC8210-9.2-10 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.2-8`](#rfc6810-7.2-8) The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; the shared router code meets it at versions 1 and 2 (rtrServerName in internal/component/bgp/plugins/rpki/rtr_tls.go makes the cache's DNS name the TLS ServerName), proven under RFC8210-9.2-8 in rfc8210_rtr_tls_test.go, and no test carries this id |
| [`RFC6810-7.3-1`](#rfc6810-7.3-1) If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (Section 7.3) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 7 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| [`RFC6810-7.3-2`](#rfc6810-7.3-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.3) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-MD5 for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-MD5 key); TCP MD5 is a MAY transport in Section 7 and the owner declined no TCP-MD5 feature, so the obligation has no producer and no test |
| [`RFC6810-7.4-1`](#rfc6810-7.4-1) Implementations MUST support key lengths of at least 80 printable ASCII bytes. (Section 7.4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC6810-7.4-2`](#rfc6810-7.4-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC6810-7.4-3`](#rfc6810-7.4-3) MAC (Message Authentication Code) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (Section 7.4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |
| [`RFC6810-7.4-4`](#rfc6810-7.4-4) The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (Section 7.4) | {gap}, no test | ze speaks RTR version 1 or later only: handlePDU in internal/component/bgp/plugins/rpki/rtr_session.go refuses every PDU but a Serial Notify or an Error Report whose version is below rtrVersionMin = 1 (rtr_pdu.go), and readLoop answers it with Error Report code 4 and ends the session, so RTR version 0, which this document defines, is not implemented; ze implements no TCP-AO for RTR (connectAndSync in internal/component/bgp/plugins/rpki/rtr_session.go dials TCP and optionally starts TLS, and sets no TCP-AO key); TCP-AO is a SHOULD transport in Section 7 and the owner declined no TCP-AO feature, so the obligation has no producer and no test |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6810-5-1`](#rfc6810-5-1)

Fields with unspecified content MUST be zero on transmission (Section 5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5-1, so there is no linked test.

### [`RFC6810-5.1-1`](#rfc6810-5.1-1)

Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the prefix. This MUST NOT be less than the Prefix Length element. (Section 5.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Section 5.1 defines Protocol Version as 0 and states of Max Length: 'This MUST NOT be less than the Prefix Length element.' Current handlePDU still refuses initial version-0 data PDUs before parseIPv4Prefix or parseIPv6Prefix because rtrVersionMin remains 1 (rtr_pdu.go); its Unsupported Protocol Version branch refuses downgrade below rtrVersionMin=1, and syncOnce retries only versions 1 through 2. Adding pduRouterKey staging (a v1/v2 PDU unassigned in v0) does not enable version 0. No test carries this id; the row remains an owed version-0 implementation gap with no credit from v1/v2 parser behavior.

No test carries RFC6810-5.1-1, so there is no linked test.

### [`RFC6810-5.1-3`](#rfc6810-5.1-3)

The value of such a field MUST be ignored on receipt. (Section 5.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Section 5.1 requires: 'Zero: Fields shown as zero or reserved MUST be zero. The value of such a field MUST be ignored on receipt.' Its PDU version is 0. Current handlePDU still rejects initial version-0 data PDUs (rtrVersionMin=1), refuses downgrade to version 0 in its code-4 branch, and syncOnce never retries below version 1. Adding pduRouterKey handling in handlePDU does not lower the supported version floor or establish a version-0 session. No test carries this id; the row remains an owed version-0 implementation gap.

No test carries RFC6810-5.1-3, so there is no linked test.

### [`RFC6810-5.6-1`](#rfc6810-5.6-1)

The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (Section 5.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.6-1, so there is no linked test.

### [`RFC6810-5.5-1`](#rfc6810-5.5-1)

When replying to a Reset Query, the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (Section 5.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.5-1, so there is no linked test.

### [`RFC6810-5.8-1`](#rfc6810-5.8-1)

The Session ID MUST be the same as that of the corresponding Cache Response that began the, possibly null, sequence of data PDUs. (Section 5.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.8-1, so there is no linked test.

### [`RFC6810-5.10-1`](#rfc6810-5.10-1)

If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (Section 5.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.10-1, so there is no linked test.

### [`RFC6810-5.10-2`](#rfc6810-5.10-2)

An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.10-2, so there is no linked test.

### [`RFC6810-5.10-3`](#rfc6810-5.10-3)

If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3269]). (Section 5.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.10-3, so there is no linked test.

### [`RFC6810-5.10-4`](#rfc6810-5.10-4)

The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (Section 5.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.10-4, so there is no linked test.

### [`RFC6810-2-1`](#rfc6810-2-1)

As a cache is receiving, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-2-1, so there is no linked test.

### [`RFC6810-5.1-2`](#rfc6810-5.1-2)

If, at any time, either the router or the cache finds the value of the session identifier is not the same as the other's, they MUST completely drop the session and the router MUST flush all data learned from that cache. (Section 5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-5.1-2, so there is no linked test.

### [`RFC6810-4-1`](#rfc6810-4-1)

The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-4-1, so there is no linked test.

### [`RFC6810-6.1-1`](#rfc6810-6.1-1)

To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query no less frequently than once an hour. (Section 6.1, Section 6.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Sections 6.1 and 6.2 require the router to send either a Serial Query or a Reset Query no less frequently than once an hour using version-0 PDUs. New sessions still open at rtrVersionMax=2, handlePDU refuses any downgrade below rtrVersionMin=1, and syncOnce retries only versions 1 through 2. Adding pduRouterKey staging in handlePDU introduces no version-0 query or polling path. No test carries this id; this row remains an owed version-0 implementation gap.

No test carries RFC6810-6.1-1, so there is no linked test.

### [`RFC6810-6.2-1`](#rfc6810-6.2-1)

The cache MUST rate limit Serial Notifies to no more frequently than one per minute. (Section 6.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-6.2-1, so there is no linked test.

### [`RFC6810-6.3-1`](#rfc6810-6.3-1)

If there are no more preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (Section 6.3)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Section 6.3 requires that on Cache Reset, if there are no more preferred caches, the router MUST issue a Reset Query and get an entire new load from the cache in protocol version 0. Current handlePDU still rejects a version-0 Cache Reset before entering its pduCacheReset branch (rtrVersionMin=1), rejects version-0 downgrade in its code-4 branch, and syncOnce retries only versions 1 through 2. Adding pduRouterKey staging does not reach version-0 Cache Reset recovery. No test carries this id; the declared version-0 gap remains unimplemented.

No test carries RFC6810-6.3-1, so there is no linked test.

### [`RFC6810-6.4-1`](#rfc6810-6.4-1)

If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (Section 6.4)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Section 6.4 requires that after No Data Available, if no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache in protocol version 0. While handlePDU accepts a version-0 Error Report header before response establishment, it never adopts version 0 (rtrVersionMin=1) and syncOnce never retries below version 1, so subsequent Reset Queries stay v1/v2 and no version-0 load can succeed. Adding pduRouterKey staging does not change version-0 support. No test carries this id; the row remains unimplemented.

No test carries RFC6810-6.4-1, so there is no linked test.

### [`RFC6810-3-1`](#rfc6810-3-1)

A relying party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any authoritative cache(s) it uses. (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-3-1, so there is no linked test.

### [`RFC6810-4-2`](#rfc6810-4-2)

As a cache server must evaluate certificates and ROAs (Route Origin Attestations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-4-2, so there is no linked test.

### [`RFC6810-7-1`](#rfc6810-7-1)

Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 12. (Section 7)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent rejudgment 2026-11-22 after Router Key (type 9) handling was added to RTRSession.handlePDU. RFC 6810 Section 7 requires unprotected TCP transport on port rpki-rtr (323) carrying this document's version-0 binary PDUs. Ze implements unprotected TCP defaulting to port 323, but current handlePDU still rejects initial version-0 PDUs and refuses version-0 downgrade (rtrVersionMin=1), and syncOnce never establishes a version below 1. Adding pduRouterKey handling in handlePDU adds no version-0 protocol support. No test carries this id; the row remains unimplemented.

No test carries RFC6810-7-1, so there is no linked test.

### [`RFC6810-7-2`](#rfc6810-7-2)

If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (Section 7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7-2, so there is no linked test.

### [`RFC6810-7-3`](#rfc6810-7-3)

If available to the operator, caches and routers MUST use one of the following more protected protocols. (Section 7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7-3, so there is no linked test.

### [`RFC6810-7.1-1`](#rfc6810-7.1-1)

Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication (Section 7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.1-1, so there is no linked test.

### [`RFC6810-7.1-2`](#rfc6810-7.1-2)

User authentication MUST be supported (Section 7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.1-2, so there is no linked test.

### [`RFC6810-8-1`](#rfc6810-8-1)

A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (Section 8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-8-1, so there is no linked test.

### [`RFC6810-8-2`](#rfc6810-8-2)

If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-8-2, so there is no linked test.

### [`RFC6810-7.2-1`](#rfc6810-7.2-1)

Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-1, so there is no linked test.

### [`RFC6810-7.2-2`](#rfc6810-7.2-2)

Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-2, so there is no linked test.

### [`RFC6810-7.2-3`](#rfc6810-7.2-3)

when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-3, so there is no linked test.

### [`RFC6810-7.2-4`](#rfc6810-7.2-4)

Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid monkey-in-the-middle attacks. (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-4, so there is no linked test.

### [`RFC6810-7.2-5`](#rfc6810-7.2-5)

Support for DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations that use TLS. (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-5, so there is no linked test.

### [`RFC6810-7.2-6`](#rfc6810-7.2-6)

DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-6, so there is no linked test.

### [`RFC6810-7.2-7`](#rfc6810-7.2-7)

rpki-rtr implementations that use TLS MUST NOT use CN-ID identifiers; a CN field may be present in the server certificate's subject name, but MUST NOT be used for authentication within the rules described in [RFC6125]. (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-7, so there is no linked test.

### [`RFC6810-7.2-8`](#rfc6810-7.2-8)

The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.2-8, so there is no linked test.

### [`RFC6810-7.3-1`](#rfc6810-7.3-1)

If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (Section 7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.3-1, so there is no linked test.

### [`RFC6810-7.3-2`](#rfc6810-7.3-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.3-2, so there is no linked test.

### [`RFC6810-7.4-1`](#rfc6810-7.4-1)

Implementations MUST support key lengths of at least 80 printable ASCII bytes. (Section 7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.4-1, so there is no linked test.

### [`RFC6810-7.4-2`](#rfc6810-7.4-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.4-2, so there is no linked test.

### [`RFC6810-7.4-3`](#rfc6810-7.4-3)

MAC (Message Authentication Code) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (Section 7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.4-3, so there is no linked test.

### [`RFC6810-7.4-4`](#rfc6810-7.4-4)

The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (Section 7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC6810-7.4-4, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6810.txt |
| Source fingerprint | 6dddd37386e09797 |
| Record | rfc/extraction/rfc6810.json |
| Mapped sentences | 39 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 2 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `5.1` | not stated | 4 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `5.5` | not stated | 1 | reviewed | not stated |
| `5.6` | not stated | 1 | reviewed | not stated |
| `5.7` | not stated | 0 | reviewed | not stated |
| `5.8` | not stated | 1 | reviewed | not stated |
| `5.9` | not stated | 0 | reviewed | not stated |
| `5.10` | not stated | 4 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 2 | reviewed | not stated |
| `6.2` | not stated | 2 | reviewed | not stated |
| `6.3` | not stated | 1 | reviewed | not stated |
| `6.4` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 3 | reviewed | not stated |
| `7.1` | not stated | 2 | reviewed | not stated |
| `7.2` | not stated | 7 | reviewed | not stated |
| `7.3` | not stated | 2 | reviewed | not stated |
| `7.4` | not stated | 4 | reviewed | not stated |
| `8` | not stated | 2 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 1 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `14.1` | not stated | 0 | reviewed | not stated |
| `14.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Section 5.1 field glossary restates the Section 5 transmission rule already mapped at site 5:1: fields with unspecified content MUST be zero on transmission. | Zero: Fields shown as zero or reserved MUST be zero. |
| `6.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the once-an-hour query cadence of the preceding sentence, mapped at site 6.1:1, as a polling interval ceiling. | As the cache MAY not keep updates for little more than one hour, the router MUST have a polling interval of no greater than once an hour. |
| `6.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 repeats the Section 6.1 sentence word for word; RFC6810-6.1-1 cites both sections. | To limit the length of time a cache must keep old withdraws, a router MUST send either a Serial Query or a Reset Query no less frequently than once an hour. |
| `11:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Security Considerations restate the Section 7 transport rule mapped at site 7:2, that with unprotected TCP the cache and router are on the same trusted and controlled network; the SHOULD half of the same sentence is row RFC6810-11-3. | Protocols that provide integrity and authenticity SHOULD be used, and if they cannot, i.e., TCP is used as the transport, the router and cache MUST be on the same trusted, controlled network. |

## Superseded

RFC 6810 is obsoleted by RFC 8210.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC6810-5-1`](#rfc6810-5-1) Fields with unspecified content MUST be zero on transmission (Section 5) | restated | RFC8210-5-1 | RFC 8210 Section 5 keeps the sentence and names the field, that reserved fields marked zero in the PDU diagrams MUST be zero on transmission |
| [`RFC6810-5.1-1`](#rfc6810-5.1-1) Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the prefix. This MUST NOT be less than the Prefix Length element. (Section 5.1) | restated | RFC8210-5.1-2 | RFC 8210 Section 5.1 keeps the rule for the IPv4 and IPv6 Prefix PDUs, that the Max Length must not be less than the Prefix Length |
| [`RFC6810-5.1-3`](#rfc6810-5.1-3) The value of such a field MUST be ignored on receipt. (Section 5.1) | restated | RFC8210-5-1 | RFC 8210 §5 keeps "Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt" and §5.1 restates it for the Flags field (RFC8210-5.1-1) |
| [`RFC6810-5.6-1`](#rfc6810-5.6-1) The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (Section 5.6) | restated | RFC8210-5.6-1 | RFC 8210 Section 5.6 keeps the one-and-only-one rule for the IPvX Prefix PDU, and Section 5.10 states the same rule for the new Router Key PDU at RFC8210-5.10-1 |
| [`RFC6810-5.5-1`](#rfc6810-5.5-1) When replying to a Reset Query, the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (Section 5.5) | restated | RFC8210-5.5-1 | RFC 8210 Section 5.5 keeps the rule that payload PDUs replying to a Reset Query carry withdraw/announce value 1 |
| [`RFC6810-5.8-1`](#rfc6810-5.8-1) The Session ID MUST be the same as that of the corresponding Cache Response that began the, possibly null, sequence of data PDUs. (Section 5.8) | restated | RFC8210-5.8-1 | RFC 8210 Section 5.8 keeps the Session ID rule and adds the Protocol Version to it, because version negotiation is new in version 1 |
| [`RFC6810-5.10-1`](#rfc6810-5.10-1) If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (Section 5.10) | restated | RFC8210-5.11-2 | the Error Report PDU moved from Section 5.10 to Section 5.11 when the Router Key PDU took 5.10, and the rule is kept, that a generic error leaves the Erroneous PDU field empty and the Length of Encapsulated PDU zero |
| [`RFC6810-5.10-2`](#rfc6810-5.10-2) An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | restated | RFC8210-5.11-1 | the Error Report PDU moved to Section 5.11 and keeps the rule that no Error Report PDU is sent for an Error Report PDU |
| [`RFC6810-5.10-3`](#rfc6810-5.10-3) If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3269]). (Section 5.10) | restated | RFC8210-5.11-3 | the Error Report PDU moved to Section 5.11 and keeps the rule that error text, when present, is UTF-8 encoded |
| [`RFC6810-5.10-4`](#rfc6810-5.10-4) The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (Section 5.10) | restated | RFC8210-5.11-4 | the Error Report PDU moved to Section 5.11 and keeps the rule that the Length of Error Text is zero when no diagnostic text is present |
| [`RFC6810-2-1`](#rfc6810-2-1) As a cache is receiving, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. (§2) | unextracted | §2 | RFC 8210 Section 2 keeps the Serial Number glossary entry word for word, that while a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-5.1-2`](#rfc6810-5.1-2) If, at any time, either the router or the cache finds the value of the session identifier is not the same as the other's, they MUST completely drop the session and the router MUST flush all data learned from that cache. (Section 5.1) | restated | RFC8210-5.1-4 | RFC 8210 Section 5.1 keeps the flush and splits the sentence in two, stating the immediate termination with an Error Report PDU of code 0 separately at RFC8210-5.1-3 |
| [`RFC6810-4-1`](#rfc6810-4-1) The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (Section 4) | restated | RFC8210-4-1 | RFC 8210 Section 4 keeps the rule that the router chooses the most preferred cache by configuration |
| [`RFC6810-6.1-1`](#rfc6810-6.1-1) To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query no less frequently than once an hour. (Section 6.1, Section 6.2) | restated | RFC8210-8.1-1 | the protocol sequences moved from Section 6 to Section 8, and RFC 8210 replaces the fixed once-an-hour floor with the cache-signalled Refresh Interval of Section 6. The periodic query itself stays a MUST |
| [`RFC6810-6.2-1`](#rfc6810-6.2-1) The cache MUST rate limit Serial Notifies to no more frequently than one per minute. (Section 6.2) | restated | RFC8210-8.2-1 | the protocol sequences moved to Section 8, and Section 8.2 keeps the one-per-minute rate limit on Serial Notifies |
| [`RFC6810-6.3-1`](#rfc6810-6.3-1) If there are no more preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (Section 6.3) | restated | RFC8210-8.3-1 | Section 8.3 keeps the rule that a router with no more-preferred cache available issues a Reset Query and takes an entire new load after a Cache Reset |
| [`RFC6810-6.4-1`](#rfc6810-6.4-1) If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (Section 6.4) | restated | RFC8210-8.4-1 | Section 8.4 keeps the rule that a router with no other cache available issues periodic Reset Queries when the cache cannot supply an update |
| [`RFC6810-3-1`](#rfc6810-3-1) A relying party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any authoritative cache(s) it uses. (Section 3) | unextracted | §3 | RFC 8210 Section 3 keeps the sentence word for word, that a Relying Party MUST have a trust relationship with, and a trusted transport channel to, any cache it uses. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-4-2`](#rfc6810-4-2) As a cache server must evaluate certificates and ROAs (Route Origin Attestations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (Section 4) | restated | RFC8210-4-2 | RFC 8210 Section 4 keeps the tolerance, that servers' clocks are correct to approximately an hour |
| [`RFC6810-7-1`](#rfc6810-7-1) Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 12. (Section 7) | restated | RFC8210-9-1 | the transport section moved from Section 7 to Section 9, which keeps the obligation to implement unprotected transport over TCP on the rpki-rtr port 323 |
| [`RFC6810-7-2`](#rfc6810-7-2) If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (Section 7) | restated | RFC8210-9-2 | Section 9 keeps the rule that cache and routers are on the same trusted and controlled network when unprotected TCP is the transport |
| [`RFC6810-7-3`](#rfc6810-7-3) If available to the operator, caches and routers MUST use one of the following more protected protocols. (Section 7) | restated | RFC8210-9-3 | Section 9 keeps the obligation to use one of the more protected protocols when it is available to the operator |
| [`RFC6810-7.1-1`](#rfc6810-7.1-1) Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication (Section 7.1) | restated | RFC8210-9.1-1 | RFC 8210 Section 9.1 keeps the obligation for RSA and REMOVES DSA. Where RFC 6810 required a cache server supporting SSH to accept RSA and DSA authentication, RFC 8210 requires RSA alone, and its only other keyword in that sentence is the SHOULD for ECDSA at RFC8210-9.1-3 which RFC 6810 also carried |
| [`RFC6810-7.1-2`](#rfc6810-7.1-2) User authentication MUST be supported (Section 7.1) | restated | RFC8210-9.1-2 | Section 9.1 keeps the obligation to support SSH user authentication |
| [`RFC6810-8-1`](#rfc6810-8-1) A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (Section 8) | restated | RFC8210-10-2 | the router-cache setup moved from Section 8 to Section 10, which keeps the obligation to mark held data as to source because later updates must affect the correct data |
| [`RFC6810-8-2`](#rfc6810-8-2) If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8) | restated | RFC8210-10-1 | Section 10 keeps the prohibition on distinguishing between data sources when performing validation of BGP announcements |
| [`RFC6810-7.2-1`](#rfc6810-7.2-1) Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (Section 7.2) | restated | RFC8210-9.2-1 | the TLS transport moved to Section 9.2, which keeps the obligation on a client router to present client-side certificates |
| [`RFC6810-7.2-2`](#rfc6810-7.2-2) Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (Section 7.2) | restated | RFC8210-9.2-2 | Section 9.2 keeps the obligation that a client router certificate include a subjectAltName extension carrying one or more iPAddress identities |
| [`RFC6810-7.2-3`](#rfc6810-7.2-3) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (Section 7.2) | restated | RFC8210-9.2-3 | Section 9.2 keeps the obligation on the cache to check the IP address of the TLS connection against those iPAddress identities |
| [`RFC6810-7.2-4`](#rfc6810-7.2-4) Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid monkey-in-the-middle attacks. (Section 7.2) | restated | RFC8210-9.2-4 | Section 9.2 keeps the obligation on routers to verify the cache's TLS server certificate using subjectAltName dNSName identities as described in RFC 6125 |
| [`RFC6810-7.2-5`](#rfc6810-7.2-5) Support for DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations that use TLS. (Section 7.2) | unextracted | §9.2 | RFC 8210 Section 9.2 keeps the sentence word for word, that support for the DNS-ID identifier type is REQUIRED in rpki-rtr server and client implementations which use TLS. rfc/short/rfc8210.md declares rows for the Certification Authority half and for the certificate half, at RFC8210-9.2-9 and RFC8210-9.2-6, and none for the implementation-support half |
| [`RFC6810-7.2-6`](#rfc6810-7.2-6) DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | restated | RFC8210-9.2-6 | Section 9.2 keeps the obligation that the DNS-ID identifier type be present in rpki-rtr server certificates |
| [`RFC6810-7.2-7`](#rfc6810-7.2-7) rpki-rtr implementations that use TLS MUST NOT use CN-ID identifiers; a CN field may be present in the server certificate's subject name, but MUST NOT be used for authentication within the rules described in [RFC6125]. (Section 7.2) | restated | RFC8210-9.2-5 | Section 9.2 keeps the prohibition on CN-ID identifiers, and states its second half, that a CN field present in the subject name is not used for authentication, at RFC8210-9.2-10 |
| [`RFC6810-7.2-8`](#rfc6810-7.2-8) The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | restated | RFC8210-9.2-8 | Section 9.2 keeps the obligation on the client router to set its reference identifier to the DNS name of the rpki-rtr cache |
| [`RFC6810-7.3-1`](#rfc6810-7.3-1) If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (Section 7.3) | restated | RFC8210-9.3-1 | the TCP MD5 transport moved to Section 9.3, which keeps the key length of at least 80 printable ASCII bytes |
| [`RFC6810-7.3-2`](#rfc6810-7.3-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.3) | restated | RFC8210-9.3-2 | Section 9.3 keeps the obligation to support hexadecimal sequences of at least 32 characters, that is 128 bits |
| [`RFC6810-7.4-1`](#rfc6810-7.4-1) Implementations MUST support key lengths of at least 80 printable ASCII bytes. (Section 7.4) | restated | RFC8210-9.4-1 | the TCP-AO transport moved to Section 9.4, which keeps the key length of at least 80 printable ASCII bytes |
| [`RFC6810-7.4-2`](#rfc6810-7.4-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (Section 7.4) | restated | RFC8210-9.4-2 | Section 9.4 keeps the obligation to support hexadecimal sequences of at least 32 characters, that is 128 bits |
| [`RFC6810-7.4-3`](#rfc6810-7.4-3) MAC (Message Authentication Code) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (Section 7.4) | restated | RFC8210-9.4-1 | Section 9.4 keeps the MAC length of at least 96 bits per Section 5.1 of RFC 5925, and rfc/short/rfc8210.md carries it in the same row as the key length |
| [`RFC6810-7.4-4`](#rfc6810-7.4-4) The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (Section 7.4) | restated | RFC8210-9.4-3 | Section 9.4 keeps the obligation to support the cryptographic algorithms and associated parameters described in RFC 5926 |
| [`RFC6810-6.2-2`](#rfc6810-6.2-2) The cache server SHOULD send a notify PDU with its current Serial Number when the cache's serial changes (Section 6.2) | restated | RFC8210-8.2-2 | Section 8.2 keeps the SHOULD to send a Notify PDU carrying the current Serial Number when the cache's serial changes |
| [`RFC6810-5.6-2`](#rfc6810-5.6-2) Should the router client receive an IPvX PDU with a {Prefix, Len, Max-Len, ASN} identical to one it already has active, it SHOULD raise a Duplicate Announcement Received error. (Section 5.6) | restated | RFC8210-5.6-2 | RFC 8210 Section 5.6 keeps the SHOULD to raise a Duplicate Announcement Received error, and Section 5.10 extends it to the new Router Key PDU |
| [`RFC6810-7-4`](#rfc6810-7-4) Caches and routers SHOULD use TCP-AO, SSHv2, TCP MD5, or IPsec transport (Section 7) | restated | RFC8210-9-5 | Section 9 keeps the SHOULD for TCP-AO alone and demotes the rest of the list to MAY, at RFC8210-9-6, adding TLS to it. RFC 6810 wrote one SHOULD over TCP-AO, SSHv2, TCP MD5 and IPsec |
| [`RFC6810-7-5`](#rfc6810-7-5) To reduce exposure to dropped but non-terminated sessions, both caches and routers SHOULD enable keep-alives when available in the chosen transport protocol. (Section 7) | restated | RFC8210-9-4 | Section 9 keeps the SHOULD to enable keep-alives when they are available in the chosen transport protocol |
| [`RFC6810-6.3-2`](#rfc6810-6.3-2) Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more preferred caches in its cache list. (Section 6.3) | restated | RFC8210-8.3-2 | Section 8.3 keeps the SHOULD to attempt a more-preferred cache on a Cache Reset |
| [`RFC6810-6.4-2`](#rfc6810-6.4-2) When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. (Section 6.4) | restated | RFC8210-8.4-2 | Section 8.4 keeps the SHOULD to attempt other caches in preference order on No Data Available |
| [`RFC6810-8-3`](#rfc6810-8-3) If a client loses connectivity to a cache it is using, or otherwise decides to switch to a new cache, it SHOULD retain the data from the previous cache until it has a full set of data from one or more other caches. (Section 8) | restated | RFC8210-10-4 | Section 10 keeps the SHOULD to retain the previous cache's data until a full set of data is held from one or more other caches |
| [`RFC6810-8-4`](#rfc6810-8-4) A client SHOULD delete the data from a cache when it has been unable to refresh from that cache for a configurable timer value. The default for that value is twice the polling period for that cache. (Section 8) | restated | RFC8210-6-2 | RFC 8210 replaces the locally configured timer, whose default was twice the polling period, with the Expire Interval the cache sends in the End Of Data PDU, and RAISES the rule from a SHOULD to delete to a MUST NOT retain. Section 10 now defers to Section 6 for what to do when the client cannot refresh from a cache |
| [`RFC6810-11-1`](#rfc6810-11-1) It also SHOULD be topologically close so that a minimum of validated routing data are needed to bootstrap a router's access to a cache. (Section 11) | unextracted | §13 | the Security Considerations moved from Section 11 to Section 13, which keeps both SHOULDs word for word, that the cache really SHOULD be as close as possible in the sense of controlled and protected transport, and SHOULD be topologically close. rfc/short/rfc8210.md declares no row for either |
| [`RFC6810-5.10-5`](#rfc6810-5.10-5) If an erroneous Error Report PDU is received, the session SHOULD be dropped. (Section 5.10) | restated | RFC8210-5.11-5 | the Error Report PDU moved to Section 5.11 and keeps the SHOULD to drop the session on an erroneous Error Report PDU |
| [`RFC6810-7.1-3`](#rfc6810-7.1-3) Cache servers supporting SSH transport MUST accept RSA and Digital Signature Algorithm (DSA) authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication. (Section 7.1) | restated | RFC8210-9.1-3 | Section 9.1 keeps the SHOULD to accept ECDSA authentication |
| [`RFC6810-7.1-4`](#rfc6810-7.1-4) Client routers SHOULD verify the public key of the cache to avoid monkey-in-the-middle attacks. (Section 7.1) | restated | RFC8210-9.1-4 | Section 9.1 keeps the SHOULD for the client router to verify the public key of the cache against a monkey-in-the-middle attack |
| [`RFC6810-7.2-9`](#rfc6810-7.2-9) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities and SHOULD reject the connection if none of the iPAddress identities match the connection. (Section 7.2) | unextracted | §9.2 | RFC 8210 Section 9.2 keeps the clause word for word, that the cache SHOULD reject the connection if none of the iPAddress identities match the connection. rfc/short/rfc8210.md declares a row for the MUST check that precedes it, RFC8210-9.2-3, and none for this SHOULD |
| [`RFC6810-7.2-10`](#rfc6810-7.2-10) DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*" (Section 7.2) | restated | RFC8210-9.2-7 | Section 9.2 keeps the SHOULD NOT on the wildcard character in a DNS name in an rpki-rtr server certificate |
| [`RFC6810-7-6`](#rfc6810-7-6) Operators SHOULD use procedural means, e.g., access control lists (ACLs), to reduce the exposure to authentication issues. (Section 7) | unextracted | §9 | RFC 8210 Section 9 keeps the sentence word for word, that operators SHOULD use procedural means, for example access control lists, to reduce the exposure to authentication issues. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-7.3-3`](#rfc6810-7.3-3) Key rollover with TCP MD5 is problematic. Cache servers SHOULD support [RFC4808]. (Section 7.3) | restated | RFC8210-9.3-3 | Section 9.3 keeps the SHOULD for a cache server to support RFC 4808, because key rollover with TCP MD5 stays problematic |
| [`RFC6810-8-5`](#rfc6810-8-5) The client SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a different cache or established a new connection to the previous cache. (Section 8) | restated | RFC8210-10-3 | Section 10 keeps the SHOULD to maintain at least one set of data regardless of a cache change or a new connection |
| [`RFC6810-11-2`](#rfc6810-11-2) The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa (Section 11) | restated | RFC8210-11-1 | the Security Considerations moved to Section 13, which keeps the sentence word for word, that the identity of the cache server SHOULD be verified and authenticated by the router client and vice versa before any data are exchanged. The rfc/short/rfc8210.md row cites §11, which is that summary's own error and not a change in the document |
| [`RFC6810-11-3`](#rfc6810-11-3) Protocols that provide integrity and authenticity SHOULD be used (Section 11) | unextracted | §13 | the Security Considerations moved to Section 13, which keeps the sentence word for word, that protocols which provide integrity and authenticity SHOULD be used, and that if they cannot be, the router and cache MUST be on the same trusted controlled network. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-10-1`](#rfc6810-10-1) Errors that are considered fatal SHOULD cause the session to be dropped. (Section 10) | restated | RFC8210-12-1 | the error codes moved from Section 10 to Section 12, and RFC 8210 RAISES the rule, from errors considered fatal SHOULD cause the session to be dropped to MUST cause the session to be dropped |
| [`RFC6810-6.1-2`](#rfc6810-6.1-2) Alternatively, if the router has significant unexpired data from a broken session with the same cache, it MAY start with a Serial Query containing the Session ID from the previous session to ensure the Serial Numbers are commensurate. (Section 6.1) | restated | RFC8210-8.1-2 | RFC 8210 Section 8.1 turns the pair of permissions into one obligation, that on a new transport connection the router MUST send either a Reset Query or a Serial Query, and keeps the unexpired-data condition as indicative prose, adding that the router MUST fall back to a Reset Query in all other cases |
| [`RFC6810-5.10-6`](#rfc6810-5.10-6) If the error is associated with a PDU of excessive length, i.e., too long to be any legal PDU other than another Error Report, or a possibly corrupt length, the Erroneous PDU field MAY be truncated. (Section 5.10) | restated | RFC8210-5.11-6 | the Error Report PDU moved to Section 5.11 and keeps the permission to truncate the Erroneous PDU field for an excessively long PDU |
| [`RFC6810-5-2`](#rfc6810-5-2) Fields with unspecified content MUST be zero on transmission and MAY be ignored on receipt. (Section 5) | restated | RFC8210-5-1 | RFC 8210 Section 5 states both halves in one sentence and RAISES this one, from MAY be ignored on receipt to MUST be ignored on receipt |
| [`RFC6810-7-7`](#rfc6810-7-7) Caches and routers MAY use SSHv2 transport [RFC4252] using a the normal SSH port. For an example, see Section 7.1. Caches and routers MAY use TCP MD5 transport [RFC2385] using the rpki-rtr port. Note that TCP MD5 has been obsoleted by TCP-AO [RFC5925]. Caches and routers MAY use IPsec transport [RFC4301] using the rpki- rtr port. Caches and routers MAY use TLS transport [RFC5246] using a port, rpki-rtr-tls (324); see Section 12. (Section 7) | restated | RFC8210-9-6 | Section 9 keeps the permission and adds TLS on port rpki-rtr-tls 324 to the list |
| [`RFC6810-7.1-5`](#rfc6810-7.1-5) host authentication MAY be supported (Section 7.1) | unextracted | §9.1 | RFC 8210 Section 9.1 keeps the sentence word for word, that host authentication MAY be supported. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-7.1-6`](#rfc6810-7.1-6) Implementations MAY support password authentication. (Section 7.1) | unextracted | §9.1 | RFC 8210 Section 9.1 keeps the sentence word for word, that implementations MAY support password authentication. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-8-6`](#rfc6810-8-6) A client MAY drop the data from a particular cache when it is fully in sync with one or more other caches. (Section 8) | unextracted | §10 | RFC 8210 Section 10 keeps the sentence word for word, that a client MAY drop the data from a particular cache when it is fully in sync with one or more other caches. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-3-2`](#rfc6810-3-2) There MAY be mechanisms for the router to assure itself of the authenticity of the cache and to authenticate itself to the cache. (Section 3) | unextracted | §3 | RFC 8210 Section 3 keeps the sentence word for word, that there MAY be mechanisms for the router to assure itself of the authenticity of the cache and to authenticate itself to the cache. rfc/short/rfc8210.md declares no row for it |
