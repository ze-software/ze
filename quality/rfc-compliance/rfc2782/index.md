# RFC 2782 - A DNS RR for specifying the location of services (DNS SRV)

Partial. Every requirement this repository extracted from RFC 2782, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 7 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 7 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 7 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 7 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 28.6% | 2 of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 7 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 7 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 7 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 71.4% | 5 of 7 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 7 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 7 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2782.md` |
| Requirement shard | `rfc/requirements/rfc2782.md` |
| RFC text | `rfc/full/rfc2782.txt` |

## Enrolment

Enrolled: DNS SRV resource records: seven MUST-level requirements. ze is the name server for its geodns zones and serves SRV records as configured (internal/plugins/geodns: config.go parseSRV, server.go recordRR emitting dns.SRV); it holds no SRV-cognizant client, since no production code issues a TypeSRV query. 2 not-applicable: Applicability-1 and Applicability-2 bind the protocol specification that adopts SRV, and ze writes none. 5 gaps: the client MUSTs (Priority-1, Notes-2, Notes-3) bind a client ze does not implement and no owner decision declines; Target-1 binds the publishing zone, which for a geodns zone is ze, and parseSRV does not check that an in-zone target has an address record; Target-2 holds by construction in-zone, since geodns serves no CNAME, but carries no test.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

GeoDNS, as the name server for its zones, serves the SRV records (priority, weight, port, target) an operator configures on a host whose type is SRV, beside its A and AAAA hosts (internal/plugins/geodns: config.go parseSRV, server.go recordRR emitting dns.SRV). No test is tagged to any RFC 2782 requirement.

**What the ledger says remains**

Five MUST gaps, each annotated in [`rfc/short/rfc2782.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2782.md).

- **Ze holds no SRV-cognizant client:** no production code issues a TypeSRV query, so the client selection and Additional-section lookup MUSTs (RFC2782-Priority-1, Notes-2, Notes-3) are unmet. parseSRV checks only that a target is a valid name, so an in-zone target with no address record is published (RFC2782-Target-1), and the no-alias rule holds in-zone by construction, since geodns serves no CNAME, but carries no test (RFC2782-Target-2). The two Applicability MUSTs bind the protocol specifications that adopt SRV and are not applicable.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (7):** [`RFC2782-Applicability-1`](#rfc2782-applicability-1), [`RFC2782-Applicability-2`](#rfc2782-applicability-2), [`RFC2782-Priority-1`](#rfc2782-priority-1), [`RFC2782-Target-1`](#rfc2782-target-1), [`RFC2782-Target-2`](#rfc2782-target-2), [`RFC2782-Notes-2`](#rfc2782-notes-2), [`RFC2782-Notes-3`](#rfc2782-notes-3)

## Attainable ceiling

Mandatory ceiling: 5 of 7 (71.4%); excluded: Not applicable (classification can be revisited): 2 of 7 (28.6%). Optional ceiling: 7 of 7 (100.0%). Lowest mandatory ceiling: rfc2782 5 of 7 (71.4%). Lowest optional ceiling: rfc2782 7 of 7 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

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

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2782-Applicability-1`](#rfc2782-applicability-1)

Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. (§Applicability)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Applicability-1, so no unit is bound to it.

### [`RFC2782-Applicability-2`](#rfc2782-applicability-2)

Such specification MUST define the symbolic name to be used in the Service field of the SRV record as described below. It also MUST include security considerations. (§Applicability)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Applicability-2, so no unit is bound to it.

### [`RFC2782-Priority-1`](#rfc2782-priority-1)

A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Priority-1, so no unit is bound to it.

### [`RFC2782-Target-1`](#rfc2782-target-1)

There MUST be one or more address records for this name (§Target)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Target-1, so no unit is bound to it.

### [`RFC2782-Target-2`](#rfc2782-target-2)

There MUST be one or more address records for this name, the name MUST NOT be an alias (in the sense of RFC 1034 or RFC 2181). (§Target)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Target-2, so no unit is bound to it.

### [`RFC2782-Notes-2`](#rfc2782-notes-2)

A client MUST parse all of the RR's in the reply. (§Notes)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Notes-2, so no unit is bound to it.

### [`RFC2782-Notes-3`](#rfc2782-notes-3)

If the Additional Data section doesn't contain address records for all the SRV RR's and the client may want to connect to the target host(s) involved, the client MUST look up the address record(s). (§Notes)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Notes-3, so no unit is bound to it.

## Extraction sign-off

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
| `front` | not stated | 12 | walked | not stated |

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
