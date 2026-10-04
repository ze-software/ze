# RFC 4035 - Protocol Modifications for the DNS Security Extensions

Partial. Every requirement this repository extracted from RFC 4035, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4.6% | 5 of 108 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.3% | 9 of 108 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 108 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 108 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 41.7% | 10 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 108 | of 158 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 91 | of 108 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 84.3% | 91 of 108 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 108 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 108 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 2.8% | 3 of 108 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 108 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 158 |
| Gated MUST-level | 108 |
| Not applicable, so out of scope | 91 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 14 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc4035.md` |
| Requirement shard | `rfc/requirements/rfc4035.md` |
| RFC text | `rfc/full/rfc4035.txt` |

## Enrolment

Enrolled: Protocol Modifications for the DNS Security Extensions

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Security-aware non-validating stub resolver: `dnssec-validation` permissive/strict sets the EDNS0 DO bit with CD clear and a 4096-octet advertised buffer ([`internal/component/resolve/dns/resolver.go`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/resolver.go)), rejects (strict) or logs (permissive) an upstream SERVFAIL, accepts an AD-clear NOERROR answer from an unsigned zone, disregards the AD bit of a response, and returns ordinary records from answers that also carry RRSIG/NSEC/DNSKEY. On the authoritative side the shared harness copies the query's CD bit into the reply, ignores the query's AD bit, never asserts AD for local zone data, performs no DNSSEC additional processing, and answers a DS query inside a served zone as authoritative no-data
- tests bound per requirement in [`rfc/short/rfc4035.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4035.md).


**What the ledger says remains**

