# RFC 2782 - A DNS RR for specifying the location of services (DNS SRV)

DNS SRV records. Every requirement listed for RFC 2782, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 7 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 7 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 7 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 7 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 7 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 7 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 2 | of 7 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 7 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 7 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 7 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 7 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 7 mandatory requirements: they add up to 7. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | DNS SRV records |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 7 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc2782.md` |
| Requirement table | `rfc/requirements/rfc2782.md` |
| RFC text | `rfc/full/rfc2782.txt` |

## Automatic checks

Included in automatic checks: DNS SRV resource records: seven MUST-level requirements. ze is the name server for its geodns zones and serves SRV records as configured (internal/plugins/geodns: config.go parseSRV, server.go recordRR emitting dns.SRV); it holds no SRV-cognizant client, since no production code issues a TypeSRV query. 2 not-applicable: Applicability-1 and Applicability-2 bind the protocol specification that adopts SRV, and ze writes none. 5 gaps: the client MUSTs (Priority-1, Notes-2, Notes-3) bind a client ze does not implement and no owner decision declines; Target-1 binds the publishing zone, which for a geodns zone is ze, and parseSRV does not check that an in-zone target has an address record; Target-2 holds by construction in-zone, since geodns serves no CNAME, but carries no test.

## Published support status

**Area:** DNS SRV records

**Proof:** 0 of 7 MUST proven

**Evidence:** -

**Unmet:** RFC2782-Notes-2, RFC2782-Notes-3, RFC2782-Priority-1, RFC2782-Target-1, RFC2782-Target-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 7 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **7** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (7):** [`RFC2782-Applicability-1`](#rfc2782-applicability-1), [`RFC2782-Applicability-2`](#rfc2782-applicability-2), [`RFC2782-Priority-1`](#rfc2782-priority-1), [`RFC2782-Target-1`](#rfc2782-target-1), [`RFC2782-Target-2`](#rfc2782-target-2), [`RFC2782-Notes-2`](#rfc2782-notes-2), [`RFC2782-Notes-3`](#rfc2782-notes-3)

## Figures

- **MUST bar**: Not built 5
- **Overview bar**: Not owed 2, Not assessed 7, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 7 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 2 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 2 |
| SHOULD / MAY rows | 7 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 7 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2782.json](../../../data/rfc/rfc2782.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2782-Applicability-1` | Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. (§Applicability) | MUST | Applicability | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below."; binds such specification; the obligation falls on the protocol specification that adopts SRV, and ze writes no specification: geodns serves the operator's _Service._Proto owner names as configured (parseSRV in internal/plugins/geodns/config.go) |
| `RFC2782-Applicability-2` | Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. It also MUST include security considerations. (§Applicability) | MUST | Applicability | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** "Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below."; binds such specification; the security considerations the next sentence asks for fall on the same protocol specification that adopts SRV, and ze writes no specification: geodns serves operator-configured SRV records (parseSRV in internal/plugins/geodns/config.go) |
| `RFC2782-Priority-1` | A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority) | MUST | Priority | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client contacts a target by priority, and the row stays a gap with no test |
| `RFC2782-Target-1` | There MUST be one or more address records for this name (§Target) | MUST | Target | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation falls on the zone that publishes the SRV record, and for a geodns zone that is ze: parseSRV (internal/plugins/geodns/config.go) checks only that the target is a valid name, so an operator can publish an SRV record whose in-zone target has no address record and nothing refuses it; a target outside geodns's zones is that zone's to answer, and no test is tagged to this row |
| `RFC2782-Target-2` | There MUST be one or more address records for this name, the name MUST NOT be an alias (in the sense of RFC 1034 or RFC 2181). (§Target) | MUST NOT | Target | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns serves A, AAAA and SRV records from its configuration (the recordKind values in internal/plugins/geodns/record.go) and builds SOA and NS records for its zones (buildSOA and the NS answers in internal/plugins/geodns/server.go), and it serves no CNAME, so an in-zone SRV target is never an alias and the rule holds by construction there, but parseSRV accepts any out-of-zone target name and no test is tagged to this row |
| `RFC2782-Notes-2` | A client MUST parse all of the RR's in the reply. (§Notes) | MUST | Notes | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client parses an SRV reply to connect, and the row stays a gap with no test |
| `RFC2782-Notes-3` | If the Additional Data section doesn't contain address records for all the SRV RR's and the client may want to connect to the target host(s) involved, the client MUST look up the address record(s). (§Notes) | MUST | Notes | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client looks up a target's address records before connecting, and the row stays a gap with no test |
| `RFC2782-Applicability-3` | Service SRV records SHOULD NOT be used in the absence of such specification. (§Applicability) | SHOULD NOT | Applicability | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Priority-2` | target hosts with the same priority SHOULD be tried in an order defined by the weight field. (§Priority) | SHOULD | Priority | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-1` | Larger weights SHOULD be given a proportionately higher probability of being selected (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-2` | Domain administrators SHOULD use Weight 0 when there isn't any server selection to do, to make the RR easier to read for humans (less noisy). (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-3` | The following algorithm SHOULD be used to order the SRV RRs of the same priority (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Usage-1` | A SRV-cognizant client SHOULD use this procedure to locate a list of servers and connect to the preferred one (§Usage) | SHOULD | Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Notes-1` | Port numbers SHOULD NOT be used in place of the symbolic service or protocol names (§Notes) | SHOULD NOT | Notes | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2782-Applicability-1`](#rfc2782-applicability-1) Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. (§Applicability) | no test | no test carries this requirement id; annotated {not-applicable}: "Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below."; binds such specification; the obligation falls on the protocol specification that adopts SRV, and ze writes no specification: geodns serves the operator's _Service._Proto owner names as configured (parseSRV in internal/plugins/geodns/config.go) |
| [`RFC2782-Applicability-2`](#rfc2782-applicability-2) Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. It also MUST include security considerations. (§Applicability) | no test | no test carries this requirement id; annotated {not-applicable}: "Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below."; binds such specification; the security considerations the next sentence asks for fall on the same protocol specification that adopts SRV, and ze writes no specification: geodns serves operator-configured SRV records (parseSRV in internal/plugins/geodns/config.go) |
| [`RFC2782-Priority-1`](#rfc2782-priority-1) A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority) | {gap}, no test | ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client contacts a target by priority, and the row stays a gap with no test |
| [`RFC2782-Target-1`](#rfc2782-target-1) There MUST be one or more address records for this name (§Target) | {gap}, no test | the obligation falls on the zone that publishes the SRV record, and for a geodns zone that is ze: parseSRV (internal/plugins/geodns/config.go) checks only that the target is a valid name, so an operator can publish an SRV record whose in-zone target has no address record and nothing refuses it; a target outside geodns's zones is that zone's to answer, and no test is tagged to this row |
| [`RFC2782-Target-2`](#rfc2782-target-2) There MUST be one or more address records for this name, the name MUST NOT be an alias (in the sense of RFC 1034 or RFC 2181). (§Target) | {gap}, no test | geodns serves A, AAAA and SRV records from its configuration (the recordKind values in internal/plugins/geodns/record.go) and builds SOA and NS records for its zones (buildSOA and the NS answers in internal/plugins/geodns/server.go), and it serves no CNAME, so an in-zone SRV target is never an alias and the rule holds by construction there, but parseSRV accepts any out-of-zone target name and no test is tagged to this row |
| [`RFC2782-Notes-2`](#rfc2782-notes-2) A client MUST parse all of the RR's in the reply. (§Notes) | {gap}, no test | ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client parses an SRV reply to connect, and the row stays a gap with no test |
| [`RFC2782-Notes-3`](#rfc2782-notes-3) If the Additional Data section doesn't contain address records for all the SRV RR's and the client may want to connect to the target host(s) involved, the client MUST look up the address record(s). (§Notes) | {gap}, no test | ze holds no SRV-cognizant client: no production code issues a TypeSRV query; geodns only answers SRV (internal/plugins/geodns/server.go), and the stub resolver flattens an SRV answer to a target:port string (internal/component/resolve/dns/resolver.go) but no caller asks it for SRV; no owner decision declines SRV use, so no ze client looks up a target's address records before connecting, and the row stays a gap with no test |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC2782-Applicability-1`](#rfc2782-applicability-1)

Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. (§Applicability)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Applicability-1, so there is no linked test.

### [`RFC2782-Applicability-2`](#rfc2782-applicability-2)

Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. It also MUST include security considerations. (§Applicability)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Applicability-2, so there is no linked test.

### [`RFC2782-Priority-1`](#rfc2782-priority-1)

A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Priority-1, so there is no linked test.

### [`RFC2782-Target-1`](#rfc2782-target-1)

There MUST be one or more address records for this name (§Target)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Target-1, so there is no linked test.

### [`RFC2782-Target-2`](#rfc2782-target-2)

There MUST be one or more address records for this name, the name MUST NOT be an alias (in the sense of RFC 1034 or RFC 2181). (§Target)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Target-2, so there is no linked test.

### [`RFC2782-Notes-2`](#rfc2782-notes-2)

A client MUST parse all of the RR's in the reply. (§Notes)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Notes-2, so there is no linked test.

### [`RFC2782-Notes-3`](#rfc2782-notes-3)

If the Additional Data section doesn't contain address records for all the SRV RR's and the client may want to connect to the target host(s) involved, the client MUST look up the address record(s). (§Notes)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2782-Notes-3, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2782.txt |
| Source fingerprint | 4fd63a129dc9b1d6 |
| Record | rfc/extraction/rfc2782.json |
| Mapped sentences | 6 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 12 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose describing the state of the art before SRV; the lowercase 'must' states what a client is forced to do today, and RFC 2782's Definitions names only the capitalised spellings as BCP 14 keywords. | Currently, one must either know the exact address of a server to contact it, or broadcast a question. |
| `front:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence says the Additional Data behaviour is urged 'but not required', so it states no obligation on a server or a client. | Implementors are urged, but not required, to return the address record(s) in the Additional Data section. |
| `front:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Lowercase 'must' in the Fallback deployment notes, a bullet list of advice to domain administrators alongside 'it probably shouldn't be listed' and 'domain administrators are strongly advised'; RFC 2782's Definitions binds only the capitalised spellings to BCP 14. | - Hosts that are referenced by backup address records must use the port number specified in Assigned Numbers for the service. |
| `front:8` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The truncated-response rules the sentence invokes are RFC 2181's, which the sentence cites; the obligation lives in that document. | - If a truncated response comes back from an SRV query, the rules described in [RFC 2181] shall apply. |
| `front:11` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IANA Considerations statement that no further IANA action is needed; it records an absence rather than an obligation. | No other IANA services are required by this document. |
| `front:12` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Full Copyright Statement boilerplate governing reproduction of the document itself, not protocol behaviour. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2782, so its obligations are stated where they were written.
