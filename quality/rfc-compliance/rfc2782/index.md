# RFC 2782 - A DNS RR for specifying the location of services (DNS SRV)

No row in the public ledger. Every requirement this repository extracted from RFC 2782, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
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

Enrolled: DNS SRV resource records: seven MUST-level requirements, all {not-applicable}. ze serves SRV records verbatim as an authoritative server (internal/plugins/geodns: config.go parseSRV, server.go recordRR emitting dns.SRV) and implements no SRV-cognizant connecting client that performs priority/weight selection or Additional-section address resolution (internal/component/resolve/dns surfaces target:port strings without contacting or ordering; no production code resolves TypeSRV). The client-selection MUSTs (Priority-1, Notes-2 parse-all-RRs, Notes-3 Additional-section lookup), the protocol-specification-author MUSTs (Applicability-1 Service token, Applicability-2 security considerations), and the Target MUSTs (Target-1 address records present, Target-2 not a CNAME) all govern roles or data ze does not own.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2782.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (7):** [`RFC2782-Applicability-1`](#rfc2782-applicability-1), [`RFC2782-Applicability-2`](#rfc2782-applicability-2), [`RFC2782-Priority-1`](#rfc2782-priority-1), [`RFC2782-Target-1`](#rfc2782-target-1), [`RFC2782-Target-2`](#rfc2782-target-2), [`RFC2782-Notes-2`](#rfc2782-notes-2), [`RFC2782-Notes-3`](#rfc2782-notes-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2782-Applicability-1` | A protocol specification indicating SRV use MUST define the symbolic name to be used in the Service field of the SRV record (§Applicability) | MUST | Applicability | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this MUST binds the author of a protocol specification that adopts SRV to define the Service token; ze authors no such specification -- the _Service._Proto owner name is operator config data served verbatim (internal/plugins/geodns/config.go:319, server.go:111), not a name ze defines |
| `RFC2782-Applicability-2` | Such a specification MUST also include security considerations (§Applicability) | MUST | Applicability | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a security-considerations obligation on the author of an SRV-adopting protocol specification; ze publishes no such specification |
| `RFC2782-Priority-1` | A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority) | MUST | Priority | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no SRV-cognizant connecting client; geodns is authoritative and serves SRV verbatim without selection (internal/plugins/geodns/server.go:111), and internal/component/resolve/dns returns target:port strings without contacting or ordering by priority (resolver.go:321) -- no production code resolves TypeSRV |
| `RFC2782-Target-1` | There MUST be one or more address records for the Target name (§Target) | MUST | Target | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an SRV Target is an arbitrary FQDN that commonly lives outside any zone geodns is authoritative for, so geodns cannot require in-zone address records for it (internal/plugins/geodns/config.go:319); the address-record obligation belongs to the target's own authoritative zone |
| `RFC2782-Target-2` | The Target name MUST NOT be an alias, in the sense of RFC 1034 or RFC 2181 (§Target) | MUST NOT | Target | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns serves no CNAME record type (internal/plugins/geodns/record.go:10-14), so a geodns SRV Target can never resolve to a geodns-served alias; whether an external target name is a CNAME is outside geodns's authority |
| `RFC2782-Notes-2` | A client MUST parse all of the RRs in the reply (§Notes) | MUST | Notes | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this parse-all-RRs MUST governs the SRV-cognizant client that locates and connects to servers; ze has no such client. internal/component/resolve/dns iterates every answer RR (resolver.go:299) but is a generic stub resolver surfacing data, not a connecting SRV client, and no production code drives it with TypeSRV |
| `RFC2782-Notes-3` | If the Additional Data section lacks address records for all the SRV RRs, the client MUST look up the missing address records before connecting (§Notes) | MUST | Notes | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Additional-section address follow-up before connecting is a connecting-SRV-client obligation ze does not implement; internal/component/resolve/dns never reads the Additional section and never connects (resolver.go:299), and geodns as a server emits no target address glue for SRV answers (server.go:182-188) |
| `RFC2782-Applicability-3` | Service SRV records SHOULD NOT be used in the absence of such a protocol specification (§Applicability) | SHOULD NOT | Applicability | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Priority-2` | Target hosts with the same priority SHOULD be tried in an order defined by the weight field (§Priority) | SHOULD | Priority | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-1` | Larger weights SHOULD be given a proportionately higher probability of being selected (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-2` | Domain administrators SHOULD use Weight 0 when there is no server selection to do (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Weight-3` | The specified ordering algorithm SHOULD be used to order the SRV RRs of the same priority (§Weight) | SHOULD | Weight | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Usage-1` | A SRV-cognizant client SHOULD use the specified procedure to locate servers and connect to the preferred one (§Usage) | SHOULD | Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC2782-Notes-1` | Port numbers SHOULD NOT be used in place of the symbolic service or protocol names (§Notes) | SHOULD NOT | Notes | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2782-Applicability-1`](#rfc2782-applicability-1) A protocol specification indicating SRV use MUST define the symbolic name to be used in the Service field of the SRV record (§Applicability) | no test | no test carries this requirement id; annotated {not-applicable}: this MUST binds the author of a protocol specification that adopts SRV to define the Service token; ze authors no such specification -- the _Service._Proto owner name is operator config data served verbatim (internal/plugins/geodns/config.go:319, server.go:111), not a name ze defines |
| [`RFC2782-Applicability-2`](#rfc2782-applicability-2) Such a specification MUST also include security considerations (§Applicability) | no test | no test carries this requirement id; annotated {not-applicable}: this is a security-considerations obligation on the author of an SRV-adopting protocol specification; ze publishes no such specification |
| [`RFC2782-Priority-1`](#rfc2782-priority-1) A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no SRV-cognizant connecting client; geodns is authoritative and serves SRV verbatim without selection (internal/plugins/geodns/server.go:111), and internal/component/resolve/dns returns target:port strings without contacting or ordering by priority (resolver.go:321) -- no production code resolves TypeSRV |
| [`RFC2782-Target-1`](#rfc2782-target-1) There MUST be one or more address records for the Target name (§Target) | no test | no test carries this requirement id; annotated {not-applicable}: an SRV Target is an arbitrary FQDN that commonly lives outside any zone geodns is authoritative for, so geodns cannot require in-zone address records for it (internal/plugins/geodns/config.go:319); the address-record obligation belongs to the target's own authoritative zone |
| [`RFC2782-Target-2`](#rfc2782-target-2) The Target name MUST NOT be an alias, in the sense of RFC 1034 or RFC 2181 (§Target) | no test | no test carries this requirement id; annotated {not-applicable}: geodns serves no CNAME record type (internal/plugins/geodns/record.go:10-14), so a geodns SRV Target can never resolve to a geodns-served alias; whether an external target name is a CNAME is outside geodns's authority |
| [`RFC2782-Notes-2`](#rfc2782-notes-2) A client MUST parse all of the RRs in the reply (§Notes) | no test | no test carries this requirement id; annotated {not-applicable}: this parse-all-RRs MUST governs the SRV-cognizant client that locates and connects to servers; ze has no such client. internal/component/resolve/dns iterates every answer RR (resolver.go:299) but is a generic stub resolver surfacing data, not a connecting SRV client, and no production code drives it with TypeSRV |
| [`RFC2782-Notes-3`](#rfc2782-notes-3) If the Additional Data section lacks address records for all the SRV RRs, the client MUST look up the missing address records before connecting (§Notes) | no test | no test carries this requirement id; annotated {not-applicable}: Additional-section address follow-up before connecting is a connecting-SRV-client obligation ze does not implement; internal/component/resolve/dns never reads the Additional section and never connects (resolver.go:299), and geodns as a server emits no target address glue for SRV answers (server.go:182-188) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2782-Applicability-1`](#rfc2782-applicability-1)

A protocol specification indicating SRV use MUST define the symbolic name to be used in the Service field of the SRV record (§Applicability)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Applicability-1, so no unit is bound to it.

### [`RFC2782-Applicability-2`](#rfc2782-applicability-2)

Such a specification MUST also include security considerations (§Applicability)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Applicability-2, so no unit is bound to it.

### [`RFC2782-Priority-1`](#rfc2782-priority-1)

A client MUST attempt to contact the target host with the lowest-numbered priority it can reach (§Priority)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Priority-1, so no unit is bound to it.

### [`RFC2782-Target-1`](#rfc2782-target-1)

There MUST be one or more address records for the Target name (§Target)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Target-1, so no unit is bound to it.

### [`RFC2782-Target-2`](#rfc2782-target-2)

The Target name MUST NOT be an alias, in the sense of RFC 1034 or RFC 2181 (§Target)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Target-2, so no unit is bound to it.

### [`RFC2782-Notes-2`](#rfc2782-notes-2)

A client MUST parse all of the RRs in the reply (§Notes)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Notes-2, so no unit is bound to it.

### [`RFC2782-Notes-3`](#rfc2782-notes-3)

If the Additional Data section lacks address records for all the SRV RRs, the client MUST look up the missing address records before connecting (§Notes)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2782-Notes-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2782, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2782, so its obligations are stated where they were written.