Three MUST gaps, each annotated in [`rfc/short/rfc4035.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4035.md): the name server supports no EDNS0 message size extension -- it emits no OPT pseudo-RR and honors no requestor payload size ([`RFC4035-3-1`](#rfc4035-3-1)) -- and its UDP listener reads at most 512 octets because `dns.Server.UDPSize` stays zero ([`RFC4035-3-2`](#rfc4035-3-2), [`internal/core/dnsserver/manager.go`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/manager.go)); and the stub rests its strict-mode decision on the upstream's validation carried over unauthenticated plain UDP ([`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2)). The zone-signing, signed-response, zone-transfer, recursive-server, and local-validation MUSTs are not-applicable: ze signs no zone, holds no DNSKEY/RRSIG/NSEC/DS record, never recurses, and runs no validator or BAD cache.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 103 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **108** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC4035-3-8`](#rfc4035-3-8), [`RFC4035-3-9`](#rfc4035-3-9), [`RFC4035-3.1.4.1-1`](#rfc4035-3.1.4.1-1), [`RFC4035-4.6-3`](#rfc4035-4.6-3), [`RFC4035-4.9-1`](#rfc4035-4.9-1)

**Annotated (including scoped evidence) (103):** [`RFC4035-2.1-2`](#rfc4035-2.1-2), [`RFC4035-2.1-4`](#rfc4035-2.1-4), [`RFC4035-2.1-5`](#rfc4035-2.1-5), [`RFC4035-2.2-1`](#rfc4035-2.2-1), [`RFC4035-2.2-3`](#rfc4035-2.2-3), [`RFC4035-2.2-4`](#rfc4035-2.2-4), [`RFC4035-2.2-5`](#rfc4035-2.2-5), [`RFC4035-2.2-6`](#rfc4035-2.2-6), [`RFC4035-2.2-7`](#rfc4035-2.2-7), [`RFC4035-2.2-8`](#rfc4035-2.2-8), [`RFC4035-2.3-1`](#rfc4035-2.3-1), [`RFC4035-2.3-3`](#rfc4035-2.3-3), [`RFC4035-2.3-4`](#rfc4035-2.3-4), [`RFC4035-2.3-5`](#rfc4035-2.3-5), [`RFC4035-2.3-6`](#rfc4035-2.3-6), [`RFC4035-2.3-7`](#rfc4035-2.3-7), [`RFC4035-2.4-3`](#rfc4035-2.4-3), [`RFC4035-2.4-4`](#rfc4035-2.4-4), [`RFC4035-2.5-1`](#rfc4035-2.5-1), [`RFC4035-2.5-2`](#rfc4035-2.5-2), [`RFC4035-2.6-1`](#rfc4035-2.6-1), [`RFC4035-3-1`](#rfc4035-3-1), [`RFC4035-3-2`](#rfc4035-3-2), [`RFC4035-3-5`](#rfc4035-3-5), [`RFC4035-3-6`](#rfc4035-3-6), [`RFC4035-3.1-1`](#rfc4035-3.1-1), [`RFC4035-3.1-2`](#rfc4035-3.1-2), [`RFC4035-3.1-3`](#rfc4035-3.1-3), [`RFC4035-3.1.1-3`](#rfc4035-3.1.1-3), [`RFC4035-3.1.1-4`](#rfc4035-3.1.1-4), [`RFC4035-3.1.1-5`](#rfc4035-3.1.1-5), [`RFC4035-3.1.1-6`](#rfc4035-3.1.1-6), [`RFC4035-3.1.1-8`](#rfc4035-3.1.1-8), [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3), [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4), [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1), [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1), [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1), [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1), [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2), [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1), [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1), [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2), [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3), [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2), [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3), [`RFC4035-3.1.5-4`](#rfc4035-3.1.5-4), [`RFC4035-3.1.5-5`](#rfc4035-3.1.5-5), [`RFC4035-3.1.5-6`](#rfc4035-3.1.5-6), [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7), [`RFC4035-3.1.6-2`](#rfc4035-3.1.6-2), [`RFC4035-3.1.6-4`](#rfc4035-3.1.6-4), [`RFC4035-3.1.6-5`](#rfc4035-3.1.6-5), [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6), [`RFC4035-3.2.1-1`](#rfc4035-3.2.1-1), [`RFC4035-3.2.1-2`](#rfc4035-3.2.1-2), [`RFC4035-3.2.1-3`](#rfc4035-3.2.1-3), [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1), [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4), [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2), [`RFC4035-4.1-1`](#rfc4035-4.1-1), [`RFC4035-4.1-2`](#rfc4035-4.1-2), [`RFC4035-4.1-4`](#rfc4035-4.1-4), [`RFC4035-4.1-5`](#rfc4035-4.1-5), [`RFC4035-4.2-1`](#rfc4035-4.2-1), [`RFC4035-4.2-3`](#rfc4035-4.2-3), [`RFC4035-4.2-5`](#rfc4035-4.2-5), [`RFC4035-4.2-6`](#rfc4035-4.2-6), [`RFC4035-4.3-1`](#rfc4035-4.3-1), [`RFC4035-4.4-1`](#rfc4035-4.4-1), [`RFC4035-4.6-2`](#rfc4035-4.6-2), [`RFC4035-4.7-2`](#rfc4035-4.7-2), [`RFC4035-4.7-3`](#rfc4035-4.7-3), [`RFC4035-4.7-7`](#rfc4035-4.7-7), [`RFC4035-4.8-1`](#rfc4035-4.8-1), [`RFC4035-4.9.1-2`](#rfc4035-4.9.1-2), [`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2), [`RFC4035-5-1`](#rfc4035-5-1), [`RFC4035-5-2`](#rfc4035-5-2), [`RFC4035-5-3`](#rfc4035-5-3), [`RFC4035-5.2-1`](#rfc4035-5.2-1), [`RFC4035-5.2-3`](#rfc4035-5.2-3), [`RFC4035-5.3.1-1`](#rfc4035-5.3.1-1), [`RFC4035-5.3.1-2`](#rfc4035-5.3.1-2), [`RFC4035-5.3.1-3`](#rfc4035-5.3.1-3), [`RFC4035-5.3.1-4`](#rfc4035-5.3.1-4), [`RFC4035-5.3.1-5`](#rfc4035-5.3.1-5), [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6), [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7), [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8), [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9), [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10), [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1), [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2), [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3), [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1), [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2), [`RFC4035-5.4-1`](#rfc4035-5.4-1), [`RFC4035-5.4-2`](#rfc4035-5.4-2), [`RFC4035-5.4-3`](#rfc4035-5.4-3), [`RFC4035-5.4-4`](#rfc4035-5.4-4), [`RFC4035-5.5-2`](#rfc4035-5.5-2), [`RFC4035-5.5-3`](#rfc4035-5.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4035-2.1-2` | A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.1-4` | Public keys associated with other DNS operations MAY be stored in DNSKEY RRs that are not marked as zone keys but MUST NOT be used to verify RRSIGs. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.1-5` | If the zone administrator intends a signed zone to be usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point into the zone. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-1` | For each authoritative RRset in a signed zone, there MUST be at least one RRSIG record that meets the following requirements (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-3` | An RRSIG RR itself MUST NOT be signed (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-4` | The NS RRset that appears at the zone apex name MUST be signed (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-5` | The NS RRset that appears at the zone apex name MUST be signed, but the NS RRsets that appear at delegation points (that is, the NS RRsets in the parent zone that delegate the name to the child zone's name servers) MUST NOT be signed. (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-6` | Glue address RRsets associated with delegations MUST NOT be signed (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-7` | There MUST be an RRSIG for each RRset using at least one DNSKEY of each algorithm in the zone apex DNSKEY RRset (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-8` | The apex DNSKEY RRset itself MUST be signed by each algorithm appearing in the DS RRset located at the delegating parent (if any). (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-1` | Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-3` | An NSEC record (and its associated RRSIG RRset) MUST NOT be the only RRset at any particular owner name. (§2.3) | MUST NOT | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-4` | That is, the signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed. (§2.3) | MUST NOT | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-5` | The type bitmap of every NSEC resource record in a signed zone MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-6` | Bits corresponding to the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-7` | bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.4-3` | All DS RRsets in a zone MUST be signed (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.4-4` | DS RRsets MUST NOT appear at a zone's apex (§2.4) | MUST NOT | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.5-1` | If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name (§2.5) | REQUIRED | 2.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.5-2` | If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name. A KEY RRset at that name for secure dynamic update purposes is also allowed ([RFC3007]). Other types MUST NOT be present at that name. (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.6-1` | At the parental side of a zone cut (that is, at a delegation point), NSEC RRs are REQUIRED at the owner name. (§2.6) | REQUIRED | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-3-1` | A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's authoritative servers give the EDNS0 message size extension no support -- the only SetEdns0 producer in ze is the stub resolver (internal/component/resolve/dns/resolver.go:261); the server path reads an OPT only for the client-subnet address (internal/core/dnsserver/client.go:23) and writes a reply built by msg.SetReply with no OPT pseudo-RR and no requestor-payload-size handling (internal/core/dnsserver/handler.go:55) |
| `RFC4035-3-2` | A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension, MUST support a message size of at least 1220 octets (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the UDP listener accepts at most 512 octets -- dns.Server is constructed with UDPSize left zero (internal/core/dnsserver/manager.go:165), which miekg defaults to MinMsgSize 512 (vendor/github.com/miekg/dns/server.go:287), so a query above 512 octets is never read whole and no reply advertises a larger size |
| `RFC4035-3-5` | A security-aware name server that receives a DNS query that does not include the EDNS OPT pseudo-RR or that has the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3-6` | A security-aware name server that receives a DNS query that does not include the EDNS OPT pseudo-RR or that has the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset and MUST NOT perform any of the additional processing described below. (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC4035DOClearQueryGetsNoDNSSECProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_do_clear_test.go#L37). **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L164). **negative:** no negative test. **{single-polarity}:** ze performs no DNSSEC additional processing for any query -- answerQuestions builds A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) and never consults the DO bit, so no processing path exists to drive negatively |
| `RFC4035-3-8` | The CD bit is controlled by resolvers; a security-aware name server MUST copy the CD bit from a query into the corresponding response. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L51). **negative:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L56) |
| `RFC4035-3-9` | The AD bit is controlled by name servers; a security-aware name server MUST ignore the setting of the AD bit in queries. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L62). **negative:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L69) |
| `RFC4035-3.1-1` | RRSIG RRs that can be used to authenticate a response MUST be included in the response according to the rules in Section 3.1.1. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1-2` | NSEC RRs that can be used to provide authenticated denial of existence MUST be included in the response automatically according to the rules in Section 3.1.3. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1-3` | Either a DS RRset or an NSEC RR proving that no DS RRs exist MUST be included in referrals automatically (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-3` | When placing a signed RRset in the Answer section, the name server MUST also place its RRSIG RRs in the Answer section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-4` | If space does not permit inclusion of these RRSIG RRs, the name server MUST set the TC bit. (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-5` | When placing a signed RRset in the Authority section, the name server MUST also place its RRSIG RRs in the Authority section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-6` | When placing a signed RRset in the Additional section, the name server MUST also place its RRSIG RRs in the Additional section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-8` | If this happens, the name server MUST NOT set the TC bit solely because these RRSIG RRs didn't fit. (§3.1.1) | MUST NOT | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.2-3` | If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them (§3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.2-4` | If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them and MUST NOT set the TC bit solely because these RRs didn't fit (§3.1.2) | MUST NOT | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3-1` | When responding to a query that has the DO bit set, a security-aware authoritative name server for a signed zone MUST include NSEC RRs in each of the following cases (§3.1.3) | MUST | 3.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.1-1` | If the zone contains RRsets matching <SNAME, SCLASS> but contains no RRset matching <SNAME, SCLASS, STYPE>, then the name server MUST include the NSEC RR for <SNAME, SCLASS> along with its associated RRSIG RR(s) in the Authority section of the response (see Section 3.1.1). (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.2-1` | If the zone does not contain any RRsets matching <SNAME, SCLASS> either exactly or via wildcard name expansion, then the name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there is no exact match for <SNAME, SCLASS>. o An NSEC RR proving that the zone contains no RRsets that would match <SNAME, SCLASS> via wildcard name expansion. (§3.1.3.2) | MUST | 3.1.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.3-1` | If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3) | MUST | 3.1.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.3-2` | If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section and MUST include in the Authority section an NSEC RR and associated RRSIG RR(s) proving that the zone does not contain a closer match for <SNAME, SCLASS>. (§3.1.3.3) | MUST | 3.1.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.4-1` | The name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there are no RRsets matching STYPE at the wildcard owner name that matched <SNAME, SCLASS> via wildcard expansion. o An NSEC RR proving that there are no RRsets in the zone that would have been a closer match for <SNAME, SCLASS>. (§3.1.3.4) | MUST | 3.1.3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-1` | If a DS RRset is present at the delegation point, the name server MUST return both the DS RRset and its associated RRSIG RR(s) in the Authority section along with the NS RRset. (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-2` | If no DS RRset is present at the delegation point, the name server MUST return both the NSEC RR that proves that the DS RRset is not present and the NSEC RR's associated RRSIG RR(s) along with the NS RRset. (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-3` | The name server MUST place the NS RRset before the NSEC RRset and its associated RRSIG RR(s). (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4.1-1` | In this case, the name server MUST return an authoritative "no data" response showing that the DS RRset does not exist in the child zone's apex. (§3.1.4.1) | MUST | 3.1.4.1 | **positive:** `unit/verify` [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L85). **negative:** `unit/verify` [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L112) |
| `RFC4035-3.1.5-2` | An authoritative name server that chooses to perform its own zone validation MUST NOT selectively reject some RRs and accept others. (§3.1.5) | MUST NOT | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-3` | As with any other authoritative RRset, the DS RRset MUST be included in zone transfers of the zone in which the RRset is authoritative data. (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-4` | NSEC RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-5` | The parental NSEC RR at a zone cut MUST be included in zone transfers of the parent zone (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-6` | the NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone. (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-7` | RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.6-2` | A security-aware name server MUST NOT set the AD bit in a response unless the name server considers all RRsets in the Answer and Authority sections of the response to be authentic. (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L177). **negative:** no negative test. **{single-polarity}:** no ze code sets AuthenticatedData on a reply -- the single non-test AuthenticatedData reference reads the upstream's bit in the stub resolver (internal/component/resolve/dns/resolver.go:278) -- so an AD-asserting response cannot be produced to reject |
| `RFC4035-3.1.6-4` | A security-aware name server's local policy MAY consider data from an authoritative zone to be authentic without further validation. However, the name server MUST NOT do so unless the name server obtained the authoritative zone via secure means (such as a secure zone transfer mechanism) (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L179). **negative:** no negative test. **{single-polarity}:** ze takes its zone data from the local running configuration and still serves it with AD clear, because no reply-building path sets AuthenticatedData (internal/core/dnsserver/handler.go:55, internal/plugins/geodns/server.go:168); there is no AD-setting path to drive negatively |
| `RFC4035-3.1.6-5` | A security-aware name server's local policy MAY consider data from an authoritative zone to be authentic without further validation. However, the name server MUST NOT do so unless the name server obtained the authoritative zone via secure means (such as a secure zone transfer mechanism) and MUST NOT do so unless this behavior has been configured explicitly. (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L182). **negative:** no negative test. **{single-polarity}:** ze has no configuration leaf that marks local zone data authentic, so the AD bit stays clear whatever the operator configures (the geodns YANG carries no such leaf and no server path writes AuthenticatedData, internal/plugins/geodns/server.go:168) |
| `RFC4035-3.1.6-6` | A security-aware name server that supports recursion MUST follow the rules for the CD and AD bits given in Section 3.2 when generating a response that involves data obtained via recursion. (§3.1.6) | MUST | 3.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-1` | The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-2` | If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-3` | If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response but MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.2-1` | The name server side of a security-aware recursive name server MUST pass the state of the CD bit to the resolver side along with the rest of an initiating query, so that the resolver side will know whether it is required to verify the response data it returns to the name server side. (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.2-4` | If the resolver side implements a BAD cache (see Section 4.7) and the name server side receives a query that matches an entry in the resolver side's BAD cache, the name server side's response depends on the state of the CD bit in the original query. If the CD bit is set, the name server side SHOULD return the data from the BAD cache; if the CD bit is not set, the name server side MUST return RCODE 2 (server failure). (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.3-2` | The resolver side MUST follow the procedure described in Section 5 to determine whether the RRs in question are authentic. (§3.2.3) | MUST | 3.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-4.1-1` | A security-aware resolver MUST include an EDNS ([RFC2671]) OPT pseudo-RR with the DO ([RFC3225]) bit set when sending queries. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035SecurityAwareQueryInEveryValidatingMode`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L45). **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L60). **negative:** no negative test. **{single-polarity}:** the DO bit is emitted rather than parsed -- SetEdns0(4096, validating) sets it whenever dnssec-validation is permissive or strict (internal/component/resolve/dns/resolver.go:261) -- so there is no non-conformant input to reject; off mode is a plain non-security-aware stub, which this requirement does not govern |
| `RFC4035-4.1-2` | A security-aware resolver MUST support a message size of at least 1220 octets (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035_LargeUDPResponseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L116). **negative:** no negative test. **{single-polarity}:** the resolver advertises 4096 octets and miekg sizes the receive buffer from that advertisement (vendor/github.com/miekg/dns/client.go:201), so a response above 1220 octets arrives whole; a message-size floor has no non-conformant input to reject |
| `RFC4035-4.1-4` | MUST use the "sender's UDP payload size" field in the EDNS OPT pseudo-RR to advertise the message size that it is willing to accept. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035SecurityAwareQueryInEveryValidatingMode`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L52). **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L70). **negative:** no negative test. **{single-polarity}:** the sender's UDP payload size field is emitted, not parsed -- SetEdns0 writes 4096 on every query (internal/component/resolve/dns/resolver.go:261) -- so no malformed input exists to reject |
| `RFC4035-4.1-5` | A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly regardless of whether any such fragmented packets were received via IPv4 or IPv6. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** UDP fragment reassembly belongs to the host IP stack, which ze neither implements nor bypasses -- the resolver exchanges datagrams through the ordinary miekg client socket (internal/component/resolve/dns/resolver.go:263) and grep -rniE 'fragment\|reassembl' --include=*.go internal/component/resolve internal/core/dnsserver finds no producer |
| `RFC4035-4.2-1` | A security-aware resolver MUST support the signature verification mechanisms described in Section 5 (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-3` | A security-aware resolver's support for signature verification MUST include support for verification of wildcard owner names. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-5` | When attempting to retrieve missing NSEC RRs that reside on the parental side at a zone cut, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-6` | When attempting to retrieve a missing DS, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.3-1` | A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.4-1` | A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.6-2` | A security-aware resolver MUST clear the AD bit when composing query messages (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L75). **negative:** no negative test. **{single-polarity}:** the AD bit of an outgoing query is emitted, not parsed -- the query is composed by new(mdns.Msg) plus SetQuestion (internal/component/resolve/dns/resolver.go:255) and no ze code sets AuthenticatedData on a query -- so no negative input exists |
| `RFC4035-4.6-3` | A resolver MUST disregard the meaning of the CD and AD bits in a response unless the response was obtained by using a secure channel or the resolver was specifically configured to regard the message header bits without using a secure channel. (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestRFC4035ResponseCDAndADBitsChangeNothing`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L90). **positive:** `unit/verify` [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L148). **negative:** `unit/verify` [`TestRFC4035ResponseCDAndADBitsChangeNothing`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L103). **negative:** `unit/verify` [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L159) |
| `RFC4035-4.7-2` | Resolvers that implement a BAD cache MUST take steps to prevent the cache from being useful as a denial-of-service attack amplifier (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.7-3` | Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL. (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.7-7` | Resolvers MUST NOT return RRsets from the BAD cache unless the resolver is not required to validate the signatures of the RRsets in question under the rules given in Section 4.2 of this document. (§4.7) | MUST NOT | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.8-1` | A validating security-aware resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs that could have been synthesized from the DNAME RR, as described in [RFC2672], at least to the extent of not rejecting a response message solely because it contains such CNAME RRs. (§4.8) | MUST | 4.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.9-1` | A security-aware stub resolver MUST support the DNSSEC RR types, at least to the extent of not mishandling responses just because they contain DNSSEC RRs. (§4.9) | MUST | 4.9 | **positive:** `unit/verify` [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L208). **negative:** `unit/verify` [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L219) |
| `RFC4035-4.9.1-2` | A validating security-aware stub resolver MUST set the DO bit (§4.9.1) | MUST | 4.9.1 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L62). **negative:** no negative test. **{single-polarity}:** ze's stub sets the DO bit under permissive and strict (internal/component/resolve/dns/resolver.go:261) and leaves the signature check to the upstream; the bit is emitted, not parsed, so there is no non-conformant input to reject |
| `RFC4035-4.9.3-2` | In any case, a security-aware stub resolver MUST NOT place any reliance on signature validation allegedly performed on its behalf, except when the security-aware stub resolver obtained the data in question from a trusted security-aware recursive name server via a secure channel. (§4.9.3) | MUST NOT | 4.9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's stub rests a security decision on validation performed for it over an unauthenticated channel -- strict mode rejects an answer solely because the configured upstream returned SERVFAIL (internal/component/resolve/dns/resolver.go:103-106) while the client speaks plain UDP with no TLS, TSIG, or other authentication of that server (internal/component/resolve/dns/resolver.go:81) |
| `RFC4035-5-1` | To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5-2` | To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set; and 2. verify that there is some RRSIG RR that covers the apex DNSKEY RRset, and that the combination of the RRSIG RR and the initial DNSKEY RR authenticates the DNSKEY RRset. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5-3` | The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.2-1` | A security-aware resolver MUST query the name servers for the parent zone for the DS RRset if the referral includes neither a DS RRset nor a NSEC RRset proving that the DS RRset does not exist (see Section 4). (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.2-3` | A security-aware resolver MUST use the parent NSEC RR when attempting to prove that a DS RRset does not exist (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-1` | The RRSIG RR and the RRset MUST have the same owner name and the same class (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-2` | The RRSIG RR's Signer's Name field MUST be the name of the zone that contains the RRset (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-3` | The RRSIG RR's Type Covered field MUST equal the RRset's type (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-4` | The number of labels in the RRset owner name MUST be greater than or equal to the value in the RRSIG RR's Labels field (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-5` | o The validator's notion of the current time MUST be less than or equal to the time listed in the RRSIG RR's Expiration field. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-6` | o The validator's notion of the current time MUST be greater than or equal to the time listed in the RRSIG RR's Inception field. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-7` | o The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag for some DNSKEY RR in the zone's apex DNSKEY RRset. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-8` | The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-9` | The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset, and MUST have the Zone Flag bit (DNSKEY RDATA Flag bit 7) set. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-10` | It is possible for more than one DNSKEY RR to match the conditions above. In this case, the validator cannot predetermine which DNSKEY RR to use to authenticate the signature, and it MUST try each matching DNSKEY RR until either the signature is validated or the validator has run out of matching public keys to try. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-1` | if rrsig_labels > fqdn_labels the RRSIG RR did not pass the necessary validation checks and MUST NOT be used to authenticate this RRset. (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-2` | When reconstructing the original NSEC RRset for the delegation from the parent zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the child zone. (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-3` | When reconstructing the original NSEC RRset for the apex of the child zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone. (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.3-1` | If the Labels field of the RRSIG RR is not equal to the number of labels in the RRset's fully qualified owner name, then the RRset is either invalid or the result of wildcard expansion. The resolver MUST verify that wildcard expansion was applied properly before considering the RRset to be authentic. (§5.3.3) | MUST | 5.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.3-2` | If the resolver accepts the RRset as authentic, the validator MUST set the TTL of the RRSIG RR and each RR in the authenticated RRset to a value no greater than the minimum of: o the RRset's TTL as received in the response; o the RRSIG RR's TTL as received in the response; o the value in the RRSIG RR's Original TTL field; and o the difference of the RRSIG RR's Signature Expiration time and the current time. (§5.3.3) | MUST | 5.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-1` | In addition, security-aware resolvers MUST authenticate the NSEC RRsets that comprise the non-existence proof as described in Section 5.3. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-2` | If the complete set of necessary NSEC RRsets is not present in a response (perhaps due to message truncation), then a security-aware resolver MUST resend the query in order to attempt to obtain the full collection of NSEC RRs necessary to verify the non-existence of the requested RRset. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-3` | As with all DNS operations, however, the resolver MUST bound the work it puts into answering any particular query. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-4` | Since a validated NSEC RR proves the existence of both itself and its corresponding RRSIG RR, a validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.5-2` | If the validation was being done to service a recursive query, the name server MUST return RCODE 2 to the originating client. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-5.5-3` | However, it MUST return the full response if and only if the original query had the CD bit set. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-2.1-1` | For each private key used to create RRSIG RRs in a zone, the zone SHOULD include a zone DNSKEY RR containing the corresponding public key. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.3-2` | The TTL value for any NSEC RR SHOULD be the same as the minimum TTL value field in the zone SOA RR (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-1` | A DS RRset SHOULD be present at a delegation point when the child zone is signed (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-5` | A DS RR SHOULD point to a DNSKEY RR that is present in the child's apex DNSKEY RRset (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-6` | A DS RR SHOULD point to a DNSKEY RR that is present in the child's apex DNSKEY RRset, and the child's apex DNSKEY RRset SHOULD be signed by the corresponding private key. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-7` | The TTL of a DS RRset SHOULD match the TTL of the delegating NS RRset (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-3` | A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension, MUST support a message size of at least 1220 octets, and SHOULD support a message size of 4000 octets. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-4` | As IPv6 packets can only be fragmented by the source host, a security aware name server SHOULD take steps to ensure that UDP datagrams it transmits over IPv6 are fragmented, if necessary, at the minimum IPv6 MTU, unless the path MTU is known. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-10` | A security aware name server that synthesizes CNAME RRs from DNAME RRs as described in [RFC2672] SHOULD NOT generate signatures for the synthesized CNAME RRs. (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-1` | When responding to a query that has the DO bit set, a security-aware authoritative name server SHOULD attempt to send RRSIG RRs that a security-aware resolver can use to authenticate the RRsets in the response. (§3.1.1) | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-2` | A name server SHOULD make every attempt to keep the RRset and its associated RRSIG(s) together in a response. (§3.1.1) | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.2-2` | The name server SHOULD NOT include the DNSKEY RRset unless there is enough space in the response message for both the DNSKEY RRset and its associated RRSIG RR(s). (§3.1.2) | SHOULD NOT | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.3.2-2` | In some cases, a single NSEC RR may prove both of these points. If it does, the name server SHOULD only include the NSEC RR and its RRSIG RR(s) once in the Authority section. (§3.1.3.2) | SHOULD | 3.1.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.3.4-2` | In some cases, a single NSEC RR may prove both of these points. If it does, the name server SHOULD only include the NSEC RR and its RRSIG RR(s) once in the Authority section. (§3.1.3.4) | SHOULD | 3.1.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.6-1` | A security-aware name server SHOULD clear the CD bit when composing an authoritative response (§3.1.6) | SHOULD | 3.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.2-2` | When the CD bit is set, the recursive name server SHOULD, if possible, return the requested data to the originating resolver, even if the recursive name server's local authentication policy would reject the records in question. (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.2-3` | If the resolver side implements a BAD cache (see Section 4.7) and the name server side receives a query that matches an entry in the resolver side's BAD cache, the name server side's response depends on the state of the CD bit in the original query. If the CD bit is set, the name server side SHOULD return the data from the BAD cache (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.3-1` | The name server side SHOULD set the AD bit if and only if the resolver side considers all RRsets in the Answer section and any relevant negative response RRs in the Authority section to be authentic. (§3.2.3) | SHOULD | 3.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.1-3` | A security-aware resolver MUST support a message size of at least 1220 octets, SHOULD support a message size of 4000 octets (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.2-2` | A security-aware resolver MUST support the signature verification mechanisms described in Section 5 and SHOULD apply them to every received response, except when: o the security-aware resolver is part of a security-aware recursive name server, and the response is the result of recursion on behalf of a query received with the CD bit set; o the response is the result of a query generated directly via some form of application interface that instructed the security-aware resolver not to perform validation for this query; or o validation for this query has been disabled by local policy. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.4-2` | A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR and SHOULD be capable of being configured with multiple trusted public keys or DS RRs. (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.4-3` | Since a security-aware resolver will not be able to validate signatures without such a configured trust anchor, the resolver SHOULD have some reasonably robust mechanism for obtaining such keys when it boots (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.5-1` | A security-aware resolver SHOULD cache each response as a single atomic entry containing the entire answer, including the named RRset and any associated DNSSEC RRs. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.5-2` | The resolver SHOULD discard the entire atomic entry when any of the RRs contained in it expire. (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-4` | Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL. This TTL SHOULD be small, in order to mitigate the effect of caching the results of an attack. (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-5` | In order to prevent caching of a transient validation failure (which might be the result of an attack), resolvers SHOULD track queries that result in validation failures (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-6` | In order to prevent caching of a transient validation failure (which might be the result of an attack), resolvers SHOULD track queries that result in validation failures and SHOULD only answer from the BAD cache after the number of times that responses to queries for that particular <QNAME, QTYPE, QCLASS> have failed to validate exceeds a threshold value. (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.2-1` | A non-validating security-aware stub resolver SHOULD NOT set the CD bit when sending queries unless it is requested by the application layer, as by definition, a non-validating stub resolver depends on the security-aware recursive name server to perform validation on its behalf. (§4.9.2) | SHOULD NOT | 4.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.2-2` | A validating security-aware stub resolver SHOULD set the CD bit (§4.9.2) | SHOULD | 4.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.3-3` | A validating security-aware stub resolver SHOULD NOT examine the setting of the AD bit (§4.9.3) | SHOULD NOT | 4.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5-4` | A resolver SHOULD expect authentication information from signed zones (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5-5` | A resolver SHOULD believe that a zone is signed if the resolver has been configured with public key information for the zone, or if the zone's parent is signed and the delegation from the parent contains a DS RRset. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.1-1` | Validating signatures within an island of security requires that the validator have some other means of obtaining an initial authenticated zone key for the island. If a validator cannot obtain such a key, it SHOULD switch to operating as if the zones in the island of security are unsigned. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.2-4` | If the resolver does not support any of the algorithms listed in an authenticated DS RRset, then the resolver will not be able to verify the authentication path to the child zone. In this case, the resolver SHOULD treat the child zone as if it were unsigned. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.5-1` | If for whatever reason none of the RRSIGs can be validated, the response SHOULD be considered BAD. (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.1-3` | Public keys associated with other DNS operations MAY be stored in DNSKEY RRs that are not marked as zone keys (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.2-2` | An RRset MAY have multiple RRSIG RRs associated with it (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-2` | The DS RRset MAY contain multiple records, each referencing a public key in the child zone used to verify the RRSIGs in that zone. (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-7` | Security aware name servers that receive explicit queries for security RR types that match the content of more than one zone that it serves (for example, NSEC and RRSIG RRs above and below a delegation point where the server is authoritative for both zones) should behave self-consistently. As long as the response is always consistent for each query to the name server, the name server MAY return one of the following: o The above-delegation RRsets. o The below-delegation RRsets. o Both above and below-delegation RRsets. o Empty answer section (no records). o Some other response. o An error. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-7` | If space does not permit inclusion of both the RRset and its associated RRSIG RRs, the name server MAY retain the RRset while dropping the RRSIG RRs. (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.2-1` | When responding to a query that has the DO bit set and that requests the SOA or NS RRs at the apex of a signed zone, a security-aware authoritative name server for that zone MAY return the zone apex DNSKEY RRset in the Additional section. (§3.1.2) | MAY | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.5-1` | However, an authoritative name server MAY choose to reject the entire zone transfer if the zone fails to meet any of the signing requirements described in Section 2. (§3.1.5) | MAY | 3.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.3-3` | However, for backward compatibility, a recursive name server MAY set the AD bit when a response includes unsigned CNAME RRs if those CNAME RRs demonstrably could have been synthesized from an authentic DNAME RR that is also included in the response according to the synthesis rules described in [RFC2672]. (§3.2.3) | MAY | 3.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.2-4` | Security-aware resolvers MAY query for missing security RRs in an attempt to perform validation (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.6-1` | A security-aware resolver MAY set a query's CD bit in order to indicate that the resolver takes responsibility for performing whatever authentication its local policy requires on the RRsets in the response. (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-1` | To prevent such unnecessary DNS traffic, security-aware resolvers MAY cache data with invalid signatures, with some restrictions. (§4.7) | MAY | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.8-2` | A validating security-aware resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs that could have been synthesized from the DNAME RR, as described in [RFC2672], at least to the extent of not rejecting a response message solely because it contains such CNAME RRs. The resolver MAY retain such CNAME RRs in its cache or in the answers it hands back, but is not required to do so. (§4.8) | MAY | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.1-1` | A non-validating security-aware stub resolver MAY include the DNSSEC RRs returned by a security-aware recursive name server as part of the data that the stub resolver hands back to the application that invoked it, but is not required to do so. (§4.9.1) | MAY | 4.9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.3-1` | A non-validating security-aware stub resolver MAY chose to examine the setting of the AD bit in response messages that it receives in order to determine whether the security-aware recursive name server that sent the response claims to have cryptographically verified the data in the Answer and Authority sections of the response message. (§4.9.3) | MAY | 4.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.2-2` | If the validator authenticates an NSEC RRset that proves that no DS RRset is present for this zone, then there is no authentication path leading from the parent to the child. If the resolver has an initial DNSKEY or DS RR that belongs to the child zone or to any delegation below the child zone, this initial DNSKEY or DS RR MAY be used to re-establish an authentication path. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4035-2.1-2`](#rfc4035-2.1-2) A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.1-4`](#rfc4035-2.1-4) Public keys associated with other DNS operations MAY be stored in DNSKEY RRs that are not marked as zone keys but MUST NOT be used to verify RRSIGs. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.1-5`](#rfc4035-2.1-5) If the zone administrator intends a signed zone to be usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point into the zone. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-1`](#rfc4035-2.2-1) For each authoritative RRset in a signed zone, there MUST be at least one RRSIG record that meets the following requirements (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-3`](#rfc4035-2.2-3) An RRSIG RR itself MUST NOT be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-4`](#rfc4035-2.2-4) The NS RRset that appears at the zone apex name MUST be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-5`](#rfc4035-2.2-5) The NS RRset that appears at the zone apex name MUST be signed, but the NS RRsets that appear at delegation points (that is, the NS RRsets in the parent zone that delegate the name to the child zone's name servers) MUST NOT be signed. (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-6`](#rfc4035-2.2-6) Glue address RRsets associated with delegations MUST NOT be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-7`](#rfc4035-2.2-7) There MUST be an RRSIG for each RRset using at least one DNSKEY of each algorithm in the zone apex DNSKEY RRset (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-8`](#rfc4035-2.2-8) The apex DNSKEY RRset itself MUST be signed by each algorithm appearing in the DS RRset located at the delegating parent (if any). (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-1`](#rfc4035-2.3-1) Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-3`](#rfc4035-2.3-3) An NSEC record (and its associated RRSIG RRset) MUST NOT be the only RRset at any particular owner name. (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-4`](#rfc4035-2.3-4) That is, the signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed. (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-5`](#rfc4035-2.3-5) The type bitmap of every NSEC resource record in a signed zone MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record. (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-6`](#rfc4035-2.3-6) Bits corresponding to the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-7`](#rfc4035-2.3-7) bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.4-3`](#rfc4035-2.4-3) All DS RRsets in a zone MUST be signed (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.4-4`](#rfc4035-2.4-4) DS RRsets MUST NOT appear at a zone's apex (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.5-1`](#rfc4035-2.5-1) If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name (§2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.5-2`](#rfc4035-2.5-2) If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name. A KEY RRset at that name for secure dynamic update purposes is also allowed ([RFC3007]). Other types MUST NOT be present at that name. (§2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.6-1`](#rfc4035-2.6-1) At the parental side of a zone cut (that is, at a delegation point), NSEC RRs are REQUIRED at the owner name. (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-3-1`](#rfc4035-3-1) A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension (§3) | {gap}, no test | ze's authoritative servers give the EDNS0 message size extension no support -- the only SetEdns0 producer in ze is the stub resolver (internal/component/resolve/dns/resolver.go:261); the server path reads an OPT only for the client-subnet address (internal/core/dnsserver/client.go:23) and writes a reply built by msg.SetReply with no OPT pseudo-RR and no requestor-payload-size handling (internal/core/dnsserver/handler.go:55) |
| [`RFC4035-3-2`](#rfc4035-3-2) A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension, MUST support a message size of at least 1220 octets (§3) | {gap}, no test | the UDP listener accepts at most 512 octets -- dns.Server is constructed with UDPSize left zero (internal/core/dnsserver/manager.go:165), which miekg defaults to MinMsgSize 512 (vendor/github.com/miekg/dns/server.go:287), so a query above 512 octets is never read whole and no reply advertises a larger size |
| [`RFC4035-3-5`](#rfc4035-3-5) A security-aware name server that receives a DNS query that does not include the EDNS OPT pseudo-RR or that has the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-1`](#rfc4035-3.1-1) RRSIG RRs that can be used to authenticate a response MUST be included in the response according to the rules in Section 3.1.1. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-2`](#rfc4035-3.1-2) NSEC RRs that can be used to provide authenticated denial of existence MUST be included in the response automatically according to the rules in Section 3.1.3. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-3`](#rfc4035-3.1-3) Either a DS RRset or an NSEC RR proving that no DS RRs exist MUST be included in referrals automatically (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-3`](#rfc4035-3.1.1-3) When placing a signed RRset in the Answer section, the name server MUST also place its RRSIG RRs in the Answer section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-4`](#rfc4035-3.1.1-4) If space does not permit inclusion of these RRSIG RRs, the name server MUST set the TC bit. (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-5`](#rfc4035-3.1.1-5) When placing a signed RRset in the Authority section, the name server MUST also place its RRSIG RRs in the Authority section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-6`](#rfc4035-3.1.1-6) When placing a signed RRset in the Additional section, the name server MUST also place its RRSIG RRs in the Additional section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-8`](#rfc4035-3.1.1-8) If this happens, the name server MUST NOT set the TC bit solely because these RRSIG RRs didn't fit. (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3) If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them (§3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4) If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them and MUST NOT set the TC bit solely because these RRs didn't fit (§3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1) When responding to a query that has the DO bit set, a security-aware authoritative name server for a signed zone MUST include NSEC RRs in each of the following cases (§3.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1) If the zone contains RRsets matching <SNAME, SCLASS> but contains no RRset matching <SNAME, SCLASS, STYPE>, then the name server MUST include the NSEC RR for <SNAME, SCLASS> along with its associated RRSIG RR(s) in the Authority section of the response (see Section 3.1.1). (§3.1.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1) If the zone does not contain any RRsets matching <SNAME, SCLASS> either exactly or via wildcard name expansion, then the name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there is no exact match for <SNAME, SCLASS>. o An NSEC RR proving that the zone contains no RRsets that would match <SNAME, SCLASS> via wildcard name expansion. (§3.1.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1) If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2) If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section and MUST include in the Authority section an NSEC RR and associated RRSIG RR(s) proving that the zone does not contain a closer match for <SNAME, SCLASS>. (§3.1.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1) The name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there are no RRsets matching STYPE at the wildcard owner name that matched <SNAME, SCLASS> via wildcard expansion. o An NSEC RR proving that there are no RRsets in the zone that would have been a closer match for <SNAME, SCLASS>. (§3.1.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1) If a DS RRset is present at the delegation point, the name server MUST return both the DS RRset and its associated RRSIG RR(s) in the Authority section along with the NS RRset. (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2) If no DS RRset is present at the delegation point, the name server MUST return both the NSEC RR that proves that the DS RRset is not present and the NSEC RR's associated RRSIG RR(s) along with the NS RRset. (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3) The name server MUST place the NS RRset before the NSEC RRset and its associated RRSIG RR(s). (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2) An authoritative name server that chooses to perform its own zone validation MUST NOT selectively reject some RRs and accept others. (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3) As with any other authoritative RRset, the DS RRset MUST be included in zone transfers of the zone in which the RRset is authoritative data. (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-4`](#rfc4035-3.1.5-4) NSEC RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-5`](#rfc4035-3.1.5-5) The parental NSEC RR at a zone cut MUST be included in zone transfers of the parent zone (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-6`](#rfc4035-3.1.5-6) the NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone. (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7) RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6) A security-aware name server that supports recursion MUST follow the rules for the CD and AD bits given in Section 3.2 when generating a response that involves data obtained via recursion. (§3.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-1`](#rfc4035-3.2.1-1) The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-2`](#rfc4035-3.2.1-2) If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-3`](#rfc4035-3.2.1-3) If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response but MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1) The name server side of a security-aware recursive name server MUST pass the state of the CD bit to the resolver side along with the rest of an initiating query, so that the resolver side will know whether it is required to verify the response data it returns to the name server side. (§3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4) If the resolver side implements a BAD cache (see Section 4.7) and the name server side receives a query that matches an entry in the resolver side's BAD cache, the name server side's response depends on the state of the CD bit in the original query. If the CD bit is set, the name server side SHOULD return the data from the BAD cache; if the CD bit is not set, the name server side MUST return RCODE 2 (server failure). (§3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2) The resolver side MUST follow the procedure described in Section 5 to determine whether the RRs in question are authentic. (§3.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-4.1-5`](#rfc4035-4.1-5) A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly regardless of whether any such fragmented packets were received via IPv4 or IPv6. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: UDP fragment reassembly belongs to the host IP stack, which ze neither implements nor bypasses -- the resolver exchanges datagrams through the ordinary miekg client socket (internal/component/resolve/dns/resolver.go:263) and grep -rniE 'fragment\|reassembl' --include=*.go internal/component/resolve internal/core/dnsserver finds no producer |
| [`RFC4035-4.2-1`](#rfc4035-4.2-1) A security-aware resolver MUST support the signature verification mechanisms described in Section 5 (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-3`](#rfc4035-4.2-3) A security-aware resolver's support for signature verification MUST include support for verification of wildcard owner names. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-5`](#rfc4035-4.2-5) When attempting to retrieve missing NSEC RRs that reside on the parental side at a zone cut, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-6`](#rfc4035-4.2-6) When attempting to retrieve a missing DS, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.3-1`](#rfc4035-4.3-1) A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed. (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.4-1`](#rfc4035-4.4-1) A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.7-2`](#rfc4035-4.7-2) Resolvers that implement a BAD cache MUST take steps to prevent the cache from being useful as a denial-of-service attack amplifier (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.7-3`](#rfc4035-4.7-3) Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL. (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.7-7`](#rfc4035-4.7-7) Resolvers MUST NOT return RRsets from the BAD cache unless the resolver is not required to validate the signatures of the RRsets in question under the rules given in Section 4.2 of this document. (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.8-1`](#rfc4035-4.8-1) A validating security-aware resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs that could have been synthesized from the DNAME RR, as described in [RFC2672], at least to the extent of not rejecting a response message solely because it contains such CNAME RRs. (§4.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2) In any case, a security-aware stub resolver MUST NOT place any reliance on signature validation allegedly performed on its behalf, except when the security-aware stub resolver obtained the data in question from a trusted security-aware recursive name server via a secure channel. (§4.9.3) | {gap}, no test | ze's stub rests a security decision on validation performed for it over an unauthenticated channel -- strict mode rejects an answer solely because the configured upstream returned SERVFAIL (internal/component/resolve/dns/resolver.go:103-106) while the client speaks plain UDP with no TLS, TSIG, or other authentication of that server (internal/component/resolve/dns/resolver.go:81) |
| [`RFC4035-5-1`](#rfc4035-5-1) To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5-2`](#rfc4035-5-2) To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set; and 2. verify that there is some RRSIG RR that covers the apex DNSKEY RRset, and that the combination of the RRSIG RR and the initial DNSKEY RR authenticates the DNSKEY RRset. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5-3`](#rfc4035-5-3) The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.2-1`](#rfc4035-5.2-1) A security-aware resolver MUST query the name servers for the parent zone for the DS RRset if the referral includes neither a DS RRset nor a NSEC RRset proving that the DS RRset does not exist (see Section 4). (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.2-3`](#rfc4035-5.2-3) A security-aware resolver MUST use the parent NSEC RR when attempting to prove that a DS RRset does not exist (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-1`](#rfc4035-5.3.1-1) The RRSIG RR and the RRset MUST have the same owner name and the same class (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-2`](#rfc4035-5.3.1-2) The RRSIG RR's Signer's Name field MUST be the name of the zone that contains the RRset (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-3`](#rfc4035-5.3.1-3) The RRSIG RR's Type Covered field MUST equal the RRset's type (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-4`](#rfc4035-5.3.1-4) The number of labels in the RRset owner name MUST be greater than or equal to the value in the RRSIG RR's Labels field (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-5`](#rfc4035-5.3.1-5) o The validator's notion of the current time MUST be less than or equal to the time listed in the RRSIG RR's Expiration field. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6) o The validator's notion of the current time MUST be greater than or equal to the time listed in the RRSIG RR's Inception field. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7) o The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag for some DNSKEY RR in the zone's apex DNSKEY RRset. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8) The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9) The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset, and MUST have the Zone Flag bit (DNSKEY RDATA Flag bit 7) set. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10) It is possible for more than one DNSKEY RR to match the conditions above. In this case, the validator cannot predetermine which DNSKEY RR to use to authenticate the signature, and it MUST try each matching DNSKEY RR until either the signature is validated or the validator has run out of matching public keys to try. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1) if rrsig_labels > fqdn_labels the RRSIG RR did not pass the necessary validation checks and MUST NOT be used to authenticate this RRset. (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2) When reconstructing the original NSEC RRset for the delegation from the parent zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the child zone. (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3) When reconstructing the original NSEC RRset for the apex of the child zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone. (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1) If the Labels field of the RRSIG RR is not equal to the number of labels in the RRset's fully qualified owner name, then the RRset is either invalid or the result of wildcard expansion. The resolver MUST verify that wildcard expansion was applied properly before considering the RRset to be authentic. (§5.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2) If the resolver accepts the RRset as authentic, the validator MUST set the TTL of the RRSIG RR and each RR in the authenticated RRset to a value no greater than the minimum of: o the RRset's TTL as received in the response; o the RRSIG RR's TTL as received in the response; o the value in the RRSIG RR's Original TTL field; and o the difference of the RRSIG RR's Signature Expiration time and the current time. (§5.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-1`](#rfc4035-5.4-1) In addition, security-aware resolvers MUST authenticate the NSEC RRsets that comprise the non-existence proof as described in Section 5.3. (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-2`](#rfc4035-5.4-2) If the complete set of necessary NSEC RRsets is not present in a response (perhaps due to message truncation), then a security-aware resolver MUST resend the query in order to attempt to obtain the full collection of NSEC RRs necessary to verify the non-existence of the requested RRset. (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-3`](#rfc4035-5.4-3) As with all DNS operations, however, the resolver MUST bound the work it puts into answering any particular query. (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-4`](#rfc4035-5.4-4) Since a validated NSEC RR proves the existence of both itself and its corresponding RRSIG RR, a validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR. (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.5-2`](#rfc4035-5.5-2) If the validation was being done to service a recursive query, the name server MUST return RCODE 2 to the originating client. (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-5.5-3`](#rfc4035-5.5-3) However, it MUST return the full response if and only if the original query had the CD bit set. (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4035-2.1-2`](#rfc4035-2.1-2)

A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-2, so no unit is bound to it.

### [`RFC4035-2.1-4`](#rfc4035-2.1-4)

Public keys associated with other DNS operations MAY be stored in DNSKEY RRs that are not marked as zone keys but MUST NOT be used to verify RRSIGs. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-4, so no unit is bound to it.

### [`RFC4035-2.1-5`](#rfc4035-2.1-5)

If the zone administrator intends a signed zone to be usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point into the zone. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-5, so no unit is bound to it.

### [`RFC4035-2.2-1`](#rfc4035-2.2-1)

For each authoritative RRset in a signed zone, there MUST be at least one RRSIG record that meets the following requirements (§2.2)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Binds the signer of a signed zone; Ze signs no zone.

No test carries RFC4035-2.2-1, so no unit is bound to it.

### [`RFC4035-2.2-3`](#rfc4035-2.2-3)

An RRSIG RR itself MUST NOT be signed (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-3, so no unit is bound to it.

### [`RFC4035-2.2-4`](#rfc4035-2.2-4)

The NS RRset that appears at the zone apex name MUST be signed (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-4, so no unit is bound to it.

### [`RFC4035-2.2-5`](#rfc4035-2.2-5)

The NS RRset that appears at the zone apex name MUST be signed, but the NS RRsets that appear at delegation points (that is, the NS RRsets in the parent zone that delegate the name to the child zone's name servers) MUST NOT be signed. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-5, so no unit is bound to it.

### [`RFC4035-2.2-6`](#rfc4035-2.2-6)

Glue address RRsets associated with delegations MUST NOT be signed (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-6, so no unit is bound to it.

### [`RFC4035-2.2-7`](#rfc4035-2.2-7)

There MUST be an RRSIG for each RRset using at least one DNSKEY of each algorithm in the zone apex DNSKEY RRset (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-7, so no unit is bound to it.

### [`RFC4035-2.2-8`](#rfc4035-2.2-8)

The apex DNSKEY RRset itself MUST be signed by each algorithm appearing in the DS RRset located at the delegating parent (if any). (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-8, so no unit is bound to it.

### [`RFC4035-2.3-1`](#rfc4035-2.3-1)

Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-1, so no unit is bound to it.

### [`RFC4035-2.3-3`](#rfc4035-2.3-3)

An NSEC record (and its associated RRSIG RRset) MUST NOT be the only RRset at any particular owner name. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-3, so no unit is bound to it.

### [`RFC4035-2.3-4`](#rfc4035-2.3-4)

That is, the signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-4, so no unit is bound to it.

### [`RFC4035-2.3-5`](#rfc4035-2.3-5)

The type bitmap of every NSEC resource record in a signed zone MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-5, so no unit is bound to it.

### [`RFC4035-2.3-6`](#rfc4035-2.3-6)

Bits corresponding to the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-6, so no unit is bound to it.

### [`RFC4035-2.3-7`](#rfc4035-2.3-7)

bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-7, so no unit is bound to it.

### [`RFC4035-2.4-3`](#rfc4035-2.4-3)

All DS RRsets in a zone MUST be signed (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.4-3, so no unit is bound to it.

### [`RFC4035-2.4-4`](#rfc4035-2.4-4)

DS RRsets MUST NOT appear at a zone's apex (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.4-4, so no unit is bound to it.

### [`RFC4035-2.5-1`](#rfc4035-2.5-1)

If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.5-1, so no unit is bound to it.

### [`RFC4035-2.5-2`](#rfc4035-2.5-2)

If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name. A KEY RRset at that name for secure dynamic update purposes is also allowed ([RFC3007]). Other types MUST NOT be present at that name. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.5-2, so no unit is bound to it.

### [`RFC4035-2.6-1`](#rfc4035-2.6-1)

At the parental side of a zone cut (that is, at a delegation point), NSEC RRs are REQUIRED at the owner name. (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.6-1, so no unit is bound to it.

### [`RFC4035-3-1`](#rfc4035-3-1)

A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-1, so no unit is bound to it.

### [`RFC4035-3-2`](#rfc4035-3-2)

A security-aware name server MUST support the EDNS0 ([RFC2671]) message size extension, MUST support a message size of at least 1220 octets (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-2, so no unit is bound to it.

### [`RFC4035-3-5`](#rfc4035-3-5)

A security-aware name server that receives a DNS query that does not include the EDNS OPT pseudo-RR or that has the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-5, so no unit is bound to it.

### [`RFC4035-3-6`](#rfc4035-3-6)

A security-aware name server that receives a DNS query that does not include the EDNS OPT pseudo-RR or that has the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset and MUST NOT perform any of the additional processing described below. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. The second arm is now covered: TestRFC4035DOClearQueryGetsNoDNSSECProcessing sends A, DNSKEY, RRSIG and NSEC queries with an OPT whose DO is clear and asserts no RRSIG/NSEC/DNSKEY in any section and the same Answer/Authority counts as the OPT-less query (the OPT-less arm stays in rfc4035_server_test.go). The comparison is by count, not content; content differences would not be DNSSEC processing, so this does not weaken the proof. Record: answerQuestions revert observed red. {single-polarity} holds: geodns has no DNSSEC path to drive with DO set.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035DOClearQueryGetsNoDNSSECProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_do_clear_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L164) | unit/verify | revert, verified |

### [`RFC4035-3-8`](#rfc4035-3-8)

The CD bit is controlled by resolvers; a security-aware name server MUST copy the CD bit from a query into the corresponding response. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4035_CDCopiedADIgnored drives the real dnsserver.Authoritative harness: a CD=1 query must draw CD=1 and a CD=0 query CD=0, so both an always-clear and an always-set implementation go red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L56) | unit/verify | unproven |
| positive | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L51) | unit/verify | unproven |

### [`RFC4035-3-9`](#rfc4035-3-9)

The AD bit is controlled by name servers; a security-aware name server MUST ignore the setting of the AD bit in queries. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. the negative asserts an AD=1 query draws a reply with AD clear through the real harness; the positive's answer-equality half runs over the answerOne test stub, which cannot vary, so the AD-echo assertion is the discriminating one

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L69) | unit/verify | unproven |
| positive | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L62) | unit/verify | unproven |

### [`RFC4035-3.1-1`](#rfc4035-3.1-1)

RRSIG RRs that can be used to authenticate a response MUST be included in the response according to the rules in Section 3.1.1. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1-1, so no unit is bound to it.

### [`RFC4035-3.1-2`](#rfc4035-3.1-2)

NSEC RRs that can be used to provide authenticated denial of existence MUST be included in the response automatically according to the rules in Section 3.1.3. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1-2, so no unit is bound to it.

### [`RFC4035-3.1-3`](#rfc4035-3.1-3)

Either a DS RRset or an NSEC RR proving that no DS RRs exist MUST be included in referrals automatically (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1-3, so no unit is bound to it.

### [`RFC4035-3.1.1-3`](#rfc4035-3.1.1-3)

When placing a signed RRset in the Answer section, the name server MUST also place its RRSIG RRs in the Answer section (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.1-3, so no unit is bound to it.

### [`RFC4035-3.1.1-4`](#rfc4035-3.1.1-4)

If space does not permit inclusion of these RRSIG RRs, the name server MUST set the TC bit. (§3.1.1)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Binds a security-aware name server holding RRSIGs; Ze's authoritative servers are not security-aware.

No test carries RFC4035-3.1.1-4, so no unit is bound to it.

### [`RFC4035-3.1.1-5`](#rfc4035-3.1.1-5)

When placing a signed RRset in the Authority section, the name server MUST also place its RRSIG RRs in the Authority section (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.1-5, so no unit is bound to it.

### [`RFC4035-3.1.1-6`](#rfc4035-3.1.1-6)

When placing a signed RRset in the Additional section, the name server MUST also place its RRSIG RRs in the Additional section (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.1-6, so no unit is bound to it.

### [`RFC4035-3.1.1-8`](#rfc4035-3.1.1-8)

If this happens, the name server MUST NOT set the TC bit solely because these RRSIG RRs didn't fit. (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.1-8, so no unit is bound to it.

### [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3)

If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.2-3, so no unit is bound to it.

### [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4)

If there is not enough space to include these DNSKEY and RRSIG RRs, the name server MUST omit them and MUST NOT set the TC bit solely because these RRs didn't fit (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.2-4, so no unit is bound to it.

### [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1)

When responding to a query that has the DO bit set, a security-aware authoritative name server for a signed zone MUST include NSEC RRs in each of the following cases (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3-1, so no unit is bound to it.

### [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1)

If the zone contains RRsets matching <SNAME, SCLASS> but contains no RRset matching <SNAME, SCLASS, STYPE>, then the name server MUST include the NSEC RR for <SNAME, SCLASS> along with its associated RRSIG RR(s) in the Authority section of the response (see Section 3.1.1). (§3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.1-1, so no unit is bound to it.

### [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1)

If the zone does not contain any RRsets matching <SNAME, SCLASS> either exactly or via wildcard name expansion, then the name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there is no exact match for <SNAME, SCLASS>. o An NSEC RR proving that the zone contains no RRsets that would match <SNAME, SCLASS> via wildcard name expansion. (§3.1.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.2-1, so no unit is bound to it.

### [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1)

If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.3-1, so no unit is bound to it.

### [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2)

If the zone does not contain any RRsets that exactly match <SNAME, SCLASS> but does contain an RRset that matches <SNAME, SCLASS, STYPE> via wildcard name expansion, the name server MUST include the wildcard-expanded answer and the corresponding wildcard-expanded RRSIG RRs in the Answer section and MUST include in the Authority section an NSEC RR and associated RRSIG RR(s) proving that the zone does not contain a closer match for <SNAME, SCLASS>. (§3.1.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.3-2, so no unit is bound to it.

### [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1)

The name server MUST include the following NSEC RRs in the Authority section, along with their associated RRSIG RRs: o An NSEC RR proving that there are no RRsets matching STYPE at the wildcard owner name that matched <SNAME, SCLASS> via wildcard expansion. o An NSEC RR proving that there are no RRsets in the zone that would have been a closer match for <SNAME, SCLASS>. (§3.1.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.4-1, so no unit is bound to it.

### [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1)

If a DS RRset is present at the delegation point, the name server MUST return both the DS RRset and its associated RRSIG RR(s) in the Authority section along with the NS RRset. (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-1, so no unit is bound to it.

### [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2)

If no DS RRset is present at the delegation point, the name server MUST return both the NSEC RR that proves that the DS RRset is not present and the NSEC RR's associated RRSIG RR(s) along with the NS RRset. (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-2, so no unit is bound to it.

### [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3)

The name server MUST place the NS RRset before the NSEC RRset and its associated RRSIG RR(s). (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-3, so no unit is bound to it.

### [`RFC4035-3.1.4.1-1`](#rfc4035-3.1.4.1-1)

In this case, the name server MUST return an authoritative "no data" response showing that the DS RRset does not exist in the child zone's apex. (§3.1.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. an apex DS query through geodns answerQuery under the real harness must return NOERROR, AA set, empty Answer, SOA in Authority, and no RA; an NXDOMAIN or REFUSED answer at the apex goes red; the negative pins that the no-data answer is not given for an out-of-zone name

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L112) | unit/verify | unproven |
| positive | [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L85) | unit/verify | unproven |

### [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2)

An authoritative name server that chooses to perform its own zone validation MUST NOT selectively reject some RRs and accept others. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-2, so no unit is bound to it.

### [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3)

As with any other authoritative RRset, the DS RRset MUST be included in zone transfers of the zone in which the RRset is authoritative data. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-3, so no unit is bound to it.

### [`RFC4035-3.1.5-4`](#rfc4035-3.1.5-4)

NSEC RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-4, so no unit is bound to it.

### [`RFC4035-3.1.5-5`](#rfc4035-3.1.5-5)

The parental NSEC RR at a zone cut MUST be included in zone transfers of the parent zone (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-5, so no unit is bound to it.

### [`RFC4035-3.1.5-6`](#rfc4035-3.1.5-6)

the NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-6, so no unit is bound to it.

### [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7)

RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-7, so no unit is bound to it.

### [`RFC4035-3.1.6-2`](#rfc4035-3.1.6-2)

A security-aware name server MUST NOT set the AD bit in a response unless the name server considers all RRsets in the Answer and Authority sections of the response to be authentic. (§3.1.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single polarity: asserts AD clear on both the OPT-less and DO-set geodns replies, and ze authenticates nothing, so any AD-setting path goes red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L177) | unit/verify | unproven |

### [`RFC4035-3.1.6-4`](#rfc4035-3.1.6-4)

A security-aware name server's local policy MAY consider data from an authoritative zone to be authentic without further validation. However, the name server MUST NOT do so unless the name server obtained the authoritative zone via secure means (such as a secure zone transfer mechanism) (§3.1.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single polarity: geodns serves zone data from the running configuration, not obtained by secure means, and the unit asserts it is never flagged authentic (AD clear) on either reply

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L179) | unit/verify | unproven |

### [`RFC4035-3.1.6-5`](#rfc4035-3.1.6-5)

A security-aware name server's local policy MAY consider data from an authoritative zone to be authentic without further validation. However, the name server MUST NOT do so unless the name server obtained the authoritative zone via secure means (such as a secure zone transfer mechanism) and MUST NOT do so unless this behavior has been configured explicitly. (§3.1.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single polarity: no configuration marks local data authentic, and the unit asserts AD clear on the geodns replies whatever the query's DO bit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L182) | unit/verify | unproven |

### [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6)

A security-aware name server that supports recursion MUST follow the rules for the CD and AD bits given in Section 3.2 when generating a response that involves data obtained via recursion. (§3.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.6-6, so no unit is bound to it.

### [`RFC4035-3.2.1-1`](#rfc4035-3.2.1-1)

The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.1-1, so no unit is bound to it.

### [`RFC4035-3.2.1-2`](#rfc4035-3.2.1-2)

If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.1-2, so no unit is bound to it.

### [`RFC4035-3.2.1-3`](#rfc4035-3.2.1-3)

If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response but MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.1-3, so no unit is bound to it.

### [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1)

The name server side of a security-aware recursive name server MUST pass the state of the CD bit to the resolver side along with the rest of an initiating query, so that the resolver side will know whether it is required to verify the response data it returns to the name server side. (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.2-1, so no unit is bound to it.

### [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4)

If the resolver side implements a BAD cache (see Section 4.7) and the name server side receives a query that matches an entry in the resolver side's BAD cache, the name server side's response depends on the state of the CD bit in the original query. If the CD bit is set, the name server side SHOULD return the data from the BAD cache; if the CD bit is not set, the name server side MUST return RCODE 2 (server failure). (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.2-4, so no unit is bound to it.

### [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2)

The resolver side MUST follow the procedure described in Section 5 to determine whether the RRs in question are authentic. (§3.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.3-2, so no unit is bound to it.

### [`RFC4035-4.1-1`](#rfc4035-4.1-1)

A security-aware resolver MUST include an EDNS ([RFC2671]) OPT pseudo-RR with the DO ([RFC3225]) bit set when sending queries. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. TestRFC4035SecurityAwareQueryInEveryValidatingMode now runs permissive AND strict (the earlier strict-only finding): the query carries an OPT with DO set in both. Record: Resolver.query revert observed red. {single-polarity} holds: an emitted field with no input to refuse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035SecurityAwareQueryInEveryValidatingMode`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L60) | unit/verify | revert, verified |

### [`RFC4035-4.1-2`](#rfc4035-4.1-2)

A security-aware resolver MUST support a message size of at least 1220 octets (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_LargeUDPResponseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L116) | unit/verify | unproven |

### [`RFC4035-4.1-4`](#rfc4035-4.1-4)

MUST use the "sender's UDP payload size" field in the EDNS OPT pseudo-RR to advertise the message size that it is willing to accept. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Same unit: 4096 advertised in both validating modes, and a ~2900-octet uncompressed UDP answer is accepted whole (all 110 records), tying the advertised size to what the receive path accepts (the earlier finding). Record: query revert observed red. {single-polarity} holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035SecurityAwareQueryInEveryValidatingMode`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L70) | unit/verify | revert, verified |

### [`RFC4035-4.1-5`](#rfc4035-4.1-5)

A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly regardless of whether any such fragmented packets were received via IPv4 or IPv6. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.1-5, so no unit is bound to it.

### [`RFC4035-4.2-1`](#rfc4035-4.2-1)

A security-aware resolver MUST support the signature verification mechanisms described in Section 5 (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-1, so no unit is bound to it.

### [`RFC4035-4.2-3`](#rfc4035-4.2-3)

A security-aware resolver's support for signature verification MUST include support for verification of wildcard owner names. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-3, so no unit is bound to it.

### [`RFC4035-4.2-5`](#rfc4035-4.2-5)

When attempting to retrieve missing NSEC RRs that reside on the parental side at a zone cut, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-5, so no unit is bound to it.

### [`RFC4035-4.2-6`](#rfc4035-4.2-6)

When attempting to retrieve a missing DS, a security-aware iterative-mode resolver MUST query the name servers for the parent zone, not the child zone. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-6, so no unit is bound to it.

### [`RFC4035-4.3-1`](#rfc4035-4.3-1)

A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed. (§4.3)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Binds a validating (security-aware) resolver; Ze's resolver is a non-validating stub.

No test carries RFC4035-4.3-1, so no unit is bound to it.

### [`RFC4035-4.4-1`](#rfc4035-4.4-1)

A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.4-1, so no unit is bound to it.

### [`RFC4035-4.6-2`](#rfc4035-4.6-2)

A security-aware resolver MUST clear the AD bit when composing query messages (§4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L75) | unit/verify | unproven |

### [`RFC4035-4.6-3`](#rfc4035-4.6-3)

A resolver MUST disregard the meaning of the CD and AD bits in a response unless the response was obtained by using a secure channel or the resolver was specifically configured to regard the message header bits without using a secure channel. (§4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Both bits now: under strict validation a NOERROR answer with AD=1 CD=1 yields exactly the records of the clear answer (a resolver requiring or honouring AD would differ), and SERVFAIL with AD, CD or both is refused, so no response bit lets a failure through. Records: query (+) and dnssecDecision (-) reverts observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035ResponseCDAndADBitsChangeNothing`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC4035ResponseCDAndADBitsChangeNothing`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_security_aware_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L148) | unit/verify | revert, verified |

### [`RFC4035-4.7-2`](#rfc4035-4.7-2)

Resolvers that implement a BAD cache MUST take steps to prevent the cache from being useful as a denial-of-service attack amplifier (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-2, so no unit is bound to it.

### [`RFC4035-4.7-3`](#rfc4035-4.7-3)

Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL. (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-3, so no unit is bound to it.

### [`RFC4035-4.7-7`](#rfc4035-4.7-7)

Resolvers MUST NOT return RRsets from the BAD cache unless the resolver is not required to validate the signatures of the RRsets in question under the rules given in Section 4.2 of this document. (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-7, so no unit is bound to it.

### [`RFC4035-4.8-1`](#rfc4035-4.8-1)

A validating security-aware resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs that could have been synthesized from the DNAME RR, as described in [RFC2672], at least to the extent of not rejecting a response message solely because it contains such CNAME RRs. (§4.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.8-1, so no unit is bound to it.

### [`RFC4035-4.9-1`](#rfc4035-4.9-1)

A security-aware stub resolver MUST support the DNSSEC RR types, at least to the extent of not mishandling responses just because they contain DNSSEC RRs. (§4.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. an A+RRSIG answer must return exactly the A record and a DNSSEC-only answer (RRSIG, NSEC, DNSKEY) must yield no records and no error, so mishandling either way goes red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L208) | unit/verify | unproven |

### [`RFC4035-4.9.1-2`](#rfc4035-4.9.1-2)

A validating security-aware stub resolver MUST set the DO bit (§4.9.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L62) | unit/verify | unproven |

### [`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2)

In any case, a security-aware stub resolver MUST NOT place any reliance on signature validation allegedly performed on its behalf, except when the security-aware stub resolver obtained the data in question from a trusted security-aware recursive name server via a secure channel. (§4.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.9.3-2, so no unit is bound to it.

### [`RFC4035-5-1`](#rfc4035-5-1)

To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-1, so no unit is bound to it.

### [`RFC4035-5-2`](#rfc4035-5-2)

To authenticate an apex DNSKEY RRset by using an initial key, the resolver MUST: 1. verify that the initial DNSKEY RR appears in the apex DNSKEY RRset, and that the DNSKEY RR has the Zone Key Flag (DNSKEY RDATA bit 7) set; and 2. verify that there is some RRSIG RR that covers the apex DNSKEY RRset, and that the combination of the RRSIG RR and the initial DNSKEY RR authenticates the DNSKEY RRset. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-2, so no unit is bound to it.

### [`RFC4035-5-3`](#rfc4035-5-3)

The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-3, so no unit is bound to it.

### [`RFC4035-5.2-1`](#rfc4035-5.2-1)

A security-aware resolver MUST query the name servers for the parent zone for the DS RRset if the referral includes neither a DS RRset nor a NSEC RRset proving that the DS RRset does not exist (see Section 4). (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.2-1, so no unit is bound to it.

### [`RFC4035-5.2-3`](#rfc4035-5.2-3)

A security-aware resolver MUST use the parent NSEC RR when attempting to prove that a DS RRset does not exist (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.2-3, so no unit is bound to it.

### [`RFC4035-5.3.1-1`](#rfc4035-5.3.1-1)

The RRSIG RR and the RRset MUST have the same owner name and the same class (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-1, so no unit is bound to it.

### [`RFC4035-5.3.1-2`](#rfc4035-5.3.1-2)

The RRSIG RR's Signer's Name field MUST be the name of the zone that contains the RRset (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-2, so no unit is bound to it.

### [`RFC4035-5.3.1-3`](#rfc4035-5.3.1-3)

The RRSIG RR's Type Covered field MUST equal the RRset's type (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-3, so no unit is bound to it.

### [`RFC4035-5.3.1-4`](#rfc4035-5.3.1-4)

The number of labels in the RRset owner name MUST be greater than or equal to the value in the RRSIG RR's Labels field (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-4, so no unit is bound to it.

### [`RFC4035-5.3.1-5`](#rfc4035-5.3.1-5)

o The validator's notion of the current time MUST be less than or equal to the time listed in the RRSIG RR's Expiration field. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-5, so no unit is bound to it.

### [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6)

o The validator's notion of the current time MUST be greater than or equal to the time listed in the RRSIG RR's Inception field. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-6, so no unit is bound to it.

### [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7)

o The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag for some DNSKEY RR in the zone's apex DNSKEY RRset. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-7, so no unit is bound to it.

### [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8)

The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-8, so no unit is bound to it.

### [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9)

The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset, and MUST have the Zone Flag bit (DNSKEY RDATA Flag bit 7) set. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-9, so no unit is bound to it.

### [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10)

It is possible for more than one DNSKEY RR to match the conditions above. In this case, the validator cannot predetermine which DNSKEY RR to use to authenticate the signature, and it MUST try each matching DNSKEY RR until either the signature is validated or the validator has run out of matching public keys to try. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-10, so no unit is bound to it.

### [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1)

if rrsig_labels > fqdn_labels the RRSIG RR did not pass the necessary validation checks and MUST NOT be used to authenticate this RRset. (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-1, so no unit is bound to it.

### [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2)

When reconstructing the original NSEC RRset for the delegation from the parent zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the child zone. (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-2, so no unit is bound to it.

### [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3)

When reconstructing the original NSEC RRset for the apex of the child zone, the NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone. (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-3, so no unit is bound to it.

### [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1)

If the Labels field of the RRSIG RR is not equal to the number of labels in the RRset's fully qualified owner name, then the RRset is either invalid or the result of wildcard expansion. The resolver MUST verify that wildcard expansion was applied properly before considering the RRset to be authentic. (§5.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.3-1, so no unit is bound to it.

### [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2)

If the resolver accepts the RRset as authentic, the validator MUST set the TTL of the RRSIG RR and each RR in the authenticated RRset to a value no greater than the minimum of: o the RRset's TTL as received in the response; o the RRSIG RR's TTL as received in the response; o the value in the RRSIG RR's Original TTL field; and o the difference of the RRSIG RR's Signature Expiration time and the current time. (§5.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.3-2, so no unit is bound to it.

### [`RFC4035-5.4-1`](#rfc4035-5.4-1)

In addition, security-aware resolvers MUST authenticate the NSEC RRsets that comprise the non-existence proof as described in Section 5.3. (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-1, so no unit is bound to it.

### [`RFC4035-5.4-2`](#rfc4035-5.4-2)

If the complete set of necessary NSEC RRsets is not present in a response (perhaps due to message truncation), then a security-aware resolver MUST resend the query in order to attempt to obtain the full collection of NSEC RRs necessary to verify the non-existence of the requested RRset. (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-2, so no unit is bound to it.

### [`RFC4035-5.4-3`](#rfc4035-5.4-3)

As with all DNS operations, however, the resolver MUST bound the work it puts into answering any particular query. (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-3, so no unit is bound to it.

### [`RFC4035-5.4-4`](#rfc4035-5.4-4)

Since a validated NSEC RR proves the existence of both itself and its corresponding RRSIG RR, a validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR. (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-4, so no unit is bound to it.

### [`RFC4035-5.5-2`](#rfc4035-5.5-2)

If the validation was being done to service a recursive query, the name server MUST return RCODE 2 to the originating client. (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.5-2, so no unit is bound to it.

### [`RFC4035-5.5-3`](#rfc4035-5.5-3)

However, it MUST return the full response if and only if the original query had the CD bit set. (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.5-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc4035.txt |
| Source fingerprint | a47565ed03b850dd |
| Record | rfc/extraction/rfc4035.json |
| Mapped sentences | 97 |
| Declined as scope | 15 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 6 | walked | not stated |
| `2.3` | not stated | 5 | walked | not stated |
| `2.4` | not stated | 1 | walked | not stated |
| `2.5` | not stated | 2 | walked | not stated |
| `2.6` | not stated | 1 | walked | not stated |
| `2.7` | not stated | 0 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.1.1` | not stated | 6 | walked | not stated |
| `3.1.2` | not stated | 1 | walked | not stated |
| `3.1.3` | not stated | 1 | walked | not stated |
| `3.1.3.1` | not stated | 2 | walked | not stated |
| `3.1.3.2` | not stated | 2 | walked | not stated |
| `3.1.3.3` | not stated | 2 | walked | not stated |
| `3.1.3.4` | not stated | 2 | walked | not stated |
| `3.1.3.5` | not stated | 0 | walked | not stated |
| `3.1.4` | not stated | 4 | walked | not stated |
| `3.1.4.1` | not stated | 1 | walked | not stated |
| `3.1.5` | not stated | 6 | walked | not stated |
| `3.1.6` | not stated | 3 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 2 | walked | not stated |
| `3.2.2` | not stated | 3 | walked | not stated |
| `3.2.3` | not stated | 2 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 5 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `4.4` | not stated | 1 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 2 | walked | not stated |
| `4.7` | not stated | 3 | walked | not stated |
| `4.8` | not stated | 2 | walked | not stated |
| `4.9` | not stated | 1 | walked | not stated |
| `4.9.1` | not stated | 2 | walked | not stated |
| `4.9.2` | not stated | 0 | walked | not stated |
| `4.9.3` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 2 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 9 | walked | not stated |
| `5.3.2` | not stated | 3 | walked | not stated |
| `5.3.3` | not stated | 2 | walked | not stated |
| `5.3.4` | not stated | 1 | walked | not stated |
| `5.4` | not stated | 5 | walked | not stated |
| `5.5` | not stated | 2 | walked | not stated |
| `5.6` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `B.1` | not stated | 0 | walked | not stated |
| `B.2` | not stated | 0 | walked | not stated |
| `B.3` | not stated | 0 | walked | not stated |
| `B.4` | not stated | 0 | walked | not stated |
| `B.5` | not stated | 0 | walked | not stated |
| `B.6` | not stated | 0 | walked | not stated |
| `B.7` | not stated | 0 | walked | not stated |
| `B.8` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |
| `C.1` | not stated | 0 | walked | not stated |
| `C.1.1` | not stated | 0 | walked | not stated |
| `C.2` | not stated | 0 | walked | not stated |
| `C.3` | not stated | 0 | walked | not stated |
| `C.4` | not stated | 0 | walked | not stated |
| `C.5` | not stated | 0 | walked | not stated |
| `C.6` | not stated | 1 | walked | not stated |
| `C.7` | not stated | 0 | walked | not stated |
| `C.8` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the lead-in sentence to the three bullets that follow; site 3.1:2 maps the first two bullets and site 3.1:3 the third | Upon receiving a relevant query that has the EDNS ([RFC2671]) OPT pseudo-RR DO bit ([RFC3225]) set, a security-aware authoritative name server for a signed zone MUST include additional RRSIG, NSEC, and DS RRs, according to the following rules: |
| `3.1.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Authority-section repeat of the sentence already stated for the Answer section; site 3.1.1:2 maps it | If space does not permit inclusion of these RRSIG RRs, the name server MUST set the TC bit. |
| `3.1.3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same 'if space does not permit ... MUST set the TC bit' rule section 3.1.1 states and this site cites back to it; site 3.1.1:2 maps it | If space does not permit inclusion of the NSEC RR or its associated RRSIG RR(s), the name server MUST set the TC bit (see Section 3.1.1). |
| `3.1.3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same 'if space does not permit ... MUST set the TC bit' rule section 3.1.1 states and this site cites back to it; site 3.1.1:2 maps it | If space does not permit inclusion of these NSEC and RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1). |
| `3.1.3.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same 'if space does not permit ... MUST set the TC bit' rule section 3.1.1 states and this site cites back to it; site 3.1.1:2 maps it | If space does not permit inclusion of the answer, NSEC and RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1). |
| `3.1.3.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same 'if space does not permit ... MUST set the TC bit' rule section 3.1.1 states and this site cites back to it; site 3.1.1:2 maps it | If space does not permit inclusion of these NSEC and RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1). |
| `3.1.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same 'if space does not permit ... MUST set the TC bit' rule section 3.1.1 states and this site cites back to it; site 3.1.1:2 maps it | If space does not permit inclusion of the DS or NSEC RRset and associated RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1). |
| `3.1.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a statement that no obligation exists ('is not required to verify that a zone is properly signed'), not an obligation; the permission it grants is carried by RFC4035-3.1.5-1 | An authoritative name server is not required to verify that a zone is properly signed before sending or accepting a zone transfer. |
| `3.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 3 already binds every security-aware name server to copy the CD bit from a query into the corresponding response; site 3:3 maps it | The name server side MUST copy the setting of the CD bit from a query to the corresponding response. |
| `3.2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same AD-bit prohibition section 3.1.6 states for a security-aware name server, narrowed here to the name server side of a recursive server; site 3.1.6:1 maps it | The name server side of a security-aware recursive name server MUST NOT set the AD bit in a response unless the name server considers all RRsets in the Answer and Authority sections of the response to be authentic. |
| `4.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the lowercase restatement of the preceding sentence, naming the four cases the resolver must distinguish; site 4.3:1 maps it | More precisely, a security-aware resolver must be able to distinguish between four cases: |
| `5.3.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a lowercase forward reference to the authenticated-denial procedure section 5.4 states with a keyword; site 5.4:1 maps it | Once the validator has verified the signature, as described in Section 5.3, it must take additional steps to verify the non- existence of an exact match or closer wildcard match for the query. |
| `5.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the lowercase restatement of what the section 5.4 denial-of-existence proof consists of; site 5.4:1 maps it | To prove the non-existence of an RRset, the resolver must be able to verify both that the queried RRset does not exist and that no relevant wildcard RRset exists. |
| `C.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Appendix C.6 is a worked authentication example, and this lowercase sentence restates the section 5.4 obligation to authenticate the NSEC RR; site 5.4:1 maps it | The NSEC proves that no closer match (exact or closer wildcard) could have been used to answer this query, and the NSEC RR must also be authenticated before the answer is considered valid. |
| `C.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF intellectual property boilerplate inviting parties to disclose patent rights, not a protocol obligation on an implementation | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4035, so its obligations are stated where they were written.
