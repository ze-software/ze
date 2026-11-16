# RFC 5905 - Network Time Protocol Version 4: Protocol and Algorithms Specification

NTP client (SNTP subset). Every requirement listed for RFC 5905, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 8 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 8 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 8 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 8 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 8 | of 8 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 8 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 8 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 8 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 8 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 8 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 8 mandatory requirements: they add up to 8. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | NTP client (SNTP subset) |
| Automatic checks | Included |
| Requirements | 8 |
| Mandatory requirements (MUST level) | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5905.md` |
| Requirement table | `rfc/requirements/rfc5905.md` |
| RFC text | `rfc/full/rfc5905.txt` |

## Automatic checks

Included in automatic checks: NTPv4 protocol and algorithms. Ze is an SNTP-style client (Section 14): it queries its configured and DHCP-learned servers, picks one sample and steps or slews the clock, without the Section 9 and later mitigation algorithms. Every MUST-level row is classified: the full-suite, alternative-algorithm and association-reset rows (RFC5905-2-1, RFC5905-2-2, RFC5905-11.2.3-1) and the kiss-of-death handling rows are gaps owned by plan/post-release/features/spec-ntp-client-kiss-of-death.md. No owner ruling names the full NTPv4 suite unwanted (ai/META-DECISIONS.md, Goal and scope), so the Section 14 subset Ze ships is recorded as a gap, not a declined feature.

## Published support status

**Area:** NTP client (SNTP subset)

**Test verification:** 0 of 8 MUST proven

**Evidence:** -

**Unmet:** RFC5905-11.2.3-1, RFC5905-2-1, RFC5905-2-2, RFC5905-7.4-1, RFC5905-7.4-2, RFC5905-7.4-3, RFC5905-7.4-4, RFC5905-7.4-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 8 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **8** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (8):** [`RFC5905-2-1`](#rfc5905-2-1), [`RFC5905-2-2`](#rfc5905-2-2), [`RFC5905-7.4-1`](#rfc5905-7.4-1), [`RFC5905-7.4-2`](#rfc5905-7.4-2), [`RFC5905-7.4-3`](#rfc5905-7.4-3), [`RFC5905-7.4-4`](#rfc5905-7.4-4), [`RFC5905-7.4-5`](#rfc5905-7.4-5), [`RFC5905-11.2.3-1`](#rfc5905-11.2.3-1)

## Figures

- **MUST bar**: Not built 8
- **Overview bar**: Not built 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 8 |
| MUST rows kept (the MUST bar) | 8 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 8 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5905.json](../../../data/rfc/rfc5905.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5905-2-1` | All servers and clients who are fully NTPv4-compliant MUST implement the entire suite of algorithms described in this document. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements the Section 14 SNTP subset, not the entire suite: internal/plugins/ntp/ntp.go::doSync queries each server once per round and keeps its latest sample, with no clock filter, selection, cluster, combine or clock discipline algorithm; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC5905-2-2` | Alternative algorithms MAY be used, but their output MUST be identical to the algorithms described in this specification. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/plugins/ntp/ntp.go::doSync picks one server with internal/plugins/ntp/state.go::selectBestServer over each server's latest sample, an alternative to the Section 10 and 11 algorithms whose output is not identical to theirs; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| `RFC5905-7.4-1` | Recipients of kiss codes MUST inspect them and, in the following cases, take these actions: (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** doSync in internal/plugins/ntp/ntp.go discards a stratum-0 response through resp.Validate and logs the error; the kiss code is never read |
| `RFC5905-7.4-2` | For kiss codes DENY and RSTR, the client MUST demobilize any associations to that server and stop sending packets to that server; (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a server answering DENY or RSTR stays in the server list and is queried again every round; internal/plugins/ntp/ntp.go doSync |
| `RFC5905-7.4-3` | For kiss code RATE, the client MUST immediately reduce its polling interval to that server and continue to reduce it each time it receives a RATE kiss code. (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sync interval is the fixed configured IntervalSec and no kiss code changes it; internal/plugins/ntp/ntp.go run |
| `RFC5905-7.4-4` | Kiss codes beginning with the ASCII character "X" are for unregistered experimentation and development and MUST be ignored if not recognized. (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no kiss code is inspected (RFC5905-7.4-1), so an X code is not distinguished from DENY, RSTR or RATE; every KoD packet is discarded alike by doSync in internal/plugins/ntp/ntp.go and no test pins the X case |
| `RFC5905-7.4-5` | The Receive Timestamp and the Transmit Timestamp (set by the server) are undefined when in a KoD packet and MUST NOT be relied upon to have valid values and MUST be discarded. (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** doSync in internal/plugins/ntp/ntp.go reads resp.Time, derived from the transmit timestamp, for its absurd-year check before resp.Validate rejects the KoD packet, so a KoD packet can be logged and counted as an absurd timestamp rather than as a kiss |
| `RFC5905-11.2.3-1` | In this case, the clock is stepped to the correct offset, but since this means all peer data have been invalidated, all associations MUST be reset and the client begins as at initial start. (§11.2.3) | MUST | 11.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** after internal/plugins/ntp/ntp.go::doSync steps the clock it keeps every server's state in w.peers (reach register, offset, delay, stratum) and does not reset the associations to their initial state; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5905-2-1`](#rfc5905-2-1) All servers and clients who are fully NTPv4-compliant MUST implement the entire suite of algorithms described in this document. (§2) | {gap}, no test | Ze implements the Section 14 SNTP subset, not the entire suite: internal/plugins/ntp/ntp.go::doSync queries each server once per round and keeps its latest sample, with no clock filter, selection, cluster, combine or clock discipline algorithm; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC5905-2-2`](#rfc5905-2-2) Alternative algorithms MAY be used, but their output MUST be identical to the algorithms described in this specification. (§2) | {gap}, no test | internal/plugins/ntp/ntp.go::doSync picks one server with internal/plugins/ntp/state.go::selectBestServer over each server's latest sample, an alternative to the Section 10 and 11 algorithms whose output is not identical to theirs; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |
| [`RFC5905-7.4-1`](#rfc5905-7.4-1) Recipients of kiss codes MUST inspect them and, in the following cases, take these actions: (§7.4) | {gap}, no test | doSync in internal/plugins/ntp/ntp.go discards a stratum-0 response through resp.Validate and logs the error; the kiss code is never read |
| [`RFC5905-7.4-2`](#rfc5905-7.4-2) For kiss codes DENY and RSTR, the client MUST demobilize any associations to that server and stop sending packets to that server; (§7.4) | {gap}, no test | a server answering DENY or RSTR stays in the server list and is queried again every round; internal/plugins/ntp/ntp.go doSync |
| [`RFC5905-7.4-3`](#rfc5905-7.4-3) For kiss code RATE, the client MUST immediately reduce its polling interval to that server and continue to reduce it each time it receives a RATE kiss code. (§7.4) | {gap}, no test | the sync interval is the fixed configured IntervalSec and no kiss code changes it; internal/plugins/ntp/ntp.go run |
| [`RFC5905-7.4-4`](#rfc5905-7.4-4) Kiss codes beginning with the ASCII character "X" are for unregistered experimentation and development and MUST be ignored if not recognized. (§7.4) | {gap}, no test | no kiss code is inspected (RFC5905-7.4-1), so an X code is not distinguished from DENY, RSTR or RATE; every KoD packet is discarded alike by doSync in internal/plugins/ntp/ntp.go and no test pins the X case |
| [`RFC5905-7.4-5`](#rfc5905-7.4-5) The Receive Timestamp and the Transmit Timestamp (set by the server) are undefined when in a KoD packet and MUST NOT be relied upon to have valid values and MUST be discarded. (§7.4) | {gap}, no test | doSync in internal/plugins/ntp/ntp.go reads resp.Time, derived from the transmit timestamp, for its absurd-year check before resp.Validate rejects the KoD packet, so a KoD packet can be logged and counted as an absurd timestamp rather than as a kiss |
| [`RFC5905-11.2.3-1`](#rfc5905-11.2.3-1) In this case, the clock is stepped to the correct offset, but since this means all peer data have been invalidated, all associations MUST be reset and the client begins as at initial start. (§11.2.3) | {gap}, no test | after internal/plugins/ntp/ntp.go::doSync steps the clock it keeps every server's state in w.peers (reach register, offset, delay, stratum) and does not reset the associations to their initial state; no owner ruling names this feature unwanted (ai/META-DECISIONS.md, Goal and scope) |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5905-2-1`](#rfc5905-2-1)

All servers and clients who are fully NTPv4-compliant MUST implement the entire suite of algorithms described in this document. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-2-1, so there is no linked test.

### [`RFC5905-2-2`](#rfc5905-2-2)

Alternative algorithms MAY be used, but their output MUST be identical to the algorithms described in this specification. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-2-2, so there is no linked test.

### [`RFC5905-7.4-1`](#rfc5905-7.4-1)

Recipients of kiss codes MUST inspect them and, in the following cases, take these actions: (§7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-7.4-1, so there is no linked test.

### [`RFC5905-7.4-2`](#rfc5905-7.4-2)

For kiss codes DENY and RSTR, the client MUST demobilize any associations to that server and stop sending packets to that server; (§7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-7.4-2, so there is no linked test.

### [`RFC5905-7.4-3`](#rfc5905-7.4-3)

For kiss code RATE, the client MUST immediately reduce its polling interval to that server and continue to reduce it each time it receives a RATE kiss code. (§7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-7.4-3, so there is no linked test.

### [`RFC5905-7.4-4`](#rfc5905-7.4-4)

Kiss codes beginning with the ASCII character "X" are for unregistered experimentation and development and MUST be ignored if not recognized. (§7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-7.4-4, so there is no linked test.

### [`RFC5905-7.4-5`](#rfc5905-7.4-5)

The Receive Timestamp and the Transmit Timestamp (set by the server) are undefined when in a KoD packet and MUST NOT be relied upon to have valid values and MUST be discarded. (§7.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-7.4-5, so there is no linked test.

### [`RFC5905-11.2.3-1`](#rfc5905-11.2.3-1)

In this case, the clock is stepped to the correct offset, but since this means all peer data have been invalidated, all associations MUST be reset and the client begins as at initial start. (§11.2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5905-11.2.3-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5905.txt |
| Source fingerprint | 05144a4d067fe99a |
| Record | rfc/extraction/rfc5905.json |
| Mapped sentences | 8 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `7.3` | not stated | 0 | reviewed | not stated |
| `7.4` | not stated | 5 | reviewed | not stated |
| `7.5` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `11.1` | not stated | 0 | reviewed | not stated |
| `11.2` | not stated | 0 | reviewed | not stated |
| `11.2.1` | not stated | 0 | reviewed | not stated |
| `11.2.2` | not stated | 0 | reviewed | not stated |
| `11.2.3` | not stated | 1 | reviewed | not stated |
| `11.3` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `13.1` | not stated | 0 | reviewed | not stated |
| `13.2` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `15` | not stated | 0 | reviewed | not stated |
| `16` | not stated | 0 | reviewed | not stated |
| `17` | not stated | 0 | reviewed | not stated |
| `18` | not stated | 0 | reviewed | not stated |
| `18.1` | not stated | 0 | reviewed | not stated |
| `18.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |
| `A.1` | not stated | 0 | reviewed | not stated |
| `A.1.1` | not stated | 0 | reviewed | not stated |
| `A.1.2` | not stated | 0 | reviewed | not stated |
| `A.1.3` | not stated | 0 | reviewed | not stated |
| `A.1.4` | not stated | 0 | reviewed | not stated |
| `A.1.5` | not stated | 0 | reviewed | not stated |
| `A.1.6` | not stated | 0 | reviewed | not stated |
| `A.2` | not stated | 0 | reviewed | not stated |
| `A.3` | not stated | 0 | reviewed | not stated |
| `A.4` | not stated | 0 | reviewed | not stated |
| `A.5` | not stated | 0 | reviewed | not stated |
| `A.5.1` | not stated | 0 | reviewed | not stated |
| `A.5.1.1` | not stated | 0 | reviewed | not stated |
| `A.5.2` | not stated | 0 | reviewed | not stated |
| `A.5.3` | not stated | 0 | reviewed | not stated |
| `A.5.4` | not stated | 0 | reviewed | not stated |
| `A.5.5` | not stated | 0 | reviewed | not stated |
| `A.5.5.1` | not stated | 0 | reviewed | not stated |
| `A.5.5.2` | not stated | 0 | reviewed | not stated |
| `A.5.5.3` | not stated | 0 | reviewed | not stated |
| `A.5.5.4` | not stated | 0 | reviewed | not stated |
| `A.5.5.5` | not stated | 0 | reviewed | not stated |
| `A.5.5.6` | not stated | 0 | reviewed | not stated |
| `A.5.5.7` | not stated | 0 | reviewed | not stated |
| `A.5.6` | not stated | 0 | reviewed | not stated |
| `A.5.6.1` | not stated | 0 | reviewed | not stated |
| `A.5.7` | not stated | 0 | reviewed | not stated |
| `A.5.7.1` | not stated | 0 | reviewed | not stated |
| `A.5.7.2` | not stated | 0 | reviewed | not stated |
| `A.5.7.3` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 5905 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 5905, so its obligations are stated where they were written.
