# RFC 5905 - Network Time Protocol Version 4: Protocol and Algorithms Specification

NTP client (SNTP subset). Every requirement this repository extracted from RFC 5905, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 8 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 8 | of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 8 gated MUSTs: they add up to 8. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public row | NTP client (SNTP subset) |
| Enrolment | Enrolled |
| Requirements | 8 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5905.md` |
| Requirement shard | `rfc/requirements/rfc5905.md` |
| RFC text | `rfc/full/rfc5905.txt` |

## Enrolment

Enrolled: NTPv4 protocol and algorithms. Ze is an SNTP-style client (Section 14): it queries its configured and DHCP-learned servers, picks one sample and steps or slews the clock, without the Section 9 and later mitigation algorithms. Every MUST-level row is classified: the full-suite, alternative-algorithm and association-reset rows (RFC5905-2-1, RFC5905-2-2, RFC5905-11.2.3-1) and the kiss-of-death handling rows are gaps owned by plan/post-release/features/spec-ntp-client-kiss-of-death.md. No owner ruling names the full NTPv4 suite unwanted (ai/META-DECISIONS.md, Goal and scope), so the Section 14 subset Ze ships is recorded as a gap, not a declined feature.

## What the public ledger says

**Area:** NTP client (SNTP subset)

**Proof:** 0 of 8 MUST proven

**Evidence:** -

**Unmet:** RFC5905-11.2.3-1, RFC5905-2-1, RFC5905-2-2, RFC5905-7.4-1, RFC5905-7.4-2, RFC5905-7.4-3, RFC5905-7.4-4, RFC5905-7.4-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (8):** [`RFC5905-2-1`](#rfc5905-2-1), [`RFC5905-2-2`](#rfc5905-2-2), [`RFC5905-7.4-1`](#rfc5905-7.4-1), [`RFC5905-7.4-2`](#rfc5905-7.4-2), [`RFC5905-7.4-3`](#rfc5905-7.4-3), [`RFC5905-7.4-4`](#rfc5905-7.4-4), [`RFC5905-7.4-5`](#rfc5905-7.4-5), [`RFC5905-11.2.3-1`](#rfc5905-11.2.3-1)

## Figures

- **MUST bar**: Ze correctness gap 8
- **Overview bar**: Ze correctness gap 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 8 |
| MUST rows kept (the MUST bar) | 8 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 8 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 0 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

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

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5905-2-1`](#rfc5905-2-1)

All servers and clients who are fully NTPv4-compliant MUST implement the entire suite of algorithms described in this document. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-2-1, so no unit is bound to it.

### [`RFC5905-2-2`](#rfc5905-2-2)

Alternative algorithms MAY be used, but their output MUST be identical to the algorithms described in this specification. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-2-2, so no unit is bound to it.

### [`RFC5905-7.4-1`](#rfc5905-7.4-1)

Recipients of kiss codes MUST inspect them and, in the following cases, take these actions: (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-7.4-1, so no unit is bound to it.

### [`RFC5905-7.4-2`](#rfc5905-7.4-2)

For kiss codes DENY and RSTR, the client MUST demobilize any associations to that server and stop sending packets to that server; (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-7.4-2, so no unit is bound to it.

### [`RFC5905-7.4-3`](#rfc5905-7.4-3)

For kiss code RATE, the client MUST immediately reduce its polling interval to that server and continue to reduce it each time it receives a RATE kiss code. (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-7.4-3, so no unit is bound to it.

### [`RFC5905-7.4-4`](#rfc5905-7.4-4)

Kiss codes beginning with the ASCII character "X" are for unregistered experimentation and development and MUST be ignored if not recognized. (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-7.4-4, so no unit is bound to it.

### [`RFC5905-7.4-5`](#rfc5905-7.4-5)

The Receive Timestamp and the Transmit Timestamp (set by the server) are undefined when in a KoD packet and MUST NOT be relied upon to have valid values and MUST be discarded. (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-7.4-5, so no unit is bound to it.

### [`RFC5905-11.2.3-1`](#rfc5905-11.2.3-1)

In this case, the clock is stepped to the correct offset, but since this means all peer data have been invalidated, all associations MUST be reset and the client begins as at initial start. (§11.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5905-11.2.3-1, so no unit is bound to it.

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 5 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |
| `11.2.1` | not stated | 0 | walked | not stated |
| `11.2.2` | not stated | 0 | walked | not stated |
| `11.2.3` | not stated | 1 | walked | not stated |
| `11.3` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `13.1` | not stated | 0 | walked | not stated |
| `13.2` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |
| `16` | not stated | 0 | walked | not stated |
| `17` | not stated | 0 | walked | not stated |
| `18` | not stated | 0 | walked | not stated |
| `18.1` | not stated | 0 | walked | not stated |
| `18.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.1.1` | not stated | 0 | walked | not stated |
| `A.1.2` | not stated | 0 | walked | not stated |
| `A.1.3` | not stated | 0 | walked | not stated |
| `A.1.4` | not stated | 0 | walked | not stated |
| `A.1.5` | not stated | 0 | walked | not stated |
| `A.1.6` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |
| `A.4` | not stated | 0 | walked | not stated |
| `A.5` | not stated | 0 | walked | not stated |
| `A.5.1` | not stated | 0 | walked | not stated |
| `A.5.1.1` | not stated | 0 | walked | not stated |
| `A.5.2` | not stated | 0 | walked | not stated |
| `A.5.3` | not stated | 0 | walked | not stated |
| `A.5.4` | not stated | 0 | walked | not stated |
| `A.5.5` | not stated | 0 | walked | not stated |
| `A.5.5.1` | not stated | 0 | walked | not stated |
| `A.5.5.2` | not stated | 0 | walked | not stated |
| `A.5.5.3` | not stated | 0 | walked | not stated |
| `A.5.5.4` | not stated | 0 | walked | not stated |
| `A.5.5.5` | not stated | 0 | walked | not stated |
| `A.5.5.6` | not stated | 0 | walked | not stated |
| `A.5.5.7` | not stated | 0 | walked | not stated |
| `A.5.6` | not stated | 0 | walked | not stated |
| `A.5.6.1` | not stated | 0 | walked | not stated |
| `A.5.7` | not stated | 0 | walked | not stated |
| `A.5.7.1` | not stated | 0 | walked | not stated |
| `A.5.7.2` | not stated | 0 | walked | not stated |
| `A.5.7.3` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5905 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5905, so its obligations are stated where they were written.
