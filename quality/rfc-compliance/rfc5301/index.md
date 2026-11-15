# RFC 5301 - Dynamic Hostname Exchange Mechanism for IS-IS

IS-IS dynamic hostname. Every requirement listed for RFC 5301, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 7 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 7 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 7 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 7 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 7 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 0 | 0 of 22 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 7 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 7 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 7 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 7 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 7 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 7 | of 15 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 7 mandatory requirements: they add up to 7. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IS-IS dynamic hostname |
| Automatic checks | Included |
| Requirements | 15 |
| Mandatory requirements (MUST level) | 7 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tests linked to requirements | 22 |
| Recorded audit verdicts | 1 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5301.md` |
| Requirement table | `rfc/requirements/rfc5301.md` |
| RFC text | `rfc/full/rfc5301.txt` |

## Automatic checks

Included in automatic checks: Dynamic Hostname Exchange Mechanism for IS-IS (TLV 137): seven MUST-level requirements read from the indicative prose of section 3, all met and test-bound with positive+negative tags. 3-4 (TLV type 137), 3-5 (the length octet is the total length of the value), 3-6 (a value of 1 to 255 bytes) and 3-8 (the string is not null-terminated) are proven over the real origination path in internal/plugins/isis/lsdb (encode_test.go builds the node's own LSP and reads the framed octets out of Entry.Raw), and 3-4 also carries interop evidence: test/interop/scenarios/isis-p2p-frr renders Ze's name in FRR's IS-IS database, which FRR can only do by decoding type 137. 3-7 (the value is encoded in 7-bit ASCII) and 3-9 (the content is a domain name per RFC 2181) are enforced at the config boundary by ISISHostnameValidator (internal/component/config/validators.go), which refuses any octet outside 0x20..0x7e and any break of the RFC 2181 section 11 label lengths; the value is REFUSED rather than sanitised at emit, so the name an operator configures and the name a peer reads are the same string. 3-10 (the IDNA duty of a user-interface that permits Unicode) is met under Reading A, recorded in rfc/short/rfc5301.md: the configuring interface refuses Unicode, so the sentence's antecedent is false and no ToASCII conversion is owed. The refusal is observable in both polarities rather than annotated, and no row carries {gap} or {not-applicable}. test/isis/isis-hostname-ascii.ci proves the boundary through `ze config validate`. Enrolled 2026-08-10.

## Published support status

**Area:** IS-IS dynamic hostname

**Proof:** 0 of 7 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **7** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC5301-3-4`](#rfc5301-3-4), [`RFC5301-3-5`](#rfc5301-3-5), [`RFC5301-3-6`](#rfc5301-3-6), [`RFC5301-3-7`](#rfc5301-3-7), [`RFC5301-3-8`](#rfc5301-3-8), [`RFC5301-3-9`](#rfc5301-3-9), [`RFC5301-3-10`](#rfc5301-3-10)

## Figures

- **MUST bar**: Proof pending 7
- **Overview bar**: Proof pending 7, Not assessed 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 7 |
| MUST rows kept (the MUST bar) | 7 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 7 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 8 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 8 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5301.json](../../../data/rfc/rfc5301.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5301-3-4` | The Dynamic hostname TLV is defined here as TLV type 137 (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L110). **negative:** `unit/verify` [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L166). **positive:** `interop/nightly` [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1214) |
| `RFC5301-3-5` | Length - total length of the value field (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L114). **negative:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L125) |
| `RFC5301-3-6` | Value - a string of 1 to 255 bytes (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L118). **negative:** `unit/verify` [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L162). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L100). **positive:** `interop/nightly` [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1218) |
| `RFC5301-3-7` | The Value field is encoded in 7-bit ASCII (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L126). **positive:** `unit/verify` [`TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`](https://github.com/ze-software/ze/blob/main/internal/component/config/cli/rfc5301_cmd_validate_startup_agreement_test.go#L43). **negative:** `unit/verify` [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L130). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L55). **positive:** `functional/verify` [`isis-hostname-startup-refused.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-startup-refused.ci#L16) |
| `RFC5301-3-8` | The string is not null- terminated. (Section 3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L121). **negative:** `unit/verify` [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L80) |
| `RFC5301-3-9` | The content of this value is a domain name, see [RFC2181] (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L173). **negative:** `unit/verify` [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L178). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L73) |
| `RFC5301-3-10` | If a user-interface for configuring or displaying this field permits Unicode characters, that user-interface is responsible for applying the ToASCII and/or ToUnicode algorithm as described in [RFC3490] to achieve the correct format for transmission or display (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameUnicodeRefusedNotConverted`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L237). **negative:** `unit/verify` [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L72). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L56) |
| `RFC5301-3-1` | The use of FQDN or a subset of it is strongly recommended (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-3` | If this TLV is present in a pseudonode LSP, then it SHOULD NOT be interpreted as the DNS hostname of the router (Section 3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-1` | If a system receives a mapping for a name or system ID that is different from the mapping in the local cache, an implementation SHOULD replace the existing mapping with the latest information (Section 4) | SHOULD | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-2` | This TLV may be present in any fragment of a non-pseudonode LSP (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-11` | The Dynamic hostname TLV is optional (Section 3) | OPTIONAL | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-2` | When originating an LSP, a router may decide to include this TLV in its LSP (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-3` | Upon receipt of an LSP with the Dynamic hostname TLV, a router may decide to ignore this TLV, or to install the symbolic name and system ID in its hostname mapping table for the IS-IS network (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-4` | A router may also optionally insert this TLV in its pseudonode LSP for the association of a symbolic name to a local LAN (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5301 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC5301-3-4`](#rfc5301-3-4)

The Dynamic hostname TLV is defined here as TLV type 137 (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L166) | unit/verify | unproven |
| positive | [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1214) | interop/nightly | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L110) | unit/verify | unproven |

### [`RFC5301-3-5`](#rfc5301-3-5)

Length - total length of the value field (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L125) | unit/verify | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L114) | unit/verify | unproven |

### [`RFC5301-3-6`](#rfc5301-3-6)

Value - a string of 1 to 255 bytes (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L162) | unit/verify | unproven |
| positive | [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1218) | interop/nightly | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L118) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L100) | functional/verify | unproven |

### [`RFC5301-3-7`](#rfc5301-3-7)

The Value field is encoded in 7-bit ASCII (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L130) | unit/verify | unproven |
| positive | [`TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`](https://github.com/ze-software/ze/blob/main/internal/component/config/cli/rfc5301_cmd_validate_startup_agreement_test.go#L43) | unit/verify | unproven |
| positive | [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L126) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L55) | functional/verify | unproven |
| positive | [`isis-hostname-startup-refused.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-startup-refused.ci#L16) | functional/verify | unproven |

### [`RFC5301-3-8`](#rfc5301-3-8)

The string is not null- terminated. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) a TLV 137 value carrying a NUL terminator; (b) TestISISHostnameTLVFraming: bytes.IndexByte(value,0)>=0 errors, len(value)!=len(name) errors, and the [137][len+1][name,0] run errors; TestISISHostnameTLVIsPrintableASCII rejects any octet outside 0x20..0x7e and requires exactly one [137][len][value] run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L80) | unit/verify | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L121) | unit/verify | unproven |

### [`RFC5301-3-9`](#rfc5301-3-9)

The content of this value is a domain name, see [RFC2181] (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L178) | unit/verify | unproven |
| positive | [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L173) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L73) | functional/verify | unproven |

### [`RFC5301-3-10`](#rfc5301-3-10)

If a user-interface for configuring or displaying this field permits Unicode characters, that user-interface is responsible for applying the ToASCII and/or ToUnicode algorithm as described in [RFC3490] to achieve the correct format for transmission or display (Section 3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5301_encode_test.go#L72) | unit/verify | unproven |
| positive | [`TestISISHostnameUnicodeRefusedNotConverted`](https://github.com/ze-software/ze/blob/main/internal/component/config/rfc5301_validators_isis_test.go#L237) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L56) | functional/verify | unproven |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 |
| Signed off | 2026-07-30 |
| Register | prose |
| Source | rfc/full/rfc5301.txt |
| Source fingerprint | d1ee0177891d12fe |
| Record | rfc/extraction/rfc5301.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract states what the document defines and directs nobody. |
| `1` | Introduction | 1 | reviewed | Introduction. Motivates a name-to-systemID mapping and describes why static tables do not scale. Its one prose site is the burden static configuration imposes on an operator, excluded below. |
| `1.1` | not stated | 0 | reviewed | Specification of Requirements: the RFC 2119 key-words paragraph. Tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. It is also the whole of the document's capitalised-keyword content. |
| `2` | Possible Solutions | 0 | reviewed | Possible Solutions. Compares static configuration and DNS against in-protocol advertisement, then states that this document defines a new TLV. Rationale only. |
| `3` | not stated | 0 | reviewed | Dynamic Hostname TLV: the definitional section, and the source of every gated requirement this summary declares. Written wholly in indicative prose, so the site scan finds nothing here and each requirement is listed as unsourced against this section rather than mapped to a site. |
| `4` | Implementation | 0 | reviewed | Implementation. Four sentences, all permissive or advisory: a router MAY include the TLV, MAY ignore it on receipt, MAY insert it in a pseudonode LSP, and SHOULD replace a differing cached mapping. Captured as RFC5301-4-1 through RFC5301-4-4. |
| `5` | Security Considerations | 0 | reviewed | Security Considerations. Warns that a compromised router can inject false mapping information and that the mapping must be treated with suspicion during an incident. An operational caution, not a directive on the protocol machinery. |
| `6` | Acknowledgments, crediting RFC 2763 and Henk Smit | 0 | skipped (acknowledgements) | Acknowledgments, crediting RFC 2763 and Henk Smit. |
| `7` | IANA Considerations | 1 | reviewed | IANA Considerations. Records that TLV 137 was already allocated by RFC 2763 and that IANA need do nothing. Walked because the prose scan attributes one site here; it is excluded below. |
| `8` | not stated | 1 | reviewed | Informative References, followed by the authors' addresses and the copyright statement the prose scan attributes its last site to. Walked for that site rather than skipped. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the maintenance burden of the STATIC mapping tables this document exists to replace. The 'must' binds a network operator keeping a hand-written table in step, not an IS-IS implementation. | These tables need to contain the names and system IDs of all routers in the network, and must be modified each time an addition, deletion, or change occurs. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA instruction, and a negative one: it states that no registry action is needed because RFC 2763 already allocated TLV 137. It binds no speaker. | As such, no new actions are required on the part of IANA. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The standard IETF copyright boilerplate. It addresses parties holding patents, not an IS-IS implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 5301, so its obligations are stated where they were written.
